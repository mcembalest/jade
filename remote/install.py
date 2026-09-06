"""Install the outbound Mac bridge without granting new folder permissions."""
from pathlib import Path
import json, os, plistlib, secrets, shutil, subprocess, sys, time
os.umask(0o077)
repo=Path(__file__).resolve().parent.parent
support=Path.home()/'Library/Application Support/JaDE'
folder=support/'remote';folder.mkdir(parents=True,exist_ok=True)
for name in ['bridge.py','manage.py']:shutil.copy2(repo/'remote'/name,folder/name)
config=support/'remote.json'
if not config.exists():
    sync=json.loads((Path.home()/'JaDE Mobile/.jade-sync/config.json').read_text())
    config.write_text(json.dumps({'endpoint':sync['endpoint'],'agentToken':secrets.token_urlsafe(48),'roots':[]},indent=2))
config.chmod(0o600)
# Provision the agent secret through stdin; it never appears in arguments/logs.
c=json.loads(config.read_text())
r=subprocess.run([str(repo/'sync/cloudflare/node_modules/.bin/wrangler'),'secret','put','REMOTE_AGENT_TOKEN'],input=c['agentToken'],text=True,cwd=repo/'sync/cloudflare',capture_output=True)
if r.returncode:raise SystemExit('Cloudflare agent-key setup failed. Authenticate Wrangler and retry.')
label='com.mcembalest.jade.remote';domain='gui/'+str(os.getuid())
agent=Path.home()/'Library/LaunchAgents'/f'{label}.plist'
agent.write_bytes(plistlib.dumps({'Label':label,'ProgramArguments':[sys.executable,str(folder/'bridge.py')],'RunAtLoad':True,'KeepAlive':True,'ThrottleInterval':15,'StandardOutPath':str(support/'remote.log'),'StandardErrorPath':str(support/'remote-error.log')}))
subprocess.run(['launchctl','bootout',domain+'/'+label],capture_output=True)
for _ in range(40):
    if subprocess.run(['launchctl','print',domain+'/'+label],capture_output=True).returncode:break
    time.sleep(.1)
subprocess.run(['launchctl','bootstrap',domain,str(agent)],check=True)
import shlex
launcher=support/'Choose JaDE Mac folders.command'
launcher.write_text('#!/bin/sh\n'+shlex.quote(sys.executable)+' '+shlex.quote(str(folder/'manage.py'))+'\n')
launcher.chmod(0o700)
print('Remote bridge installed. No additional folders have been enabled.')
