#!/usr/bin/env python3

import json
import math
import random
import sys
from collections import Counter, defaultdict
from pathlib import Path


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: python3 bigram.py ../names.txt")

    names = [name.strip().lower() for name in Path(sys.argv[1]).read_text().splitlines() if name.strip()]
    alphabet = [".", *sorted({character for name in names for character in name})]
    counts: dict[str, Counter[str]] = defaultdict(Counter)
    pairs = 0
    loss = 0.0

    for name in names:
        characters = [".", *name, "."]
        for left, right in zip(characters, characters[1:]):
            counts[left][right] += 1
            pairs += 1

    probabilities: dict[str, dict[str, float]] = {}
    for left in alphabet:
        total = sum(counts[left].values()) + len(alphabet)
        probabilities[left] = {
            right: (counts[left][right] + 1) / total
            for right in alphabet
        }

    for name in names:
        characters = [".", *name, "."]
        for left, right in zip(characters, characters[1:]):
            loss -= math.log(probabilities[left][right])

    Path("model.json").write_text(json.dumps(probabilities, indent=2, sort_keys=True) + "\n")

    random.seed(2147483647)
    generated: list[str] = []
    for _ in range(20):
        name = ""
        character = "."
        while len(name) < 20:
            row = probabilities[character]
            character = random.choices(alphabet, weights=[row[next_character] for next_character in alphabet])[0]
            if character == ".":
                break
            name += character
        generated.append(name or "(empty)")

    report = [
        "BIGRAM MAKEMORE",
        f"training names: {len(names)}",
        f"character pairs: {pairs}",
        f"mean negative log likelihood: {loss / pairs:.4f}",
        "",
        *generated,
        "",
    ]
    Path("samples.txt").write_text("\n".join(report))


if __name__ == "__main__":
    main()
