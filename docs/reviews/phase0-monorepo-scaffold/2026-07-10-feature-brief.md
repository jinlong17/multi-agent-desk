# Feature Brief: Phase 0 — monorepo empty skeleton

- Slug: `phase0-monorepo-scaffold`
- Date: 2026-07-10
- Owner module: `project-system`
- Impacted modules: all six product modules (empty directories and toolchain only)
- Requested by: `docs/IMPLEMENTATION_PLAN.md` §17/§19 Phase 0; lifecycle-readiness breakdown

## Motivation and outcome

No product toolchain exists. Outcome: the exact §17 skeleton builds empty on
all three platforms.

## Scope

1. Create the ADR 0009 layout: `cmd/{multidesk,multidesk-server}`,
   `internal/*`, `apps/{web,desktop}`, `packages/{ui,protocol,config}`,
   `api/{openapi,events}`, `migrations/{device,server}`, `deploy/docker`.
2. `go.mod`, `pnpm-workspace.yaml`, `justfile`, pinned toolchain version
   files (Go, Node, pnpm, Rust).
3. Hello-world compile targets so an empty build proves the toolchain.

## Non-goals

Any product behavior; CI wiring (`phase0-ci-governance`).

## User journeys

`just build` (or equivalent) succeeds on a fresh clone on Linux, macOS, and
Windows.

## Data and trust boundaries

None.

## Provider/external assumptions

None.

## Dependencies and gates

- Depends on: ADR 0009; ideally after `phase0-repository-layout` rename.
- Forbidden: `apps/daemon`, `apps/cli`, `apps/server`, `packages/provider-*`.

## Acceptance criteria

- [ ] Empty build passes locally on the three platforms (CI proof lands in
      `phase0-ci-governance`).
- [ ] Directory set matches §17 exactly; module registry paths resolve.
- [ ] `npm run project:verify` passes.

## Risks and open questions

Tauri toolchain pinning on Windows is the likely friction point; record exact
versions in the dev_log.

## Evidence

`docs/IMPLEMENTATION_PLAN.md` §17; `docs/adr/0009-repository-layout-authority.md`.

## Handoff

Next role: `feature-plan`.
