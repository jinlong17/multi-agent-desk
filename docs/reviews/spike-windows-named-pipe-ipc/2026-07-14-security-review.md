# Security review: Windows Named Pipe local IPC

Date: 2026-07-14  
Role: `security-review`  
Verdict: `ACCEPTED`

## Scope of acceptance

This verdict accepts native Windows Named Pipes as the local Daemon IPC
transport under the controls below. It does not accept the probe as production
IPC code and does not treat a Windows DACL, client PID, or session ID as
application identity or authorization.

Phase 1 must implement mutual protocol authentication, per-request capability
and `ControllerLease` checks, bounded resource use, and fail-closed endpoint
ownership. Windows 11 multi-user acceptance remains a release gate.

## Evidence reviewed

- `docs/spikes/windows/2026-07-14-windows-named-pipe-spike.md`
- `docs/spikes/windows/named-pipe-result.json`
- `docs/spikes/windows/named_pipe_probe_windows.go`
- Passing GitHub Actions run `29378594831` and retained negative harness run
  `29378469528`
- `docs/THREAT_MODEL.md`, especially T-07, T-08, T-14, T-15, and T-18
- `docs/IMPLEMENTATION_PLAN.md` local IPC, single-instance, Attachment, and
  `ControllerLease` contracts
- [Microsoft Named Pipe Security and Access Rights](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights)
- [Microsoft Named Pipes](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipes)
- [Microsoft Named Pipe Client Impersonation](https://learn.microsoft.com/en-us/windows/win32/ipc/impersonating-a-named-pipe-client)

## Findings and required downstream controls

### P1 — same-logon access is not peer authentication

The protected DACL correctly removes the unsafe default and limits the pipe to
the current logon SID while denying the Network SID. Every process in that
logon session can still attempt to connect. Client PID and session ID are
useful audit context, not stable cryptographic identity.

Acceptance boundary:

- mutually authenticate Daemon and client before accepting any protected
  request; a client must also reject an unauthenticated pipe server;
- bind the authenticated local Device/client identity, protocol version,
  request ID, capability, and lease revision to each mutation;
- require the current `ControllerLease` for input, resize, approval, stop, and
  kill operations; observer connections remain read-only;
- use the client's restricted security quality-of-service flags and never
  grant authorization solely because impersonation or PID lookup succeeds;
- treat same-user malware and host administrator compromise as explicit
  residual risk, not as a property solved by the DACL.

### P1 — endpoint-name squatting must fail closed

`FILE_FLAG_FIRST_PIPE_INSTANCE` detects an already-owned name but does not
authenticate a server to a connecting client. A same-logon process can race a
predictable endpoint, causing denial of service or attempting server
impersonation.

Production must bind endpoint discovery to the installed Daemon identity,
retain single-instance ownership without an unauthenticated fallback, fail
closed on first-instance conflict, authenticate the server in the protocol
handshake, and audit ownership conflicts. Clients must never silently retry a
different or less-protected transport.

### P1 — transport access must not permit unbounded resource consumption

The probe enforces a one-MiB read ceiling, process timeouts, sequential clients,
and bounded shutdown. Production requires per-message schema bounds, read/write
deadlines, connection and request concurrency limits, rate limiting, slow-client
eviction, cancellation, idempotency, and cleanup tests for partial messages and
disconnect storms. Invalid input must not reach Provider or Vault operations.

### P2 — service and multi-session DACL construction needs acceptance evidence

The evidence covers one GitHub-hosted Windows Server logon SID. A packaged
Daemon may run from an interactive user process, startup task, or service
context; those token/session differences must not accidentally grant Session 0,
another signed-in user, Administrators, Everyone, or Anonymous access.

The Windows 11 lane must read the live DACL, test two simultaneous users and
Fast User Switching, validate sleep/resume and service restart, and prove that
only the intended interactive logon can connect. Any service-mode design must
document how the target logon SID is selected and refreshed.

### P2 — audit records must exclude IPC payloads

Audit endpoint ownership conflicts, authenticated peer identity, capability
decision, lease revision, request type, result, and bounded error code. Do not
log terminal content, Provider prompts, credentials, raw messages, or security
tokens. PID and session ID may be recorded only as non-authoritative context.

## Verdict rationale

No P0 condition was found. The probe demonstrated that a protected
current-logon DACL, explicit Network denial, remote-client rejection,
first-instance protection, message framing, reconnect recovery, and bounded
teardown work on the tested Windows x64 runner. The evidence is sufficient to
select the OS transport, while the controls above prevent that selection from
being overstated as end-to-end local authorization.

`ACCEPTED` resolves this Spike's Security Gate only. Omitting mutual protocol
authentication, fail-closed endpoint ownership, capability/lease checks, or
resource bounds from the Phase 1 decision must reopen the gate.

## Residual risk

Malware running within the same logon session can attempt connections, consume
local resources, race endpoint ownership, inspect the user's memory, or invoke
authorized clients. Host administrator/kernel compromise can bypass the local
boundary. A DACL or later revocation cannot undo content or credentials already
copied by a compromised local process.

## Handoff

**Target**: `Windows Named Pipe local Daemon IPC`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: Native message-mode Named Pipes are accepted only with the protected current-logon DACL plus mutual protocol authentication, fail-closed endpoint ownership, capability/lease authorization, and bounded resource use.
**Findings**: No P0; P1 same-logon access is not identity, endpoint squatting requires authenticated fail-closed ownership, and resource bounds are mandatory; P2 service/multi-session DACL acceptance and redacted audit evidence remain.
**Evidence**: `docs/spikes/windows/`, GitHub Actions runs `29378594831` and `29378469528`, `docs/THREAT_MODEL.md`, `docs/IMPLEMENTATION_PLAN.md`, and Microsoft Named Pipe documentation.
**Residual Risk**: Same-logon malware and host administrators can attack availability or act with local user authority; DACLs and revocation cannot erase already copied material.

### Next Step

Run `feature-plan` to record the Named Pipe decision and bind every accepted
control into the Phase 1 architecture and threat model.
