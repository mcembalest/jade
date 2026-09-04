# JaDE engine

The Go engine is an inner JaDE. Its implementation files open like an ordinary IDE; selecting this `jade.md` resolves [main.go](main.go) beside the source.

The filesystem boundary and safety rules live in [core.go](core.go), the native terminal bridge in `../native`, and GitHub/Substack/arXiv publishing in [publish.go](publish.go).

## Run

From this inner JaDE:

```sh
jade
```

The native launcher appears as ◆ in the macOS menu bar. Choose any repository with **Open folder…**.

## Verify

```sh
go test ./...
```

## Install

```sh
uv run ../native/install.py
```

The module remains rooted one directory above so this repository can contain multiple inner JaDEs without duplicating dependency state. Go discovers that parent `go.mod` automatically.
