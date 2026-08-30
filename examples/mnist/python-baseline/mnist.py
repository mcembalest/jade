#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.10"
# ///

import base64
import json
import os
import struct
import sys
import zlib
from pathlib import Path


def images(path: Path) -> tuple[int, int, memoryview]:
    data = path.read_bytes()
    magic, count, rows, columns = struct.unpack_from(">IIII", data)
    if magic != 2051 or len(data) != 16 + count * rows * columns:
        raise ValueError(f"invalid MNIST image file: {path}")
    return count, rows * columns, memoryview(data)[16:]


def labels(path: Path) -> memoryview:
    data = path.read_bytes()
    magic, count = struct.unpack_from(">II", data)
    if magic != 2049 or len(data) != 8 + count:
        raise ValueError(f"invalid MNIST label file: {path}")
    return memoryview(data)[8:]


def png_data_uri(rows: list[list[int]]) -> str:
    def chunk(tag: bytes, payload: bytes) -> bytes:
        return struct.pack(">I", len(payload)) + tag + payload + struct.pack(">I", zlib.crc32(tag + payload))

    header = struct.pack(">IIBBBBB", len(rows[0]), len(rows), 8, 0, 0, 0, 0)
    raw = b"".join(b"\x00" + bytes(row) for row in rows)
    png = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", header) + chunk(b"IDAT", zlib.compress(raw, 9)) + chunk(b"IEND", b"")
    return "data:image/png;base64," + base64.b64encode(png).decode()


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: uv run mnist.py ..")
    data = Path(sys.argv[1]) / "data"
    train_count, pixels, train_images = images(data / "train-images-idx3-ubyte")
    train_labels = labels(data / "train-labels-idx1-ubyte")
    test_count, test_pixels, test_images = images(data / "t10k-images-idx3-ubyte")
    test_labels = labels(data / "t10k-labels-idx1-ubyte")
    if pixels != test_pixels:
        raise ValueError("training and test image shapes differ")

    train_limit = min(train_count, int(os.getenv("MNIST_TRAIN", "10000")))
    test_limit = min(test_count, int(os.getenv("MNIST_TEST", "1000")))
    sums = [[0] * pixels for _ in range(10)]
    counts = [0] * 10
    for index in range(train_limit):
        label = train_labels[index]
        counts[label] += 1
        offset = index * pixels
        row = sums[label]
        for pixel in range(pixels):
            row[pixel] += train_images[offset + pixel]

    centroids = [[value / counts[digit] for value in sums[digit]] for digit in range(10)]
    correct = 0
    for index in range(test_limit):
        offset = index * pixels
        best_digit = 0
        best_distance = float("inf")
        for digit, centroid in enumerate(centroids):
            distance = 0.0
            for pixel in range(pixels):
                difference = test_images[offset + pixel] - centroid[pixel]
                distance += difference * difference
            if distance < best_distance:
                best_digit, best_distance = digit, distance
        correct += best_digit == test_labels[index]

    Path("model.json").write_text(json.dumps({"centroids": centroids}) + "\n")
    Path("metrics.json").write_text(json.dumps({
        "implementation": "python-standard-library",
        "model": "nearest-centroid",
        "training_images": train_limit,
        "test_images": test_limit,
        "correct": correct,
        "accuracy": correct / test_limit,
    }, indent=2) + "\n")

    side = int(pixels ** 0.5)
    scale = 4
    grid: list[list[int]] = []
    for y in range(side):
        row: list[int] = []
        for digit in range(10):
            for x in range(side):
                row.extend([255 - min(255, round(centroids[digit][y * side + x]))] * scale)
        grid.extend([row] * scale)

    Path("report.md").write_text("\n".join([
        "# Python baseline — nearest centroid",
        "",
        "| training images | test images | correct | accuracy |",
        "|--:|--:|--:|--:|",
        f"| {train_limit:,} | {test_limit:,} | {correct:,} | **{correct / test_limit:.3f}** |",
        "",
        "The whole model — one average image per digit:",
        "",
        f"![digit centroids]({png_data_uri(grid)})",
        "",
    ]))


if __name__ == "__main__":
    main()
