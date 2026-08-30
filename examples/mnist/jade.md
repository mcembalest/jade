# MNIST inference

One shared problem — classify 28×28 grayscale digits — solved twice against the same data. Fetch the canonical IDX files once (52 MB):

```sh
./fetch.sh
```

- [python-baseline/](python-baseline/) — nearest-centroid, Python standard library
- [mojo-max/](mojo-max/) — the same model, native Mojo

The two implementations must agree exactly: **773 / 1000 correct**.
