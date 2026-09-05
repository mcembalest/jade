// The smallest editor baseline using the same pinned upstream packages, without JaDE's file/session code.
import { basicSetup } from 'codemirror';
import { EditorView, keymap } from '@codemirror/view';
import { indentWithTab } from '@codemirror/commands';

fetch('/document').then(response => response.text()).then(doc => new EditorView({
  doc,
  parent: document.querySelector('#editor')!,
  extensions: [basicSetup, EditorView.lineWrapping, EditorView.theme({'&':{height:'700px',fontSize:'13px'},'.cm-scroller':{overflow:'auto'}}), keymap.of([indentWithTab]), EditorView.contentAttributes.of({'aria-label':'Editor'})],
}));
