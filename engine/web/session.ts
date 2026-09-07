// UI metadata lives beside desktop preferences; text and drafts stay separate.
interface Position { head: number; scroll: number }
interface Session { file: string; positions: Record<string, Position> | null; filesOpen: boolean; filesPinned: boolean; folders: string[] | null }
const empty: Session = {file:'', positions:null, filesOpen:false, filesPinned:false, folders:null};
export const restoredSession: Session = (() => {
  try { return JSON.parse(document.querySelector('#session-state')?.textContent || 'null') || empty; }
  catch { return empty; }
})();
const positions = new Map(Object.entries(restoredSession.positions || {}));
let folders = restoredSession.folders || [];
let timer = 0;
let queue: Promise<void> | undefined;
let pending: {url: string; payload: string} | undefined;
let capture: (() => {file: string; head: number; scroll: number}) | undefined;
let previous = '';
export function position(file: string): Partial<Position> { return positions.get(file) || {}; }
export function rememberPosition(file: string, head: number, scroll: number) {
  if (file) {
    positions.delete(file); positions.set(file, {head, scroll:Math.max(0, scroll)});
    while (positions.size > 100) positions.delete(positions.keys().next().value!);
  }
  schedule();
}
function folderKey(folder: HTMLDetailsElement): string {
  const parts: string[] = [];
  let current: HTMLDetailsElement | null = folder;
  while (current) {
    parts.unshift(current.querySelector(':scope > summary')?.getAttribute('title') || '');
    current = current.parentElement?.closest('details') || null;
  }
  return parts.join('/');
}
function schedule() { clearTimeout(timer); timer = window.setTimeout(() => { void flushSession(); }, 350); }
export function flushSession(): Promise<void> {
  clearTimeout(timer);
  if (!capture) return queue || Promise.resolve();
  const current = capture();
  if (current.file) {
    positions.delete(current.file); positions.set(current.file, {head:current.head, scroll:Math.max(0, current.scroll)});
    while (positions.size > 100) positions.delete(positions.keys().next().value!);
  }
  // Filter expansion is temporary, not the user's folder preference.
  if (!document.querySelector<HTMLInputElement>('#file-filter')?.value.trim()) {
    folders = [...document.querySelectorAll<HTMLDetailsElement>('.tree details')].filter(folder => folder.open).map(folderKey).slice(0, 500);
  }
  const payload = JSON.stringify({file:current.file, positions:Object.fromEntries(positions),
    filesOpen:document.querySelector('#files-toggle')?.getAttribute('aria-expanded') === 'true',
    filesPinned:document.querySelector('#pin-files')?.getAttribute('aria-pressed') === 'true', folders});
  if (payload === previous) return queue || Promise.resolve();
  previous = payload;
  const url = '/session?jade=' + encodeURIComponent(document.body.dataset.jade!);
  // Coalesce changes while a request is in flight. Selection/scroll events
  // must never create a backlog that a later project switch has to drain.
  pending = {url, payload};
  if (!queue) queue = persistLatest().finally(() => { queue = undefined; });
  return queue;
}
async function persistLatest() {
  while (pending) {
    const {url, payload} = pending;
    pending = undefined;
    try {
      const response = await fetch(url, {method:'POST', headers:{'Content-Type':'application/json'}, body:payload, keepalive:true, signal:AbortSignal.timeout(3000)});
      if (!response.ok) throw new Error();
      document.querySelector('#session-save-notice')?.remove();
    } catch {
      // Metadata is optional. Release navigation after this one failed request;
      // the next user interaction can retry the current snapshot.
      previous = ''; pending = undefined;
      if (!document.querySelector('#session-save-notice')) {
        const notice = document.createElement('p'); notice.id = 'session-save-notice'; notice.setAttribute('role', 'status');
        notice.textContent = 'Could not remember your editing position. File saving is separate.';
        document.querySelector('#editor')!.before(notice);
      }
    }
  }
}

export function initSession(getPosition: () => {file: string; head: number; scroll: number}, scroller: HTMLElement) {
  capture = getPosition;
  for (const folder of document.querySelectorAll<HTMLDetailsElement>('.tree details')) {
    folder.open = folders.includes(folderKey(folder)); folder.addEventListener('toggle', schedule);
  }
  scroller.addEventListener('scroll', schedule, {passive:true});
  document.querySelector('#files-toggle')!.addEventListener('click', schedule);
  document.querySelector('#pin-files')!.addEventListener('click', schedule);
  document.addEventListener('visibilitychange', () => { if (document.hidden) void flushSession(); });
  addEventListener('pagehide', () => { void flushSession(); });
  schedule();
}
