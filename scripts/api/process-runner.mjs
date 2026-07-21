import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync, realpathSync, statSync } from "node:fs";
import { basename, isAbsolute, join, relative, sep } from "node:path";

const projectMetadata = JSON.parse(readFileSync(new URL("../../package.json", import.meta.url), "utf8"));
const packageManagerMatch = /^pnpm@(\d+\.\d+\.\d+)$/.exec(projectMetadata.packageManager ?? "");
if (!packageManagerMatch) {
  throw new Error("packageManager must pin an exact pnpm version");
}
const PINNED_PNPM_VERSION = packageManagerMatch[1];

function invocation(executable, args) {
  return { executable, args: [...args] };
}

function directInvocation(executable, args) {
  return invocation(executable, args);
}

function nodeCLIInvocation(cliPath, args, nodeExecutable = process.execPath) {
  return invocation(nodeExecutable, [cliPath, ...args]);
}

function isWithin(root, candidate) {
  const relativePath = relative(root, candidate);
  return relativePath === "" ||
    (!relativePath.startsWith(`..${sep}`) && relativePath !== ".." && !isAbsolute(relativePath));
}

function pnpmCLIFromPackageRoot(packageRoot, expectedVersion, trustedRoots) {
  const canonicalRoot = realpathSync(packageRoot);
  if (!trustedRoots.some((trustedRoot) => isWithin(trustedRoot, canonicalRoot))) {
    throw new Error(`pnpm package escapes trusted PNPM_HOME roots: ${canonicalRoot}`);
  }
  const packageMetadata = JSON.parse(readFileSync(join(canonicalRoot, "package.json"), "utf8"));
  if (packageMetadata.name !== "pnpm" || packageMetadata.version !== expectedVersion ||
      packageMetadata.bin?.pnpm !== "bin/pnpm.cjs") {
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

function activePnpmInstallRoots(canonicalHome, expectedVersion) {
  const canonicalGlobalRoot = realpathSync(join(canonicalHome, "global", "v11"));
  if (!isWithin(canonicalHome, canonicalGlobalRoot)) {
    throw new Error(`pnpm global root escapes PNPM_HOME: ${canonicalGlobalRoot}`);
  }

  const storeVersionPath = join(canonicalHome, "store", "v11", "links", "@", "pnpm", expectedVersion);
  const trustedRoots = [canonicalGlobalRoot];
  if (existsSync(storeVersionPath)) {
    const canonicalStoreVersionRoot = realpathSync(storeVersionPath);
    if (!isWithin(canonicalHome, canonicalStoreVersionRoot)) {
      throw new Error(`pnpm store root escapes PNPM_HOME: ${canonicalStoreVersionRoot}`);
    }
    trustedRoots.push(canonicalStoreVersionRoot);
  }

  const packageRoots = new Map();
  for (const entry of readdirSync(canonicalGlobalRoot, { withFileTypes: true })) {
    if (!entry.isDirectory() || !/^[a-z0-9]+(?:-[a-z0-9]+)?$/i.test(entry.name)) continue;
    const installRoot = realpathSync(join(canonicalGlobalRoot, entry.name));
    if (!isWithin(canonicalGlobalRoot, installRoot)) {
      throw new Error(`pnpm install root escapes its global root: ${installRoot}`);
    }
    const installMetadataPath = join(installRoot, "package.json");
    if (!existsSync(installMetadataPath)) continue;
    const installMetadata = JSON.parse(readFileSync(installMetadataPath, "utf8"));
    if (installMetadata.dependencies?.pnpm !== expectedVersion) {
      throw new Error(`invalid pnpm global install metadata: ${installRoot}`);
    }
    const packageRoot = join(installRoot, "node_modules", "pnpm");
    const canonicalPackageRoot = realpathSync(packageRoot);
    packageRoots.set(canonicalPackageRoot, packageRoot);
  }
  return { packageRoots: [...packageRoots.values()], trustedRoots };
}

function resolvePnpmCLI({ env = process.env, platform = process.platform } = {}) {
  if (platform !== "win32") throw new Error("pnpm JavaScript CLI resolution is only required on Windows");
  if (!env.PNPM_HOME || !isAbsolute(env.PNPM_HOME)) {
    throw new Error("PNPM_HOME must be an absolute path on Windows");
  }
  const canonicalHome = realpathSync(env.PNPM_HOME);
  const canonicalActiveBin = realpathSync(join(canonicalHome, "bin"));
  if (!isWithin(canonicalHome, canonicalActiveBin)) {
    throw new Error(`pnpm active bin directory escapes PNPM_HOME: ${canonicalActiveBin}`);
  }
  const canonicalShim = realpathSync(join(canonicalActiveBin, "pnpm.cmd"));
  if (!isWithin(canonicalActiveBin, canonicalShim) || !statSync(canonicalShim).isFile()) {
    throw new Error(`invalid active pnpm Windows shim: ${canonicalShim}`);
  }

  const { packageRoots, trustedRoots } = activePnpmInstallRoots(canonicalHome, PINNED_PNPM_VERSION);
  if (packageRoots.length !== 1) {
    throw new Error(`expected exactly one canonical pnpm ${PINNED_PNPM_VERSION} install, found ${packageRoots.length}`);
  }
  return pnpmCLIFromPackageRoot(packageRoots[0], PINNED_PNPM_VERSION, trustedRoots);
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

export {
  PINNED_PNPM_VERSION,
  directInvocation,
  nodeCLIInvocation,
  resolvePnpmCLI,
  pnpmInvocation,
  noShellOptions,
  execFileNoShell,
};
