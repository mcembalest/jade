"""Train and benchmark MNIST with MAX and a custom Mojo GPU activation."""
import argparse
import json
import platform
import statistics
import struct
import subprocess
import time
from pathlib import Path

import numpy as np
from max.driver import Accelerator, Buffer, CPU, accelerator_count
from max.engine import InferenceSession
from model import SHAPES, build, initialize, reference

ROOT = Path(__file__).resolve().parent


def positive(value):
    value = int(value)
    if value <= 0:
        raise argparse.ArgumentTypeError("must be positive")
    return value


def data(prefix, limit):
    root = ROOT.parent / "data"
    raw = (root / f"{prefix}-images-idx3-ubyte").read_bytes()
    labels = (root / f"{prefix}-labels-idx1-ubyte").read_bytes()
    magic, count, rows, cols = struct.unpack_from(">IIII", raw)
    lm, lc = struct.unpack_from(">II", labels)
    if (magic, lm, rows, cols) != (2051, 2049, 28, 28) or count != lc or len(raw) != 16+count*784 or len(labels) != 8+count or limit > count:
        raise ValueError("invalid MNIST data or subset size; run ../fetch.sh")
    x = np.frombuffer(raw, np.uint8, offset=16).reshape(count, 784)[:limit].astype(np.float32)/255
    y = np.frombuffer(labels, np.uint8, offset=8)[:limit].astype(np.int32)
    if np.any(y > 9):
        raise ValueError("invalid digit label")
    return x, y


def hardware():
    if platform.system() == "Darwin":
        return subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip()
    return platform.processor() or platform.machine()


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--device", choices=["gpu", "cpu"], default="gpu")
    p.add_argument("--epochs", type=positive, default=3)
    p.add_argument("--train", type=positive, default=10000)
    p.add_argument("--test", type=positive, default=1000)
    p.add_argument("--batch-size", type=positive, default=128)
    p.add_argument("--repeats", type=positive, default=50)
    p.add_argument("--seed", type=int, default=0)
    p.add_argument("--weights", type=Path)
    p.add_argument("--output", type=Path)
    args = p.parse_args()
    if args.device == "gpu" and not accelerator_count():
        p.error("no MAX accelerator; use --device cpu explicitly")
    if args.device == "gpu" and platform.system() == "Darwin":
        for tool in ("metal", "metallib"):
            found = subprocess.run(["xcrun", "--find", tool], capture_output=True)
            if found.returncode:
                p.error("MAX GPU compilation requires Apple’s Metal Toolchain. "
                        "Install full Xcode and its Metal Toolchain component, "
                        "or use --device cpu explicitly.")
    device = Accelerator() if args.device == "gpu" else CPU()
    session = InferenceSession(devices=[device])
    buffer = lambda x: Buffer.from_numpy(np.ascontiguousarray(x, dtype=np.float32)).to(device)
    host = lambda x: x.to(CPU()).to_numpy()
    test_x, test_y = data("t10k", args.test)
    weights = initialize(args.seed)
    if args.weights:
        with np.load(args.weights) as checkpoint:
            weights = [checkpoint[f"w{i}"] for i in range(4)]
    with np.errstate(over="ignore", invalid="ignore"):
        weights = [w.astype(np.float32) for w in weights]
    if any(w.shape != shape or not np.all(np.isfinite(w)) for w, shape in zip(weights, SHAPES)):
        p.error("checkpoint has wrong shapes or nonfinite values")
    state = [buffer(w) for w in weights]
    models, compilation = {}, {}

    def compiled(batch, train=False):
        key = (batch, train)
        if key not in models:
            start = time.perf_counter()
            models[key] = session.load(build(device, batch, train))
            compilation[f"{'train' if train else 'infer'}_batch_{batch}"] = time.perf_counter()-start
        return models[key]

    # Validate our custom operation/model against an independent NumPy forward pass.
    check_size = min(3, args.test)
    check = host(compiled(check_size).execute(buffer(test_x[:check_size]), *state)[0])
    if not np.all(np.isfinite(check)):
        p.error("checkpoint produces nonfinite predictions")
    np.testing.assert_allclose(check, reference(weights, test_x[:check_size]), atol=2e-4, rtol=2e-4)
    epoch_seconds, epoch_accuracy = [], []
    if not args.weights:
        train_x, train_y = data("train", args.train)
        targets = np.eye(10, dtype=np.float32)[train_y]
        for size in {min(args.batch_size, args.train), args.train % args.batch_size} - {0}:
            compiled(size, True)
        compiled(args.test)
        state += [buffer(np.zeros(s, np.float32)) for s in SHAPES * 2]
        rng = np.random.default_rng(args.seed)
        step = 0
        for epoch in range(args.epochs):
            start = time.perf_counter()
            order = rng.permutation(args.train)
            for offset in range(0, args.train, args.batch_size):
                idx = order[offset:offset+args.batch_size]
                step += 1
                corrections = buffer(np.array([1-.9**step, 1-.999**step], np.float32))
                state = list(compiled(len(idx), True).execute(buffer(train_x[idx]), *state[:4],
                              buffer(targets[idx]), corrections, *state[4:]))
                device.synchronize()
            epoch_seconds.append(time.perf_counter()-start)
            pred = host(compiled(args.test).execute(buffer(test_x), *state[:4])[0])
            epoch_accuracy.append(float(np.mean(pred.argmax(1) == test_y)))
            print(f"Epoch {epoch+1}: {epoch_seconds[-1]:.3f}s, {epoch_accuracy[-1]:.3%}", flush=True)
    weights = [host(w).copy() for w in state[:4]]
    inference = []
    for size in [n for n in [1, 8, 32, 128, 512, 1000] if n <= args.test]:
        model = compiled(size)
        inputs = buffer(test_x[:size])
        device.synchronize()
        samples = []
        for repetition in range(args.repeats+6):
            start = time.perf_counter()
            output = model.execute(inputs, *state[:4])[0]
            device.synchronize()
            elapsed = (time.perf_counter()-start)*1000
            if repetition == 0:
                first = elapsed
            if repetition >= 6:
                samples.append(elapsed)
        prediction = host(output)
        if not np.all(np.isfinite(prediction)):
            p.error("model produces nonfinite predictions")
        np.testing.assert_allclose(prediction, reference(weights, test_x[:size]), atol=.001, rtol=.001)
        inference.append({"batch_size": size, "first_timed_call_ms": first, "warm_median_ms": statistics.median(samples),
                          "warm_p95_ms": float(np.percentile(samples, 95)), "repeats": args.repeats})
    prediction = host(compiled(args.test).execute(buffer(test_x), *state[:4])[0])
    accuracy = float(np.mean(prediction.argmax(1) == test_y))
    directory = args.output or ROOT / "results" / args.device
    directory.mkdir(parents=True, exist_ok=True)
    np.savez(directory / "model.npz", **{f"w{i}": w for i, w in enumerate(weights)})
    metrics = {"framework":"max", "version":"26.5.0", "mojo_version":subprocess.check_output(["mojo","--version"],text=True).strip(),
               "device":args.device, "hardware":hardware(), "os":platform.platform(),
               "model":"MLP 784-128-10 ReLU", "optimizer":"Adam lr=0.001; explicit derivatives", "seed":args.seed,
               "training_images":0 if args.weights else args.train, "test_images":args.test, "batch_size":args.batch_size,
               "training_seconds":sum(epoch_seconds), "epoch_seconds":epoch_seconds, "epoch_accuracy":epoch_accuracy,"accuracy":accuracy,
               "compile_seconds":compilation,"inference":inference,
               "timing":"device synchronize; inference inputs/weights resident; compile and output host copy excluded; "
                        "first_timed_call_ms is the first benchmark call (evaluation may already have used this shape); "
                        "training includes per-batch input transfer and excludes compilation and epoch evaluation"}
    (directory/"metrics.json").write_text(json.dumps(metrics,indent=2)+"\n")
    print(json.dumps(metrics,indent=2))


if __name__ == "__main__":
    main()
