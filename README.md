# JaDE

## Install

macOS · [Go 1.22+](https://go.dev/doc/install)

```sh
go install github.com/mcembalest/jade@main
export PATH="$(go env GOPATH)/bin:$PATH"
jade /path/to/project
```

## Use

Browser editor · folder homepage: `README.md`

| In JaDE | Action |
| --- | --- |
| Files | Browse files; Pin keeps the sidebar open |
| Search | Find filenames or saved text; ⌘⇧F |
| Show preview | Render Markdown; hover local links to explore further |
| Keep open · Edit · ‹ | Keep a preview, edit its file, or return to its parent |
| Saved | Edits are on disk; wait for this before stopping JaDE |

Update: reinstall.

[Development](engine/README.md) · [MNIST example](examples/mnist/README.md)
