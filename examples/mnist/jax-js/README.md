# jax-js

![Browser training latency and inference throughput](learning.svg)

| Implementation | Setting |
| --- | --- |
| Framework | jax-js 0.1.24 + Optax 0.1.2 |
| Execution | WebAssembly or WebGPU; compiled forward/gradient; eager Optax updates |
| Model, optimizer, timing | [Shared protocol](../README.md#protocol) |
| App + comparison | [app.js](app.js) |
| Automated runner | [compare.mjs](compare.mjs) |
| Results | [measurements.json](../measurements.json): `jax-js-wasm`, `jax-js-webgpu` |

## Comparison

Node.js 22+ · Chrome · `uv`

```sh
../fetch.sh
uv run --script --locked ../compare.py --backend numpy
npm ci
npx playwright install chromium
CHROME_CHANNEL=chrome node compare.mjs
```

Chrome · WASM + WebGPU required · no fallback
Fixture: `../data/comparison.json` · NumPy logits + Adam validation
Results: `../results/jax-js-{wasm,webgpu}.json`
Bundled Chromium: `node compare.mjs` (WebGPU build-dependent)

## Interactive app

```sh
npm start
```

Local URL → train · weights import/export · inference · measurements export

App timing: independent random initialization/order; batch gathering + compilation included.

## App checks

```sh
npm test
```

WASM · 512 images · training, inference, checkpoints, reference logits
Outputs: `results/` (ignored)

## Chart

```sh
uv run --script --locked ../plot.py
```
