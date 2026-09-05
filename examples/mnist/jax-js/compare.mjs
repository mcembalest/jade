import { chromium } from "playwright";
import { once } from "node:events";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import { server } from "./server.mjs";
if (!server.listening) await once(server, "listening");
let browser;
try {
  browser = await chromium.launch({
    headless: true,
    channel: process.env.CHROME_CHANNEL || undefined,
  });
  for (const backend of ["wasm", "webgpu"]) {
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${server.address().port}`);
    await page.waitForFunction(
      () => !document.querySelector("#train").disabled,
      null,
      { timeout: 60000 },
    );
    const available = await page
      .locator("#backend option")
      .evaluateAll((options) => options.map((o) => o.value));
    if (!available.includes(backend))
      throw Error(
        `${backend} unavailable in this browser; try CHROME_CHANNEL=chrome. Available: ${available}`,
      );
    await page.selectOption("#backend", backend);
    const result = await page.evaluate(
      async (backend) => (await import("/app.js")).compare(backend),
      backend,
    );
    result.fixture_sha256 = createHash("sha256")
      .update(
        await readFile(new URL("../data/comparison.json", import.meta.url)),
      )
      .digest("hex");
    result.browser = browser.version();
    result.timing_notes =
      "Shared host-prepared batches; per-batch input upload and synchronization included. Training excludes first-step warmup, reset, evaluation. Inference uses identical initial weights and resident inputs; readback excluded.";
    await mkdir(new URL("../results/", import.meta.url), { recursive: true });
    await writeFile(
      new URL(`../results/jax-js-${backend}.json`, import.meta.url),
      JSON.stringify(result, null, 2) + "\n",
    );
    console.log(backend, result.training_seconds, result.accuracy);
    await page.close();
  }
} finally {
  await browser?.close();
  server.closeAllConnections();
  await new Promise((done) => server.close(done));
}
