import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, realpathSync, rmSync, symlinkSync, unlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  PINNED_PNPM_VERSION,
  directInvocation,
  execFileNoShell,
  nodeCLIInvocation,
  pnpmInvocation,
  noShellOptions,
} from "./process-runner.mjs";

function writePnpmPackage(packageRoot, { version, bin = "bin/pnpm.cjs", source } = {}) {
  const cli = join(packageRoot, bin);
  mkdirSync(join(packageRoot, "bin"), { recursive: true });
  writeFileSync(join(packageRoot, "package.json"), JSON.stringify({
    name: "pnpm",
    version,
    bin: { pnpm: bin },
  }));
  writeFileSync(cli, source ?? "process.stdout.write(JSON.stringify(process.argv.slice(2)));\n");
  return cli;
}

function pnpmFixture(t) {
  const root = mkdtempSync(join(tmpdir(), "mad-pnpm-runner-"));
  t.after(() => rmSync(root, { recursive: true, force: true }));
  const home = join(root, "node_modules", ".bin");
  const stalePackageRoot = join(root, "node_modules", "pnpm");
  const storePackageRoot = join(
    home,
    "store",
    "v11",
    "links",
    "@",
    "pnpm",
    PINNED_PNPM_VERSION,
    "fixture-store-hash",
    "node_modules",
    "pnpm",
  );
  const installRoot = join(home, "global", "v11", "1a18-19f86746116");
  const activePackageRoot = join(installRoot, "node_modules", "pnpm");

  mkdirSync(join(home, "bin"), { recursive: true });
  writeFileSync(join(home, "bin", "pnpm.cmd"), [
    "@ECHO off",
    "GOTO start",
    ":find_dp0",
    "SET dp0=%~dp0",
    "EXIT /b",
    ":start",
    "SETLOCAL",
    "CALL :find_dp0",
    `node \"%dp0%\\..\\global\\v11\\1a18-19f86746116\\node_modules\\pnpm\\bin\\pnpm.cjs\" %*`,
    "",
  ].join("\r\n"));
  writePnpmPackage(stalePackageRoot, {
    version: "11.1.1",
    bin: "bin/pnpm.mjs",
    source: "throw new Error('stale bootstrap pnpm v11 must not run');\n",
  });
  const cli = writePnpmPackage(storePackageRoot, { version: PINNED_PNPM_VERSION });
  mkdirSync(join(installRoot, "node_modules"), { recursive: true });
  writeFileSync(join(installRoot, "package.json"), JSON.stringify({
    dependencies: { pnpm: PINNED_PNPM_VERSION },
  }));
  symlinkSync(storePackageRoot, activePackageRoot, process.platform === "win32" ? "junction" : "dir");
  return { root, home, installRoot, activePackageRoot, storePackageRoot, cli: realpathSync(cli) };
}

test("Windows ignores stale bootstrap v11 and executes the action self-update v10 CLI through Node", (t) => {
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
  const { home, storePackageRoot } = pnpmFixture(t);
  writeFileSync(join(storePackageRoot, "package.json"), JSON.stringify({
    name: "pnpm",
    version: "11.1.1",
    bin: { pnpm: "bin/pnpm.mjs" },
  }));
  assert.throws(
    () => pnpmInvocation([], { platform: "win32", env: { PNPM_HOME: home } }),
    /invalid pnpm package metadata/,
  );
});

test("Windows pnpm resolution rejects an active package that escapes PNPM_HOME", (t) => {
  const { root, home, activePackageRoot } = pnpmFixture(t);
  const escapedPackageRoot = join(root, "escaped-pnpm");
  writePnpmPackage(escapedPackageRoot, { version: PINNED_PNPM_VERSION });
  unlinkSync(activePackageRoot);
  symlinkSync(escapedPackageRoot, activePackageRoot, process.platform === "win32" ? "junction" : "dir");
  assert.throws(
    () => pnpmInvocation([], { platform: "win32", env: { PNPM_HOME: home } }),
    /escapes trusted PNPM_HOME roots/,
  );
});

test("Windows pnpm resolution rejects ambiguous canonical action installs", (t) => {
  const { home } = pnpmFixture(t);
  const secondStorePackageRoot = join(
    home,
    "store",
    "v11",
    "links",
    "@",
    "pnpm",
    PINNED_PNPM_VERSION,
    "second-store-hash",
    "node_modules",
    "pnpm",
  );
  writePnpmPackage(secondStorePackageRoot, { version: PINNED_PNPM_VERSION });
  const secondInstallRoot = join(home, "global", "v11", "1a19-19f86746117");
  mkdirSync(join(secondInstallRoot, "node_modules"), { recursive: true });
  writeFileSync(join(secondInstallRoot, "package.json"), JSON.stringify({
    dependencies: { pnpm: PINNED_PNPM_VERSION },
  }));
  symlinkSync(
    secondStorePackageRoot,
    join(secondInstallRoot, "node_modules", "pnpm"),
    process.platform === "win32" ? "junction" : "dir",
  );
  assert.throws(
    () => pnpmInvocation([], { platform: "win32", env: { PNPM_HOME: home } }),
    /expected exactly one canonical pnpm 10\.23\.0 install, found 2/,
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
