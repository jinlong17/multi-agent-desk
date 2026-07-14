# Feature verification: phase0-ci-governance P2

## Verdict

`BLOCKED`

Every scoped P2 technical acceptance criterion passes: the clean remote head
has seven successful checks across Ubuntu, macOS, and Windows; the seeded
GPL-3.0-only fixture fails the license gate for the intended reason; Actions
permissions are read-only; no release/deployment workflow exists; and the
operator-authorized strict `main` protection is applied and independently read
back without weakening.

The final feature cannot truthfully enter `READY_TO_SHIP`, however, because the
current protected-branch review contract has no satisfiable approval path for
PR #1. The base branch assigns every path only to `@jinlong17`, and
`@jinlong17` is also the PR author. GitHub forbids pull-request authors from
approving their own pull requests. The live PR is consequently `MERGEABLE` at
the Git level but `BLOCKED` / `REVIEW_REQUIRED` at the governance level.

## Module classification

- Owner: `project-system`
- Confidence: high
- Why: CI, CODEOWNERS, GitHub governance, workflow state, and dashboard
  contracts are project-system-owned surfaces.
- Impacts: none
- Branch: `codex/project-system/phase0-ci-governance`
- Workflow: feature
- Gates: no Provider or Security Gate; remote settings and merge are explicit
  human gates.

## P2 acceptance audit

| Requirement | Independent evidence | Result |
|---|---|---|
| Authorized unmerged test PR | PR #1 remains open; remote head `22e2240`; base `main` at `928a290`; no merge performed | pass |
| Seven exact clean checks | check-runs for `22e2240`: `project-verify`, Ubuntu, macOS, Windows, `license-gate`, `dco`, and `link-check` all completed `success`; runs `29315826964` and `29315826965` | pass |
| GPL rejection and recovery | run `29315247924` failed `license-gate` with `pnpm group GPL-3.0-only: disallowed license expression GPL-3.0-only`, exit 1; clean recovery is seven-check green | pass |
| Pre/post settings evidence | receipt retains pre-mutation 404/403 history; authenticated post-mutation GET matches the authorized protection subset | pass |
| Strict branch protection | strict seven contexts, one approval, CODEOWNER review, stale dismissal, conversation resolution, linear history, admin enforcement, force-push/delete false | pass |
| Actions permissions | Actions enabled, SHA pinning required, default token `read`, Actions PR approval false | pass |
| No release/deployment workflow | only active workflows are `CI` and `Governance`; static workflow validator finds no write/release/deploy surface | pass |
| Project/regression verification | workflow/dashboard, CI gates, DCO, licenses, Go, TypeScript, Web, Cargo, and Tauri release no-bundle build all pass | pass |
| Final ship feasibility | sole base-branch CODEOWNER is also PR author; no qualifying approval exists | **blocked** |

## Independent local evidence

- `pnpm install --offline --frozen-lockfile` — pass for all six workspaces with
  pnpm 10.23.0.
- Workflow generation/verification — pass: agents=10, skills=3, docs=17,
  edges=20, statuses=15.
- Dashboard generation/verification before verdict — pass: clean branch facts,
  phases=9, agents=10, skills=3. The verdict writer did not change manual
  dashboard judgment.
- Post-verdict `dashboard:verify` — expected nonzero: it first detects that the
  generated dirty count still reflects the pre-verdict clean worktree. After
  this verdict is committed, an operator-directed writer must refresh generated
  facts and align manual focus from `READY_FOR_VERIFY` to `BLOCKED`; the
  verifier does not perform that separate judgment write.
- Direct CI leaves — pass: exact checks=7, pinned actions=15, generated
  CODEOWNERS parity, positive/negative fixtures, 135 Markdown files, five pnpm
  license groups, and 418 Cargo packages.
- `verify-dco.mjs --base 928a290... --head HEAD` — pass: 26 commits with three
  exact grandfathered pre-policy exceptions.
- Scaffold/regression leaves with pinned Go 1.26.5 — pass: layout (27
  directories, 49 files, seven modules), Go formatting (15 files), all Go
  tests/builds, four TypeScript workspace checks/builds, Web Vite production
  build, Cargo locked check/format, and Tauri release `--no-bundle` build.
- `git diff --check`, conflict-marker scan, branch/worktree inspection — pass;
  worktree was clean before verdict.

The shell still has no standalone `npm`, so verification executed the exact
leaf commands behind the aggregate scripts. Unlike the earlier aggregate
wrapper interruption, the complete scaffold and governance command set ran to
completion in this verifier session.

## Independent remote evidence

- `gh auth status` plus `gh api user` — real API access as `jinlong17` with
  repository ADMIN permission; repository visibility is PUBLIC.
- Negative check-runs at `0bce526` — seven materialized; `license-gate` failed,
  Windows was cancelled by the superseding recovery push, and no cancelled job
  is represented as pass.
- Clean check-runs at `22e2240` — seven materialized and seven succeeded.
- Actions settings — `sha_pinning_required=true`, default workflow permission
  `read`, PR-review approval false.
- Branch protection GET — exact seven contexts with `strict=true`,
  `enforce_admins=true`, one approval, CODEOWNER review, stale dismissal,
  conversation resolution, linear history, and force-push/delete false.
- PR #1 — `MERGEABLE`, `BLOCKED`, `REVIEW_REQUIRED`, no reviews, seven green
  checks.

## Blocking finding

### F1 — required CODEOWNER approval cannot be supplied for PR #1

Severity: blocking.

The base branch `.github/CODEOWNERS` assigns `*` and every owned module path
only to `@jinlong17`. GitHub evaluates CODEOWNERS from the pull request's base
branch, and GitHub's review rules state that pull-request authors cannot approve
their own pull requests. The current author is `jinlong17`, so an ordinary
approval from another non-owner would not satisfy the enabled CODEOWNER rule.

Primary references:

- [GitHub: About code owners](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners)
- [GitHub: About pull request reviews](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/about-pull-request-reviews)

Clearing role and preferred resolution: the operator supplies a distinct
GitHub identity with repository write permission to author a replacement PR
containing the same verified commits, after which `@jinlong17` can provide the
required CODEOWNER approval. This preserves the steady-state protection
contract.

If no second identity is available, `feature-plan` must explicitly design and
review a bootstrap exception or revised ownership model. The verifier does not
accept or implement a weaker rule, admin bypass, direct push, or unreviewed
merge.

## Non-blocking retained facts

- Five final receipt/status commits are local and not yet present on remote PR
  head `22e2240`; any replacement PR or later authorized push must include them
  and rerun the seven checks.
- The pre-mutation value of the full-length Action SHA setting was not
  persisted. Current required state is proven, but exact rollback parity for
  that prior value remains unknown.
- Windows evidence is GitHub-hosted runner evidence, not local hardware or
  interactive Windows acceptance.

## Scope

Verification made no plan, implementation, dashboard, remote-setting, review,
push, merge, release, or risk-acceptance change. The only writes are this report
and the verifier-owned verdict entries in `dev_log.md`.
