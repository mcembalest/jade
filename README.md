# JaDE

JaDE (“Just a Development Environment”) is for work, writing, visualization, & artifact publishing.

## Setup: write `jade.md`

A normal directory becomes a JaDE with `jade.md`, a Markdown file optionally declaring an output artifact & a generation command, e.g.
```markdown
# A paper

Artifact: paper.pdf
Command: typst compile paper.typ paper.pdf
```
## Visualize: JaDE UI
```sh
go run . /path/to/a/jade
```
Open `http://127.0.0.1:7333`. 

## Design principle: nested development environments

A JaDE may contain JaDEs. Files that are used by multiple inner JaDEs should live near your outer JaDE root so that multiple inner JaDEs can use them.
```
project/          
├── jade.md       summarize project, declare artifact & generation command
├── …             
├── <artifact>    the generated artifact
└── inner/        nested inner JaDE
    ├── jade.md   summarize inner project, declares inner artifact & generation command
    └── …
```
## Sources of inspiration

- Zhang, Kraska, and Khattab, [“Recursive Language Models” (2025)](https://arxiv.org/abs/2512.24601)
