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
| [web/terminal.ts](web/terminal.ts) | Terminal controls |
| [web/build.mjs](web/build.mjs) | Frontend build → `web/dist/` |

## Development

Browser source, dependencies, configuration, and tests: `engine/web/`. Development requires Node and Go; installation requires Go only.

From the repository root:

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
| `go -C ../.. run ./cmd/jade --no-open .` | Start without opening a browser |
| `npx playwright test tests/e2e/visual.spec.ts --update-snapshots` | Refresh visual baselines |

Commit rebuilt `web/dist/` assets and visually reviewed screenshot baselines with source changes.
Visual baselines: macOS · Chromium + WebKit · 2× density · 390–1440 px.

Measurements: 50 KB / 500 KB / 4.5 MB; opening, input-to-render, search; three trials per target.
Results: `.tmp/measurements/**/measurements.json`. Timings are machine-dependent.

Not covered: OS clipboard integration, native IME, screen readers. Automated Sublime/Cursor/Obsidian comparisons are deferred.

Search: case-insensitive literal matching within the current workspace; editable text only; up to 100 results, 32 MB of contents, and two seconds. Partial results are labeled.

[Installation](../jade.md)
