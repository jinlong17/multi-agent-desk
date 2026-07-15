# Security review: Windows Tauri sidecar lifecycle

- Date: 2026-07-14
- Role: `security-review`
- Target: `spike-windows-desktop-sidecar`
- Evidence commit: `0f9f982`
- Final passing run: `29383875862`
- Verdict: **ACCEPTED**

## Scope

Reviewed the Tauri v2 externalBin package and fixed Rust-side launch, NSIS
inclusion, graceful owner shutdown, intentional Desktop-crash survival,
restart/reuse behavior, duplicate prevention, wrong-owner denial, pre-existing
daemon separation, descendant cleanup, fixture control channel, Windows CI
limitations, and selected separately-installed-service fallback.

Evidence reviewed:

- `docs/spikes/windows/2026-07-14-windows-tauri-sidecar-spike.md`
- `docs/spikes/windows/tauri-sidecar-result.json`
- `docs/spikes/windows/tauri-sidecar-probe/`
- `.github/workflows/spike-windows-sidecar.yml`
- ADR 0013 Windows Named Pipe authentication/authorization requirements
- `docs/THREAT_MODEL.md` T-07, T-09, T-14, T-16, and T-18

## Verdict rationale

**ACCEPTED.** The candidate demonstrates that pinned Tauri externalBin tooling
can package and launch the expected Windows x64 sidecar and that the selected
continuity policy is mechanically viable: Desktop abort does not destroy the
daemon tree, restart reuses the same instance, a second daemon is not created,
wrong-owner stop is denied, a pre-existing differently owned daemon is not
claimed, and cooperative shutdown leaves no descendants.

Acceptance is conditional on replacing every fixture trust mechanism in
production. The file token is test scaffolding, not authorization. A production
Desktop may start only the fixed, signed, packaged binary through Rust; it must
authenticate daemon identity and ownership over ADR 0013 Named Pipe IPC before
attaching or stopping. Generic frontend shell execution is not allowed.

## Findings

- P0: none.
- P1: none after treating the fixture token-file protocol as non-production and
  making authenticated Named Pipe ownership plus packaged-binary authenticity
  mandatory.
- P2: Windows Server CI inspected but did not execute the NSIS installer or
  exercise signing, upgrade, rollback, OS logoff, sleep/resume, service
  installation, endpoint security software, or Windows 11 multi-user behavior.
  Windows Desktop remains Experimental until those acceptance lanes pass.

## Required implementation obligations

1. Discover and authenticate a system-installed Daemon first. If it is healthy
   and protocol-compatible, Desktop attaches and never launches or stops a
   second instance.
2. If Desktop owns the fallback sidecar, launch only a compile-time fixed
   `ShellExt::sidecar` name from Rust. Expose no generic shell command, argument,
   executable path, or shell capability to webview/frontend input.
3. Install the sidecar under an ACL-protected application directory. Verify
   release provenance, expected publisher signature, architecture, version,
   and an update-manifest digest before launch; fail closed on mismatch. Avoid a
   verify-then-replace race by binding verification to the protected installed
   artifact and updater transaction.
4. Use ADR 0013 protected Named Pipe IPC with mutual authentication. Bind a
   random daemon instance ID, daemon Device ID, Desktop Device ID, protocol
   version, executable version, ownership mode, and boot/start epoch. A file,
   PID, inherited handle, pipe possession, or same-logon identity alone is not
   stop authority.
5. Enforce one Daemon lock/first-pipe instance. On ownership ambiguity, stale
   metadata, signature mismatch, or incompatible version, fail closed and offer
   repair; never choose the newest PID or silently spawn beside it.
6. Desktop exit/detach must not stop the daemon. Explicit stop requires the
   authenticated owner and current capability/lease, is idempotent, drains or
   terminates the owned Provider tree within policy, and never targets a system
   service or differently owned instance.
7. Make updates atomic and anti-downgrade. Coordinate Desktop, sidecar, schema,
   and installer versions; preserve rollback without running an unverified old
   binary; define behavior when a crash-surviving old daemon meets a newer
   Desktop.
8. Record bounded lifecycle decisions and error classes only. Exclude ownership
   credentials, raw IPC payloads, Provider credentials, terminal/model content,
   command lines containing secrets, and executable private paths from normal
   telemetry/support bundles.
9. Run signed Windows 11 installer/update/uninstall, standard-user/admin,
   multi-user/Fast User Switching, startup/logoff, sleep/resume, crash/reboot,
   endpoint-security, service coexistence, and real Codex/Claude continuity
   acceptance before removing Experimental status.

## Residual risk

A trusted signed sidecar is native code with the user's local authority. A
compromised signer, updater, installed directory, Desktop process, same-logon
malware, administrator, or endpoint-security exception can execute code,
impersonate lifecycle messages, deny service, or retain Provider plaintext.
Crash continuity intentionally leaves the daemon running without the Desktop;
bugs in discovery, version negotiation, stale-instance cleanup, or updater
coordination can strand it. Signature and hash checks do not protect against a
compromised legitimate release pipeline. A service fallback introduces a
higher-privilege boundary and requires its own review.

These risks are accepted only for the lifecycle design decision. Production
implementation and signed Windows 11 release acceptance remain gated.
