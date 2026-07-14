import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, extname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const explicit = process.argv.slice(2);
const files = [];

function walk(path) {
  for (const entry of readdirSync(path)) {
    const target = resolve(path, entry);
    const rel = relative(repoRoot, target).replaceAll("\\", "/");
    if ([".git", "node_modules", "dist", "target"].some(part => rel.split("/").includes(part))) continue;
    if (!explicit.length && rel.startsWith("scripts/ci/fixtures/")) continue;
    if (statSync(target).isDirectory()) walk(target);
    else if (extname(target).toLowerCase() === ".md") files.push(target);
  }
}

if (explicit.length) explicit.forEach(path => files.push(resolve(repoRoot, path)));
else walk(repoRoot);

function slug(text) {
  return text.trim().toLowerCase().replace(/<[^>]*>/g, "").replace(/[^\p{L}\p{N}\s-]/gu, "").replace(/\s+/g, "-");
}

const failures = [];
for (const file of files) {
  const text = readFileSync(file, "utf8");
  let inFence = false;
  const visible = [];
  for (const [offset, line] of text.split(/\r?\n/).entries()) {
    if (/^\s*```/.test(line)) {
      inFence = !inFence;
      continue;
    }
    if (!inFence) visible.push({ line, index: offset + 1 });
  }
  for (const { line, index } of visible) {
    for (const match of line.matchAll(/(?<!!)\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g)) {
      const raw = match[1];
      if (/^(https?:|mailto:|tel:|data:)/i.test(raw)) continue;
      const decoded = decodeURIComponent(raw);
      const [pathPart, anchor] = decoded.split("#", 2);
      const target = pathPart ? resolve(dirname(file), pathPart) : file;
      if (!existsSync(target)) {
        failures.push(`${relative(repoRoot, file)}:${index}: missing local target ${raw}`);
        continue;
      }
      if (anchor && extname(target).toLowerCase() === ".md") {
        const headings = readFileSync(target, "utf8").split(/\r?\n/).filter(value => /^#{1,6}\s+/.test(value)).map(value => slug(value.replace(/^#{1,6}\s+/, "")));
        if (!headings.includes(anchor.toLowerCase())) failures.push(`${relative(repoRoot, file)}:${index}: missing anchor #${anchor} in ${relative(repoRoot, target)}`);
      }
    }
  }
}
if (failures.length) throw new Error(`local link failures:\n${failures.join("\n")}`);
console.log(`verified local links: markdown_files=${files.length}`);
