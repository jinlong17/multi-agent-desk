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
| Status | `READY_FOR_VERIFY` |
| Executor | `Codex as feature-build` |
| Updated | `2026-07-14` |
| Suggested Next | `independent feature-verify audits P2 remote runs, Actions permissions, exact main protection readback, and CODEOWNER approval feasibility` |
| Branch / Worktree | `codex/project-system/phase0-ci-governance @ agent-deck-worktrees/phase0-ci-governance` |
| Plan Version | `v0.2` |
| Provider Gate | `none` |
| Security Gate | `none (CI holds no provider credentials — verify in review)` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 local CI contracts + gates | read-only 3-platform workflows, exact required job names, CODEOWNERS generator, DCO/local-link/license validators and negative fixtures | shipped scaffold | static contracts and positive/negative local evidence | `VERIFIED` |
| P2 remote Actions + governance | authorized push/test PR, clean and GPL-fail Actions runs, strict main protection, Actions/release-permission audit and readback | P1 verified; explicit operator authorization | seven checks proven; remote rules/permissions match and receipt persisted | `READY_FOR_VERIFY` |

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
| 2026-07-14 01:32 -0700 | P2 post-reinstall browser diagnostics | Chrome 150 running; extension 1.2.27203.26575 installed/enabled in selected Default profile; native host and allowed origin correct; controlled-tab listing retried once after two seconds | local install health passed, but both post-reinstall control calls timed out; no remote evidence inferred | `remote-receipt.md` |
| 2026-07-14 02:00 -0700 | P2 authenticated API readback | real `gh api user` and ADMIN repository read; check-runs/runs for GPL head `0bce526` and clean head `22e2240`; Actions permissions/workflows; protection and rulesets endpoints | GPL `license-gate` failure proven in run `29315247924`; clean seven checks proven in runs `29315826964`/`29315826965`; Actions read-only/no-approval proven; both protection surfaces return plan-required HTTP 403 | `remote-receipt.md` |
| 2026-07-14 02:03 -0700 | P2 local receipt verification | workflow generate/verify; dashboard generate/verify; `ci:verify` and `ci:static`; direct leaf action/CODEOWNERS/fixture/link/license checks; DCO range; `git diff --check` | workflow/dashboard pass; aggregate npm wrappers could not start because local `npm` is absent; all equivalent leaf checks pass (`checks=7`, `actions=15`, Markdown=135, pnpm groups=5, Cargo packages=418); DCO pass for 24 commits with three exact grandfathered exceptions | command output retained in task |
| 2026-07-14 02:32 -0700 | P2 authorized protection mutation/readback | operator made repository public and separately confirmed exact `main` protection; authenticated PUT followed by independent GET; post-rule PR/check/review readback | exact seven strict checks, one approval, CODEOWNER review, stale dismissal, conversations, linear history, admin enforcement, no force-push/delete all match; PR remains seven-check green and `MERGEABLE` but is `BLOCKED` / `REVIEW_REQUIRED` as the rule requires | `remote-receipt.md` |
| 2026-07-14 02:34 -0700 | P2 transition verification | workflow/dashboard generation and verification; leaf CI contracts/fixtures/links/licenses; DCO; diff check; protection and PR re-read | first workflow verification correctly rejected a Suggested Next phrase containing `ship` from `READY_FOR_VERIFY`; phrase corrected to feature-verify-only guidance, then all local checks passed and remote readback remained exact | command output retained in task |

## Risks and Blockers

- Merge remains a separate explicit human gate after independent verification.
- The local machine currently lacks Go/gofmt. The combined scaffold rerun failed
  at that prerequisite; no downstream scaffold result is inferred.
- GPL-negative and clean-recovery GitHub Actions evidence is now proven through
  authenticated API readback; the superseded GPL CI Windows job was cancelled
  and is retained as cancelled, not pass.
- Authenticated `gh` access clears the Chrome/browser evidence blocker.
- The repository is now public by operator choice; exact `main` protection is
  applied and independently read back.
- CODEOWNERS currently names only PR author `@jinlong17`. GitHub correctly
  reports `REVIEW_REQUIRED`; a later ship run needs an eligible CODEOWNER
  approval path and must not weaken or bypass protection.
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
| 2026-07-14 01:32 -0700 | Codex as feature-build P2 and operator-directed dashboard writer | Verified the post-reinstall Chrome, extension, selected profile, native-host manifest, and allowed origin; retried the documented lightweight connection once after two seconds; retained the timeout as evidence and refreshed only the blocked next action | `remote-receipt.md`, this file, `docs/workflow/project/dashboard-state.json`, generated dashboard state | remains `BLOCKED`; installation health is proven but authenticated tab control still fails | operator fully restarts Chrome and the ChatGPT/Codex desktop host, or supplies authenticated GitHub API/CLI access |
| 2026-07-14 02:00 -0700 | Codex as feature-build P2 and operator-directed dashboard writer | Verified `gh` authentication end to end, cleared the browser-readback condition, persisted GPL/clean run IDs and exact conclusions, read back Actions permissions/workflow inventory, and queried both protection surfaces without weakening criteria | `remote-receipt.md`, this file, `docs/workflow/project/dashboard-state.json`, generated dashboard state | remains `BLOCKED`; GitHub returns plan-required HTTP 403 for protection/rulesets on this private repository | operator chooses public visibility or a GitHub plan supporting private-repository protection; then resume P2 before merge |
| 2026-07-14 02:32 -0700 | Codex as feature-build P2 and operator-directed dashboard writer | After the operator made the repository public and confirmed the exact mutation, applied strict `main` protection, read it back independently, checked live PR/check/review state, persisted the remote receipt, and refreshed dashboard focus without merging or pushing | `remote-receipt.md`, this file, `docs/workflow/project/dashboard-state.json`, generated dashboard state | `READY_FOR_VERIFY`; all P2 build criteria proven, with CODEOWNER approval retained as a later ship gate | independent feature-verify P2 |
