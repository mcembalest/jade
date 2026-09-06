// Cloudflare Cron + private R2. No Mac, API token, or external scheduler required.
// Revisions are immutable and never pruned. A transactional manifest captures
// their high-water marks; paginated reads therefore describe one consistent cut.
const hash=async text=>Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text))),b=>b.toString(16).padStart(2,'0')).join('');
export async function backup(env, now=new Date()) {
 const startedAt=now.toISOString(), name=startedAt.replaceAll(':','-')+'-'+crypto.randomUUID(), prefix='daily/data/'+name+'/';
 try {
  if(!env.BACKUPS)throw new Error('Backup bucket is not configured');
  const snapshot=await env.DB.batch([
   env.DB.prepare('SELECT COALESCE(MAX(rowid),0) AS last FROM revisions'),
   env.DB.prepare('SELECT COALESCE(MAX(rowid),0) AS last FROM project_revisions'),
   env.DB.prepare('SELECT * FROM projects ORDER BY id'),
   env.DB.prepare('SELECT * FROM acknowledgements ORDER BY path,deviceId'),
   env.DB.prepare('SELECT path,revision FROM files ORDER BY path'),
   env.DB.prepare('SELECT project,path,revision,macRevision,macIssue FROM project_files ORDER BY project,path'),
   env.DB.prepare("SELECT sql FROM sqlite_schema WHERE type IN ('table','index','trigger') AND name IN ('revisions','files','acknowledgements','revision_applied','projects','project_files','project_revisions','project_history_path','project_revision_applied') ORDER BY CASE type WHEN 'table' THEN 0 WHEN 'index' THEN 1 ELSE 2 END")
  ]);
  const manifest={format:'jade-d1-backup-v1',createdAt:startedAt,retentionDays:30,
   projects:snapshot[2].results,acknowledgements:snapshot[3].results,noteHeads:snapshot[4].results,projectHeads:snapshot[5].results,
   schema:snapshot[6].results.map(r=>r.sql),chunks:[]};
  for(const [index,table] of ['revisions','project_revisions'].entries()) {
   const last=snapshot[index].results[0].last;let cursor=0,part=0;
   while(cursor<last) {
    // Eight maximum-size notes fit safely in a Worker, even with JSON escaping.
    const {results}=await env.DB.prepare(`SELECT rowid AS _rowid,* FROM ${table} WHERE rowid>? AND rowid<=? ORDER BY rowid LIMIT 8`).bind(cursor,last).all();
    if(!results.length)throw new Error('Incomplete revision history');
    cursor=results.at(-1)._rowid;
    const text=JSON.stringify(results),key=prefix+table+'-'+String(part++).padStart(8,'0')+'.json';
    const sha256=await hash(text);
    await env.BACKUPS.put(key,text,{httpMetadata:{contentType:'application/json'},customMetadata:{sha256}});
    manifest.chunks.push({key,table,sha256,count:results.length});
   }
  }
  // This is the completion marker. Failed/partial runs never publish a manifest.
  // Data lives one day longer than manifests, so lifecycle expiration cannot
  // remove an early chunk while its later completion marker is still valid.
  const text=JSON.stringify(manifest),key='daily/manifests/'+name+'.json';
  await env.BACKUPS.put(key,text,{httpMetadata:{contentType:'application/json'},customMetadata:{sha256:await hash(text)}});
  await env.DB.prepare("INSERT INTO backup_status(id,lastSuccess,manifest,error) VALUES(1,?,?, '') ON CONFLICT(id) DO UPDATE SET lastSuccess=excluded.lastSuccess,manifest=excluded.manifest,error='' ").bind(startedAt,key).run();
  return {lastSuccess:startedAt,manifest:key,error:'',retentionDays:30};
 } catch(error) {
  // Preserve the previous successful snapshot; never report partial backup as OK.
  await env.DB.prepare("INSERT INTO backup_status(id,lastSuccess,manifest,error) VALUES(1,'','',?) ON CONFLICT(id) DO UPDATE SET error=excluded.error").bind('Automatic backup failed; previous completed backups are retained.').run();
  throw error;
 }
}
export async function backupRoute(request,env,ctx) {
 const auth=request.headers.get('Authorization');
 const agent=!!env.REMOTE_AGENT_TOKEN && auth===`Bearer ${env.REMOTE_AGENT_TOKEN}`;
 if(!agent&&(!env.SYNC_TOKEN||auth!==`Bearer ${env.SYNC_TOKEN}`))return Response.json({error:'Pairing required'},{status:401});
 if(request.method==='POST') {
  if(!agent)return Response.json({error:'Mac credentials required'},{status:403});
  ctx.waitUntil(backup(env));return Response.json({queued:true},{status:202});
 }
 if(request.method!=='GET')return Response.json({error:'Not found'},{status:404});
 const status=await env.DB.prepare('SELECT lastSuccess,error FROM backup_status WHERE id=1').first();
 return Response.json({backup:{...(status||{lastSuccess:'',error:''}),retentionDays:30}},{headers:{'Cache-Control':'no-store'}});
}
