"""One-time, lossless Sanjana migration. Install the new desktop service first.

Reads credentials privately; preserves both the original chat and a durable migration
payload. Retries reuse the same payload identity and never overwrite cloud history.
"""
from pathlib import Path
import datetime,fcntl,hashlib,json,os,urllib.request,urllib.error
os.umask(0o077)
root=Path(__file__).resolve().parent.parent
folder=Path.home()/'Library/Application Support/JaDE/companion'
folder.mkdir(parents=True,exist_ok=True)
# Compatible with the previous Go file locks: do not race a finishing local chat.
locks=[]
for name in ['.running','.lock']:
 handle=(folder/name).open('a+');fcntl.flock(handle,fcntl.LOCK_EX|fcntl.LOCK_NB);locks.append(handle)
config=json.loads((folder.parent/'remote.json').read_text())
payload_path=folder/'cloud-migration.json'
if not payload_path.exists():
 original=(folder/'chat.json').read_bytes()
 state=json.loads(original)
 backup=folder/('chat.pre-cloud-'+datetime.datetime.now().strftime('%Y%m%d-%H%M%S')+'.json')
 with backup.open('xb') as f:f.write(original);f.flush();os.fsync(f.fileno())
 profile=(root/'engine/web/companion/character.md').read_text()
 payload={'action':'migrate','migration':hashlib.sha256(original+profile.encode()).hexdigest(),'state':state,'profile':profile}
 with payload_path.open('xb') as f:f.write(json.dumps(payload).encode());f.flush();os.fsync(f.fileno())
payload=json.loads(payload_path.read_text())
request=urllib.request.Request(config['endpoint'].rstrip('/')+'/v1/companion',data=json.dumps(payload).encode(),headers={'Authorization':'Bearer '+config['agentToken'],'Content-Type':'application/json','User-Agent':'JaDE/0.4'})
try:
 with urllib.request.urlopen(request,timeout=30) as r:cloud=json.load(r)
except urllib.error.HTTPError as e:
 raise SystemExit('Migration not accepted (HTTP '+str(e.code)+'). Existing cloud and local histories were preserved.')
# On an initial migration all these values must be present exactly. Later retries
# can observe subsequent cloud updates, so only the first import verifies equality.
marker=folder/'cloud-migrated.json'
if not marker.exists():
 assert cloud['messages']==payload['state']['messages'],'History mismatch'
 assert cloud['pending']==(payload['state'].get('pending') or []),'Pending mismatch'
 assert cloud['profile']==payload['profile'],'Profile mismatch'
 marker.write_text(json.dumps({'migration':payload['migration'],'verifiedAt':datetime.datetime.now(datetime.timezone.utc).isoformat()}))
print('Cloud migration verified. Preserved messages:',len(payload['state']['messages']),'pending:',len(payload['state'].get('pending') or []))
print('Original history and private migration backup retained in:',folder)
