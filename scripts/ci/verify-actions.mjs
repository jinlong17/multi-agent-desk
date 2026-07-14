import { readFileSync } from "node:fs";

const args = process.argv.slice(2);
const value = (flag, fallback) => {
  const index = args.indexOf(flag);
  return index >= 0 ? args[index + 1] : fallback;
};
const ci = readFileSync(value("--ci", ".github/workflows/ci.yml"), "utf8");
const governance = readFileSync(value("--governance", ".github/workflows/governance.yml"), "utf8");
const all = `${ci}\n${governance}`;
const required = ["project-verify", "build-ubuntu", "build-macos", "build-windows", "license-gate", "dco", "link-check"];
const assert = (condition, message) => { if (!condition) throw new Error(message); };

assert(!/secrets\.|id-token:|contents:\s*write|pull-requests:\s*write|packages:\s*write|deploy|release/i.test(all), "workflow contains secret/write/deploy/release surface");
for (const token of ["pull_request:", "push:", "workflow_dispatch:"]) {
  assert(ci.includes(token) && governance.includes(token), `workflow trigger missing: ${token}`);
}
assert((all.match(/permissions:\n\s+contents: read/g) ?? []).length === 2, "workflows must use top-level contents: read permissions");
assert(ci.includes("name: build-${{ matrix.id }}"), "matrix job must render unique build names");
for (const id of ["ubuntu", "macos", "windows"]) assert(ci.includes(`- id: ${id}`), `build matrix missing ${id}`);
for (const name of ["project-verify", "license-gate", "dco", "link-check"]) assert(all.includes(`name: ${name}`), `required job name missing: ${name}`);
for (const name of required) {
  if (name.startsWith("build-")) continue;
  assert((all.match(new RegExp(`name: ${name.replaceAll("-", "\\-")}\\b`, "g")) ?? []).length === 1, `job name must be unique: ${name}`);
}
const uses = [...all.matchAll(/uses:\s*([^\s#]+)(?:\s*#\s*(.+))?/g)];
assert(uses.length > 0, "no actions found");
for (const match of uses) {
  const ref = match[1];
  assert(/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+@[0-9a-f]{40}$/.test(ref), `action is not SHA-pinned: ${ref}`);
  assert(match[2]?.trim().startsWith("v"), `SHA-pinned action lacks version comment: ${ref}`);
}
assert((all.match(/persist-credentials:\s*false/g) ?? []).length >= 5, "every checkout must disable persisted credentials");
for (const command of ["npm run project:verify", "npm run ci:static", "npm run scaffold:verify", "npm run ci:licenses", "verify-dco.mjs", "check-local-links.mjs", "go-licenses", "check --include_tests"]) {
  assert(all.includes(command), `workflow command missing: ${command}`);
}
assert(governance.includes("fetch-depth: 0"), "DCO checkout must fetch full history");
assert(governance.includes("--exclude-path scripts/ci/fixtures/links"), "HTTP link check must exclude intentional negative fixtures");
assert(ci.includes("libwebkit2gtk-4.1-dev") && ci.includes("libayatana-appindicator3-dev"), "Linux Tauri prerequisites incomplete");
console.log(`verified Actions contracts: checks=${required.length}, actions=${uses.length}`);
