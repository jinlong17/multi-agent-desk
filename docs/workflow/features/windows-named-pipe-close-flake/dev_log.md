# Bug log: Windows Named Pipe close can hang behind a synchronous read

## Status Panel

| Field | Value |
|---|---|
| Workflow | `BUGFIX` |
| Target | `windows-named-pipe-close-flake` |
| Title | Windows Named Pipe close can hang behind a synchronous read |
| Owner Module | `core` |
| Impacted Modules | `security`, `project-system` |
| Current Phase | `SHIP` |
| Status | `SHIPPED` |
| Executor | `Codex (GPT-5) as ship` |
| Updated | `2026-07-20 19:43 PDT` |
| Suggested Next | `none` |
| Branch / Worktree | `codex/core/windows-named-pipe-close-flake` / `/Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/windows-named-pipe-close-flake` |
| Provider Gate | `none` |
| Security Gate | `none` |

## Reproduction

| Field | Value |
|---|---|
| Environment / versions | PR #23 head `8fb25a57d1f9baa2afa6894ea98dfae73f4b078e`, synthetic merge ref `fe3912c3718fc0918f86594de0cb4e857d33263c`, Windows Server 2025 Datacenter `10.0.26100`, `windows-2025-vs2026` image `20260714.173.1`, Go `1.26.5 windows/amd64`; diagnosed against final `main` `154f882e8baea8165b729dfa53d7bdf4d3e546f1` where the affected product code is unchanged. |
| Minimal reproduction | Observed remotely in `npm run scaffold:verify` -> `go test ./...`. The reduced Windows stress command for repair verification is `go test -run '^TestNamedPipeAuthenticatedDaemon$' -count=100 -timeout=5m ./internal/device`: authenticate, complete one request, leave the client connection open so the server waits for the next frame, then call `server.Close()`. The single-test stress command is a regression-test shape, not a claim that the macOS host can execute Windows runtime code. |
| Expected behavior | `server.Close()` cancels the listener and all active connection I/O, returns promptly, and lets `Server.Serve` settle within the test's two-second bound even while the peer remains open. |
| Actual behavior | Attempt 1 blocked for the suite's full ten-minute timeout. The test goroutine was inside `pipeConn.Close -> syscall.CloseHandle(0x218)` while the server connection goroutine was inside synchronous `syscall.ReadFile(0x218)` waiting for the next frame. The pending accept goroutine was also still inside synchronous `ConnectNamedPipe`. |

## Root cause (bug-diagnose)

The Windows endpoint performs blocking synchronous I/O but closes it as if it
had tracked asynchronous completion:

1. `createNamedPipe` and `openNamedPipe` omit `FILE_FLAG_OVERLAPPED`, and
   `pipeConn.Read` / `Write` pass a nil `OVERLAPPED` pointer. The connection
   goroutine therefore blocks in a synchronous `ReadFile` while waiting for the
   next frame.
2. `pipeConn.Close` calls `CancelIoEx(handle, nil)` and immediately calls
   `CloseHandle` without waiting for the outstanding read to complete. The
   five-minute deadline uses the same non-waiting cancellation path.
3. Microsoft distinguishes synchronous cancellation (`CancelSynchronousIo`)
   from asynchronous cancellation (`CancelIoEx`) and states that
   `CancelIoEx` does not wait for canceled operations to complete. The failing
   stack demonstrates the resulting race on the same handle: `CloseHandle`
   and `ReadFile` remained blocked concurrently until the ten-minute test
   alarm. The fact that the five-minute deadline did not release the read is
   additional direct evidence that the current cancellation mechanism is not
   a reliable completion barrier for this synchronous handle.
4. The failed-job rerun used the same source and runner image and passed, so
   scheduling sometimes lets the read unwind before close and sometimes
   exposes the missing completion synchronization. This is a latent Windows
   transport race, not a deterministic request/authentication failure.

Microsoft's `go-winio` reference avoids this lifecycle by using overlapped I/O
and an I/O completion port, tracking every outstanding operation, issuing
`CancelIoEx`, waiting for all operations, and only then closing the handle.
The repository does not currently depend on `go-winio`; this comparison is
evidence for the required close semantics, not a preselected dependency.

The Phase 1 P2 correction remains logically necessary but is incomplete on
this transport. It added `Server` active-connection tracking and made
`Server.Close` invoke each connection's `Close`. The present failure is below
that layer: the Windows `pipeConn.Close` invoked by the corrected server can
itself block. PR #23 changed only governance documentation, and PR #22 plus the
immediate rerun passed with the same endpoint implementation, so no Provider or
SQLite change introduced this symptom.

## Fix scope (smallest repair)

- Repair only the Windows Named Pipe I/O lifecycle in
  `internal/device/endpoint_windows.go`: use cancellable overlapped operations
  (including pending `ConnectNamedPipe`, reads, writes, and deadlines), track
  their completion, reject new I/O after close begins, cancel outstanding I/O,
  wait for completion, then close the handle exactly once.
- Preserve ADR 0013's protected current-logon DACL, Network deny,
  `PIPE_REJECT_REMOTE_CLIENTS`, first-instance ownership, live DACL readback,
  message mode, same-session check, and protocol mutual-authentication boundary.
- Add/strengthen only Windows lifecycle regression coverage in
  `internal/device/endpoint_windows_test.go`: hold an authenticated idle peer
  open, prove bounded `Server.Close` and `Serve` completion over a stress loop,
  exercise idle-read deadline cancellation, and check repeated/double close
  does not leak or reuse a handle. A Windows runner is required for the runtime
  verdict.
- `internal/device/server.go` should retain its active tracking and close
  ordering unless the fix demonstrates a narrowly required synchronization
  adjustment. Do not broaden the repair into protocol, auth, Provider, SQLite,
  session, or cross-platform server redesign.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-21 00:12-00:24 UTC | bug-diagnose | PR #23 CI attempt 1, `npm run scaffold:verify` -> `go test ./...` | `FAIL`; `TestNamedPipeAuthenticatedDaemon` timed out after 10m. Same handle `0x218` was blocked in `CloseHandle` and synchronous `ReadFile`. | [run 29789591659, job 88508269207](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659/job/88508269207) |
| 2026-07-21 00:24-00:28 UTC | bug-diagnose | Rerun failed job with unchanged PR head/image | `PASS`; `internal/device` passed twice in the job (`4.990s`, `5.604s`), establishing intermittency rather than clearing the race. | [job 88510103968](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659/job/88510103968) |
| 2026-07-20 23:56-23:59 UTC | bug-diagnose | PR #22 `build-windows` on the same endpoint implementation | `PASS`; confirms the product tree can pass unchanged and that PR #23's docs-only delta is not causal. | [job 88505865029](https://github.com/jinlong17/multi-agent-desk/actions/runs/29788780639/job/88505865029) |
| 2026-07-15 05:09-05:13 UTC | bug-diagnose | Phase 1 verification job at `c9cb5c6`, Go `1.26.5 windows/amd64` | `PASS`; `internal/device` `2.419s`. The active-tracking correction was previously green but did not eliminate the latent Windows close race. | [run 29390736909, job 87273433004](https://github.com/jinlong17/multi-agent-desk/actions/runs/29390736909/job/87273433004) |
| 2026-07-20 17:43 PDT | bug-diagnose | Static inspection of `endpoint_windows.go:168-228`, `server.go:95-129`, and `endpoint_windows_test.go:16-87` at `154f882` | `DIAGNOSED`; synchronous handle + non-waiting cancellation/close matches the captured stacks and flaky pass/fail behavior. | [diagnosis report](../../../reviews/windows-named-pipe-close-flake/2026-07-20-bug-diagnose.md) |
| 2026-07-20 17:43 PDT | bug-diagnose | `go test -count=1 ./internal/device` on macOS arm64 | `PASS` in `1.672s`; confirms no common-path failure, but is not Windows runtime proof. | local command output |
| 2026-07-20 17:43 PDT | bug-diagnose | `GOOS=windows GOARCH=amd64 go test -c -o /dev/null ./internal/device` | `PASS`; Windows test package compiles, but cross-compilation is not runtime proof. | local command output |
| 2026-07-20 19:00 PDT | bug-fix | Replaced synchronous Named Pipe accept/read/write with event-backed overlapped operations; close now rejects new I/O, cancels pending operations, waits for completion, then closes each handle once | `PASS` by static invariant review; DACL, remote-client denial, same-session check, message framing and server/auth ownership unchanged | `internal/device/endpoint_windows.go` |
| 2026-07-20 19:02 PDT | bug-fix regression | Added pending-accept cancellation, 32 pending-read close iterations, idle-read deadline cancellation and repeated-close checks | Windows-only tests compile; each test synchronizes on submitted operations rather than relying on an arbitrary sleep | `internal/device/endpoint_windows_test.go` |
| 2026-07-20 19:04 PDT | bug-fix local/compile | `go test -count=1 ./internal/device`; `go test -race -count=1 ./internal/device`; `go test -count=1 ./...`; `go vet ./...`; `GOOS=windows GOARCH=amd64 go vet ./internal/device`; `GOOS=windows GOARCH=amd64 go test -c -o /tmp/mad-windows-device-test.exe ./internal/device` | `PASS`; Windows commands are compile/static evidence only, not native runtime acceptance | local command output |
| 2026-07-20 19:05 PDT | bug-fix governance | `project:verify`; `ci:verify`; JSON/diff checks using fixed Node plus pnpm-10 npm shim | `PASS`; workflow `agents=10 skills=3 docs=17 edges=20 statuses=15`; seven CI contracts, fixtures, 282 Markdown links and licenses pass | local command output |
| 2026-07-20 19:06 PDT | bug-fix environment correction | First `scaffold:verify` reached Go but Web checks could not find `tsc` because the new worktree had no `node_modules`; ran `pnpm install --offline --frozen-lockfile` and reran the unchanged command | initial environment-only failure retained; frozen install used 17 cached packages and changed no tracked file; final `scaffold:verify` passed Go/Web/Rust checks and the release no-bundle build | local command output |
| 2026-07-20 19:14 PDT | bug-verify original-failure reproduction | Retrieved PR #23 run `29789591659`, attempt 1 job `88508269207`, directly from Actions | `FAIL` reproduced from the authoritative log: `TestNamedPipeAuthenticatedDaemon` timed out at 10m with `CloseHandle(0x218)` concurrent with synchronous `ReadFile(0x218)` and a pending `ConnectNamedPipe` | [job 88508269207](https://github.com/jinlong17/multi-agent-desk/actions/runs/29789591659/job/88508269207) |
| 2026-07-20 19:18 PDT | bug-verify exact-tree and scope audit | Compared PR #26 head `4454dfc849f62fd89042c5e93782553e723809f3`, synthetic merge `524107c51ebd5191555af8ec886da9ed873ef1cb`, and base `154f882e8baea8165b729dfa53d7bdf4d3e546f1`; inspected `origin/main...HEAD` | `PASS`; merge and head trees are both `d563504fbf708eadd40aa9ba89b467bd72fd5b3f`; product diff is limited to the Windows endpoint and Windows tests; `server.go`, protocol/auth, Provider, storage and migrations are unchanged; DACL, Network deny, remote rejection, first-instance, message mode and same-session controls remain | [bug-verify report](../../../reviews/windows-named-pipe-close-flake/2026-07-20-bug-verify.md) |
| 2026-07-20 19:18 PDT | bug-verify Windows runtime | PR #26 run `29795204963`, job `88525033621`, Windows Server 2025 `10.0.26100`, image `windows-2025-vs2026` `20260714.173.1`, Go `1.26.5 windows/amd64`; unfiltered `go test -count=1 ./...` plus `scaffold:verify -> go test ./...` | `PASS`; `internal/device` completed in `6.052s` and `6.601s`. Both native executions included the authenticated idle-server close regression plus pending-accept, 32-iteration pending-read close/double-close, and idle-read deadline tests; total pending-read close stress iterations=`64` | [job 88525033621](https://github.com/jinlong17/multi-agent-desk/actions/runs/29795204963/job/88525033621) |
| 2026-07-20 19:18 PDT | bug-verify protected matrix | PR #26 required checks at exact head: `project-verify`, `build-ubuntu`, `build-macos`, `build-windows`, `license-gate`, `dco`, `link-check` | `PASS`; seven of seven successful, PR merge state `CLEAN` and Draft retained | [PR #26](https://github.com/jinlong17/multi-agent-desk/pull/26) |
| 2026-07-20 19:16 PDT | bug-verify local adjacent/static | `go test -count=1 ./internal/device`; `go test -race -count=1 ./internal/device`; `go test -count=1 ./...`; `go vet ./...`; Windows `go vet` and test-binary cross-compile | `PASS`; device `1.407s`, race `6.077s`, full Go green; Windows commands treated only as compile/static corroboration, not runtime acceptance | local command output |
| 2026-07-20 19:30 PDT | ship source integration | Protected rebase merge of PR #26 from locked head `3350c8241c5aaadb2a7a4e6439822c7ddd2ccacf`; direct locked-head versus landed-tree comparison | `PASS`; implementation landed as `0891bdb2d3454cdec747db63b6a77d0e99696086`, verification/state as `8e15dd13f0a4c79948a360070639b2a17219d73b`, with no tree drift | [PR #26](https://github.com/jinlong17/multi-agent-desk/pull/26) |
| 2026-07-20 19:43 PDT | ship exact-main reconciliation | Final combined `main@621a4a217394d78bdf3fd14aadfd37d5df15e246`; CI run `29796308826` and Governance run `29796308835` | `PASS`; exact remote main passed `project-verify`, Ubuntu, macOS, Windows, license, DCO and links (`7/7`) | [ship receipt](../../../reviews/windows-named-pipe-close-flake/2026-07-20-ship-receipt.md) |
| 2026-07-20 19:44 PDT | ship local receipt verification | `pnpm run project:verify`; `pnpm run ci:verify`; `go test -count=1 ./...`; `go vet ./...`; `git diff --check` | `PASS`; operator-directed project verification refreshed and verified generated dashboard facts (`dirty=4`) without changing the operator-owned dashboard judgment; workflow, seven CI contracts, 288 links, licenses, Go and vet all pass | local command output |

## Risks and Blockers

- Impact is Windows daemon availability and lifecycle reliability: an idle
  connected peer can make stop/update/test teardown hang. No evidence shows an
  authentication bypass, plaintext disclosure, or integrity violation.
- The server tracks a connection before authentication finishes, so the
  availability exposure also covers a current-logon peer stalled during
  handshake. ADR 0013's DACL, session, and remote-client restrictions narrow
  the attacker to the existing same-logon residual-risk boundary; the repair
  must not relax those controls.
- A macOS run and Windows cross-compile cannot verify the repair. `bug-fix`
  is therefore only `READY_FOR_VERIFY`; independent `bug-verify` must inspect
  fresh native Windows stress and the full protected job before Ship.
- This bug is explicitly separate from the Provider/SQLite runtime flake:
  the failing stack contains only `internal/device` Named Pipe and server
  lifecycle frames, while `internal/providers/codex`, `internal/storage`, and
  migrations completed successfully in the timed-out job.
- Blockers for `bug-fix`: none. The implementation choice must prove the
  completion-before-close invariant and preserve the security boundary.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-20 17:43 PDT | `Codex bug-diagnose` | Classified the bug as `core`, retrieved both Windows attempts and prior Phase 1 evidence, reduced the captured stacks against Win32 and `go-winio` close semantics, and bounded the repair. | `docs/workflow/features/windows-named-pipe-close-flake/dev_log.md`; `docs/reviews/windows-named-pipe-close-flake/2026-07-20-bug-diagnose.md`; no commit | `DRAFT -> DIAGNOSED`; root cause supported; production/tests unchanged. | Run `bug-fix` for `windows-named-pipe-close-flake`. |
| 2026-07-20 19:10 PDT | `Codex (GPT-5) as bug-fix` | Implemented overlapped, cancellation-aware and completion-tracked accept/read/write/close semantics; added synchronized Windows lifecycle regressions; preserved server, protocol, authentication, DACL and non-Windows behavior; ran local, race, Windows compile/static and full governance/scaffold evidence | this file; `internal/device/endpoint_windows.go`; `internal/device/endpoint_windows_test.go`; no commit yet | `DIAGNOSED -> READY_FOR_VERIFY`; no local blocker; native Windows acceptance remains deliberately unclaimed | independent `bug-verify` on a fresh protected Windows job |
| 2026-07-20 19:18 PDT | `Codex (GPT-5) as bug-verify` | Independently reproduced the original failure from its authoritative Actions log; locked PR/head/base and tree identity; reviewed the completion-before-close implementation, scope and ADR 0013 controls; ran adjacent local/race/static checks; and inspected both fresh native Windows package executions plus all protected checks | `docs/reviews/windows-named-pipe-close-flake/2026-07-20-bug-verify.md`; this file; exact head `4454dfc849f62fd89042c5e93782553e723809f3` | `READY_FOR_VERIFY -> READY_TO_SHIP`; all required regression, scope, security-boundary and protected-CI evidence passes; no findings or blockers | `ship` only with explicit human authorization |
| 2026-07-20 19:44 PDT | `Codex (GPT-5) as ship` | Used the operator's explicit push/PR/merge authorization, locked and protected-rebase-merged PR #26, proved landed-tree identity, verified all seven checks on exact final combined remote main, ran operator-directed project verification including generated dashboard refresh/verify, and wrote the durable remote receipt without changing dashboard judgment or creating a release | this file; `docs/reviews/windows-named-pipe-close-flake/2026-07-20-ship-receipt.md`; PR #26; landed commits `0891bdb2`, `8e15dd13`; final main `621a4a21` | `READY_TO_SHIP -> SHIPPED`; no tag, package, release, publication or deployment; no remaining bug-ship blocker | none |
