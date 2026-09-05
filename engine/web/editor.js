import { basicSetup } from 'codemirror';
import { EditorState, Compartment } from '@codemirror/state';
import { EditorView, keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';
import { markdown } from '@codemirror/lang-markdown';
import { python } from '@codemirror/lang-python';
import { javascript } from '@codemirror/lang-javascript';
import { go } from '@codemirror/lang-go';
import { initTerminals } from './terminal.js';

const body = document.body;
const form = document.querySelector('#editor-form');
const status = document.querySelector('#save-status');
const reloadButton = document.querySelector('#reload-file');
const copyButton = document.querySelector('#save-copy');
const saveButton = document.querySelector('#save-now');
const previewButton = document.querySelector('#preview-toggle');
const compact = matchMedia('(max-width:700px)');
let previewOpen = !compact.matches;
const filesButton = document.querySelector('#files-toggle');
function showFiles(open) {
  const explorer = document.querySelector('#file-explorer');
  if (!open && explorer.contains(document.activeElement)) filesButton.focus();
  explorer.hidden = !open;
  document.querySelector('#shell').classList.toggle('files-hidden', !open);
  filesButton.setAttribute('aria-expanded', String(open));
}
filesButton.addEventListener('click', () => showFiles(filesButton.getAttribute('aria-expanded') !== 'true'));
showFiles(!compact.matches);
compact.addEventListener('change', () => showFiles(!compact.matches));
function showPreview() {
  const visible = !previewButton.hidden && previewOpen;
  document.querySelector('#document').classList.toggle('jade-open', visible);
  document.querySelector('#resolved').hidden = !visible;
  previewButton.setAttribute('aria-pressed', String(visible));
  previewButton.textContent = visible ? 'Hide preview' : 'Show preview';
}
previewButton.addEventListener('click', () => { previewOpen = !previewOpen; showPreview(); });
showPreview();
const viewFrame = document.querySelector('#view-frame');
const readOnly = new Compartment();
const sessions = new Map();
const initial = document.querySelector('#initial-content').value.replace(/\r\n/g, '\n');
let active = {file: body.dataset.file, revision: body.dataset.revision, saved: body.dataset.crlf === 'true' ? initial.replace(/\n/g, '\r\n') : initial};
let saving = null, moving = false, checking = false, conflict = false, autosaveTimer = 0;
let editor;
let discardConfirmed = false, recovering = false, loadingDrafts = true;
const draftPanel = document.querySelector('#draft-recovery');
const draftSelect = document.querySelector('#draft-select');
const backupStatus = document.createElement('div');
backupStatus.id = 'draft-status'; backupStatus.setAttribute('role', 'status');
form.insertBefore(backupStatus, document.querySelector('#editor'));
const draftOwners = new Map();
let availableDrafts = [], recoveredDraft = null;
function owner() {
  if (!draftOwners.has(active.file)) draftOwners.set(active.file, {id:crypto.randomUUID(), queue:Promise.resolve(), version:0, draft:null});
  return draftOwners.get(active.file);
}
function draftURL(file = active.file) {
  const url = fileURL(file); url.pathname = '/drafts'; return url;
}
function backup() {
  const state = owner(), version = ++state.version;
  const file = active.file, content = text(), revision = active.revision;
  state.queue = state.queue.catch(() => {}).then(async () => {
    if (version !== state.version) return;
    const data = new URLSearchParams({jade:body.dataset.jade,file,id:state.id,content,revision});
    state.draft = await (await request('/drafts', {method:'POST',body:data})).json();
    if (active.file === file) backupStatus.textContent = '';
  }).catch(error => { backupStatus.textContent = 'Recovery backup unavailable: ' + error.message; });
  return state.queue;
}
async function removeDraft(draft, file = active.file) {
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
    const data = await (await request(draftURL())).json();
    availableDrafts = data.drafts.filter(draft => draft.id !== owner().id);
    renderDrafts();
  } catch (error) { backupStatus.textContent = 'Cannot load recovery drafts: ' + error.message; }
  finally { loadingDrafts = false; freeze(false); body.dataset.draftsReady = 'true'; }
}
function download(contents) {
  const link = document.createElement('a');
  link.href = URL.createObjectURL(new Blob([contents], {type:'text/plain;charset=utf-8'}));
  link.download = active.file.split('/').pop() || 'notes.txt'; link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 1000);
}

const text = () => editor.state.sliceDoc().replace(/\r\n/g, '\n').replace(/\n/g, (active.lineEnding ?? (active.saved.includes('\r\n') ? '\r\n' : '\n')));
const dirty = () => text() !== active.saved;
const cacheKey = file => 'jade-position:' + body.dataset.jade + ':' + file;

function report(message, problem = false) {
  status.textContent = message;
  status.parentElement.dataset.problem = String(problem);
  saveButton.hidden = !problem || conflict;
  copyButton.hidden = !problem;
  reloadButton.hidden = !problem;
}
function rememberPosition() {
  try { sessionStorage.setItem(cacheKey(active.file), JSON.stringify({head:editor.state.selection.main.head, scroll:editor.scrollDOM.scrollTop})); } catch (_) {}
}
function position(file) {
  try { return JSON.parse(sessionStorage.getItem(cacheKey(file))) || {}; } catch (_) { return {}; }
}
function language(file) {
  if (/\.md$/i.test(file)) return markdown();
  if (/\.py$/i.test(file)) return python();
  if (/\.[cm]?[jt]sx?$/i.test(file)) return javascript({typescript:/\.tsx?$/i.test(file), jsx:/x$/i.test(file)});
  if (/\.go$/i.test(file)) return go();
  return [];
}
function makeState(contents, file) {
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
      autosaveTimer = setTimeout(save, 800);
    }),
  ]});
}
editor = new EditorView({state:makeState(active.saved, active.file), parent:document.querySelector('#editor')});
editor.scrollDOM.scrollTop = position(active.file).scroll || 0;
editor.focus();
function freeze(value) {
  editor.dispatch({effects:readOnly.reconfigure([EditorState.readOnly.of(value || !active.file), EditorView.editable.of(!value && !!active.file)])});
}
async function request(url, options) {
  const response = await fetch(url, {...options, signal:AbortSignal.timeout(10000)});
  if (!response.ok) {
    const error = new Error((await response.text()).trim());
    error.code = response.status;
    throw error;
  }
  return response;
}
function fileURL(file = active.file) {
  const url = new URL('/file', location.origin);
  url.searchParams.set('jade', body.dataset.jade); url.searchParams.set('file', file);
  const view = new URL(location.href).searchParams.get('view');
  if (file === active.file && view) url.searchParams.set('view', view);
  return url;
}
function refreshPreview(data) {
  document.querySelector('.project').textContent = data.title;
  document.querySelector('.project').title = data.title;
  document.title = data.title + ' · JaDE';
  const visible = data.isJade;
  previewButton.hidden = !visible;
  showPreview();
  if (visible) {
    viewFrame.src = data.viewURL;
    document.querySelector('#view-name').textContent = data.view ? 'Preview · ' + data.view : 'Preview';
    document.querySelector('#view-name').title = data.view || data.title;
  }
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
          data.set('jade', body.dataset.jade); data.set('file', active.file);
          data.set('content', contents); data.set('revision', active.revision);
          const result = await (await request('/save', {method:'POST', body:data})).json();
          active.saved = contents; active.revision = result.revision;
        }
        try { await clearSavedDrafts(); }
        catch (error) { backupStatus.textContent = 'Saved; recovery draft retained: ' + error.message; }
        if (!dirty()) break;
      }
      report('Saved'); rememberPosition();
      return true;
    } catch (error) {
      conflict = error.code === 409;
      report(conflict ? 'File changed or was removed on disk. Your edits are kept here.' : 'Not saved: ' + error.message, true);
      return false;
    }
  })();
  try { return await saving; } finally { saving = null; checkDisk(); }
}
async function leave(action) {
  if (moving || loadingDrafts) return;
  moving = true; freeze(true);
  try { if (await save()) { rememberPosition(); await action(); } }
  catch (error) { report(error.message, true); }
  finally { moving = false; freeze(false); }
}
async function showFile(data, href, link) {
  sessions.set(active.file, {state:editor.state, revision:active.revision, scroll:editor.scrollDOM.scrollTop});
  const cached = sessions.get(data.selected);
  active = {file:data.selected, saved:data.contents, revision:data.revision};
  conflict = false; recovering = false; recoveredDraft = null;
  editor.setState(cached?.revision === data.revision ? cached.state : makeState(data.contents, data.selected));
  editor.scrollDOM.scrollTop = cached?.revision === data.revision ? cached.scroll : position(data.selected).scroll || 0;
  body.dataset.file = data.selected;
  document.querySelector('#file-name').textContent = data.selected || 'No file selected';
  document.querySelector('#file-name').title = data.selected;
  document.querySelector('#empty-editor').hidden = !!data.selected;
  document.querySelectorAll('.file-link').forEach(node => node.classList.toggle('active', node === link));
  if (href) history.pushState({}, '', href);
  refreshPreview(data); report('Saved'); await loadDrafts();
  if (compact.matches) showFiles(false);
  editor.focus();
}
document.querySelector('#workspace-root')?.addEventListener('click', event => {
  event.preventDefault(); leave(async () => { location.href = '/'; });
});
document.querySelectorAll('.file-link').forEach(link => {
  link.classList.toggle('active', link.dataset.file === active.file && link.dataset.jade === body.dataset.jade);
  link.addEventListener('click', event => {
    event.preventDefault();
    leave(async () => {
      if (link.dataset.jade !== body.dataset.jade) { location.href = link.href; return; }
      if (link.dataset.file === active.file && !new URL(location.href).searchParams.has('view')) return;
      const url = fileURL(link.dataset.file); url.searchParams.delete('view');
      const data = await (await request(url)).json();
      await showFile(data, link.href, link);
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
  request(fileURL()).then(response => response.json()).then(async data => {
    const state = owner(); await state.queue;
    if (state.draft) { await removeDraft(state.draft); state.draft = null; }
    if (recoveredDraft) { await removeDraft(recoveredDraft); recoveredDraft = null; }
    recovering = false;
    sessions.delete(active.file);
    active = {file:data.selected, saved:data.contents, revision:data.revision}; conflict = false;
    editor.setState(makeState(data.contents, data.selected));
    refreshPreview(data); report('Reloaded from disk');
  }).catch(error => report(error.message, true)).finally(() => { moving = false; freeze(false); });
});
saveButton.addEventListener('click', save);
copyButton.addEventListener('click', () => download(text()));
document.querySelector('#download-draft').addEventListener('click', () => {
  const draft = availableDrafts.find(item => item.id === draftSelect.value);
  if (draft) download(draft.content);
});
document.querySelector('#recover-draft').addEventListener('click', () => {
  if (dirty() || moving || saving) { report('Save or download your current edits before recovering a draft.', true); return; }
  const draft = availableDrafts.find(item => item.id === draftSelect.value);
  if (!draft) return;
  recoveredDraft = draft; recovering = true; conflict = draft.revision !== active.revision && draft.content !== active.saved;
  if (draft.content !== active.saved) active.revision = draft.revision;
  active.lineEnding = draft.content.includes('\r\n') ? '\r\n' : '\n';
  editor.setState(makeState(draft.content, active.file));
  backup(); clearTimeout(autosaveTimer);
  availableDrafts = availableDrafts.filter(item => item.id !== draft.id); renderDrafts();
  report(conflict ? 'Recovered draft; file changed on disk. Download your edits or reload from disk.' : 'Recovered draft. Save when ready, or reload from disk.', true);
  editor.focus();
});
document.querySelector('#discard-draft').addEventListener('click', async () => {
  if (moving || saving || loadingDrafts) return;
  const file = active.file;
  const draft = availableDrafts.find(item => item.id === draftSelect.value);
  if (!draft) return;
  try { await removeDraft(draft, file); if (active.file === file) { availableDrafts = availableDrafts.filter(item => item.id !== draft.id); renderDrafts(); } }
  catch (error) { backupStatus.textContent = error.message; }
});
if (active.file) loadDrafts(); else { loadingDrafts = false; body.dataset.draftsReady = 'true'; }
async function checkDisk() {
  if (checking || moving || saving || loadingDrafts || recovering || !active.file || document.hidden) return;
  checking = true;
  const file = active.file, previousDoc = editor.state.doc;
  try {
    const data = await (await request(fileURL(file))).json();
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
    if (!dirty() && !conflict && data.isJade) {
      // Refresh changed Markdown and linked output paths without reloading an unchanged preview.
      const signature = data.revision + ':' + data.viewURL;
      if (viewFrame.dataset.signature !== signature) {
        refreshPreview(data); viewFrame.dataset.signature = signature;
      }
    }
  } catch (error) {
    if (file !== active.file || moving || saving) return;
    if (error.code === 400 || error.code === 404) {
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
addEventListener('popstate', () => leave(async () => location.reload()));
form.addEventListener('submit', event => { event.preventDefault(); save(); });
document.querySelector('#refresh-files').addEventListener('click', () => leave(async () => location.reload()));
const newFileDialog = document.querySelector('#new-file-dialog');
const newFileForm = document.querySelector('#new-file-form');
const newFileError = document.querySelector('#new-file-error');
let creating = false;
document.querySelector('#new-file').addEventListener('click', () => {
  newFileForm.reset(); newFileError.textContent = '';
  newFileForm.elements.path.removeAttribute('aria-invalid');
  newFileDialog.showModal();
});
newFileDialog.addEventListener('close', () => document.querySelector('#new-file').focus());
document.querySelector('#new-file-cancel').addEventListener('click', () => newFileDialog.close());
newFileDialog.addEventListener('cancel', event => { if (creating) event.preventDefault(); });
newFileForm.addEventListener('submit', async event => {
  event.preventDefault();
  const path = newFileForm.elements.path.value.trim();
  if (!path || creating || moving || loadingDrafts) return;
  creating = moving = true; freeze(true); newFileError.textContent = '';
  for (const control of newFileForm.elements) control.disabled = true;
  try {
    if (!await save()) throw new Error('Save or download your current edits before creating another file.');
    const data = new FormData(); data.set('jade', body.dataset.jade); data.set('path', path);
    const response = await request('/new', {method:'POST', body:data});
    location.href = await response.text();
  } catch (error) {
    newFileError.textContent = error.message;
    newFileForm.elements.path.setAttribute('aria-invalid', 'true');
  } finally {
    creating = moving = false; freeze(false);
    for (const control of newFileForm.elements) control.disabled = false;
    if (newFileError.textContent) newFileForm.elements.path.focus();
  }
});
const openTerminal = initTerminals(body, document.querySelector('#terminal-status'));
addEventListener('keydown', event => {
  if (!(event.metaKey || event.ctrlKey)) return;
  if (event.key.toLowerCase() === 's') { event.preventDefault(); if (!newFileDialog.open) save(); }
  if (event.key.toLowerCase() === 'j') { event.preventDefault(); openTerminal(); }
});
