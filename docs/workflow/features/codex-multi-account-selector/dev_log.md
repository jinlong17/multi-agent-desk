# Development log: Codex explicit multi-account selector

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `codex-multi-account-selector` |
| Title | `Codex explicit multi-account selector` |
| Owner Module | `provider` |
| Impacted Modules | `core, security, desktop, project-system` |
| Current Phase | `VERIFY P1` |
| Status | `READY_FOR_VERIFY` |
| Executor | `Codex (GPT-5) as feature-build P1` |
| Updated | `2026-07-16 17:26 PDT` |
| Suggested Next | `independent feature-verify P1` |
| Branch / Worktree | `codex/provider/codex-multi-account-selector @ /Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/multi-account-usage-control` |
| Plan Version | `v2` |
| Provider Gate | `resolved for exact Linux Codex CLI 0.144.2; macOS distinct-identity and real Windows Codex remain open capability gates` |
| Security Gate | `open — explicit identity confirmation, Vault/revision binding, selector TOCTOU, scoped logout, redaction, and public raw-ID bypass are in scope` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 preview and enrollment confirmation contracts | migration 7; `awaiting_confirmation`; persistent one-time Session previews; alias-aware auth lifecycle; sole preview/confirm start contract; safe errors/audit; synthetic tests | independent plan approval; P1 registry and Phase 2 runtime | migration/restart, forged/cross-client/race/replay preview matrix, confirmation negatives, no PII/secret, raw-ID start denied | `READY_FOR_VERIFY` |
| P2 selector-bound exact Linux runtime | public CLI/TUI selector path; confirmed Session start; alias status/logout/re-login; exact compatibility; A/B runtime and Usage tests | P1 verified; Vault/runtime baseline | two concurrent exact Linux Accounts, immutable tuple, active-logout denial, scoped B re-login, no auto-rotation | `PLANNED` |
| P3 platform/docs/security closure | macOS/Windows typed gates; user guide/compatibility/dashboard; full platform/governance matrix; final Security Review | P2 verified | truthfully labelled capabilities, all checks pass, Security Gate accepted | `PLANNED` |

Each build run completes one approved phase and stops at `READY_FOR_VERIFY`.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-16 17:04 PDT | BRIEF/PLAN | classified owner/impacts; read parent P1 contracts, shipped Phase 2 runtime, resolved distinct-account Spike, ADR 0014 addendum, compatibility matrix and Security Review | exact Linux scope frozen; operator confirmation selected because no accepted durable non-PII upstream identity key exists; no code/branch/build action | Feature Brief; `design.md`; `api.md`; `test.md` |
| 2026-07-16 17:26 PDT | BUILD P1 | migration 7; revision-bound `awaiting_confirmation`; alias-aware auth; persistent random preview; atomic consume/Session insert/replay; raw-ID denial; CLI gates; synthetic migration/authority/race tests | full Go/vet/race, three-OS Go builds, Web/Desktop and governance checks pass; live P2 and platform/Security gates unchanged | `p1-as-built.md`; `docs/reviews/codex-multi-account-selector/2026-07-16-p1-build.md` |

## Risks and Blockers

- P1 is built and requires independent verification before P2 can begin.
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
| 2026-07-16 17:10 PDT | Codex (GPT-5) as independent feature-review | Reviewed ownership, exact Provider scope, identity attestation, migration, preview/start authorization, raw-ID compatibility, logout races, platform gates, tests, rollback and phase ordering; modified only verdict surfaces | `docs/reviews/codex-multi-account-selector/2026-07-16-feature-review.md`; this log | `REVISE`; preview issuance/one-time consumption and the sole non-bypass start contract are not frozen; external compatibility drift timing also needs truthful wording | `feature-plan` resolves the ranked findings and re-enters `NEEDS_REVIEW` |
| 2026-07-16 17:16 PDT | Codex (GPT-5) as feature-plan | Resolved every ranked finding: migration 7 now owns random persistent client-bound preview rows with atomic consume/Session insert and replay retention; every IPC/CLI Codex start including acceptance uses preview/confirm; raw-ID-only start fails; pre-transaction and post-reservation binary drift have distinct truthful outcomes | `design.md`; `api.md`; `test.md`; this log | plan v2 `NEEDS_REVIEW`; no debug capability, product code, branch, or support claim was added | independent `feature-review` |
| 2026-07-16 17:18 PDT | Codex root as operator-directed provider planning writer via `mad-dashboard-sync` | Regenerated workflow/dashboard facts after plan v2, retained dashboard focus at exact `NEEDS_REVIEW`, and verified workflow, dashboard, links, and diff integrity | generated dashboard unchanged; this log | all checks PASS; no build authorization inferred | independent `feature-review` |
| 2026-07-16 17:21 PDT | Codex (GPT-5) as independent feature-review | Re-reviewed only plan v2 against every ranked v1 finding and the full feature contract; confirmed persistent one-time previews, the sole non-bypass start path, truthful compatibility-race outcomes, migration/recovery, security, tests and rollback; modified only verdict surfaces | `docs/reviews/codex-multi-account-selector/2026-07-16-feature-review-v2.md`; this log | `APPROVED` for P1; no remaining finding; P2 live, platform and final Security gates remain dependencies | `feature-build` P1 only |
| 2026-07-16 17:24 PDT | Codex root as operator-authorized feature-build P1 | Created the approved short-lived Provider branch and entered P1 without changing exact-version, platform, live P2, or Security gates | branch `codex/provider/codex-multi-account-selector`; this log | P1 build active; no product file changed yet | implement approved P1 then stop at `READY_FOR_VERIFY` |
| 2026-07-16 17:25 PDT | Codex root as operator-directed feature-build writer via `mad-dashboard-sync` | Rebound dashboard branch/focus/manual state to exact `APPROVED` P1 build, regenerated machine facts, and verified workflow, dashboard, links, and diff integrity | `docs/workflow/project/dashboard-state.json`; generated dashboard unchanged | all checks PASS; later phases and Security Gate remain explicit | implement P1 |
| 2026-07-16 17:26 PDT | Codex (GPT-5) as feature-build P1 | Implemented the approved synthetic contract slice: migration 7, revision-bound typed login confirmation, persistent owner-bound one-time Session previews, atomic preview consumption/Session insertion and replay, alias status/logout, public raw-ID denial, exact Linux preflight, CLI confirmation, and adversarial tests | product/storage/CLI/tests; `p1-as-built.md`; P1 build report | `READY_FOR_VERIFY`; full Go/vet/race, three-OS builds, Web/Desktop checks pass; no live Provider process, platform acceptance, Security closure, push, merge, or ship | independent `feature-verify` P1 |
| 2026-07-16 17:26 PDT | Codex root as operator-directed feature-build writer via `mad-dashboard-sync` | Rebound dashboard manual focus to exact `READY_FOR_VERIFY`, regenerated workflow/dashboard facts, and verified workflow, dashboard, layout, Actions, CODEOWNERS, fixtures, links, licenses and diff integrity | `docs/workflow/project/dashboard-state.json`; `docs/prototypes/dev-dashboard/state.generated.js`; this log | all governance checks PASS; dashboard does not infer P2 authorization | independent `feature-verify` P1 |
