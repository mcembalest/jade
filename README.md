# JaDE

## Install

macOS · [Go 1.22+](https://go.dev/doc/install)

```sh
go install github.com/mcembalest/jade@main
export PATH="$(go env GOPATH)/bin:$PATH"
jade /path/to/project
```

## Use

Browser editor · reopens the last file in each project · initial homepage: `README.md`

| In JaDE | Action |
| --- | --- |
| Files | Browse files; Pin keeps the sidebar open |
| Filter files | Match filenames or paths; Enter opens a match; Escape clears |
| Projects | Open a folder by its full Mac path or `~/…`; return through Recent projects |
| Search | Find filenames or saved text; ⌘⇧F |
| Link | Insert a Markdown link around selected text; ⌘K |
| Show preview | Render Markdown; hover local links to explore further |
| Keep open · Edit · ‹ | Keep a preview, edit its file, or return to its parent |
| Saved | Edits are on disk; wait for this before stopping JaDE |

Update: reinstall.

Opening a project restores its last file, cursor, scroll position and sidebar layout. An explicit file argument or file URL takes precedence. Running `jade` without a path still opens the current directory. Desktop project opening does not enable phone access or cloud sync.

[Development](engine/README.md) · [MNIST example](examples/mnist/README.md)
