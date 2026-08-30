# Python baseline

Nearest-centroid, standard library only: average one image per digit, closest centroid wins.

`mnist.py` reads `../data`, writes the report below (the model itself, drawn as ten centroid images) plus `model.json` and `metrics.json`. Expected accuracy: **0.773**.

Artifact: report.md

Command: ../fetch.sh && python3 mnist.py ..
