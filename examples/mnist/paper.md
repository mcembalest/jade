# One average image per digit: a small MNIST baseline

## Abstract

How much digit recognition can ten average images explain? This example fits a nearest-centroid classifier to the first 10,000 MNIST training images and evaluates the first 1,000 test images. The Python standard-library implementation classifies 773 correctly (77.3%). A companion native Mojo implementation produces the same aggregate result. This is a reproducible baseline for exploring implementations, not a competitive digit recognizer.

## Method

The [Python source](python-baseline/mnist.py) reads MNIST's IDX images and labels. Each 28 × 28 image becomes 784 raw pixel intensities. For each digit, fitting computes the mean of its training images. Prediction selects the centroid with the smallest sum of squared pixel differences; ties favor the lower digit. There is no gradient descent, augmentation, normalization, or hyperparameter search.

The default subsets are prefixes of the supplied dataset, not random samples. Python allows `MNIST_TRAIN` and `MNIST_TEST` overrides; leave these unset for the comparison below. Very small training subsets may omit a class and cannot be used by this implementation.

## Reproduce

From this directory, with Python 3.10 or newer available:

```sh
./fetch.sh
(cd python-baseline && MNIST_TRAIN=10000 MNIST_TEST=1000 python3 mnist.py ..)
(cd mojo-max && pixi run --locked mnist)
```

The [fetch script](fetch.sh) downloads the four shared IDX files from the Google-hosted MNIST mirror. The Mojo command requires Pixi and a platform listed in its [environment](mojo-max/pixi.toml). Each implementation writes `metrics.json` and `report.md` in its own directory; Python also writes `model.json` with the ten centroids. Generated artifacts are not committed. The [Mojo paper](mojo-max/paper.md) describes the comparison's scope.

## Results and limits

| Implementation | Training images | Test images | Correct | Accuracy |
| --- | ---: | ---: | ---: | ---: |
| Python standard library | 10,000 | 1,000 | 773 | 0.773 |
| Native Mojo | 10,000 | 1,000 | 773 | 0.773 |

These are results for the default subsets. Equal totals do not prove identical per-image predictions: the scripts currently export only aggregate metrics. Neither script records execution time, so this experiment supports no speed claim. Pixel-distance centroids also ignore translation and other variations in handwriting. Evaluation on the full test set would answer a different, useful question.

## TODO: train in the browser with jax-js

Add a `jax-js/` subproject with its own `jade.md`, pinned dependency, runnable browser experiment, and saved metrics. [jax-js](https://github.com/ekzhang/jax-js) supplies JAX-style array operations with WebAssembly and WebGPU backends. Its [MNIST demo](https://jax-js.com/mnist) trains an MLP or convolutional network on 60,000 training images with Adam. That is a different model and workload from this centroid baseline.

Investigate the reported subsecond browser-training result in this [post by Eric Zhang](https://x.com/ekzhang1/status/2095713466174583152?s=20). The linked post could not be retrieved for verification; subsecond training remains an open benchmark target here. When implementing the experiment, record model, accuracy, epochs, dataset size, browser, hardware, backend, and whether timing includes loading and compilation. Treat the timing as motivation to investigate, not a result of this example.
