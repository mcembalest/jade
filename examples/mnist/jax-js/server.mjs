import { build } from "esbuild";
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
const root = new URL("./", import.meta.url);
await build({
  entryPoints: [fileURLToPath(new URL("app.js", root))],
  bundle: true,
  format: "esm",
  outfile: fileURLToPath(new URL("dist/app.js", root)),
});
const routes = new Map([
  ["/", ["index.html", "text/html"]],
  ["/app.js", ["dist/app.js", "text/javascript"]],
  ...[
    "train-images-idx3-ubyte",
    "train-labels-idx1-ubyte",
    "t10k-images-idx3-ubyte",
    "t10k-labels-idx1-ubyte",
  ].map((name) => [
    "/data/" + name,
    ["../data/" + name, "application/octet-stream"],
  ]),
]);
export const server = createServer(async (req, res) => {
  const route = routes.get(req.url);
  if (req.method !== "GET" || !route) {
    res.writeHead(404).end();
    return;
  }
  try {
    const body = await readFile(new URL(route[0], root));
    res
      .writeHead(200, {
        "Content-Type": route[1],
        "X-Content-Type-Options": "nosniff",
        "Cache-Control": "no-store",
      })
      .end(body);
  } catch {
    res.writeHead(404).end("Missing file. Run ../fetch.sh first.");
  }
});
server.listen(Number(process.env.PORT || 0), "127.0.0.1", () =>
  console.log(`http://127.0.0.1:${server.address().port}`),
);
