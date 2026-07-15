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
| Current Phase | `PROVIDER_SPIKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex (GPT-5)` |
| Updated | `2026-07-14 17:03 -0700` |
| Suggested Next | `provider-spike` |
| Security Gate | `open — local IPC client authorization and cross-client control are a trust boundary` |
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

## Result, limitations, and fallback

Pending. Fallback: TCP loopback with local access control, recorded in the platform matrix.

## Risks and Blockers

- Blocks Phase 1 Windows IPC design freeze.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-11 09:20 -0700 | Claude Code (Fable 5), lifecycle-readiness P3 build | Spike created by R3 single-owner re-split of spike-windows-pty-ipc (IPC → core per module-registry signals) | this file | `DRAFT` | feature-plan |
| 2026-07-14 17:03 -0700 | Codex (GPT-5), feature-plan spike intake | Confirmed sole `core` ownership; opened the mandatory security gate; froze current-logon DACL, remote rejection, framing, 100 reconnects, abrupt disconnect recovery, and bounded shutdown criteria; refreshed dashboard focus | this file; `docs/workflow/project/dashboard-state.json`; `codex/core/spike-windows-named-pipe-ipc` | `SPIKE_READY`; default Named Pipe DACL explicitly rejected | provider-spike |
