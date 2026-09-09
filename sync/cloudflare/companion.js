import {researchProvider, providerStatus} from './companion-provider.js';
const HOUR=3600000;
const json=(body,status=200)=>Response.json(body,{status,headers:{'Cache-Control':'no-store'}});
export function localDate(now) {
 const parts=new Intl.DateTimeFormat('en-US',{timeZone:'America/New_York',year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',hourCycle:'h23'}).formatToParts(new Date(now));
 const p=Object.fromEntries(parts.map(p=>[p.type,p.value]));return {date:`${p.year}-${p.month}-${p.day}`,hour:Number(p.hour)};
}
export async function readState(db) {
 const row=await db.prepare('SELECT * FROM companion_state WHERE id=1').first();
 return row ? {...row,state:JSON.parse(row.document)} : null;
}
// Retrying storage is safe; the callback MUST have no external side effects.
export async function changeState(db,change) {
 for(let attempt=0;attempt<20;attempt++) {
  const row=await readState(db);if(!row)throw new Error('Migration required');
  const result=change(row.state);if(result===false)return null;
  const updated=await db.prepare('UPDATE companion_state SET document=?,revision=revision+1 WHERE id=1 AND revision=?').bind(JSON.stringify(row.state),row.revision).run();
  if(updated.meta.changes)return {state:row.state,result};
 }
 throw new Error('Companion storage busy');
}
export function canonicalURL(value) {
 try {const u=new URL(value);if(!['http:','https:'].includes(u.protocol)||u.username||u.password)return null;
 u.hash='';for(const key of [...u.searchParams.keys()])if(key.startsWith('utm_')||['fbclid','gclid'].includes(key))u.searchParams.delete(key);
 return u.href.replace(/\/$/,'');}catch{return null;}
}
function view(state,env) { return {...state,providerStatus:providerStatus(env),enabled:!state.paused}; }
export async function companionRoute(request,env) {
 const auth=request.headers.get('Authorization');
 const agent=!!env.REMOTE_AGENT_TOKEN&&auth===`Bearer ${env.REMOTE_AGENT_TOKEN}`;
 if(!agent&&(!env.SYNC_TOKEN||auth!==`Bearer ${env.SYNC_TOKEN}`))return json({error:'Pairing required'},401);
 if(request.method==='GET') {
  const row=await readState(env.DB);return row?json(view(row.state,env)):json({error:'Sanjana’s desktop history needs migration'},409);
 }
 if(request.method!=='POST')return json({error:'Not found'},404);
 const raw=await request.text();if(new TextEncoder().encode(raw).length>1024*1024)return json({error:'Request too large'},413);
 const b=JSON.parse(raw);
 if(b.action==='migrate') {
  if(!agent)return json({error:'Mac credentials required'},403);
  if(typeof b.migration!=='string'||!b.migration||typeof b.profile!=='string'||b.profile.length>20000||!b.state||!Array.isArray(b.state.messages)||!Array.isArray(b.state.pending??[]))return json({error:'Invalid migration'},400);
  // Refuse oversized history rather than silently discard it.
  if(b.state.messages.length>100||(b.state.pending??[]).length>24)return json({error:'Archive oversized history before migration'},400);
  const s={...b.state,profile:b.profile,paused:b.state.enabled===false,messages:b.state.messages,pending:b.state.pending??[],dedup:[...new Set([...b.state.messages,...(b.state.pending??[])].flatMap(m=>(m.sources??[]).map(s=>canonicalURL(s.url)).filter(Boolean)))],migratedAt:Date.now()};
  await env.DB.prepare('INSERT OR IGNORE INTO companion_state(id,revision,document,migration) VALUES(1,1,?,?)').bind(JSON.stringify(s),b.migration).run();
  const row=await readState(env.DB);return row.migration===b.migration?json(view(row.state,env)):json({error:'Cloud history already exists; migration did not overwrite it'},409);
 }
 if(b.action==='settings') {
  if(typeof b.paused!=='boolean')return json({error:'Specify paused'},400);
  const r=await changeState(env.DB,s=>{s.paused=b.paused;});return json(view(r.state,env));
 }
 if(b.action==='seen'&&typeof b.seen==='string'&&b.seen.length<=100) {
  const r=await changeState(env.DB,s=>{s.seen=b.seen;});return json(view(r.state,env));
 }
 if(b.action==='appendChat'&&agent) {
  if(typeof b.id!=='string'||b.id.length>100||!Array.isArray(b.messages)||b.messages.length!==2||b.messages.some(m=>typeof m.text!=='string'||m.text.length>16000||!['user','assistant'].includes(m.role)))return json({error:'Invalid chat'},400);
  const r=await changeState(env.DB,s=>{if(!s.messages.some(m=>m.id===b.id+'-reply'))s.messages.push(...b.messages.map((m,i)=>({...m,id:b.id+(i?'-reply':'-user'),proactive:false})));s.messages=s.messages.slice(-100);});
  return json(view(r.state,env));
 }
 // Old clients and relay requests cannot trigger research or publication.
 return json({error:'Research and daily updates are scheduled only by Cloudflare'},400);
}
export async function scheduledCompanion(env,now=Date.now(),provider=researchProvider) {
 if(!await readState(env.DB))return;
 const {date,hour}=localDate(now);
 // Publication is a single CAS with no model call; findings survive any failure.
 await changeState(env.DB,s=>{
  if(s.paused||hour<20||s.dailyDate>=date||!s.pending.length)return false;
  const sources=[...new Map(s.pending.flatMap(m=>m.sources??[]).map(s=>[canonicalURL(s.url),s])).values()];
  s.messages.push({id:'daily-'+date,role:'assistant',text:'Daily update\n\n'+s.pending.map(m=>m.text).join('\n\n'),sources,proactive:true,foundAt:now});
  s.messages=s.messages.slice(-100);s.pending=[];s.dailyDate=date;
 });
 const id=crypto.randomUUID();
 const reserved=await changeState(env.DB,s=>{
  if(s.paused||s.pending.length>=24||now<(s.researchNext??0))return false;
  // Reservation is durable BEFORE any provider call; never replay uncertain work.
  s.researchNext=now+HOUR;s.researchChecked=now;s.run={id,at:now,status:'reserved'};
  s.researchError=provider===researchProvider?providerStatus(env):'';
 });
 if(!reserved)return;
 let finding,error='';
 try {finding=await provider(env,reserved.state,now);}catch {error='Research could not finish. No automatic retry; the next hourly opportunity will check again.';}
 await changeState(env.DB,s=>{
  if(s.run?.id!==id)return false;
  s.run.status=error?'failed':'complete';s.researchError=provider===researchProvider&&providerStatus(env)||error;
  if(!error&&finding?.text&&finding.text.length<=600&&finding.sources?.length&&s.pending.length<24) {
   const sources=finding.sources.slice(0,3).filter(v=>canonicalURL(v.url));
   const urls=sources.map(v=>canonicalURL(v.url));
   if(urls.length&&!urls.some(u=>(s.dedup??[]).includes(u))) {
    s.pending.push({text:finding.text,sources,foundAt:now});s.dedup=[...(s.dedup??[]),...urls].slice(-2000);
   }
  }
 });
}
