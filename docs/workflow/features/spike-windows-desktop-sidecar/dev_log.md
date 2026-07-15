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
| Current Phase | `SECURITY_REVIEW` |
| Status | `ACCEPTED` |
| Executor | `Codex (GPT-5), security-review` |
| Updated | `2026-07-14 19:38 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `resolved — ACCEPTED only with fixed Rust-side launch, signed/provenance-verified package, ADR 0013 authenticated ownership, atomic updates, and Windows 11 acceptance` |
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
| 2026-07-14 19:20 -0700 | GitHub Actions run `29383265674` | Tauri host and NSIS externalBin built; lifecycle step exposed a harness-only PowerShell pipeline wait on crash-surviving console handles; run cancelled | workflow logs; commit `4dd42bd` |
| 2026-07-14 19:29 -0700 | GitHub Actions run `29383875862` after host-process wait correction | all packaging, ownership, graceful stop, abort survival, reconnect/reuse, wrong-owner denial, pre-existing-daemon, descendant cleanup, and artifact assertions passed | `docs/spikes/windows/tauri-sidecar-result.json`; run `29383875862` |

## Result, limitations, and fallback

Evidence ready. The real Tauri externalBin/NSIS build launched the resolved
sidecar. Graceful owner stop cleaned the process tree; Desktop abort preserved
it; restart reused the same daemon without a duplicate; wrong-owner shutdown
was denied; a pre-existing system-style daemon was never claimed/stopped; final
orphan count was zero. Security review must accept packaged-binary and ownership
requirements. Fallback: Windows Desktop stays Experimental and uses a separately
installed signed Daemon service.

## Risks and Blockers

- Blocks the Windows Desktop lifecycle decision until security review accepts the signed-binary/authenticated-ownership boundary.
- Windows Server CI does not replace Windows 11 installer, updater, logoff, sleep/resume, service, security-software, and real-Provider acceptance.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 21:50 -0700 | Claude Code (Fable 5), lifecycle-readiness P2 build | Spike created by R2 single-owner split of spike-windows-conpty-sidecar | this file | `DRAFT` | feature-plan |
| 2026-07-14 17:43 -0700 | Codex (GPT-5), feature-plan spike intake | Confirmed sole `desktop` ownership with `core` impact; opened the external-binary/ownership security gate; froze actual Tauri externalBin packaging, plugin launch, crash survival, duplicate prevention, owner-only cooperative shutdown, descendant cleanup, and pre-existing-daemon criteria | this file; dashboard state; `codex/desktop/spike-windows-desktop-sidecar` | `SPIKE_READY`; Windows Server CI scope separated from Windows 11 installer/workstation acceptance | provider-spike |
| 2026-07-14 19:38 -0700 | Codex (GPT-5), provider-spike | Built the pinned Tauri/NSIS externalBin fixture on Windows, corrected a harness-only pipe wait, reproduced owner/crash/reconnect/pre-existing-daemon semantics, persisted sanitized hashes/results, and refreshed dashboard binding | `ebbc219`; Actions `29383875862`; `docs/spikes/windows/`; dashboard; this file | `EVIDENCE_READY`; Security Gate remains open | security-review |
| 2026-07-14 19:38 -0700 | Codex (GPT-5), security-review | Reviewed packaged-binary authenticity, fixed Rust launch, frontend shell exclusion, authenticated instance ownership, split-brain/stop authority, crash continuity, update/downgrade, audit safety, Windows 11 gaps, service fallback, and residual risk | `docs/reviews/spike-windows-desktop-sidecar/2026-07-14-security-review.md`; this file | `ACCEPTED`; Security Gate resolved for the constrained lifecycle decision | feature-plan decision |
