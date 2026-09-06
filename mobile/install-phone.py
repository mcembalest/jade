"""Build, renew and install JaDE on one explicitly selected personal iPhone.

Reinstall over the same bundle to preserve local notes; never uninstall to renew.
"""
from pathlib import Path
import argparse
import json
import os
import subprocess
import urllib.parse

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--team", required=True, help="Apple signing team ID")
parser.add_argument("--device", required=True, help="Connected iPhone UDID")
parser.add_argument("--pair", action="store_true", help="Pair using this Mac's JaDE Mobile configuration")
args = parser.parse_args()
os.umask(0o077)
repo = Path(__file__).resolve().parent.parent
output = repo / ".tmp" / "ios-phone"
output.mkdir(parents=True, exist_ok=True)
log = output / "install.log"

def run(command, *, private=False):
    with log.open("a") as stream:
        result = subprocess.run(command, cwd=repo, stdout=stream, stderr=subprocess.STDOUT)
    log.chmod(0o600)
    if result.returncode:
        if private:
            detail = log.read_text()
            if "profile has not been explicitly trusted" in detail:
                raise SystemExit("JaDE is installed. On the iPhone, open Settings → General → VPN & Device Management, select your developer account, and Trust it. Then open JaDE and scan the private pairing QR, or rerun with --pair.")
            raise SystemExit("JaDE is installed, but pairing needs attention. Open JaDE on your phone and scan the private pairing QR on your Mac.")
        raise SystemExit(f"Apple could not complete this step. Keep the phone connected, unlocked, and in Developer Mode. Details: {log}")

print("Building and renewing JaDE with your Apple signing team…", flush=True)
run(["xcodebuild", "-project", "mobile/JaDE.xcodeproj", "-scheme", "JaDE",
     "-configuration", "Release", "-destination", "platform=iOS,id=" + args.device,
     "-derivedDataPath", str(output), "DEVELOPMENT_TEAM=" + args.team,
     "-allowProvisioningUpdates", "-allowProvisioningDeviceRegistration", "build"])
print("Installing over the existing app; local notes are retained…", flush=True)
run(["xcrun", "devicectl", "device", "install", "app", "--device", args.device,
     str(output / "Build/Products/Release-iphoneos/JaDE.app")])
command = ["xcrun", "devicectl", "device", "process", "launch", "--device", args.device]
if args.pair:
    config = json.loads((Path.home() / "JaDE Mobile/.jade-sync/config.json").read_text())
    command += ["--payload-url", "jade://pair?" + urllib.parse.urlencode(config)]
command += ["com.mcembalest.jade.mobile"]
run(command, private=args.pair)
print("JaDE installed and launched. Wait for the sync status on your phone.")
