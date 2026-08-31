# JaDE

JaDE (Just a Development Environment) is for thinking, working, coding, visualizing, and publishing.

## Install (requires [go](https://go.dev/doc/install))

```sh
go install github.com/mcembalest/jade@latest
```

Run `jade` from any terminal in a folder containing a `jade.md`; it opens the UI at `http://127.0.0.1:7333`.

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

Each inner JaDE owns its working directory and terminal context. It may also be the root of its own Git repository or worktree. Publish always uses the nearest repository rather than assuming the outer JaDE owns every change.

## Inspiration

Zhang, Kraska, and Khattab, “Recursive Language Models” (2025): https://arxiv.org/abs/2512.24601
