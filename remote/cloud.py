"""Durable project replicas; local permissions and v1 direct editing remain separate."""
from pathlib import Path
import json, os, stat, subprocess, time, uuid, urllib.parse, urllib.error
import bridge

class CloudError(Exception): pass

def durable(path, value):
    path.parent.mkdir(parents=True,exist_ok=True)
    temp=path.with_suffix('.tmp')
    with temp.open('w') as f:
        os.chmod(temp,0o600);json.dump(value,f,ensure_ascii=False);f.flush();os.fsync(f.fileno())
    temp.replace(path)
    fd=os.open(path.parent,os.O_RDONLY)
    try:os.fsync(fd)
    finally:os.close(fd)

def eligible(path):
    try:bridge.parts(path)
    except ValueError:return False
    return bool(path) and len(path.encode())<=768 and not any(ord(ch)<32 or ord(ch)==127 for ch in path) and Path(path).suffix.lower() not in {'.pem','.key','.p12','.pfx'}

def git_paths(root):
    if not (Path(root)/'.git').exists():return None
    result=subprocess.run(['git','-c','core.fsmonitor=false','ls-files','-z','--cached','--others','--exclude-standard'],cwd=root,capture_output=True,timeout=15)
    if result.returncode:raise CloudError('Cannot evaluate Git exclusions; project paused')
    return {p.decode('utf-8') for p in result.stdout.split(b'\0') if p}

def ignored(root,path):
    if not (Path(root)/'.git').exists():return False
    r=subprocess.run(['git','-c','core.fsmonitor=false','check-ignore','--quiet','--',path],cwd=root,capture_output=True,timeout=10)
    if r.returncode not in (0,1):raise CloudError('Cannot evaluate Git exclusions')
    return r.returncode==0

def scan(root):
    result={};skipped=0;total=0;git=git_paths(root)
    def walk(fd,prefix):
        nonlocal skipped,total
        for name in sorted(os.listdir(fd)):
            path=prefix+name
            if not eligible(path):continue
            st=os.stat(name,dir_fd=fd,follow_symlinks=False)
            if stat.S_ISDIR(st.st_mode):
                child=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
                try:walk(child,path+'/')
                finally:os.close(child)
            elif stat.S_ISREG(st.st_mode):
                if git is not None and path not in git:continue
                try:data,_=bridge.read_at(fd,name)
                except (ValueError,UnicodeError):skipped+=1;continue
                result[path]=data.decode('utf-8');total+=len(data)
                if len(result)>2000 or total>33554432:raise CloudError('Project exceeds 2,000 text files / 32 MB; choose a smaller folder')
    fd=bridge.open_dir(root,[])
    try:walk(fd,'')
    finally:os.close(fd)
    return result,skipped

def current(root,path):
    fd=bridge.open_dir(root,bridge.parts(path)[:-1])
    try:return bridge.read_at(fd,bridge.parts(path)[-1])[0].decode('utf-8')
    finally:os.close(fd)

def ensure_parents(root,path):
    fd=bridge.open_dir(root,[])
    try:
        for part in bridge.parts(path)[:-1]:
            try:os.mkdir(part,0o755,dir_fd=fd)
            except FileExistsError:pass
            child=os.open(part,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            os.close(fd);fd=child
    finally:os.close(fd)

class ProjectSync:
    def __init__(self,support=None,transport=None,permissions=None):
        self.support=support or bridge.SUPPORT
        self.transport=transport or bridge.api
        self.permissions=permissions
    def api(self,c,p,suffix='',body=None):
        return self.transport(c,'/v1/projects/'+p+suffix,body)
    def allowed(self,root):
        if self.permissions is None:return
        now=self.permissions()
        if not any(r['id']==root['id'] and r['path']==root['path'] and r.get('cloud') is True for r in now['roots']):
            raise CloudError('Project access changed; sync stopped')
    def cycle(self,c):
        roots={r['id']:r for r in c['roots'] if r.get('cloud') is True}
        # Empty default configuration does not publish any folders.
        projects=self.transport(c,'/v1/projects').get('projects',[])
        for p in projects:
            if p['enabled'] and p['id'] not in roots:self.api(c,p['id'],body={'name':p['name'],'enabled':False,'status':'Paused on Mac; cloud history retained'})
        statuses={}
        for root in roots.values():
            try:
                self.allowed(root)
                self.api(c,root['id'],body={'name':root['name'],'enabled':True,'status':'Checking files'})
                statuses[root['id']]=self.project(c,root)
            except Exception as e:statuses[root['id']]='Paused: '+str(e)
            try:
                self.allowed(root)
                self.api(c,root['id'],body={'name':root['name'],'enabled':True,'status':statuses[root['id']]})
            except Exception:pass
        durable(self.support/'cloud-status.json',{'checkedAt':time.time(),'projects':statuses})
        return statuses
    def project(self,c,root):
        p=root['id'];statepath=self.support/'cloud-state'/(p+'.json')
        state=json.loads(statepath.read_text()) if statepath.exists() else {}
        if not isinstance(state,dict) or any(not eligible(k) or not isinstance(v,dict) or not isinstance(v.get('content'),str) or not isinstance(v.get('revision'),str) for k,v in state.items()):
            raise CloudError('Saved sync state needs recovery; nothing was replaced')
        def persist():durable(statepath,state)
        def upload(path):
            pending=state[path].get('pending')
            if not pending:return
            self.allowed(root)
            try:reply=self.api(c,p,'/file',pending)
            except urllib.error.HTTPError as e:
                if e.code!=409:raise
                # A different cloud version won; reconciliation below preserves both.
                state[path].pop('pending',None);persist();return
            if reply.get('acceptedRevision')!=pending['mutationId']:raise CloudError('Cloud did not acknowledge the edit')
            state[path]={'content':pending['content'],'revision':pending['mutationId']};persist()
        # Replay durable upload IDs before reading current cloud revisions.
        for path in sorted(state):
            if not eligible(path) or ignored(root['path'],path):continue
            upload(path)
        locals,skipped=scan(root['path'])
        remotes={f['path']:f for f in self.api(c,p,'/files')['files']}
        problems=0
        for path in sorted(set(locals)|set(remotes)):
            self.allowed(root)
            remote=remotes.get(path);base=state.get(path)
            def ack(applied,issue=''):
                if remote and (remote.get('macRevision')!=remote['revision'] or remote.get('macIssue','')!=issue):
                    self.api(c,p,'/ack',{'path':path,'revision':remote['revision'],'applied':applied,'issue':issue})
            try:
                if not eligible(path) or ignored(root['path'],path):
                    ack(False,'Excluded by Mac project policy');problems+=1;continue
                local=locals.get(path)
                if local is None:
                    # Existing but binary/large/excluded files and local deletions
                    # must never be silently recreated or overwritten.
                    try:
                        current(root['path'],path)
                        ack(False,'Local file excluded from snapshot');problems+=1;continue
                    except FileNotFoundError:pass
                    if base:
                        ack(False,'Removed on Mac; cloud copy retained. Restore deliberately.');problems+=1;continue
                if remote:
                    if base and base['revision']==remote['revision']:
                        cloud=base['content']
                    else:
                        remote=self.api(c,p,'/file?'+urllib.parse.urlencode({'path':path}))['file']
                        cloud=remote['content']
                    if local==cloud:
                        state[path]={'content':cloud,'revision':remote['revision']};persist();ack(True);continue
                    if (base is None and local is None) or (base and local==base['content'] and remote['revision']!=base['revision']):
                        with bridge.FILE_LOCK:
                            self.allowed(root)
                            ensure_parents(root['path'],path)
                            r=bridge.perform(c,{'id':str(uuid.uuid4()),'action':'write','root':p,'path':path,'content':cloud,'revision':'new' if local is None else bridge.digest(local.encode())})
                            if not r.get('saved'):raise CloudError(r.get('error','Could not apply cloud edit'))
                            state[path]={'content':cloud,'revision':remote['revision']};persist()
                            # Acknowledgement describes this exact disk write.
                            ack(True)
                        continue
                    if base is None or remote['revision']!=base['revision']:
                        ack(False,'Conflict: Mac and cloud changed independently. Both versions are retained.');problems+=1;continue
                if local is not None:
                    state.setdefault(path,{'content':'','revision':''})
                    state[path]['pending']={'path':path,'content':local,'baseRevision':remote['revision'] if remote else '', 'mutationId':str(uuid.uuid4())}
                    persist();upload(path)
                    if state[path].get('pending') is None and state[path]['content']==local:
                        rev=state[path]['revision']
                        # Don't claim a stale scan still describes the Mac file.
                        if current(root['path'],path)==local:self.api(c,p,'/ack',{'path':path,'revision':rev,'applied':True})
            except Exception as e:
                problems+=1
                if remote:
                    try:ack(False,'Mac delivery paused: '+str(e))
                    except Exception:pass
                else:raise
        return f'{len(locals)} local text files; {skipped} unsupported files skipped; {problems} need attention'

def run():
    client=ProjectSync(permissions=bridge.config)
    while True:
        try:client.cycle(bridge.config())
        except Exception as e:print('Cloud projects waiting:',type(e).__name__,flush=True)
        time.sleep(20)
