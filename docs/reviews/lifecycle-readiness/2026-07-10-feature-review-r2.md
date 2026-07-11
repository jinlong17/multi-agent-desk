# Review record R2: lifecycle-readiness — REVISE

- Date: 2026-07-10
- Role: `feature-review` (independent session, delivered via operator; verdict
  recorded verbatim below)
- Target: `lifecycle-readiness`
- Verdict: **REVISE** — must not enter `ship`

## Confirmed as fixed

Layout authority, verdict-writer roles, full-content mirror verification,
template fields, and the 10 work units are in place; `workflow:verify`,
`dashboard:verify`, and `git diff --check` pass.

## Blocking findings

### P0-A: Bug workflow has no legal entry transition

A bug log starts at `DRAFT`, but `DRAFT` only allows `feature-plan` →
`NEEDS_REVIEW`/`SPIKE_READY`; there is no `bug-diagnose → DIAGNOSED` entry.
Bugs also reuse the `FEATURE_DEV` template whose Suggested Next is
`feature-plan`, contradicting SOP_BUGFIX. Add a bug-specific template and a
workflow-aware `DRAFT → DIAGNOSED` transition owned by `bug-diagnose`.

### P0-B: Spike `REVISE` routes into the feature pipeline

`security-review` can emit `REVISE` from `EVIDENCE_READY`, but the only
`REVISE` transition returns `NEEDS_REVIEW`, sending a spike into feature
review. A revised spike must return to `SPIKE_READY` (or use a dedicated
state).

### P0-C: Two provider spikes violate the spike security-gate SOP

SOP_SPIKE rule 5 requires `security-review` when credentials, keys, or auth
are affected, yet `spike-codex-auth-refresh` (file credential store, device
auth, concurrent refresh) and `spike-claude-config-keychain` (Keychain,
setup-token, revocation) both say `Security Gate: none`. Both must open the
gate.

### P1-D: Work units violate the single-owner rule

- `phase0-security-docs-adrs` (owner `project-system`) contains
  `docs/THREAT_MODEL.md`, which the module registry assigns to `security`.
  Split into `phase0-architecture-adrs` (`project-system`) and
  `phase0-threat-model` (`security`).
- `spike-windows-conpty-sidecar` spans provider/core/desktop but is owned by
  `desktop`. Split into a Windows PTY/IPC spike and a Desktop sidecar spike.

### P1-E: Dashboard manual state is stale and unchecked

Manual state still says "awaiting independent verification / next:
feature-verify" while the feature log reads `READY_TO_SHIP`. The verifier
checks field existence, not manual-vs-log consistency.

### P1-F: This feature bypassed its own process

After `feature-plan`, the operator wrote `APPROVED` directly with no
independent `feature-review`; the verifier's verdict was persisted by the
parent. Human authorization may gate priority, branches, or ship — it cannot
replace mandatory independent review, and verdict writers must persist their
own verdicts.

### P1-G: Semantic verification is still shallow

Status tokens are only checked for presence anywhere in `workflow.md`. Parse
the state-machine table and validate `(workflow, current, writer, next)`
edges, per-workflow initial transitions, registry↔handoff bidirectional
consistency, security-gate declarations, owner-module validity, and
dashboard-manual-vs-log consistency.

## Handoff

**Target**: `lifecycle-readiness`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: Original P0 fixes mostly landed, but bug entry, spike REVISE
loop, security gates, single ownership, dashboard state, and this feature's
own review order are not closed.
**Findings**: P0 A–C; P1 D–G above.
**Evidence**: `workflow:verify`, `dashboard:verify`, `git diff --check` pass;
edge-by-edge state-machine review found the reproducible gaps above.
**Blockers**: `READY_TO_SHIP` does not hold; do not run `ship`.

### Next Step

Run `feature-plan` for `lifecycle-readiness`.
