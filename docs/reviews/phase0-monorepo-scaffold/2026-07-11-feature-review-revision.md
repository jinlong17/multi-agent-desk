# Feature review: phase0-monorepo-scaffold revision

## Verdict

`APPROVED`

The revision closes all five findings. P1 is executable without inventing file,
manifest, version, Tauri, command, or dependency decisions; P2 has deterministic
failure semantics and preserves Linux/Windows evidence as unknown until CI.

## Finding closure

1. **§17 coverage — closed.** The contract now enumerates the missing docs,
   compose file, all product paths, and truthful placeholder semantics.
2. **Manifests/pins — closed.** Exact files, workspace/crate surfaces, Go
   directive, Node engine/major pin, exact pnpm, and exact Rust channel are set.
3. **Tauri proof — closed.** A valid Tauri 2 config/capability/crate set and
   actual CLI `tauri build --no-bundle` are required. Official Tauri v2
   distribution documentation confirms this command and flag.
4. **Command graph — closed.** Every root script and its responsibility is
   fixed; `scaffold:verify` fails rather than skips when a tool is missing; just
   delegates to root npm scripts.
5. **Dependencies/licenses — closed.** Minimal dependency scope and notice
   recording are separated from CI license enforcement/GPL negative fixture.

## Builder notes

- Keep P1 dependency-free: manifests may declare dependencies, but lockfile
  resolution and network installs belong to P2.
- The structure validator must derive module-owned path checks from
  `module-registry.json` and separately assert the §17 file list; do not create
  a second module map.
- Placeholder docs must link to current authorities and say what remains
  future work; do not copy large plan sections or claim implemented behavior.
- Tauri `beforeBuildCommand` must call the Web build using a cross-platform
  pnpm command, and `frontendDist` must resolve from `src-tauri` to Web output.
- P2 must record actual versions/license outputs and exact failed commands
  before attempting fixes.

## Evidence

- revised design/api/test/dev_log
- Plan v0.2 §17, ADR 0009, module registry
- official Tauri v2 prerequisites/config/distribution documentation
- official Go 1.26 and Node 24 LTS evidence recorded in planning
- `npm run project:verify` passed after revision.

## Blockers

None. Next legal writer is `feature-build` for P1 only.
