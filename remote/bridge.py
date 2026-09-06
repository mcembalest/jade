"""Outbound-only personal Mac file bridge. Folder permissions are local to this Mac."""
from pathlib import Path
import hashlib, json, os, stat, tempfile, time, urllib.request, urllib.error

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

def run():
    os.umask(0o077)
    while True:
        try:
            c=config()
            for b in api(c,'/v1/remote/agent')['requests']:
                receipt=SUPPORT/'remote-receipts'/b['id'];receipt.parent.mkdir(parents=True,exist_ok=True)
                if receipt.exists():result=json.loads(receipt.read_text())
                else:
                    try:result=perform(config(),b)
                    except Exception as e:result={'error':str(e)}
                    tmp=receipt.with_suffix('.tmp');tmp.write_text(json.dumps(result));tmp.replace(receipt)
                api(c,'/v1/remote/agent/result',{'id':b['id'],'result':result})
            # Receipts only need to outlive the relay's one-hour retention.
            for p in (SUPPORT/'remote-receipts').glob('*'):
                if time.time()-p.stat().st_mtime>86400:p.unlink()
        except Exception as e:print('Remote connection waiting:',type(e).__name__,flush=True)
        time.sleep(3)
if __name__=='__main__':run()
