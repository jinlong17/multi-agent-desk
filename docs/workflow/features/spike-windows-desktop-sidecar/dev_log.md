# Spike log: Windows Tauri sidecar lifecycle

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-windows-desktop-sidecar` |
| Title | `Windows Tauri sidecar lifecycle` |
| Owner Module | `desktop` |
| Impacted Modules | `core` |
| Hypothesis | `A Tauri shell on Windows can install, start, supervise, and cleanly stop the daemon as a sidecar process across app restarts and crashes` |
| Time-box | `3 days` |
| Current Phase | `INTAKE` |
| Status | `DRAFT` |
| Executor | `pending assignment` |
| Updated | `2026-07-10 21:50 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `none (no credentials or trust boundaries in scope)` |
| Evidence Path | `docs/spikes/windows/` |
| Decision Record | `pending — platform matrix entry` |

## Success and failure criteria

- Supported when: sidecar starts with the app, survives app crash/restart per the chosen policy, and stops cleanly on exit.
- Falsified when: sidecar lifecycle cannot be controlled from Tauri without OS-service privileges the app does not hold.

## Environment

| Field | Value |
|---|---|
| Tool + version | Windows 11, Tauri + Rust (pin at intake) |
| OS | Windows |
| Auth mode | not applicable |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|

## Result, limitations, and fallback

Pending. Fallback: Windows Desktop stays Experimental for v0.1; daemon installed as a separate service.

## Risks and Blockers

- Blocks desktop platform acceptance on Windows.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 21:50 -0700 | Claude Code (Fable 5), lifecycle-readiness P2 build | Spike created by R2 single-owner split of spike-windows-conpty-sidecar | this file | `DRAFT` | feature-plan |
