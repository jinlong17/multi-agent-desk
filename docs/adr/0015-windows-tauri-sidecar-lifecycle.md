# ADR 0015: Windows Tauri sidecar survives Desktop exit

- Status: Accepted
- Date: 2026-07-14
- Owner: `desktop`
- Impacted modules: `core`, `security`
- Security gate: accepted by `docs/reviews/spike-windows-desktop-sidecar/2026-07-14-security-review.md`

## Context

Windows Desktop needs to connect to one Device Daemon without creating
split-brain Vault/session state. A user may already have a system-installed
Daemon, or Desktop may need to start a packaged fallback sidecar. Detach and
Desktop crash must not destroy active Provider sessions, while an explicit stop
must never target a system service or another owner's daemon.

Phase 0.5 built a pinned Tauri `2.11.5`/shell plugin `2.3.5` Rust host, a Go
sidecar, and an NSIS externalBin bundle on Windows x64. Final run `29383875862`
verified target-suffixed resolution and installer inclusion, graceful owned-tree
cleanup, process-tree survival across Desktop abort, restart reuse without a
duplicate, wrong-owner stop denial, pre-existing-daemon separation, and zero
final orphan processes.

The fixture used a token-file control channel and an unsigned/uninstalled NSIS
archive. Those are test mechanisms, not production trust anchors.

## Decision

Use a discover-first, crash-surviving Windows Tauri sidecar lifecycle:

1. Desktop first discovers and mutually authenticates a system-installed
   Daemon over the ADR 0013 protected Named Pipe. When present and compatible,
   Desktop attaches and never manages that daemon's lifecycle.
2. When no daemon exists, Desktop may launch only the fixed, signed, packaged
   `multidesk` sidecar through a compile-time Rust `ShellExt::sidecar` name.
   Frontend/webview input cannot select a command, executable, path, or
   arguments, and generic shell execution is disabled.
3. The daemon owns the first-pipe/single-instance lock. Desktop authenticates a
   random daemon instance ID, daemon and Desktop Device IDs, ownership mode,
   executable/protocol version, and start epoch over Named Pipe IPC before it
   attaches or records ownership.
4. Desktop exit, webview reload, detach, or crash does not stop the daemon.
   Restart discovers and reuses the authenticated surviving instance.
5. Explicit stop is a separate, idempotent operation requiring authenticated
   Desktop ownership, capability, and current lease. It drains/stops only the
   owned daemon tree and never stops a system service or differently owned
   instance.
6. Signature/publisher, release provenance, architecture, version, and update
   manifest digest are verified in an ACL-protected install/update transaction.
   Mismatch, downgrade, stale ownership, incompatible version, or ambiguous
   recovery fails closed rather than spawning a second daemon.

Windows Desktop remains Experimental until signed Windows 11 installation,
upgrade/rollback/uninstall, multi-user, logoff, sleep/resume, reboot/crash,
endpoint-security, system-service coexistence, accessibility, IME, and real
Codex/Claude continuity acceptance passes.

## Consequences

### Positive

- Desktop lifecycle no longer controls Provider session lifetime; active work
  survives Desktop failure and reconnects to one daemon.
- System service and Desktop-owned sidecar authority are explicitly separated.
- Fixed Rust-side externalBin launch avoids a generic frontend code-execution
  surface.

### Obligations and residual limits

- A surviving daemon requires version negotiation, stale-instance repair,
  authenticated discovery, and updater coordination.
- A trusted signed sidecar is native code with user authority. Signer, updater,
  installed-directory, same-logon malware, and administrator compromise remain
  code-execution and credential risks.
- Windows Server CI did not execute/sign the installer or replace physical
  Windows 11 acceptance.
- If packaged sidecar acceptance fails, the fallback is a separately installed
  signed Daemon service with its own privilege/security review; Desktop must not
  silently spawn beside it.

## Evidence

- `docs/spikes/windows/2026-07-14-windows-tauri-sidecar-spike.md`
- `docs/spikes/windows/tauri-sidecar-result.json`
- `docs/spikes/windows/tauri-sidecar-probe/`
- `docs/reviews/spike-windows-desktop-sidecar/2026-07-14-security-review.md`
- GitHub Actions run `29383875862`
