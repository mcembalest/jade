import {
  init,
  defaultDevice,
  jit,
  nn,
  numpy as np,
  random,
  tree,
  valueAndGrad,
  blockUntilReady,
} from "@jax-js/jax";
import { adam, applyUpdates } from "@jax-js/optax";
const $ = (id) => document.getElementById(id);
const shapes = { w1: [784, 128], b1: [128], w2: [128, 10], b2: [10] };
const predict = jit((p, x) =>
  np.dot(nn.relu(np.dot(x, p.w1).add(p.b1)), p.w2).add(p.b2),
);
const loss = (p, x, y) =>
  nn
    .logSoftmax(predict(p, x))
    .mul(nn.oneHot(y, 10))
    .sum()
    .mul(-1 / y.shape[0]);
let params = null,
  data = null,
  busy = false,
  metrics = {};
let gpuInfo = null;
function randomSource(seed) {
  let a = seed;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
function initialize() {
  const [a, b] = random.split(random.key(0), 2);
  return {
    w1: random.normal(a, [784, 128]).mul(1 / Math.sqrt(784)),
    b1: np.zeros([128]),
    w2: random.normal(b, [128, 10]).mul(1 / Math.sqrt(128)),
    b2: np.zeros([10]),
  };
}
async function idx(name, magic) {
  const r = await fetch("/data/" + name);
  if (!r.ok) throw Error("MNIST missing. Run ../fetch.sh, then reload.");
  const b = await r.arrayBuffer(),
    v = new DataView(b),
    n = v.getUint32(4),
    offset = magic === 2051 ? 16 : 8;
  if (
    v.getUint32(0) !== magic ||
    b.byteLength !== offset + n * (magic === 2051 ? 784 : 1) ||
    (magic === 2051 && (v.getUint32(8) !== 28 || v.getUint32(12) !== 28))
  )
    throw Error("Invalid MNIST IDX file");
  return new Uint8Array(b, offset);
}
async function load() {
  if (data) return data;
  const [x, y, tx, ty] = await Promise.all([
    idx("train-images-idx3-ubyte", 2051),
    idx("train-labels-idx1-ubyte", 2049),
    idx("t10k-images-idx3-ubyte", 2051),
    idx("t10k-labels-idx1-ubyte", 2049),
  ]);
  if (
    x.length !== y.length * 784 ||
    tx.length !== ty.length * 784 ||
    y.some((v) => v > 9) ||
    ty.some((v) => v > 9)
  )
    throw Error("Invalid MNIST labels");
  return (data = { x, y, tx, ty });
}
function images(bytes) {
  return np.array(Float32Array.from(bytes, (v) => v / 255)).reshape([-1, 784]);
}
function setBusy(value) {
  busy = value;
  for (const id of ["train", "backend", "count", "epochs", "file"])
    $(id).disabled = value;
  for (const id of ["save", "benchmark", "metrics"])
    $(id).disabled = value || !params;
}
async function action(fn) {
  if (busy) return;
  setBusy(true);
  $("error").textContent = "";
  try {
    await fn();
  } catch (e) {
    $("error").textContent = String(e);
    $("status").textContent = "Stopped";
    console.error(e);
  } finally {
    setBusy(false);
  }
}
function draw() {
  const values = metrics.loss || [];
  if (!values.length) return;
  const max = Math.max(...values) * 1.05;
  const points = values
    .map(
      (v, i) =>
        `${50 + (710 * i) / Math.max(values.length - 1, 1)},${220 - (190 * v) / max}`,
    )
    .join(" ");
  $("chart").innerHTML =
    `<path d="M50 20V220H770" fill="none" stroke="#d5dfdc"/><polyline points="${points}" fill="none" stroke="#147d72" stroke-width="3"/><text x="4" y="30" fill="#627477">${max.toFixed(1)}</text><text x="25" y="220" fill="#627477">0</text><text x="50" y="249" fill="#627477">step 1</text><text x="690" y="249" fill="#627477">${values.length} steps</text>`;
}
async function accuracy(candidate = params) {
  const d = await load();
  const logits = await predict(
    tree.ref(candidate),
    images(d.tx.slice(0, 784000)),
  ).jsAsync();
  let correct = 0;
  for (let n = 0; n < logits.length; n++) {
    if (!logits[n].every(Number.isFinite))
      throw Error("Non-finite model output");
    const predicted = logits[n].indexOf(Math.max(...logits[n]));
    if (predicted === d.ty[n]) correct++;
  }
  return correct / logits.length;
}
function metadata() {
  return {
    framework: "jax-js",
    jax: "0.1.24",
    optax: "0.1.2",
    backend: $("backend").value,
    user_agent: navigator.userAgent,
    gpu: $("backend").value === "webgpu" ? gpuInfo : null,
    model: "784-128-10 ReLU",
    test: 1000,
    timing_notes:
      "Training includes first-use compilation, batch gather, optimizer and synchronization; excludes data loading, initialization, shuffling, evaluation and chart rendering. GPU driver caches may already be warm. Inference includes synchronized execution with resident input; excludes upload and readback. First timed call may follow evaluation of the same shape.",
  };
}

async function train() {
  const count = Number($("count").value),
    epochs = Number($("epochs").value);
  if (
    !Number.isInteger(count) ||
    count < 128 ||
    count > 60000 ||
    !Number.isInteger(epochs) ||
    epochs < 1 ||
    epochs > 20
  )
    throw Error("Use 128–60000 digits and 1–20 epochs.");
  $("status").textContent = "Loading MNIST…";
  const d = await load();
  $("timings").textContent = "";
  $("chart").innerHTML = "";
  tree.dispose(params);
  params = initialize();
  await blockUntilReady(params);
  const x = images(d.x.slice(0, count * 784)),
    y = np.array(Int32Array.from(d.y.slice(0, count)), { dtype: np.int32 }),
    solver = adam(0.001);
  let state = solver.init(tree.ref(params));
  metrics = {
    ...metadata(),
    seed: 0,
    initialization: "jax random.normal; not the NumPy random stream",
    shuffle: "Mulberry32 seed 0",
    train: count,
    test: 1000,
    epochs,
    batch: 128,
    learning_rate: 0.001,
    loss: [],
    epoch_accuracy: [],
    epoch_seconds: [],
  };
  const rng = randomSource(0);
  try {
    for (let epoch = 0; epoch < epochs; epoch++) {
      const order = Array.from({ length: count }, (_, i) => i);
      for (let i = count - 1; i > 0; i--) {
        const j = Math.floor(rng() * (i + 1));
        [order[i], order[j]] = [order[j], order[i]];
      }
      let elapsed = 0;
      for (let i = 0; i < count; i += 128) {
        const start = performance.now(),
          indices = np.array(order.slice(i, i + 128), { dtype: np.int32 });
        const [v, g] = valueAndGrad(loss)(
          tree.ref(params),
          x.ref.slice(indices.ref),
          y.ref.slice(indices),
        );
        let updates;
        [updates, state] = solver.update(g, state);
        params = applyUpdates(params, updates);
        await blockUntilReady(params);
        metrics.loss.push(await v.jsAsync());
        elapsed += performance.now() - start;
        if (i % 1024 === 0) {
          draw();
          $("status").textContent =
            `Epoch ${epoch + 1} of ${epochs} · ${Math.min(i + 128, count)} / ${count} digits`;
          await new Promise(requestAnimationFrame);
        }
      }
      metrics.epoch_seconds.push(elapsed / 1000);
      metrics.epoch_accuracy.push(await accuracy());
      draw();
      $("summary").textContent =
        `${(metrics.epoch_accuracy.at(-1) * 100).toFixed(1)}% test accuracy · epoch ${epoch + 1}`;
    }
    metrics.training_seconds = metrics.epoch_seconds.reduce((a, b) => a + b, 0);
    $("status").textContent =
      `Trained in ${metrics.training_seconds.toFixed(2)} s, including compilation. Ready for inference.`;
  } finally {
    tree.dispose(state);
    x.dispose();
    y.dispose();
  }
}
// An independent scalar JavaScript forward pass catches layout and reload mistakes.
async function verifyForward(candidate = params) {
  const d = await load(),
    { weights: w } = await checkpoint(candidate);
  const actual = await predict(
    tree.ref(candidate),
    images(d.tx.slice(0, 3 * 784)),
  ).jsAsync();
  let error = 0;
  for (let n = 0; n < 3; n++) {
    const hidden = w.b1.map((bias, h) => {
      let value = bias;
      for (let i = 0; i < 784; i++)
        value += (d.tx[n * 784 + i] / 255) * w.w1[i][h];
      return Math.max(0, value);
    });
    for (let k = 0; k < 10; k++) {
      let expected = w.b2[k];
      for (let h = 0; h < 128; h++) expected += hidden[h] * w.w2[h][k];
      if (!Number.isFinite(expected) || !Number.isFinite(actual[n][k]))
        throw Error("Non-finite model output");
      error = Math.max(error, Math.abs(expected - actual[n][k]));
      if (
        Math.abs(expected - actual[n][k]) >
        0.001 * (1 + Math.abs(expected))
      ) {
        throw Error("Device logits differ from the JavaScript reference");
      }
    }
  }
  return error;
}
async function benchmark() {
  if (!params) throw Error("Train or load weights first.");
  const d = await load();
  metrics.reference_max_abs_error = await verifyForward();
  metrics.inference = [];
  for (const batch of [1, 8, 32, 128, 512, 1000]) {
    const input = images(d.tx.slice(0, batch * 784));
    await blockUntilReady(input);
    const once = async () => {
      const start = performance.now(),
        out = predict(tree.ref(params), input.ref);
      await blockUntilReady(out);
      const elapsed = performance.now() - start;
      out.dispose();
      return elapsed;
    };
    try {
      const first_timed_call_ms = await once();
      for (let i = 0; i < 5; i++) await once();
      const times = [];
      for (let i = 0; i < 20; i++) times.push(await once());
      times.sort((a, b) => a - b);
      metrics.inference.push({
        batch,
        first_timed_call_ms,
        warm_median_ms: (times[9] + times[10]) / 2,
        repeats: 20,
      });
    } finally {
      input.dispose();
    }
  }
  $("timings").innerHTML =
    "<table><tr><th>Digits / call</th><th>First timed call</th><th>Warm median</th></tr>" +
    metrics.inference
      .map(
        (v) =>
          `<tr><td>${v.batch}</td><td>${v.first_timed_call_ms.toFixed(3)} ms</td><td>${v.warm_median_ms.toFixed(3)} ms</td></tr>`,
      )
      .join("") +
    "</table>";
  $("status").textContent =
    "Inference measured. Save measurements to keep this run.";
}
async function checkpoint(candidate = params) {
  const weights = {};
  for (const key of Object.keys(shapes))
    weights[key] = await candidate[key].ref.jsAsync();
  return { format: "jade-mnist-mlp-v1", weights };
}
async function restore(value) {
  if (value.format !== "jade-mnist-mlp-v1")
    throw Error("Unrecognized checkpoint format");
  const next = {};
  let referenceError, score;
  try {
    for (const [key, shape] of Object.entries(shapes)) {
      const a = value.weights?.[key];
      const valid = (v, d) =>
        d.length
          ? Array.isArray(v) &&
            v.length === d[0] &&
            v.every((w) => valid(w, d.slice(1)))
          : typeof v === "number" && Number.isFinite(Math.fround(v));
      if (!valid(a, shape))
        throw Error("Invalid weight shape or values: " + key);
      next[key] = np.array(a);
    }
    await blockUntilReady(next);
    referenceError = await verifyForward(next);
    score = await accuracy(next);
  } catch (e) {
    tree.dispose(next);
    throw e;
  }
  tree.dispose(params);
  params = next;
  $("chart").innerHTML =
    '<text x="20" y="130" fill="#627477">Reloaded weights · no training history.</text>';
  $("timings").textContent = "";
  metrics = { ...metadata(), reloaded: true };
  metrics.reference_max_abs_error = referenceError;
  metrics.accuracy = score;
  $("summary").textContent =
    `${(score * 100).toFixed(1)}% test accuracy · reloaded weights`;
  $("status").textContent = "Weights reloaded. Ready for inference.";
}
function download(name, value) {
  const url = URL.createObjectURL(
      new Blob([JSON.stringify(value)], { type: "application/json" }),
    ),
    a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
$("train").onclick = () => action(train);
$("benchmark").onclick = () => action(benchmark);
$("save").onclick = () =>
  action(async () => download("mnist-weights.json", await checkpoint()));
$("metrics").onclick = () => download("mnist-measurements.json", metrics);
$("file").onchange = () =>
  action(async () => {
    const file = $("file").files[0];
    if (!file) return;
    if (file.size > 5e6) throw Error("Checkpoint exceeds 5 MB");
    await restore(JSON.parse(await file.text()));
  });
$("backend").onchange = () => {
  defaultDevice($("backend").value);
  tree.dispose(params);
  params = null;
  metrics = {};
  $("summary").textContent = "Watch the loss fall.";
  $("chart").innerHTML =
    '<text x="20" y="130" fill="#627477">Train to see the learning curve.</text>';
  $("timings").textContent = "";
  $("status").textContent = "Ready";
  setBusy(false);
};
try {
  const adapter = await navigator.gpu?.requestAdapter();
  if (adapter)
    gpuInfo = {
      vendor: adapter.info.vendor,
      architecture: adapter.info.architecture,
      device: adapter.info.device,
      description: adapter.info.description,
      fallback:
        adapter.info.isFallbackAdapter ?? adapter.isFallbackAdapter ?? null,
    };
  const devices = await init("webgpu", "wasm");
  const available = ["webgpu", "wasm"].filter((v) => devices.includes(v));
  if (!available.length) throw Error("No supported compute backend");
  for (const v of available)
    $("backend").add(new Option(v === "webgpu" ? "WebGPU" : "WebAssembly", v));
  defaultDevice(available[0]);
  $("status").textContent = "Ready · choose compute, then train";
  setBusy(false);
} catch (e) {
  $("error").textContent = String(e);
}
