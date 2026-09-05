# A small trainable MAX graph with a Mojo operation

![Compilation and warm execution have different costs](latency.svg)

## Question

Can we keep an operation in code we own while using MAX for the surrounding computation? This example places a custom Mojo ReLU between the two affine layers of a 784–128–10 MLP. Training and inference share that forward path. The experiment concerns latency, setup, and the amount of code required to connect the stack.

## Workload

Float32 MNIST pixels are scaled to [0, 1]. The first 10,000 training and 1,000 test images are used with three epochs, batch size 128, and seed 0. Parameters are initialized using NumPy's generator, with normal weights scaled by inverse square-root fan-in and zero biases. Each epoch shuffles the training prefix.

The training graph implements derivatives of mean softmax cross-entropy and Adam with learning rate 0.001, beta1 0.9, beta2 0.999, and epsilon 1e-8. These derivatives are explicit; this example does not provide an automatic differentiation library. [verify.py](verify.py) checks the computation independently. Saved NPZ files hold inference weights, not optimizer state for resuming training.

## Measurements

[measurements.json](measurements.json) records the local CPU run. The chart shows graph-loading/compilation time, the first timed call, and the warm median for a batch of 128, alongside warm throughput over several batch sizes. Compilation is real setup work and should remain visible when comparing developer experience.

Each shape gets its own graph. Graph loading/compilation is timed separately. Training includes shuffling, host batch construction, input transfers, optimizer execution, and device synchronization; it excludes graph compilation and per-epoch test evaluation. Inference uses resident input and weight buffers, synchronizes each call, and excludes copying its output back to NumPy. The first call and five warmups precede 50 measured repetitions. These are sequential calls, not server throughput; timing varies with the machine's other work.

The CPU forward path is compared with NumPy, and a separate process reloads the checkpoint. GPU execution has **not** been verified here: MAX detected the Apple GPU but compilation failed, and the machine has no Metal compiler. The CLI checks that prerequisite. A successful toolchain installation and numerical checks on the accelerator are still needed before claiming GPU support for this particular graph.

## Reproduce

Follow this folder's [introduction](jade.md). The checked-in Pixi lockfile covers macOS ARM64 and Linux x86-64; only macOS CPU execution was tested locally. From the parent MNIST folder, `uv run --script --locked plot.py` rebuilds the figures, including this chart from its checked-in measurements.

The old [mnist.mojo](mnist.mojo) remains an independent nearest-centroid reference. It agrees with Python on 773/1,000 labels in aggregate, but it uses a different model and contributes no MLP performance comparison.
