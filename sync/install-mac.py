"""Install the built Mac sync service for the current user's personal workspace."""
from pathlib import Path
import os
import plistlib
import subprocess
import sys
import time

repo = Path(__file__).resolve().parent.parent
workspace = Path(sys.argv[1]).expanduser().resolve() if len(sys.argv) > 1 else Path.home() / "JaDE Mobile"
if not (workspace / ".jade-sync" / "config.json").exists():
    raise SystemExit("Pairing configuration is missing; configure this workspace first.")
support = Path.home() / "Library" / "Application Support" / "JaDE"
support.mkdir(parents=True, exist_ok=True)
binary = support / "jade-sync"
subprocess.run(["go", "build", "-o", str(binary), "."], cwd=repo, check=True)
label = "com.mcembalest.jade.sync"
agent = Path.home() / "Library" / "LaunchAgents" / (label + ".plist")
agent.parent.mkdir(exist_ok=True)
config = {
    "Label": label,
    "ProgramArguments": [str(binary), "--no-open", "--address", "127.0.0.1:7339", str(workspace)],
    "RunAtLoad": True,
    "KeepAlive": True,
    "ThrottleInterval": 15,
    "StandardOutPath": str(support / "sync.log"),
    "StandardErrorPath": str(support / "sync-error.log"),
}
agent.write_bytes(plistlib.dumps(config))
domain = "gui/" + str(os.getuid())
subprocess.run(["launchctl", "bootout", domain + "/" + label], capture_output=True)
# bootout can return before launchd finishes removing the previous job.
for _ in range(40):
    if subprocess.run(["launchctl", "print", domain + "/" + label], capture_output=True).returncode != 0:
        break
    time.sleep(0.1)
subprocess.run(["launchctl", "bootstrap", domain, str(agent)], check=True)
launcher = support / "Open JaDE.command"
launcher.write_text('#!/bin/sh\nopen "http://127.0.0.1:7339"\n')
launcher.chmod(0o755)
print("JaDE sync installed for", workspace)
print("Open http://127.0.0.1:7339 on this Mac. Sync starts automatically at login.")
