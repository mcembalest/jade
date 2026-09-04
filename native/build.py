#!/usr/bin/env -S uv run --script
from __future__ import annotations

import platform
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent
BUILD = ROOT / ".build"
ARCHITECTURE = platform.machine()
if ARCHITECTURE not in {"arm64", "x86_64"}:
    raise RuntimeError(f"unsupported macOS architecture: {ARCHITECTURE}")
TARGET = f"{ARCHITECTURE}-apple-macos13.0"


def run(*args: str) -> None:
    print("+", " ".join(args), flush=True)
    subprocess.run(args, cwd=REPO, check=True)


def compile_native() -> None:
    BUILD.mkdir(parents=True, exist_ok=True)
    app_sources = sorted((ROOT / "Sources/JaDENative").glob("*.swift"))
    run(
        "swiftc", "-target", TARGET, "-parse-as-library", *(str(path) for path in app_sources),
        "-framework", "AppKit", "-framework", "WebKit",
        "-o", str(BUILD / "JaDENative"),
    )


if __name__ == "__main__":
    try:
        compile_native()
    except subprocess.CalledProcessError as error:
        sys.exit(error.returncode)
