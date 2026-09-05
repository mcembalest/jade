# JaDE

JaDE (Just a Development Environment) is for thinking, working, coding, visualizing, writing, and publishing.

## Install

JaDE currently targets macOS. The browser editor and Go engine are bundled into one executable; running it needs no Node or separate app installation.

With Go installed:

```sh
go install github.com/mcembalest/jade/cmd/jade@main
jade /path/to/project
```

Make sure Go's binary directory (normally `~/go/bin`) is on your `PATH`.

Or use the development Homebrew formula:

```sh
brew tap mcembalest/jade https://github.com/mcembalest/jade
brew install --HEAD mcembalest/jade/jade
```

These install the current development branch; there is no tagged stable release yet.

`jade` opens the current folder in your browser. It listens only on loopback, chooses an available port, and keeps running in the launching terminal. Save your edits before stopping it with Ctrl+C. Use `jade --no-open [folder]` to print the address without opening a browser, or `--address 127.0.0.1:7333` for a fixed port. `jade.md` is optional.

## Editing

CodeMirror provides undo/redo, indentation, search/replace (⌘F), and highlighting for Markdown, Python, JavaScript/TypeScript, and Go. Undo history is kept separately for each file while the page is open.

JaDE autosaves after a short pause and saves before navigating. Saves check the disk revision and replace the file atomically. If another tool changes or deletes a file, JaDE retains conflicting local edits for you to download or discard explicitly. Unmodified open files refresh from disk; use **Refresh files** to update the file tree after external additions, renames, or deletions.

Unsaved text is held in the open editor, not a crash-recovery store. Browser closing warns about unsaved changes; abrupt process or browser termination can still lose them.

## Development

```sh
npm ci
npm run build
go test -race ./...
go run ./cmd/jade --no-open .
```

### Automated local checks

One-time browser-test setup after `npm ci`:

```sh
npm run test:setup
```

Then run everything with:

```sh
npm test
```

This builds the bundled editor, runs Go's race-enabled tests, and drives a headless browser against fresh temporary workspace copies. Editing mechanics run against both JaDE and a minimal CodeMirror baseline pinned by the lockfile. JaDE-specific checks cover saving, navigation, external changes, conflict recovery, line endings, file creation, and the minimum window layout. No installed desktop editors, personal files, external services, or desktop keystrokes are involved.

On failure, screenshots, videos, browser traces, and engine logs are kept under `.tmp/test-results`. Run `npm run test:report` for the HTML report. It does not claim crash recovery or Sublime/Obsidian feature parity.

Frontend sources live in `engine/web`. Commit the generated `editor.bundle.js` and third-party notices after frontend changes so `go install` works without a JavaScript build step. Runtime assets are served locally; dependency licenses are available at `/licenses.txt`.

## Design idea

A JaDE is just a development environment, and working in it should feel like one, but a little better.

Any inner JaDEs can be included directly in the filesystem as subfolders with a `jade.md`.

```text
repository/
├── jade.md       outer intent; no child registry
├── shared files
├── inner-a/
│   ├── jade.md   automatically discovered
│   └── work
└── inner-b/
    ├── jade.md   automatically discovered
    └── work
```

Each inner JaDE owns its working directory. The Go engine handles files, terminal launching, and the local HTTP interface; your browser hosts that interface. There are no embedded terminal dependencies.

The header’s terminal dropdown lists installed apps (including Terminal and Ghostty). **Open terminal** (⌘J) opens the selected app at the active workspace directory. The choice is saved across app restarts and workspaces. Without a saved choice, JaDE detects an installed alternative, or uses Terminal. `JADE_TERMINAL` overrides the choice with an app name or path, such as `Ghostty` or `/Applications/Ghostty.app`; it is never interpreted as a shell command. If launching the selected app fails, JaDE falls back to macOS Terminal.

Use `git` and `gh` in your terminal for version control and GitHub workflows, including when working with agents.

## Direction

Focus on reliable text/file editing, nested project context, and rendered outputs. Prefer established dependencies when they reduce the behavior JaDE must maintain.

Specialized subprojects with their own `jade.md` can eventually prepare arXiv-ready paper artifacts or Substack-ready Markdown. The long-term paper-writing goal is to make Overleaf unnecessary. These are future workflows, not built-in publishing integrations; JaDE currently neither builds submission packages nor publishes to those services. Richer integrations and IDE controls can wait until the core editing experience is dependable.

## Inspiration

Zhang, Kraska, and Khattab, “Recursive Language Models” (2025): https://arxiv.org/abs/2512.24601
