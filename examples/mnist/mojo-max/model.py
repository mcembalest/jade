"""A small MAX graph with a Mojo ReLU and explicit MLP/Adam derivatives."""
from pathlib import Path
import numpy as np
from max.dtype import DType
from max.graph import DeviceRef, Graph, TensorType, ops

SHAPES = [(784, 128), (128,), (128, 10), (10,)]


def initialize(seed):
    rng = np.random.default_rng(seed)
    return [rng.normal(0, 1 / np.sqrt(784), SHAPES[0]).astype(np.float32),
            np.zeros(128, np.float32),
            rng.normal(0, 1 / np.sqrt(128), SHAPES[2]).astype(np.float32),
            np.zeros(10, np.float32)]


def build(device, batch, training=False):
    ref = DeviceRef.from_device(device)
    tensor = lambda shape: TensorType(DType.float32, shape, device=ref)
    types = [tensor((batch, 784)), *[tensor(s) for s in SHAPES]]
    if training:
        types += [tensor((batch, 10)), tensor((2,)), *[tensor(s) for s in SHAPES] * 2]
    graph = Graph(f"mnist_{'train' if training else 'infer'}_{batch}", input_types=types,
                  custom_extensions=[Path(__file__).parent / "kernels"])
    with graph:
        values = [v.tensor for v in graph.inputs]
        x, w1, b1, w2, b2 = values[:5]
        z = x @ w1 + b1
        h = ops.custom("mnist_relu", device=ref, values=[z], out_types=[z.type])[0].tensor
        logits = h @ w2 + b2
        if not training:
            graph.output(logits)
            return graph
        target, corrections = values[5:7]
        # Mean softmax cross-entropy derivative, followed by the chain rule.
        dy = (ops.softmax(logits) - target) / batch
        dh = (dy @ ops.transpose(w2, 0, 1)) * ops.cast(ops.greater(z, 0), DType.float32)
        grads = [ops.transpose(x, 0, 1) @ dh, ops.sum(dh, 0).reshape((128,)),
                 ops.transpose(h, 0, 1) @ dy, ops.sum(dy, 0).reshape((10,))]
        moments = [0.9 * m + 0.1 * g for m, g in zip(values[7:11], grads)]
        variances = [0.999 * v + 0.001 * g * g for v, g in zip(values[11:15], grads)]
        weights = [w - .001 * (m / corrections[0]) / (ops.sqrt(v / corrections[1]) + 1e-8)
                   for w, m, v in zip(values[1:5], moments, variances)]
        graph.output(*weights, *moments, *variances)
    return graph


def reference(weights, x):
    return np.maximum(x @ weights[0] + weights[1], 0) @ weights[2] + weights[3]
