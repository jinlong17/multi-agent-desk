# Design: Phase 0 monorepo scaffold

## Decision snapshot

- Owner: `project-system`; product modules are impacted only by empty package
  and directory boundaries.
- ADR 0009 and Plan v0.2 §17 are the physical-layout authorities.
- Split into P1 structure/manifests and P2 reproducible empty builds so each
  phase receives independent verification.
- No domain behavior, Provider integration, crypto protocol, IPC transport,
  persistence schema, or Windows-specific implementation is introduced.

## P1: structure, manifests, and pins

Create every §17 directory and no retired alternatives. Add two minimal Go
command packages, compile-safe internal package placeholders, pnpm workspace
packages for Web/UI/protocol/config, a Vite Web entry, and a Tauri 2 desktop
shell whose Rust code contains no domain logic. Add API/migration/deploy
placeholders that state ownership rather than invent contracts.

Root manifests pin supported lines with unambiguous files: `go.mod` uses
`go 1.26` and `.go-version` records `1.26.5`; `.node-version` records major
`24` and package engines require `>=24 <25`; root `packageManager` pins
`pnpm@10.23.0`; `rust-toolchain.toml` pins exact `1.91.1`; Tauri dependencies
use major 2 and lock exact versions in P2. `scripts/scaffold/verify-layout.mjs`
asserts required/forbidden paths and module-registry path coverage.

The four §17 documentation files not yet present — `ARCHITECTURE.md`,
`DATA_MODEL.md`, `PROVIDER_ADAPTER.md`, and `ROADMAP.md` — are created as
truthful placeholders that link to Plan v0.2/ADRs and label future content;
they make no implementation or compatibility claim. `deploy/docker-compose.yml`
is a valid empty service map (`services: {}`), not a deployable product claim.

## P2: lockfiles and empty-build orchestration

Resolve and commit `pnpm-lock.yaml` and `apps/desktop/src-tauri/Cargo.lock`.
Provide cross-platform root npm scripts as the CI source of truth and a
`justfile` wrapper only around those scripts. The empty build compiles both Go
commands, workspace TypeScript/Vite packages, and the actual Tauri 2 shell via
`pnpm --filter @multi-agent-desk/desktop tauri build --no-bundle`; it does not
start services or Provider processes.

The Tauri shell consists of `Cargo.toml`, `build.rs`, `src/main.rs`,
`tauri.conf.json`, and `capabilities/default.json`; it loads the Web build and
uses only Tauri core default capability. Bundling, updater, sidecar, tray,
deep-link, and keychain behavior are absent.

The local macOS environment must record exact tool versions and actual command
results. Go/just may be installed into a temporary, non-repository tool root
for P2; if still unavailable, their commands fail/remain unknown rather than
being skipped. Linux and Windows build proof
belongs to `phase0-ci-governance`, whose matrix must run the same repository
commands. Windows interactive acceptance is outside this scaffold.

Root scripts are fixed as `scaffold:structure`, `go:fmt-check`, `go:build`,
`go:test`, `web:check`, `web:build`, `desktop:fmt-check`, `desktop:check`,
`desktop:build`, `scaffold:check`, `scaffold:build`, and `scaffold:verify`.
`scaffold:verify` runs structure + every format/check/build/test command and
therefore exits nonzero when a required executable or dependency is missing;
it contains no skip-on-missing branch. `just check|build|verify` delegates to
these npm scripts.

## Boundaries and ownership

- `core`: `cmd/multidesk`, app/domain/runtime/device/vault/storage, device
  migrations; placeholders only.
- `provider`: provider directories/protocol package; no adapter behavior.
- `control-plane`: server command, controlplane/transport, API/server
  migrations; placeholders only.
- `web`: Vite shell and shared TypeScript packages; no product screens.
- `desktop`: Tauri 2 shell; no sidecar lifecycle or OS integration.
- `security`: crypto directory placeholder; no algorithm or key format.

## Failure, compatibility, and rollback

Missing toolchains, dependency resolution, platform system libraries, or build
failures are recorded exactly. P1 can roll back generated paths/manifests; P2
can roll back lockfiles/orchestration. No data migration or compatibility claim
exists. Windows Named Pipe, ConPTY, and sidecar decisions remain DRAFT Spikes.
P2 resolves only Tauri CLI/runtime, Vite, and TypeScript scaffold dependencies,
records their declared licenses in `THIRD_PARTY_NOTICES.md`, and leaves license
enforcement plus the incompatible-GPL negative fixture to
`phase0-ci-governance`.
