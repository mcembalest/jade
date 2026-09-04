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
const viewFrame = document.querySelector('#view-frame');
const readOnly = new Compartment();
const sessions = new Map();
const initial = document.querySelector('#initial-content').value.replace(/\r\n/g, '\n');
let active = {file: body.dataset.file, revision: body.dataset.revision, saved: body.dataset.crlf === 'true' ? initial.replace(/\n/g, '\r\n') : initial};
let saving = null, moving = false, checking = false, conflict = false, autosaveTimer = 0;
let editor;
let discardConfirmed = false;
const text = () => editor.state.sliceDoc().replace(/\r\n/g, '\n').replace(/\n/g, active.saved.includes('\r\n') ? '\r\n' : '\n');
const dirty = () => text() !== active.saved;
const cacheKey = file => 'jade-position:' + body.dataset.jade + ':' + file;

function report(message, problem = false) {
  status.textContent = message;
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
    EditorView.contentAttributes.of({'aria-label':'Editor', spellcheck:'false'}),
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
      if (conflict) { report('File changed on disk. Your edits are kept here.', true); return; }
      report(dirty() ? 'Edited' : 'Saved');
      if (dirty()) autosaveTimer = setTimeout(save, 800);
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
  return url;
}
function refreshPreview(data) {
  const visible = data.isJade;
  document.querySelector('#document').classList.toggle('jade-open', visible);
  document.querySelector('#resolved').hidden = !visible;
  if (visible) {
    viewFrame.src = data.viewURL;
    document.querySelector('#view-name').textContent = data.view || data.title;
  }
}
async function save() {
  clearTimeout(autosaveTimer);
  if (saving) return saving;
  if (!dirty()) return true;
  if (conflict) return false;
  saving = (async () => {
    try {
      while (dirty()) {
        const contents = text();
        report('Saving…');
        // URL encoding preserves LF/CRLF; multipart form serialization normalizes newlines.
        const data = new URLSearchParams();
        data.set('jade', body.dataset.jade); data.set('file', active.file);
        data.set('content', contents); data.set('revision', active.revision);
        const result = await (await request('/save', {method:'POST', body:data})).json();
        active.saved = contents; active.revision = result.revision;
      }
      report('Saved'); rememberPosition();
      return true;
    } catch (error) {
      conflict = error.code === 409;
      report(conflict ? 'File changed or was removed on disk. Your edits are kept here.' : 'Not saved: ' + error.message, true);
      return false;
    }
  })();
  try { return await saving; } finally { saving = null; }
}
async function leave(action) {
  if (moving) return;
  moving = true; freeze(true);
  try { if (await save()) { rememberPosition(); await action(); } }
  catch (error) { report(error.message, true); }
  finally { moving = false; freeze(false); }
}
window.__jadeFlush = async () => {
  if (moving) return false;
  moving = true; freeze(true);
  try { return await save(); } finally { moving = false; freeze(false); }
};
function showFile(data, href, link) {
  sessions.set(active.file, {state:editor.state, revision:active.revision, scroll:editor.scrollDOM.scrollTop});
  const cached = sessions.get(data.selected);
  active = {file:data.selected, saved:data.contents, revision:data.revision};
  conflict = false;
  editor.setState(cached?.revision === data.revision ? cached.state : makeState(data.contents, data.selected));
  editor.scrollDOM.scrollTop = cached?.revision === data.revision ? cached.scroll : position(data.selected).scroll || 0;
  body.dataset.file = data.selected;
  document.querySelector('#file-name').textContent = data.selected;
  document.querySelectorAll('.file-link').forEach(node => node.classList.toggle('active', node === link));
  if (href) history.pushState({}, '', href);
  refreshPreview(data); report('Saved'); editor.focus();
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
      if (link.dataset.file === active.file) return;
      const data = await (await request(fileURL(link.dataset.file))).json();
      showFile(data, link.href, link);
    });
  });
});
reloadButton.addEventListener('click', () => {
  if (moving || saving) return;
  if (dirty() && !discardConfirmed) {
    discardConfirmed = true; reloadButton.textContent = 'Discard edits and reload';
    report('Download your edits to keep a copy, or confirm discarding them.', true);
    return;
  }
  discardConfirmed = false; reloadButton.textContent = 'Reload from disk';
  moving = true; freeze(true); clearTimeout(autosaveTimer);
  request(fileURL()).then(response => response.json()).then(data => {
    sessions.delete(active.file);
    active = {file:data.selected, saved:data.contents, revision:data.revision}; conflict = false;
    editor.setState(makeState(data.contents, data.selected));
    refreshPreview(data); report('Reloaded from disk');
  }).catch(error => report(error.message, true)).finally(() => { moving = false; freeze(false); });
});
copyButton.addEventListener('click', () => {
  const link = document.createElement('a');
  link.href = URL.createObjectURL(new Blob([text()], {type:'text/plain;charset=utf-8'}));
  link.download = active.file.split('/').pop() || 'notes.txt'; link.click();
  setTimeout(() => URL.revokeObjectURL(link.href), 1000);
});
async function checkDisk() {
  if (checking || moving || saving || !active.file || document.hidden) return;
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
document.querySelector('#new-file').addEventListener('click', () => {
  newFileForm.reset(); newFileDialog.showModal();
});
document.querySelector('#new-file-cancel').addEventListener('click', () => newFileDialog.close());
newFileForm.addEventListener('submit', event => {
  event.preventDefault();
  const path = new FormData(newFileForm).get('path').trim();
  if (!path) return;
  newFileDialog.close();
  leave(async () => {
    const data = new FormData(); data.set('jade', body.dataset.jade); data.set('path', path);
    const response = await request('/new', {method:'POST', body:data});
    location.href = await response.text();
  });
});
const openTerminal = initTerminals(body, document.querySelector('#terminal-status'));
addEventListener('keydown', event => {
  if (!(event.metaKey || event.ctrlKey)) return;
  if (event.key.toLowerCase() === 's') { event.preventDefault(); save(); }
  if (event.key.toLowerCase() === 'j') { event.preventDefault(); openTerminal(); }
});
