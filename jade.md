# JaDE

JaDE (Just a Development Environment) is for thinking, working, coding, visualizing, writing, and publishing.

## Install (requires Go, uv, and Apple Command Line Tools)

```sh
uv run native/install.py
```

Run `jade` to keep JaDE in the macOS menu bar, then choose **Open folder…** for any repository or directory. Run `jade /path/to/repo` to open one directly; `jade.md` is optional.

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

Each inner JaDE owns its working directory. The Go engine handles files, terminal launching, and the local HTTP interface; the native macOS shell hosts that interface. There are no embedded terminal dependencies.

The header’s terminal dropdown lists installed apps (including Terminal and Ghostty). **Open terminal** (⌘J) opens the selected app at the active workspace directory. The choice is saved across app restarts and workspaces. Without a saved choice, JaDE detects an installed alternative, or uses Terminal. `JADE_TERMINAL` overrides the choice with an app name or path, such as `Ghostty` or `/Applications/Ghostty.app`; it is never interpreted as a shell command. If launching the selected app fails, JaDE falls back to macOS Terminal.

Use `git` and `gh` in your terminal for version control and GitHub workflows, including when working with agents.

## Direction

Focus on reliable text/file editing, nested project context, and rendered outputs. Prefer established dependencies when they reduce the behavior JaDE must maintain.

Specialized subprojects with their own `jade.md` can eventually prepare arXiv-ready paper artifacts or Substack-ready Markdown. The long-term paper-writing goal is to make Overleaf unnecessary. These are future workflows, not built-in publishing integrations; JaDE currently neither builds submission packages nor publishes to those services. Richer integrations and IDE controls can wait until the core editing experience is dependable.

## Inspiration

Zhang, Kraska, and Khattab, “Recursive Language Models” (2025): https://arxiv.org/abs/2512.24601
