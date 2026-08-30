# MNIST inference

One shared problem — classify 28×28 grayscale digits — solved twice against the same data.

- `data/` — canonical IDX files; `./fetch.sh` downloads them once (52 MB)
- `python-baseline/` — nearest-centroid, Python standard library
- `mojo-max/` — the same model, native Mojo

The two implementations must agree exactly: **773 / 1000 correct**.
