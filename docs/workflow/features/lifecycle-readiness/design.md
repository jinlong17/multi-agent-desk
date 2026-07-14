# Design: Lifecycle readiness hardening

## Decision snapshot

- Selected option: fix governance in place — ADR for layout authority,
  verdict-writer roles, feature-plan-owned spike intake/decision, semantic
  verifier — instead of adding an orchestrator/state-recorder role.
- Review evidence:
  `docs/reviews/lifecycle-readiness/2026-07-10-lifecycle-readiness-review.md`.
- Frozen assumptions: `docs/IMPLEMENTATION_PLAN.md` §17 is the physical layout
  authority (ADR 0009); `dev_log.md` stays the single state authority.
- Rejected alternatives:
  - Separate `state-recorder` role: adds a hop between verdict and
    persistence, recreating the atomicity gap it was meant to fix.
  - New `spike-intake` role: intake is planning work; a dedicated role would
    duplicate `feature-plan` context for no isolation benefit.
  - Keeping reviewers fully read-only with "next writer records the verdict":
    the verdict would be persisted by a party with an incentive to reframe it.

## Context and boundaries

Owner: `project-system`. Files: `docs/adr/*`, `docs/workflow/**`,
`.agents/**`, `scripts/workflow/*`, `scripts/dashboard/*`, `CLAUDE.md`,
`AGENTS.md`, feature/spike state records. No product code.

## Components and ownership

1. **Layout authority (P0-1)** — `docs/adr/0009-repository-layout-authority.md`;
   `CLAUDE.md` boundaries and `module-registry.json` `owns` rewritten to
   physical paths; module keys unchanged.
2. **Verdict writers (P0-2)** — `feature-review`, `feature-verify`,
   `bug-verify`, `security-review` persist their own verdict atomically with
   exactly two writable surfaces: `docs/reviews/<slug>/<date>-<role>.md` and
   the target `dev_log.md` (status + one Work Log row). Registry mode
   `verdict-writer`, sandbox `workspace-write`.
3. **Spike closure (P0-3)** — `feature-plan` owns intake (`SPIKE_READY`) and
   decision (`GATE_RESOLVED`); `spike_log.md` template added; state machine
   gains `ACCEPTED`, `INCONCLUSIVE` re-scope, and a `BLOCKED` recovery rule
   (clearing role restores the last non-blocked status).
4. **Phase breakdown (P0-4)** — Phase 0 split into
   `phase0-repository-layout`, `phase0-monorepo-scaffold`,
   `phase0-ci-governance`, `phase0-security-docs-adrs`; Phase 0.5 split into
   `spike-codex-auth-refresh`, `spike-claude-config-keychain`,
   `spike-browser-key-storage`, `spike-windows-conpty-sidecar`,
   `spike-e2ee-protocol-vectors`.
5. **Semantic verification (P1-5)** — mirror rendering extracted to
   `scripts/workflow/render-mirrors.mjs`; `verify-workflow.mjs` compares full
   mirror content, checks role-handoff statuses against registry outputs and
   the state machine, checks template Status Panel fields, and parses every
   feature dev_log; `verify-static.mjs` fails on `MISSING_DEV_LOG` or
   unparseable fields.

## Data flow and state transitions

State machine table in `docs/workflow/project/workflow.md` §3 is the single
transition authority; the verifier asserts all fifteen canonical statuses
appear there and that no role emits a non-canonical status.

## Failure and recovery

`BLOCKED` names a reproducible condition and clearing role; the clearing role
appends evidence and restores the last non-blocked status. Verifier failures
name the drifted file and the regeneration command.

## Security and privacy

No trust-boundary changes. Verdict writers gain filesystem write; scope is
constrained by role contract (two files) and reviewed diffs — accepted
residual risk, recorded in the brief.

## Compatibility and migration

Existing generated mirrors regenerate byte-identically from the registry;
`schema_version` of the module registry bumps to 2 (adds
`layout_authority`); no consumer reads that field yet.

## Pre-release gates deferred out of this feature

Release candidate/rollback, backup/restore drills, upgrade and
migration-failure policy, performance budgets/SLOs, incident response,
signing/SBOM/provenance, deployment verification, and maintenance workflows
are Phase 6 gates and must be planned as their own feature units before v0.1.

## Revision R2 (2026-07-10, after REVISE — see 2026-07-10-feature-review-r2.md)

Decisions for phase P2 "state machine closure and semantic verifier v2":

1. **Workflow-typed state machine (P0-A, P0-B).** The §3 table gains a
   `Workflow` column; every edge is `(workflow, current, writer, next)`.
   - `BUGFIX` entry: `DRAFT → bug-diagnose → DIAGNOSED | BLOCKED`.
   - Spike revise loop: `(SPIKE, REVISE, feature-plan) → SPIKE_READY |
     BLOCKED` — a revised spike re-enters `SPIKE_READY`, never
     `NEEDS_REVIEW`. No new state; `feature-plan` applies the reviewer
     findings to hypothesis/criteria before re-run.
   - `BLOCKED` recovery stays prose (not a table row): the named clearing
     role restores the last non-blocked status.
2. **Bug log template (P0-A).** New `docs/workflow/templates/bug_log.md`
   (Workflow `BUGFIX`, Suggested Next `bug-diagnose`, reproduction fields);
   verified for the same Status Panel fields as the other templates.
3. **Spike security gates (P0-C).** `spike-codex-auth-refresh` and
   `spike-claude-config-keychain` open their Security Gate per SOP_SPIKE
   rule 5. Rejected alternative: leaving gates to intake judgment — the SOP
   wording is unconditional for credentials/keys/auth.
4. **Single-owner splits (P1-D).**
   - `phase0-security-docs-adrs` → `phase0-architecture-adrs`
     (`project-system`: ADR 0001–0008, RESEARCH_LOG, PROVIDER_COMPATIBILITY
     placeholder) + `phase0-threat-model` (`security`: THREAT_MODEL.md with
     open security gate).
   - `spike-windows-conpty-sidecar` → `spike-windows-pty-ipc` (`core`:
     ConPTY full-screen TUI + Named Pipe IPC prototype; impacted provider,
     desktop) + `spike-windows-desktop-sidecar` (`desktop`: Tauri sidecar
     lifecycle; impacted core).
5. **Semantic verifier v2 (P1-G).** `verify-workflow.mjs` parses the §3
   table into edges and asserts:
   - table statuses ≡ canonical status set (both directions);
   - required initial edges: `(FEATURE_DEV, DRAFT, feature-plan,
     NEEDS_REVIEW)`, `(BUGFIX, DRAFT, bug-diagnose, DIAGNOSED)`,
     `(SPIKE, DRAFT, feature-plan, SPIKE_READY)`;
   - `(SPIKE, REVISE)` targets `SPIKE_READY` and never `NEEDS_REVIEW`;
   - every status a role handoff emits is produced by ≥1 edge naming that
     role as writer; registry `output` tokens ≡ role handoff tokens
     (bidirectional);
   - every feature log's `(Workflow, Status)` is a known workflow and a
     `Current` or terminal `Next` status of that workflow;
   - `Owner Module` is a module-registry key;
   - spike logs whose Hypothesis/Title mentions
     credential/auth/key/token/secret/keychain/E2EE must not have
     `Security Gate: none`; any log owned by `security` must not have
     `Security Gate: none`.
   `verify-static.mjs` adds: `manual.focus` entries (`{slug,
   expected_status}`) must match the generated feature logs.
6. **Dashboard manual state (P1-E).** `dashboard-state.json` gains a
   `focus` array binding manual judgment to concrete feature statuses; the
   manual status/next-action text is refreshed at each verdict.
7. **Process conformance (P1-F).** This revision itself runs
   plan → independent `feature-review` → build → independent
   `feature-verify`, and each verdict writer persists its own verdict. The
   R1 deviation (operator-approved skip of review; parent-persisted verify
   verdict) stays on record in the Work Log and is not repeated.

## Revision R3 (2026-07-11, after REVISE — see 2026-07-11-feature-review.md)

Decisions for phase P3 "security-gate execution paths and dashboard truth":

1. **FEATURE_DEV security-gate closure (P0-A).** Mirror the spike gate
   pattern at the ship boundary. New/changed edges:
   - `(FEATURE_DEV, READY_TO_SHIP, security-review) → ACCEPTED | REVISE |
     BLOCKED` — the only legal writer when the Security Gate is open.
   - `(FEATURE_DEV, READY_TO_SHIP, ship) → SHIPPED | BLOCKED` — only when
     the gate is `none` or `resolved` (existing row, now gate-scoped).
   - `(FEATURE_DEV, ACCEPTED, ship) → SHIPPED | BLOCKED` — ship after
     security acceptance; `security-review` sets the gate to `resolved`
     when it writes `ACCEPTED` (within its existing two-file scope).
   - A security `REVISE` reuses `(FEATURE_DEV, REVISE, feature-plan)`.
   Registry `workflows.feature` gains `security-review`; the role file
   documents the feature-gate duty. This makes `phase0-threat-model`'s
   declared acceptance executable. Rejected alternative: gating at
   `READY_FOR_VERIFY` — it would conflate functional verification with
   security acceptance and give the state two verdict writers at once.
2. **Gate-selected transitions enforced (P0-B).** Verifier v3:
   - Generic Suggested-Next legality: for every log whose `(Workflow,
     Status)` is non-terminal, Suggested Next must name at least one agent,
     and every agent it names must be a legal writer of an edge from that
     `(Workflow, Status)`.
   - Gate linkage: `SPIKE` at `EVIDENCE_READY` with gate `open…` →
     Suggested Next must name `security-review` and must not name
     `feature-plan`; gate `none`/`resolved` → must name `feature-plan`.
     `FEATURE_DEV` at `READY_TO_SHIP` with gate `open…` → must name
     `security-review` and must not name `ship`; otherwise must name `ship`.
   - Keyword heuristic extended to the full SOP_SPIKE rule 5 list:
     `credential|auth|key|token|secret|keychain|e2ee|remote control|trust
     boundar`.
3. **Windows single-owner re-split (P1-C).** `spike-windows-pty-ipc`
   (mixed provider/core) is replaced by:
   - `spike-windows-conpty` (`provider`; impacted `core, desktop`): ConPTY
     hosting a full-screen provider TUI (render, resize, replay).
   - `spike-windows-named-pipe-ipc` (`core`; impacted `desktop`): Named
     Pipe daemon IPC minimal prototype.
   Classification follows the module registry signals (ConPTY/PTY →
   `provider`; IPC → `core`).
4. **One dashboard authority rule + truthful static fallback (P1-D).**
   Chosen rule: `docs/workflow/project/dashboard-state.json` is operator
   judgment. A refresh may be executed by an operator-directed writer
   session, which must record the refresh in the target unit's Work Log;
   verdict writers never touch it. `AGENTS.md`, `dev-dashboard.md`, and
   `FILE_STRUCTURE.md` all state this one rule. The static fallback in
   `index.html` is updated to verdict-writer semantics, canonical BUGFIX
   statuses (`DIAGNOSED`, `READY_FOR_VERIFY`), and the current spike flow
   (`feature-plan` intake → `provider-spike` → gated `security-review` →
   `feature-plan` decision; `REVISE → SPIKE_READY`). `verify-static.mjs`
   adds a stale-token blacklist (`FIX_READY`, `FIX_READY_FOR_VERIFY`,
   `spike-intake`, `只读 Agent`) and requires `DIAGNOSED` +
   `READY_FOR_VERIFY` in the bug flow copy.
5. **Successor-reference cleanup (P1-E).** `docs/adr/README.md` and the
   `phase0-repository-layout` brief point to `phase0-architecture-adrs`;
   the lifecycle brief scope reads five `phase0-*` feature units and seven
   Phase 0.5 spike units. Historical records (verdict files, Work Log rows)
   are not rewritten.

## Rollback

Single git revert of the governance commit; no data or code migration.
