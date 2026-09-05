# MNIST: from a MacBook to the browser

One dataset, several ways to own the training and inference stack. Explore **latency, GPU acceleration, setup simplicity, and how much code we need to control**. Accuracy tells us whether the computation is useful; it is not the main contest.

![MLX CPU and GPU inference throughput across batch sizes](inference.svg)

Measured on an Apple M3 Max with MLX 0.32.2. Warm, synchronized inference; Python dispatch included. This is one local run, not a framework ranking. The [paper](paper.md) explains the measurements and their limits.

## Explore

| Inner JaDE | What you can do now | Next question |
| --- | --- | --- |
| [MLX](mlx/) | Train an MLP on the Apple GPU or CPU, save/reload weights, measure latency | Where does batching make GPU execution worthwhile? |
| [Mojo / MAX](mojo/) | Train a MAX MLP with a custom Mojo activation; save/reload and time it on CPU | What does the Apple GPU path require? Metal toolchain verification remains open. |
| [jax-js](jax-js/) | Train in the browser, save/reload weights, and measure WebGPU or WebAssembly inference | How much work can stay entirely inside the browser? |
| [Python reference](python-baseline/) | Understand a ten-centroid model with no ML dependencies | What work are the accelerated stacks replacing? |

## Run the GPU experiment

With [uv](https://docs.astral.sh/uv/) installed, from this directory:

```sh
./fetch.sh
uv run --script --locked mlx/mnist.py
uv run --script --locked mlx/mnist.py --device cpu
```

The four canonical IDX files are shared by every implementation. Downloads and framework installation happen before benchmark timing. MLX results go to `mlx/results/<device>/`.

To regenerate all checked-in plots after those runs:

```sh
(cd python-baseline && MNIST_TRAIN=10000 MNIST_TEST=1000 python3 mnist.py ..)
uv run --script --locked plot.py
```

MLX plot inputs are recorded in [measurements.json](measurements.json); MAX and jax-js keep measurements beside their own introductions. Each framework has its own `jade.md` and matching local `README.md`; all belong to this one MNIST project.
