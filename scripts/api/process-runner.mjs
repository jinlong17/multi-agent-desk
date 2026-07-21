import { execFileSync } from "node:child_process";
import { readFileSync, realpathSync, statSync } from "node:fs";
import { basename, isAbsolute, join, relative, resolve, sep } from "node:path";

function invocation(executable, args) {
  return { executable, args: [...args] };
}

function directInvocation(executable, args) {
  return invocation(executable, args);
}

function nodeCLIInvocation(cliPath, args, nodeExecutable = process.execPath) {
  return invocation(nodeExecutable, [cliPath, ...args]);
}

function pnpmCLIFromPackageRoot(packageRoot) {
  const canonicalRoot = realpathSync(packageRoot);
  const packageMetadata = JSON.parse(readFileSync(join(canonicalRoot, "package.json"), "utf8"));
  if (packageMetadata.name !== "pnpm" || packageMetadata.bin?.pnpm !== "bin/pnpm.cjs") {
    throw new Error(`invalid pnpm package metadata: ${canonicalRoot}`);
  }
  const canonicalCLI = realpathSync(join(canonicalRoot, packageMetadata.bin.pnpm));
  const relativeCLI = relative(canonicalRoot, canonicalCLI);
  if (!relativeCLI || relativeCLI.startsWith(`..${sep}`) || relativeCLI === ".." || isAbsolute(relativeCLI) ||
      basename(canonicalCLI) !== "pnpm.cjs" || !statSync(canonicalCLI).isFile()) {
    throw new Error(`pnpm CLI escapes its package root: ${canonicalCLI}`);
  }
  return canonicalCLI;
}

function resolvePnpmCLI({ env = process.env, platform = process.platform } = {}) {
  if (platform !== "win32") throw new Error("pnpm JavaScript CLI resolution is only required on Windows");
  if (!env.PNPM_HOME || !isAbsolute(env.PNPM_HOME)) {
    throw new Error("PNPM_HOME must be an absolute path on Windows");
  }
  const canonicalHome = realpathSync(env.PNPM_HOME);
  return pnpmCLIFromPackageRoot(resolve(canonicalHome, "..", "pnpm"));
}

function pnpmInvocation(args, options = {}) {
  const { nodeExecutable = process.execPath, platform = process.platform, env = process.env } = options;
  if (platform !== "win32") return directInvocation("pnpm", args);
  return nodeCLIInvocation(resolvePnpmCLI({ platform, env }), args, nodeExecutable);
}

function noShellOptions(options = {}) {
  return { ...options, shell: false };
}

function execFileNoShell({ executable, args }, options = {}) {
  return execFileSync(executable, args, noShellOptions(options));
}

export { directInvocation, nodeCLIInvocation, resolvePnpmCLI, pnpmInvocation, noShellOptions, execFileNoShell };
