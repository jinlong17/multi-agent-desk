# Bug verification: Windows Codex usage-fixture deadline flake

**Verdict:** `BLOCKED`

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
