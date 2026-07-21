# Bug verification: Windows Named Pipe close flake

- Date: 2026-07-20 PDT
- Role: `bug-verify`
- Target: `windows-named-pipe-close-flake`
- Owner: `core`
- Impacted modules: `security`, `project-system`
- Verified head: `4454dfc849f62fd89042c5e93782553e723809f3`
- PR: [#26](https://github.com/jinlong17/multi-agent-desk/pull/26)
- Verdict: `READY_TO_SHIP`

## Conclusion

The repair closes the diagnosed synchronous-I/O race and is ready for the
explicitly authorized Ship step. Fresh Windows-native evidence exercises the
original authenticated idle-connection shutdown path and the three added
lifecycle regressions. The implementation now registers overlapped operations
before Close can begin, rejects new I/O after close starts, cancels outstanding
operations, waits for their completion, and closes each pipe handle once.

No implementation, test, plan, dashboard, commit, push, or PR state was changed
by this verification. This verdict writes only this report and the atomic
status/evidence transition in the target bug log.

## Original failure reproduced

I independently retrieved PR #23 Actions run
[29789591659](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659),
attempt 1 job
[88508269207](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659/job/88508269207).
The authoritative log reproduces the diagnosed failure:

- `TestNamedPipeAuthenticatedDaemon` timed out after `10m0s`;
- the closing goroutine was blocked in `syscall.CloseHandle(0x218)`;
- the connection goroutine was blocked in synchronous
  `syscall.ReadFile(0x218)`;
- another goroutine remained in `ConnectNamedPipe`.

This is the same handle-level completion race described by bug-diagnose, not a
Provider, SQLite, authentication, or documentation failure.

## Exact source and Windows runtime

PR #26 remained Draft and locked to:

- base `154f882e8baea8165b729dfa53d7bdf4d3e546f1`;
- head `4454dfc849f62fd89042c5e93782553e723809f3`;
- Actions synthetic merge
  `524107c51ebd5191555af8ec886da9ed873ef1cb`.

The checkout log explicitly says that the runner merged the exact head into
the exact base. The synthetic merge and head have the identical Git tree
`d563504fbf708eadd40aa9ba89b467bd72fd5b3f`; `git diff` between them is empty.
The test result therefore applies to the exact candidate content.

Fresh native environment from
[Windows job 88525033621](https://github.com/jinlong17/multi-agent-desk/actions/runs/29795204963/job/88525033621):

- Microsoft Windows Server 2025 Datacenter `10.0.26100`;
- runner image `windows-2025-vs2026`, version `20260714.173.1`;
- Go `1.26.5 windows/amd64`.

The job ran the Windows-tagged `internal/device` package twice without any
`-run` filter:

1. `go test -count=1 ./...` — `internal/device` passed in `6.052s`.
2. `npm run scaffold:verify` -> `go test ./...` — `internal/device` passed
   again in `6.601s`, not as a cached result.

Because both commands executed the complete Windows package at the exact tree,
both included:

- `TestNamedPipeAuthenticatedDaemon`: authenticate, complete a request, keep
  the peer open, and require `Server.Close`/`Serve` to settle within two
  seconds;
- `TestNamedPipeListenerCloseUnblocksPendingAccept`: synchronize on a submitted
  accept, cancel it, and bound both Close and Accept completion;
- `TestNamedPipeCloseUnblocksPendingRead`: create a submitted idle read, close
  it, and repeat/double-close over 32 independent iterations;
- `TestNamedPipeReadDeadlineCancelsPendingRead`: cancel a submitted idle read at
  a 100-ms deadline and require `os.ErrDeadlineExceeded` within two seconds.

Across the two native package executions the pending-read close stress path ran
64 iterations, while authenticated idle close, pending accept, deadline, and
repeated-close behavior all passed twice. macOS and cross-compilation are not
used as substitutes for this Windows runtime result.

## Implementation and scope review

The production delta is confined to
`internal/device/endpoint_windows.go`; regression coverage is confined to
`internal/device/endpoint_windows_test.go`. The remaining two PR files are the
bug diagnosis and state log. There is no delta in `server.go`, protocol/auth,
Provider, storage, migrations, or non-Windows endpoints.

The completion invariant is explicit:

1. Named Pipe server and client handles use `FILE_FLAG_OVERLAPPED`.
2. Accept/read/write allocate event-backed `OVERLAPPED` values and wait through
   `GetOverlappedResult`.
3. A connection adds the operation to its wait group while holding the same
   mutex that guards the closing state.
4. Close marks the connection closed, calls `CancelIoEx`, waits for every
   registered operation, and only then calls `CloseHandle` once; concurrent or
   repeated Close waits on the same completion channel.
5. Deadlines cancel the specific overlapped operation and map the completed
   cancellation to `os.ErrDeadlineExceeded`.

ADR 0013 controls are preserved: protected current-logon DACL, Network SID
deny, `PIPE_REJECT_REMOTE_CLIENTS`, `FILE_FLAG_FIRST_PIPE_INSTANCE`, startup
DACL readback, message mode, same-session verification, and protocol mutual
authentication. The new overlapped flag changes I/O lifecycle only and does
not broaden endpoint access.

## Adjacent and protected checks

Independent local checks at the exact head:

- `go test -count=1 ./internal/device` — pass, `1.407s`;
- `go test -race -count=1 ./internal/device` — pass, `6.077s`;
- `go test -count=1 ./...` — pass across all Go packages;
- `go vet ./...` — pass;
- `GOOS=windows GOARCH=amd64 go vet ./internal/device` — pass, static only;
- `GOOS=windows GOARCH=amd64 go test -c ... ./internal/device` — pass,
  compile only.

PR #26 required checks are seven of seven successful:

- `project-verify`;
- `build-ubuntu`;
- `build-macos`;
- `build-windows`;
- `license-gate`;
- `dco`;
- `link-check`.

## Findings

None. No scope creep, security-boundary regression, missing native runtime
evidence, or protected-check failure remains.

## Residual evidence boundary

The evidence is a bounded hosted-runner regression, not a long-duration
production soak or a Windows 11 release certification. That does not block this
bugfix: the exact low-level close race runs 64 synchronized iterations across
two fresh package executions, the original authenticated server path passes
twice, and the implementation's completion-before-close ordering is directly
reviewable. Broader Windows release acceptance remains governed by the
project's existing platform gates.
