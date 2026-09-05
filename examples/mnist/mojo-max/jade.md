# Mojo/MAX: own the operation, run the graph

A **Mojo ReLU inside a MAX graph**. MAX handles the matrix operations; a small training graph expresses backpropagation and Adam. Train on CPU or the Apple GPU, save the weights, and measure inference.

![MAX compilation cost and CPU/GPU inference throughput](latency.svg)

## Run

With [Pixi](https://pixi.sh) installed, from this directory:

```sh
../fetch.sh
pixi run --locked python run.py --device gpu
pixi run --locked python run.py --device gpu --weights results/gpu/model.npz --output results/gpu-reloaded
pixi run --locked python verify.py --device gpu
```

Choose `--device cpu` in either script for CPU execution. The Apple GPU needs Xcode's **Metal Toolchain** component; install it in Xcode Settings or with `xcodebuild -downloadComponent MetalToolchain`. The environment pins MAX 26.5.0 and Mojo 1.0.0.

## Read the code

[model.py](model.py) defines the [shared MLP workload](../jade.md), its explicit cross-entropy derivatives, and Adam updates (betas 0.9/0.999, epsilon 1e-8). [kernels/relu.mojo](kernels/relu.mojo) owns the activation. [run.py](run.py) trains, saves/reloads, and benchmarks. [verify.py](verify.py) compares forward execution and optimizer updates with NumPy, checking the reference gradients by finite differences.

Checkpoints in `results/<device>/model.npz` contain inference weights. They do not contain optimizer state for resumed training. Both CPU and Apple GPU pass numerical checks and fresh-process reload; both recorded runs reach 917/1,000 correct.

## Read the measurements

[measurements.json](measurements.json) preserves CPU and GPU runs on an Apple M3 Max. The left chart shows GPU compilation, first timed call, and warm median for 128 images; the right shows warm throughput across batch sizes. CPU inference was faster at every tested size in these runs.

Each shape gets a separate graph. Training includes shuffling, batch construction, transfers, updates, and synchronization; graph compilation and per-epoch evaluation are excluded. Inference uses resident inputs and weights, synchronizes output, and excludes host readback. Five warmups precede 50 measured calls. First timed calls may follow earlier evaluation, and compiler caches may already be warm.

Rebuild charts with `uv run --script --locked ../plot.py`. macOS CPU and Apple GPU execution are verified; Linux is included in the environment lock but has not been tested here.
