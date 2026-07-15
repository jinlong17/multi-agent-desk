# Spike log: Windows Named Pipe daemon IPC

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-windows-named-pipe-ipc` |
| Title | `Windows Named Pipe daemon IPC` |
| Owner Module | `core` |
| Impacted Modules | `desktop` |
| Hypothesis | `A local-only Windows Named Pipe with an explicit current-logon DACL can preserve daemon protocol message boundaries, authorize the local IPC trust boundary, survive repeated client reconnects, reject remote access, and shut down cleanly` |
| Time-box | `2 days` |
| Current Phase | `SECURITY_REVIEW` |
| Status | `ACCEPTED` |
| Executor | `Codex (GPT-5)` |
| Updated | `2026-07-14 17:30 -0700` |
| Suggested Next | `feature-plan decision` |
| Security Gate | `resolved — transport accepted only with mutual protocol authentication, fail-closed endpoint ownership, capability/lease checks, resource bounds, and Windows 11 multi-session acceptance` |
| Evidence Path | `docs/spikes/windows/` |
| Decision Record | `pending — platform matrix entry` |

## Success and failure criteria

- Supported when: a current-logon-only, remote-rejecting prototype preserves framed daemon protocol messages, survives at least 100 client reconnects plus an abrupt disconnect, rejects an unauthorized/remote-style connection, and shuts down within a bounded interval.
- Falsified when: Named Pipes cannot enforce the local trust boundary, preserve framing, support reconnection semantics, or terminate cleanly.

## Environment

| Field | Value |
|---|---|
| Tool + version | Go `1.26.5`; Win32 Named Pipe, token/SID, and security-descriptor APIs |
| OS | GitHub-hosted `windows-latest` (`x64`); Windows 11 workstation acceptance remains outside this automated Spike |
| Auth mode | not applicable |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-15 00:15Z | GitHub Actions run `29378469528`, `named-pipe-windows-x64` | Functional/security assertions passed; rejected by an over-broad startup handle-growth heuristic (`117` to `142`) | failed run log; retained as negative harness evidence |
| 2026-07-15 00:17Z | GitHub Actions run `29378594831`, `named-pipe-windows-x64` | Passed on Windows `10.0.26100.32995`/amd64: protected current-logon DACL, anonymous/remote denial, 100 independent reconnects, abrupt-disconnect recovery, 71,741-byte message, no second-half handle growth, 0 ms shutdown | `docs/spikes/windows/named-pipe-result.json`; `docs/spikes/windows/2026-07-14-windows-named-pipe-spike.md` |

## Result, limitations, and fallback

Supported for the automated Windows x64 transport scope. Message-mode Named
Pipes preserved framing and reconnect behavior, and the protected DACL plus
remote rejection constrained transport access to the current logon SID.

Limitations: the passing host is a GitHub Windows Server runner rather than a
physical Windows 11 workstation; it tests one logon SID, not simultaneous user
sessions; OS access control does not replace protocol authentication,
authorization, lease enforcement, deadlines, rate limits, or payload bounds.
Fallback: loopback transport with equivalent local access-control handshake,
recorded in the platform matrix.

## Risks and Blockers

- Blocks Phase 1 Windows IPC design freeze.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-11 09:20 -0700 | Claude Code (Fable 5), lifecycle-readiness P3 build | Spike created by R3 single-owner re-split of spike-windows-pty-ipc (IPC → core per module-registry signals) | this file | `DRAFT` | feature-plan |
| 2026-07-14 17:03 -0700 | Codex (GPT-5), feature-plan spike intake | Confirmed sole `core` ownership; opened the mandatory security gate; froze current-logon DACL, remote rejection, framing, 100 reconnects, abrupt disconnect recovery, and bounded shutdown criteria; refreshed dashboard focus | this file; `docs/workflow/project/dashboard-state.json`; `codex/core/spike-windows-named-pipe-ipc` | `SPIKE_READY`; default Named Pipe DACL explicitly rejected | provider-spike |
| 2026-07-14 17:22 -0700 | Codex (GPT-5), provider-spike | Ran native message-mode Named Pipe evidence on Windows x64; retained the failed startup-handle heuristic, corrected it to measure steady-state per-client growth, captured a passing 100-client result, and isolated mutually exclusive Windows Spike build tags | probe/workflow; `docs/spikes/windows/named-pipe-result.json`; `docs/spikes/windows/2026-07-14-windows-named-pipe-spike.md`; this file | `EVIDENCE_READY`; hypothesis supported within recorded limits | security-review |
| 2026-07-14 17:30 -0700 | Codex (GPT-5), security-review | Reviewed the DACL, local/remote boundary, peer identity, endpoint ownership, resource exhaustion, service/multi-session behavior, auditability, and residual same-logon/admin risk | `docs/reviews/spike-windows-named-pipe-ipc/2026-07-14-security-review.md`; this file | `ACCEPTED`; no P0, with mandatory Phase 1 controls recorded | feature-plan decision |
