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
export function notebook(state) {
 return {name:'',profile:state.profile??'',instructions:'',memory:'',timezone:'America/New_York',hour:20,avatar:'',revision:'',...state.notebook};
}
function view(state,env) { return {...state,notebook:notebook(state),providerStatus:env.RESEARCH&&!state.runtimeConnected?'Sign in to your OpenAI/Codex subscription on the cloud runtime to enable daily research.':providerStatus(env),enabled:!state.paused}; }
export async function history(db, before=Number.MAX_SAFE_INTEGER, kind='') {
 const kinds = kind === 'research' ? ['daily','finding'] : ['daily','finding','chat','notebook','run'];
 const {results}=await db.prepare(`SELECT * FROM companion_archive WHERE seq<? AND kind IN (${kinds.map(()=>'?').join(',')}) ORDER BY seq DESC LIMIT 51`).bind(before,...kinds).all();
 const rows=results.slice(0,50);
 return {entries:rows.map(r=>({...r,document:JSON.parse(r.document)})),before:results.length>50?rows.at(-1).seq:null};
}
function validNotebook(n) {
 if(!n || typeof n!=='object')return false;
 for(const [key,max] of Object.entries({name:80,profile:20000,instructions:20000,memory:100000,timezone:100,avatar:750000}))if(typeof n[key]!=='string'||n[key].length>max)return false;
 if(!Number.isInteger(n.hour)||n.hour<0||n.hour>23)return false;
 if(n.avatar && n.avatar!=='sanjana' && !/^data:image\/(png|jpeg|webp);base64,[A-Za-z0-9+/=]+$/.test(n.avatar))return false;
 try{new Intl.DateTimeFormat('en-US',{timeZone:n.timezone}).format();}catch{return false;}
 return true;
}
export async function companionRoute(request,env) {
 const auth=request.headers.get('Authorization');
 const agent=!!env.REMOTE_AGENT_TOKEN&&auth===`Bearer ${env.REMOTE_AGENT_TOKEN}`;
 if(!agent&&(!env.SYNC_TOKEN||auth!==`Bearer ${env.SYNC_TOKEN}`))return json({error:'Pairing required'},401);
 if(request.method==='GET') {
  const u=new URL(request.url);
  if(u.searchParams.has('runtime')) {
   if(!agent)return json({error:'Mac credentials required'},403);
   if(!env.RESEARCH)return json({connected:false,error:'Cloud runtime is not deployed'});
   const status=await runtimeCall(env,'/login/status');
   if(status.connected)await changeState(env.DB,s=>{if(s.runtimeConnected)return false;s.runtimeConnected=true;s.researchError='';});
   return json(status);
  }
  if(u.searchParams.has('history')) {
   const before=u.searchParams.has('before')?Number(u.searchParams.get('before')):Number.MAX_SAFE_INTEGER;
   if(!Number.isSafeInteger(before)||before<1)return json({error:'Invalid history cursor'},400);
   return json(await history(env.DB,before,u.searchParams.get('kind')??''));
  }
  const row=await readState(env.DB);return row?json(view(row.state,env)):json(view({messages:[],pending:[],paused:true},env));
 }
 if(request.method!=='POST')return json({error:'Not found'},404);
 const raw=await request.text();if(new TextEncoder().encode(raw).length>1024*1024)return json({error:'Request too large'},413);
 const b=JSON.parse(raw);
 if(b.action==='runtimeLogin') {
  if(!agent)return json({error:'Mac credentials required'},403);
  if(!env.RESEARCH)return json({error:'Cloud runtime is not deployed'},503);
  return json(await runtimeCall(env,'/login/start'));
 }
 if(b.action==='migrate') {
  if(!agent)return json({error:'Mac credentials required'},403);
  if(typeof b.migration!=='string'||!b.migration||typeof b.profile!=='string'||b.profile.length>20000||!b.state||!Array.isArray(b.state.messages)||!Array.isArray(b.state.pending??[]))return json({error:'Invalid migration'},400);
  // Refuse oversized history rather than silently discard it.
  if(b.state.messages.length>100||(b.state.pending??[]).length>24)return json({error:'Archive oversized history before migration'},400);
  const s={...b.state,profile:b.profile,paused:b.state.enabled===false,messages:b.state.messages,pending:b.state.pending??[],dedup:[...new Set([...b.state.messages,...(b.state.pending??[])].flatMap(m=>(m.sources??[]).map(s=>canonicalURL(s.url)).filter(Boolean)))],migratedAt:Date.now()};
  await env.DB.prepare('INSERT OR IGNORE INTO companion_state(id,revision,document,migration) VALUES(1,1,?,?)').bind(JSON.stringify(s),b.migration).run();
  const row=await readState(env.DB);return row.migration===b.migration?json(view(row.state,env)):json({error:'Cloud history already exists; migration did not overwrite it'},409);
 }
 if(b.action==='notebook') {
  if(!validNotebook(b.notebook)||typeof b.baseRevision!=='string'||typeof b.id!=='string'||!/^[a-zA-Z0-9-]{1,80}$/.test(b.id))return json({error:'Invalid companion settings'},400);
  b.notebook=Object.fromEntries(['name','profile','instructions','memory','timezone','hour','avatar'].map(k=>[k,b.notebook[k]]));
  const initial={messages:[],pending:[],paused:true,profile:''};
  await env.DB.prepare("INSERT OR IGNORE INTO companion_state(id,revision,document,migration) VALUES(1,1,?,'new-user')").bind(JSON.stringify(initial)).run();
  const previous=await env.DB.prepare('SELECT document FROM companion_archive WHERE id=?').bind('notebook-'+b.id).first();
  if(previous) {
   const old=JSON.parse(previous.document);
   if(Object.keys(b.notebook).some(k=>old[k]!==b.notebook[k]))return json({error:'Save ID already used'},409);
   return json(view((await readState(env.DB)).state,env));
  }
  const updated=await changeState(env.DB,s=>{
   if(notebook(s).revision!==b.baseRevision)return false;
   s.notebook={...b.notebook,revision:b.id,updatedAt:Date.now()};s.profile=b.notebook.profile;
  });
  return updated?json(view(updated.state,env)):json({error:'Settings changed on another device. Your edits are still here; reload before saving.'},409);
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
 if(env.RESEARCH)return scheduledDaily(env,now);
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
  const blocked=provider===researchProvider&&providerStatus(env);
  s.run.status=blocked?'blocked':error?'failed':'complete';s.researchError=blocked||error;
  if(!error&&finding?.text&&finding.text.length<=600&&finding.sources?.length&&s.pending.length<24) {
   const sources=finding.sources.slice(0,3).filter(v=>canonicalURL(v.url));
   const urls=sources.map(v=>canonicalURL(v.url));
   if(urls.length&&!urls.some(u=>(s.dedup??[]).includes(u))) {
    s.pending.push({text:finding.text,sources,foundAt:now});s.dedup=[...(s.dedup??[]),...urls].slice(-2000);
   }
  }
 });
}

async function runtimeCall(env,path,payload={}) {
 const stub=env.RESEARCH.get(env.RESEARCH.idFromName('companion'));
 const response=await stub.fetch('https://runtime'+path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(payload)});
 const data=await response.json();if(!response.ok)throw new Error(data.error||'Cloud runtime unavailable');return data;
}
export async function scheduledDaily(env,now=Date.now(),run=payload=>runtimeCall(env,'/run',payload)) {
 if(!await readState(env.DB))return;
 await changeState(env.DB,s=>{
  if(s.paused||!s.queuedDaily)return false;
  s.messages=[...s.messages,s.queuedDaily].slice(-100);delete s.queuedDaily;
 });
 const row=await readState(env.DB);
 const n=notebook(row.state);
 if(row.state.paused||!n.name.trim()||!n.instructions.trim())return;
 const parts=Object.fromEntries(new Intl.DateTimeFormat('en-US',{timeZone:n.timezone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',hourCycle:'h23'}).formatToParts(new Date(now)).map(p=>[p.type,p.value]));
 const date=`${parts.year}-${parts.month}-${parts.day}`;
 if(Number(parts.hour)<n.hour)return;
 const id=crypto.randomUUID();
 const reserved=await changeState(env.DB,s=>{
  if(s.paused||s.dailyResearchDate>=date||notebook(s).revision!==n.revision)return false;
  s.dailyResearchDate=date;s.researchChecked=now;s.researchError='';s.run={id,at:now,date,status:'running'};
 });
 if(!reserved)return;
 try {
  const entries=[];let before=Number.MAX_SAFE_INTEGER,bytes=0;
  while(before) {
   const page=await history(env.DB,before);entries.push(...page.entries);bytes+=JSON.stringify(page.entries).length;
   if(bytes>32*1024*1024)throw new Error('The retained archive exceeds this runner’s current capacity. History is preserved; expand the runner before resuming.');
   before=page.before;
  }
  const result=await run({date,notebook:n,history:entries});
  if(typeof result.report!=='string'||!result.report.trim()||result.report.length>16000||typeof result.memory!=='string'||result.memory.length>100000||!Array.isArray(result.findings)||result.findings.length>12)throw new Error('Research returned an invalid report; existing memory was preserved.');
  for(const f of result.findings)if(typeof f.text!=='string'||!f.text.trim()||f.text.length>6000||!Array.isArray(f.sources)||!f.sources.length||f.sources.length>10||f.sources.some(x=>typeof x.title!=='string'||x.title.length>500||!canonicalURL(x.url)))throw new Error('A finding was missing valid sources; existing memory was preserved.');
  await changeState(env.DB,s=>{
   if(s.run?.id!==id)return false;
   const findings=result.findings.map((f,i)=>({...f,id:id+'-'+i,foundAt:now}));
   // The archive trigger captures these before a later run clears the short queue.
   s.pending=[...(s.pending??[]),...findings];
   const report={id:'daily-'+date+'-'+id,role:'assistant',text:result.report,sources:[...new Map(findings.flatMap(f=>f.sources).map(x=>[canonicalURL(x.url),x])).values()],proactive:true,foundAt:now};
   if(s.paused)s.queuedDaily=report;else s.messages.push(report);
   s.messages=s.messages.slice(-100);s.dailyDate=date;s.run.status='complete';
   if(notebook(s).revision===n.revision)s.notebook={...n,memory:result.memory,revision:'run-'+id,updatedAt:now};
   else s.memoryProposal={id,foundAt:now,text:result.memory};
  });
  await changeState(env.DB,s=>{if(s.run?.id!==id)return false;s.pending=[];});
 } catch(error) {
  await changeState(env.DB,s=>{if(s.run?.id!==id)return false;s.run.status='failed';s.researchError=error.message||'Cloud research failed. The next daily run will try again.';});
 }
}
