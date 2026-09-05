# JaDE

## Install

macOS · Go

```sh
go install github.com/mcembalest/jade/cmd/jade@main
export PATH="$(go env GOPATH)/bin:$PATH"
jade /path/to/project
```

## Reference

| Item | Location / behavior |
| --- | --- |
| UI | Browser |
| Workspace marker | `jade.md` |
| Saving | Autosave; wait for `Saved` before stopping |
| Terminal | Terminal / Ghostty |
| Example | [MNIST: MLX, Mojo/MAX, jax-js](examples/mnist/) |
| README | Symlink to local `jade.md` |

## Development

```sh
npm ci
npm run test:setup
npm test
go run ./cmd/jade --no-open .
```

| Path | Contents |
| --- | --- |
| `cmd/jade/` | CLI |
| `engine/` | HTTP server, filesystem, recovery |
| `engine/web/` | Frontend source |
| `engine/web/dist/` | Committed frontend build |
| `examples/mnist/` | ML examples |
| `tests/` | Browser tests |
| `.tmp/` | Test output |

```sh
npm run build
npm run test:report
npx playwright test tests/e2e/visual.spec.ts --update-snapshots
```

Visual baselines: macOS · Chromium + WebKit · 2× density · 390–1440 px.

Commit rebuilt assets and reviewed screenshot baselines with source changes.
