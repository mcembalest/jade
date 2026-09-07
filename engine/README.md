# Engine

| File | Responsibility |
| --- | --- |
| [http.go](http.go) | HTTP routes, Markdown previews |
| [workspace.go](workspace.go) | Workspace discovery, file trees, path boundaries |
| [save.go](save.go) | Revision checks, atomic saves |
| [drafts.go](drafts.go) | Recovery storage, locking |
| [terminal.go](terminal.go) | Terminal discovery, launching |
| [server.go](server.go) | Loopback listener, shutdown |
| [projects.go](projects.go) | Explicit desktop project opening, immutable project servers, recent projects |
| [session.go](session.go) | Bounded editor navigation preferences, separate from documents and drafts |
| [web/page.html](web/page.html) | Page template |
| [web/style.css](web/style.css) | Editor layout |
| [web/preview.css](web/preview.css) | Preview styles |
| [web/editor.ts](web/editor.ts) | Editor behavior |
| [web/file-filter.ts](web/file-filter.ts) | Filename/path filtering in the existing file tree |
| [web/projects.ts](web/projects.ts) | Project chooser using existing save-before-navigation checks |
| [web/session.ts](web/session.ts) | Cursor, scroll and sidebar restoration; coalesced preference writes |
| [web/writing.ts](web/writing.ts) | Markdown link insertion using ordinary editor transactions |
| [web/preview.ts](web/preview.ts) | Recursive link previews, keeping, moving, editing |
| [web/terminal.ts](web/terminal.ts) | Terminal controls |
| [web/build.mjs](web/build.mjs) | Frontend build → `web/dist/` |

## Development

Node + Go · source: `engine/web/` · run from repository root

```sh
cd engine/web
npm ci
npm run test:setup
npm test
```

| Command | Action |
| --- | --- |
| `npm run build` | TypeScript check and frontend build |
| `npm test` | Build, Go race tests, Chromium and WebKit regression tests |
| `npm run test:report` | Open the browser test report |
| `npm run test:measure` | Informational latency measurements |
| `go -C ../.. run . --no-open .` | Start without opening a browser |
| `npx playwright test tests/e2e/visual.spec.ts --update-snapshots` | Refresh visual baselines |

| Development | Details |
| --- | --- |
| Commit with source | Rebuilt `web/dist/`; visually reviewed screenshot baselines |
| Visual baselines | macOS · Chromium + WebKit · 2× · 390–1440 px |
| Measurements | 50 KB / 500 KB / 4.5 MB · open, input-to-render, search · 3 trials |
| Results | `.tmp/measurements/**/measurements.json`; machine-dependent |
| Untested | OS clipboard, native IME, screen readers, editor comparisons |

| Preview / search | Behavior |
| --- | --- |
| Formats | Markdown, text/code, images, PDF, folders |
| Hover / click | Temporary / kept preview |
| Back / Escape | Parent preview; kept children persist |
| External links | Browser tab |
| Markdown | Saved contents |
| Homepage | `README.md`, case-insensitive |
| File access | Launched folder, including parent/sibling paths within it |
| Search | Case-insensitive literal; editable text; 100 results / 32 MB / 2 s; partial results labeled |
| Files filter | Case-insensitive filename/path match in the loaded tree; reveals nested matches; Enter opens the first match through normal save checks; Escape or × clears and restores folder expansion; Refresh files picks up new files |

## Desktop projects and restoration

**Projects** accepts an absolute folder path or `~/…` and remembers up to 12 recent projects. The current file must save before navigation. Each opened root gets its own immutable loopback listener; old tabs keep their original root. Reopening a root reuses its listener, and stopping the parent process drains child servers too. Child project servers are editor-only: they do not start Notes sync or change phone/cloud permissions. The original launched server retains its existing sync behavior.

Editor preferences live under the operating system's user configuration directory in `JaDE/editor-sessions/`; recent project paths live in `JaDE/recent-projects.json`. Sessions are keyed by canonical launched root and workspace and contain file names, cursor/scroll positions and sidebar state, never document text. Reopening a project restores its last file; an explicit file URL or CLI file argument wins. Missing remembered files fall back to the default file with a notice. Plain `jade` continues to use the current directory.

Preference writes are coalesced and flushed before navigation. A failure is reported separately from document saving and does not block working. These preferences are not recovery storage; draft and save mechanisms remain in their existing modules.

**Link** (Markdown only, ⌘/Ctrl+K) wraps the selection or inserts at the cursor. The dialog accepts web URLs and relative file links, preserves selection when cancelled, and inserts a single undoable edit through normal autosave. The native mobile source editor is documented [separately](../mobile/README.md#source-editing).

[Installation](../README.md)
