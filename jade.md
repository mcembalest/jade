# JaDE

## Install

macOS · [Go 1.22+](https://go.dev/doc/install)

```sh
go install github.com/mcembalest/jade/cmd/jade@main
export PATH="$(go env GOPATH)/bin:$PATH"
jade /path/to/project
```

## Use

Opens your folder in the browser. `jade.md` is optional.

| In JaDE | Action |
| --- | --- |
| Files | Browse files; Pin keeps the sidebar open |
| Search | Find filenames or saved text; ⌘⇧F |
| Show preview | Render Markdown in a movable window; hover files for a temporary preview |
| Saved | Edits are on disk; wait for this before stopping JaDE |

Updates: rerun the install command.

[Development](engine/jade.md) · [MNIST example](examples/mnist/jade.md)
