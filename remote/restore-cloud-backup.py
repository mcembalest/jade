"""Download an R2 backup into a NEW SQLite recovery file; never touch live storage.

Usage: python3 remote/restore-cloud-backup.py daily/.../manifest.json /new/recovery.sqlite
The manifest key is shown by D1's backup_status table or the private R2 bucket.
"""
import argparse,hashlib,json,os,sqlite3,subprocess,tempfile
from pathlib import Path

def restore(manifest,read_chunk,target):
    if manifest.get('format')!='jade-d1-backup-v1':raise ValueError('Unsupported backup format')
    if target.exists():raise ValueError('Recovery target already exists')
    db=sqlite3.connect(target)
    try:
        for sql in manifest['schema']:db.execute(sql)
        def insert(table,row):
            row={k:v for k,v in row.items() if k!='_rowid'}
            if not all(k.replace('_','').isalnum() for k in row):raise ValueError('Invalid column')
            db.execute('INSERT INTO '+table+'('+','.join('"'+k+'"' for k in row)+') VALUES('+','.join('?' for _ in row)+')',list(row.values()))
        for row in manifest['projects']:insert('projects',row)
        for row in manifest.get('companion',[]):insert('companion_state',row)
        for part in manifest['chunks']:
            if part['table'] not in ('revisions','project_revisions'):raise ValueError('Invalid backup table')
            raw=read_chunk(part['key'])
            if hashlib.sha256(raw).hexdigest()!=part['sha256']:raise ValueError('Backup checksum mismatch')
            rows=json.loads(raw)
            if len(rows)!=part['count']:raise ValueError('Backup row count mismatch')
            for row in rows:insert(part['table'],row)
        for row in manifest['acknowledgements']:insert('acknowledgements',row)
        for row in manifest['projectHeads']:
            db.execute('UPDATE project_files SET macRevision=?,macIssue=? WHERE project=? AND path=? AND revision=?',(row['macRevision'],row['macIssue'],row['project'],row['path'],row['revision']))
        db.row_factory=sqlite3.Row
        actual=[dict(r) for r in db.execute('SELECT path,revision FROM files ORDER BY path')]
        if actual!=manifest['noteHeads']:raise ValueError('Notes snapshot does not match history')
        actual=[dict(r) for r in db.execute('SELECT project,path,revision,macRevision,macIssue FROM project_files ORDER BY project,path')]
        if actual!=manifest['projectHeads']:raise ValueError('Project snapshot does not match history')
        if db.execute('PRAGMA integrity_check').fetchone()[0]!='ok':raise ValueError('SQLite integrity failed')
        db.commit()
    except BaseException:
        db.close();target.unlink(missing_ok=True);raise
    db.close()

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('manifest');parser.add_argument('output',type=Path);args=parser.parse_args()
    os.umask(0o077)
    repo=Path(__file__).resolve().parent.parent
    def download(key):
        if not key.startswith('daily/') or '..' in key.split('/'):raise ValueError('Invalid backup key')
        with tempfile.TemporaryDirectory() as folder:
            local=Path(folder)/'object.json'
            subprocess.run([str(repo/'sync/cloudflare/node_modules/.bin/wrangler'),'r2','object','get','jade-personal-backups/'+key,'--remote','--file',str(local)],cwd=repo/'sync/cloudflare',check=True,stdout=subprocess.DEVNULL)
            return local.read_bytes()
    # Download/validate in isolation; publish the recovery file only after success.
    if args.output.exists():raise SystemExit('Choose a new output file')
    with tempfile.TemporaryDirectory(dir=args.output.resolve().parent) as folder:
        staged=Path(folder)/'recovery.sqlite'
        restore(json.loads(download(args.manifest)),download,staged)
        os.link(staged,args.output)
    print('Backup restored and verified in:',args.output.resolve())
    print('Live notes, projects, credentials and services were not changed.')

if __name__=='__main__':main()
