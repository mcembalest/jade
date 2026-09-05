# Mojo experiment

The same nearest-centroid model, written natively in Mojo via `pixi`. This experiment does not yet use MAX. Read the [short paper](paper.md) for the method and comparison limits.

```sh
../fetch.sh && pixi run --locked mnist
```

Results: [report.md](report.md), [metrics.json](metrics.json). Must match the Python baseline: **0.773**.
