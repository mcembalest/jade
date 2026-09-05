# JaDE engine

The Go engine is an inner JaDE. Its implementation files open like an ordinary IDE; selecting this `jade.md` resolves [main.go](main.go) beside the source.

The filesystem boundary and safety rules live in [core.go](core.go), the editor and HTTP interface in [main.go](main.go), and external terminal discovery and launching in [terminal.go](terminal.go). Git and publishing workflows use external tools.

## Run

From this inner JaDE:

```sh
jade
```

The self-contained launcher opens this workspace in a browser.

## Verify

```sh
go test ./...
```

## Install

```sh
go install github.com/mcembalest/jade/cmd/jade@main
```

The module remains rooted one directory above so this repository can contain multiple inner JaDEs without duplicating dependency state. Go discovers that parent `go.mod` automatically.
