# Bug verification: Windows Codex usage-fixture deadline flake

**Verdict:** `BLOCKED`

**Verified target:** clean `c6dcf2ec1f34cc65e53868aad43876956dfe5e49`

## Second verification — race repair accepted locally

This supersedes the earlier local-race finding below: `c6dcf2e` removes the
post-start `fixture.rpcClient.MaxWait` assignment that raced with
`RuntimeManager.eventPump`. Its commit changes only this bug log and
`internal/providers/codex/runtime_test.go`.

The general fixture passes `fixtureRuntimeMaxWait` (five seconds) through
`prepareRuntimeManagerFixture` into `newFixtureRuntime`, where the client is
constructed and configured before `Spawn` returns. The blocked-write test
passes `fixtureBlockedWriteMaxWait` (100 ms) through the same construction
path before it calls `RuntimeManager.Start`. `Start` schedules `eventPump`
only later, after the fixture client has already been configured. The only
remaining `client.MaxWait =` in this fixture is therefore construction-time.
Production `NewClient` and `waitDuration` remain unchanged at five seconds.

| Command or inspection | Result |
|---|---|
| `git diff-tree --no-commit-id --name-only -r c6dcf2e`; `git diff --check c6dcf2e^` | PASS — only `runtime_test.go` and the bug log changed; no whitespace errors. |
| `go test -count=150 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex` | PASS — 300 focused normal executions. |
| `go test -race -count=100 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex` | PASS — 200 focused race executions; no race report. |
| `go test -count=1 ./internal/providers/codex`; `go vet ./internal/providers/codex` | PASS. |
| `rg -n -C 3 'windows-latest|go test' .github/workflows/ci.yml`; `git ls-remote origin refs/heads/codex/provider/windows-codex-usage-fixture-deadline-flake` | CI is configured to run `go test -count=1 ./...` on `windows-latest`, but the exact branch/SHA is absent from `origin`; no native Windows result exists. |

No cross-compilation was used as acceptance evidence. It can establish Windows
buildability only; it cannot execute this race-sensitive fixture on Windows.
No push, merge, production-code edit, or production-timeout edit was made by
this verifier.

## Remaining clearing condition

Human-authorized publication of exact `c6dcf2e` (or a later documented repair
SHA) must produce a passing native `windows-latest` `go test -count=1 ./...`
result. The bug workflow then returns through `bug-fix` to `READY_FOR_VERIFY`
for a final independent native-Windows verification.

## Earlier verification record

## Scope verdict

`93e6a6c` is clean and changes only the bug log and
`internal/providers/codex/runtime_test.go`.  The product client implementation
is untouched: `NewClient` and `waitDuration()` still use the five-second
production default in `internal/providers/codex/protocol.go`.  The repair also
keeps the intended 100 ms blocked-write deadline explicitly in
`TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay`; normal
fixtures use a five-second test value.

## Independent checks

| Command or inspection | Result |
|---|---|
| `go test -count=100 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex` | PASS |
| `go test -count=1 ./internal/providers/codex` | PASS |
| `go vet ./internal/providers/codex` | PASS |
| `go test -v -race -count=20 -run '^(TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated|TestRuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay)$' ./internal/providers/codex` | FAIL — race detector reports a write at `runtime_test.go:1119` racing with `Client.waitDuration()` at `protocol.go:413`, invoked by `RuntimeManager.eventPump`. |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/windows-codex-usage-fixture-deadline-flake-verify.test.exe ./internal/providers/codex` | PASS — produces a Windows amd64 PE test executable; it is compile evidence only. |

The first race iteration reports the new
`fixture.rpcClient.MaxWait = fixtureBlockedWriteMaxWait` assignment racing with
the event pump's concurrent `waitDuration()` read.  Consequently the 100 ms
blocked-write coverage exists but is not race-safe; verification cannot accept
the repair.

## Native Windows boundary

The CI definition runs `go test -count=1 ./...` on `windows-latest`.
This Mac verifier cannot execute the generated PE file. The exact SHA is not
present on `origin`: `git ls-remote` returns no branch and the read-only GitHub
check-runs API returns HTTP 422 for `93e6a6c`. Therefore no native Windows CI
result is attached to this SHA. Cross-compilation does not satisfy that gate.

## Clearing condition

`bug-fix` must remove the concurrent mutation of `Client.MaxWait` while
preserving the 100 ms blocked-write behavior, then repeat focused/package/race
checks. A published repaired SHA must receive the repository's
`windows-latest` Go test job before it can be verified for this Windows flake.
