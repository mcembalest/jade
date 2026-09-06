// Durable, opt-in project replicas. Separate tables preserve the v1 Notes protocol.
const json = (value,status=200)=>Response.json(value,{status,headers:{'Cache-Control':'no-store'}});
const bytes = s=>new TextEncoder().encode(s).length;
const id = s=>typeof s==='string' && /^[a-zA-Z0-9-]{1,80}$/.test(s);
const excluded = new Set(['node_modules','vendor','build','dist','__pycache__','DerivedData']);
export const projectPath = p=>typeof p==='string' && bytes(p)<=768 && p.length>0 &&
 p.split('/').every(s=>s && !s.startsWith('.') && !excluded.has(s) && !/[\\\u0000-\u001f\u007f]/.test(s));
const metaColumns='path,revision,writer,updatedAt,bytes,macRevision,macIssue';
async function file(db,project,path) {return db.prepare('SELECT * FROM project_files WHERE project=? AND path=?').bind(project,path).first();}
export async function projects(request,env) {
 const u=new URL(request.url), auth=request.headers.get('Authorization');
 const agent=!!env.REMOTE_AGENT_TOKEN && auth===`Bearer ${env.REMOTE_AGENT_TOKEN}`;
 const phone=!!env.SYNC_TOKEN && auth===`Bearer ${env.SYNC_TOKEN}`;
 if(!agent&&!phone)return json({error:'Pairing required'},401);
 const parts=u.pathname.split('/').filter(Boolean);
 if(parts.length===2 && request.method==='GET') {
  const {results}=await env.DB.prepare('SELECT id,name,enabled,lastSeen,status FROM projects ORDER BY name').all();
  return json({projects:results});
 }
 const project=parts[2];if(!id(project))return json({error:'Invalid project'},400);
 let b;
 if(request.method==='POST') {
  const raw=await request.text();if(bytes(raw)>1600000)return json({error:'Request too large'},413);
  b=JSON.parse(raw);
 }
 if(parts.length===3 && request.method==='POST') {
  if(!agent)return json({error:'Only the Mac can enable projects'},403);
  if(typeof b.name!=='string'||!b.name.trim()||bytes(b.name)>256||typeof b.enabled!=='boolean')return json({error:'Invalid project'},400);
  await env.DB.prepare(`INSERT INTO projects(id,name,enabled,lastSeen,status) SELECT ?,?,?,?,? WHERE
    EXISTS(SELECT 1 FROM projects WHERE id=?) OR (SELECT COUNT(*) FROM projects)<16
    ON CONFLICT(id) DO UPDATE SET name=excluded.name,enabled=excluded.enabled,lastSeen=excluded.lastSeen,status=excluded.status`)
    .bind(project,b.name,b.enabled?1:0,new Date().toISOString(),String(b.status||'').slice(0,500),project).run();
  const p=await env.DB.prepare('SELECT * FROM projects WHERE id=?').bind(project).first();
  return p?json({project:p}):json({error:'Maximum 16 projects'},413);
 }
 const p=await env.DB.prepare('SELECT * FROM projects WHERE id=?').bind(project).first();
 if(!p)return json({error:'Project unavailable'},404);
 // Pausing stops writes, but existing cloud copies stay readable/exportable.
 if(request.method==='POST'&&!p.enabled)return json({error:'Cloud sync paused on Mac; your draft is retained'},403);
 const route=parts[3];
 if(route==='files' && request.method==='GET') {
  const {results}=await env.DB.prepare(`SELECT ${metaColumns} FROM project_files WHERE project=? ORDER BY path`).bind(project).all();
  return json({files:results});
 }
 const path=request.method==='GET'?u.searchParams.get('path'):b?.path;
 if(!projectPath(path))return json({error:'Unsupported project path'},400);
 if(route==='file'&&request.method==='GET') {
  const f=await file(env.DB,project,path);return f?json({file:f}):json({error:'File unavailable'},404);
 }
 if(route==='history'&&request.method==='GET') {
  const {results}=await env.DB.prepare('SELECT revision,writer,updatedAt,length(CAST(content AS BLOB)) AS bytes FROM project_revisions WHERE project=? AND path=? ORDER BY rowid DESC LIMIT 50').bind(project,path).all();
  return json({revisions:results});
 }
 if(route==='revision'&&request.method==='GET') {
  const r=await env.DB.prepare('SELECT path,content,revision,writer,updatedAt FROM project_revisions WHERE project=? AND path=? AND revision=?').bind(project,path,u.searchParams.get('revision')||'').first();
  return r?json({file:r}):json({error:'Revision unavailable'},404);
 }
 if(route==='ack'&&request.method==='POST') {
  if(!agent)return json({error:'Only the Mac can acknowledge disk delivery'},403);
  if(!id(b.revision))return json({error:'Invalid revision'},400);
  await env.DB.prepare('UPDATE project_files SET macRevision=?,macIssue=? WHERE project=? AND path=? AND revision=?')
   .bind(b.applied===true?b.revision:'',String(b.issue||'').slice(0,500),project,path,b.revision).run();
  return json({file:await file(env.DB,project,path)});
 }
 if(route!=='file'||request.method!=='POST')return json({error:'Not found'},404);
 if(typeof b.content!=='string'||b.content.includes('\0')||bytes(b.content)>262144||!id(b.mutationId)||!(b.baseRevision===''||id(b.baseRevision)))return json({error:'Invalid edit; text files up to 256 KB'},400);
 const writer=agent?'mac':'iphone';
 const previous=await env.DB.prepare('SELECT * FROM project_revisions WHERE project=? AND revision=?').bind(project,b.mutationId).first();
 const same=r=>r&&r.path===path&&r.content===b.content&&r.baseRevision===b.baseRevision&&r.writer===writer;
 if(previous)return same(previous)?json({acceptedRevision:b.mutationId,file:await file(env.DB,project,path)}):json({error:'Mutation ID already used'},409);
 const result=await env.DB.prepare(`INSERT OR IGNORE INTO project_revisions(project,revision,path,content,baseRevision,writer,updatedAt)
   SELECT ?,?,?,?,?,?,? WHERE COALESCE((SELECT revision FROM project_files WHERE project=? AND path=?),'')=?
   AND (EXISTS(SELECT 1 FROM project_files WHERE project=? AND path=?) OR (SELECT COUNT(*) FROM project_files WHERE project=?)<2000)
   AND COALESCE((SELECT SUM(bytes) FROM project_files WHERE project=? AND path<>?),0)+?<=33554432`)
  .bind(project,b.mutationId,path,b.content,b.baseRevision,writer,new Date().toISOString(),project,path,b.baseRevision,project,path,project,project,path,bytes(b.content)).run();
 if(!result.meta.changes) {
  const accepted=await env.DB.prepare('SELECT * FROM project_revisions WHERE project=? AND revision=?').bind(project,b.mutationId).first();
  if(same(accepted))return json({acceptedRevision:b.mutationId,file:await file(env.DB,project,path)});
  const f=await file(env.DB,project,path);
  return (f?.revision||'')!==b.baseRevision?json({error:'Conflict; your draft is retained',file:f},409):json({error:'Project limit: 2,000 files / 32 MB'},413);
 }
 return json({acceptedRevision:b.mutationId,file:await file(env.DB,project,path)});
}
