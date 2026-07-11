import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { renderCodeowners, repoRoot } from "./codeowners.mjs";

const args = process.argv.slice(2);
const value = flag => {
  const index = args.indexOf(flag);
  return index >= 0 ? args[index + 1] : undefined;
};
const owner = value("--owner") ?? "@jinlong17";
const file = resolve(repoRoot, value("--file") ?? ".github/CODEOWNERS");
const expected = renderCodeowners(owner);

if (args.includes("--write")) {
  writeFileSync(file, expected);
  console.log(`wrote CODEOWNERS: ${file}`);
} else {
  const actual = readFileSync(file, "utf8");
  if (actual !== expected) throw new Error(`CODEOWNERS drift: ${file}; regenerate from module registry`);
  console.log(`verified CODEOWNERS: owner=${owner}`);
}
