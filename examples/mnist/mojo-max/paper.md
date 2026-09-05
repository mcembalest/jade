# Porting a nearest-centroid MNIST baseline to Mojo

## Abstract

This subproject translates a small Python digit classifier into native Mojo while preserving its data and algorithm. On the first 10,000 MNIST training images and first 1,000 test images, both implementations classify 773 test images correctly. The experiment establishes an aggregate correctness check before any performance work.

## Implementation

The [Mojo program](mnist.mojo) reads the same IDX files as the sibling Python baseline (`../python-baseline/mnist.py`). It accumulates each class's pixel values in a flat `List[Float64]`, divides by class counts, and evaluates squared distances from each test image to the ten resulting centroids. The loops visit classes and pixels in the same order as Python, and a strict less-than comparison resolves distance ties in favor of the first class.

Python accumulates integer sums before division; Mojo accumulates Float64 values. At the default dataset size, the integer-valued sums are small enough to be represented exactly in Float64. This makes the translation straightforward, but matching aggregate accuracy is still weaker than checking every prediction and centroid.

Despite the directory name `mojo-max`, this program uses native Mojo's standard library. It does not use MAX tensor operations, an inference graph, or a GPU. The package channel's name is not evidence that the experiment measures MAX.

## Reproduce

From this directory, with Pixi installed:

```sh
../fetch.sh
pixi run --locked mnist
```

The checked-in [environment](pixi.toml) and lockfile specify the Mojo toolchain for macOS ARM64 and Linux x86-64. The command writes `metrics.json` and `report.md`; these generated files are excluded from version control. The program caps training at 10,000 images and evaluation at 1,000 images. Unlike Python, it does not read subset-size environment overrides.

## Result and interpretation

| Model | Training images | Test images | Correct | Accuracy |
| --- | ---: | ---: | ---: | ---: |
| Nearest centroid | 10,000 | 1,000 | 773 | 0.773 |

This agrees with the parent experiment described in `../paper.md`. The result measures predictive accuracy on a fixed prefix of the test set. It does not establish full-dataset accuracy, per-image parity, robustness to malformed IDX files, or a performance advantage over Python.

## Next experiments

Export predictions and compare them directly with Python before optimizing. A timing experiment should separate compilation, file loading, centroid fitting, and inference, and record the toolchain, hardware, repetitions, and dataset sizes. An eventual MAX implementation should be a separate experiment with an explicit model and backend; retain this native Mojo version as the small correctness baseline.
