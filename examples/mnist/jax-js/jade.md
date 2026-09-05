# MNIST, in the browser

Train a small neural network locally with [jax-js](https://github.com/ekzhang/jax-js), then keep the weights and measure inference. Choose **WebGPU** for the GPU or **WebAssembly** for the CPU. The page shows which backends your browser supports; it never silently changes your choice.

![Learning and inference in the browser](learning.svg)

## Run

From this folder, with Node.js 22+ installed:

```sh
../fetch.sh
npm ci
npm start
```

Open the printed local URL. **Train from scratch** runs a 784 → 128 → 10 ReLU MLP with Adam (0.001), batch 128, seed 0, three epochs on the first 10,000 training digits. Evaluation uses the first 1,000 test digits. **Save weights** exports JSON; **Reload weights** restores it in a fresh page without retraining. **Measure inference** compares six batch sizes and **Save measurements** keeps the run's numbers.

Training and inference run entirely in your browser. The small server only serves the page and four local MNIST files. The interactive experiment opens separately from JaDE's document preview.

## What the numbers mean

Training time includes batch gathering, automatic differentiation, Adam, synchronization and first-use compilation. It excludes loading, initialization, shuffling, evaluation and chart rendering. Browser/GPU driver caches can already be warm. Warm inference medians follow five warmups and 20 synchronized calls, with weights and input resident; transfers and result readback are excluded. “First timed call” may already follow evaluation at the same shape. Browser timer resolution limits very short measurements.

The architecture matches the MLX/MAX examples, but random initialization and shuffling use different generators. These are reproducible local experiments, not a controlled framework speed ranking.

## Repeatable checks

```sh
npx playwright install chromium
npm test
# Full run in installed Chrome, explicitly requesting WebGPU:
CHROME_CHANNEL=chrome BACKEND=webgpu TRAIN=10000 EPOCHS=3 npm test
```

The short default test uses WebAssembly and 512 training digits. It trains through the real UI, checks falling loss, measures all six batch sizes, exports weights, reloads the page, and confirms identical test accuracy after import. It also checks device logits against an independent scalar JavaScript implementation, and verifies that malformed weights leave the model intact. Measurements and a screenshot are written to ignored `results/`.

The checked-in [measurements](measurements.json) come from the full WebGPU run. After preparing the MLX and Python plot inputs [in the parent project](../jade.md), regenerate the charts with `uv run --script --locked ../plot.py` from this folder. The [pixel landscape](landscape.svg) offers another view: NumPy PCA of 4,000 shared digits.
