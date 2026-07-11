# Development log: Phase 0 — repository identity and root governance files

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `phase0-repository-layout` |
| Title | `Phase 0 — repository identity and root governance files` |
| Owner Module | `project-system` |
| Impacted Modules | `none` |
| Current Phase | `P2` |
| Status | `READY_TO_SHIP` |
| Executor | `Codex as independent feature-verify` |
| Updated | `2026-07-11` |
| Suggested Next | `ship` |
| Branch / Worktree | `codex/project-system/phase0-repository-layout @ /Users/jinlong/Desktop/jinlong_project/multi-agent-desk` |
| Plan Version | `v0.2` |
| Provider Gate | `none` |
| Security Gate | `none` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 governance + verifier follow-ups | LICENSE, CONTRIBUTING, SECURITY, NOTICES, README; lifecycle F1/F2 | ADR 0009 | root contracts, F1 negative tests, F2 wording, project verification | `VERIFIED` |
| P2 repository identity window | local directory rename and verified origin transition | P1; operator maintenance window; prepared GitHub repository | canonical path and real origin verified; linked worktrees repaired; project verification | `VERIFIED` |

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-11 | P1 | `npm run project:verify` | pass: agents=10, skills=3, docs=17, edges=20, statuses=15; dashboard verified | console output |
| 2026-07-11 | P1 | local Markdown link scan across README, CONTRIBUTING, SECURITY, THIRD_PARTY_NOTICES | pass: 4 files, no missing local targets; durable CI link checker not yet available = `unknown` | console output |
| 2026-07-11 | P1 F1 | remove gated READY_TO_SHIP security-review edge, run `npm run workflow:verify`, restore | expected fail with exact missing gated-edge error | console output |
| 2026-07-11 | P1 F1 | remove ACCEPTED ship edge, run `npm run workflow:verify`, restore | expected fail with exact missing gated-edge error | console output |
| 2026-07-11 | P1 F2 | inspect stale-focus error text | pass: names operator-directed writer session and target Work Log | `scripts/dashboard/verify-static.mjs` |
| 2026-07-11 | P1 Windows | Windows acceptance | not applicable to this documentation/verifier phase | plan boundary |
| 2026-07-11 | P1 independent verify | `npm run project:verify && git diff --check`; local link scan; root-policy markers; F1/F2 source and build-receipt inspection; scope/origin check | pass for P1; durable CI link checker remains `unknown`; origin already names multi-agent-desk; directory still agent-deck | `docs/reviews/phase0-repository-layout/2026-07-11-feature-verify.md` |
| 2026-07-11 | P2 | operator explicitly authorized directory maintenance window; moved checkout to `/Users/jinlong/Desktop/jinlong_project/multi-agent-desk` | pass: basename `multi-agent-desk`; old path removed | filesystem and command output |
| 2026-07-11 | P2 | `git worktree repair` for architecture/threat linked worktrees, then `git status -sb` and `git rev-parse --git-common-dir` in each | repair reported broken old `.git` pointers and fixed them; both branches clean and common dir is new main `.git` | command output |
| 2026-07-11 | P2 | `git remote get-url origin`; `git worktree list --porcelain`; `npm run project:verify` | pass: real SSH origin is `git@github.com:jinlong17/multi-agent-desk.git`; three worktrees canonical; project verification green | command output |
| 2026-07-11 | P2 independent verify | old/new path assertions; exact origin; worktree topology; clean linked-worktree status/common-dir assertions; `npm run project:verify && git diff --check` | pass; repair proven effective; only expected state files dirty before verdict | `docs/reviews/phase0-repository-layout/2026-07-11-feature-verify-p2.md` |

## Risks and Blockers

- Rename requires an isolated maintenance window (open sessions break).

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Unit created from Phase 0 breakdown | this file + brief | `DRAFT` | feature-plan |
| 2026-07-11 | Codex as feature-plan | Planned P1 governance plus F1/F2 and isolated P2 repository identity maintenance; preserved directory, remote settings, push, and merge as explicit human gates; dashboard focus refreshed by operator direction | `design.md`, `api.md`, `test.md`, `dev_log.md`, `dashboard-state.json` | `NEEDS_REVIEW` | feature-review |
| 2026-07-11 | Codex as independent feature-review | Reviewed scope, contracts, failure modes, gates, testability, rollback, and phase ordering; no blocking findings; builder notes preserve truthful security/link evidence and prohibit P2 mutations during P1 | `docs/reviews/phase0-repository-layout/2026-07-11-feature-review.md`, this file | `APPROVED` | feature-build P1 |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the independently persisted `APPROVED` verdict | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | feature-build P1 |
| 2026-07-11 | Codex as feature-build | Built only approved P1: completed root governance set, refreshed README, hard-asserted both F1 gated edges with independent negative evidence, and corrected F2 authority hint; directory, origin, GitHub settings, push, and merge untouched | root docs, `scripts/workflow/verify-workflow.mjs`, `scripts/dashboard/verify-static.mjs`, this file | `READY_FOR_VERIFY`; durable CI link check remains `unknown` until phase0-ci-governance | feature-verify P1 |
| 2026-07-11 | Codex as independent feature-verify | Independently reran P1 checks and inspected acceptance, scope, security, compatibility, and evidence fidelity; P1 passes with CI link gate still explicitly unknown; P2 remains | `docs/reviews/phase0-repository-layout/2026-07-11-feature-verify.md`, this file | `VERIFIED` | feature-build P2 after operator maintenance-window authorization |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the independently persisted P1 `VERIFIED` verdict | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | await P2 maintenance-window authorization |
| 2026-07-11 | Codex as feature-build | With explicit operator maintenance-window authorization, renamed the local checkout to `multi-agent-desk`, repaired both linked-worktree pointers, verified real origin/path/topology, and ran project verification; no GitHub setting, push, or merge performed | filesystem identity; this file; dashboard-state.json | `READY_FOR_VERIFY` | feature-verify P2 |
| 2026-07-11 | Codex as independent feature-verify | Independently verified canonical path/origin, old-path removal, all three worktree branches/commits, linked common-dir repair, clean linked status, project checks, and scope | `docs/reviews/phase0-repository-layout/2026-07-11-feature-verify-p2.md`, this file | `READY_TO_SHIP` | ship (goal pre-authorized; no push/merge) |
| 2026-07-11 | operator-directed writer | Refreshed dashboard focus and manual status to the independently persisted P2 `READY_TO_SHIP` verdict | `docs/workflow/project/dashboard-state.json`, this file | focus aligned | ship |
