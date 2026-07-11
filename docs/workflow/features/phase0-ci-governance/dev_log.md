# Development log: Phase 0 — CI and remote governance

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `phase0-ci-governance` |
| Title | `Phase 0 — CI and remote governance` |
| Owner Module | `project-system` |
| Impacted Modules | `none` |
| Current Phase | `FEATURE_BUILD_P1` |
| Status | `READY_FOR_VERIFY` |
| Executor | `Codex as feature-build` |
| Updated | `2026-07-11` |
| Suggested Next | `feature-verify P1` |
| Branch / Worktree | `codex/project-system/phase0-ci-governance @ agent-deck-worktrees/phase0-ci-governance` |
| Plan Version | `v0.2` |
| Provider Gate | `none` |
| Security Gate | `none (CI holds no provider credentials — verify in review)` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 local CI contracts + gates | read-only 3-platform workflows, exact required job names, CODEOWNERS generator, DCO/local-link/license validators and negative fixtures | shipped scaffold | static contracts and positive/negative local evidence | `READY_FOR_VERIFY` |
| P2 remote Actions + governance | authorized push/test PR, clean and GPL-fail Actions runs, strict main protection, Actions/release-permission audit and readback | P1 verified; explicit operator authorization | seven checks proven; remote rules/permissions match and receipt persisted | `PLANNED` |

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-11 | P1 build | `npm run ci:verify` | pass: seven-name Actions contract, deterministic CODEOWNERS, positive/negative fixtures, 133 Markdown files, five pnpm license groups and 418 Cargo packages | `.github/`, `scripts/ci/` |
| 2026-07-11 | P1 build | Ruby 2.6 `YAML.safe_load` on both workflows | pass | `.github/workflows/*.yml` |
| 2026-07-11 | P1 build | `npm run scaffold:verify` | fail at `gofmt`: local Go toolchain absent; remaining scaffold checks unknown in this run | command output retained in task |
| 2026-07-11 | P1 build | `npm run project:verify`; `git diff --check` | pass | generated dashboard state; working tree |

## Risks and Blockers

- Remote settings need operator permissions (human gate).
- P1 static workflow validation is not runner evidence; all GitHub checks remain
  unknown until P2.
- The local machine currently lacks Go/gofmt. The combined scaffold rerun failed
  at that prerequisite; no downstream scaffold result is inferred.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Unit created from Phase 0 breakdown | this file + brief | `DRAFT` | feature-plan |
| 2026-07-11 | Codex as feature-plan | Planned P1 read-only SHA-pinned workflows and deterministic positive/negative gate validators; planned P2 exact seven checks, clean/GPL test-PR runs, strict main protection and read-only Actions permission readback behind explicit human authorization; dashboard focus refreshed by operator direction | `design.md`, `api.md`, `test.md`, `dev_log.md`, `dashboard-state.json` | `NEEDS_REVIEW`; remote evidence unknown | feature-review |
| 2026-07-11 | Codex as independent feature-review | Reviewed stable check names, matrix/platform setup, permissions/caches, DCO ranges, SPDX/Go licenses, link gates, CODEOWNERS, supply-chain pins, remote read/write/readback and rollback; no blocking finding; eight builder notes recorded | `docs/reviews/phase0-ci-governance/2026-07-11-feature-review.md`, this file | `APPROVED` | feature-build P1 |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the independently persisted `APPROVED` verdict | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | feature-build P1 |
| 2026-07-11 | Codex as feature-build | Built only approved P1: read-only SHA-pinned Linux/macOS/Windows workflows with seven stable check names; registry-derived CODEOWNERS; DCO, local-link, pnpm/Cargo SPDX and Go license gates; deterministic positive/negative fixtures; retained missing-local-Go scaffold failure and made no remote change | `.github/`, `scripts/ci/`, `package.json`, this file | `READY_FOR_VERIFY`; runner evidence remains unknown | feature-verify P1 |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the P1 build handoff; remote Actions and protection state remain unknown | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | feature-verify P1 |
