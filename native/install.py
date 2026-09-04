#!/usr/bin/env -S uv run --script
from __future__ import annotations

import os
import plistlib
import shutil
import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parent
BUILD = ROOT / ".build"
APP = Path.home() / "Applications" / "JaDE.app"
CONTENTS = APP / "Contents"
MACOS = CONTENTS / "MacOS"
RESOURCES = CONTENTS / "Resources"


def run(*args: str) -> None:
    print("+", " ".join(args), flush=True)
    subprocess.run(args, cwd=REPO, check=True)


def install() -> None:
    run("uv", "run", str(ROOT / "build.py"))
    run("go", "build", "-o", str(BUILD / "jade-engine"), "./cmd/jade-engine")

    if APP.exists():
        shutil.rmtree(APP)
    MACOS.mkdir(parents=True)
    RESOURCES.mkdir(parents=True)
    shutil.copy2(BUILD / "JaDENative", MACOS / "JaDE")
    shutil.copy2(BUILD / "jade-engine", RESOURCES / "jade-engine")
    shutil.copytree(BUILD / "GhosttyTerminal.bundle", RESOURCES / "GhosttyTerminal.bundle")
    os.chmod(MACOS / "JaDE", 0o755)
    os.chmod(RESOURCES / "jade-engine", 0o755)

    info = {
        "CFBundleDevelopmentRegion": "en",
        "CFBundleDisplayName": "JaDE",
        "CFBundleExecutable": "JaDE",
        "CFBundleIdentifier": "com.cerebellica.jade",
        "CFBundleInfoDictionaryVersion": "6.0",
        "CFBundleName": "JaDE",
        "CFBundlePackageType": "APPL",
        "CFBundleShortVersionString": "0.1.0",
        "CFBundleVersion": "1",
        "LSMinimumSystemVersion": "13.0",
        "LSUIElement": True,
        "NSHighResolutionCapable": True,
    }
    with (CONTENTS / "Info.plist").open("wb") as output:
        plistlib.dump(info, output)

    run("codesign", "--force", "--deep", "--sign", "-", str(APP))
    run("go", "install", "./cmd/jade")
    print(f"Installed {APP}")


if __name__ == "__main__":
    install()
