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
| [web/editor.js](web/editor.js) | Editor behavior |
| [web/terminal.js](web/terminal.js) | Terminal controls |
| [web/build.mjs](web/build.mjs) | Frontend build → `web/dist/` |

```sh
go test ./...
```

[Installation and browser tests](../README.md)
