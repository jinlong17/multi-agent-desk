# Feature verification v1: Phase 1 Device Kernel P2

- Date: 2026-07-14
- Role: `feature-verify`
- Phase: `P2 identity, IPC, and Daemon lifecycle`
- Verified head: `df6fbacbdf1d2f5f2b272f22b516770d75fd9279`
- Pull request: `#13` (Draft)
- Verdict: `BLOCKED`

## Conclusion

P2's authentication, authorization, Unix endpoint, and Windows Named Pipe
execution reached the handler on the actual runners, but the Daemon did not
close an authenticated connection that was waiting for its next frame when the
listener was stopped. The same lifecycle assertion failed in the macOS,
Ubuntu, and Windows suites. P2 cannot advance until server shutdown closes
active connections and the three-platform suite passes again.

## Blocking finding

`internal/device.Server.Serve` closed the listener on context cancellation but
did not retain or close accepted connections. A connected client that had
completed the handshake remained blocked in `readFrame`, so the server
goroutine and the native endpoint lifecycle could not settle within the test
deadline. The failure was reported by:

- macOS build job `87272170390`,
- Ubuntu build job `87272170409`, and
- Windows build job `87272170387`.

The Windows log explicitly shows `TestNamedPipeAuthenticatedDaemon` and
`TestBootstrapAndAuthenticatedUnixDaemon` failing at `server did not stop`;
domain, storage, and all other scaffold checks passed. This is a real common
Daemon lifecycle defect, not a platform-specific Named Pipe authorization
failure.

## Required clearing change

The original writer must track bounded active connections, close them when the
listener or Serve context ends, accept `net.ErrClosed` as normal shutdown, and
rerun local tests/race/vet plus the actual macOS, Ubuntu, and Windows jobs.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-verify / P2 identity, IPC, and Daemon lifecycle`
**Verdict**: `BLOCKED`
**Summary**: P2 reaches authenticated handlers, but active connections are not closed during Daemon shutdown on any runner.
**Evidence**: PR #13 head df6fbac; CI run 29390337206; macOS job 87272170390; Ubuntu job 87272170409; Windows job 87272170387.
**Findings**: Rank P1 — Server shutdown leaks/waits on active authenticated connections.
**Blockers**: Active-connection closure and a fresh three-platform runner pass are required.

### Next Step

Run `feature-build` P2 lifecycle correction for `phase1-device-kernel`.
