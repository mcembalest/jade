# JaDE

A quiet place for code, notes, and experiments.

Your files. Your browser. Your terminal.

## Start

With Go installed on a Mac:

```sh
go install github.com/mcembalest/jade/cmd/jade@main
jade /path/to/project
```

Put Go’s binary directory (usually `~/go/bin`) on your `PATH`. JaDE opens in your browser; no separate app or Node installation is needed. This installs the development version.

## Make something

Write Markdown, edit code, and see your work beside it. A folder with a `jade.md` becomes its own small workspace.

Explore the **[MNIST playground](examples/mnist/)**: train on your Mac, study the pictures, and explore MLX, Mojo/MAX, and browser ML.

Edits autosave. Recovery drafts stay on your Mac, and conflicting changes are kept for you to resolve. Wait for **Saved** before stopping JaDE. Use **Open terminal** for your chosen terminal, including Terminal or Ghostty; use `git` and `gh` there as usual.

## Develop

```sh
npm ci
npm run test:setup   # once: install the test browser
npm test            # build + Go tests + browser tests
go run ./cmd/jade --no-open .
```

`cmd/` launches. `engine/` runs the editor. `examples/mnist/` explores. `tests/` checks the browser workflows.

Frontend source lives in `engine/web/`; commit rebuilt `web/dist/` assets so `go install` stays self-contained. Test reports live in `.tmp/` (`npm run test:report`). Each `README.md` points to its folder's `jade.md`—one introduction to maintain.
