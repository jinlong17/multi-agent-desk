# Feature Brief: Phase 0 — repository identity and root governance files

- Slug: `phase0-repository-layout`
- Date: 2026-07-10
- Owner module: `project-system`
- Impacted modules: none (repo root only)
- Requested by: `docs/IMPLEMENTATION_PLAN.md` §19 Phase 0; lifecycle-readiness breakdown

## Motivation and outcome

The repository still lives at `agent-deck` with no license or contribution
governance. Outcome: repo identity matches the product (`multi-agent-desk`)
and the root governance files exist.

## Scope

1. Rename the local directory and Git remote to `multi-agent-desk`
   (dedicated maintenance window; operator-gated).
2. Apache-2.0 `LICENSE`, `CONTRIBUTING.md` (DCO sign-off, no CLA),
   `SECURITY.md`, `THIRD_PARTY_NOTICES.md`, refreshed `README.md`.
3. Confirm layout per `docs/adr/0009-repository-layout-authority.md`; no
   product directories created here.

## Non-goals

Monorepo skeleton (`phase0-monorepo-scaffold`), CI
(`phase0-ci-governance`), ADR 0001–0008 (`phase0-architecture-adrs`),
threat model (`phase0-threat-model`).

## User journeys

A new contributor clones `multi-agent-desk`, reads README/CONTRIBUTING, and
knows the license, DCO rule, and where security reports go.

## Data and trust boundaries

None; documentation and repo metadata only.

## Provider/external assumptions

None.

## Dependencies and gates

- Depends on: ADR 0009 (done in `lifecycle-readiness`).
- Human gates: remote rename and any push.

## Acceptance criteria

- [ ] `git remote get-url origin` points at `multi-agent-desk`.
- [ ] LICENSE, CONTRIBUTING (DCO), SECURITY, THIRD_PARTY_NOTICES, README exist
      and are link-clean.
- [ ] `npm run project:verify` passes.

## Risks and open questions

Renaming the directory breaks open editor/agent sessions; do it in an
isolated maintenance window.

## Evidence

`docs/IMPLEMENTATION_PLAN.md` §19 Phase 0 deliverables.

## Handoff

Next role: `feature-plan`.
