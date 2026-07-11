# Feature Brief: Phase 0 — first ADR batch and research log

- Slug: `phase0-architecture-adrs`
- Date: 2026-07-10
- Owner module: `project-system`
- Impacted modules: none (architecture documentation only)
- Requested by: `docs/IMPLEMENTATION_PLAN.md` §18/§19 Phase 0; lifecycle-readiness R2 split of `phase0-security-docs-adrs`

## Motivation and outcome

The eight reserved architecture decisions exist only inside the
implementation plan. Outcome: ADR 0001–0008 become standalone, reviewable
documents, plus the research-log scaffolding.

## Scope

1. ADR 0001–0008 per the reserved list in `docs/adr/README.md`.
2. `docs/RESEARCH_LOG.md` (external/AGPL research boundaries, dates,
   conclusions).
3. `docs/PROVIDER_COMPATIBILITY.md` placeholder that Phase 0.5 spikes fill.

## Non-goals

`docs/THREAT_MODEL.md` (owned by `security`, split into
`phase0-threat-model`); new security decisions; anything Phase 0.5 spikes
must first prove.

## User journeys

An engineer reads one ADR to understand one irreversible decision without
opening the 1000-line implementation plan.

## Data and trust boundaries

Documentation only; must not weaken any `CLAUDE.md` security invariant.

## Provider/external assumptions

None; provider claims stay behind spike gates.

## Dependencies and gates

- Depends on: none (parallel to scaffold).
- Gate: docs link check.

## Acceptance criteria

- [ ] ADR 0001–0008 exist and are indexed in `docs/adr/README.md`.
- [ ] Spike-gated sections are explicitly marked, not frozen.
- [ ] Docs link check passes; `npm run project:verify` passes.

## Risks and open questions

ADR wording must not freeze decisions Phase 0.5 spikes may overturn.

## Evidence

`docs/IMPLEMENTATION_PLAN.md` §18 first ADR batch;
`docs/reviews/lifecycle-readiness/2026-07-10-feature-review-r2.md` P1-D.

## Handoff

Next role: `feature-plan`.
