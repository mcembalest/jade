"""Check MAX's Mojo activation, derivatives, and Adam against NumPy.

Run with: pixi run --locked python verify.py
Uses synthetic inputs, so the check needs no MNIST download or GPU.
"""
import numpy as np
from max.driver import Buffer, CPU
from max.engine import InferenceSession
from model import SHAPES, build, initialize, reference


def loss_and_grad(weights, x, target):
    w1, b1, w2, b2 = weights
    z = x @ w1 + b1
    hidden = np.maximum(z, 0)
    logits = hidden @ w2 + b2
    shifted = logits - logits.max(axis=1, keepdims=True)
    log_prob = shifted - np.log(np.exp(shifted).sum(axis=1, keepdims=True))
    loss = -np.sum(target * log_prob) / len(x)
    dy = (np.exp(log_prob) - target) / len(x)
    dz = (dy @ w2.T) * (z > 0)
    return loss, [x.T @ dz, dz.sum(axis=0), hidden.T @ dy, dy.sum(axis=0)]


def main():
    rng = np.random.default_rng(42)
    x = rng.normal(size=(7, 784)).astype(np.float32)
    target = np.eye(10, dtype=np.float32)[rng.integers(10, size=len(x))]
    weights = initialize(0)
    # First establish the independent NumPy derivatives by central differences.
    precise = [w.astype(np.float64) for w in weights]
    _, gradients = loss_and_grad(precise, x.astype(np.float64), target)
    for tensor, gradient in zip(precise, gradients):
        for index in rng.choice(tensor.size, size=min(12, tensor.size), replace=False):
            original = tensor.flat[index]
            tensor.flat[index] = original + 1e-5
            plus = loss_and_grad(precise, x.astype(np.float64), target)[0]
            tensor.flat[index] = original - 1e-5
            minus = loss_and_grad(precise, x.astype(np.float64), target)[0]
            tensor.flat[index] = original
            np.testing.assert_allclose((plus-minus)/2e-5, gradient.flat[index], atol=1e-8, rtol=1e-5)
    device = CPU()
    session = InferenceSession(devices=[device])
    infer = session.load(build(device, len(x)))
    train = session.load(build(device, len(x), training=True))
    buffer = lambda a: Buffer.from_numpy(np.ascontiguousarray(a, dtype=np.float32))
    host = lambda a: a.to_numpy().copy()
    actual = host(infer.execute(buffer(x), *map(buffer, weights))[0])
    np.testing.assert_allclose(actual, reference(weights, x), atol=2e-5, rtol=2e-5)
    # Nonzero optimizer states exercise decay and bias correction, not just step 1.
    moments = [rng.normal(0, .01, shape).astype(np.float32) for shape in SHAPES]
    variances = [rng.uniform(.001, .01, shape).astype(np.float32) for shape in SHAPES]
    for step in (7, 8):
        corrections = np.array([1-.9**step, 1-.999**step], np.float32)
        actual = list(map(host, train.execute(buffer(x), *map(buffer, weights), buffer(target),
                      buffer(corrections), *map(buffer, moments), *map(buffer, variances))))
        _, gradients = loss_and_grad(weights, x, target)
        moments = [.9*m + .1*g for m, g in zip(moments, gradients)]
        variances = [.999*v + .001*g*g for v, g in zip(variances, gradients)]
        weights = [w - .001*(m/corrections[0])/(np.sqrt(v/corrections[1])+1e-8)
                   for w, m, v in zip(weights, moments, variances)]
        for got, expected in zip(actual, weights + moments + variances):
            np.testing.assert_allclose(got, expected, atol=2e-6, rtol=2e-5)
    print("Passed: finite-difference gradients, Mojo/MAX forward pass, two Adam updates (all 12 tensors).")


if __name__ == "__main__":
    main()
