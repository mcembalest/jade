# JaDE

JaDE (“Just a Development Environment”) is for work, writing, visualization, & artifact publishing.

## Setup: write `jade.md`

A normal directory becomes a JaDE with `jade.md` — plain Markdown, no special syntax. JaDE makes its `sh` code blocks runnable and opens its first file link in the viewer pane, e.g.

````markdown
# A paper

```sh
typst compile paper.typ paper.pdf
```

The result: [paper.pdf](paper.pdf)
````

Use `sh` fences only for commands that make sense to run in that directory; use `console` fences for instructions meant for a human elsewhere.

## Visualize: JaDE UI

Install once (requires [Go](https://go.dev/dl/)), then launch from anywhere inside a project containing a `jade.md`:

```console
go install github.com/mcembalest/jade@latest
jade
```

(`go install` places the binary in `$(go env GOPATH)/bin`, usually `~/go/bin` — ensure that is on your `PATH`.)

Open `http://127.0.0.1:7333`. The UI is a window onto one JaDE: its front page, plain-text editing, a Run button for every `sh` block in `jade.md` plus a box for one-off commands, and the linked file rendered beside its sources. `jade` finds the nearest `jade.md` at or above where you run it; pass a path (`jade ~/some/project`) to aim it elsewhere. It runs beside your editor, terminal, and Git — it does not replace them.

## Design principle: nested development environments

A JaDE may contain JaDEs. Files that are used by multiple inner JaDEs should live near your outer JaDE root so that multiple inner JaDEs can use them.

```
project/
├── jade.md       front page: prose, runnable sh blocks, links to results
├── …
├── <result>      files produced by the sh blocks
└── inner/        nested inner JaDE
    ├── jade.md   same idea, one level down
    └── …
```

## This repository

The engine itself, and a JaDE: [core.go](core.go) holds the workspace rules, [main.go](main.go) the local UI. Working on the engine, run it directly (`go run . examples/makemore`) and verify with:

```sh
go test ./...
```

- [examples/mnist/](examples/mnist/) — one problem, two languages, identical metrics
- [examples/makemore/](examples/makemore/) — learn names, generate new ones

## Sources of inspiration

- Zhang, Kraska, and Khattab, [“Recursive Language Models” (2025)](https://arxiv.org/abs/2512.24601)
