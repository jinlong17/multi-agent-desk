# Spike log: Windows Tauri sidecar lifecycle

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-windows-desktop-sidecar` |
| Title | `Windows Tauri sidecar lifecycle` |
| Owner Module | `desktop` |
| Impacted Modules | `core` |
| Hypothesis | `On Windows x64, Tauri 2.11.5 plus tauri-plugin-shell 2.3.5 externalBin can package and start a daemon sidecar; the selected continuity policy can preserve the daemon process tree across a Desktop crash, reconnect without spawning a duplicate, reject non-owner shutdown, and cooperatively stop only the owned sidecar tree` |
| Time-box | `3 days` |
| Current Phase | `PROVIDER_SPIKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex (GPT-5)` |
| Updated | `2026-07-14 17:43 -0700` |
| Suggested Next | `provider-spike` |
| Security Gate | `open — packaged external-binary authenticity, process ownership, and stop authority are local code-execution trust boundaries` |
| Evidence Path | `docs/spikes/windows/` |
| Decision Record | `pending — platform matrix entry` |

## Success and failure criteria

- Supported when: a real Tauri v2 `externalBin` build resolves and launches the
  target-suffixed Windows sidecar; an owner-authorized graceful path stops the
  daemon and its child within five seconds; a hard Desktop abort leaves both
  processes alive; a restarted Desktop discovers that instance without
  spawning a duplicate; a wrong owner token cannot stop it; and a pre-existing
  system-style instance is never treated as Desktop-owned.
- Falsified when: Tauri cannot resolve/package the Windows sidecar, host exit
  unintentionally destroys the selected crash-survival process tree, restart
  creates split brain, an unowned client can stop the daemon, or cooperative
  shutdown leaves an orphaned descendant.

## Environment

| Field | Value |
|---|---|
| Tool + version | Tauri `2.11.5`; Tauri CLI `2.11.4`; `tauri-plugin-shell 2.3.5`; Rust and Go versions recorded by CI |
| OS | GitHub-hosted `windows-latest` (`x64`); Windows 11 workstation/install acceptance remains outside this automated Spike |
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
| 2026-07-14 17:43 -0700 | Codex (GPT-5), feature-plan spike intake | Confirmed sole `desktop` ownership with `core` impact; opened the external-binary/ownership security gate; froze actual Tauri externalBin packaging, plugin launch, crash survival, duplicate prevention, owner-only cooperative shutdown, descendant cleanup, and pre-existing-daemon criteria | this file; dashboard state; `codex/desktop/spike-windows-desktop-sidecar` | `SPIKE_READY`; Windows Server CI scope separated from Windows 11 installer/workstation acceptance | provider-spike |
