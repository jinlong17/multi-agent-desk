import { spawnSync } from "node:child_process";

const node = process.execPath;
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

console.log("verified CI gate fixtures: positive and negative cases");
