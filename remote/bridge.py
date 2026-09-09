"""Outbound-only personal Mac file bridge. Folder permissions are local to this Mac."""
from pathlib import Path
import hashlib, json, os, stat, tempfile, time, urllib.request, urllib.error, threading
from concurrent.futures import ThreadPoolExecutor

FILE_LOCK = threading.RLock()

SUPPORT = Path.home() / 'Library/Application Support/JaDE'
CONFIG = SUPPORT / 'remote.json'
MAX = 256 * 1024
EXCLUDED = {'node_modules', 'vendor', 'build', 'dist', '__pycache__', 'DerivedData'}

def digest(data): return hashlib.sha256(data).hexdigest()
def config(): return json.loads(CONFIG.read_text())
def parts(path):
    if not isinstance(path,str) or path.startswith('/') or '\\' in path: raise ValueError('Invalid path')
    result=path.split('/') if path else []
    if any(not p or p.startswith('.') or p in EXCLUDED for p in result): raise ValueError('Hidden, generated and parent paths are excluded')
    return result

def root_for(c, identifier):
    item=next((r for r in c['roots'] if r['id']==identifier),None)
    if not item: raise ValueError('This folder is not enabled on your Mac')
    return item

def open_dir(root, components):
    # Walk using descriptor-relative O_NOFOLLOW opens, including the root. A
    # symlink swapped in during a request cannot redirect access elsewhere.
    fd=os.open(root,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:
        for part in components:
            child=os.open(part,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            os.close(fd);fd=child
        return fd
    except:
        os.close(fd);raise

def read_at(fd,name):
    f=os.open(name,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=fd)
    with os.fdopen(f,'rb') as stream:
        st=os.fstat(stream.fileno())
        if not stat.S_ISREG(st.st_mode) or st.st_size>MAX: raise ValueError('Only text files up to 256 KB can be edited')
        data=stream.read(MAX+1)
        if len(data)>MAX or b'\0' in data: raise ValueError('Not a supported text file')
        data.decode('utf-8')
        return data,st

def perform(c,b):
    if b['action']=='roots': return {'roots':[{'id':r['id'],'name':r['name']} for r in c['roots']]}
    root=root_for(c,b['root']); components=parts(b.get('path',''))
    if b['action']=='list':
        fd=open_dir(root['path'],components)
        try:
            entries=[]
            for name in sorted(os.listdir(fd)):
                if name.startswith('.') or name in EXCLUDED: continue
                st=os.stat(name,dir_fd=fd,follow_symlinks=False)
                if stat.S_ISDIR(st.st_mode) or (stat.S_ISREG(st.st_mode) and st.st_size<=MAX):
                    entries.append({'name':name,'directory':stat.S_ISDIR(st.st_mode)})
            return {'entries':entries[:1000],'truncated':len(entries)>1000}
        finally: os.close(fd)
    if not components: raise ValueError('Choose a file')
    fd=open_dir(root['path'],components[:-1]);name=components[-1]
    try:
        try: data,st=read_at(fd,name)
        except FileNotFoundError:
            if b['action']!='write' or b.get('revision')!='new': raise
            data=None;st=None
        if b['action']=='read':return {'content':data.decode('utf-8'),'revision':digest(data)}
        if b['action']!='write':raise ValueError('Unsupported operation')
        new=b['content'].encode('utf-8')
        if len(new)>MAX or b'\0' in new:raise ValueError('Only text files up to 256 KB can be saved')
        if data==new:return {'revision':digest(new),'saved':True}
        if (digest(data) if data is not None else 'new')!=b.get('revision'):
            return {'error':'File changed on Mac. Your phone draft is kept. Reload the Mac version or save your draft under a new filename.','conflict':True}
        if data is not None:
            backup=SUPPORT/'remote-backups'/b['id'];backup.parent.mkdir(parents=True,exist_ok=True)
            with backup.open('xb') as stream:stream.write(data);stream.flush();os.fsync(stream.fileno())
        # Never truncate the original: write and fsync a sibling before replacing.
        temp='.jade-remote-'+b['id']
        f=os.open(temp,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=fd)
        try:
            with os.fdopen(f,'wb') as stream:
                stream.write(new);stream.flush();os.fsync(stream.fileno())
                if st:os.fchmod(stream.fileno(),stat.S_IMODE(st.st_mode))
            if data is None:
                os.link(temp,name,src_dir_fd=fd,dst_dir_fd=fd,follow_symlinks=False)
                os.unlink(temp,dir_fd=fd)
            else:
                latest,_=read_at(fd,name)
                if latest!=data:raise ValueError('File changed during save; phone draft retained')
                os.rename(temp,name,src_dir_fd=fd,dst_dir_fd=fd)
            os.fsync(fd)
        finally:
            try:os.unlink(temp,dir_fd=fd)
            except FileNotFoundError:pass
        return {'revision':digest(new),'saved':True}
    finally:os.close(fd)

def api(c,path,body=None):
    req=urllib.request.Request(c['endpoint']+path,data=None if body is None else json.dumps(body).encode(),headers={'Authorization':'Bearer '+c['agentToken'],'Content-Type':'application/json','User-Agent':'JaDE/1.0'})
    with urllib.request.urlopen(req,timeout=20) as response:return json.load(response)

def companion_request(b):
    """Only proxy Sanjana's fixed local endpoint, never an arbitrary URL or command."""
    payload=b.get('companion')
    if payload is not None:
        if not isinstance(payload,dict) or payload.get('action') not in {'chat','research','discover','enabled','seen'} or set(payload)-{'action','message','enabled','seen'}:raise ValueError('Invalid companion action')
        if payload['action']=='chat' and (not isinstance(payload.get('message'),str) or not payload['message'].strip() or len(payload['message'].encode())>8000):raise ValueError('Invalid message')
        if payload['action']=='enabled' and not isinstance(payload.get('enabled'),bool):raise ValueError('Invalid visibility')
        if payload['action']=='seen' and (not isinstance(payload.get('seen'),str) or len(payload['seen'])>80):raise ValueError('Invalid receipt')
    request=urllib.request.Request('http://127.0.0.1:7339/companion',data=None if payload is None else json.dumps(payload).encode(),headers={'Content-Type':'application/json','User-Agent':'JaDE/1.0'})
    try:
        with urllib.request.urlopen(request,timeout=195) as response:
            raw=response.read(524289)
            if len(raw)>524288:raise ValueError('Conversation is too large to download; open JaDE on Mac')
            state=json.loads(raw)
            if not isinstance(state,dict) or 'enabled' not in state:raise ValueError('Update the desktop JaDE service to use Sanjana')
            return {'companion':state}
    except urllib.error.HTTPError as e:
        return {'error':e.read(4000).decode('utf-8',errors='replace')}

def save_receipt(receipt,result):
    receipt.parent.mkdir(parents=True,exist_ok=True)
    tmp=receipt.with_suffix('.tmp')
    with tmp.open('w') as stream:
        json.dump(result,stream);stream.flush();os.fsync(stream.fileno())
    tmp.replace(receipt)
    fd=os.open(receipt.parent,os.O_RDONLY)
    try:os.fsync(fd)
    finally:os.close(fd)

def run_companion(b,receipt):
    # Reserve before calling the model. After an interrupted helper, return an
    # uncertain outcome instead of replaying a potentially completed conversation.
    save_receipt(receipt,{'error':'Connection interrupted. Refresh Sanjana to check the shared conversation before sending again.'})
    try:result=companion_request(b)
    except Exception:result={'error':'Sanjana could not be reached. Keep the Mac and its desktop JaDE service running, then refresh.'}
    save_receipt(receipt,result)
    return result

def run():
    os.umask(0o077)
    # Cloud and direct requests must share one module and one write lock.
    import sys
    sys.modules.setdefault("bridge",sys.modules[__name__])
    import cloud
    threading.Thread(target=cloud.run,daemon=True).start()
    companions=ThreadPoolExecutor(max_workers=4)
    running={}
    while True:
        try:
            c=config()
            for identifier,future in list(running.items()):
                if future.done():
                    try:result=future.result()
                    except Exception:result={'error':'Could not save the conversation receipt on Mac. Refresh shared history before trying again.'}
                    api(c,'/v1/remote/agent/result',{'id':identifier,'result':result})
                    del running[identifier]
            for b in api(c,'/v1/remote/agent')['requests']:
                if b['id'] in running:continue
                receipt=SUPPORT/'remote-receipts'/b['id'];receipt.parent.mkdir(parents=True,exist_ok=True)
                if receipt.exists():result=json.loads(receipt.read_text())
                elif b['action']=='companion':
                    if len(running)<4:
                        running[b['id']]=companions.submit(run_companion,b,receipt)
                        continue
                    result={'error':'Sanjana is busy. Wait a moment, then refresh.'}
                else:
                    try:
                        with FILE_LOCK:result=perform(config(),b)
                    except Exception as e:result={'error':str(e)}
                    tmp=receipt.with_suffix('.tmp');tmp.write_text(json.dumps(result));tmp.replace(receipt)
                api(c,'/v1/remote/agent/result',{'id':b['id'],'result':result})
            # Receipts only need to outlive the relay's one-hour retention.
            for p in (SUPPORT/'remote-receipts').glob('*'):
                if time.time()-p.stat().st_mtime>86400:p.unlink()
        except Exception as e:print('Remote connection waiting:',type(e).__name__,flush=True)
        time.sleep(3)
if __name__=='__main__':run()
