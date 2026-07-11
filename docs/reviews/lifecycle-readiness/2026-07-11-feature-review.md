# Review record R3: lifecycle-readiness P2 — REVISE

- Date: 2026-07-11 02:26 -0700
- Role: `feature-review` (independent Codex session; verdict persisted by this role)
- Target: `lifecycle-readiness`
- Object reviewed: current uncommitted governance diff on `main`, the R2 plan,
  R2 build/verify records, workflow/dashboard contracts, all current work-unit
  state records, and the generated/static dashboard.
- Verdict: **REVISE** — do not enter `ship`.

## Classification

- Owner: `project-system` (high confidence)
- Impacts: `security`, `provider`, `core`, `desktop`, `web`, and
  `control-plane` through workflow, registry, gate, and dashboard contracts.
- Expected branch: `codex/project-system/lifecycle-readiness` unless the
  operator explicitly records another branch.
- Workflow: feature.
- Gates: workflow and dashboard verification.

## Findings

### P0-A — A security-gated Feature has no legal security-review transition

`phase0-threat-model` is a `FEATURE_DEV` record with an open Security Gate and
acceptance requiring `security-review` to emit `ACCEPTED`. The FEATURE_DEV state
machine and feature registry pipeline contain no `security-review` edge, while
`ACCEPTED` is only legal in the SPIKE workflow. The unit therefore cannot reach
its declared acceptance condition without violating the state machine. Add a
workflow-typed security review path for security-gated Features, or remodel the
unit/gate so its declared lifecycle is executable.

Evidence:

- `docs/workflow/features/phase0-threat-model/dev_log.md` Status Panel and P1
  acceptance row.
- `docs/workflow/project/workflow.md` FEATURE_DEV and SPIKE edges.
- `.agents/registry.json` feature and spike pipelines.
- `.agents/roles/security-review.md` (`ACCEPTED | REVISE | BLOCKED`).

### P0-B — An open Spike Security Gate is still bypassable

The workflow prose says an open gate makes `security-review` the only legal
writer from `EVIDENCE_READY`, but verifier v2 does not connect Security Gate,
Status, and Suggested Next. It only checks that the gate is non-`none` for a
keyword match. A gated spike may therefore be recorded as `EVIDENCE_READY`
with `Suggested Next: feature-plan` and still pass `workflow:verify`, allowing
the decision edge to bypass security review. Enforce the mutually exclusive
`EVIDENCE_READY` edge from the actual gate value and validate Suggested Next
against the selected writer.

The keyword rule is also narrower than SOP_SPIKE rule 5: it omits `remote
control` and `trust boundaries`, so those mandatory gates can remain `none`
without failing.

Evidence:

- `docs/workflow/project/workflow.md` mutually exclusive EVIDENCE_READY prose.
- `docs/workflow/SOP_SPIKE.md` rule 5.
- `scripts/workflow/verify-workflow.mjs` lines 145–195: Suggested Next is only
  checked for presence; the gate heuristic does not select an edge.

### P1-C — The Windows split still contains two owners

`spike-windows-pty-ipc` combines ConPTY/full-screen PTY work (classified as
`provider`) with Named Pipe local IPC work (classified as `core`) while naming
`core` as owner. This retains the original cross-owner defect after only
separating the Desktop sidecar. Split PTY/ConPTY from local IPC, or obtain and
record an operator boundary decision. Checking only that `Owner Module` is a
valid registry key cannot detect this drift.

Evidence:

- `.codex/skills/mad-module-classify/SKILL.md` owner definitions.
- `docs/workflow/features/spike-windows-pty-ipc/dev_log.md` Title, Owner,
  Impacted Modules, and Hypothesis.
- `scripts/workflow/verify-workflow.mjs` owner check only tests membership in
  module keys.

### P1-D — Dashboard truth and edit authority remain inconsistent

The static dashboard still describes review/verify roles as read-only, uses
non-canonical Bug statuses `FIX_READY` and `FIX_READY_FOR_VERIFY`, retains the
old `spike-intake` and `REVISE -> provider-spike` flow, and labels the P2 change
as still awaiting verification. `dashboard:verify` passes because it checks
DOM ids and the single `focus.expected_status`, not these workflow/static
fallback claims.

The authority contract also conflicts: `dev-dashboard.md` and
`FILE_STRUCTURE.md` reserve `dashboard-state.json` for humans/operator, while
AGENTS.md allows the next writer role to refresh manual state. Choose one
authority rule and enforce it.

Evidence:

- `docs/prototypes/dev-dashboard/index.html` workflow definitions and static
  copy.
- `docs/workflow/project/dashboard-state.json` status row.
- `scripts/dashboard/verify-static.mjs` focus-only consistency check.
- `AGENTS.md`, `docs/workflow/project/dev-dashboard.md`, and
  `docs/workflow/FILE_STRUCTURE.md` edit-ownership rules.

### P1-E — Superseded work-unit references remain in active artifacts

The removed `phase0-security-docs-adrs` slug is still named as the Phase 0 ADR
unit in `docs/adr/README.md` and as a non-goal dependency in the repository
layout Feature Brief. The lifecycle Feature Brief also still requires four
Phase 0 Features and five Spikes, rather than the current five and six. These
are actionable intake documents, not historical verdict records; update them
to the successor units and current acceptance scope.

## Evidence and checks

- `npm run project:verify`: pass before this verdict — agents=10, skills=3,
  docs=17, edges=18, statuses=15; dashboard branch=main, dirty=46, phases=9.
- `npm run workflow:generate`, `npm run dashboard`, and
  `npm run dashboard:verify`: pass; generation introduced no additional dirty
  paths.
- `git diff --check`: pass.
- Repository state: `main` at `342be57`, one commit ahead of `origin/main`, 46
  dirty path entries, one worktree.
- Static, edge-by-edge, gate, ownership, dashboard, and cross-document review
  produced the blockers above. Green checks do not exercise those paths.

## Scope conclusion

R2 fixes the original Bug entry, Spike REVISE destination, two named credential
gates, and much of verifier v2. It does not yet make the governance scheme
closed or ship-ready. Separately, this feature is governance readiness only;
the v0.1 product implementation has not started.

## Handoff

**Target**: `lifecycle-readiness`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: R2 closes several original findings, but security-gated Feature and Spike paths are still not executable/enforced end to end; ownership and dashboard authority also remain inconsistent.
**Findings**: P0-A add a legal security-review path for gated Features; P0-B enforce gate-selected Spike transitions and full SOP keywords; P1-C split Windows PTY from IPC; P1-D align and verify dashboard truth/authority; P1-E remove stale successor references.
**Evidence**: `project:verify`, dashboard sync checks, `git diff --check`, state-machine/code review, all current work-unit panels, and static dashboard inspection.
**Blockers**: `READY_TO_SHIP` is withdrawn; do not run `ship`.

### Next Step

Run `feature-plan` for `lifecycle-readiness`.
