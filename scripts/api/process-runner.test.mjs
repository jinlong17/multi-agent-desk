import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { directInvocation, execFileNoShell, nodeCLIInvocation, pnpmInvocation, noShellOptions } from "./process-runner.mjs";

function pnpmFixture(t) {
  const root = mkdtempSync(join(tmpdir(), "mad-pnpm-runner-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const home = join(root, "node_modules", ".bin");
  const packageRoot = join(root, "node_modules", "pnpm");
  const cli = join(packageRoot, "bin", "pnpm.cjs");
  mkdirSync(home, { recursive: true });
  mkdirSync(join(packageRoot, "bin"), { recursive: true });
  writeFileSync(join(packageRoot, "package.json"), JSON.stringify({ name: "pnpm", bin: { pnpm: "bin/pnpm.cjs" } }));
  writeFileSync(cli, "process.stdout.write(JSON.stringify(process.argv.slice(2)));\n");
  return { home, packageRoot, cli: realpathSync(cli) };
}

test("Windows resolves PNPM_HOME to the real pnpm CLI and invokes it through Node", (t) => {
  const { home, cli } = pnpmFixture(t);
  const args = ["licenses", "list", "--json", "literal & not-a-command"];
  const command = pnpmInvocation(args, {
    platform: "win32",
    env: { PNPM_HOME: home, PATH: "" },
    nodeExecutable: process.execPath,
  });
  assert.deepEqual(command, {
    executable: process.execPath,
    args: [cli, ...args],
  });
  assert.deepEqual(JSON.parse(execFileNoShell(command, { encoding: "utf8" })), args);
});

test("Windows pnpm resolution fails closed without an absolute PNPM_HOME", () => {
  assert.throws(() => pnpmInvocation([], { platform: "win32", env: {} }), /PNPM_HOME must be an absolute path/);
  assert.throws(() => pnpmInvocation([], { platform: "win32", env: { PNPM_HOME: "relative" } }), /PNPM_HOME must be an absolute path/);
});

test("Windows pnpm resolution rejects unexpected package metadata", (t) => {
  const { home, packageRoot } = pnpmFixture(t);
  writeFileSync(join(packageRoot, "package.json"), JSON.stringify({ name: "not-pnpm", bin: { pnpm: "bin/pnpm.cjs" } }));
  assert.throws(
    () => pnpmInvocation([], { platform: "win32", env: { PNPM_HOME: home } }),
    /invalid pnpm package metadata/,
  );
});

test("non-Windows pnpm remains a direct executable with separate arguments", () => {
  const args = ["why", "argparse", "literal;still-one-argument"];
  for (const platform of ["darwin", "linux"]) {
    assert.deepEqual(pnpmInvocation(args, { platform }), { executable: "pnpm", args });
  }
  assert.deepEqual(directInvocation("go", args), { executable: "go", args });
});

test("local Node CLI uses the current runtime and keeps paths as separate arguments", () => {
  const cliPath = "C:\\repo path\\node_modules\\openapi-typescript\\bin\\cli.js";
  const outputPath = "C:\\output path\\generated & safe.ts";
  assert.deepEqual(nodeCLIInvocation(cliPath, ["spec.yaml", "--output", outputPath], "C:\\Program Files\\node.exe"), {
    executable: "C:\\Program Files\\node.exe",
    args: [cliPath, "spec.yaml", "--output", outputPath],
  });
});

test("child process options force shell off", () => {
  assert.deepEqual(noShellOptions({ cwd: "C:\\repo path", shell: true }), {
    cwd: "C:\\repo path",
    shell: false,
  });
});
