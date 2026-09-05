# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib==3.11.1", "numpy==2.5.2"]
# ///
"""Rebuild the MNIST figures from local runs and the shared dataset."""
import json
from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
import numpy as np

ROOT = Path(__file__).resolve().parent
INK, MUTED, BG = "#eaf3f8", "#91a6b6", "#101923"
TEAL, GOLD = "#50dfbd", "#ffca70"
plt.rcParams.update({"figure.facecolor": BG, "axes.facecolor": BG, "savefig.facecolor": BG,
                     "text.color": INK, "axes.labelcolor": MUTED, "xtick.color": MUTED,
                     "ytick.color": MUTED, "axes.edgecolor": "#304353", "font.family": "DejaVu Sans",
                     "font.size": 11, "svg.fonttype": "path", "svg.hashsalt": "jade-mnist"})


def finish(fig, path, subtitle, subtitle_y=.88):
    fig.text(.07, subtitle_y, subtitle, color=MUTED, fontsize=10)
    fig.savefig(ROOT / path, bbox_inches="tight", metadata={"Date": None})
    svg = ROOT / path
    svg.write_text("\n".join(line.rstrip() for line in svg.read_text().splitlines()) + "\n")
    plt.close(fig)


def clean(ax):
    ax.spines[["top", "right"]].set_visible(False)
    ax.grid(alpha=.12)
    ax.set_axisbelow(True)


def max_plot():
    runs = json.loads((ROOT / "mojo/measurements.json").read_text())
    run = runs["gpu"]
    row = run["inference"]
    batch128 = next(point for point in row if point["batch_size"] == 128)
    fig, axes = plt.subplots(1, 2, figsize=(10, 5.6))
    fig.subplots_adjust(top=.77, bottom=.2, left=.09, right=.97, wspace=.4)
    fig.suptitle("Compile once. Call it again.", x=.07, ha="left", fontsize=24, fontweight="bold")
    costs = [run["compile_seconds"]["infer_batch_128"], batch128["first_timed_call_ms"] / 1000,
             batch128["warm_median_ms"] / 1000]
    axes[0].bar(["Compile", "First call", "Warm call"], costs, color=[MUTED, GOLD, TEAL], width=.6)
    axes[0].set(yscale="log", ylabel="Seconds · logarithmic scale", title="GPU · 128 images per call")
    for index, cost in enumerate(costs):
        axes[0].text(index, cost * 1.4, f"{cost:.2f} s" if cost >= 1 else f"{cost*1000:.3f} ms",
                     ha="center", fontsize=9, color=INK)
    axes[0].set_ylim(min(costs) / 3, max(costs) * 6)
    for device, color in [("cpu", GOLD), ("gpu", TEAL)]:
        points = runs[device]["inference"]
        batches = np.array([point["batch_size"] for point in points])
        rate = batches / np.array([point["warm_median_ms"] for point in points]) * 1000
        axes[1].plot(batches, rate, "o-", color=color, lw=2.5, label=device.upper())
    axes[1].legend(frameon=False, labelcolor=INK)
    axes[1].set(xscale="log", yscale="log", xlabel="Images per call", ylabel="Images / second · warm median")
    axes[1].set_xticks([1, 8, 32, 128, 1000], ["1", "8", "32", "128", "1,000"])
    for ax in axes: clean(ax)
    finish(fig, "mojo/latency.svg", f"MAX {run['version']} + CUSTOM MOJO RELU  /  {run['hardware'].upper()}  /  CPU + METAL GPU")


def jax_plot():
    run = json.loads((ROOT / "jax-js/measurements.json").read_text())
    fig, axes = plt.subplots(1, 2, figsize=(10, 5.6))
    fig.subplots_adjust(top=.76, bottom=.18, left=.08, right=.97, wspace=.36)
    fig.suptitle("The browser learns. Then it answers.", x=.07, ha="left", fontsize=23, fontweight="bold")
    loss = np.array(run["loss"])
    steps = np.arange(1, len(loss) + 1)
    axes[0].plot(steps, loss, color=TEAL, alpha=.35, lw=1)
    window = 12
    axes[0].plot(steps[window-1:], np.convolve(loss, np.ones(window)/window, mode="valid"),
                 color=TEAL, lw=2.5, label="12-step moving mean")
    axes[0].set(xlabel="Optimizer step", ylabel="Training cross entropy", ylim=(0, None))
    axes[0].legend(frameon=False, labelcolor=INK, fontsize=9)
    row = run["inference"]
    batch = np.array([point["batch"] for point in row])
    axes[1].plot(batch, [point["warm_median_ms"] for point in row], "o-", color=GOLD, lw=2.5)
    axes[1].set(xscale="log", xlabel="Images per call", ylabel="Warm inference · ms", ylim=(0, None))
    axes[1].set_xticks([1, 8, 32, 128, 1000], ["1", "8", "32", "128", "1,000"])
    for ax in axes: clean(ax)
    finish(fig, "jax-js/learning.svg", f"JAX-JS / WEBGPU / APPLE GPU / {run['train']:,} DIGITS × {run['epochs']} EPOCHS / {run['training_seconds']:.2f} S TRAINING*\n*First-use compilation included; driver caches may be warm. Loading, shuffling and evaluation excluded.", subtitle_y=.85)


def main():
    max_plot()
    jax_plot()
    runs = {device: json.loads((ROOT / f"mlx/results/{device}/metrics.json").read_text())
            for device in ["cpu", "gpu"]}
    for run in runs.values():
        if run["training_images"] != 10000 or run["test_images"] != 1000 or len(run["epoch_seconds"]) != 3:
            raise ValueError("plots expect default 10k/1k, three-epoch runs; rerun MLX defaults")
    (ROOT / "measurements.json").write_text(json.dumps(runs, indent=2) + "\n")
    fig, ax = plt.subplots(figsize=(10, 5.6))
    fig.subplots_adjust(top=.78, bottom=.18, left=.1, right=.95)
    fig.suptitle("Tiny batches have a cost.", x=.07, ha="left", fontsize=24, fontweight="bold")
    for device, color in [("cpu", GOLD), ("gpu", TEAL)]:
        row = runs[device]["inference"]
        batch = np.array([m["batch_size"] for m in row])
        rate = batch / np.array([m["warm_median_ms"] for m in row]) * 1000
        ax.plot(batch, rate, "o-", lw=2.5, ms=6, color=color, label=f"MLX · {device.upper()}")
    ax.set(xscale="log", yscale="log", xlabel="Images per call", ylabel="Images / second · median-derived")
    ax.set_xticks([1, 8, 32, 128, 512, 1000], ["1", "8", "32", "128", "512", "1,000"])
    clean(ax); ax.legend(frameon=False, labelcolor=INK)
    finish(fig, "inference.svg", f"{runs['gpu']['hardware'].upper()}  /  MLP 784–128–10  /  50 WARM CALLS PER POINT  /  HOST + DEVICE TIME")

    fig, axes = plt.subplots(1, 2, figsize=(10, 5.3))
    fig.subplots_adjust(top=.77, bottom=.18, left=.08, right=.97, wspace=.38)
    fig.suptitle("Watch the model learn. Count the milliseconds.", x=.07, ha="left", fontsize=21, fontweight="bold")
    for device, color in [("cpu", GOLD), ("gpu", TEAL)]:
        row = runs[device]; epochs = np.arange(1, 4)
        axes[0].plot(np.cumsum(row["epoch_seconds"]), np.array(row["epoch_accuracy"])*100,
                     "o-", color=color, lw=2.5, label=device.upper())
        axes[1].plot(epochs, np.array(row["epoch_seconds"])*1000, "o-", color=color, lw=2.5)
    axes[0].set(xlabel="Cumulative training time · s", ylabel="Test accuracy · %")
    axes[1].set(xlabel="Epoch", ylabel="Training time · ms", xticks=[1, 2, 3])
    for ax in axes: clean(ax)
    axes[0].legend(frameon=False, labelcolor=INK)
    finish(fig, "mlx/learning.svg", "SAME ARCHITECTURE + SETTINGS  /  10,000 TRAINING IMAGES  /  TEST EVALUATION EXCLUDED FROM TIMING")

    model = json.loads((ROOT / "python-baseline/model.json").read_text())
    means = np.array(model["centroids"])
    fig, axes = plt.subplots(2, 5, figsize=(10, 5.8))
    fig.subplots_adjust(top=.79, bottom=.07, hspace=.36, wspace=.12)
    fig.suptitle("Ten memories. A whole classifier.", x=.07, ha="left", fontsize=24, fontweight="bold")
    for digit, ax in enumerate(axes.flat):
        ax.imshow(means[digit].reshape(28, 28), cmap="magma", vmin=0, vmax=255)
        ax.set_title(str(digit), fontsize=15, color=GOLD); ax.axis("off")
    finish(fig, "python-baseline/centroids.svg", "ONE AVERAGE IMAGE PER DIGIT  /  FIRST 10,000 TRAINING IMAGES  /  NO NEURAL NETWORK")

    distances = np.sqrt(((means[:, None] - means[None, :])**2).mean(axis=2))
    fig, ax = plt.subplots(figsize=(9, 7))
    fig.subplots_adjust(top=.8, bottom=.1, left=.12, right=.91)
    fig.suptitle("Which digits look alike to the model?", x=.07, ha="left", fontsize=22, fontweight="bold")
    art = ax.imshow(distances, cmap="magma")
    ax.set(xticks=range(10), yticks=range(10), xlabel="Digit prototype", ylabel="Digit prototype")
    for i in range(10):
        for j in range(10):
            ax.text(j, i, f"{distances[i,j]:.0f}", ha="center", va="center",
                    color=BG if distances[i,j] > distances.max()*.65 else INK, fontsize=9)
    fig.colorbar(art, ax=ax, label="RMS pixel distance · 0–255 intensities", shrink=.8)
    finish(fig, "mojo/distances.svg", "SHARED CENTROID MATH  /  COMPUTED FROM PYTHON REFERENCE  /  SMALLER = MORE SIMILAR")

    # PCA of a fixed prefix, with axis signs fixed for repeatable rendering.
    images = np.frombuffer((ROOT / "data/train-images-idx3-ubyte").read_bytes(), dtype=np.uint8, offset=16)
    images = images.reshape(-1, 784)[:4000].astype(np.float64) / 255
    labels = np.frombuffer((ROOT / "data/train-labels-idx1-ubyte").read_bytes(), dtype=np.uint8, offset=8)[:4000]
    images -= images.mean(axis=0)
    values, vectors = np.linalg.eigh(images.T @ images)
    basis = vectors[:, -2:][:, ::-1]
    for column in range(2):
        if basis[np.argmax(abs(basis[:, column])), column] < 0: basis[:, column] *= -1
    points = images @ basis
    fig, ax = plt.subplots(figsize=(10, 6.5))
    fig.subplots_adjust(top=.81, bottom=.15, left=.1, right=.86)
    fig.suptitle("4,000 digits. Two dimensions. A place to explore.", x=.07, ha="left", fontsize=21, fontweight="bold")
    colors = plt.get_cmap("tab10")
    for digit in range(10):
        mask = labels == digit
        ax.scatter(points[mask,0], points[mask,1], s=10, alpha=.5, color=colors(digit), label=str(digit), linewidths=0)
    variance = values[-2:][::-1] / values.sum() * 100
    ax.set(xlabel=f"Principal component 1 · {variance[0]:.1f}% variance",
           ylabel=f"Principal component 2 · {variance[1]:.1f}% variance")
    clean(ax); ax.legend(title="Digit", frameon=False, labelcolor=INK, bbox_to_anchor=(1.02, 1), loc="upper left")
    finish(fig, "jax-js/landscape.svg", "ACTUAL MNIST DATA  /  NUMPY PCA  /  A BROWSER EXPLORATION IDEA, NOT A JAX-JS BENCHMARK")


if __name__ == "__main__":
    main()
