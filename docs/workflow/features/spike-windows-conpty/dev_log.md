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
| Current Phase | `PROVIDER_SPIKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex (GPT-5)` |
| Updated | `2026-07-14 16:40 -0700` |
| Suggested Next | `provider-spike` |
| Security Gate | `none (no credentials in scope)` |
| Evidence Path | `docs/spikes/windows/` |
| Decision Record | `pending — PROVIDER_COMPATIBILITY.md + platform matrix entry` |

## Success and failure criteria

- Supported when: a full-screen TUI renders, resizes, and replays correctly under ConPTY across a long session with clean teardown.
- Falsified when: ConPTY corrupts full-screen output, loses resize events, or cannot tear down cleanly.

## Environment

| Field | Value |
|---|---|
| Tool + version | Go `1.26.0`; native `CreatePseudoConsole`, `ResizePseudoConsole`, and `ClosePseudoConsole` APIs |
| OS | GitHub-hosted `windows-latest` (`x64`); Windows 11 desktop acceptance remains outside this automated Spike |
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
| 2026-07-14 16:40 -0700 | Codex (GPT-5), feature-plan spike intake | Classified ConPTY under the sole `provider` owner; froze native-API probe scope, 15-second interactive stress, resize observation, transcript replay, and bounded teardown criteria; refreshed the operator-directed dashboard snapshot | this file; `docs/workflow/project/dashboard-state.json`; `docs/prototypes/dev-dashboard/state.generated.js`; `codex/provider/spike-windows-conpty` | `SPIKE_READY`; `project:verify` passed; no credentials or trust boundary in scope | provider-spike |
