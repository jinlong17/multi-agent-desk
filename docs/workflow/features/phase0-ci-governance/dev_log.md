# Development log: Phase 0 — CI and remote governance

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `phase0-ci-governance` |
| Title | `Phase 0 — CI and remote governance` |
| Owner Module | `project-system` |
| Impacted Modules | `none` |
| Current Phase | `FEATURE_BUILD_P2` |
| Status | `BLOCKED` |
| Executor | `Codex as feature-build` |
| Updated | `2026-07-14` |
| Suggested Next | `operator reinstalls the Chrome plugin and confirms ready, or provides authenticated GitHub API/CLI; then feature-build resumes P2` |
| Branch / Worktree | `codex/project-system/phase0-ci-governance @ agent-deck-worktrees/phase0-ci-governance` |
| Plan Version | `v0.2` |
| Provider Gate | `none` |
| Security Gate | `none (CI holds no provider credentials — verify in review)` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 local CI contracts + gates | read-only 3-platform workflows, exact required job names, CODEOWNERS generator, DCO/local-link/license validators and negative fixtures | shipped scaffold | static contracts and positive/negative local evidence | `VERIFIED` |
| P2 remote Actions + governance | authorized push/test PR, clean and GPL-fail Actions runs, strict main protection, Actions/release-permission audit and readback | P1 verified; explicit operator authorization | seven checks proven; remote rules/permissions match and receipt persisted | `BLOCKED` |

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-11 | P1 build | `npm run ci:verify` | pass: seven-name Actions contract, deterministic CODEOWNERS, positive/negative fixtures, 133 Markdown files, five pnpm license groups and 418 Cargo packages | `.github/`, `scripts/ci/` |
| 2026-07-11 | P1 build | Ruby 2.6 `YAML.safe_load` on both workflows | pass | `.github/workflows/*.yml` |
| 2026-07-11 | P1 build | `npm run scaffold:verify` | fail at `gofmt`: local Go toolchain absent; remaining scaffold checks unknown in this run | command output retained in task |
| 2026-07-11 | P1 build | `npm run project:verify`; `git diff --check` | pass | generated dashboard state; working tree |
| 2026-07-11 | P1 verify | offline frozen install; `npm run ci:verify`; DCO range; YAML parse; temporary `go-licenses v2.0.1`; pinned-Go `npm run scaffold:verify`; `npm run project:verify`; conflict/diff scans | pass; GitHub runner and remote evidence remain unknown | `docs/reviews/phase0-ci-governance/2026-07-11-feature-verify-p1.md` |
| 2026-07-14 | P2 remote build | PR #1 and remote refs; intermediate Actions runs `29314251246`/`29314251259` and `29314803988`/`29314804058`; GPL head `6811788`; recovery head `22e2240`; Actions/settings readback | intermediate Linux/macOS/governance checks passed and two Windows defects were fixed; final clean/GPL run IDs and strict `main` rule remain unproven | `remote-receipt.md` |
| 2026-07-14 | P2 local recheck | workflow/dashboard generators and verifiers; Actions/CODEOWNERS/fixtures/links; DCO range; pnpm 10.23.0 + Cargo license inventory; `git diff --check` | pass: checks=7, actions=15, Markdown=134, pnpm groups=5, Cargo packages=418, commits=21 with three exact pre-policy exceptions | command output retained in task; `remote-receipt.md` |
| 2026-07-14 | P2 blocker reproduction | Chrome running, extension installed/enabled, native host valid; browser-client `openTabs()` retried twice | connection timed out; operator approval required by browser recovery policy before opening a fresh window | `remote-receipt.md` |
| 2026-07-14 01:11 -0700 | P2 authorized browser recovery | operator-authorized fresh Chrome window; two-second wait; one permitted reconnect and `openTabs()` retry | retry timed out; Chrome troubleshooting now requires plugin reinstall and forbids alternate automation fallback | `remote-receipt.md` |

## Risks and Blockers

- Remote settings need operator permissions (human gate).
- P1 static workflow validation is not runner evidence; all GitHub checks remain
  unknown until P2.
- The local machine currently lacks Go/gofmt. The combined scaffold rerun failed
  at that prerequisite; no downstream scaffold result is inferred.
- The final GPL-negative and clean-recovery Actions run IDs/results are unknown
  until authenticated GitHub readback is restored.
- The operator-authorized fresh-window recovery failed on its single permitted
  retry; the Chrome plugin must now be reinstalled before browser work resumes.
- `main` has no proven protection rule. Applying and reading back the exact rule
  remains mandatory; the rule may also expose a second-approver operational
  requirement for this single-owner repository.
- The pre-mutation value of the full-length Action SHA setting was not persisted,
  so exact rollback parity for that one setting remains unknown.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Unit created from Phase 0 breakdown | this file + brief | `DRAFT` | feature-plan |
| 2026-07-11 | Codex as feature-plan | Planned P1 read-only SHA-pinned workflows and deterministic positive/negative gate validators; planned P2 exact seven checks, clean/GPL test-PR runs, strict main protection and read-only Actions permission readback behind explicit human authorization; dashboard focus refreshed by operator direction | `design.md`, `api.md`, `test.md`, `dev_log.md`, `dashboard-state.json` | `NEEDS_REVIEW`; remote evidence unknown | feature-review |
| 2026-07-11 | Codex as independent feature-review | Reviewed stable check names, matrix/platform setup, permissions/caches, DCO ranges, SPDX/Go licenses, link gates, CODEOWNERS, supply-chain pins, remote read/write/readback and rollback; no blocking finding; eight builder notes recorded | `docs/reviews/phase0-ci-governance/2026-07-11-feature-review.md`, this file | `APPROVED` | feature-build P1 |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the independently persisted `APPROVED` verdict | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | feature-build P1 |
| 2026-07-11 | Codex as feature-build | Built only approved P1: read-only SHA-pinned Linux/macOS/Windows workflows with seven stable check names; registry-derived CODEOWNERS; DCO, local-link, pnpm/Cargo SPDX and Go license gates; deterministic positive/negative fixtures; retained missing-local-Go scaffold failure and made no remote change | `.github/`, `scripts/ci/`, `package.json`, this file | `READY_FOR_VERIFY`; runner evidence remains unknown | feature-verify P1 |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the P1 build handoff; remote Actions and protection state remain unknown | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | feature-verify P1 |
| 2026-07-11 | Codex as independent feature-verify | Independently verified workflow syntax/contracts/pins, CODEOWNERS, positive/negative DCO/link/SPDX/policy fixtures, actual pnpm/Cargo inventory, signed P1 range, pinned Go license scanner, full macOS scaffold regression, project integrity, and Windows Spike deferral; made no implementation or dashboard write | `docs/reviews/phase0-ci-governance/2026-07-11-feature-verify-p1.md`, this file | `VERIFIED`; all remote evidence unknown | feature-build P2 after explicit operator authorization |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the independently persisted P1 `VERIFIED` verdict; retained P2 remote authorization as next action | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | request explicit P2 remote authorization |
| 2026-07-14 00:59 -0700 | Codex as feature-build P2 and operator-directed dashboard writer | Pushed/opened PR #1, retained two Windows runner failures and fixes, exercised GPL/recovery heads, read back least-privilege Actions settings, reproduced the authenticated-browser connection failure, persisted a sanitized partial receipt, and refreshed dashboard focus without inferring final remote success | `remote-receipt.md`, this file, `docs/workflow/project/dashboard-state.json`, generated dashboard state | `BLOCKED`; final seven-check/GPL evidence and strict `main` protection remain unknown | operator authorizes a fresh Chrome window or supplies authenticated GitHub API/CLI access; resume feature-build P2 |
| 2026-07-14 01:11 -0700 | Codex as feature-build P2 and operator-directed dashboard writer | Used the operator-authorized fresh-window recovery exactly once, retained the failed connection as evidence, and refreshed the dashboard next action without weakening the remote acceptance criteria | `remote-receipt.md`, this file, `docs/workflow/project/dashboard-state.json`, generated dashboard state | remains `BLOCKED`; Chrome troubleshooting requires plugin reinstall | operator reinstalls the Chrome plugin and confirms ready, or supplies authenticated GitHub API/CLI access |
