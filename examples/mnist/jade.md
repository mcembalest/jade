# MNIST, three ways

Train a small model, keep its weights, and measure how quickly it answers. Explore GPU acceleration and how much code it takes to own the stack.

![MLX CPU and GPU throughput across batch sizes](inference.svg)

## Choose a stack

| JaDE | Implementation | Runs on |
| --- | --- | --- |
| [MLX](mlx/) | Python with automatic differentiation | Apple GPU or CPU |
| [Mojo/MAX](mojo-max/) | MAX graph, explicit derivatives, custom Mojo activation | Apple GPU or CPU |
| [jax-js](jax-js/) | Browser training with automatic differentiation and Optax | WebGPU or WebAssembly |

Each folder contains its code, chart, recorded measurements, and run instructions. Its `README.md` points to its `jade.md`.

## One workload

A **784 → 128 → 10 ReLU MLP**, float32 pixels scaled to [0, 1], Adam at 0.001, batch size 128, three epochs, seed 0. Use the first 10,000 training and 1,000 test images. Accuracy checks useful learning; latency and implementation simplicity are the questions.

Download the four shared MNIST files once with `./fetch.sh`. They live in `data/`; each implementation writes local outputs under its own `results/`. Neither is committed.

Architecture and settings match, but initialization and shuffle streams differ. MAX records graph compilation separately; jax-js includes first-use compilation in training time. Measurements are separate local runs, with potentially warm caches. A controlled framework comparison would need identical weights, timing boundaries, and repeated trials.

## Rebuild the charts

With [uv](https://docs.astral.sh/uv/) installed:

```sh
uv run --script --locked plot.py
```

Every chart reads checked-in `measurements.json` files. No training or dataset download is required, and rendering never overwrites measurements. The overview chart uses the MLX CPU/GPU runs on an Apple M3 Max.
