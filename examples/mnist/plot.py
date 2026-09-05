# /// script
# requires-python = ">=3.12"
# dependencies = ["matplotlib==3.11.1", "numpy==2.5.2"]
# ///
"""Rebuild charts from measurements.json; --collect first imports local benchmark runs."""

import argparse
import json
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import Patch
import numpy as np

ROOT = Path(__file__).resolve().parent
INK, MUTED, BG = "#eaf3f8", "#91a6b6", "#101923"
TEAL, GOLD = "#50dfbd", "#ffca70"
KEYS = [
    "numpy-cpu",
    "torch-cpu",
    "torch-gpu",
    "mlx-cpu",
    "mlx-gpu",
    "jax-js-wasm",
    "jax-js-webgpu",
    "max-cpu",
    "max-gpu",
]
LABELS = [
    "Python / NumPy · CPU",
    "PyTorch · CPU",
    "PyTorch · MPS",
    "MLX · CPU",
    "MLX · Metal",
    "jax-js · WASM",
    "jax-js · WebGPU",
    "Mojo / MAX · CPU",
    "Mojo / MAX · Metal",
]
plt.rcParams.update(
    {
        "figure.facecolor": BG,
        "axes.facecolor": BG,
        "savefig.facecolor": BG,
        "text.color": INK,
        "axes.labelcolor": MUTED,
        "xtick.color": MUTED,
        "ytick.color": MUTED,
        "axes.edgecolor": "#304353",
        "font.family": "DejaVu Sans",
        "font.size": 10,
        "svg.fonttype": "path",
        "svg.hashsalt": "jade-mnist",
    }
)


def collect():
    runs = [json.loads((ROOT / "results" / f"{key}.json").read_text()) for key in KEYS]
    if len({r["fixture_sha256"] for r in runs}) != 1:
        raise ValueError(
            "Runs must use the same dataset, initial weights and sample order"
        )
    for key, run in zip(KEYS, runs):
        if (
            f"{run['backend']}-{run['device']}" != key
            or len(run["training_seconds"]) != 3
        ):
            raise ValueError(f"Expected three trials: {key}")
        if np.shape(run["epoch_seconds"]) != (3, 3) or min(run["accuracy"]) < 0.85:
            raise ValueError(f"Training sanity check failed: {key}")
        if [r["batch"] for r in run["inference"]] != [1, 128, 1000]:
            raise ValueError(f"Missing inference sizes: {key}")
        for row in run["inference"]:
            samples = row["samples_ms"]
            if (
                len(samples) != 50
                or not np.all(np.isfinite(samples))
                or min(samples) <= 0
            ):
                raise ValueError(f"Invalid inference timings: {key}")
    fixture = json.loads((ROOT / "data/comparison.json").read_text())
    output = {
        "protocol": {
            "model": "784-128-10 ReLU, float32",
            "training_images": 10000,
            "test_images": 1000,
            "epochs": 3,
            "batch": 128,
            "tail_batch": 16,
            "optimizer": "Adam",
            "learning_rate": 0.001,
            "betas": [0.9, 0.999],
            "epsilon": 1e-8,
            "bias_correction": True,
            "initialization_seed": 0,
            "shuffle_seed": 1,
            "trials": 3,
            "training": "Host batches prepared before timing. Includes per-batch upload, gradients, Adam and synchronization. Excludes setup, warmup, reset and evaluation.",
            "inference": "Identical initial (untrained) weights; resident inputs. Fresh output synchronized each call. Upload/readback excluded. Five warmups, 50 samples, median and p95.",
            "warmup": "One discarded update at each training shape before trials. First-use measurements are not process/driver-cold. MAX graphs compile separately.",
            "cpu_threads": "NumPy and PyTorch: 1; MLX, MAX and browser: runtime defaults.",
            "dataset_sha256": fixture["dataset_sha256"],
        },
        "runs": dict(zip(KEYS, runs)),
    }
    (ROOT / "measurements.json").write_text(json.dumps(output, indent=2) + "\n")


def tables(runs):
    def write(path, lines):
        doc = ROOT / path
        before, rest = doc.read_text().split("<!-- measurements -->")
        _, after = rest.split("<!-- /measurements -->")
        doc.write_text(
            before
            + "<!-- measurements -->\n\n"
            + "\n".join(lines)
            + "\n\n<!-- /measurements -->"
            + after
        )

    lines = [
        "| Stack / device | Train 3 epochs · ms | Infer 1 · µs | Infer 128 · µs | Infer 1,000 · µs |",
        "| --- | ---: | ---: | ---: | ---: |",
    ]
    for key, label in zip(KEYS, LABELS):
        run = runs[key]
        values = [np.median(run["training_seconds"]) * 1000] + [
            p["median_ms"] * 1000 for p in run["inference"]
        ]
        lines.append(
            "| " + label + " | " + " | ".join(f"{v:,.2f}" for v in values) + " |"
        )
    write("README.md", lines)
    lines = ["| Graph | CPU · s | Metal · s |", "| --- | ---: | ---: |"]
    for key in runs["max-cpu"]["compile_seconds"]:
        size, training = key.split(":")
        label = ("Train " if training == "True" else "Infer ") + f"{int(size):,}"
        values = [runs[f"max-{d}"]["compile_seconds"][key] for d in ["cpu", "gpu"]]
        lines.append(f"| {label} | {values[0]:.3f} | {values[1]:.3f} |")
    write("mojo-max/README.md", lines)


def clean(ax):
    ax.spines[["top", "right"]].set_visible(False)
    ax.grid(alpha=0.12)
    ax.set_axisbelow(True)


def finish(fig, path):
    fig.savefig(ROOT / path, bbox_inches="tight", metadata={"Date": None})
    svg = ROOT / path
    svg.write_text(
        "\n".join(line.rstrip() for line in svg.read_text().splitlines()) + "\n"
    )
    if args.preview:
        args.preview.mkdir(parents=True, exist_ok=True)
        fig.savefig(
            args.preview / (path.replace("/", "-") + ".png"),
            dpi=130,
            bbox_inches="tight",
        )
    plt.close(fig)


def overview(runs):
    fig, axes = plt.subplots(1, 2, figsize=(12, 5.2))
    fig.subplots_adjust(top=0.94, bottom=0.19, left=0.23, right=0.95, wspace=0.18)
    colors = [TEAL if runs[k]["device"] in ("gpu", "webgpu") else GOLD for k in KEYS]
    for ax, field in zip(axes, ["train", "infer"]):
        values = [
            np.median(runs[k]["training_seconds"]) * 1000
            if field == "train"
            else runs[k]["inference"][0]["median_ms"] * 1000
            for k in KEYS
        ]
        baseline = min(values) / 2
        ax.barh(
            range(len(KEYS)),
            np.array(values) - baseline,
            left=baseline,
            color=colors,
            height=0.6,
        )
        ax.set_yticks(
            range(len(KEYS)), LABELS if field == "train" else [""] * len(KEYS)
        )
        ax.invert_yaxis()
        ax.set_xscale("log")
        ax.set_xlim(min(values) / 2, max(values) * 5)
        ax.set_title(
            "Training · 3 epochs"
            if field == "train"
            else "Inference · 1 image",
            pad=14,
        )
        ax.set_xlabel(
            "Median training time (ms)"
            if field == "train"
            else "Median inference time (µs)"
        )
        for i, value in enumerate(values):
            ax.text(
                value * 1.12,
                i,
                f"{value:,.2f}" if value < 10 else f"{value:,.1f}",
                va="center",
                fontsize=9,
            )
        clean(ax)
    fig.legend(
        handles=[Patch(color=GOLD, label="CPU / WASM"), Patch(color=TEAL, label="GPU")],
        loc="lower center", ncol=2, frameon=False,
    )
    finish(fig, "inference.svg")


def detail(runs, backend, path):
    selected = [runs[k] for k in KEYS if runs[k]["backend"] == backend]
    fig, axes = plt.subplots(1, 2, figsize=(10, 4.2))
    fig.subplots_adjust(top=0.94, bottom=0.16, left=0.09, right=0.95, wspace=0.35)
    for run, color in zip(selected, [GOLD, TEAL]):
        times = np.array(run["epoch_seconds"]) * 1000
        axes[0].plot(
            [1, 2, 3],
            np.median(times, axis=0),
            "o-",
            color=color,
            lw=2,
            label=LABELS[KEYS.index(f"{backend}-{run['device']}")].split(" · ")[-1],
        )
        axes[0].fill_between(
            [1, 2, 3], times.min(axis=0), times.max(axis=0), color=color, alpha=0.2
        )
        points = run["inference"]
        batches = np.array([p["batch"] for p in points])
        medians = np.array([p["median_ms"] for p in points])
        axes[1].plot(
            batches,
            batches / medians * 1000,
            "o-",
            color=color,
            lw=2,
            label=LABELS[KEYS.index(f"{backend}-{run['device']}")].split(" · ")[-1],
        )
    axes[0].set(
        xlabel="Epoch",
        ylabel="Training time (ms)",
        xticks=[1, 2, 3],
        ylim=(0, None),
    )
    axes[1].set(
        xscale="log",
        yscale="log",
        xlabel="Images per inference call",
        ylabel="Inference throughput (images/s)",
    )
    axes[1].set_xticks([1, 128, 1000], ["1", "128", "1,000"])
    axes[0].legend(frameon=False, labelcolor=INK)
    for ax in axes:
        clean(ax)
    finish(fig, path)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--collect", action="store_true")
    parser.add_argument("--preview", type=Path)
    args = parser.parse_args()
    if args.collect:
        collect()
    runs = json.loads((ROOT / "measurements.json").read_text())["runs"]
    if args.collect:
        tables(runs)
    overview(runs)
    detail(runs, "mlx", "mlx/learning.svg")
    detail(runs, "max", "mojo-max/latency.svg")
    detail(runs, "jax-js", "jax-js/learning.svg")
