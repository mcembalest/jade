# Mojo experiment

The identical nearest-centroid model, written natively in Mojo via `pixi`.

`mnist.mojo` reads `../data`, writes the metrics. Must match the Python baseline: **0.773**.

Artifact: metrics.json

Command: ../fetch.sh && pixi run mnist
