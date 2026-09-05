// The smallest editor baseline using the same pinned upstream packages, without JaDE's file/session code.
import { basicSetup } from 'codemirror';
import { EditorView, keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';

new EditorView({
  parent: document.querySelector('#editor'),
  extensions: [basicSetup, keymap.of([indentWithTab]), EditorView.contentAttributes.of({'aria-label':'Editor'})],
});
