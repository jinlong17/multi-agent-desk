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
| Current Phase | `EVIDENCE` |
| Status | `EVIDENCE_READY` |
| Executor | `Codex (GPT-5)` |
| Updated | `2026-07-14 16:51 -0700` |
| Suggested Next | `feature-plan decision` |
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
| 2026-07-14 23:47Z | GitHub Actions run `29377146037`, `conpty-windows-x64` | Failed: Go child retained host PowerShell standard handles; `GetConsoleScreenBufferInfo` returned invalid handle | failed run log; retained as negative evidence |
| 2026-07-14 23:50Z | GitHub Actions run `29377266540`, `conpty-windows-x64` | Passed on Windows `10.0.26100.32995`/amd64: exact 3-way resize, 1,405 frames over 15,004 ms, 512 replayed history markers, exit `0`, EOF, 1 ms teardown | `docs/spikes/windows/conpty-result.json`; `docs/spikes/windows/2026-07-14-windows-conpty-spike.md` |

## Result, limitations, and fallback

Supported for the automated Windows x64 transport scope. ConPTY carried the
full-screen VT fixture, resize events, input, captured history, deterministic
replay, and clean teardown. The child must reopen `CONIN$`/`CONOUT$`, and the
host must drain output on a dedicated goroutine/thread through teardown.

Limitation: the passing host is a GitHub Windows Server runner, not a physical
Windows 11 workstation, and the fixture is not a real Codex/Claude binary.
Fallback: retain ConPTY for proven CLI/Daemon transport while keeping Windows
Desktop Experimental and narrowing any Windows 11 interaction that fails the
later platform-acceptance lane.

## Risks and Blockers

- Blocks Phase 3 Windows scope (Claude PTY on Windows).

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-11 09:20 -0700 | Claude Code (Fable 5), lifecycle-readiness P3 build | Spike created by R3 single-owner re-split of spike-windows-pty-ipc (ConPTY/PTY → provider per module-registry signals) | this file | `DRAFT` | feature-plan |
| 2026-07-14 16:40 -0700 | Codex (GPT-5), feature-plan spike intake | Classified ConPTY under the sole `provider` owner; froze native-API probe scope, 15-second interactive stress, resize observation, transcript replay, and bounded teardown criteria; refreshed the operator-directed dashboard snapshot | this file; `docs/workflow/project/dashboard-state.json`; `docs/prototypes/dev-dashboard/state.generated.js`; `codex/provider/spike-windows-conpty` | `SPIKE_READY`; `project:verify` passed; no credentials or trust boundary in scope | provider-spike |
| 2026-07-14 16:51 -0700 | Codex (GPT-5), provider-spike | Ran native ConPTY evidence on Windows x64; retained the failed inherited-handle attempt, corrected child console binding, then captured a passing 15-second result and refreshed the dashboard | probe/workflow; `docs/spikes/windows/conpty-result.json`; `docs/spikes/windows/2026-07-14-windows-conpty-spike.md`; this file; dashboard state | `EVIDENCE_READY`; hypothesis supported within recorded limits | feature-plan decision |
