import {Container} from '@cloudflare/containers';
// Dedicated single-user runtime. No credential is exposed through Worker responses,
// D1 research history, backups, or public container ports.
export class ResearchContainer extends Container {
 defaultPort=8080;
 sleepAfter='12m';
 enableInternet=true;
 async captureLogin({until}) {
  if(Date.now()>until)return;
  try {
   const response=await this.fetch(new Request('https://runtime/login/status',{method:'POST',body:'{}'}));
   const state=await response.json();
   if(state.connected)return;
   if(response.ok&&!state.pending)return;
  } catch { /* A later bounded check can still retain the login result. */ }
  await this.schedule(20,'captureLogin',{until});
 }
 async fetch(request) {
  const path=new URL(request.url).pathname;
  if(!['/login/start','/login/status','/run'].includes(path))return Response.json({error:'Not found'},{status:404});
  // Reject overlapping calls rather than replaying a model request after uncertainty.
  if(this.busy)return Response.json({error:'Runtime busy'},{status:409});
  this.busy=true;
  try {
   const payload=await request.json();
   const auth=await this.ctx.storage.get('codex-auth');
   const response=await this.containerFetch('http://runtime'+path,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({...payload,auth}),signal:AbortSignal.timeout(590000)});
   const data=await response.json();
   if(data.auth)await this.ctx.storage.put('codex-auth',data.auth);
   if(path==='/login/status' && data.result && data.result.runtimeVersion!==2) {
    await this.destroy();
    return Response.json({connected:false,error:'Cloud runtime updated. Start cloud sign-in again.'});
   }
   if(path==='/login/start' && response.ok) {
    this.deleteSchedules('captureLogin');
    await this.schedule(20,'captureLogin',{until:Date.now()+15*60*1000});
   }
   return Response.json(data.error?{error:data.error}:data.result,{status:response.status});
  } finally {this.busy=false;}
 }
}
