# Test plan: Phase 0 monorepo scaffold

## P1 acceptance

1. Run the structure validator and an independent `find`/module-registry audit;
   every §17 and owned path must exist, and retired paths must not exist.
2. Parse every JSON manifest/config with Node, `cargo metadata --no-deps` for
   Cargo TOML when available, and inspect Go/YAML/version files; assert the
   exact manifest set from `api.md`.
3. Confirm all Go/TypeScript/Rust files are scaffolding only and search for
   Provider invocation, credential, socket, crypto-algorithm, and database
   behavior.
4. Confirm the three Windows Spikes remain DRAFT and no Windows transport is
   frozen.
5. Run `npm run project:verify` and `git diff --check`.

## P2 acceptance

1. Record `go version`, `node --version`, `pnpm --version`, `rustc --version`,
   `cargo --version`, and `just --version`; missing tools are `unknown`/failed,
   never pass.
2. Generate only the minimal Tauri 2/Vite/TypeScript lockfiles, inspect declared
   licenses, update THIRD_PARTY_NOTICES, then run `pnpm install --frozen-lockfile`
   and Cargo `--locked` checks/builds.
3. Run every exact root script from `api.md`, including the actual
   `desktop:build` Tauri `--no-bundle` command. `scaffold:verify` must fail if a
   required tool is missing; a temporary local Go/just install may supply the
   tools without modifying repository or system configuration.
4. Run `just check`, `just build`, and `just verify` when just is present; if
   unavailable after the attempted temporary install, record each as unknown
   and do not use underlying-command success as proof of the just wrapper.
5. Run `npm run project:verify`, `git diff --check`, and a conflict-marker scan.
6. Record macOS results. Linux and Windows build/unit results remain `unknown`
   until `phase0-ci-governance` executes the matrix. Record exactly:
   `Windows acceptance: deferred (no local Windows machine)` for interactive
   Tauri/sidecar scope; CI build evidence is separate.

## External evidence used by planning

- Official Go release history/downloads: Go 1.26 stable line.
- Official Node release schedule: Node 24 LTS.
- Official Tauri 2 prerequisites: Rust plus platform-specific macOS, Linux,
  and Windows system dependencies.
