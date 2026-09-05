# MLX: a small model, a real Apple GPU

![MLX learning progress and time spent in each training epoch](learning.svg)

A 784 → 128 → 10 MLP, ReLU, Adam, and ordinary Python. This local run uses the first 10,000 training images and 1,000 test images, three epochs, batch size 128, and seed 0. Test accuracy is a sanity check alongside the timing story.

## Train and measure

From this directory, with uv and a supported Apple-silicon Mac:

```sh
../fetch.sh
uv run --script --locked mnist.py
uv run --script --locked mnist.py --device cpu
```

GPU execution is the default and fails clearly if Metal is unavailable. The CPU run is explicit. Dependencies are isolated and pinned by the script and its lockfile; no global Python environment is modified.

Each run saves `results/<device>/model.safetensors`, `metrics.json`, and `report.md`. It measures synchronized per-epoch training, first calls at several inference batch sizes, and warm median/p95 latency. It uses eager execution, without explicit `mx.compile` or custom kernels.

## Reload for inference

```sh
uv run --script --locked mnist.py --weights results/gpu/model.safetensors --output results/reloaded
```

This skips training and repeats the inference benchmark using saved weights. Loading time is recorded separately. Use `--help` to change dataset sizes, batch size, epochs, or repetitions.

Warm timings include Python dispatch and output evaluation. They exclude downloads, data preparation, checkpoint writes, and process startup. Per-epoch test evaluation is outside training timing. First calls occur after model setup or training and may reuse Metal shader caches from previous runs; they are not cold-install measurements. Tiny batches can favor the CPU.

The API patterns are documented in [MLX optimizers](https://ml-explore.github.io/mlx/build/html/python/optimizers.html); [lazy evaluation](https://ml-explore.github.io/mlx/build/html/usage/lazy_evaluation.html) is why every timed output is explicitly evaluated.
