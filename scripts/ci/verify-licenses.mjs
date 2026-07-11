import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";

const allowed = new Set(["0BSD", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "CC0-1.0", "ISC", "MIT", "MIT-0", "MPL-2.0", "Unicode-3.0", "Unlicense", "Zlib"]);
const allowedExceptions = new Set(["LLVM-exception"]);

function tokenize(expression) {
  return expression.replaceAll("/", " OR ").match(/\(|\)|\bAND\b|\bOR\b|\bWITH\b|[A-Za-z0-9.+-]+/g) ?? [];
}

function parse(expression) {
  const tokens = tokenize(expression);
  let index = 0;
  const primary = () => {
    if (tokens[index] === "(") {
      index += 1;
      const node = or();
      if (tokens[index++] !== ")") throw new Error(`unbalanced SPDX expression: ${expression}`);
      return node;
    }
    const id = tokens[index++];
    if (!id || ["AND", "OR", "WITH", ")"].includes(id)) throw new Error(`invalid SPDX expression: ${expression}`);
    return { op: "id", id };
  };
  const withNode = () => {
    let node = primary();
    if (tokens[index] === "WITH") {
      index += 1;
      const exception = tokens[index++];
      node = { op: "with", base: node, exception };
    }
    return node;
  };
  const and = () => {
    let node = withNode();
    while (tokens[index] === "AND") { index += 1; node = { op: "and", left: node, right: withNode() }; }
    return node;
  };
  const or = () => {
    let node = and();
    while (tokens[index] === "OR") { index += 1; node = { op: "or", left: node, right: and() }; }
    return node;
  };
  const tree = or();
  if (index !== tokens.length) throw new Error(`unparsed SPDX expression: ${expression}`);
  return tree;
}

function accepted(node) {
  if (node.op === "id") return allowed.has(node.id);
  if (node.op === "with") return accepted(node.base) && allowedExceptions.has(node.exception);
  if (node.op === "and") return accepted(node.left) && accepted(node.right);
  if (node.op === "or") return accepted(node.left) || accepted(node.right);
  return false;
}

function verifyExpression(expression, source) {
  if (!expression || /unknown|LicenseRef|SEE LICENSE/i.test(expression)) throw new Error(`${source}: unknown/custom license ${expression || "<missing>"}`);
  if (!accepted(parse(expression))) throw new Error(`${source}: disallowed license expression ${expression}`);
}

const fixtureIndex = process.argv.indexOf("--fixture");
let pnpm;
let cargo;
if (fixtureIndex >= 0) {
  ({ pnpm, cargo } = JSON.parse(readFileSync(process.argv[fixtureIndex + 1], "utf8")));
} else {
  const commandOptions = { encoding: "utf8", maxBuffer: 16 * 1024 * 1024 };
  pnpm = JSON.parse(execFileSync("pnpm", ["licenses", "list", "--json"], commandOptions));
  const metadata = JSON.parse(execFileSync("cargo", ["metadata", "--locked", "--format-version", "1", "--manifest-path", "apps/desktop/src-tauri/Cargo.toml"], commandOptions));
  cargo = metadata.packages.map(pkg => ({ name: `${pkg.name}@${pkg.version}`, license: pkg.license }));
}
for (const expression of Object.keys(pnpm)) verifyExpression(expression, `pnpm group ${expression}`);
for (const pkg of cargo) verifyExpression(pkg.license, `Cargo ${pkg.name}`);
console.log(`verified licenses: pnpm_groups=${Object.keys(pnpm).length}, cargo_packages=${cargo.length}`);

export { parse, accepted, verifyExpression };
