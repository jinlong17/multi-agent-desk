# Feature review: phase0-monorepo-scaffold

## Verdict

`REVISE`

The phase split and evidence policy are sound, but P1/P2 are not yet executable
without the builder inventing file and command decisions.

## Findings

### P1 — §17 file coverage is incomplete in the contract

`api.md` says every §17 path is required but enumerates only product
directories. Plan §17 also names `deploy/docker-compose.yml` and documentation
files `ARCHITECTURE.md`, `DATA_MODEL.md`, `PROVIDER_ADAPTER.md`, and
`ROADMAP.md`; these are absent today and the plan does not decide whether this
feature creates truthful placeholders or explicitly excludes them. Because
the feature claims exact §17 structure, list every required file and its
minimal non-claiming content.

### P1 — manifests and version-file names are ambiguous

The plan does not freeze exact root/package/crate manifest paths, workspace
members, or version-file names. Specify at least `go.mod`, root `package.json`
scripts and `packageManager`, `pnpm-workspace.yaml`, `.node-version`,
`.go-version`, `rust-toolchain.toml`, `justfile`, Web/Desktop/shared package
manifests, `Cargo.toml`, `tauri.conf.json`, and the structure-validator path.
State whether Go 1.26 means `go 1.26` and whether the Rust channel is exact
`1.91.1` or floating `stable`; the current text says both.

### P1/P2 — Tauri build contract is weaker than the acceptance claim

`cargo check` proves a Rust crate but not that the Tauri 2 application
configuration/CLI/frontend integration builds. Specify a valid Tauri 2
manifest/config/capability set and the exact smoke command, such as the Tauri
CLI build with `--no-bundle`, plus which platform system dependencies are
required. If P2 only runs Cargo check, narrow the acceptance claim; otherwise
test the actual Tauri shell.

### P2 — root command graph and missing-tool behavior are not deterministic

Name the exact root scripts and what each invokes, including whether
`scaffold:verify` fails on missing Go/just or runs only available checks. A
verification command that silently skips missing tools would violate the
evidence policy. `just` may wrap scripts, but root npm commands must remain the
CI source of truth on Windows where shell assumptions differ.

### P2 — dependency resolution and license ownership need a boundary

Clarify that P2 may resolve only minimal scaffold dependencies and records
their licenses, while the enforcement/negative GPL fixture remains owned by
`phase0-ci-governance`. Lockfiles must be generated once and verified frozen;
do not add dependencies merely to make placeholders look realistic.

## Evidence

- Plan v0.2 §17 and §19
- ADR 0009 and module registry
- feature brief, design, API, and test plan
- current repository tree and local tool inventory
- official Go, Node, and Tauri release/prerequisite evidence recorded by the
  planner
- `npm run project:verify` passed at `NEEDS_REVIEW`.

## Blockers

None external. Clearing role: `feature-plan` revises the five decision gaps and
returns the unit to `NEEDS_REVIEW`.
