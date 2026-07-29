# Bug log: Windows Codex usage-fixture deadline flake

## Status Panel

| Field | Value |
|---|---|
| Workflow | `BUGFIX` |
| Target | `windows-codex-usage-fixture-deadline-flake` |
| Title | `Windows Codex usage-fixture deadline flake` |
| Owner Module | `provider` |
| Impacted Modules | `project-system` |
| Current Phase | `BUG_FIX` |
| Status | `READY_FOR_VERIFY` |
| Executor | `Codex (GPT-5) as bug-fix` |
| Updated | `2026-07-29 02:17 PDT` |
| Suggested Next | `bug-verify` |
| Branch / Worktree | `codex/provider/windows-codex-usage-fixture-deadline-flake` / `/Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/windows-codex-usage-fixture-deadline-flake` |
| Provider Gate | `resolved — this is a test-fixture timing repair only; it changes no compatibility or support claim` |
| Security Gate | `none` |

## Reproduction

| Field | Value |
|---|---|
| Environment / versions | `origin/main@aed5320dc048bbcd18275e5ce4c4f9666ec105a1`; reported GitHub Windows Codex usage-fixture failure. This diagnosis intentionally did not rerun CI or a local test command. |
| Minimal reproduction | On a Windows runner, execute `go test -count=1 -run '^TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated$' ./internal/providers/codex`; the test starts two fixture runtimes then calls `RuntimeManager.ReadUsage` for each. Its fixture imposes a separate 100 ms RPC wait. |
| Expected behavior | A healthy in-memory fixture response may be scheduled normally and the test should assert account/credential/usage isolation, not fail because of an incidental sub-second test-harness deadline. |
| Actual behavior | Under Windows scheduling delay, a usage RPC can exceed the fixture-only 100 ms wait and return `deadline_exceeded: codex protocol call timed out`, intermittently failing the isolation assertion path without demonstrating a product isolation defect. |

## Root cause (bug-diagnose)

The flake is a **test-fixture RPC-deadline defect** in the Provider-owned Codex
runtime tests, not a production Codex usage or account-isolation failure.

- `newFixtureRuntime` assigns `client.MaxWait = 100 * time.Millisecond`
  (`internal/providers/codex/runtime_test.go:78-85`). Git history shows that
  value originated with the Phase 2 Codex vertical-slice implementation
  (`9f82fb6`, shipped as `250bf57`); it is not the five-second
  context-placement bug already fixed and shipped under
  `windows-codex-runtime-ci-flake`.
- `TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated` starts two
  fixture runtimes and immediately makes three successful `ReadUsage` calls
  (`runtime_test.go:517-589`); the truthful-usage table also makes one per
  case (`:683-730`). The fixture server answers `account/usage/read` on its
  goroutine only after decoding the request (`:127-149`).
- `Client.Call` creates a timer from `Client.waitDuration()` and returns
  `CodeDeadlineExceeded` with `codex protocol call timed out` when that timer
  wins (`internal/providers/codex/protocol.go:237-265`). `waitDuration()` uses
  the fixture override when present and otherwise uses the production default
  of five seconds (`:412-416`).
- Thus ordinary Windows runner scheduling can consume the fixture's 100 ms
  response budget even though the enclosing test operation budget remains
  five seconds. The failure is confined to `runtime_test.go`; no production
  `RuntimeManager`, protocol default, provider capability, or support boundary
  is implicated by this evidence.

## Fix scope (smallest repair)

- Change only `internal/providers/codex/runtime_test.go`.
- Give the general runtime fixture a scheduler-tolerant response deadline
  (the normal client five-second default is suitable), then set the short
  deadline explicitly only in the blocked-write test that verifies a bounded
  failure. That test currently requires the operation to finish within one
  second (`runtime_test.go:1074-1117`), so its narrow timeout assertion remains
  intentional rather than becoming global fixture behavior.
- Add or adapt a deterministic regression that proves normal usage fixtures do
  not inherit the short blocked-write deadline, while retaining a bounded
  blocked-write assertion. Do not alter production protocol timeouts, usage
  persistence, compatibility rows, or Windows support claims.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-29 01:44 PDT | INTAKE | Initialized the new bug log from `docs/workflow/templates/bug_log.md` on isolated branch/worktree `codex/provider/windows-codex-usage-fixture-deadline-flake` from `origin/main@aed5320` | `DRAFT` intake recorded; prior `windows-codex-runtime-ci-flake` was read-only and remains `SHIPPED` | this file |
| 2026-07-29 01:44 PDT | STATIC REPRODUCTION TRACE | Inspected `internal/providers/codex/runtime_test.go:78-149,517-589,683-730,1074-1117`, `protocol.go:237-265,412-416`, and `runtime.go:954-1004`; checked `git log -S '100 * time.Millisecond'` | The fixture forces 100 ms for all calls; usage exercises consume it; the client converts expiry to the reported deadline error; production default is five seconds | source files; commits `9f82fb6`, `250bf57` |
| 2026-07-29 01:44 PDT | BOUNDARY CHECK | Read existing `docs/workflow/features/windows-codex-runtime-ci-flake/dev_log.md` and diff of its shipped repair | The earlier bug fixed five-second context placement after fixture setup; this independent global fixture override remained and requires a new bug unit | prior shipped bug log; commit `a4c3f219` |
| 2026-07-29 01:44 PDT | EXECUTION LIMIT | No CI, local test, push, PR, or implementation command run, per operator direction | Diagnosis rests on the reported Windows symptom plus direct source and history trace; native Windows regression remains required after the fix | operator instruction; this log |
| 2026-07-29 01:51 PDT | FIX AND REGRESSION | Changed only `runtime_test.go`: common fixtures now use an explicit five-second wait; the concurrent usage-isolation regression asserts both spawned fixtures retain that value, and the blocked-approval-write test explicitly sets its 100 ms wait before exercising its existing one-second upper bound | The new usage assertion would fail against the diagnosed 100 ms common fixture; the short timeout remains local to the failure contract | `internal/providers/codex/runtime_test.go` |
| 2026-07-29 01:51 PDT | TARGETED REPEATS | `go test -count=100 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex`; `go test -race -count=20 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex` | PASS: 200 normal and 40 race executions; both the scheduler-tolerant usage path and explicit bounded blocked-write path passed | local command output |
| 2026-07-29 01:51 PDT | PACKAGE AND WINDOWS COMPILE | `go test -count=1 ./internal/providers/codex`; `go vet ./internal/providers/codex`; `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/windows-codex-usage-fixture-deadline-flake.test.exe ./internal/providers/codex` | PASS; full Codex provider package, vet, and Windows amd64 test compilation passed. Compile evidence is not native Windows execution evidence. | local command output; `/tmp/windows-codex-usage-fixture-deadline-flake.test.exe` |
| 2026-07-29 01:51 PDT | STATIC CI CONTRACTS | With bundled Codex Node/Pnpm, `pnpm run ci:actions`; `pnpm run ci:codeowners` | PASS: Actions checks=7/actions=15; CODEOWNERS owner=`@jinlong17`. Aggregate `pnpm run ci:static` remains unavailable because its script invokes absent `npm`; direct underlying checks passed. | local command output |
| 2026-07-29 01:55 PDT | BUG VERIFY — SCOPE AND NON-RACE CHECKS | Re-read clean `93e6a6c` and its parent diff; ran `go test -count=100 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex`, `go test -count=1 ./internal/providers/codex`, and `go vet ./internal/providers/codex` | PASS: focused repetitions, provider package, and vet. The diff changes only this log and `runtime_test.go`; production `NewClient`/fallback remain five seconds, while the blocked-write test retains an explicit 100 ms value. | local command output; `git diff 93e6a6c^ 93e6a6c`; `internal/providers/codex/protocol.go:91,412-416` |
| 2026-07-29 01:55 PDT | BUG VERIFY — RACE AND WINDOWS LIMIT | Ran `go test -v -race -count=20 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex`; cross-compiled `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/windows-codex-usage-fixture-deadline-flake-verify.test.exe ./internal/providers/codex`; inspected `.github/workflows/ci.yml` and read-only remote SHA status | BLOCKED: race detector reports `runtime_test.go:1119` writing `fixture.rpcClient.MaxWait` concurrently with `RuntimeManager.eventPump` reading it through `protocol.go:413`. Cross-compile emits a Windows PE executable only. CI would execute `go test -count=1 ./...` on `windows-latest`, but this SHA/branch is not on origin (GitHub check-runs returns 422), so no native Windows execution exists. | local race output; `/tmp/windows-codex-usage-fixture-deadline-flake-verify.test.exe`; `.github/workflows/ci.yml`; `gh api` 422 |
| 2026-07-29 02:01 PDT | BLOCKER REPAIR | Parameterized fixture construction with an immutable `maxWait` argument. Common runtime-manager fixtures pass five seconds through `prepareRuntimeManagerFixture` before `Spawn`; the blocked-write test passes 100 ms through `runtimeManagerFixtureWithMaxWait` before `RuntimeManager.Start` can create an event pump. Removed the post-start `fixture.rpcClient.MaxWait` mutation and retained assertions for the ordinary five-second usage fixtures and one-second blocked-write upper bound. | `internal/providers/codex/runtime_test.go` |
| 2026-07-29 02:01 PDT | BLOCKER RECHECK | `go test -count=200 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex`; `go test -race -count=50 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex`; `go test -count=1 ./internal/providers/codex`; `go vet ./internal/providers/codex`; `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/windows-codex-usage-fixture-deadline-flake-race-fix.test.exe ./internal/providers/codex`; `git diff --check` | PASS: 400 focused normal executions, 100 focused race executions with no race report, full Codex provider package, vet, Windows amd64 test compilation, and diff integrity. The Windows artifact remains compile-only evidence. | local command output; `/tmp/windows-codex-usage-fixture-deadline-flake-race-fix.test.exe` |
| 2026-07-29 02:04 PDT | BUG VERIFY — SECOND REPAIR | Read clean `c6dcf2e` and `c6dcf2e^`; inspected fixture construction, `RuntimeManager.Start`, event-pump scheduling, production client defaults, and Windows CI configuration; ran `go test -count=150 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex`; `go test -race -count=100 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex`; `go test -count=1 ./internal/providers/codex`; `go vet ./internal/providers/codex`; `git diff --check c6dcf2e^`; `git ls-remote origin refs/heads/codex/provider/windows-codex-usage-fixture-deadline-flake` | PASS: `c6dcf2e` changes only this log and `runtime_test.go`; all normal fixtures construct a five-second client and the blocked-write fixture constructs 100 ms before `Start` can schedule `eventPump`; 300 normal and 200 race executions, package tests, vet, and diff check pass. BLOCKED: `ls-remote` has no published branch/SHA, so no native `windows-latest` job exists. Cross-compilation is not treated as execution. | local command output; `.github/workflows/ci.yml:50,71`; `docs/reviews/windows-codex-usage-fixture-deadline-flake/2026-07-29-bug-verify.md` |
| 2026-07-29 02:17 PDT | REMOTE WINDOWS RECEIPT | Read PR #35 through GitHub CLI at exact head `38f3e5bbd9942e479ba1827f8052cc021c860e64`; its seven required checks are all `SUCCESS`: project-verify, license-gate, build-ubuntu, dco, build-macos, build-windows, and link-check. | Native `build-windows` completed successfully in CI run `30438687149`, job `90532479920`; this clears the previously missing native-Windows execution receipt for the race-free test-fixture repair. | [PR #35](https://github.com/jinlong17/multi-agent-desk/pull/35); [build-windows job](https://github.com/jinlong17/multi-agent-desk/actions/runs/30438687149/job/90532479920) |

## Risks and Blockers

- The `MaxWait` race blocker is repaired and PR #35's exact head has a
  successful native Windows CI receipt. Independent `bug-verify` remains the
  required next workflow gate.
- Do not close, amend, or reinterpret the already `SHIPPED`
  `windows-codex-runtime-ci-flake` unit; it addressed a different timeout.
- The repair must preserve the blocked-write test's explicit bounded failure
  assertion, so increasing the general fixture deadline cannot mask that
  contract.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-29 01:44 PDT | Codex (GPT-5) as bug-diagnose | Classified Provider ownership (project-system CI impact), initialized this separate DRAFT bug unit, traced the reported Windows usage failure through the all-purpose fixture's 100 ms `MaxWait`, protocol deadline path, usage call sites, and Phase 2 history; preserved the old shipped bug unchanged | this file; `docs/reviews/windows-codex-usage-fixture-deadline-flake/2026-07-29-bug-diagnose.md` | `DRAFT -> DIAGNOSED`; minimum repair is test-only: general fixture deadline plus a local short override for blocked-write coverage | `bug-fix` |
| 2026-07-29 01:51 PDT | Codex (GPT-5) as bug-fix | Replaced the common fixture's 100 ms `MaxWait` with a five-second scheduler-tolerant value, added a deterministic assertion that the two concurrent usage fixtures use it, and moved the 100 ms setting into the blocked-approval-write contract before its existing bounded-failure assertion | `internal/providers/codex/runtime_test.go`; this file | `DIAGNOSED -> READY_FOR_VERIFY`; focused normal/race repeats, full provider package, vet, Windows amd64 compile, and direct CI contracts pass; no production timeout, workflow, link checker, PR, commit, push, or support claim changed | `bug-verify`, including fresh native Windows execution evidence |
| 2026-07-29 01:55 PDT | Codex (GPT-5) as bug-verify | Independently checked the exact clean SHA and test-only diff, repeated focused and package checks, inspected production/fixture timeout boundaries and native-Windows CI availability, and ran race coverage | `docs/reviews/windows-codex-usage-fixture-deadline-flake/2026-07-29-bug-verify.md`; this file | `READY_FOR_VERIFY -> BLOCKED`: the post-start test mutation of `Client.MaxWait` races the active event pump; exact SHA also has no native Windows run because it is not published remotely | `bug-fix` |
| 2026-07-29 02:01 PDT | Codex (GPT-5) as bug-fix | Cleared the verifier's `MaxWait` data-race blocker by parameterizing fixture creation: default callers construct five-second clients, and the blocked-write test constructs its 100 ms client before any RuntimeManager/event pump begins. Removed live client mutation; retained the ordinary-fixture and bounded-write regression assertions. | `internal/providers/codex/runtime_test.go`; this file | `BLOCKED -> READY_FOR_VERIFY`; 400 focused normal and 100 focused race executions pass, as do the provider package, vet, Windows amd64 test compile, and diff check; no production timeout, Provider behavior, support claim, commit, push, or CI mutation | `bug-verify`, including native Windows execution for the repaired SHA |
| 2026-07-29 02:04 PDT | Codex (GPT-5) as bug-verify | Independently verified the clean exact `c6dcf2e` repair. Its only code change is fixture construction: five seconds is supplied before common fixture spawn, and 100 ms is supplied before the blocked-write test calls `Start`; `Start` schedules `eventPump` only after the fixture client is constructed. Repeated high-count normal/race coverage and provider checks; no live `MaxWait` mutation or production timeout change remains. | `docs/reviews/windows-codex-usage-fixture-deadline-flake/2026-07-29-bug-verify.md`; this file | `READY_FOR_VERIFY -> BLOCKED`: repair is race-clean locally, but exact SHA is absent from `origin`, leaving no native Windows CI execution. Cross-compile is compile evidence only and was not used as acceptance. | `bug-fix` after human-authorized publication and a passing native Windows CI result |
| 2026-07-29 02:17 PDT | Codex (GPT-5) as bug-fix | Re-read remote PR #35 exact head `38f3e5bbd9942e479ba1827f8052cc021c860e64` and its check rollup. All seven required checks succeeded, including native `build-windows` CI run `30438687149`, job `90532479920`; confirmed intervening commits are documentation-only and made no implementation change. | this file; PR #35 | `BLOCKED -> READY_FOR_VERIFY`; the missing native-Windows evidence is now present for the repaired test-fixture scope. No implementation, commit, push, or support-boundary mutation. | `bug-verify` |
