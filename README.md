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

### Editing and companion setup

Use ⌘/Ctrl+P to filter and open files, ⌘/Ctrl+F for find/replace, ⌘/Ctrl+S to save, and ⌘/Ctrl+J to open a real terminal in the current project. Terminal opening saves the current file first and stops on unresolved save conflicts. On macOS, Ghostty 1.3+ and Terminal use their native scripting interfaces; macOS may require Automation permission. A launch error is shown rather than silently switching applications.

**Character, research & history** configures your companion from scratch. Name, character, research brief, memory, daily time and image are shared with the phone. Read [daily research setup](sync/cloudflare/RESEARCH.md) for the server runtime, sign-in, durable history and current limits.
