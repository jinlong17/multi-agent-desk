# Review record R2-plan: lifecycle-readiness P2 plan — APPROVED

- Date: 2026-07-10
- Role: `feature-review` (independent session; verdict persisted by this role)
- Target: `lifecycle-readiness`, phase P2 "state machine closure and semantic
  verifier v2"
- Object under review: the revised plan (design.md "Revision R2", api.md,
  test.md, dev_log.md at `NEEDS_REVIEW`). The P2 implementation has not been
  built; this review approves the plan, not a diff.
- Verdict: **APPROVED** — P2 is executable by `feature-build` without
  inventing decisions, subject to the derivable implementation notes below.

## Scope reviewed

- `docs/workflow/features/lifecycle-readiness/design.md` §"Revision R2"
- `docs/workflow/features/lifecycle-readiness/api.md`, `test.md`, `dev_log.md`
- Triggering verdict: `docs/reviews/lifecycle-readiness/2026-07-10-feature-review-r2.md`
- Context: `docs/workflow/project/workflow.md`, `AGENTS.md`,
  `docs/workflow/SOP_BUGFIX.md`, `docs/workflow/SOP_SPIKE.md`,
  `.agents/registry.json`, `.agents/roles/*.md`,
  `scripts/workflow/verify-workflow.mjs`, `scripts/dashboard/verify-static.mjs`,
  `scripts/dashboard/generate-state.mjs`,
  `docs/workflow/project/{module-registry,dashboard-state}.json`,
  `docs/workflow/templates/{dev_log,spike_log}.md`, all 10 feature/spike logs.

## Finding-by-finding resolution check

### P0-A (bug workflow entry) — resolved by plan

Design R2 items 1–2 add the edge `(BUGFIX, DRAFT, bug-diagnose) → DIAGNOSED |
BLOCKED` plus a `bug_log.md` template (Workflow `BUGFIX`, Suggested Next
`bug-diagnose`, reproduction fields) verified for the same Status Panel
fields. This matches SOP_BUGFIX step 2 and the existing `bug-diagnose`
handoff/registry tokens (`DIAGNOSED | BLOCKED`). test.md row "Bug workflow has
legal entry" makes it a standing verifier assertion.

### P0-B (spike REVISE loop) — resolved by plan

`(SPIKE, REVISE, feature-plan) → SPIKE_READY | BLOCKED`, with an explicit
verifier assertion that `(SPIKE, REVISE)` never targets `NEEDS_REVIEW`.
Choosing no new state is sound: it avoids churning the canonical 15-status
set, and `feature-plan` applying reviewer findings before re-entering
`SPIKE_READY` matches the existing `INCONCLUSIVE` re-scope pattern.

### P0-C (spike security gates) — resolved by plan

Both named spikes open their Security Gate; SOP_SPIKE rule 5 is indeed
unconditional for credentials/keys/auth, and the rejected alternative is
recorded. Cross-checked the full spike corpus: `spike-browser-key-storage`
and `spike-e2ee-protocol-vectors` already carry open gates, and the post-split
Windows spikes (ConPTY/Named Pipe; Tauri sidecar) trip none of the planned
keywords, so the new heuristic will pass with no false negatives or positives
on the current corpus.

### P1-D (single-owner splits) — resolved by plan

Both splits name owner, scope, and impacted modules and align with
`module-registry.json` ownership (`docs/THREAT_MODEL.md` → `security`;
ConPTY/PTY/IPC → `core`; Tauri sidecar → `desktop`). See note N3 on the
mechanical disposition of the superseded unit directories.

### P1-E (dashboard focus binding) — resolved by plan

`manual.focus: [{slug, expected_status}]` checked by `verify-static.mjs`
against generated feature logs is directly executable: the generator passes
`manual` through wholesale and `feature_logs` already carry `slug` + `status`.
See note N5 on the refresh actor.

### P1-F (process conformance) — resolved by plan and by execution

The R1 deviation stays on record in the Work Log (not erased), and this very
review is the committed independent `feature-review` persisting its own
verdict, demonstrating the loop is closed in practice.

### P1-G (verifier v2 edge semantics) — resolved by plan

The check list (edge parsing; canonical-set equivalence; required initial
edges per workflow; spike-REVISE target; role-emitted statuses backed by
authoring edges; registry↔handoff bidirectional equality; feature-log
`(Workflow, Status)` legality; owner-module validity; security-gate
heuristics) covers every gap the R2 review named. Feasibility verified against
the current repository: all 10 registry `output` token sets already equal
their role handoff tokens, so the bidirectional check will not be born red;
`verify-workflow.mjs` already parses Status Panels, and the four-column table
shape mandated by api.md is a parseable contract.

## State-machine completeness analysis

Enumerating the planned workflow-typed edges (existing §3 rows typed by
workflow, plus the R2 additions):

- `FEATURE_DEV` Current set {DRAFT, NEEDS_REVIEW, REVISE, APPROVED,
  READY_FOR_VERIFY, VERIFIED, READY_TO_SHIP}; terminal-Next {SHIPPED, BLOCKED}.
- `BUGFIX` Current set {DRAFT, DIAGNOSED, READY_FOR_VERIFY, READY_TO_SHIP};
  terminal-Next {SHIPPED, BLOCKED}.
- `SPIKE` Current set {DRAFT, SPIKE_READY, EVIDENCE_READY, ACCEPTED, REVISE,
  INCONCLUSIVE}; terminal-Next {GATE_RESOLVED, BLOCKED}.

The union of Current and Next columns covers exactly the 15 canonical
statuses; every status is reachable from its workflow's `DRAFT` entry; each
role's emitted statuses have at least one authoring edge. Keeping `BLOCKED`
recovery as prose is correct: "the named clearing role restores the last
non-blocked status" is not a fixed `(current, writer, next)` triple and would
be false if forced into a row; as a never-Current status, `BLOCKED` also
falls out naturally as a legal resting status under the "Current or terminal
Next" log-state check.

## Implementation notes (binding but derivable — not plan defects)

- **N1 (conditional writer row).** The current `EVIDENCE_READY` row names a
  conditional writer. Under the four-column parseable contract plus the
  "edge naming that role as writer" check, it must split into two rows:
  `(SPIKE, EVIDENCE_READY, security-review) → ACCEPTED | REVISE | BLOCKED`
  (gated) and `(SPIKE, EVIDENCE_READY, feature-plan) → GATE_RESOLVED`
  (ungated), keeping the gating condition as prose that does not break the
  single-role Writer token.
- **N2 (terminal Next definition).** Define "terminal Next" operationally in
  the verifier: a Next status that appears as no row's Current within the same
  workflow (yields SHIPPED, BLOCKED, GATE_RESOLVED). This makes the log-state
  check deterministic.
- **N3 (superseded units).** The split replaces `phase0-security-docs-adrs`
  and `spike-windows-conpty-sidecar`: remove or supersede their state
  directories and carry briefs for the four successor units, or the dashboard
  and log-state checks will report stale/duplicate scope.
- **N4 (bug_log gate fields).** Model `bug_log.md` on `dev_log.md` including
  the Provider/Security Gate fields, so the "security-owned log must not have
  `Security Gate: none`" rule cannot be vacuously satisfied by a missing
  field; alternatively treat a missing field on a security-owned log as
  failure.
- **N5 (focus refresh actor).** `dashboard-state.json` is the manual-judgment
  file; verdict writers are limited to two files and cannot refresh `focus`.
  "Refreshed at each verdict" therefore falls to the operator or the next
  writer role; the builder should state this actor in the dashboard contract
  text when implementing item 6 so a post-verdict red `dashboard:verify` has a
  named resolver.
- **N6 (keyword matching).** Apply the credential/auth/key/token/secret/
  keychain/E2EE heuristic case-insensitively to the Title and Hypothesis
  fields only, as specified — not to free text — to keep the current corpus
  free of false positives.

## Nits (no action required before build)

- dev_log Phase Plan says "9 new rows" in test.md; the acceptance matrix adds
  8 new rows (plus one modified template row). Count either way at
  verification; do not let the label drift the evidence.
- `AGENTS.md` "State authority" mentions only the spike template; when
  `bug_log.md` lands, that sentence should mention it (mechanical doc
  consistency within the already-scoped files).

## Evidence

- Files listed under "Scope reviewed", read in full in this session.
- Registry↔handoff token equality checked manually for all 10 agents.
- Spike gate corpus check across all 5 spike logs.
- `scripts/dashboard/generate-state.mjs` confirmed to pass `manual` through
  and emit `feature_logs` with `slug`/`status` (focus check executable).

## Handoff

**Target**: `lifecycle-readiness`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: The R2 plan fully resolves P0-A/B/C and P1-D/E/F/G; the
workflow-typed edge table is complete and unambiguous over the 15 canonical
statuses with prose BLOCKED recovery; verifier v2 checks are executable as
specified; P2 is buildable without inventing decisions.
**Findings**: No blocking findings. Binding-but-derivable implementation
notes N1–N6 and two nits recorded above.
**Evidence**: See "Evidence" section; key checks: registry↔handoff equality
(10/10), status-set closure (15/15), spike gate corpus (5/5), dashboard
generator pass-through.
**Blockers**: none

### Next Step

Run `feature-build` for `lifecycle-readiness`.
