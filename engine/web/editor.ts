import { initCompanion } from './companion.js';
import { initSync } from './sync.js';
import { basicSetup } from 'codemirror';
import { EditorState, Compartment } from '@codemirror/state';
import { EditorView, keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';
import { markdown } from '@codemirror/lang-markdown';
import { python } from '@codemirror/lang-python';
import { javascript } from '@codemirror/lang-javascript';
import { go } from '@codemirror/lang-go';
import { initTerminals } from './terminal.js';
import { initSearch } from './search.js';
import { initPreview } from './preview.js';

interface Draft { id: string; token: string; content: string; revision: string; updated: string }
interface DraftOwner { id: string; queue: Promise<void>; version: number; draft: Draft | null }
interface FileData { selected: string; contents: string; revision: string; markdown: boolean; view: string; viewURL: string; title: string }
interface ActiveFile { file: string; revision: string; saved: string }
class HTTPError extends Error {
  constructor(message: string, readonly code: number) { super(message); }
}

const body = document.body;
let activeURL = location.href;
let historyIndex: number = history.state?.jadeIndex ?? 0;
let restoringHistory = false;
history.replaceState({...history.state, jadeIndex:historyIndex}, '', activeURL);
const form = document.querySelector<HTMLFormElement>('#editor-form')!;
const status = document.querySelector<HTMLElement>('#save-status')!;
const reloadButton = document.querySelector<HTMLButtonElement>('#reload-file')!;
const copyButton = document.querySelector<HTMLButtonElement>('#save-copy')!;
const saveButton = document.querySelector<HTMLButtonElement>('#save-now')!;
const updatePreview = initPreview(editPreviewFile);
let filesPinned = false;
try { filesPinned = document.cookie.split('; ').includes('jade-files-pinned=true'); } catch (_) {}
const pinFiles = document.querySelector<HTMLButtonElement>('#pin-files')!;
pinFiles.setAttribute('aria-pressed', String(filesPinned));
const filesButton = document.querySelector<HTMLButtonElement>('#files-toggle')!;
function showFiles(open: boolean) {
  const explorer = document.querySelector<HTMLElement>('#file-explorer')!;
  if (!open && explorer.contains(document.activeElement)) filesButton.focus();
  explorer.hidden = !open;
  document.querySelector<HTMLElement>('#shell')!.classList.toggle('files-hidden', !open);
  filesButton.setAttribute('aria-expanded', String(open));
}
filesButton.addEventListener('click', () => showFiles(filesButton.getAttribute('aria-expanded') !== 'true'));
showFiles(filesPinned);
pinFiles.addEventListener('click', () => {
  filesPinned = !filesPinned;
  pinFiles.setAttribute('aria-pressed', String(filesPinned));
  try { document.cookie = `jade-files-pinned=${filesPinned}; Path=/; Max-Age=31536000; SameSite=Strict`; } catch (_) {}
});
const viewFrame = document.querySelector<HTMLIFrameElement>('#view-frame')!;
const readOnly = new Compartment();
const sessions = new Map<string, {state: EditorState; revision: string; scroll: number}>();
const initial = document.querySelector<HTMLTextAreaElement>('#initial-content')!.value.replace(/\r\n/g, '\n');
let active: ActiveFile = {file: body.dataset.file!, revision: body.dataset.revision!, saved: body.dataset.crlf === 'true' ? initial.replace(/\n/g, '\r\n') : initial};
let saving: Promise<boolean> | null = null;
let moving = false, checking = false, conflict = false, autosaveTimer = 0;
let editor: EditorView;
let discardConfirmed = false, recovering = false, loadingDrafts = true;
const draftPanel = document.querySelector<HTMLElement>('#draft-recovery')!;
const draftSelect = document.querySelector<HTMLSelectElement>('#draft-select')!;
const backupStatus = document.createElement('div');
backupStatus.id = 'draft-status'; backupStatus.setAttribute('role', 'status');
form.insertBefore(backupStatus, document.querySelector<HTMLElement>('#editor')!);
const draftOwners = new Map<string, DraftOwner>();
let availableDrafts: Draft[] = [], recoveredDraft: Draft | null = null;
function owner() {
  if (!draftOwners.has(active.file)) draftOwners.set(active.file, {id:crypto.randomUUID(), queue:Promise.resolve(), version:0, draft:null});
  return draftOwners.get(active.file)!;
}
function draftURL(file = active.file) {
  const url = fileURL(file); url.pathname = '/drafts'; return url;
}
function backup() {
  const state = owner(), version = ++state.version;
  const file = active.file, content = text(), revision = active.revision;
  state.queue = state.queue.catch(() => {}).then(async () => {
    if (version !== state.version) return;
    const data = new URLSearchParams({jade:body.dataset.jade!,file,id:state.id,content,revision});
    state.draft = await (await request('/drafts', {method:'POST',body:data})).json();
    if (active.file === file) backupStatus.textContent = '';
  }).catch((error: unknown) => { backupStatus.textContent = 'Recovery backup unavailable: ' + (error instanceof Error ? error.message : String(error)); });
  return state.queue;
}
async function removeDraft(draft: Draft, file = active.file) {
  const url = draftURL(file); url.searchParams.set('id',draft.id); url.searchParams.set('token',draft.token);
  await request(url, {method:'DELETE'});
}
async function clearSavedDrafts() {
  const state = owner(); await state.queue;
  const draft = state.draft;
  if (draft?.content === active.saved) { await removeDraft(draft); if (state.draft === draft) state.draft = null; }
  if (recoveredDraft) { await removeDraft(recoveredDraft); recoveredDraft = null; }
}
function renderDrafts() {
  draftSelect.replaceChildren(...availableDrafts.map(draft => {
    const option = document.createElement('option'); option.value = draft.id;
    option.textContent = new Date(draft.updated).toLocaleString(); return option;
  }));
  draftPanel.hidden = availableDrafts.length === 0;
}
async function loadDrafts() {
  backupStatus.textContent = '';
  availableDrafts = []; renderDrafts();
  loadingDrafts = true; freeze(true); body.dataset.draftsReady = 'false';
  try {
    const data: {drafts: Draft[]} = await (await request(draftURL())).json();
    availableDrafts = data.drafts.filter(draft => draft.id !== owner().id);
    renderDrafts();
  } catch (error) { backupStatus.textContent = 'Cannot load recovery drafts: ' + (error instanceof Error ? error.message : String(error)); }
  finally { loadingDrafts = false; freeze(false); body.dataset.draftsReady = 'true'; }
}
function download(contents: string) {
  const link = document.createElement('a');
  link.href = URL.createObjectURL(new Blob([contents], {type:'text/plain;charset=utf-8'}));
  link.download = active.file.split('/').pop() || 'notes.txt'; link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 1000);
}

const text = () => editor.state.sliceDoc().replace(/\r\n/g, '\n').replace(/\n/g, editor.state.lineBreak);
const dirty = () => text() !== active.saved;
const cacheKey = (file: string) => 'jade-position:' + body.dataset.jade! + ':' + file;

function report(message: string, problem = false) {
  status.textContent = message;
  status.parentElement!.dataset.problem = String(problem);
  saveButton.hidden = !problem || conflict;
  copyButton.hidden = !problem;
  reloadButton.hidden = !problem;
}
function rememberPosition() {
  try { sessionStorage.setItem(cacheKey(active.file), JSON.stringify({head:editor.state.selection.main.head, scroll:editor.scrollDOM.scrollTop})); } catch (_) {}
}
function position(file: string): {head?: number; scroll?: number} {
  try { return JSON.parse(sessionStorage.getItem(cacheKey(file)) || "{}") || {}; } catch (_) { return {}; }
}
function language(file: string) {
  if (/\.md$/i.test(file)) return markdown();
  if (/\.py$/i.test(file)) return python();
  if (/\.[cm]?[jt]sx?$/i.test(file)) return javascript({typescript:/\.tsx?$/i.test(file), jsx:/x$/i.test(file)});
  if (/\.go$/i.test(file)) return go();
  return [];
}
function makeState(contents: string, file: string) {
  const head = Math.min(Math.max(0, position(file).head || 0), contents.replace(/\r\n/g, '\n').length);
  return EditorState.create({doc:contents, selection:{anchor:head}, extensions:[
    basicSetup, language(file), keymap.of([indentWithTab]), EditorView.lineWrapping,
    EditorState.lineSeparator.of(contents.includes('\r\n') ? '\r\n' : '\n'),
    EditorView.contentAttributes.of({'aria-label':'Editor', 'aria-description':'Press Escape then Tab to move focus out of the editor.', spellcheck:'false'}),
    readOnly.of([EditorState.readOnly.of(!file), EditorView.editable.of(!!file)]),
    EditorView.theme({
      '&':{height:'100%',fontSize:'13px'}, '.cm-scroller':{overflow:'auto',fontFamily:'ui-monospace, SFMono-Regular, Menlo, monospace'},
      '.cm-content':{padding:'20px 0',minHeight:'100%'}, '.cm-line':{padding:'0 18px'},
      '.cm-gutters':{backgroundColor:'#fbfcfb',border:'none',color:'#89948d'}, '&.cm-focused':{outline:'none'},
    }),
    EditorView.updateListener.of(update => {
      if (!update.docChanged) return;
      discardConfirmed = false; reloadButton.textContent = 'Reload from disk';
      clearTimeout(autosaveTimer);
      backup();
      if (conflict) { report('File changed on disk. Your edits are kept here.', true); return; }
      if (recovering) { report('Recovered draft. Save when ready, or reload from disk.', true); return; }
      report(dirty() ? 'Edited' : 'Saved');
      autosaveTimer = window.setTimeout(save, 800);
    }),
  ]});
}
editor = new EditorView({state:makeState(active.saved, active.file), parent:document.querySelector<HTMLElement>('#editor')!});
editor.scrollDOM.scrollTop = position(active.file).scroll || 0;
editor.focus();
function freeze(value: boolean) {
  editor.dispatch({effects:readOnly.reconfigure([EditorState.readOnly.of(value || !active.file), EditorView.editable.of(!value && !!active.file)])});
}
async function request(url: string | URL, options?: RequestInit) {
  const response = await fetch(url, {...options, signal:AbortSignal.timeout(10000)});
  if (!response.ok) {
    throw new HTTPError((await response.text()).trim(), response.status);
  }
  return response;
}
function fileURL(file = active.file) {
  const url = new URL('/file', location.origin);
  url.searchParams.set('jade', body.dataset.jade!); url.searchParams.set('file', file);
  const view = new URL(activeURL).searchParams.get('view');
  if (file === active.file && view) url.searchParams.set('view', view);
  return url;
}
function refreshPreview(data: FileData, changedFile = false) {
  document.querySelector<HTMLElement>('.project')!.textContent = data.title;
  document.querySelector<HTMLElement>('.project')!.title = data.title;
  document.title = data.title + ' · JaDE';
  updatePreview(data.viewURL, data.view ? 'Preview · ' + data.view : 'Preview · ' + data.selected, data.markdown, changedFile);
}
async function save() {
  clearTimeout(autosaveTimer);
  if (loadingDrafts) return false;
  recovering = false;
  if (saving) return saving;
  if (conflict) return false;
  saving = (async () => {
    try {
      for (;;) {
        while (dirty()) {
          const contents = text();
          await backup();
          report('Saving…');
          // URL encoding preserves LF/CRLF; multipart form serialization normalizes newlines.
          const data = new URLSearchParams();
          data.set('jade', body.dataset.jade!); data.set('file', active.file);
          data.set('content', contents); data.set('revision', active.revision);
          const result: {revision: string} = await (await request('/save', {method:'POST', body:data})).json();
          active.saved = contents; active.revision = result.revision;
        }
        try { await clearSavedDrafts(); }
        catch (error) { backupStatus.textContent = 'Saved; recovery draft retained: ' + (error instanceof Error ? error.message : String(error)); }
        if (!dirty()) break;
      }
      report('Saved'); rememberPosition();
      return true;
    } catch (error) {
      conflict = error instanceof HTTPError && error.code === 409;
      report(conflict ? 'File changed or was removed on disk. Your edits are kept here.' : 'Not saved: ' + (error instanceof Error ? error.message : String(error)), true);
      return false;
    }
  })();
  try { return await saving; } finally { saving = null; checkDisk(); }
}
async function leave(action: () => Promise<void>) {
  if (moving || loadingDrafts) return;
  moving = true; freeze(true);
  try { if (await save()) { rememberPosition(); await action(); } }
  catch (error) { report((error instanceof Error ? error.message : String(error)), true); }
  finally { moving = false; freeze(false); }
}
async function showFile(data: FileData, href: string) {
  sessions.set(active.file, {state:editor.state, revision:active.revision, scroll:editor.scrollDOM.scrollTop});
  const cached = sessions.get(data.selected);
  active = {file:data.selected, saved:data.contents, revision:data.revision};
  conflict = false; recovering = false; recoveredDraft = null;
  editor.setState(cached?.revision === data.revision ? cached.state : makeState(data.contents, data.selected));
  editor.scrollDOM.scrollTop = cached?.revision === data.revision ? cached.scroll : position(data.selected).scroll || 0;
  body.dataset.file = data.selected;
  document.querySelector<HTMLElement>('#file-name')!.textContent = data.selected || 'No file selected';
  document.querySelector<HTMLElement>('#file-name')!.title = data.selected;
  document.querySelector<HTMLElement>('#empty-editor')!.hidden = !!data.selected;
  document.querySelectorAll<HTMLAnchorElement>('.file-link').forEach(node => node.classList.toggle('active', node.dataset.file === data.selected && node.dataset.jade === body.dataset.jade));
  if (href) {
    history.pushState({jadeIndex:++historyIndex}, '', href);
    activeURL = location.href;
  }
  refreshPreview(data, true); report('Saved'); await loadDrafts();
  if (!filesPinned) showFiles(false);
  editor.focus();
}
document.querySelector<HTMLElement>('#workspace-root')!?.addEventListener('click', event => {
  event.preventDefault(); leave(async () => { location.href = '/'; });
});
document.querySelectorAll<HTMLAnchorElement>('.file-link').forEach(link => {
  link.classList.toggle('active', link.dataset.file === active.file && link.dataset.jade === body.dataset.jade!);
  link.addEventListener('click', event => {
    event.preventDefault();
    leave(async () => {
      if (link.dataset.jade !== body.dataset.jade!) { location.href = link.href; return; }
      if (link.dataset.file === active.file && !new URL(location.href).searchParams.has('view')) { if (!filesPinned) showFiles(false); editor.focus(); return; }
      const url = fileURL(link.dataset.file!); url.searchParams.delete('view');
      const data: FileData = await (await request(url)).json();
      await showFile(data, link.href);
    });
  });
});
reloadButton.addEventListener('click', () => {
  if (moving || saving || loadingDrafts) return;
  if (dirty() && !discardConfirmed) {
    discardConfirmed = true; reloadButton.textContent = 'Discard edits and reload';
    report('Download your edits to keep a copy, or confirm discarding them.', true);
    return;
  }
  discardConfirmed = false; reloadButton.textContent = 'Reload from disk';
  moving = true; freeze(true); clearTimeout(autosaveTimer);
  request(fileURL()).then(response => response.json()).then(async (data: FileData) => {
    const state = owner(); await state.queue;
    if (state.draft) { await removeDraft(state.draft); state.draft = null; }
    if (recoveredDraft) { await removeDraft(recoveredDraft); recoveredDraft = null; }
    recovering = false;
    sessions.delete(active.file);
    active = {file:data.selected, saved:data.contents, revision:data.revision}; conflict = false;
    editor.setState(makeState(data.contents, data.selected));
    refreshPreview(data); report('Reloaded from disk');
  }).catch((error: unknown) => report((error instanceof Error ? error.message : String(error)), true)).finally(() => { moving = false; freeze(false); });
});
saveButton.addEventListener('click', save);
copyButton.addEventListener('click', () => download(text()));
document.querySelector<HTMLButtonElement>('#download-draft')!.addEventListener('click', () => {
  const draft = availableDrafts.find(item => item.id === draftSelect.value);
  if (draft) download(draft.content);
});
document.querySelector<HTMLButtonElement>('#recover-draft')!.addEventListener('click', () => {
  if (dirty() || moving || saving) { report('Save or download your current edits before recovering a draft.', true); return; }
  const draft = availableDrafts.find(item => item.id === draftSelect.value);
  if (!draft) return;
  recoveredDraft = draft; recovering = true; conflict = draft.revision !== active.revision && draft.content !== active.saved;
  if (draft.content !== active.saved) active.revision = draft.revision;
  editor.setState(makeState(draft.content, active.file));
  backup(); clearTimeout(autosaveTimer);
  availableDrafts = availableDrafts.filter(item => item.id !== draft.id); renderDrafts();
  report(conflict ? 'Recovered draft; file changed on disk. Download your edits or reload from disk.' : 'Recovered draft. Save when ready, or reload from disk.', true);
  editor.focus();
});
document.querySelector<HTMLButtonElement>('#discard-draft')!.addEventListener('click', async () => {
  if (moving || saving || loadingDrafts) return;
  const file = active.file;
  const draft = availableDrafts.find(item => item.id === draftSelect.value);
  if (!draft) return;
  try { await removeDraft(draft, file); if (active.file === file) { availableDrafts = availableDrafts.filter(item => item.id !== draft.id); renderDrafts(); } }
  catch (error) { backupStatus.textContent = (error instanceof Error ? error.message : String(error)); }
});
if (active.file) loadDrafts(); else { loadingDrafts = false; body.dataset.draftsReady = 'true'; }
async function checkDisk() {
  if (checking || moving || saving || loadingDrafts || recovering || !active.file || document.hidden) return;
  checking = true;
  const file = active.file, previousDoc = editor.state.doc;
  try {
    const data: FileData = await (await request(fileURL(file))).json();
    if (moving || saving || file !== active.file || previousDoc !== editor.state.doc) return;
    if (data.revision !== active.revision) {
      if (dirty()) {
        conflict = true; clearTimeout(autosaveTimer);
        report('File changed on disk. Your edits are kept here.', true);
      } else {
        rememberPosition(); sessions.delete(file);
        active = {file, saved:data.contents, revision:data.revision}; conflict = false;
        editor.setState(makeState(data.contents, file));
        editor.scrollDOM.scrollTop = position(file).scroll || 0;
        report('Updated from disk');
      }
    }
    if (!dirty() && !conflict && data.markdown) {
      // Refresh changed Markdown and linked output paths without reloading an unchanged preview.
      const signature = data.revision + ':' + data.viewURL;
      if (viewFrame.dataset.signature !== signature) {
        refreshPreview(data); viewFrame.dataset.signature = signature;
      }
    }
  } catch (error) {
    if (file !== active.file || moving || saving) return;
    if (error instanceof HTTPError && (error.code === 400 || error.code === 404)) {
      conflict = true; clearTimeout(autosaveTimer); report('File unavailable on disk. Your text is kept here.', true);
    }
  } finally { checking = false; }
}
setInterval(checkDisk, 2000);
addEventListener('focus', checkDisk);
addEventListener('beforeunload', event => {
  rememberPosition();
  if (dirty() || saving) { event.preventDefault(); event.returnValue = ''; }
});
addEventListener('popstate', async () => {
  if (restoringHistory) { restoringHistory = false; return; }
  const targetURL = location.href;
  const restore = () => {
    const targetIndex = history.state?.jadeIndex;
    // Back/Forward changes the URL before we can save. Return to the current
    // entry on failure so the address and subsequent navigation match the text.
    if (Number.isInteger(targetIndex) && targetIndex !== historyIndex) {
      restoringHistory = true;
      history.go(historyIndex - targetIndex);
    } else {
      history.replaceState({jadeIndex:historyIndex}, '', activeURL);
    }
  };
  if (moving || loadingDrafts) { restore(); return; }
  let navigating = false;
  await leave(async () => {
    if (restoringHistory || location.href !== targetURL) return;
    navigating = true; location.reload();
  });
  if (!navigating && !restoringHistory) restore();
});
form.addEventListener('submit', event => { event.preventDefault(); save(); });
document.querySelector<HTMLButtonElement>('#refresh-files')!.addEventListener('click', () => leave(async () => location.reload()));
const newFileDialog = document.querySelector<HTMLDialogElement>('#new-file-dialog')!;
const newFileForm = document.querySelector<HTMLFormElement>('#new-file-form')!;
const newFilePath = newFileForm.elements.namedItem('path') as HTMLInputElement;
const newFileControls = newFileForm.querySelectorAll<HTMLInputElement | HTMLButtonElement>('input, button');
const newFileError = document.querySelector<HTMLElement>('#new-file-error')!;
let creating = false;
document.querySelector<HTMLButtonElement>('#new-file')!.addEventListener('click', () => {
  newFileForm.reset(); newFileError.textContent = '';
  newFilePath.removeAttribute('aria-invalid');
  newFileDialog.showModal();
});
newFileDialog.addEventListener('close', () => document.querySelector<HTMLButtonElement>('#new-file')!.focus());
document.querySelector<HTMLButtonElement>('#new-file-cancel')!.addEventListener('click', () => newFileDialog.close());
newFileDialog.addEventListener('cancel', event => { if (creating) event.preventDefault(); });
newFileForm.addEventListener('submit', async event => {
  event.preventDefault();
  const path = newFilePath.value.trim();
  if (!path || creating || moving || loadingDrafts) return;
  creating = moving = true; freeze(true); newFileError.textContent = '';
  for (const control of newFileControls) control.disabled = true;
  try {
    if (!await save()) throw new Error('Save or download your current edits before creating another file.');
    const data = new FormData(); data.set('jade', body.dataset.jade!); data.set('path', path);
    const response = await request('/new', {method:'POST', body:data});
    location.href = await response.text();
  } catch (error) {
    newFileError.textContent = (error instanceof Error ? error.message : String(error));
    newFilePath.setAttribute('aria-invalid', 'true');
  } finally {
    creating = moving = false; freeze(false);
    for (const control of newFileControls) control.disabled = false;
    if (newFileError.textContent) newFilePath.focus();
  }
});

async function editPreviewFile(rootFile: string) {
  const folder = body.dataset.jade === '.' ? [] : body.dataset.jade!.split('/');
  const path = rootFile.split('/');
  while (folder.length && path.length && folder[0]===path[0]) { folder.shift(); path.shift(); }
  const file = [...folder.map(()=>'..'), ...path].join('/');
  let opened = false;
  await leave(async () => {
    if (file !== active.file) {
      const url = fileURL(file); url.searchParams.delete('view');
      const data: FileData = await (await request(url)).json();
      const href = new URL(url); href.pathname = '/';
      await showFile(data, href.href);
    }
    editor.focus(); opened = true;
  });
  return opened;
}

initSearch(async (file, line) => {
  await leave(async () => {
    if (file !== active.file || new URL(activeURL).searchParams.has('view')) {
      const url = fileURL(file); url.searchParams.delete('view');
      const data: FileData = await (await request(url)).json();
      const href = new URL(url); href.pathname = '/';
      await showFile(data, href.href);
    }
    if (line) {
      const target = editor.state.doc.line(Math.min(line, editor.state.doc.lines));
      editor.dispatch({selection:{anchor:target.from, head:target.to}, effects:EditorView.scrollIntoView(target.from, {y:'center'})});
    }
    editor.focus();
  });
});
const openTerminal = initTerminals(body, document.querySelector<HTMLElement>('#terminal-status')!);
addEventListener('keydown', event => {
  if (!(event.metaKey || event.ctrlKey)) return;
  if (event.key.toLowerCase() === 's') { event.preventDefault(); if (!newFileDialog.open) save(); }
  if (event.key.toLowerCase() === 'j') { event.preventDefault(); openTerminal(); }
});

initCompanion();
initSync();
