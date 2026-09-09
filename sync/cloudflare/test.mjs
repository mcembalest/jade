import {test, after, before} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {Miniflare, convertV4MiniflareOptions} from 'miniflare';
import {backup} from './backups.js';
import {mkdtemp,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {execFileSync} from 'node:child_process';
const token='local-test-key-not-for-production-123456';
const mf=new Miniflare(convertV4MiniflareOptions({modules:[{type:'ESModule',path:'worker.js',contents:await readFile('worker.js','utf8')},...await Promise.all(['companion.js','companion-provider.js'].map(async path=>({type:'ESModule',path,contents:await readFile(path,'utf8')}))),{type:'ESModule',path:'remote.js',contents:await readFile('remote.js','utf8')},{type:'ESModule',path:'projects.js',contents:await readFile('projects.js','utf8')},{type:'ESModule',path:'backups.js',contents:await readFile('backups.js','utf8')}],compatibilityDate:'2026-09-01',r2Buckets:['BACKUPS'],d1Databases:['DB'],bindings:{SYNC_TOKEN:token,REMOTE_AGENT_TOKEN:"agent-test-secret"}}));
before(async()=>{
 const db=await mf.getD1Database('DB');
 for(const statement of (await readFile('schema.sql','utf8') + '\n' + await readFile('projects.sql','utf8') + '\n' + await readFile('backups.sql','utf8') + '\n' + await readFile('companion.sql','utf8')).match(/CREATE TABLE[\s\S]*?;|CREATE INDEX[\s\S]*?;|CREATE TRIGGER[\s\S]*?END;/g)) await db.prepare(statement).run();
});
after(()=>mf.dispose());
const call=async(path,body,auth=token)=>{
 const r=await mf.dispatchFetch('https://jade.test'+path,{method:body?'POST':'GET',headers:{Authorization:'Bearer '+auth,'Content-Type':'application/json'},body:body?JSON.stringify(body):undefined});
 return {status:r.status,...await r.json()};
};
test('unauthorized readers and writers cannot access notes',async()=>{
 assert.equal((await call('/v1/files',null,'wrong')).status,401);
 assert.equal((await call('/v1/files',{path:'secret.md'},'wrong')).status,401);
});
test('compare-and-swap, lost response retry, revision history, and device receipts',async()=>{
 const first={path:'journal.md',content:'first',baseRevision:'',mutationId:'op-first',deviceId:'iphone'};
 let r=await call('/v1/files',first);assert.equal(r.status,200);assert.equal(r.file.acks.mac,undefined);
 assert.equal((await call('/v1/files',first)).acceptedRevision,'op-first');
 let receipt=await call('/v1/ack',{path:first.path,revision:'op-first',deviceId:'mac'});
 assert.equal(receipt.file.acks.mac,'op-first');
 const edits=await Promise.all(['mac','iphone'].map(deviceId=>call('/v1/files',{...first,content:deviceId,baseRevision:'op-first',mutationId:'op-'+deviceId,deviceId})));
 assert.deepEqual(edits.map(x=>x.status).sort(),[200,409]);
 const winner=edits.find(x=>x.status===200).file;
 assert.notEqual(winner.acks.mac,winner.revision);
 // A retry of an older committed upload must still succeed after later edits.
 r=await call('/v1/files',first);assert.equal(r.acceptedRevision,'op-first');assert.equal(r.file.revision,winner.revision);
 assert.equal((await call('/v1/files',{...first,content:'different'})).status,409);
 receipt=await call('/v1/ack',{path:first.path,revision:'bogus',deviceId:'mac'});
 assert.notEqual(receipt.file.acks.mac,'bogus');
 const db=await mf.getD1Database('DB');assert.equal((await db.prepare('SELECT COUNT(*) AS n FROM revisions').first()).n,2);
});
test('reject unsafe paths, unsupported types, oversized content, and arbitrary device IDs',async()=>{
 for(const path of ['../escape.md','.jade-sync/token.txt','/absolute.md','a//b.md','x.pdf','a/../b.md']){
  assert.equal((await call('/v1/files',{path,content:'x',baseRevision:'',mutationId:'unsafe',deviceId:'mac'})).status,400);
 }
 assert.equal((await call('/v1/files',{path:'big.txt',content:'a'.repeat(512*1024+1),baseRevision:'',mutationId:'big',deviceId:'mac'})).status,400);
 assert.equal((await call('/v1/files',{path:'ok.md',content:'x',baseRevision:'',mutationId:'bad-device',deviceId:'stranger'})).status,400);
});

test('remote relay requires separate agent credentials and preserves results',async()=>{
 assert.equal((await call('/v1/remote/agent')).status,401);
 assert.equal((await call('/v1/remote/request',{id:'remote-1',action:'shell'})).status,400);
 assert.equal((await call('/v1/remote/request',{id:'remote-1',action:'read',root:'allowed',path:'x.py'})).status,200);
 const jobs=await call('/v1/remote/agent',null,'agent-test-secret');assert.equal(jobs.requests[0].path,'x.py');
 assert.equal((await call('/v1/remote/agent/result',{id:'remote-1',result:{content:'x'}})).status,401);
 await call('/v1/remote/agent/result',{id:'remote-1',result:{content:'x',revision:'r'}},'agent-test-secret');
 assert.equal((await call('/v1/remote/result?id=remote-1')).result.content,'x');
 assert.equal((await call('/v1/remote/request',{id:'remote-1',action:'read',path:'other'})).status,409);
});
test('companion relay is authenticated, bounded, and preserves request identity',async()=>{
 const request={id:'companion-one',action:'companion',companion:{action:'chat',message:'Hello Sanjana'}};
 assert.equal((await call('/v1/remote/request',request,'wrong')).status,401);
 assert.equal((await call('/v1/remote/request',request)).status,200);
 assert.equal((await call('/v1/remote/request',request)).status,200);
 assert.equal((await call('/v1/remote/request',{...request,companion:{action:'chat',message:'different'}})).status,409);
 for(const companion of [{action:'shell'},{action:'chat',message:'a'.repeat(8001)},{action:'chat',message:'x',url:'https://bad.test'},{action:'enabled',enabled:'true'}])
  assert.equal((await call('/v1/remote/request',{...request,companion})).status,400);
 const jobs=await call('/v1/remote/agent',null,'agent-test-secret');
 assert.deepEqual(jobs.requests.find(r=>r.id===request.id).companion,request.companion);
 await call('/v1/remote/agent/result',{id:request.id,result:{companion:{enabled:true,messages:[{id:'one',role:'assistant',text:'Shared reply'}]}}},'agent-test-secret');
 assert.equal((await call('/v1/remote/result?id='+request.id)).result.companion.messages[0].text,'Shared reply');
});

test('durable cloud projects: opt-in, offline edits, receipts, retries, isolation and history',async()=>{
 const agent='agent-test-secret';
 assert.equal((await call('/v1/projects/p1',{name:'Project',enabled:true})).status,403);
 assert.equal((await call('/v1/projects/p1',{name:'Project',enabled:true},agent)).status,200);
 assert.equal((await call('/v1/projects/p2',{name:'Other',enabled:true},agent)).status,200);
 const initial={path:'src/café.py',content:'print(1)',baseRevision:'',mutationId:'project-initial'};
 assert.equal((await call('/v1/projects/p1/file',initial,agent)).acceptedRevision,initial.mutationId);
 assert.equal((await call('/v1/projects/p2/files')).files.length,0);
 const phone={...initial,baseRevision:initial.mutationId,mutationId:'phone-edit',content:'print(2)'};
 assert.equal((await call('/v1/projects/p1/file',phone)).acceptedRevision,'phone-edit');
 let snapshot=await call('/v1/projects/p1/files');assert.notEqual(snapshot.files[0].macRevision,'phone-edit');
 assert.equal((await call('/v1/projects/p1/ack',{path:initial.path,revision:'phone-edit',applied:true})).status,403);
 await call('/v1/projects/p1/ack',{path:initial.path,revision:initial.mutationId,applied:true},agent);
 assert.notEqual((await call('/v1/projects/p1/files')).files[0].macRevision,'phone-edit');
 await call('/v1/projects/p1/ack',{path:initial.path,revision:'phone-edit',applied:true},agent);
 assert.equal((await call('/v1/projects/p1/files')).files[0].macRevision,'phone-edit');
 assert.equal((await call('/v1/projects/p1/file',initial,agent)).acceptedRevision,initial.mutationId);
 assert.equal((await call('/v1/projects/p1/file',{...phone,mutationId:'stale',content:'loser'})).status,409);
 assert.equal((await call('/v1/projects/p1/history?path='+encodeURIComponent(initial.path))).revisions.length,2);
 const db=await mf.getD1Database('DB');
 await db.prepare("UPDATE project_revisions SET updatedAt='2020-01-01' WHERE project='p1'").run();
 await call('/v1/remote/agent',null,agent);
 assert.equal((await call('/v1/projects/p1/history?path='+encodeURIComponent(initial.path))).revisions.length,2);
 await call('/v1/projects/p1',{name:'Project',enabled:false},agent);
 assert.equal((await call('/v1/projects/p1/file',{...phone,baseRevision:'phone-edit',mutationId:'paused'})).status,403);
 assert.equal((await call('/v1/projects/p1/files')).files.length,1);
});
test('project CAS permits one concurrent writer and rejects unsafe paths',async()=>{
 const agent='agent-test-secret';await call('/v1/projects/race',{name:'Race',enabled:true},agent);
 const b={path:'file.ts',content:'first',baseRevision:'',mutationId:'race-initial'};
 await call('/v1/projects/race/file',b,agent);
 const winners=await Promise.all(['one','two'].map(x=>call('/v1/projects/race/file',{...b,content:x,baseRevision:b.mutationId,mutationId:x})));
 assert.deepEqual(winners.map(x=>x.status).sort(),[200,409]);
 for(const path of ['../x','/x','a//x','.git/config','a/../../x','a\\x','node_modules/x','x\u0000'])assert.equal((await call('/v1/projects/race/file',{...b,path})).status,400);
 assert.equal((await call('/v1/projects/race/file',{...b,content:'a'.repeat(262145)})).status,400);
});
test('backup is coherent during new writes, recoverable, and never marks partial runs complete',async()=>{
 const DB=await mf.getD1Database('DB'), bucket=await mf.getR2Bucket('BACKUPS');
 let wrote=false;
 const BACKUPS={put:async(...args)=>{
  if(!wrote){wrote=true;await call('/v1/files',{path:'during-backup.md',content:'after snapshot',baseRevision:'',mutationId:'during-backup',deviceId:'mac'});}
  return bucket.put(...args);
 }};
 const result=await backup({DB,BACKUPS});
 const manifest=await (await bucket.get(result.manifest)).json();
 assert.ok(!manifest.noteHeads.some(f=>f.path==='during-backup.md'));
 const dir=await mkdtemp(join(tmpdir(),'jade-backup-'));
 try {
  await writeFile(join(dir,'manifest.json'),JSON.stringify(manifest));
  for(let i=0;i<manifest.chunks.length;i++)await writeFile(join(dir,String(i)),await (await bucket.get(manifest.chunks[i].key)).text());
  execFileSync('python3',['-c',`import importlib.util,json,pathlib,sys
spec=importlib.util.spec_from_file_location('restore','../../remote/restore-cloud-backup.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
p=pathlib.Path(sys.argv[1]);manifest=json.loads((p/'manifest.json').read_text());keys={c['key']:str(i) for i,c in enumerate(manifest['chunks'])}
m.restore(manifest,lambda key:(p/keys[key]).read_bytes(),p/'recovered.sqlite')
try:
 m.restore(manifest,lambda key:b'corrupt',p/'broken.sqlite')
 raise AssertionError('Accepted corrupt backup')
except ValueError: pass
assert not (p/'broken.sqlite').exists()
`,dir],{stdio:'pipe'});
 } finally { await rm(dir,{recursive:true,force:true}); }
 await assert.rejects(backup({DB,BACKUPS:{put:async()=>{throw Error('R2 unavailable');}}}));
 const status=await DB.prepare('SELECT * FROM backup_status WHERE id=1').first();
 assert.equal(status.manifest,result.manifest);assert.ok(status.error);
 assert.equal((await call('/v1/backup')).backup.lastSuccess,result.lastSuccess);
 assert.equal((await call('/v1/backup',{},token)).status,403);
 assert.equal((await call('/v1/backup',null,'wrong')).status,401);
});

test('Cloudflare owns research: migration, no client triggers, overlap, failure, pause, DST and atomic publication',async()=>{
 const {scheduledCompanion,readState,changeState,localDate}=await import('./companion.js');
 const db=await mf.getD1Database('DB'),env={DB:db};
 const migration={action:'migrate',migration:'fixture-v1',profile:'Existing character',state:{enabled:true,messages:[{id:'old',role:'user',text:'Retained history'}],pending:[],researchNext:0}};
 assert.equal((await call('/v1/companion',migration)).status,403);
 assert.equal((await call('/v1/companion',migration,'agent-test-secret')).status,200);
 assert.equal((await call('/v1/companion',migration,'agent-test-secret')).messages[0].id,'old');
 assert.equal((await call('/v1/companion',{...migration,migration:'different'},'agent-test-secret')).status,409);
 for(const action of ['research','discover','enabled'])assert.equal((await call('/v1/companion',{action})).status,400);
 assert.equal((await call('/v1/companion',null,'bad')).status,401);
 let calls=0,finish;
 const provider=async()=>{calls++;return new Promise(resolve=>{finish=resolve;});};
 const noon=Date.parse('2026-09-05T16:00:00Z');
 const run=scheduledCompanion(env,noon,provider);
 while(!finish)await new Promise(r=>setTimeout(r,1));
 await Promise.all([scheduledCompanion(env,noon,provider),scheduledCompanion(env,noon+100,provider),call('/v1/companion'),call('/v1/companion')]);
 assert.equal(calls,1);
 finish({text:'Sourced finding',sources:[{title:'Original',url:'https://example.com/story?utm_source=x'}]});await run;
 assert.equal((await readState(db)).state.pending.length,1);
 // A new process/tick sees the durable reservation; failure cannot spend again.
 await scheduledCompanion(env,noon+3600000,async()=>{calls++;throw Error('failure')});
 await scheduledCompanion(env,noon+3600010,async()=>{calls++;});assert.equal(calls,2);
 assert.equal((await readState(db)).state.pending.length,1);
 await call('/v1/companion',{action:'settings',paused:true});
 await scheduledCompanion(env,noon+7200000,async()=>{calls++;});assert.equal(calls,2);
 await call('/v1/companion',{action:'settings',paused:false});
 await scheduledCompanion(env,noon+7200000,async()=>{calls++;return {text:'Duplicate URL',sources:[{url:'https://example.com/story'}]};});
 assert.equal((await readState(db)).state.pending.length,1);
 const evening=Date.parse('2026-09-06T00:00:00Z');
 await Promise.all(Array.from({length:5},()=>scheduledCompanion(env,evening,async()=>null)));
 let state=(await readState(db)).state;
 assert.equal(state.messages.filter(m=>m.proactive).length,1);assert.equal(state.pending.length,0);assert.equal(state.dailyDate,'2026-09-05');
 assert.equal(state.messages[0].text,'Retained history');assert.equal(state.profile,'Existing character');
 // Findings arriving after publication are retained for tomorrow, including pause midrun.
 await changeState(db,s=>{s.researchNext=0;});
 const late=scheduledCompanion(env,evening+1000,provider);while(calls<4)await new Promise(r=>setTimeout(r,1));
 await call('/v1/companion',{action:'settings',paused:true});
 finish({text:'Late finding',sources:[{title:'New',url:'https://example.com/new'}]});await late;
 assert.equal((await readState(db)).state.pending.length,1);
 await call('/v1/companion',{action:'settings',paused:false});
 await scheduledCompanion(env,evening+2000,async()=>null);assert.equal((await readState(db)).state.pending.length,1);
 assert.deepEqual(localDate(Date.parse('2026-11-02T01:00:00Z')),{date:'2026-11-01',hour:20});
 assert.deepEqual(localDate(Date.parse('2026-03-09T00:00:00Z')),{date:'2026-03-08',hour:20});
 // Downtime causes only one current opportunity, never a loop over missed hours.
 let recovery=0;await scheduledCompanion(env,evening+10*86400000,async()=>{recovery++;return null;});assert.equal(recovery,1);
 // The snapshot includes the complete authoritative document and CAS revision.
 const bucket=await mf.getR2Bucket('BACKUPS');const result=await backup({DB:db,BACKUPS:bucket});
 const manifest=JSON.parse(await (await bucket.get(result.manifest)).text());
 assert.deepEqual(manifest.companion[0].document,(await readState(db)).document);
 const folder=await mkdtemp(join(tmpdir(),'jade-companion-restore-'));
 try {
  await writeFile(join(folder,'manifest.json'),JSON.stringify(manifest));
  for(const part of manifest.chunks)await writeFile(join(folder,part.key.split('/').at(-1)),await(await bucket.get(part.key)).text());
  execFileSync('python3',['-c',`import importlib.util,json,pathlib,sqlite3
spec=importlib.util.spec_from_file_location('restore','../../remote/restore-cloud-backup.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
p=pathlib.Path(${JSON.stringify(folder)});manifest=json.loads((p/'manifest.json').read_text());m.restore(manifest,lambda key:(p/key.split('/')[-1]).read_bytes(),p/'restored.sqlite')
c=sqlite3.connect(p/'restored.sqlite');assert c.execute('SELECT document FROM companion_state').fetchone()[0]==manifest['companion'][0]['document']`]);
 }finally{await rm(folder,{recursive:true,force:true});}
});

test('provider adapter enforces search/page/output budgets and rejects uncited text',async()=>{
 const {researchProvider,MODEL}=await import('./companion-provider.js');let calls=0;
 const state={profile:'Original profile',messages:[],pending:[]};
 const env={SANJANA_AI_ENABLED:'true',SANJANA_GATEWAY:'test',AI:{run:async(model,input,options)=>{
  calls++;assert.equal(model,MODEL);assert.equal(input.max_tokens,1200);
  assert.equal(input.tools[0].max_uses,2);assert.equal(input.tools[1].max_uses,1);assert.equal(input.tools[1].max_content_tokens,3000);
  assert.equal(options.gateway.collectLog,false);assert.equal(options.gateway.skipCache,true);
  return {stop_reason:'end_turn',content:[{type:'text',text:'One sourced discovery',citations:[{url:'https://example.com/original',title:'Original'}]}]};
 }}};
 assert.equal(await researchProvider({...env,SANJANA_AI_ENABLED:'false'},state,0),null);assert.equal(calls,0);
 assert.equal((await researchProvider(env,state,0)).sources[0].url,'https://example.com/original');assert.equal(calls,1);
 env.AI.run=async()=>({stop_reason:'end_turn',content:[{type:'text',text:'Unsourced'}]});await assert.rejects(researchProvider(env,state,0));
 env.AI.run=async()=>({stop_reason:'pause_turn',content:[]});await assert.rejects(researchProvider(env,state,0));
});
