# Feature Brief: Phase 0 — threat model initial version

- Slug: `phase0-threat-model`
- Date: 2026-07-10
- Owner module: `security`
- Impacted modules: `project-system` (docs link/verify wiring)
- Requested by: `docs/IMPLEMENTATION_PLAN.md` §18/§19 Phase 0; lifecycle-readiness R2 split of `phase0-security-docs-adrs`

## Motivation and outcome

Security invariants live scattered in the implementation plan. Outcome:
`docs/THREAT_MODEL.md` initial version — assets, attackers, trust boundaries,
mitigations, and explicit residual risk — accepted by independent
`security-review`.

## Scope

1. Extract assets/attackers/boundaries/mitigations from plan §12–§16.
2. Cover the five `CLAUDE.md` security invariants with residual-risk wording.
3. Mark every claim that Phase 0.5 spikes must still prove as spike-gated.

## Non-goals

ADR 0001–0008 (`phase0-architecture-adrs`); new security decisions; E2EE
protocol spec (spike-e2ee-protocol-vectors).

## User journeys

A security reviewer reads THREAT_MODEL.md and can enumerate attacker
capabilities and residual risks without the implementation plan.

## Data and trust boundaries

This document *is* the trust-boundary record; owned by `security` per the
module registry (`docs/THREAT_MODEL.md`).

## Provider/external assumptions

None; provider claims stay behind spike gates.

## Dependencies and gates

- Depends on: none.
- Gate: independent `security-review` must return `ACCEPTED` before Phase 0
  exit.

## Acceptance criteria

- [ ] THREAT_MODEL.md covers assets, attackers, boundaries, mitigations,
      residual risk, and the five `CLAUDE.md` invariants.
- [ ] `security-review` verdict `ACCEPTED` recorded under
      `docs/reviews/phase0-threat-model/`.
- [ ] `npm run project:verify` passes.

## Risks and open questions

Residual-risk wording must never claim remote erasure of already-copied
secrets.

## Evidence

`docs/IMPLEMENTATION_PLAN.md` §12–§16, §18;
`docs/reviews/lifecycle-readiness/2026-07-10-feature-review-r2.md` P1-D.

## Handoff

Next role: `feature-plan`.
