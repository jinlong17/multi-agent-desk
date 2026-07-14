import { createServer } from "node:http";
import { existsSync, readFileSync, statSync } from "node:fs";
import { dirname, extname, normalize, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const root = resolve(repoRoot, "docs/prototypes/dev-dashboard");
const requestedPort = Number(process.env.PORT || process.argv.find(value => /^\d+$/.test(value)) || 4178);
const mime = { ".html":"text/html; charset=utf-8", ".js":"application/javascript; charset=utf-8", ".css":"text/css; charset=utf-8", ".png":"image/png", ".svg":"image/svg+xml" };

const generated = spawnSync(process.execPath, [resolve(repoRoot, "scripts/dashboard/generate-state.mjs")], { cwd: repoRoot, stdio: "inherit" });
if (generated.status !== 0) process.exit(generated.status || 1);

const server = createServer((req, res) => {
  const url = new URL(req.url || "/", "http://127.0.0.1");
  const relativePath = url.pathname === "/" ? "index.html" : decodeURIComponent(url.pathname.slice(1));
  const file = normalize(resolve(root, relativePath));
  if (!file.startsWith(`${root}/`) && file !== root) {
    res.writeHead(403).end("Forbidden");
    return;
  }
  if (!existsSync(file) || !statSync(file).isFile()) {
    res.writeHead(404).end("Not found");
    return;
  }
  res.writeHead(200, { "Content-Type": mime[extname(file)] || "application/octet-stream", "Cache-Control":"no-store" });
  res.end(readFileSync(file));
});

server.listen(requestedPort, "127.0.0.1", () => {
  console.log(`MultiAgentDesk dashboard serving at http://127.0.0.1:${requestedPort}`);
});
