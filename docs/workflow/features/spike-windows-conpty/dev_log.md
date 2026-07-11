# Spike log: Windows ConPTY full-screen provider TUI

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-windows-conpty` |
| Title | `Windows ConPTY full-screen provider TUI` |
| Owner Module | `provider` |
| Impacted Modules | `core, desktop` |
| Hypothesis | `ConPTY can host a full-screen provider TUI reliably: render, resize, scrollback replay, and clean teardown behave correctly for long interactive sessions` |
| Time-box | `3 days` |
| Current Phase | `INTAKE` |
| Status | `DRAFT` |
| Executor | `pending assignment` |
| Updated | `2026-07-11 09:20 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `none (no credentials in scope)` |
| Evidence Path | `docs/spikes/windows/` |
| Decision Record | `pending — PROVIDER_COMPATIBILITY.md + platform matrix entry` |

## Success and failure criteria

- Supported when: a full-screen TUI renders, resizes, and replays correctly under ConPTY across a long session with clean teardown.
- Falsified when: ConPTY corrupts full-screen output, loses resize events, or cannot tear down cleanly.

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

Pending. Fallback: reduce Windows terminal scope for v0.1 (Windows Desktop stays Experimental per plan Phase 6).

## Risks and Blockers

- Blocks Phase 3 Windows scope (Claude PTY on Windows).

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-11 09:20 -0700 | Claude Code (Fable 5), lifecycle-readiness P3 build | Spike created by R3 single-owner re-split of spike-windows-pty-ipc (ConPTY/PTY → provider per module-registry signals) | this file | `DRAFT` | feature-plan |
