# Engine

| File | Responsibility |
| --- | --- |
| [http.go](http.go) | HTTP routes, Markdown previews |
| [workspace.go](workspace.go) | Workspace discovery, file trees, path boundaries |
| [save.go](save.go) | Revision checks, atomic saves |
| [drafts.go](drafts.go) | Recovery storage, locking |
| [terminal.go](terminal.go) | Terminal discovery, launching |
| [server.go](server.go) | Loopback listener, shutdown |
| [web/page.html](web/page.html) | Page template |
| [web/style.css](web/style.css) | Editor layout |
| [web/preview.css](web/preview.css) | Preview styles |
| [web/editor.ts](web/editor.ts) | Editor behavior |
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

[Installation](../README.md)
