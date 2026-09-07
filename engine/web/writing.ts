import { isolateHistory } from '@codemirror/commands';
import { Transaction, type EditorState } from '@codemirror/state';
import type { EditorView } from '@codemirror/view';

// A presentation-only command: edits flow through the editor's normal save listener.
export function initWriting(editor: EditorView, currentFile: () => string) {
  const button = document.querySelector<HTMLButtonElement>('#insert-link')!;
  const dialog = document.querySelector<HTMLDialogElement>('#link-dialog')!;
  const form = document.querySelector<HTMLFormElement>('#link-form')!;
  const label = document.querySelector<HTMLInputElement>('#link-text')!;
  const destination = document.querySelector<HTMLInputElement>('#link-destination')!;
  const error = document.querySelector<HTMLElement>('#link-error')!;
  let target: { file: string; doc: EditorState['doc']; from: number; to: number } | null = null;

  const available = () => /\.md$/i.test(currentFile()) && !editor.state.readOnly;
  const refresh = () => { button.hidden = !/\.md$/i.test(currentFile()); };
  new MutationObserver(refresh).observe(document.body, { attributes: true, attributeFilter: ['data-file'] });
  refresh();

  function open() {
    if (!available() || dialog.open) return false;
    const { from, to } = editor.state.selection.main;
    target = { file: currentFile(), doc: editor.state.doc, from, to };
    label.value = editor.state.sliceDoc(from, to).replace(/[\r\n]+/g, ' ');
    destination.value = '';
    error.textContent = '';
    dialog.showModal();
    (label.value ? destination : label).focus();
    return true;
  }
  button.addEventListener('click', open);
  editor.dom.addEventListener('keydown', event => {
    if (editor.hasFocus && (event.metaKey || event.ctrlKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === 'k' && open()) {
      event.preventDefault();
      event.stopImmediatePropagation();
    }
  }, { capture: true });
  document.querySelector<HTMLButtonElement>('#link-cancel')!.addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', () => { target = null; editor.focus(); });
  form.addEventListener('submit', event => {
    event.preventDefault();
    if (!target || !available() || currentFile() !== target.file || editor.state.doc !== target.doc) {
      error.textContent = 'The file changed. Cancel and select the text again.';
      return;
    }
    const url = destination.value.trim();
    if (!label.value.trim() || !url) {
      error.textContent = 'Enter link text and a destination.';
      return;
    }
    if (/[\u0000-\u001f\u007f]/.test(url) || (/^[a-z][a-z\d+.-]*:/i.test(url) && !/^(https?:|mailto:)/i.test(url))) {
      error.textContent = 'Use an http, https, or mailto URL, or a relative file path.';
      destination.focus();
      return;
    }
    // Literal text remains literal Markdown; angle-delimited destinations permit parentheses.
    const text = label.value.replace(/[\\`*_[\]{}()<>#+.!|~&-]/g, '\\$&');
    const escapedURL = url.replace(/[ <>\\]/g, character => encodeURIComponent(character));
    const insert = `[${text}](<${escapedURL}>)`;
    editor.dispatch({
      changes: { from: target.from, to: target.to, insert },
      selection: { anchor: target.from + insert.length },
      annotations: [isolateHistory.of('full'), Transaction.userEvent.of('input')],
      scrollIntoView: true,
    });
    dialog.close();
  });
}
