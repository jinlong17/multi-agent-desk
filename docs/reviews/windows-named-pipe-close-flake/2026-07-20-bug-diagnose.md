# Bug diagnosis: Windows Named Pipe close flake

- Date: 2026-07-20 PDT
- Role: `bug-diagnose`
- Target: `windows-named-pipe-close-flake`
- Owner: `core`
- Impacted modules: `security`, `project-system`
- Diagnosed baseline: `154f882e8baea8165b729dfa53d7bdf4d3e546f1`
- Verdict: `DIAGNOSED`

## Conclusion

The PR #23 Windows timeout is an evidence-backed close race in the custom
Windows Named Pipe transport. The connection uses synchronous `ReadFile` /
`WriteFile` / `ConnectNamedPipe`, but `pipeConn.Close` requests cancellation
with `CancelIoEx` and immediately calls `CloseHandle` without a completion
barrier. On the failed attempt, `CloseHandle` and `ReadFile` remained blocked
on the same handle until the Go test alarm fired. The unchanged rerun passed,
which explains the symptom as scheduling-dependent rather than as a docs or
Provider regression.

## Exact remote reproduction

The authoritative failure is CI run
[29789591659](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659),
attempt 1 job
[88508269207](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659/job/88508269207).
The run API identifies PR head `8fb25a57d1f9baa2afa6894ea98dfae73f4b078e`;
Actions checked out synthetic merge ref
`fe3912c3718fc0918f86594de0cb4e857d33263c`.

Environment:

- GitHub-hosted `windows-latest`;
- Microsoft Windows Server 2025 Datacenter `10.0.26100`;
- runner image `windows-2025-vs2026`, version `20260714.173.1`;
- Go `1.26.5 windows/amd64`;
- workflow step `Verify empty scaffold`, command `npm run scaffold:verify`;
- nested command `go test ./...`.

Observed stack at the ten-minute alarm:

```text
TestNamedPipeAuthenticatedDaemon (9m59s)
goroutine 42: syscall.CloseHandle(0x218)
  internal/device.(*pipeConn).Close             endpoint_windows.go:197
  internal/device.(*Server).closeActive         server.go:127
  internal/device.(*Server).Close               server.go:97
  TestNamedPipeAuthenticatedDaemon              endpoint_windows_test.go:80

goroutine 12: syscall.ReadFile(0x218, ..., nil)
  internal/device.(*pipeConn).Read               endpoint_windows.go:171
  internal/device.readFrame                      protocol.go:132
  internal/device.(*Server).serveConnection      server.go:142

goroutine 13: ConnectNamedPipe
  internal/device.(*windowsListener).Accept      endpoint_windows.go:122
```

The exact failed-job rerun,
[job 88510103968](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659/job/88510103968),
used the same PR head and runner image and passed in 4m03s. In that job,
`internal/device` passed in both Go-suite invocations (`4.990s` and `5.604s`).
PR #22's preceding
[Windows job 88505865029](https://github.com/jinlong17/multi-agent-desk/actions/runs/29788780639/job/88505865029)
also passed with the same endpoint implementation. PR #23 changed only three
governance documents, so the docs-only delta cannot explain a transport stack.

## Code-to-stack proof

At final `main` `154f882`:

- `internal/device/endpoint_windows.go:244-246` creates server pipe handles
  without `FILE_FLAG_OVERLAPPED`.
- `internal/device/endpoint_windows.go:267-268` opens client handles without
  `FILE_FLAG_OVERLAPPED`.
- `internal/device/endpoint_windows.go:171` and `:182` call `ReadFile` and
  `WriteFile` with a nil `OVERLAPPED`, making them synchronous operations.
- `internal/device/endpoint_windows.go:187-197` marks the connection closed,
  calls `CancelIoEx(handle, nil)`, and immediately calls `CloseHandle`.
- `internal/device/endpoint_windows.go:218-226` implements deadlines with the
  same `CancelIoEx` request and no completion wait.
- `internal/device/server.go:95-129` correctly reaches the active connection's
  `Close`; the captured test blocks inside that transport close, before
  `endpoint_windows_test.go:81` can cancel context or `:82-86` can apply the
  two-second Serve bound.

Microsoft documents that synchronous I/O blocks its issuing thread and should
be canceled with `CancelSynchronousIo`, while `CancelIoEx` is the cancellation
mechanism for pending asynchronous operations. It also documents that
`CancelIoEx` only marks operations for cancellation and does not wait for them
to finish:

- [Synchronous and Asynchronous I/O](https://learn.microsoft.com/en-us/windows/win32/fileio/synchronous-and-asynchronous-i-o)
- [CancelIoEx](https://learn.microsoft.com/en-us/windows/win32/fileio/cancelioex-func)
- [CancelSynchronousIo](https://learn.microsoft.com/en-us/windows/win32/fileio/cancelsynchronousio-func)

The established Go implementation pattern in Microsoft's
[go-winio `file.go`](https://github.com/microsoft/go-winio/blob/main/file.go)
opens/uses overlapped I/O, tracks every outstanding operation, calls
`CancelIoEx`, waits for its operation group, and then closes the handle. That
completion-before-close ordering is the invariant missing here. This report
does not decide whether the repair imports `go-winio` or implements equivalent
semantics locally.

## Relation to the Phase 1 shutdown correction

Phase 1's first P2 finding was cross-platform: `Server` did not retain and
close accepted connections. Commits `46d5824` and `c9cb5c6` added active
tracking, public `Server.Close`, and context-aware accept dispatch. The P2 v2
verification then passed Windows job `87273433004`, where `internal/device`
finished in `2.419s`.

That correction remains correct at the server layer. This new evidence shows a
latent transport-layer gap: `Server.Close` does call the Windows connection's
`Close`, but that method cannot guarantee progress while its synchronous read
is outstanding. No later commit changed the relevant endpoint code; the flake
surfaced only when scheduling exposed the missing completion barrier.

## Impact and security boundary

- Availability: Windows daemon stop, teardown, update, uninstall, or CI can
  hang indefinitely while an accepted connection waits for a frame.
- A connection is tracked before authentication completes, so a current-logon
  peer can exercise the same availability path during handshake. The existing
  DACL, same-session check, and remote-client rejection still limit the trust
  boundary; there is no evidence of remote access, authentication bypass,
  plaintext disclosure, or state corruption.
- The repair must preserve every ADR 0013 security flag and check. Replacing
  the Named Pipe with a permissive loopback fallback is out of scope.
- Provider/SQLite boundary: the failed stack is confined to
  `internal/device`; the same timed-out job reports `internal/providers/codex`,
  `internal/storage`, and device migrations passing. The separately tracked
  Provider/SQLite runtime flake is not part of this diagnosis or fix scope.

## Required regression shape

On an actual Windows runner:

1. create the protected listener and authenticated client;
2. complete one request/response and deliberately keep the client open;
3. synchronize until the server is waiting for the next frame;
4. call `Server.Close` and require both it and `Serve` to return within a
   bounded deadline;
5. repeat enough times (`-count=100` or an equivalent in-test loop) to expose
   scheduling races;
6. separately prove idle-read deadline cancellation and repeated/double-close
   safety without handle growth;
7. rerun the complete Windows job plus cross-platform unit/race coverage.

Cross-compilation and macOS tests remain useful structural checks, but neither
may be reported as Windows runtime verification.

## Smallest repair

Limit production changes to `internal/device/endpoint_windows.go`: make pending
accept/read/write operations cancellable and completion-tracked, prevent new
operations once closing begins, and close each handle only after outstanding
operations have completed. Add the Windows regression to
`internal/device/endpoint_windows_test.go`. Keep `server.go`, protocol/auth,
Provider, SQLite, and non-Windows transport behavior unchanged unless a test
proves a narrowly necessary adjustment.

## Handoff

**Target**: `windows-named-pipe-close-flake`
**Completed**: `bug-diagnose`
**Status**: `DIAGNOSED`
**Root Cause**: The custom Windows pipe uses synchronous I/O but Close relies on non-waiting `CancelIoEx` followed immediately by `CloseHandle`; an outstanding `ReadFile` can race and block close indefinitely.
**Evidence**: PR #23 run 29789591659 attempt 1 job 88508269207 captured `CloseHandle(0x218)` concurrent with `ReadFile(0x218)` until the 10m timeout; unchanged attempt 2 job 88510103968 passed; source and Microsoft/go-winio semantics corroborate the missing completion barrier.
**Fix Scope**: Repair overlapped/cancellable, completion-tracked Named Pipe accept/read/write/close semantics in `internal/device/endpoint_windows.go` and add Windows shutdown/deadline stress regression coverage; preserve server/auth/DACL behavior and exclude Provider/SQLite.
**Blockers**: None for `bug-fix`; an actual Windows runner is mandatory before verification.

### Next Step

Run `bug-fix` for `windows-named-pipe-close-flake`.
