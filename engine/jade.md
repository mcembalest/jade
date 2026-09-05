# JaDE engine

[http.go](http.go) handles requests and Markdown previews. The rest of the engine is organized by responsibility:

- [workspace.go](workspace.go): workspace discovery, file trees, and path boundaries.
- [save.go](save.go): revision checks and atomic file saves.
- [drafts.go](drafts.go): local recovery storage and locking.
- [terminal.go](terminal.go): external terminal discovery and launching.
- [server.go](server.go): loopback listening and graceful shutdown.
- [web/page.html](web/page.html), [web/style.css](web/style.css), and [web/preview.css](web/preview.css): layout and styling.
- [web/editor.js](web/editor.js) and [web/terminal.js](web/terminal.js): browser behavior.
- [web/build.mjs](web/build.mjs): builds the committed browser assets in `web/dist/`.

Go tests sit beside the code they exercise. Run `go test ./...` here; Go finds the parent module automatically. See the root README for installation and the full browser test suite.
