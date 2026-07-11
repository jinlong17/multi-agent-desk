import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const requiredDirectories = [
  "cmd/multidesk", "cmd/multidesk-server",
  "internal/app", "internal/domain", "internal/providers",
  "internal/providers/codex", "internal/providers/claude", "internal/runtime",
  "internal/vault", "internal/device", "internal/crypto",
  "internal/providerprotocol", "internal/transport", "internal/controlplane",
  "internal/storage", "apps/web", "apps/desktop/src-tauri",
  "packages/ui", "packages/protocol", "packages/config", "api/openapi",
  "api/events", "migrations/device", "migrations/server", "deploy/docker",
  "docs/adr", "docs/reviews"
];
const requiredFiles = [
  "go.mod", "package.json", "pnpm-workspace.yaml", "pnpm-lock.yaml", ".go-version",
  ".node-version", "rust-toolchain.toml", "justfile",
  "cmd/multidesk/main.go", "cmd/multidesk-server/main.go",
  "apps/web/package.json", "apps/web/tsconfig.json", "apps/web/index.html",
  "apps/web/src/main.ts", "apps/desktop/package.json",
  "apps/desktop/src-tauri/Cargo.toml", "apps/desktop/src-tauri/Cargo.lock",
  "apps/desktop/src-tauri/build.rs",
  "apps/desktop/src-tauri/src/main.rs",
  "apps/desktop/src-tauri/tauri.conf.json",
  "apps/desktop/src-tauri/capabilities/default.json",
  "apps/desktop/src-tauri/icons/icon.svg",
  "apps/desktop/src-tauri/icons/icon.png",
  "packages/ui/package.json", "packages/ui/tsconfig.json", "packages/ui/src/index.ts",
  "packages/protocol/package.json", "packages/protocol/tsconfig.json", "packages/protocol/src/index.ts",
  "packages/config/package.json", "packages/config/tsconfig.json", "packages/config/src/index.ts",
  "api/openapi/README.md", "api/events/README.md",
  "migrations/device/README.md", "migrations/server/README.md",
  "deploy/docker/README.md", "deploy/docker-compose.yml",
  "docs/ARCHITECTURE.md", "docs/DATA_MODEL.md", "docs/PROVIDER_ADAPTER.md",
  "docs/THREAT_MODEL.md", "docs/PROVIDER_COMPATIBILITY.md",
  "docs/RESEARCH_LOG.md", "docs/ROADMAP.md", "THIRD_PARTY_NOTICES.md",
  "scripts/scaffold/check-go-format.mjs", "scripts/scaffold/verify-layout.mjs"
];
const forbiddenPaths = ["apps/daemon", "apps/cli", "apps/server"];

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

for (const path of requiredDirectories) {
  const target = resolve(repoRoot, path);
  assert(existsSync(target) && statSync(target).isDirectory(), `missing required directory: ${path}`);
}
for (const path of requiredFiles) {
  const target = resolve(repoRoot, path);
  assert(existsSync(target) && statSync(target).isFile(), `missing required file: ${path}`);
}
for (const path of forbiddenPaths) {
  assert(!existsSync(resolve(repoRoot, path)), `forbidden retired path exists: ${path}`);
}
for (const entry of readdirSync(resolve(repoRoot, "packages"))) {
  assert(!entry.startsWith("provider-"), `forbidden retired package exists: packages/${entry}`);
}

const registry = JSON.parse(readFileSync(resolve(repoRoot, "docs/workflow/project/module-registry.json"), "utf8"));
for (const module of registry.modules) {
  for (const owned of module.owns) {
    assert(existsSync(resolve(repoRoot, owned)), `module ${module.key} owns missing path: ${owned}`);
  }
}

for (const path of [
  "package.json", "apps/web/package.json", "apps/web/tsconfig.json",
  "apps/desktop/package.json", "apps/desktop/src-tauri/tauri.conf.json",
  "apps/desktop/src-tauri/capabilities/default.json", "packages/ui/package.json",
  "packages/ui/tsconfig.json", "packages/protocol/package.json",
  "packages/protocol/tsconfig.json", "packages/config/package.json",
  "packages/config/tsconfig.json"
]) JSON.parse(readFileSync(resolve(repoRoot, path), "utf8"));

console.log(`verified scaffold layout: directories=${requiredDirectories.length}, files=${requiredFiles.length}, modules=${registry.modules.length}`);
