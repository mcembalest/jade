# MNIST inference

One shared problem — classify 28×28 grayscale digits — solved twice against the same data. Fetch the canonical IDX files once (52 MB):

```sh
./fetch.sh
```

- [python-baseline/](python-baseline/) — nearest-centroid, Python standard library
- [mojo-max/](mojo-max/) — the same model, native Mojo

Read the [short paper](paper.md) for methods, results, limitations, and the **jax-js browser-training TODO**.

The two implementations should match on aggregate accuracy: **773 / 1000 correct**. Per-image parity is not yet checked.
