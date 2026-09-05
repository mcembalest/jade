export function initPreview() {
  const panel = document.querySelector<HTMLElement>('#resolved')!;
  const frame = document.querySelector<HTMLIFrameElement>('#view-frame')!;
  const toggle = document.querySelector<HTMLButtonElement>('#preview-toggle')!;
  const close = document.querySelector<HTMLButtonElement>('#preview-close')!;
  const keep = document.querySelector<HTMLButtonElement>('#preview-keep')!;
  const handle = document.querySelector<HTMLElement>('#preview-handle')!;
  const title = document.querySelector<HTMLElement>('#view-name')!;
  let sticky = new URL(location.href).searchParams.has('view');
  let active = {url:frame.getAttribute('src') || '', title:title.textContent || 'Preview', available:!toggle.hidden};
  let hoverTimer = 0, closeTimer = 0, hovering = false;
  let drag: {id:number; x:number; y:number; left:number; top:number} | undefined;
  function clamp(left: number, top: number) {
    panel.style.left = `${Math.max(8, Math.min(left, innerWidth-panel.offsetWidth-8))}px`;
    panel.style.top = `${Math.max(8, Math.min(top, innerHeight-panel.offsetHeight-8))}px`;
    panel.style.right = 'auto';
  }
  function show(url: string, name: string) {
    panel.hidden = false;
    if (frame.getAttribute('src') !== url) frame.src = url;
    title.textContent = name; title.title = name;
    keep.hidden = sticky;
    toggle.setAttribute('aria-pressed', String(sticky));
    toggle.textContent = sticky ? 'Hide preview' : 'Show preview';
    const box = panel.getBoundingClientRect(); clamp(box.x, box.y);
  }
  function dismiss() {
    clearTimeout(hoverTimer); clearTimeout(closeTimer); hovering = false; sticky = false;
    const focused = panel.contains(document.activeElement);
    panel.hidden = true;
    toggle.setAttribute('aria-pressed','false'); toggle.textContent = 'Show preview';
    if (focused) (toggle.hidden ? document.querySelector<HTMLElement>('.cm-content') : toggle)?.focus();
  }
  function pin() { sticky = true; clearTimeout(closeTimer); keep.hidden = true; toggle.setAttribute('aria-pressed','true'); toggle.textContent = 'Hide preview'; }
  toggle.addEventListener('click', () => {
    if (sticky && !panel.hidden) dismiss();
    else { sticky = true; hovering = false; frame.src = active.url; show(active.url,active.title); close.focus(); }
  });
  close.addEventListener('click', dismiss);
  document.querySelector('#search-input')!.addEventListener('input', () => { if (!sticky) dismiss(); });
  document.querySelector('#search-dialog')!.addEventListener('close', () => { document.body.append(panel); if (!sticky) dismiss(); });
  keep.addEventListener('click', () => { pin(); close.focus(); });
  function scheduleClose() {
    clearTimeout(hoverTimer);
    clearTimeout(closeTimer);
    closeTimer = window.setTimeout(() => { if (!sticky) dismiss(); }, 250);
  }
  document.addEventListener('pointerover', event => {
    if (event.pointerType === 'touch' || sticky) return;
    const source = (event.target as Element).closest<HTMLElement>('[data-file], [data-preview-file]');
    const file = source?.dataset.previewFile || (source?.matches('.file-link') ? source.dataset.file : '');
    if (!source || !file || !/\.md$/i.test(file) || source.contains(event.relatedTarget as Node)) return;
    clearTimeout(closeTimer); clearTimeout(hoverTimer);
    hoverTimer = window.setTimeout(() => {
      if (sticky || !source.isConnected) return;
      const url = new URL('/view',location.origin);
      url.searchParams.set('jade',document.body.dataset.jade!); url.searchParams.set('file',file);
      const parent = source.closest('dialog[open]') || document.body;
      parent.append(panel);
      hovering = true; show(url.pathname+url.search,file);
    }, 450);
  });
  document.addEventListener('pointerout', event => {
    const source = (event.target as Element).closest('[data-preview-file], .file-link');
    if (source && !source.contains(event.relatedTarget as Node)) scheduleClose();
  });
  panel.addEventListener('pointerenter', () => clearTimeout(closeTimer));
  panel.addEventListener('pointerleave', scheduleClose);
  handle.addEventListener('pointerdown', event => {
    if (event.button !== 0) return;
    pin(); const box = panel.getBoundingClientRect();
    drag = {id:event.pointerId,x:event.clientX,y:event.clientY,left:box.x,top:box.y};
    handle.setPointerCapture(event.pointerId); panel.classList.add('dragging'); event.preventDefault();
  });
  handle.addEventListener('pointermove', event => {
    if (drag?.id === event.pointerId) clamp(drag.left+event.clientX-drag.x,drag.top+event.clientY-drag.y);
  });
  const endDrag = () => { drag = undefined; panel.classList.remove('dragging'); };
  handle.addEventListener('pointerup',endDrag); handle.addEventListener('lostpointercapture',endDrag);
  handle.addEventListener('keydown', event => {
    const offsets: Record<string,[number,number]> = {ArrowLeft:[-20,0],ArrowRight:[20,0],ArrowUp:[0,-20],ArrowDown:[0,20]};
    if (!offsets[event.key]) return;
    event.preventDefault(); pin(); const box = panel.getBoundingClientRect();
    clamp(box.x+offsets[event.key][0],box.y+offsets[event.key][1]);
  });
  addEventListener('resize', () => { if (!panel.hidden) { const box=panel.getBoundingClientRect(); clamp(box.x,box.y); } });
  addEventListener('keydown', event => {
    if (event.key === 'Escape' && !panel.hidden && !document.querySelector('dialog[open], :popover-open')) { event.preventDefault(); dismiss(); }
  });
  if (sticky && active.available) show(active.url,active.title);
  return (url: string, name: string, available: boolean, changedFile = false) => {
    active = {url,title:name,available}; toggle.hidden = !available;
    if (changedFile && !sticky) { dismiss(); }
    if (!available && !hovering) dismiss();
    else if (sticky && !hovering) {
      // A saved document has the same URL but new contents.
      if (frame.getAttribute('src') === url) frame.src = url;
      show(url,name);
    }
  };
}
