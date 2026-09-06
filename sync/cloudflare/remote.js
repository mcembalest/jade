const json = (v,s=200)=>Response.json(v,{status:s,headers:{'Cache-Control':'no-store'}});
export async function remote(request,env) {
 const u=new URL(request.url), agent=u.pathname.startsWith('/v1/remote/agent');
 const key=agent?env.REMOTE_AGENT_TOKEN:env.SYNC_TOKEN;
 if(!key || request.headers.get('Authorization')!==`Bearer ${key}`) return json({error:'Pairing required'},401);
 const now=Date.now();
 if(u.pathname==='/v1/remote/agent' && request.method==='GET') {
  await env.DB.prepare('DELETE FROM remote_requests WHERE created < ?').bind(now-3600000).run();
  const {results}=await env.DB.prepare('SELECT id, payload FROM remote_requests WHERE result IS NULL AND created > ? ORDER BY created LIMIT 8').bind(now-60000).all();
  return json({requests:results.map(r=>({id:r.id,...JSON.parse(r.payload)}))});
 }
 if(u.pathname==='/v1/remote/result' && request.method==='GET') {
  const row=await env.DB.prepare('SELECT result, created FROM remote_requests WHERE id=?').bind(u.searchParams.get('id')||'').first();
  if(!row) return json({error:'Request unavailable; reconnect and try again'},404);
  if(row.result) return json({result:JSON.parse(row.result)});
  return json({pending:true,expired:row.created<now-60000});
 }
 if(request.method!=='POST') return json({error:'Not found'},404);
 const raw=await request.text();
 if(new TextEncoder().encode(raw).length>800000) return json({error:'File too large'},413);
 const b=JSON.parse(raw);
 if(typeof b.id!=='string'|| !/^[a-zA-Z0-9-]{1,80}$/.test(b.id))return json({error:'Invalid request'},400);
 if(u.pathname==='/v1/remote/agent/result') {
  await env.DB.prepare('UPDATE remote_requests SET result=? WHERE id=? AND result IS NULL').bind(JSON.stringify(b.result),b.id).run();
  return json({ok:true});
 }
 if(u.pathname!=='/v1/remote/request'||!['roots','list','read','write'].includes(b.action))return json({error:'Unsupported operation'},400);
 const payload=JSON.stringify({action:b.action,root:b.root||'',path:b.path||'',content:b.content,revision:b.revision});
 const old=await env.DB.prepare('SELECT payload FROM remote_requests WHERE id=?').bind(b.id).first();
 if(old && old.payload!==payload)return json({error:'Request ID already used'},409);
 await env.DB.prepare('INSERT OR IGNORE INTO remote_requests(id,payload,created) SELECT ?,?,? WHERE (SELECT COUNT(*) FROM remote_requests WHERE result IS NULL AND created>?)<32').bind(b.id,payload,now,now-60000).run();
 return json({id:b.id});
}
