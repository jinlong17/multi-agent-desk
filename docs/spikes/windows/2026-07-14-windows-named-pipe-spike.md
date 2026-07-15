# Windows Named Pipe local IPC Spike

## Verdict

**SUPPORTED, subject to security review and protocol-layer controls.** A native
Windows Named Pipe endpoint can preserve framed daemon messages, survive
repeated reconnects, reject anonymous and remote-style clients, and bind access
to the current logon SID with a protected DACL.

This result supports Named Pipes as the Windows local daemon IPC transport. It
does not make the OS DACL the application authorization layer: production must
still authenticate the protocol peer, authorize every mutation, enforce
`ControllerLease`, bound payloads and rates, and apply deadlines.

## Frozen hypothesis

Named Pipes can preserve local daemon message boundaries, support repeated
client reconnects, enforce a current-logon-only trust boundary, reject remote
clients, recover from an abrupt disconnect, and shut down cleanly.

## Reproducible method

The probe is implemented in
`docs/spikes/windows/named_pipe_probe_windows.go` and runs in
`.github/workflows/spike-windows-named-pipe.yml`.

1. Resolve the current process logon SID and construct a protected DACL that
   denies the Network SID and grants only that logon SID.
2. Create one message-mode pipe with `FILE_FLAG_FIRST_PIPE_INSTANCE` and
   `PIPE_REJECT_REMOTE_CLIENTS`.
3. Read the live kernel-object DACL back and reject any default-DACL fallback.
4. Spawn 100 independent client processes and round-trip versioned JSON
   messages, including a 70 KiB payload.
5. Disconnect one client during an invalid message and require the server to
   accept all following clients.
6. Attempt an anonymous-token connection and a remote-style machine path; both
   must fail.
7. Warm the process-spawn runtime, sample process handles across both halves of
   the reconnect run, and require no linear per-client handle growth.
8. Require bounded server shutdown after the final client.

Exact CI build command:

```powershell
go build -tags named_pipe_spike -trimpath -o docs/spikes/windows/.bin/named-pipe-probe.exe docs/spikes/windows/named_pipe_probe_windows.go
```

## Evidence

Final passing run: GitHub Actions `29379406760`, job
`named-pipe-windows-x64`, completed 2026-07-14. The sanitized artifact is
committed as `docs/spikes/windows/named-pipe-result.json`.

| Assertion | Result |
|---|---:|
| OS/architecture | Windows build `10.0.26100.32995`, `amd64` |
| Go | `go1.26.5` |
| DACL | protected; deny Network; allow current logon SID |
| Default DACL | not used |
| First-instance guard | enabled |
| Remote clients | rejected |
| Anonymous client | rejected |
| Independent client processes | `100` |
| Successful session matches | `101` |
| Abrupt disconnects recovered | `1` |
| Largest framed message | `71,741` bytes |
| Message boundaries | preserved |
| Transcript SHA-256 | `30db010957060d20b47ed99065b43055bdcde495173f12167cbb6dc86d9a8123` |
| Handles baseline / midpoint / end | `127 / 143 / 142` |
| Second-half handle growth | `-1` |
| Shutdown | `0 ms` |

The initial run `29378469528` passed the functional and authorization checks
but failed a cold-start process-handle heuristic (`117` to `142`). Run
`29379229462` later showed why a four-handle steady-state threshold was still
too tight: its second half grew by five handles (`143` to `148`), far below the
50 handles expected from a one-handle-per-client leak, while all transport and
security assertions passed. The final probe warms eight child launches, takes
the minimum of five quiescent samples at each checkpoint, and allows at most ten
retained runtime handles across clients 51–100. Final run `29379406760`
decreased from `143` to `142` in that interval.

## Security contract checked

- [Named Pipe Security and Access Rights](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-security-and-access-rights)
  documents why the permissive default DACL is unsuitable and how a logon SID
  can distinguish Terminal Services sessions.
- [Named Pipes](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipes)
  defines local and network access behavior.
- [Named Pipe Operations](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-operations)
  covers per-client instances and connection lifecycle.
- [Named Pipe Type, Read, and Wait Modes](https://learn.microsoft.com/en-us/windows/win32/ipc/named-pipe-type-read-and-wait-modes)
  defines message-mode boundary behavior.

## Scope limits and fallback

- The GitHub-hosted runner is Windows Server, not a physical Windows 11
  workstation.
- The probe covers one current interactive logon SID; a later Windows 11
  acceptance lane must exercise two simultaneous user sessions.
- The DACL constrains who can open the transport. It does not replace protocol
  identity, per-command authorization, lease ownership, or abuse controls.
- If Windows 11 multi-session acceptance fails, use loopback transport with an
  equivalent local access-control handshake while Windows remains Experimental.

## Decision recommendation

Adopt native message-mode Named Pipes for Windows local daemon IPC with a
protected current-logon-SID DACL, Network denial, remote-client rejection, and
first-instance protection. Require security review before resolving the Spike
and carry protocol authentication, authorization, deadlines, rate/payload
limits, and `ControllerLease` into Phase 1 implementation.
