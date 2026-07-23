import { existsSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

import { directInvocation, execFileNoShell, pnpmInvocation } from "./process-runner.mjs";

const root = resolve(import.meta.dirname, "../..");
const commandOptions = { cwd: root, encoding: "utf8", maxBuffer: 16 * 1024 * 1024 };
const pins = new Map([
  ["github.com/oapi-codegen/oapi-codegen/v2", { version: "v2.8.0", license: /Apache License/ }],
  ["github.com/getkin/kin-openapi", { version: "v0.142.0", license: /MIT License/ }],
  ["github.com/oapi-codegen/runtime", { version: "v1.6.0", license: /Apache License/ }],
  ["github.com/google/uuid", { version: "v1.6.0", license: /Redistribution and use in source and binary forms/ }],
]);

for (const [modulePath, expected] of pins) {
  const metadata = JSON.parse(execFileNoShell(directInvocation("go", ["list", "-m", "-json", modulePath]), commandOptions));
  if (metadata.Path !== modulePath || metadata.Version !== expected.version || !metadata.Dir) {
    throw new Error(`${modulePath}: expected ${expected.version} in the resolved Go graph`);
  }
  const licensePath = ["LICENSE", "LICENSE.md", "COPYING"].map((name) => join(metadata.Dir, name)).find(existsSync);
  if (!licensePath || !expected.license.test(readFileSync(licensePath, "utf8"))) {
    throw new Error(`${modulePath}@${metadata.Version}: expected reviewed license text not found`);
  }
}

const generatorPackage = JSON.parse(readFileSync(join(root, "node_modules/openapi-typescript/package.json"), "utf8"));
if (generatorPackage.version !== "7.13.0" || generatorPackage.license !== "MIT") {
  throw new Error("openapi-typescript must remain exactly 7.13.0 / MIT");
}
const workspace = readFileSync(join(root, "pnpm-workspace.yaml"), "utf8");
const lock = readFileSync(join(root, "pnpm-lock.yaml"), "utf8");
if (!workspace.includes("'js-yaml@4.2.0>argparse': '-'") || /argparse@|Python-2\.0/.test(lock)) {
  throw new Error("the reviewed js-yaml CLI-edge override is absent or argparse remains locked");
}
const whyArgparse = execFileNoShell(pnpmInvocation(["why", "argparse"]), commandOptions);
if (/argparse\s+\d/.test(whyArgparse)) throw new Error("argparse remains reachable in the pnpm dependency graph");
const pnpmLicenses = JSON.parse(execFileNoShell(pnpmInvocation(["licenses", "list", "--json"]), commandOptions));
if (Object.keys(pnpmLicenses).some((license) => /Python-2\.0/i.test(license))) {
  throw new Error("Python-2.0 remains in the pnpm license graph");
}

console.log(`verified API tool licenses: go_modules=${pins.size}, openapi_typescript=7.13.0, argparse=absent`);
