// Local-only integration server for the Go and Swift clients. No real notes.
import {Miniflare,convertV4MiniflareOptions} from 'miniflare';
import {readFile} from 'node:fs/promises';
const mf=new Miniflare(convertV4MiniflareOptions({host:'127.0.0.1',port:8799,modules:[{type:'ESModule',path:'worker.js',contents:await readFile('worker.js','utf8')},{type:'ESModule',path:'remote.js',contents:await readFile('remote.js','utf8')},{type:'ESModule',path:'projects.js',contents:await readFile('projects.js','utf8')},{type:'ESModule',path:'backups.js',contents:await readFile('backups.js','utf8')}],compatibilityDate:'2026-09-01',r2Buckets:['BACKUPS'],d1Databases:['DB'],bindings:{SYNC_TOKEN:'local-test-key-not-for-production-123456',REMOTE_AGENT_TOKEN:'agent-test-secret'}}));
const db=await mf.getD1Database('DB');
for(const sql of (await readFile('schema.sql','utf8') + '\n' + await readFile('projects.sql','utf8') + '\n' + await readFile('backups.sql','utf8')).match(/CREATE TABLE[\s\S]*?;|CREATE INDEX[\s\S]*?;|CREATE TRIGGER[\s\S]*?END;/g)) await db.prepare(sql).run();
console.log('Local sync test server ready on',String(await mf.ready));
process.on('SIGTERM',async()=>{await mf.dispose();process.exit()});
process.on('SIGINT',async()=>{await mf.dispose();process.exit()});
await new Promise(()=>{});
