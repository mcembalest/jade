# MLX

![MLX training latency and inference throughput](learning.svg)

| Implementation | Setting |
| --- | --- |
| Model, optimizer, timing | [Shared protocol](../README.md#protocol) |
| Code | [compare.py](../compare.py): MLX adapter |
| Execution | Eager; automatic differentiation; CPU or Metal |
| Adam | Bias correction explicitly enabled |
| Results | [measurements.json](../measurements.json): `mlx-cpu`, `mlx-gpu` |

## Run

Apple silicon · `uv`

```sh
../fetch.sh
uv run --script --locked ../compare.py --backend mlx --device cpu
uv run --script --locked ../compare.py --backend mlx --device gpu
```

Results: `../results/mlx-{cpu,gpu}.json`

## Chart

```sh
uv run --script --locked ../plot.py
```
