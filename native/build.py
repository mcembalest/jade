#!/usr/bin/env -S uv run --script
from __future__ import annotations

import platform
import shutil
import subprocess
import sys
import urllib.request
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent
DEPS = ROOT / ".deps"
BUILD = ROOT / ".build" / "manual"
GHOSTTY_REVISION = "dacbb0b8bf8ca96a988afdcc6a7f359c05cd015b"
DISPLAY_LINK_TAG = "2.2.0"
GHOSTTY_ARCHIVE = "https://github.com/Lakr233/libghostty-spm/releases/download/upstream.1.3.1/GhosttyKit.xcframework.zip"
ARCHITECTURE = platform.machine()
if ARCHITECTURE not in {"arm64", "x86_64"}:
    raise RuntimeError(f"unsupported macOS architecture: {ARCHITECTURE}")
TARGET = f"{ARCHITECTURE}-apple-macos13.0"


def run(*args: str) -> None:
    print("+", " ".join(args), flush=True)
    subprocess.run(args, cwd=REPO, check=True)


def dependencies() -> None:
    DEPS.mkdir(parents=True, exist_ok=True)
    ghostty = DEPS / "libghostty-spm"
    if not ghostty.exists():
        run("git", "clone", "--quiet", "https://github.com/Lakr233/libghostty-spm.git", str(ghostty))
        run("git", "-C", str(ghostty), "checkout", "--quiet", GHOSTTY_REVISION)
    display = DEPS / "MSDisplayLink"
    if not display.exists():
        run("git", "clone", "--quiet", "--depth", "1", "--branch", DISPLAY_LINK_TAG, "https://github.com/Lakr233/MSDisplayLink.git", str(display))
    framework = DEPS / "GhosttyKit.xcframework"
    if not framework.exists():
        archive = DEPS / "GhosttyKit.xcframework.zip"
        urllib.request.urlretrieve(GHOSTTY_ARCHIVE, archive)
        with zipfile.ZipFile(archive) as bundle:
            bundle.extractall(DEPS)
        archive.unlink()


def compile_native() -> None:
    BUILD.mkdir(parents=True, exist_ok=True)
    headers = DEPS / "GhosttyKit.xcframework" / "macos-arm64_x86_64" / "Headers"
    ghostty_library = DEPS / "GhosttyKit.xcframework" / "macos-arm64_x86_64" / "libghostty.a"

    run(
        "clang", "-target", TARGET, "-c", str(ROOT / "Sources/CPTY/CPTY.c"),
        "-I" + str(ROOT / "Sources/CPTY/include"), "-o", str(BUILD / "CPTY.o"),
    )
    run(
        "swiftc", "-target", TARGET, "-emit-module", "-parse-as-library", "-module-name", "GhosttyKit",
        str(DEPS / "libghostty-spm/Sources/GhosttyKit/GhosttyKit.swift"),
        "-I" + str(headers), "-emit-module-path", str(BUILD / "GhosttyKit.swiftmodule"),
    )

    display_sources = sorted(
        path for path in (DEPS / "MSDisplayLink/Sources/MSDisplayLink").glob("*.swift")
        if path.name != "DisplayLink+SwiftUI.swift"
    )
    run(
        "swiftc", "-target", TARGET, "-emit-library", "-static", "-emit-module", "-parse-as-library",
        "-module-name", "MSDisplayLink", *(str(path) for path in display_sources),
        "-emit-module-path", str(BUILD / "MSDisplayLink.swiftmodule"),
        "-o", str(BUILD / "libMSDisplayLink.a"),
    )

    terminal_sources = sorted(
        path for path in (DEPS / "libghostty-spm/Sources/GhosttyTerminal").rglob("*.swift")
        if "Platform/UIKit" not in str(path)
        and not path.name.startswith("TerminalViewRepresentable")
        and path.name != "TerminalSurfaceView.swift"
    )
    terminal_sources.extend(sorted((ROOT / "Support").glob("*.swift")))
    run(
        "swiftc", "-target", TARGET, "-emit-library", "-static", "-emit-module", "-parse-as-library",
        "-module-name", "GhosttyTerminal", *(str(path) for path in terminal_sources),
        "-I" + str(BUILD), "-I" + str(headers), "-L" + str(BUILD), "-lMSDisplayLink",
        "-emit-module-path", str(BUILD / "GhosttyTerminal.swiftmodule"),
        "-o", str(BUILD / "libGhosttyTerminal.a"),
    )

    app_sources = sorted((ROOT / "Sources/JaDENative").glob("*.swift"))
    run(
        "swiftc", "-target", TARGET, "-parse-as-library", *(str(path) for path in app_sources), str(BUILD / "CPTY.o"),
        "-I" + str(ROOT / "Sources/CPTY/include"), "-I" + str(BUILD), "-I" + str(headers),
        "-L" + str(BUILD), "-lGhosttyTerminal", "-lMSDisplayLink", str(ghostty_library),
        "-lc++", "-framework", "AppKit", "-framework", "Carbon", "-framework", "CoreGraphics",
        "-framework", "CoreText", "-framework", "CoreVideo", "-framework", "IOSurface",
        "-framework", "Metal", "-framework", "QuartzCore", "-framework", "SwiftUI",
        "-framework", "UniformTypeIdentifiers", "-framework", "WebKit",
        "-o", str(ROOT / ".build/JaDENative"),
    )

    resources = ROOT / ".build/GhosttyTerminal.bundle"
    if resources.exists():
        shutil.rmtree(resources)
    resources.mkdir(parents=True)
    source_resources = DEPS / "libghostty-spm/Sources/GhosttyTerminal/Resources"
    shutil.copytree(source_resources / "Ghostty", resources / "Ghostty")
    shutil.copytree(source_resources / "terminfo", resources / "terminfo")


if __name__ == "__main__":
    try:
        dependencies()
        compile_native()
    except subprocess.CalledProcessError as error:
        sys.exit(error.returncode)
