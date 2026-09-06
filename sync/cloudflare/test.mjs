import {test, after, before} from 'node:test';
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {Miniflare, convertV4MiniflareOptions} from 'miniflare';
const token='local-test-key-not-for-production-123456';
const mf=new Miniflare(convertV4MiniflareOptions({modules:[{type:'ESModule',path:'worker.js',contents:await readFile('worker.js','utf8')},{type:'ESModule',path:'remote.js',contents:await readFile('remote.js','utf8')}],compatibilityDate:'2026-09-01',d1Databases:['DB'],bindings:{SYNC_TOKEN:token,REMOTE_AGENT_TOKEN:"agent-test-secret"}}));
before(async()=>{
 const db=await mf.getD1Database('DB');
 for(const statement of (await readFile('schema.sql','utf8')).match(/CREATE TABLE[\s\S]*?;|CREATE TRIGGER[\s\S]*?END;/g)) await db.prepare(statement).run();
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
