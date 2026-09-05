# One dataset, several stacks: MNIST on a Mac and in a browser

## Question

How much machinery is needed to train a useful small model, save it, and run low-latency inference on hardware we own? MNIST makes the computation inspectable while we explore MLX, Mojo/MAX, and jax-js. The aim is to understand performance and developer effort together.

## Current scope

MLX is the first runnable GPU training/inference experiment: an eager 784–128–10 ReLU MLP trained with Adam at learning rate 0.001, float32 pixels scaled to [0, 1], batch size 128, three epochs, and seed 0. It uses the first 10,000 training and 1,000 test images. The CPU run uses the same architecture and settings. Seeds support local repeatability; device-level numerical behavior can differ.

MAX now expresses the same MLP architecture with a custom Mojo activation and explicit backpropagation/Adam. Its CPU training and checkpoint reload are verified; the Apple GPU path still needs a working Metal compiler and validation. The jax-js example trains the MLP in the browser using automatic differentiation and Optax, with WebGPU and WebAssembly choices, checkpoint downloads, and inference measurements.

These implementations share architecture, subset sizes, and optimizer settings, but use different initialization and shuffle streams. Their timing boundaries also differ: MAX records graph compilation separately, while jax-js includes first-use compilation in its training measurements. Treat the individual plots as local experiments, not a framework ranking.

The older Python and native Mojo implementations fit ten centroids and agree on 773/1,000 correct. They explain a simple algorithm; they are not training or latency competitors to the MLP.

## Measurements

![Warm inference throughput as the batch grows](inference.svg)

The checked-in plot uses actual Apple M3 Max measurements recorded in [measurements.json](measurements.json). Every inference call constructs a fresh output and evaluates it before its timer stops. Each batch size gets five warmups followed by 50 repetitions. The curve shows batch size divided by median call duration, not concurrent-server throughput. The run is sequential and local; power state and other applications can affect the result.

Training times include shuffling, batch selection, gradients, optimizer updates, and evaluation of updated parameters. They exclude dependency installation, data loading, model initialization, per-epoch test evaluation, and checkpoint writing. First inference calls occur after setup/training; shader caches may already be warm. These measurements are not cold-start guarantees or a claim that one framework is universally faster.

## Reproduce and extend

Run the commands in this project's `jade.md` and the [MLX](mlx/), [MAX](mojo/), and [jax-js](jax-js/) introductions. Each experiment owns its saved checkpoints and full metrics; [plot.py](plot.py) generates the figures. CPU and GPU runs are separate processes or browser runs. For a strict future inference comparison between frameworks, load identical exported weights and validate their outputs before timing.

Next, hold the MLP workload fixed across frameworks and record install steps, dependencies, code needed, unsupported operations, latency distributions, and time to a chosen accuracy. Add full-dataset runs and repeated trials before drawing broad conclusions. MNIST won't establish LLM serving performance, but it provides a compact way to learn how each stack fits together.
