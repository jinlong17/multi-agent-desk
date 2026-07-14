# Feature Brief: Lifecycle readiness hardening

- Slug: `lifecycle-readiness`
- Date: 2026-07-10
- Owner module: `project-system`
- Impacted modules: all six product modules (governance only; no product code)
- Requested by: operator, acting on the 2026-07-10 lifecycle readiness review

## Motivation and outcome

The 2026-07-10 readiness review returned `REVISE`
(`docs/reviews/lifecycle-readiness/2026-07-10-lifecycle-readiness-review.md`).
Before Phase 0 product work starts, the project system must close four P0
gaps (layout authority, verdict persistence, spike state machine, phase work
breakdown) and the project-system-scoped P1 gap (semantic verification).
Outcome: the workflow is a closed loop — every state transition has a defined
on-disk writer, every workflow has a complete state machine, and
`npm run project:verify` checks semantics, not just file existence.

## Scope

1. ADR 0009 selecting the single repository layout authority; align
   `CLAUDE.md` and `docs/workflow/project/module-registry.json`.
2. Convert review/verify roles to verdict writers with an exact write scope;
   update workflow policy, role contracts, and agent registry.
3. Close the spike state machine: `feature-plan` owns intake and decision;
   add `spike_log.md` template; add `ACCEPTED`, `GATE_RESOLVED`,
   `INCONCLUSIVE`, and `BLOCKED`-recovery transitions.
4. Break Phase 0 into executable feature units and Phase 0.5 into named spike
   units with state records.
5. Strengthen `workflow:verify` and `dashboard:verify`: mirror content
   equality, role/registry/state-machine status consistency, template field
   completeness, and feature dev_log parseability.

## Non-goals

- No product code, monorepo scaffold, or CI implementation (that is
  `phase0-*` work).
- No remote GitHub governance changes.
- No release/operations workflow implementation (tracked as pre-release
  gates in `design.md`).

## User journeys

An operator or agent picks up any feature, bug, or spike at any state and can
determine from files alone: who writes next, what statuses are legal, and how
`BLOCKED` recovers.

## Data and trust boundaries

Governance files only. No credentials, no provider calls. Human gates
(priority, risk acceptance, merge, push, ship) unchanged.

## Provider/external assumptions

None. Provider assumptions stay behind Phase 0.5 spike gates.

## Dependencies and gates

- Gate: `npm run project:verify` must pass after every change.
- Blocks: `phase0-monorepo-scaffold` depends on ADR 0009.

## Acceptance criteria

- [ ] ADR 0009 exists; `CLAUDE.md` and module registry carry no
      `apps/daemon`-style paths.
- [ ] Every status in every role handoff appears in the registry output and
      the workflow state machine, enforced by `workflow:verify`.
- [ ] `feature-review`, `feature-verify`, `bug-verify`, `security-review`
      have an explicit two-file write scope (report + dev_log).
- [ ] Spike lifecycle `DRAFT → SPIKE_READY → EVIDENCE_READY →
      ACCEPTED/GATE_RESOLVED` is fully written with owners; spike template
      exists and is verified.
- [ ] Five `phase0-*` feature units and seven Phase 0.5 spike units have
      state records visible on the dashboard.
- [ ] `workflow:verify` fails on mirror content drift; `dashboard:verify`
      fails on `MISSING_DEV_LOG` or unparseable dev_log fields.

## Risks and open questions

- Verdict writers gain workspace-write sandboxes; their write scope is
  enforced by role contract and review, not by the sandbox. Accepted:
  the alternative (a separate state-recorder role) adds a handoff hop with
  its own atomicity problems.
- Remote governance (P1-7) remains open and is deferred to
  `phase0-ci-governance`.

## Evidence

`docs/reviews/lifecycle-readiness/2026-07-10-lifecycle-readiness-review.md`;
`npm run project:verify` results recorded in the feature dev_log.

## Handoff

Next role: `feature-plan`.
