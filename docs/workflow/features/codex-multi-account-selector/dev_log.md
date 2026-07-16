# Development log: Codex explicit multi-account selector

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `codex-multi-account-selector` |
| Title | `Codex explicit multi-account selector` |
| Owner Module | `provider` |
| Impacted Modules | `core, security, desktop, project-system` |
| Current Phase | `PLAN` |
| Status | `NEEDS_REVIEW` |
| Executor | `Codex (GPT-5) as feature-plan` |
| Updated | `2026-07-16 17:04 PDT` |
| Suggested Next | `feature-review` |
| Branch / Worktree | `planned codex/provider/codex-multi-account-selector; planning currently on codex/provider/multi-account-usage-control` |
| Plan Version | `v1` |
| Provider Gate | `resolved for exact Linux Codex CLI 0.144.2; macOS distinct-identity and real Windows Codex remain open capability gates` |
| Security Gate | `open — explicit identity confirmation, Vault/revision binding, selector TOCTOU, scoped logout, redaction, and public raw-ID bypass are in scope` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 preview and enrollment confirmation contracts | migration 7; `awaiting_confirmation`; alias-aware auth lifecycle; Session preview/confirmation; safe errors/audit; synthetic tests | independent plan approval; P1 registry and Phase 2 runtime | migration/restart, confirmation negative matrix, no PII/secret, no public raw-ID bypass | `PLANNED` |
| P2 selector-bound exact Linux runtime | public CLI/TUI selector path; confirmed Session start; alias status/logout/re-login; exact compatibility; A/B runtime and Usage tests | P1 verified; Vault/runtime baseline | two concurrent exact Linux Accounts, immutable tuple, active-logout denial, scoped B re-login, no auto-rotation | `PLANNED` |
| P3 platform/docs/security closure | macOS/Windows typed gates; user guide/compatibility/dashboard; full platform/governance matrix; final Security Review | P2 verified | truthfully labelled capabilities, all checks pass, Security Gate accepted | `PLANNED` |

Each build run completes one approved phase and stops at `READY_FOR_VERIFY`.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-16 17:04 PDT | BRIEF/PLAN | classified owner/impacts; read parent P1 contracts, shipped Phase 2 runtime, resolved distinct-account Spike, ADR 0014 addendum, compatibility matrix and Security Review | exact Linux scope frozen; operator confirmation selected because no accepted durable non-PII upstream identity key exists; no code/branch/build action | Feature Brief; `design.md`; `api.md`; `test.md` |

## Risks and Blockers

- No phase is approved for build until independent `feature-review`.
- Exact Linux CLI `0.144.2` is the only live support claim. macOS distinct-
  identity, Windows, other versions and passive soak remain unaccepted.
- Operator confirmation is an explicit attestation, not automated upstream
  identity proof; UX and audit wording must not imply otherwise.
- Migration 7 and the public raw-ID boundary require careful compatibility
  review against the shipped Phase 2 acceptance harness.
- Live P2 may require operator participation in official login, but must not
  store identity/secret values or issue hidden quota-only requests.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-16 17:04 PDT | Codex (GPT-5) as feature-plan | Converted the accepted exact-Linux Provider decision into a three-phase selector plan: migration-backed explicit enrollment confirmation, revision-bound preview/start, alias-scoped auth/logout, no public raw-ID bypass, and truthful macOS/Windows gates | Feature Brief; `design.md`; `api.md`; `test.md`; this log | `NEEDS_REVIEW`; no production code, branch, Provider login, or support claim changed | independent `feature-review` |
| 2026-07-16 17:07 PDT | Codex root as operator-directed provider planning writer via `mad-dashboard-sync` | Added this unit to dashboard focus at exact `NEEDS_REVIEW`, regenerated workflow/dashboard facts, and verified workflow, dashboard, local links, and diff integrity | `docs/workflow/project/dashboard-state.json`; generated dashboard unchanged | all checks PASS; dashboard states no build is authorized | independent `feature-review` |
