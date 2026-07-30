import { copyFileSync, mkdirSync, readFileSync, readdirSync, rmSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const dist = join(root, "apps/web/dist");
const embedded = join(root, "internal/controlplane/webassets");
const mode = process.argv[2] ?? "verify";

function relativeFiles(directory, prefix = "") {
  const result = [];
  for (const entry of readdirSync(join(directory, prefix), { withFileTypes: true })) {
    const path = join(prefix, entry.name);
    if (entry.isSymbolicLink()) throw new Error(`Web asset must not be a symlink: ${path}`);
    if (entry.isDirectory()) result.push(...relativeFiles(directory, path));
    else if (entry.isFile()) result.push(path.split(sep).join("/"));
    else throw new Error(`unsupported Web asset entry: ${path}`);
  }
  return result.sort();
}

function validateDist() {
  const files = relativeFiles(dist);
  const assets = files.filter((file) => file.startsWith("assets/"));
  if (files.length !== 3 || assets.length !== 2 || !files.includes("index.html")) {
    throw new Error(`expected index.html plus one hashed JS and CSS asset, got: ${files.join(", ")}`);
  }
  const js = assets.find((file) => /^assets\/index-[A-Za-z0-9_-]{8,}\.js$/u.test(file));
  const css = assets.find((file) => /^assets\/index-[A-Za-z0-9_-]{8,}\.css$/u.test(file));
  if (!js || !css) throw new Error("Vite output filenames are not content-hashed JS/CSS assets");
  const html = readFileSync(join(dist, "index.html"), "utf8");
  if (!html.includes(`src="/${js}"`) || !html.includes(`href="/${css}"`) || html.includes("/src/main.ts") ||
      /<link[^>]+rel=["']manifest["']/iu.test(html) || /https?:\/\//iu.test(html)) {
    throw new Error("embedded index must reference only the two local hashed Vite assets and no source/manifest/third-party URL");
  }
  const javascript = readFileSync(join(dist, js), "utf8");
  if (/serviceWorker|navigator\.serviceWorker|workbox/iu.test(javascript)) {
    throw new Error("P2 Web bundle must not register a service worker");
  }
  return files;
}

function generate(files) {
  mkdirSync(embedded, { recursive: true });
  rmSync(join(embedded, "assets"), { recursive: true, force: true });
  for (const file of files) {
    const destination = join(embedded, file);
    mkdirSync(dirname(destination), { recursive: true });
    copyFileSync(join(dist, file), destination);
  }
}

function verify(files) {
  const embeddedFiles = ["index.html", ...relativeFiles(join(embedded, "assets"), "").map((file) => `assets/${file}`)].sort();
  if (JSON.stringify(embeddedFiles) !== JSON.stringify(files)) {
    throw new Error(`embedded Web asset set drifted: ${embeddedFiles.join(", ")}`);
  }
  for (const file of files) {
    if (!readFileSync(join(dist, file)).equals(readFileSync(join(embedded, file)))) {
      throw new Error(`embedded Web asset content drifted: ${relative(root, join(embedded, file))}`);
    }
  }
}

const files = validateDist();
if (mode === "generate") generate(files);
else if (mode === "verify") verify(files);
else throw new Error(`unknown mode: ${mode}`);
console.log(`${mode === "generate" ? "generated" : "verified"} deterministic embedded Web assets`);
