# Python baseline

Nearest-centroid, standard library only: average one image per digit, closest centroid wins.

`mnist.py` reads `../data`, writes `model.json` and the metrics. Expected accuracy: **0.773**.

Artifact: metrics.json

Command: ../fetch.sh && python3 mnist.py ..
