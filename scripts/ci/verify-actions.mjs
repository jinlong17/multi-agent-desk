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
const block = (source, startPattern, siblingPattern, label) => {
  const start = source.search(startPattern);
  assert(start >= 0, `${label} block missing`);
  const remainder = source.slice(start);
  const sibling = remainder.slice(1).search(siblingPattern);
  return sibling < 0 ? remainder : remainder.slice(0, sibling + 1);
};

assert(!/secrets\.|id-token:|contents:\s*write|pull-requests:\s*write|packages:\s*write|deploy|release/i.test(all), "workflow contains secret/write/deploy/release surface");
for (const token of ["pull_request:", "push:", "workflow_dispatch:"]) {
  assert(ci.includes(token) && governance.includes(token), `workflow trigger missing: ${token}`);
}
assert((all.match(/permissions:\n\s+contents: read/g) ?? []).length === 2, "workflows must use top-level contents: read permissions");
assert(ci.includes("name: build-${{ matrix.id }}"), "matrix job must render unique build names");
const buildJob = block(ci, /^  build:\s*$/mu, /^  [a-zA-Z0-9_-]+:\s*$/mu, "build job");
assert((buildJob.match(/^    runs-on: \$\{\{ matrix\.os \}\}$/gmu) ?? []).length === 1, "build job must run directly on matrix.os");
for (const [id, os] of [["ubuntu", "ubuntu-latest"], ["macos", "macos-latest"], ["windows", "windows-latest"]]) {
  const mapping = new RegExp(`^          - id: ${id}\\n            os: ${os}$`, "gmu");
  assert((buildJob.match(mapping) ?? []).length === 1, `build matrix must map ${id} directly to ${os}`);
}
const fullGoStep = block(buildJob, /^      - name: Run Go test suite\s*$/mu, /^      - name: /mu, "full Go test step");
assert(/^        run: go test -count=1 \.\/\.\.\.$/mu.test(fullGoStep), "build matrix must directly run the full Go test suite");
assert(!/^        if:/mu.test(fullGoStep), "full Go test suite must run on every matrix OS");
const setupGoStep = block(buildJob, /^      - name: Setup Go\s*$/mu, /^      - name: /mu, "Setup Go step");
assert(!/^        if:/mu.test(setupGoStep), "Go setup must run on every matrix OS");
const windowsStepContract = (step, name, label) => {
  assert((buildJob.match(new RegExp(`^      - name: ${name}$`, "gmu")) ?? []).length === 1, `${label} name must occur exactly once`);
  assert(buildJob.indexOf(`      - name: ${name}`) > buildJob.indexOf("      - name: Run Go test suite"), `${label} must follow the full Go test suite`);
  for (const [pattern, description] of [
    [/^        if: \$\{\{ !cancelled\(\) && runner\.os == 'Windows' \}\}$/gmu, "condition"],
    [/^        shell: pwsh$/gmu, "shell"],
    [/^          CGO_ENABLED: "0"$/gmu, "CGO setting"],
  ]) assert((step.match(pattern) ?? []).length === 1, `${label} ${description} must occur exactly once`);
  assert((step.match(/^          go test /gmu) ?? []).length === 1, `${label} must contain exactly one go test command`);
  assert(!/^        continue-on-error:/mu.test(step), `${label} must not continue on error`);
};
const windowsStorageStep = block(buildJob, /^      - name: Windows P2 private-storage acceptance\s*$/mu, /^      - name: /mu, "Windows P2 private-storage acceptance step");
windowsStepContract(windowsStorageStep, "Windows P2 private-storage acceptance", "Windows P2 private-storage acceptance");
for (const token of [
  "if: ${{ !cancelled() && runner.os == 'Windows' }}",
  "shell: pwsh",
  "CGO_ENABLED: \"0\"",
  "go version",
  "go env GOOS GOARCH CGO_ENABLED",
  "go test -count=1 -v -skip '^(TestAuthBeginCancelUsesPrivateOwnerBoundEnrollment|TestVersionDiscoveryUsesAbsoluteExecutableAndBoundedProbe|TestConfiguredCodexBinaryCanonicalSchemaProbe|TestConfiguredCodexBinaryEmptyHomeHandshake)$' ./internal/app ./internal/controlplane ./internal/storage ./internal/device ./internal/providers/codex",
]) assert(windowsStorageStep.includes(token), `Windows P2 private-storage acceptance missing: ${token}`);
const exactWindowsStorageStep = [
  "      - name: Windows P2 private-storage acceptance",
  "        if: ${{ !cancelled() && runner.os == 'Windows' }}",
  "        shell: pwsh",
  "        env:",
  "          CGO_ENABLED: \"0\"",
  "        run: |",
  "          go version",
  "          go env GOOS GOARCH CGO_ENABLED",
  "          go test -count=1 -v -skip '^(TestAuthBeginCancelUsesPrivateOwnerBoundEnrollment|TestVersionDiscoveryUsesAbsoluteExecutableAndBoundedProbe|TestConfiguredCodexBinaryCanonicalSchemaProbe|TestConfiguredCodexBinaryEmptyHomeHandshake)$' ./internal/app ./internal/controlplane ./internal/storage ./internal/device ./internal/providers/codex",
  "",
].join("\n");
assert(windowsStorageStep === exactWindowsStorageStep, "Windows P2 private-storage acceptance command sequence must match exactly");
const windowsMigrationStep = block(buildJob, /^      - name: Windows P2 migration stress acceptance\s*$/mu, /^      - name: /mu, "Windows P2 migration stress acceptance step");
windowsStepContract(windowsMigrationStep, "Windows P2 migration stress acceptance", "Windows P2 migration stress acceptance");
for (const token of [
  "if: ${{ !cancelled() && runner.os == 'Windows' }}",
  "shell: pwsh",
  "CGO_ENABLED: \"0\"",
  "go env GOOS GOARCH CGO_ENABLED",
  "go test -count=20 -run '^TestStoreConcurrentMigrationHasOneCompleteLedger$' -v ./internal/controlplane",
]) assert(windowsMigrationStep.includes(token), `Windows P2 migration stress acceptance missing: ${token}`);
const exactWindowsMigrationStep = [
  "      - name: Windows P2 migration stress acceptance",
  "        if: ${{ !cancelled() && runner.os == 'Windows' }}",
  "        shell: pwsh",
  "        env:",
  "          CGO_ENABLED: \"0\"",
  "        run: |",
  "          go env GOOS GOARCH CGO_ENABLED",
  "          go test -count=20 -run '^TestStoreConcurrentMigrationHasOneCompleteLedger$' -v ./internal/controlplane",
  "",
].join("\n");
assert(windowsMigrationStep === exactWindowsMigrationStep, "Windows P2 migration stress acceptance command sequence must match exactly");
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
