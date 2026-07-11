# Spike log: Windows Named Pipe daemon IPC

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-windows-named-pipe-ipc` |
| Title | `Windows Named Pipe daemon IPC` |
| Owner Module | `core` |
| Impacted Modules | `desktop` |
| Hypothesis | `A Named Pipe local IPC minimal prototype round-trips daemon protocol messages on Windows with correct connect/disconnect semantics and clean shutdown` |
| Time-box | `2 days` |
| Current Phase | `INTAKE` |
| Status | `DRAFT` |
| Executor | `pending assignment` |
| Updated | `2026-07-11 09:20 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `none (local IPC prototype, no credentials in scope)` |
| Evidence Path | `docs/spikes/windows/` |
| Decision Record | `pending — platform matrix entry` |

## Success and failure criteria

- Supported when: the prototype round-trips daemon protocol messages, survives client reconnects, and shuts down cleanly.
- Falsified when: Named Pipes cannot support the daemon protocol shape or reconnection semantics.

## Environment

| Field | Value |
|---|---|
| Tool + version | Windows 11, Go toolchain (pin at intake) |
| OS | Windows |
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
