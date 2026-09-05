import { chromium } from "playwright";
import { once } from "node:events";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import assert from "node:assert/strict";
import { server } from "./server.mjs";
if (!server.listening) await once(server, "listening");
let browser;
try {
  browser = await chromium.launch({
    channel: process.env.CHROME_CHANNEL || undefined,
    executablePath:
      process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH || undefined,
    headless: true,
  });
} catch (error) {
  server.close();
  throw error;
}
try {
  const page = await browser.newPage({ acceptDownloads: true });
  page.on("console", (m) => {
    if (m.type() === "error") console.error(m.text());
  });
  const pageErrors = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  await page.goto(`http://127.0.0.1:${server.address().port}`);
  await page.locator("#train").waitFor({ state: "visible" });
  await page.waitForFunction(
    () => !document.querySelector("#train").disabled,
    null,
    {
      timeout: 60000,
    },
  );
  const backend = process.env.BACKEND || "wasm";
  await page.selectOption("#backend", backend);
  await page.fill("#count", process.env.TRAIN || "512");
  await page.fill("#epochs", process.env.EPOCHS || "2");
  await page.click("#train");
  await page.waitForFunction(
    () => !document.querySelector("#train").disabled,
    null,
    { timeout: 180000 },
  );
  assert.equal(await page.locator("#error").textContent(), "");
  assert.match(await page.locator("#status").textContent(), /^Trained/);
  const download = async (id) => {
    const promise = page.waitForEvent("download");
    await page.click(id);
    const d = await promise;
    return JSON.parse(await readFile(await d.path(), "utf8"));
  };
  const weights = await download("#save");
  await page.click("#benchmark");
  await page.waitForFunction(
    () => !document.querySelector("#benchmark").disabled,
    null,
    { timeout: 180000 },
  );
  assert.equal(await page.locator("#error").textContent(), "");
  const metrics = await download("#metrics");
  assert.ok(metrics.loss.at(-1) < metrics.loss[0]);
  assert.equal(metrics.inference.length, 6);
  assert.ok(metrics.inference.every((v) => v.warm_median_ms >= 0));
  await mkdir(new URL("./results/", import.meta.url), { recursive: true });
  await writeFile(
    new URL(`./results/${backend}.json`, import.meta.url),
    JSON.stringify(metrics, null, 2) + "\n",
  );
  await page.screenshot({
    path: new URL(`./results/${backend}.png`, import.meta.url).pathname,
    fullPage: true,
  });
  await page.reload();
  await page.waitForFunction(
    () => !document.querySelector("#train").disabled,
    null,
    { timeout: 60000 },
  );
  await page.selectOption("#backend", backend);
  await page.locator("#file").setInputFiles({
    name: "weights.json",
    mimeType: "application/json",
    buffer: Buffer.from(JSON.stringify(weights)),
  });
  await page.waitForFunction(
    () =>
      document
        .querySelector("#status")
        .textContent.startsWith("Weights reloaded"),
    null,
    { timeout: 60000 },
  );
  const restored = await download("#metrics");
  assert.ok(Math.abs(restored.accuracy - metrics.epoch_accuracy.at(-1)) < 1e-6);
  assert.ok(Number.isFinite(restored.reference_max_abs_error));
  await page.locator("#file").setInputFiles({
    name: "bad.json",
    mimeType: "application/json",
    buffer: Buffer.from(
      JSON.stringify({ format: "jade-mnist-mlp-v1", weights: {} }),
    ),
  });
  await page.waitForFunction(() =>
    document.querySelector("#error").textContent.includes("Invalid weight"),
  );
  const afterInvalid = await download("#save");
  assert.deepEqual(afterInvalid, weights);
  for (const kind of ["float32 overflow", "output overflow"]) {
    const invalid = structuredClone(weights);
    if (kind === "float32 overflow") invalid.weights.w1[0][0] = 1e100;
    else {
      invalid.weights.w1.forEach((row) => row.fill(1e38));
      invalid.weights.w2.forEach((row) => row.fill(1e38));
    }
    await page
      .locator("#file")
      .setInputFiles({
        name: kind + ".json",
        mimeType: "application/json",
        buffer: Buffer.from(JSON.stringify(invalid)),
      });
    await page.waitForFunction(() => !document.querySelector("#file").disabled);
    assert.notEqual(await page.locator("#error").textContent(), "");
    assert.deepEqual(await download("#save"), weights);
  }
  assert.equal(restored.backend, backend);
  assert.equal(restored.model, metrics.model);
  assert.ok(restored.timing_notes);
  assert.deepEqual(pageErrors, []);
  console.log(
    JSON.stringify(
      {
        backend,
        training_seconds: metrics.training_seconds,
        accuracy: metrics.epoch_accuracy,
        loss: [metrics.loss[0], metrics.loss.at(-1)],
        inference: metrics.inference,
        reload_accuracy: restored.accuracy,
      },
      null,
      2,
    ),
  );
} finally {
  await browser.close();
  server.close();
}
