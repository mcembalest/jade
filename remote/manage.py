"""Choose which Mac folders JaDE may remotely edit."""
from pathlib import Path
import argparse, json, os, subprocess, uuid
from bridge import CONFIG

def main():
    os.umask(0o077)
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--add');parser.add_argument('--remove');parser.add_argument('--list',action='store_true')
    args=parser.parse_args();c=json.loads(CONFIG.read_text())
    if args.list:
        for r in c['roots']:print(r['id'],r['name'],r['path'])
        return
    if args.remove:
        c['roots']=[r for r in c['roots'] if r['id']!=args.remove]
    else:
        chosen=args.add
        if not chosen:
            result=subprocess.run(['osascript','-e','POSIX path of (choose folder with prompt "Choose a folder JaDE on your iPhone may read and edit. For a repo, choose the working copy you want to use.")'],capture_output=True,text=True)
            if result.returncode:return
            chosen=result.stdout.strip()
        folder=Path(chosen).expanduser().resolve(strict=True)
        if not folder.is_dir():raise SystemExit('Choose a folder')
        if folder==Path.home() or folder==Path('/'):
            raise SystemExit('Choose a specific writing folder or repo instead of your whole home folder or disk.')
        if not any(r['path']==str(folder) for r in c['roots']):c['roots'].append({'id':str(uuid.uuid4()),'name':folder.name,'path':str(folder)})
    temp=CONFIG.with_suffix('.tmp');temp.write_text(json.dumps(c,indent=2));temp.replace(CONFIG)
    print('Folder access updated. Tap Connect / refresh in JaDE → Mac files.')
if __name__=='__main__':main()
