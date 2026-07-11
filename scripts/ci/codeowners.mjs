import { existsSync, readFileSync, statSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");

export function renderCodeowners(owner = "@jinlong17") {
  if (!/^@[A-Za-z0-9-]+$/.test(owner)) throw new Error(`invalid CODEOWNER handle: ${owner}`);
  const registry = JSON.parse(readFileSync(resolve(repoRoot, "docs/workflow/project/module-registry.json"), "utf8"));
  const lines = [
    "# Generated from docs/workflow/project/module-registry.json by scripts/ci/verify-codeowners.mjs.",
    "# Do not edit path ownership by hand.",
    `* ${owner}`,
    ""
  ];
  for (const module of registry.modules) {
    lines.push(`# ${module.key}`);
    for (const owned of module.owns) {
      const target = resolve(repoRoot, owned);
      if (!existsSync(target)) throw new Error(`module ${module.key} owns missing path: ${owned}`);
      const suffix = statSync(target).isDirectory() ? "/" : "";
      lines.push(`/${owned}${suffix} ${owner}`);
    }
    lines.push("");
  }
  return `${lines.join("\n").trimEnd()}\n`;
}

export { repoRoot };
