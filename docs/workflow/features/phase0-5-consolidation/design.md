# Design: Phase 0.5 compatibility consolidation

## Decision snapshot

- Selected option: close Phase 0.5 only after a fail-closed reconciliation of
  all seven Spike authorities, accepted ADR 0010-0016, compatibility claims,
  retained gates, and protected-main evidence; then activate Phase 1.
- Review evidence: the feature brief, Spike logs and reports, ADR index,
  compatibility matrix, implementation plan, threat model, dashboard state,
  generated dashboard facts, and GitHub check/protection readback.
- Frozen assumptions: accepted Spike decisions are design inputs, not proof of
  production implementation; exact tested versions bound every claim; GitHub
  is the protected synchronization authority; one operator account requires
  zero PR approvals but all seven checks remain mandatory.
- Rejected alternatives: treating each green Spike as implicit phase closure;
  leaving Phase 0.5 indefinitely active; deleting unsupported or deferred
  rows; or upgrading GitHub-runner evidence to Windows 11 release acceptance.

## Context and boundaries

The repository contains seven independently resolved Phase 0.5 Spikes owned by
five modules. Their conclusions are already merged and security-reviewed where
required. The remaining problem is cross-document state coherence. This unit
owns only the project-level transition and must not change product code,
provider credentials, cryptographic design, or previously accepted decisions.

The authoritative inputs are:

1. each Spike `dev_log.md` status and evidence ledger;
2. ADR 0010-0016 and `PROVIDER_COMPATIBILITY.md`;
3. the Phase 0.5/1 plan text and threat-model gates;
4. protected-main pull-request and workflow evidence.

The authoritative outputs are this feature's workflow documents, the manual
dashboard transition, generated dashboard facts, and a persisted verification
report. Chat history is never used as state authority.

## Components and ownership

- `project-system` owns the reconciliation, phase transition, dashboard state,
  workflow documents, and ship receipt.
- `core`, `provider`, `web`, `desktop`, and `security` retain ownership of their
  accepted Spike conclusions and later implementation/acceptance gates.
- `control-plane` is impacted by the E2EE and browser decisions but receives no
  Phase 0.5 implementation change.
- The operator owns manual dashboard judgment and has authorized this writer to
  refresh it after a persisted lifecycle transition.
- GitHub remains the remote source for merge, check, and protection facts.

## Data flow and state transitions

P1, evidence reconciliation:

1. Enumerate the exact seven expected Spike slugs.
2. Require each workflow status to equal `GATE_RESOLVED`.
3. Require each decision to point to an evidence artifact, an accepted ADR or
   compatibility result, a bounded support claim, and an explicit fallback or
   retained gate.
4. Require ADR numbers 0010-0016 to be indexed `Accepted`.
5. Classify residual requirements by owning later phase; absence is failure.

P2, project transition:

1. Set the consolidation feature from `APPROVED` to `READY_FOR_VERIFY` only
   after P1 passes and the plan/dashboard changes are complete.
2. Mark Phase 0.5 completed and Phase 1 active in manual project state.
3. Bind dashboard focus to this feature's exact lifecycle status; refresh
   generated facts using the dashboard generator.
4. An independent feature-verify role recomputes the complete matrix and writes
   only its verdict surfaces.
5. Authorized ship requires protected-main checks, merges the exact verified
   head, waits for main checks, and records `SHIPPED`.

Allowed feature transitions are
`NEEDS_REVIEW -> APPROVED -> READY_FOR_VERIFY -> READY_TO_SHIP -> SHIPPED`.
Any mismatch moves to `REVISE` or `BLOCKED`; no phase state advances on partial
evidence.

## Failure and recovery

- Missing or non-resolved Spike: retain Phase 0.5 active and route back to that
  Spike's owning role.
- Missing evidence, fallback, or version boundary: fail verification and fix
  the owning decision rather than infer support.
- Stale dashboard focus: `dashboard:verify` must fail; an operator-directed
  writer aligns the binding to the already persisted verdict.
- GitHub check or protection mismatch: do not merge; retain exact remote failure
  evidence and rerun after an in-scope correction.
- Provider/browser version drift: later phases probe exact versions and fall
  back or fail closed; this consolidation is not silently reopened.
- Merge succeeds but main checks fail: restore the last verified feature state,
  correct through a protected branch, and retain the failed run in the receipt.

## Security and privacy

Only sanitized, non-secret evidence may be referenced. No provider home,
Keychain payload, credential file, token, private key, cookie, or user account
identifier is copied into workflow or review documents. The consolidation must
preserve:

- pairwise E2EE roots and browser downgrade rules;
- authenticated/authorized local IPC and sidecar ownership;
- one canonical Codex refresh writer with revisioned CAS;
- target-profile official Claude interactive login;
- later Windows 11 and signed-package acceptance gates.

Because this unit adds no trust boundary and changes no security control, its
Security Gate is `none`; previously accepted security-review verdicts remain
authoritative for the underlying decisions.

## Compatibility and migration

No runtime or stored-data migration occurs. The documentation migration is
forward-only: Phase 0.5 moves from active to completed and Phase 1 becomes
active. Exact-version rows remain historical evidence and are not rewritten as
evergreen support. Experimental and unsupported results remain visible.

## Rollback

Before merge, revert this feature branch to restore prior project status.
After merge, use a new signed correction PR. If reconciliation later proves
wrong, mark the affected later phase gated and add a superseding ADR or
compatibility decision; do not rewrite the original Spike evidence or ADR
history.
