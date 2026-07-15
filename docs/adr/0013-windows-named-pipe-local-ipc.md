# ADR 0013: Windows Named Pipe local IPC

- Status: Accepted
- Date: 2026-07-14
- Owner: `core`
- Impacted modules: `desktop`, `security`
- Security gate: accepted by `docs/reviews/spike-windows-named-pipe-ipc/2026-07-14-security-review.md`

## Context

The Windows CLI, TUI, and Desktop need a local transport to the Device Daemon
that preserves framed protocol messages, survives client reconnects, enforces a
least-privilege OS boundary, and supports single-instance Daemon ownership.
Unix-domain sockets remain the macOS/Linux reference, but are not the Windows
mechanism selected by this project.

Phase 0.5 evidence exercised native Win32 message-mode Named Pipes on an x64
GitHub-hosted Windows runner. The probe used a protected DACL scoped to the
current logon SID, denied the Network SID, rejected remote clients, read the
live object DACL back, recovered from an abrupt disconnect, served 100
independent client processes, preserved a 71,741-byte message boundary, showed
no steady-state per-client handle growth, and shut down in the measured interval.

The security review confirmed that this proves the transport boundary, not
application identity. Every process in the same logon session can still
attempt to connect or race endpoint ownership.

## Decision

Use native message-mode Windows Named Pipes for local Device Daemon IPC.

The Windows IPC adapter must:

- create the endpoint with a protected DACL that denies the Network SID and
  grants access only to the intended interactive logon SID; never use the
  permissive default Named Pipe DACL;
- set `PIPE_REJECT_REMOTE_CLIENTS` and `FILE_FLAG_FIRST_PIPE_INSTANCE`, read
  the live DACL back during startup, and fail closed on ownership or policy
  mismatch;
- retain stable single-instance ownership and never silently fall back to a
  different pipe name, an unprotected pipe, or unauthenticated loopback;
- mutually authenticate Daemon and client above the transport before exposing
  protected data or accepting mutation; pipe possession, PID, session ID, and
  OS impersonation are not sufficient identity;
- bind protocol version, authenticated local Device/client identity, request
  ID, capability, and lease revision to every request;
- require the current `ControllerLease` for terminal input, resize, approval,
  stop, and kill; observer connections remain read-only;
- enforce schema and payload bounds, read/write deadlines, connection and
  request concurrency limits, rate limits, cancellation, idempotency, and
  cleanup after malformed messages, slow clients, and disconnect storms;
- record only bounded identity/decision/error metadata in audit events and
  exclude credentials, terminal/model content, and raw IPC payloads.

The packaged Windows 11 lane must test interactive-user, startup-task, and any
proposed service context; two simultaneous signed-in users; Fast User
Switching; restart; sleep/resume; live-DACL verification; endpoint squatting;
and authenticated client/server rejection. A service design must explicitly
select and refresh the target interactive logon SID rather than granting
Session 0 or broad administrator access.

## Consequences

### Positive

- Phase 1 can freeze one local IPC abstraction with Unix-domain sockets on
  macOS/Linux and native Named Pipes on Windows.
- Message boundaries, repeated reconnects, remote rejection, and current-logon
  access control have reproducible Windows x64 evidence.
- Endpoint ownership conflicts fail closed instead of starting a split-brain
  Daemon.

### Obligations and residual limits

- Same-logon malware can still connect, race the endpoint, consume resources,
  or invoke an already authorized client; host administrators can bypass local
  isolation.
- Client PID and Windows session ID are audit context only. Mutual protocol
  authentication and per-request authorization remain mandatory.
- Windows Server CI does not replace Windows 11 multi-user/service acceptance.
- If the Windows 11 lane fails, the fallback is loopback transport only with an
  equivalent authenticated local access-control handshake; Windows remains
  Experimental and no silent downgrade is allowed.

## Evidence

- `docs/spikes/windows/named_pipe_probe_windows.go`
- `docs/spikes/windows/named-pipe-result.json`
- `docs/spikes/windows/2026-07-14-windows-named-pipe-spike.md`
- `docs/reviews/spike-windows-named-pipe-ipc/2026-07-14-security-review.md`
- GitHub Actions final passing run `29379406760`
- GitHub Actions retained negative harness runs `29378469528` and `29379229462`
