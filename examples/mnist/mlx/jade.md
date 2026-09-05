# MLX

![MLX training latency and inference throughput](learning.svg)

| Implementation | Setting |
| --- | --- |
| Model, optimizer, timing | [Shared protocol](../jade.md#protocol) |
| Code | [compare.py](../compare.py): MLX adapter |
| Execution | Eager; automatic differentiation; CPU or Metal |
| Adam | Bias correction explicitly enabled |
| Results | [measurements.json](../measurements.json): `mlx-cpu`, `mlx-gpu` |

## Run

From this folder, on an Apple-silicon Mac with `uv`:

```sh
../fetch.sh
uv run --script --locked ../compare.py --backend mlx --device cpu
uv run --script --locked ../compare.py --backend mlx --device gpu
```

Each run checks initial logits and one Adam update against NumPy, then checks trained accuracy. All timed updates and inference outputs are evaluated explicitly. Raw results: `../results/mlx-{cpu,gpu}.json`.

## Chart

```sh
uv run --script --locked ../plot.py
```
