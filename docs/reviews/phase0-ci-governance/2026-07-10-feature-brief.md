# Feature Brief: Phase 0 — CI and remote governance

- Slug: `phase0-ci-governance`
- Date: 2026-07-10
- Owner module: `project-system`
- Impacted modules: none (`.github` and remote settings only)
- Requested by: `docs/IMPLEMENTATION_PLAN.md` §19 Phase 0; lifecycle-readiness review P1-6/P1-7

## Motivation and outcome

There is no `.github/workflows`, no branch protection, and remote governance
has never been verified. Outcome: every push is checked on three platforms
and the remote enforces the checks.

## Scope

1. GitHub Actions: Linux/macOS/Windows empty-build matrix, formatting,
   static checks, dependency cache.
2. License gate (Go + pnpm dependency scan; unknown/GPL/AGPL/custom fail),
   DCO check, docs link check.
3. `npm run project:verify` as a required check.
4. CODEOWNERS generated from `docs/workflow/project/module-registry.json`.
5. Verify and record remote settings: branch protection on `main`, required
   checks, Actions permissions, release permissions (operator performs
   permission changes; this feature documents and verifies them).

## Non-goals

Release/signing/SBOM pipelines (pre-release gates, Phase 6); product tests.

## User journeys

A PR that violates layout, license policy, or verification cannot merge.

## Data and trust boundaries

CI must hold no provider credentials; repo-scoped tokens only.

## Provider/external assumptions

GitHub-hosted runners for all three platforms.

## Dependencies and gates

- Depends on: `phase0-monorepo-scaffold` (build targets to run).
- Human gates: all remote settings changes.

## Acceptance criteria

- [ ] Three-platform empty build green in Actions.
- [ ] License gate demonstrably fails a seeded GPL dev-dependency (test PR).
- [ ] `main` branch protection with required checks verified and recorded.
- [ ] CODEOWNERS matches the module registry.

## Risks and open questions

Windows runner time; whether DCO uses an app or a workflow check.

## Evidence

`docs/IMPLEMENTATION_PLAN.md` §17 root toolchain + §19 Phase 0 exit criteria.

## Handoff

Next role: `feature-plan`.
