# JaDE engine

The Go engine is an inner JaDE. Its implementation files open like an ordinary IDE; selecting this `jade.md` resolves [main.go](main.go) beside the source.

The filesystem boundary and safety rules live in [core.go](core.go), the PTY terminal in [terminal.go](terminal.go), and the direct GitHub/Substack publishing paths in [publish.go](publish.go).

## Run

From this inner JaDE:

```sh
go run ..
```

The launcher appears as 🐉 in the macOS menu bar and opens `http://127.0.0.1:7333`.

## Verify

```sh
go test ./...
```

## Install

```sh
go install github.com/mcembalest/jade@latest
```

The module remains rooted one directory above so this repository can contain multiple inner JaDEs without duplicating dependency state. Go discovers that parent `go.mod` automatically.
