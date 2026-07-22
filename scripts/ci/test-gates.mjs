import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const node = process.execPath;
const replaceExactlyOnce = (source, needle, replacement, label) => {
  const occurrences = source.split(needle).length - 1;
  if (occurrences !== 1) throw new Error(`${label}: expected one occurrence of ${needle}, found ${occurrences}`);
  const index = source.indexOf(needle);
  return `${source.slice(0, index)}${replacement}${source.slice(index + needle.length)}`;
};
const replaceNth = (source, needle, occurrence, replacement, label) => {
  let index = -1;
  let offset = 0;
  for (let current = 1; current <= occurrence; current++) {
    index = source.indexOf(needle, offset);
    if (index < 0) throw new Error(`${label}: occurrence ${occurrence} of ${needle} is missing`);
    offset = index + needle.length;
  }
  return `${source.slice(0, index)}${replacement}${source.slice(index + needle.length)}`;
};
const run = (args, expected, pattern) => {
  const result = spawnSync(node, args, { encoding: "utf8" });
  const output = `${result.stdout}${result.stderr}`;
  if ((expected === 0 && result.status !== 0) || (expected !== 0 && result.status === 0)) {
    throw new Error(`${args.join(" ")} returned ${result.status}, expected ${expected === 0 ? "success" : "failure"}\n${output}`);
  }
  if (pattern && !pattern.test(output)) throw new Error(`${args.join(" ")} did not report ${pattern}\n${output}`);
};

const dco = "scripts/ci/verify-dco.mjs";
run([dco, "--fixture", "scripts/ci/fixtures/dco-pass.json"], 0, /verified DCO/);
run([dco, "--fixture", "scripts/ci/fixtures/dco-missing.json"], 1, /DCO missing or malformed/);
run([dco, "--fixture", "scripts/ci/fixtures/dco-malformed.json"], 1, /DCO missing or malformed/);
run([dco, "--fixture", "scripts/ci/fixtures/dco-grandfathered-pass.json"], 0, /grandfathered=1/);
run([dco, "--fixture", "scripts/ci/fixtures/dco-grandfathered-mismatch.json"], 1, /DCO missing or malformed/);

const licenses = "scripts/ci/verify-licenses.mjs";
run([licenses, "--fixture", "scripts/ci/fixtures/licenses-clean.json"], 0, /verified licenses/);
run([licenses, "--fixture", "scripts/ci/fixtures/licenses-gpl.json"], 1, /disallowed license/);
run([licenses, "--fixture", "scripts/ci/fixtures/licenses-unknown.json"], 1, /unknown\/custom license/);
run([licenses, "--fixture", "scripts/ci/fixtures/licenses-with-denied.json"], 1, /disallowed license/);
run([licenses, "--fixture", "scripts/ci/fixtures/licenses-custom.json"], 1, /unknown\/custom license/);

const links = "scripts/ci/check-local-links.mjs";
run([links, "scripts/ci/fixtures/links/pass.md"], 0, /verified local links/);
run([links, "scripts/ci/fixtures/links/broken-file.md"], 1, /missing local target/);
run([links, "scripts/ci/fixtures/links/broken-anchor.md"], 1, /missing anchor/);

run(["scripts/ci/verify-codeowners.mjs"], 0, /verified CODEOWNERS/);
run(["scripts/ci/verify-codeowners.mjs", "--file", "scripts/ci/fixtures/CODEOWNERS-drift"], 1, /CODEOWNERS drift/);
run(["scripts/ci/verify-actions.mjs"], 0, /verified Actions contracts/);
run(["scripts/ci/verify-actions.mjs", "--ci", "scripts/ci/fixtures/actions-write.yml"], 1, /secret\/write\/deploy\/release surface/);
const actionsTemporary = mkdtempSync(join(tmpdir(), "mad-actions-fixtures-"));
try {
  const workflow = readFileSync(".github/workflows/ci.yml", "utf8");
  const remapped = join(actionsTemporary, "windows-remapped.yml");
  writeFileSync(remapped, replaceExactlyOnce(workflow, "          - id: windows\n            os: windows-latest", "          - id: windows\n            os: ubuntu-latest", "windows mapping fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", remapped], 1, /map windows directly to windows-latest/);
  const fixedRunner = join(actionsTemporary, "fixed-runner.yml");
  writeFileSync(fixedRunner, replaceExactlyOnce(workflow, "    runs-on: ${{ matrix.os }}", "    runs-on: ubuntu-latest", "fixed runner fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", fixedRunner], 1, /run directly on matrix.os/);
  const partialGo = join(actionsTemporary, "partial-go.yml");
  writeFileSync(partialGo, replaceExactlyOnce(workflow, "run: go test -count=1 ./...", "run: go test -count=1 ./internal/controlplane", "partial Go fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", partialGo], 1, /directly run the full Go test suite/);
  const missingVersion = join(actionsTemporary, "missing-go-version.yml");
  writeFileSync(missingVersion, replaceExactlyOnce(workflow, "          go version\n", "", "missing Go version fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", missingVersion], 1, /private-storage acceptance missing: go version/);
  const weakenedWindowsCondition = join(actionsTemporary, "weakened-windows-condition.yml");
  writeFileSync(weakenedWindowsCondition, replaceNth(workflow, "if: ${{ !cancelled() && runner.os == 'Windows' }}", 1, "if: runner.os == 'Windows'", "private-storage condition fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", weakenedWindowsCondition], 1, /private-storage acceptance condition must occur exactly once/);
  const missingWindowsShell = join(actionsTemporary, "missing-windows-shell.yml");
  writeFileSync(missingWindowsShell, replaceNth(workflow, "        shell: pwsh\n", 1, "", "private-storage shell fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", missingWindowsShell], 1, /private-storage acceptance shell must occur exactly once/);
  const enabledWindowsCGO = join(actionsTemporary, "enabled-windows-cgo.yml");
  writeFileSync(enabledWindowsCGO, replaceNth(workflow, '          CGO_ENABLED: "0"', 1, '          CGO_ENABLED: "1"', "private-storage CGO fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", enabledWindowsCGO], 1, /private-storage acceptance CGO setting must occur exactly once/);
  const missingWindowsCodex = join(actionsTemporary, "missing-windows-codex.yml");
  writeFileSync(missingWindowsCodex, replaceExactlyOnce(workflow, " ./internal/device ./internal/providers/codex", " ./internal/device", "missing Codex package fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", missingWindowsCodex], 1, /private-storage acceptance missing: go test/);
  const weakenedWindowsSkip = join(actionsTemporary, "weakened-windows-skip.yml");
  writeFileSync(weakenedWindowsSkip, replaceExactlyOnce(workflow, "|TestConfiguredCodexBinaryEmptyHomeHandshake", "", "Windows skip allowlist fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", weakenedWindowsSkip], 1, /private-storage acceptance missing: go test/);
  const missingWindowsVerbose = join(actionsTemporary, "missing-windows-verbose.yml");
  writeFileSync(missingWindowsVerbose, replaceExactlyOnce(workflow, "go test -count=1 -v -skip", "go test -count=1 -skip", "Windows verbose fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", missingWindowsVerbose], 1, /private-storage acceptance missing: go test/);
  const weakenedMigrationStress = join(actionsTemporary, "weakened-migration-stress.yml");
  writeFileSync(weakenedMigrationStress, replaceExactlyOnce(workflow, "go test -count=20 -run", "go test -count=1 -run", "migration stress fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", weakenedMigrationStress], 1, /migration stress acceptance missing: go test/);
  const weakenedMigrationCondition = join(actionsTemporary, "weakened-migration-condition.yml");
  writeFileSync(weakenedMigrationCondition, replaceNth(workflow, "if: ${{ !cancelled() && runner.os == 'Windows' }}", 2, "if: runner.os == 'Windows'", "migration condition fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", weakenedMigrationCondition], 1, /migration stress acceptance condition must occur exactly once/);
  const enabledMigrationCGO = join(actionsTemporary, "enabled-migration-cgo.yml");
  writeFileSync(enabledMigrationCGO, replaceNth(workflow, '          CGO_ENABLED: "0"', 2, '          CGO_ENABLED: "1"', "migration CGO fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", enabledMigrationCGO], 1, /migration stress acceptance CGO setting must occur exactly once/);
  const storageCommand = "go test -count=1 -v -skip '^(TestAuthBeginCancelUsesPrivateOwnerBoundEnrollment|TestVersionDiscoveryUsesAbsoluteExecutableAndBoundedProbe|TestConfiguredCodexBinaryCanonicalSchemaProbe|TestConfiguredCodexBinaryEmptyHomeHandshake)$' ./internal/app ./internal/controlplane ./internal/storage ./internal/device ./internal/providers/codex";
  const maskedWindowsFailure = join(actionsTemporary, "masked-windows-failure.yml");
  writeFileSync(maskedWindowsFailure, replaceExactlyOnce(workflow, storageCommand, `${storageCommand}; exit 0`, "masked Windows failure fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", maskedWindowsFailure], 1, /private-storage acceptance command sequence must match exactly/);
  const migrationCommand = "          go test -count=20 -run '^TestStoreConcurrentMigrationHasOneCompleteLedger$' -v ./internal/controlplane";
  const extraMigrationCommand = join(actionsTemporary, "extra-migration-command.yml");
  writeFileSync(extraMigrationCommand, replaceExactlyOnce(workflow, migrationCommand, `${migrationCommand}\n          Write-Output 'masked'`, "extra migration command fixture"));
  run(["scripts/ci/verify-actions.mjs", "--ci", extraMigrationCommand], 1, /migration stress acceptance command sequence must match exactly/);
} finally {
  rmSync(actionsTemporary, { recursive: true, force: true });
}

console.log("verified CI gate fixtures: positive and negative cases");
