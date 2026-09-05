#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = ["mlx==0.32.2", "numpy==2.5.2"]
# ///
"""Train, save, reload, and time a small MNIST MLP on Apple GPU or CPU."""
import argparse
import json
import platform
import statistics
import struct
import subprocess
import time
from importlib.metadata import version
from pathlib import Path

import mlx.core as mx
import mlx.nn as nn
import mlx.optimizers as optim
import numpy as np

ROOT = Path(__file__).resolve().parent


def positive(value):
    number = int(value)
    if number <= 0:
        raise argparse.ArgumentTypeError("must be positive")
    return number


def load_split(data, prefix, limit):
    images = (data / f"{prefix}-images-idx3-ubyte").read_bytes()
    labels = (data / f"{prefix}-labels-idx1-ubyte").read_bytes()
    magic, count, rows, cols = struct.unpack_from(">IIII", images)
    label_magic, label_count = struct.unpack_from(">II", labels)
    if (magic, label_magic, rows, cols) != (2051, 2049, 28, 28):
        raise ValueError("expected MNIST IDX files with 28x28 images")
    if count != label_count or len(images) != 16 + count * 784 or len(labels) != 8 + count:
        raise ValueError("truncated or mismatched MNIST files")
    if limit > count:
        raise ValueError(f"requested {limit} images, only {count} available")
    x = np.frombuffer(images, dtype=np.uint8, offset=16).reshape(count, 784)[:limit]
    y = np.frombuffer(labels, dtype=np.uint8, offset=8)[:limit]
    if np.any(y > 9):
        raise ValueError("invalid digit label")
    return mx.array(x.astype(np.float32) / 255), mx.array(y.astype(np.int32))


def benchmark(model, inputs, repeats):
    # Each call creates fresh output; eval waits for it before stopping the clock.
    start = time.perf_counter()
    mx.eval(model(inputs))
    first = time.perf_counter() - start
    for _ in range(5):
        mx.eval(model(inputs))
    samples = []
    for _ in range(repeats):
        start = time.perf_counter()
        mx.eval(model(inputs))
        samples.append((time.perf_counter() - start) * 1000)
    return {"first_call_ms": first * 1000, "warm_median_ms": statistics.median(samples),
            "warm_p95_ms": float(np.percentile(samples, 95)), "repeats": repeats,
            "warmup_calls": 5, "batch_size": inputs.shape[0]}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--device", choices=["gpu", "cpu"], default="gpu")
    parser.add_argument("--epochs", type=positive, default=3)
    parser.add_argument("--train", type=positive, default=10000)
    parser.add_argument("--test", type=positive, default=1000)
    parser.add_argument("--batch-size", type=positive, default=128)
    parser.add_argument("--repeats", type=positive, default=50)
    parser.add_argument("--seed", type=int, default=0)
    parser.add_argument("--weights", type=Path, help="load a checkpoint and skip training")
    parser.add_argument("--output", type=Path, help="defaults to results/<device>")
    args = parser.parse_args()
    if args.device == "gpu" and not mx.metal.is_available():
        parser.error("Apple Metal GPU unavailable; use a supported Mac or --device cpu")
    mx.set_default_device(mx.gpu if args.device == "gpu" else mx.cpu)
    mx.random.seed(args.seed)
    rng = np.random.default_rng(args.seed)
    output = args.output or ROOT / "results" / args.device
    setup_start = time.perf_counter()
    data = ROOT.parent / "data"
    if not data.exists():
        parser.error("MNIST data missing; run ../fetch.sh first")
    test_x, test_y = load_split(data, "t10k", args.test)
    model = nn.Sequential(nn.Linear(784, 128), nn.ReLU(), nn.Linear(128, 10))
    optimizer = optim.Adam(learning_rate=0.001)
    epoch_seconds = []
    epoch_accuracy = []
    checkpoint_load_seconds = None
    if args.weights:
        start = time.perf_counter()
        model.load_weights(str(args.weights))
        mx.eval(model.parameters())
        checkpoint_load_seconds = time.perf_counter() - start
    else:
        train_x, train_y = load_split(data, "train", args.train)
        mx.eval(train_x, train_y)
    mx.eval(test_x, test_y, model.parameters())
    setup_seconds = time.perf_counter() - setup_start

    def loss_fn(model, x, y):
        return nn.losses.cross_entropy(model(x), y, reduction="mean")

    if not args.weights:
        step = nn.value_and_grad(model, loss_fn)
        for epoch in range(args.epochs):
            start = time.perf_counter()
            order = rng.permutation(args.train)
            for offset in range(0, args.train, args.batch_size):
                indices = mx.array(order[offset:offset + args.batch_size])
                loss, grads = step(model, train_x[indices], train_y[indices])
                optimizer.update(model, grads)
                mx.eval(model.parameters(), optimizer.state, loss)
            epoch_seconds.append(time.perf_counter() - start)
            epoch_accuracy.append(mx.mean(mx.argmax(model(test_x), axis=1) == test_y).item())
            print(f"Epoch {epoch + 1}: {epoch_seconds[-1]:.3f}s, accuracy {epoch_accuracy[-1]:.3%}", flush=True)

    model.eval()
    inference = []
    for size in sorted({1, min(args.batch_size, args.test), *[n for n in [8, 32, 128, 512, 1000] if n <= args.test]}):
        inputs = test_x[:size]
        mx.eval(inputs)
        inference.append(benchmark(model, inputs, args.repeats))
    single = inference[0]
    batch = next(item for item in inference if item["batch_size"] == min(args.batch_size, args.test))
    accuracy = mx.mean(mx.argmax(model(test_x), axis=1) == test_y).item()
    output.mkdir(parents=True, exist_ok=True)
    model.save_weights(str(output / "model.safetensors"))
    hardware = platform.machine()
    if platform.system() == "Darwin":
        hardware = subprocess.check_output(["sysctl", "-n", "machdep.cpu.brand_string"], text=True).strip()
    metrics = {"framework": "mlx", "version": version("mlx"), "numpy_version": version("numpy"),
               "python": platform.python_version(), "os": platform.platform(), "hardware": hardware,
               "device": args.device, "model": "MLP 784-128-10 ReLU", "optimizer": "Adam lr=0.001",
               "compiled": False, "seed": args.seed, "training_images": 0 if args.weights else args.train,
               "test_images": args.test, "batch_size": args.batch_size,
               "checkpoint": str(args.weights) if args.weights else None,
               "setup_seconds": setup_seconds, "checkpoint_load_seconds": checkpoint_load_seconds,
               "epoch_seconds": epoch_seconds, "epoch_accuracy": epoch_accuracy, "training_seconds": sum(epoch_seconds),
               "accuracy": accuracy, "single_image": single, "batch": batch, "inference": inference}
    (output / "metrics.json").write_text(json.dumps(metrics, indent=2) + "\n")
    (output / "report.md").write_text(
        f"# MLX on {hardware} ({args.device})\n\n"
        f"MLX {version('mlx')}; MLP 784–128–10, eager execution.\n\n"
        f"- Training: {sum(epoch_seconds):.3f}s across {len(epoch_seconds)} epochs.\n"
        f"- Checkpoint loaded: {args.weights or 'none (trained here)'}.\n"
        f"- Test accuracy: {accuracy:.3%} on {args.test} images.\n"
        f"- Warm single-image median: {single['warm_median_ms']:.3f}ms.\n"
        f"- Warm {batch['batch_size']}-image batch median: {batch['warm_median_ms']:.3f}ms.\n\n"
        "Times synchronize fresh outputs and include Python dispatch. Data loading, setup, and file writes "
        "are outside training/inference timings. First calls occur after setup/training; they are not process cold starts. "
        "See metrics.json for configuration, first-call measurements, and p95 latency.\n")
    print(json.dumps(metrics, indent=2))


if __name__ == "__main__":
    main()
