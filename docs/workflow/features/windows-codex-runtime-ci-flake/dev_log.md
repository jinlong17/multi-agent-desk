# Bug log: Windows Codex runtime CI deadline flake

## Status Panel

| Field | Value |
|---|---|
| Workflow | `BUGFIX` |
| Target | `windows-codex-runtime-ci-flake` |
| Title | `Windows Codex runtime CI deadline flake` |
| Owner Module | `provider` |
| Impacted Modules | `project-system` |
| Current Phase | `VERIFY` |
| Status | `READY_TO_SHIP` |
| Executor | `Codex (GPT-5) as bug-verify` |
| Updated | `2026-07-20 19:20 PDT` |
| Suggested Next | `ship` |
| Branch / Worktree | `codex/provider/windows-codex-runtime-ci-flake @ /Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/windows-codex-runtime-ci-flake` |
| Provider Gate | `resolved — diagnosis does not change the exact Linux amd64 Codex 0.144.2 support claim or any platform capability` |
| Security Gate | `none` |

## Reproduction

| Field | Value |
|---|---|
| Environment / versions | `main@154f882e8baea8165b729dfa53d7bdf4d3e546f1`; GitHub-hosted `windows-latest`, Microsoft Windows Server 2025 `10.0.26100`, image `windows-2025-vs2026@20260714.173.1`, runner `2.335.1`, Go `1.26.5 windows/amd64`; CI run `29790417988` |
| Minimal reproduction | On that Windows runner, run the repository CI command `go test -count=1 ./...`. Attempt 1 job `88510801096` fails `TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent`; rerunning the same job as attempt 2 `88511433690` fails `TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated`; attempt 3 `88511790401` passes the Go suite twice on the unchanged SHA. |
| Expected behavior | Codex runtime fixture tests remain deterministic under ordinary hosted-runner scheduling and verify shared-runtime binding isolation plus concurrent account/Usage isolation. |
| Actual behavior | Each affected test creates one five-second context before its SQLite/filesystem-heavy `runtimeManagerFixture`; slow Windows setup consumes that budget, and a later Store call observes the expired context. Safe storage wrappers surface the cause only as `conflict: session could not be read` or `conflict: database transaction could not start`. |

## Root cause (bug-diagnose)

The flake is a **test-harness deadline-placement defect**, not evidence of a
Codex runtime isolation failure or a SQLite locking defect.

- Both affected tests create `context.WithTimeout(..., 5*time.Second)` before
  `runtimeManagerFixture(t)` (`internal/providers/codex/runtime_test.go:260-263`
  and `:451-456`).
- The fixture deliberately uses `context.Background()` while it opens and
  migrates a new SQLite database, creates the initial Device/Account/
  Credential/Profile/Workspace, and prepares temporary filesystem state
  (`runtime_test.go:207-256`; `materialization_test.go:63-83`). Those setup
  operations therefore continue even after the separate five-second test
  context expires.
- Attempt 1 reports the first test at `5.42s`, then its final
  `store.Session(ctx, ...)` at line 293 receives the already-expired context.
  `Store.Session` wraps any query error at `internal/storage/repository.go:741-755`
  as `conflict: session could not be read`.
- Attempt 2 reports the second test at `9.14s` and fails at its first operation
  after the fixture, `store.CreateAccount(ctx, ...)` at lines 456-458.
  `CreateAccount` enters `withTx`; `BeginTx` wraps the expired context at
  `internal/storage/store.go:412-415` as `conflict: database transaction could
  not start`.
- The application error intentionally omits internal causes from ordinary
  formatting while retaining them through `Unwrap`
  (`internal/domain/errors.go:72-97`), explaining why CI text looks like two
  unrelated database failures.
- Attempt 3 passed `internal/providers/codex` twice (`18.752s` in the direct Go
  step and `20.115s` inside `scaffold:verify`) on the exact same main SHA. That
  scheduling sensitivity, the two different first-post-expiry failure sites,
  and the exact five-second boundary rule out a deterministic data-isolation
  regression in the observed evidence.

## Fix scope (smallest repair)

- Change only `internal/providers/codex/runtime_test.go`.
- Start the per-test operation context **after** `runtimeManagerFixture(t)` has
  completed, so SQLite migration/filesystem setup is not charged to the runtime
  assertion budget. Apply the same ordering to runtime-manager tests using this
  fixture, not only the two names that happened to fail, because the same
  five-second-before-fixture pattern occurs repeatedly in the file.
- Retain bounded operation/polling deadlines; do not remove timeouts or change
  production Store busy-timeout, transaction, runtime, materialization, or
  account-isolation behavior.
- Regression shape: repeat the two originally failing tests on Windows under
  the normal full-package command, add a stress run of both names, and require
  at least one fresh full Windows CI job. Assert failures, if deliberately
  induced with an expired context, unwrap to `context.DeadlineExceeded` so a
  future diagnosis cannot mistake timeout exhaustion for database corruption.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-20 17:32 PDT | REMOTE REPRO attempt 1 | `go test -count=1 ./...`; CI run `29790417988`, job `88510801096`, SHA `154f882e8baea8165b729dfa53d7bdf4d3e546f1` | FAIL only on Windows: `TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent (5.42s)`, line 294, `conflict: session could not be read`; Codex package `64.452s`; storage package passed | [attempt 1 job](https://github.com/jinlong17/multi-agent-desk/actions/runs/29790417988/job/88510801096) |
| 2026-07-20 17:35 PDT | REMOTE REPRO attempt 2 | rerun of the same workflow/job/SHA, job `88511433690` | FAIL only on Windows: `TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated (9.14s)`, line 458, `conflict: database transaction could not start`; storage package passed | [attempt 2 job](https://github.com/jinlong17/multi-agent-desk/actions/runs/29790417988/job/88511433690) |
| 2026-07-20 17:39 PDT | REMOTE CONTROL attempt 3 | second rerun on unchanged SHA, job `88511790401` | PASS: direct Go suite passed Codex in `18.752s`; `scaffold:verify` repeated it and passed Codex in `20.115s`; complete Windows job succeeded | [attempt 3 job](https://github.com/jinlong17/multi-agent-desk/actions/runs/29790417988/job/88511790401) |
| 2026-07-20 17:41 PDT | LOCAL STRESS | `go test -count=100 -run '^(TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent\|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` on Go `1.26.5 darwin/arm64` | PASS, `200` targeted test executions; package `17.903s` | local command output |
| 2026-07-20 17:42 PDT | LOCAL RACE | `go test -race -count=10 -run '^(TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent\|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` | PASS, `20` targeted race executions; package `15.066s` | local command output |
| 2026-07-20 17:42 PDT | STATIC TRACE | inspected affected tests/fixture, `RuntimeManager`, `Store.Session`, `CreateAccount`, `withTx`, error wrapping, Git blame, and CI workflow | Root cause localized to test context lifetime; production runtime/storage changes are outside the minimum repair | `internal/providers/codex/runtime_test.go`; `internal/providers/codex/materialization_test.go`; `internal/storage/{store,repository}.go`; `internal/domain/errors.go`; `.github/workflows/ci.yml` |
| 2026-07-20 17:44 PDT | GOVERNANCE | `pnpm run project:verify`; `pnpm run ci:static` with bundled Node plus system pnpm | Initial aggregate wrappers could not find `npm` in this non-installed worktree; no verifier executed through those wrappers | local command output; retained diagnostic failure |
| 2026-07-20 17:45 PDT | GOVERNANCE RERUN | `pnpm run workflow:generate`; `workflow:verify`; `dashboard`; `dashboard:verify`; `ci:actions`; `ci:codeowners`; `ci:fixtures`; `ci:links`; `ci:licenses`; `git diff --check` | PASS via every exact underlying script; workflow `agents=10 skills=3 docs=17 edges=20 statuses=15`; dashboard valid with two dirty diagnostic paths; `282` Markdown files link-clean | local command output |
| 2026-07-20 18:31 PDT | FIX | Moved all 15 runtime-manager tests onto a fixture API that creates the unchanged five-second operation context only after SQLite/filesystem setup returns; no production RuntimeManager, Store, migration, SQLite, Provider, or Named Pipe code changed | PASS; every same-shaped test now receives a fresh bounded operation context after setup | `internal/providers/codex/runtime_test.go` |
| 2026-07-20 18:32 PDT | REGRESSION | `go test -count=1 -run '^TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget$' ./internal/providers/codex` | PASS; deterministic deadline assertion proves the operation deadline is not earlier than `setupFinished + operationTimeout`, without sleep or a widened production/test operation budget | local command output |
| 2026-07-20 18:33 PDT | TARGET STRESS | `go test -count=100 -run '^(TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget|TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` | PASS; `300` targeted executions; package `49.890s` | local command output |
| 2026-07-20 18:35 PDT | TARGET RACE | `go test -race -count=10 -run '^(TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget|TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` | PASS; `30` targeted race executions; package `30.976s` | local command output |
| 2026-07-20 18:35 PDT | FULL GO | `go test -count=1 ./...`; `go vet ./...` | PASS; all Go packages passed and vet reported no findings | local command output |
| 2026-07-20 18:38 PDT | WINDOWS CROSS-COMPILE | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/providers/codex`; `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/multidesk` | PASS; produced Windows amd64 test binary and CLI binary; compile evidence only, not native Windows runtime acceptance | `/tmp/mad-runtime-flake.4GKM3L/{codex.test.exe,multidesk.exe}` |
| 2026-07-20 18:39 PDT | GOVERNANCE | exact underlying `workflow:generate`; `workflow:verify`; `dashboard`; `dashboard:verify`; scaffold structure/Go format; CI Actions/CODEOWNERS/fixtures/links/licenses; `git diff --check` | PASS after rerunning the license verifier with bundled Node on `PATH`; the first license invocation failed only because the pnpm shebang could not locate `node`, and is retained here rather than hidden | local command output |
| 2026-07-20 19:10 PDT | VERIFY SCOPE | `git diff 154f882e8baea8165b729dfa53d7bdf4d3e546f1...6c53c6869cccf8081f3c1b3a7130d3a724123a02`; `git diff --check` | PASS; implementation delta is confined to `internal/providers/codex/runtime_test.go` (`44` additions, `46` deletions), moving all 15 same-shaped tests to post-setup bounded contexts plus one deterministic regression; no production or support-boundary change | local diff; PR #24 |
| 2026-07-20 19:13 PDT | VERIFY TARGET STRESS | `go test -count=100 -run '^(TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget\|TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent\|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` | PASS; `300` targeted executions; package `20.342s` | local command output |
| 2026-07-20 19:14 PDT | VERIFY TARGET RACE | `go test -race -count=10 -run '^(TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget\|TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent\|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` | PASS; `30` targeted race executions; package `21.344s` | local command output |
| 2026-07-20 19:16 PDT | VERIFY FULL GO | `go test -count=1 ./...`; `go vet ./...` | PASS; all Go packages passed, Codex `5.087s`; vet reported no findings | local command output |
| 2026-07-20 19:18 PDT | VERIFY ADJACENT | `go test -count=100 -run '^TestNativeTwoClientFakeSessionControl$' ./internal/device` | PASS; `100` executions; package `78.909s`; preserves but does not reproduce PR #24's first unrelated Ubuntu Device E2E failure | local command output |
| 2026-07-20 19:20 PDT | VERIFY WINDOWS | PR #24 CI run `29794650755`, native Windows job `88525423470`, merge ref `3472cdb` = head `6c53c68` + base `154f882`; unfiltered `go test -count=1 ./...` and scaffold repetition | PASS; Go `1.26.5 windows/amd64`; Codex package passed twice in `32.059s` and `34.714s`, exercising both original tests and the regression; complete Windows job succeeded | [Windows job](https://github.com/jinlong17/multi-agent-desk/actions/runs/29794650755/job/88525423470) |
| 2026-07-20 19:20 PDT | VERIFY REQUIRED CHECKS | PR #24 exact head `6c53c68`; required check rollup | PASS final `7/7`: `project-verify`, Ubuntu rerun, macOS, Windows, license, DCO, links. First Ubuntu job `88523410234` failed unrelated `internal/device/TestNativeTwoClientFakeSessionControl`; rerun job `88525411694` passed Device and Codex twice; failure remains recorded | [PR #24](https://github.com/jinlong17/multi-agent-desk/pull/24); [Ubuntu first job](https://github.com/jinlong17/multi-agent-desk/actions/runs/29794650755/job/88523410234); [Ubuntu rerun](https://github.com/jinlong17/multi-agent-desk/actions/runs/29794650755/job/88525411694) |
| 2026-07-20 19:22 PDT | VERIFY GOVERNANCE | `pnpm run workflow:verify`; direct `pnpm run dashboard:verify`; `pnpm run ci:links`; `git diff --check` | Workflow, 283-file link check, and diff check PASS. Direct dashboard check reported `generated commit is stale` because the verdict writer did not run the writer-owned preceding dashboard refresh; canonical CI `project:verify` generated then verified the snapshot and passed at exact head | local command output; [project-verify job](https://github.com/jinlong17/multi-agent-desk/actions/runs/29794650755/job/88525424090) |

## Risks and Blockers

- No blocker remains for `bug-fix`; scoped local checks support
  `READY_FOR_VERIFY`.
- Darwin stress/race and Windows cross-compilation do not authorize a native
  Windows pass claim. Independent `bug-verify` plus a fresh protected Windows
  job must exercise the repaired tests before Ship.
- Scope boundary: the separate Windows Named Pipe close/read hang occurs after
  the Go suite in another package/path and has its own bug unit. It is not part
  of this root cause or fix scope.
- No production data, credentials, Provider calls, migrations, or support
  claims were changed.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-20 17:42 PDT | Codex (GPT-5) as bug-diagnose | Classified the runtime/account-isolation flake as Provider-owned with project-system CI impact; reproduced from two complete Windows job logs; correlated both failure sites with a five-second context created before slow fixture setup; checked unchanged-SHA attempt 3 and local stress/race controls; documented only diagnosis evidence | this file; `docs/reviews/windows-codex-runtime-ci-flake/2026-07-20-bug-diagnose.md` | `DIAGNOSED`; minimum repair is test-only context placement and Windows regression coverage; Named Pipe hang remains a separate bug | `bug-fix` |
| 2026-07-20 18:39 PDT | Codex (GPT-5) as bug-fix | Replaced the repeated context-before-fixture pattern in all 15 runtime-manager tests with one fixture contract that starts the same bounded operation context after setup; added a deterministic setup/deadline ordering regression; completed targeted stress/race, full Go/vet, Windows cross-compile, and governance checks | this file; `internal/providers/codex/runtime_test.go` | `READY_FOR_VERIFY`; production behavior and the independent Named Pipe bug remain untouched | `bug-verify`, including a fresh protected Windows job |
| 2026-07-20 19:20 PDT | Codex (GPT-5) as bug-verify | Independently reviewed the test-only diff for scope, repeated the new regression and both original tests under stress/race, ran full Go/vet and adjacent Device stress, inspected native Windows logs, preserved the first unrelated Ubuntu Device E2E failure and its successful rerun, and locked the verdict to exact head `6c53c68` with seven required checks green | this file; `docs/reviews/windows-codex-runtime-ci-flake/2026-07-20-bug-verify.md` | `READY_TO_SHIP`; no implementation, plan, dashboard, PR, commit, push, or merge mutation by verifier | `ship` with explicit human authorization |
