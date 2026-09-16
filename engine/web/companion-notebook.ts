export interface Notebook { name:string; profile:string; instructions:string; memory:string; timezone:string; hour:number; avatar:string; revision:string }
export function initCompanionNotebook(changed:()=>void) {
  const dialog=document.querySelector<HTMLDialogElement>('#companion-notebook-dialog')!;
  const form=document.querySelector<HTMLFormElement>('#companion-notebook-form')!;
  const status=document.querySelector<HTMLElement>('#companion-notebook-status')!;
  const history=document.querySelector<HTMLElement>('#companion-history')!;
  const more=document.querySelector<HTMLButtonElement>('#companion-history-more')!;
  const keys=['name','profile','instructions','memory','timezone','hour'] as const;
  const field=(name:string)=>form.elements.namedItem(name) as HTMLInputElement|HTMLTextAreaElement;
  let base='',avatar='',dirty=false,busy=false,loaded=false,before:number|null|undefined;
  const draftKey='jade.companion.notebook-draft';
  const values=()=>Object.fromEntries(keys.map(k=>[k,k==='hour'?Number(field(k).value):field(k).value]));
  function stage() {
    dirty=true;
    try { localStorage.setItem(draftKey,JSON.stringify({base,avatar,values:values()}));status.textContent='Draft kept on this browser. Save to share with your companion.'; }
    catch { status.textContent='Draft storage unavailable. Keep this window open until saved.'; }
  }
  function fill(n:Record<string,unknown>) { for(const key of keys)field(key).value=String(n[key]??(key==='hour'?20:key==='timezone'?'America/New_York':'')); }
  async function request(url:string,body?:object) {
    const response=await fetch(url,{method:body?'POST':'GET',headers:body?{'Content-Type':'application/json'}:undefined,body:body?JSON.stringify(body):undefined,signal:AbortSignal.timeout(url.includes("runtime=") || (body as {action?:string})?.action==="runtimeLogin" ? 95000 : 25000)});
    if(!response.ok)throw new Error(await response.text());return response.json();
  }
  async function load() {
    if(busy)return;busy=true;
    try {
      const data=await request('/companion');if(data.offline)throw new Error('Offline. Saved browser drafts are kept; reconnect to edit shared settings.');
      const n=data.notebook??{};fill(n);base=n.revision??'';avatar=n.avatar??'';dirty=false;loaded=true;status.textContent='Saved settings loaded.';
      try { const draft=JSON.parse(localStorage.getItem(draftKey)??'null');if(draft) {fill(draft.values);base=draft.base;avatar=draft.avatar;dirty=true;status.textContent='Recovered browser draft. Review and save; newer cloud edits will be protected.';} } catch {status.textContent='Browser draft could not be read. Original storage is preserved.';}
    } catch(e) {status.textContent=(e as Error).message;}
    finally{busy=false;}
  }
  async function loadHistory(reset=false) {
    if(more.disabled)return;
    if(reset){before=undefined;history.replaceChildren();}
    if(before===null)return;
    more.disabled=true;
    try {
      const data=await request('/companion?history=1&kind=research'+(before?'&before='+before:''));
      for(const entry of data.entries) {
        const row=document.createElement('article');row.className='companion-message';
        const title=document.createElement('strong');title.textContent=(entry.kind==='daily'?'Daily report':'Finding')+' · '+(entry.foundAt?new Date(entry.foundAt).toLocaleString():'Imported history');
        const text=document.createElement('p');text.textContent=entry.document.text;row.append(title,text);
        for(const source of entry.document.sources??[]) {try {const url=new URL(source.url);if(!['http:','https:'].includes(url.protocol))continue;const link=document.createElement('a');link.href=url.href;link.textContent=source.title||url.hostname;link.target='_blank';link.rel='noopener noreferrer';row.append(link);}catch{}}
        history.append(row);
      }
      before=data.before;more.textContent=before===null?'All history loaded':'Older history';
      if(!history.children.length)history.textContent='No research recorded yet.';
    }catch(e){status.textContent=(e as Error).message;}
    finally{more.disabled=before===null;}
  }
  document.querySelector('#companion-notebook-open')!.addEventListener('click',()=>{dialog.showModal();if(!loaded)void load();if(before===undefined)void loadHistory();});
  document.querySelector('#companion-notebook-close')!.addEventListener('click',()=>dialog.close());
  form.addEventListener('input',stage);
  document.querySelector('#companion-notebook-reload')!.addEventListener('click',()=>{
    if(busy||dirty&&!confirm('Replace this browser draft with the saved companion settings?'))return;
    localStorage.removeItem(draftKey);void load();
  });
  document.querySelector('#companion-avatar-clear')!.addEventListener('click',()=>{avatar='';stage();});
  document.querySelector<HTMLInputElement>('#companion-avatar-file')!.addEventListener('change',async event=>{
    const file=(event.target as HTMLInputElement).files?.[0];if(!file)return;
    if(!['image/png','image/jpeg','image/webp'].includes(file.type)||file.size>500000){status.textContent='Choose a PNG, JPEG or WebP image under 500 KB.';return;}
    const reader=new FileReader();reader.onload=()=>{avatar=String(reader.result);stage();};reader.onerror=()=>{status.textContent='Could not read image.';};reader.readAsDataURL(file);
  });
  form.addEventListener('submit',async event=>{
    event.preventDefault();if(busy||!loaded)return;busy=true;
    const snapshot=JSON.stringify({values:values(),avatar});const body={action:'notebook',id:crypto.randomUUID(),baseRevision:base,notebook:{...values(),avatar}};
    try {
      const data=await request('/companion',body);base=data.notebook.revision;
      if(snapshot===JSON.stringify({values:values(),avatar})){dirty=false;try{localStorage.removeItem(draftKey);}catch{}status.textContent='Companion saved for all devices.';}else stage();
      changed();
    }catch(e){status.textContent=(e as Error).message;}
    finally{busy=false;}
  });
  const runtimeStatus=document.querySelector<HTMLElement>('#companion-runtime-status')!;
  async function checkRuntime(start=false) {
    try {
      if(start)await request('/companion',{action:'runtimeLogin'});
      const data=await request('/companion?runtime=1');runtimeStatus.replaceChildren();
      if(data.sandboxReady===false){runtimeStatus.textContent='Cloud runtime tools are unavailable. Research cannot run yet; settings and history are preserved.';}
      else if(data.connected){runtimeStatus.textContent='Cloud Codex connected. Daily research does not require your Mac.';changed();}
      else if(data.userCode){const link=document.createElement('a');link.href='https://auth.openai.com/codex/device';link.target='_blank';link.rel='noopener noreferrer';link.textContent='Sign in to OpenAI';runtimeStatus.append(link,document.createTextNode(' and enter '+data.userCode+'. Then check connection.'));}
      else runtimeStatus.textContent=data.error||'Preparing device login. Check connection in a moment.';
    }catch(e){runtimeStatus.textContent=(e as Error).message;}
  }
  document.querySelector('#companion-runtime-login')!.addEventListener('click',()=>void checkRuntime(true));
  document.querySelector('#companion-runtime-check')!.addEventListener('click',()=>void checkRuntime());
  more.addEventListener('click',()=>void loadHistory());
}
