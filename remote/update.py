"""Upgrade an existing Mac connection after building and backing it up. No new folder grants."""
from pathlib import Path
import datetime,json,os,plistlib,shutil,signal,subprocess,sys,tempfile,time
os.umask(0o077)
repo=Path(__file__).resolve().parent.parent
support=Path.home()/'Library/Application Support/JaDE'
app=Path.home()/'Applications/JaDE Mac Connection.app'
config=support/'remote.json'
if not config.exists() or not app.exists():raise SystemExit('Use the initial connection installers first.')
c=json.loads(config.read_text());assert 'agentToken' in c and 'roots' in c
stamp=datetime.datetime.now().strftime('%Y%m%d-%H%M%S')
backup=support/'upgrades'/stamp;backup.mkdir(parents=True)
shutil.copytree(app,backup/app.name)
shutil.copytree(support/'remote',backup/'remote')
shutil.copy2(config,backup/'remote.json')
with tempfile.TemporaryDirectory(prefix='jade-connection-update-') as temp:
 staged=Path(temp)/app.name;shutil.copytree(app,staged)
 executable=staged/'Contents/MacOS/JaDEMacConnection'
 subprocess.run(['swiftc',str(repo/'remote/MacConnection.swift'),'-o',str(executable)],check=True)
 info=staged/'Contents/Info.plist';settings=plistlib.loads(info.read_bytes());settings['JaDEPython']=sys.executable;settings['CFBundleVersion']='2';info.write_bytes(plistlib.dumps(settings))
 subprocess.run(['codesign','--force','--sign','-',str(staged)],check=True,capture_output=True)
 subprocess.run(['codesign','--verify','--deep','--strict',str(staged)],check=True,capture_output=True)
 next_app=app.with_name('JaDE Mac Connection.next.app')
 old=app.with_name('JaDE Mac Connection.previous.app')
 if next_app.exists() or old.exists():raise SystemExit('A staged app exists; inspect it before retrying.')
 shutil.copytree(staged,next_app)
 # Stop only this installed connection and its exact bridge script. Notes service is separate.
 lines=subprocess.check_output(['ps','-axo','pid=,command='],text=True).splitlines()
 targets=[]
 for line in lines:
  pid,command=line.strip().split(None,1)
  if command.startswith(str(app/'Contents/MacOS/JaDEMacConnection')) or command.endswith(' '+str(support/'remote/bridge.py')):
   targets.append(int(pid))
 for pid in targets:
  try:os.kill(pid,signal.SIGTERM)
  except ProcessLookupError:pass
 for _ in range(30):
  live=[]
  for pid in targets:
   try:os.kill(pid,0);live.append(pid)
   except ProcessLookupError:pass
  if not live:break
  time.sleep(.1)
 else:raise SystemExit('Connection is still running; source backup retained. Close it before retrying.')
 for name in ['bridge.py','manage.py','cloud.py']:
  dest=support/'remote'/name;tmp=dest.with_suffix('.update');shutil.copy2(repo/'remote'/name,tmp);tmp.replace(dest)
 app.rename(old)
 try:next_app.rename(app)
 except Exception:old.rename(app);raise
 shutil.rmtree(old)
 subprocess.run(['open','-g',str(app)],check=True)
print('Mac connection updated; existing folder permissions retained. Cloud storage remains opt-in.')
print('Previous app and scripts backed up at',backup)
