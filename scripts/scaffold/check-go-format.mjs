import { execFileSync } from "node:child_process";
import { readdirSync, statSync } from "node:fs";
import { dirname, extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const roots = ["cmd", "internal"];
const files = [];

function walk(path) {
  for (const entry of readdirSync(path)) {
    const target = resolve(path, entry);
    if (statSync(target).isDirectory()) walk(target);
    else if (extname(target) === ".go") files.push(target);
  }
}

roots.forEach(root => walk(resolve(repoRoot, root)));
const output = execFileSync("gofmt", ["-l", ...files], { encoding: "utf8" }).trim();
if (output) throw new Error(`Go files require gofmt:\n${output}`);
console.log(`verified Go formatting: files=${files.length}`);
