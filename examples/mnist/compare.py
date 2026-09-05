# /// script
# requires-python = ">=3.12"
# dependencies = ["numpy==2.5.2", "mlx==0.32.2", "torch==2.10.0"]
# ///
"""Shared MNIST latency protocol. Run each backend separately on an idle machine."""

import argparse
import hashlib
import json
import os
import platform
import statistics
import struct
import subprocess
import sys
import time
from pathlib import Path

# One CPU thread for numerical libraries; GPU rows are explicitly separate.
for name in (
    "OMP_NUM_THREADS",
    "OPENBLAS_NUM_THREADS",
    "VECLIB_MAXIMUM_THREADS",
    "MKL_NUM_THREADS",
):
    os.environ[name] = "1"
import numpy as np

ROOT = Path(__file__).resolve().parent
SHAPES = [(784, 128), (128,), (128, 10), (10,)]
NAMES = ["w1", "b1", "w2", "b2"]


def load(prefix, limit):
    raw = (ROOT / "data" / f"{prefix}-images-idx3-ubyte").read_bytes()
    labels = (ROOT / "data" / f"{prefix}-labels-idx1-ubyte").read_bytes()
    magic, n, rows, cols = struct.unpack_from(">IIII", raw)
    lm, ln = struct.unpack_from(">II", labels)
    if (
        (magic, lm, rows, cols) != (2051, 2049, 28, 28)
        or n != ln
        or len(raw) != 16 + n * 784
        or len(labels) != 8 + n
        or limit > n
    ):
        raise ValueError("invalid MNIST files; run ./fetch.sh")
    return (
        np.frombuffer(raw, np.uint8, offset=16)
        .reshape(n, 784)[:limit]
        .astype(np.float32)
        / 255,
        np.frombuffer(labels, np.uint8, offset=8)[:limit].astype(np.int32),
    )


def fixture():
    rng = np.random.default_rng(0)
    weights = [
        rng.normal(0, 1 / np.sqrt(784), SHAPES[0]).astype(np.float32),
        np.zeros(128, np.float32),
        rng.normal(0, 1 / np.sqrt(128), SHAPES[2]).astype(np.float32),
        np.zeros(10, np.float32),
    ]
    order = np.random.default_rng(1)
    return weights, [order.permutation(10000) for _ in range(3)]


def forward(w, x):
    return np.maximum(x @ w[0] + w[1], 0) @ w[2] + w[3]


def gradients(w, x, y):
    z = x @ w[0] + w[1]
    h = np.maximum(z, 0)
    logits = h @ w[2] + w[3]
    p = np.exp(logits - logits.max(1, keepdims=True))
    p /= p.sum(1, keepdims=True)
    p[np.arange(len(y)), y] -= 1
    p /= len(y)
    dh = (p @ w[2].T) * (z > 0)
    return [x.T @ dh, dh.sum(0), h.T @ p, p.sum(0)]


def adapter(backend, device):
    """Return reset, train-step, forward, host-read, sync, version, compile-times."""
    compilation = {}
    if backend == "numpy":
        if device != "cpu":
            raise ValueError("NumPy baseline is CPU only")

        def reset(initial):
            nonlocal w, m, v, t
            w = [a.copy() for a in initial]
            m = [np.zeros_like(a) for a in w]
            v = [a.copy() for a in m]
            t = 0

        w = m = v = []
        t = 0

        def step(x, y):
            nonlocal w, m, v, t
            g = gradients(w, x, y)
            t += 1
            m = [0.9 * a + 0.1 * b for a, b in zip(m, g)]
            v = [0.999 * a + 0.001 * b * b for a, b in zip(v, g)]
            w = [
                a - 0.001 * (b / (1 - 0.9**t)) / (np.sqrt(c / (1 - 0.999**t)) + 1e-8)
                for a, b, c in zip(w, m, v)
            ]

        return (
            reset,
            step,
            lambda x: forward(w, x),
            lambda a: a,
            lambda a: None,
            np.__version__,
            compilation,
        )
    if backend == "torch":
        import torch

        torch.set_num_threads(1)
        target = "mps" if device == "gpu" else "cpu"
        if target == "mps" and not torch.backends.mps.is_available():
            raise RuntimeError("MPS unavailable")
        w = []
        solver = None

        def reset(initial):
            nonlocal w, solver
            w = [torch.tensor(a, device=target, requires_grad=True) for a in initial]
            solver = torch.optim.Adam(
                w, lr=0.001, betas=(0.9, 0.999), eps=1e-8, foreach=False
            )

        def predict(x):
            return torch.relu(x @ w[0] + w[1]) @ w[2] + w[3]

        def step(x, y):
            solver.zero_grad(set_to_none=True)
            torch.nn.functional.cross_entropy(
                predict(torch.tensor(x, device=target)),
                torch.tensor(y, dtype=torch.long, device=target),
            ).backward()
            solver.step()
            if target == "mps":
                torch.mps.synchronize()

        def infer(x):
            with torch.no_grad():
                return predict(x)

        def resident(x):
            return torch.tensor(x, device=target)

        infer.resident = resident
        return (
            reset,
            step,
            infer,
            lambda a: a.detach().cpu().numpy(),
            lambda a: torch.mps.synchronize() if target == "mps" else None,
            torch.__version__,
            compilation,
        )
    if backend == "mlx":
        import mlx.core as mx
        import mlx.nn as nn
        import mlx.optimizers as optim
        from importlib.metadata import version

        mx.set_default_device(mx.gpu if device == "gpu" else mx.cpu)
        w = []
        solver = None

        def reset(initial):
            nonlocal w, solver
            w = {"layers": [mx.array(a) for a in initial]}
            solver = optim.Adam(
                0.001, betas=[0.9, 0.999], eps=1e-8, bias_correction=True
            )
            solver.init(w)
            mx.eval(w, solver.state)

        def predict(weights, x):
            a = weights["layers"]
            return mx.maximum(x @ a[0] + a[1], 0) @ a[2] + a[3]

        grad = mx.value_and_grad(
            lambda weights, x, y: nn.losses.cross_entropy(
                predict(weights, x), y, reduction="mean"
            )
        )

        def step(x, y):
            nonlocal w
            loss, g = grad(w, mx.array(x), mx.array(y))
            w = solver.apply_gradients(g, w)
            mx.eval(w, solver.state, loss)

        def infer(x):
            return predict(w, x)

        infer.resident = mx.array
        return (
            reset,
            step,
            infer,
            lambda a: np.array(a),
            lambda a: mx.eval(a),
            version("mlx"),
            compilation,
        )
    if backend == "max":
        sys.path.insert(0, str(ROOT / "mojo-max"))
        from model import build
        from max.driver import Accelerator, CPU, Buffer
        from max.engine import InferenceSession

        target = Accelerator() if device == "gpu" else CPU()
        session = InferenceSession(devices=[target])
        models = {}
        state = []
        t = 0

        def buffer(x):
            return Buffer.from_numpy(np.ascontiguousarray(x, dtype=np.float32)).to(
                target
            )

        for size, train in [
            (128, True),
            (16, True),
            (1, False),
            (128, False),
            (1000, False),
        ]:
            start = time.perf_counter()
            models[size, train] = session.load(build(target, size, train))
            compilation[f"{size}:{train}"] = time.perf_counter() - start

        def reset(initial):
            nonlocal state, t
            state = [buffer(a) for a in initial] + [
                buffer(np.zeros(s, np.float32)) for s in SHAPES * 2
            ]
            t = 0
            target.synchronize()

        def step(x, y):
            nonlocal state, t
            t += 1
            state = list(
                models[len(x), True].execute(
                    buffer(x),
                    *state[:4],
                    buffer(np.eye(10, dtype=np.float32)[y]),
                    buffer(np.array([1 - 0.9**t, 1 - 0.999**t], np.float32)),
                    *state[4:],
                )
            )
            target.synchronize()

        def infer(x):
            return models[x.shape[0], False].execute(x, *state[:4])[0]

        infer.resident = buffer
        return (
            reset,
            step,
            infer,
            lambda a: a.to(CPU()).to_numpy(),
            lambda a: target.synchronize(),
            json.loads(
                next((Path(sys.prefix) / "conda-meta").glob("max-*.json")).read_text()
            )["version"],
            compilation,
        )
    raise ValueError(backend)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--backend", choices=["numpy", "torch", "mlx", "max"], default="numpy"
    )
    parser.add_argument("--device", choices=["cpu", "gpu"], default="cpu")
    parser.add_argument("--trials", type=int, default=3)
    args = parser.parse_args()
    if args.trials < 1:
        parser.error("trials must be positive")
    initial, orders = fixture()
    x, y = load("train", 10000)
    tx, ty = load("t10k", 1000)
    batches = [
        (x[order[i : i + 128]], y[order[i : i + 128]])
        for order in orders
        for i in range(0, 10000, 128)
    ]
    shared = {
        "weights": {k: a.tolist() for k, a in zip(NAMES, initial)},
        "orders": [o.tolist() for o in orders],
    }
    reference = adapter("numpy", "cpu")
    reference[0](initial)
    reference[1](*batches[0])
    shared["initial_logits"] = forward(initial, tx[:128]).tolist()
    shared["first_update_logits"] = reference[2](tx[:128]).tolist()
    shared["dataset_sha256"] = {
        p.name: hashlib.sha256(p.read_bytes()).hexdigest()
        for p in sorted((ROOT / "data").glob("*-ubyte"))
    }
    fixture_path = ROOT / "data/comparison.json"
    if not fixture_path.exists() or "first_update_logits" not in json.loads(
        fixture_path.read_text()
    ):
        fixture_path.write_text(json.dumps(shared, separators=(",", ":")))
    recorded = json.loads(fixture_path.read_text())
    if (
        recorded["weights"] != shared["weights"]
        or recorded["orders"] != shared["orders"]
        or recorded["dataset_sha256"] != shared["dataset_sha256"]
    ):
        raise ValueError(
            "comparison fixture differs; remove data/comparison.json to regenerate"
        )
    identity = hashlib.sha256(fixture_path.read_bytes()).hexdigest()
    start = time.perf_counter()
    reset, step, infer, host, sync, version, compilation = adapter(
        args.backend, args.device
    )
    resident = getattr(infer, "resident", lambda a: a)
    reset(initial)
    check = resident(tx[:128])
    sync(check)
    actual = host(infer(check))
    np.testing.assert_allclose(actual, forward(initial, tx[:128]), atol=2e-4, rtol=2e-4)
    setup = time.perf_counter() - start
    # First update warms the training path; tail shape is warmed too. Both are discarded.
    start = time.perf_counter()
    step(*batches[0])
    first_step = time.perf_counter() - start
    np.testing.assert_allclose(
        host(infer(check)), reference[2](tx[:128]), atol=3e-4, rtol=3e-4
    )
    step(*batches[78])
    totals = []
    accuracies = []
    epochs = []
    for trial in range(args.trials):
        reset(initial)
        per_epoch = []
        for epoch in range(3):
            start = time.perf_counter()
            for xb, yb in batches[epoch * 79 : (epoch + 1) * 79]:
                step(xb, yb)
            per_epoch.append(time.perf_counter() - start)
        prediction = host(infer(resident(tx)))
        accuracy = float(np.mean(prediction.argmax(1) == ty))
        if not np.all(np.isfinite(prediction)) or accuracy < 0.85:
            raise RuntimeError(f"training sanity check failed: {accuracy}")
        epochs.append(per_epoch)
        totals.append(sum(per_epoch))
        accuracies.append(accuracy)
        print(args.backend, args.device, trial, totals[-1], accuracy, flush=True)
    # Identical initialization also makes inference outputs directly comparable.
    reset(initial)
    inference = []
    for size in [1, 128, 1000]:
        inputs = resident(tx[:size])
        sync(inputs)
        start = time.perf_counter()
        out = infer(inputs)
        sync(out)
        first = (time.perf_counter() - start) * 1000
        np.testing.assert_allclose(
            host(out), forward(initial, tx[:size]), atol=2e-4, rtol=2e-4
        )
        for _ in range(5):
            sync(infer(inputs))
        samples = []
        for _ in range(50):
            start = time.perf_counter()
            out = infer(inputs)
            sync(out)
            samples.append((time.perf_counter() - start) * 1000)
        inference.append(
            {
                "batch": size,
                "first_ms": first,
                "median_ms": statistics.median(samples),
                "p95_ms": float(np.percentile(samples, 95)),
                "samples_ms": samples,
            }
        )
    result = {
        "backend": args.backend,
        "device": args.device,
        "version": version,
        "python": platform.python_version(),
        "os": platform.platform(),
        "cpu_threads": "1 for NumPy/PyTorch; runtime default for MLX/MAX",
        "numpy_version": np.__version__,
        "hardware": subprocess.check_output(
            ["sysctl", "-n", "machdep.cpu.brand_string"], text=True
        ).strip(),
        "fixture_sha256": identity,
        "setup_seconds": setup,
        "compile_seconds": compilation,
        "first_train_step_ms": first_step * 1000,
        "training_seconds": totals,
        "epoch_seconds": epochs,
        "accuracy": accuracies,
        "inference": inference,
    }
    output = ROOT / "results"
    output.mkdir(exist_ok=True)
    (output / f"{args.backend}-{args.device}.json").write_text(
        json.dumps(result, indent=2) + "\n"
    )


if __name__ == "__main__":
    main()
