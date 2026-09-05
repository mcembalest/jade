type Card = { panel: HTMLElement; frame: HTMLIFrameElement; url: string; parent?: Card; source?: HTMLElement; kept: boolean; file: string; close: HTMLButtonElement; keep: HTMLButtonElement; edit: HTMLButtonElement; title: HTMLElement };

export function initPreview(editFile: (file: string) => Promise<boolean>) {
  const base = document.querySelector<HTMLElement>('#resolved')!;
  const template = base.cloneNode(true) as HTMLElement;
  const toggle = document.querySelector<HTMLButtonElement>('#preview-toggle')!;
  const search = document.querySelector<HTMLDialogElement>('#search-dialog')!;
  const cards: Card[] = [];
  let activeURL = canonical(base.querySelector('iframe')!.getAttribute('src') || '');
  let activeName = base.querySelector('#view-name')!.textContent || 'Preview';
  let hoverTimer = 0, sweepTimer = 0, layer = 5;
  let activeCard: Card | undefined;
  function canonical(href: string) {
    const url=new URL(href,location.origin);
    if(url.pathname!=='/view')return href;
    const path: string[]=[];
    for(const part of ((url.searchParams.get('jade') || '.')+'/'+(url.searchParams.get('file') || '')).split('/')) {
      if(part==='..')path.pop(); else if(part && part!=='.')path.push(part);
    }
    url.searchParams.set('jade','.');url.searchParams.set('file',path.join('/'));url.searchParams.sort();
    return url.pathname+url.search+url.hash;
  }
  const editorFocus = () => document.querySelector<HTMLElement>('.cm-content')?.focus();
  function raise(card: Card) { card.panel.style.zIndex = String(++layer); }
  function position(card: Card, x: number, y: number) {
    card.panel.style.left = `${Math.max(8, Math.min(x, innerWidth-card.panel.offsetWidth-8))}px`;
    card.panel.style.top = `${Math.max(8, Math.min(y, innerHeight-card.panel.offsetHeight-8))}px`;
    card.panel.style.right = 'auto';
  }
  function updateToggle() {
    toggle.setAttribute('aria-pressed',String(!!activeCard));
    toggle.textContent = activeCard ? 'Hide preview' : 'Show preview';
  }
  function dismiss(card: Card, restore = true) {
    for (const child of [...cards].filter(c=>c.parent===card)) {
      if (child.kept) { child.parent=undefined; child.panel.querySelector<HTMLButtonElement>('.preview-back')!.hidden=true; }
      else dismiss(child,false);
    }
    cards.splice(cards.indexOf(card),1);
    if (card.panel===base) { base.hidden=true; base.querySelector('iframe')!.src='about:blank'; }
    else card.panel.remove();
    if (activeCard===card) activeCard=undefined;
    updateToggle();
    if (restore) {
      if (card.parent && cards.includes(card.parent)) { raise(card.parent); card.source?.focus(); }
      else if (card.source?.isConnected && card.source.getClientRects().length) card.source.focus();
      else editorFocus();
    }
  }
  function retain(card: Card): boolean {
    return card.kept || card.panel.matches(':hover') || !!card.source?.matches(':hover') || cards.some(c=>c.parent===card && retain(c));
  }
  function sweep() {
    clearTimeout(hoverTimer); clearTimeout(sweepTimer);
    sweepTimer=window.setTimeout(()=>{
      for (const card of [...cards].reverse()) if (cards.includes(card) && !retain(card)) dismiss(card,false);
    },300);
  }
  function pin(card: Card) { card.kept=true;card.keep.hidden=true; }
  function localURL(href: string, parent?: Card): string | null {
    const url=new URL(href,location.origin);
    if (url.origin!==location.origin || !/^https?:$/.test(url.protocol)) return null;
    if (url.pathname==='/view') return url.pathname+url.search+url.hash;
    const fileURL=new URL(href, new URL('/'+(parent?.file || ''),location.origin));
    const target=new URL('/view',location.origin);
    target.searchParams.set('jade','.'); target.searchParams.set('file',decodeURIComponent(fileURL.pathname.slice(1)));target.hash=fileURL.hash;
    return target.pathname+target.search+target.hash;
  }
  function reveal(url: string, name: string, source?: HTMLElement, parent?: Card, kept=false): Card {
    url=canonical(url);
    clearTimeout(hoverTimer);clearTimeout(sweepTimer);
    // Following a cycle brings its existing ancestor forward instead of cloning it.
    for (let ancestor=parent;ancestor;ancestor=ancestor.parent) {
      if (ancestor.url===url) { if (kept) { pin(ancestor); ancestor.close.focus(); } raise(ancestor);return ancestor; }
    }
    const existing=cards.find(c=>c.url===url && c.parent===parent);
    if (existing) { if (kept) { pin(existing);existing.close.focus(); }raise(existing);return existing; }
    for (const sibling of [...cards].filter(c=>c.parent===parent && !c.kept)) dismiss(sibling,false);
    const panel=base.hidden ? base : template.cloneNode(true) as HTMLElement;
    if(panel!==base) { panel.removeAttribute('id');panel.querySelectorAll('[id]').forEach(n=>n.removeAttribute('id')); }
    const owner=source?.closest('dialog[open]') || parent?.panel.closest('dialog[open]') || document.body;
    owner.append(panel);panel.hidden=false;
    panel.style.width='';
    const card: Card={panel,frame:panel.querySelector('iframe')!,url,parent,source,kept,file:'',close:panel.querySelector('.preview-close')!,keep:panel.querySelector('.preview-keep')!,edit:panel.querySelector('.preview-edit')!,title:panel.querySelector('.preview-title')!};
    cards.push(card);raise(card);card.title.textContent=name;card.title.title=name;card.keep.hidden=kept;card.edit.hidden=true;
    const back=panel.querySelector<HTMLButtonElement>('.preview-back')!;back.hidden=!parent;
    back.onclick=()=>dismiss(card);
    card.close.onclick=()=>dismiss(card);
    card.keep.onclick=()=>{pin(card);card.close.focus();};
    card.edit.onclick=async()=>{
      card.edit.disabled=true;
      const status=panel.querySelector<HTMLElement>('.preview-status')!;
      status.hidden=true;
      try {
        if (await editFile(card.file)) {
          if(search.open)search.close('edit');
          dismiss(card,false);editorFocus();
        } else {
          status.textContent=document.querySelector('#save-status')!.textContent;
          status.hidden=false;
        }
      }
      finally { card.edit.disabled=false; }
    };
    panel.querySelector<HTMLElement>('.preview-status')!.hidden=true;
    const box=parent?.panel.getBoundingClientRect();
    const left=box ? box.left-20 : 0, right=box ? innerWidth-box.right-20 : 0;
    // Keep the source readable beside its reference whenever the window has room.
    if(box && Math.max(left,right)>=440) {
      const width=Math.min(620,Math.max(left,right));
      panel.style.width=`min(${width}px,calc(100vw - 16px))`;
      position(card,left>right ? box.left-width-12 : box.right+12,box.y);
    } else position(card,box ? box.x+48 : innerWidth-644,box ? box.y+44 : 112);
    panel.onpointerenter=()=>clearTimeout(sweepTimer);
    panel.onpointerleave=sweep;
    panel.onpointerdown=()=>raise(card);
    panel.onkeydown=event=>{
      if(event.key==='Escape'){event.preventDefault();event.stopPropagation();dismiss(card);}
    };
    const handle=panel.querySelector<HTMLButtonElement>('.preview-handle')!;
    let drag: {id:number;x:number;y:number;left:number;top:number}|undefined;
    handle.onpointerdown=event=>{
      if(event.button!==0)return;
      pin(card);raise(card);const box=panel.getBoundingClientRect();
      drag={id:event.pointerId,x:event.clientX,y:event.clientY,left:box.x,top:box.y};
      handle.setPointerCapture(event.pointerId);panel.classList.add('dragging');event.preventDefault();
    };
    handle.onpointermove=event=>{if(drag?.id===event.pointerId)position(card,drag.left+event.clientX-drag.x,drag.top+event.clientY-drag.y);};
    handle.onpointerup=handle.onlostpointercapture=()=>{drag=undefined;panel.classList.remove('dragging');};
    handle.onkeydown=event=>{
      const steps: Record<string,[number,number]>={ArrowLeft:[-20,0],ArrowRight:[20,0],ArrowUp:[0,-20],ArrowDown:[0,20]};
      if(!steps[event.key])return;
      event.preventDefault();pin(card);const box=panel.getBoundingClientRect();position(card,box.x+steps[event.key][0],box.y+steps[event.key][1]);
    };
    card.frame.onload=()=>{
      if(!cards.includes(card))return;
      let doc: Document|null;
      try {doc=card.frame.contentDocument;} catch {return;}
      if(!doc?.body)return;
      card.file=doc.body.dataset.file || '';
      card.edit.hidden=doc.body.dataset.editable!=='true';
      card.frame.title=name;
      bindLinks(doc,card);
      doc.addEventListener('pointerdown',()=>{pin(card);raise(card);});
      doc.addEventListener('keydown',event=>{if(event.key==='Escape'){event.preventDefault();dismiss(card);}});
      if(new URL(card.url,location.origin).hash) {
        const id=decodeURIComponent(new URL(card.url,location.origin).hash.slice(1));doc.getElementById(id)?.scrollIntoView();
      }
    };
    card.frame.src=url;
    if(kept)card.close.focus();
    return card;
  }
  function bindLinks(doc: Document, parent?: Card) {
    const target=(event: Event)=>{
      const source=(event.target as Element).closest<HTMLElement>(parent ? 'a[href],img[data-preview-url]' : '.file-link,[data-preview-file]');
      if(!source)return;
      if(parent){
        const href=source.getAttribute('data-preview-url') || source.getAttribute('href')!;
        if(!href || href.startsWith('#'))return;
        return {source,url:localURL(href,parent),href,name:source.textContent?.trim() || source.getAttribute('alt') || href};
      }
      const file=source.dataset.previewFile || source.dataset.file!;
      if(!/\.md$/i.test(file))return;
      const url=new URL('/view',location.origin);url.searchParams.set('jade',document.body.dataset.jade!);url.searchParams.set('file',file);
      return {source,url:url.pathname+url.search,href:'',name:file};
    };
    doc.addEventListener('pointerover',event=>{
      if(event.pointerType==='touch')return;
      const link=target(event);
      if(!link?.url || link.source.contains(event.relatedTarget as Node))return;
      clearTimeout(hoverTimer);clearTimeout(sweepTimer);
      hoverTimer=window.setTimeout(()=>{if(link.source.isConnected)reveal(link.url!,link.name,link.source,parent);},450);
    });
    doc.addEventListener('pointerout',event=>{if(target(event))sweep();});
    if(parent) doc.addEventListener('click',event=>{
      const link=target(event);if(!link)return;
      event.preventDefault();clearTimeout(hoverTimer);
      if(!link.url){
        const url=new URL(link.href,location.origin);
        if(/^(https?:|mailto:)$/.test(url.protocol))window.open(url.href,'_blank','noopener');
        return;
      }
      if(event.metaKey||event.ctrlKey){window.open(link.url,'_blank','noopener');return;}
      reveal(link.url,link.name,link.source,parent,true);
    });
  }
  bindLinks(document);
  toggle.addEventListener('click',()=>{
    if(activeCard){dismiss(activeCard);return;}
    activeCard=reveal(activeURL,activeName,toggle,undefined,true);updateToggle();
  });
  search.addEventListener('close',()=>{
    for(const card of [...cards])if(search.contains(card.panel)){document.body.append(card.panel);if(!card.kept)dismiss(card,false);}
  });
  document.querySelector('#search-input')!.addEventListener('input',()=>{clearTimeout(hoverTimer);for(const card of [...cards])if(search.contains(card.panel)&&!card.kept)dismiss(card,false);});
  addEventListener('resize',()=>cards.forEach(c=>{const box=c.panel.getBoundingClientRect();position(c,box.x,box.y);}));
  addEventListener('keydown',event=>{
    if(event.key!=='Escape' || document.querySelector('dialog[open],:popover-open'))return;
    const card=[...cards].sort((a,b)=>Number(b.panel.style.zIndex)-Number(a.panel.style.zIndex))[0];
    if(card){event.preventDefault();dismiss(card);}
  });
  if(new URL(location.href).searchParams.has('view') && !toggle.hidden){activeCard=reveal(activeURL,activeName,toggle,undefined,true);updateToggle();}
  return (url: string, name: string, available: boolean, changedFile=false)=>{
    url=canonical(url);activeURL=url;activeName=name;toggle.hidden=!available;
    if(changedFile){activeCard=cards.find(c=>c.url===url);updateToggle();}
    else if(activeCard && activeCard.url===url){activeCard.frame.src=url;}
  };
}
