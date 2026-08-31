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

Each inner JaDE owns its working directory and embedded libghostty terminal context. The Go engine remains responsible for files, Git, publishing, and the local HTTP interface; the native macOS shell hosts that interface and keeps terminal sessions alive across editor reloads. Publish uses the nearest repository for GitHub, prepares Markdown for Substack, and packages TeX/PDF/ZIP papers for arXiv’s interactive submission workflow.

The header’s branch menu lists local Git branches and switches the active repository without leaving JaDE.

## Inspiration

Zhang, Kraska, and Khattab, “Recursive Language Models” (2025): https://arxiv.org/abs/2512.24601
