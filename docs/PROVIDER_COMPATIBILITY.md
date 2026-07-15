# Provider compatibility

This matrix contains only reproducible compatibility evidence. `Pending` is an
open gate, not a support claim. A row may become supported, unsupported, or
supported-with-fallback only after its owning Spike records commands, versions,
platform, artifacts, and a workflow decision.

## Evidence schema

| Provider/tool | Tested version | Platform | Capability | Evidence artifact | Result | Fallback | Date |
|---|---|---|---|---|---|---|---|
| Web Device Key / Chrome | 150.0.7871.116 | macOS 26.5.2 arm64 | non-exportable Ed25519 + X25519 in IndexedDB across process restart | `spikes/browser/chrome-macos.json` | Supported | AES-GCM software wrapping also passed | 2026-07-14 |
| Web Device Key / Edge | 149.0.4022.98 | Windows GitHub runner | non-exportable Ed25519 + X25519 in IndexedDB across process restart | `spikes/browser/edge-windows.json` | Supported | AES-GCM software wrapping also passed | 2026-07-14 |
| Web Device Key / Firefox | 152.0.4 | Linux GitHub runner | non-exportable Ed25519 + X25519 in IndexedDB across process restart | `spikes/browser/firefox-linux.json` | Supported | AES-GCM software wrapping also passed | 2026-07-14 |
| Web Device Key / Safari + WebKit | Safari 26.4; WebKit on macOS 26.4/26.5.2 | macOS | Ed25519 persists; persisted X25519 use returns `TypeError` | `spikes/browser/safari-macos.json`, `spikes/browser/webkit-macos-ci.json`, `spikes/browser/webkit-macos.json` | Supported with fallback | `software_wrapped` AES-GCM; otherwise `metadata_only` | 2026-07-14 |
| Windows ConPTY | Windows `10.0.26100.32995`; Go `1.26.5` | GitHub `windows-latest`, amd64 | full-screen VT stream, input, 3 exact resizes, 512-line captured-history replay, bounded clean teardown | `spikes/windows/conpty-result.json`, `spikes/windows/2026-07-14-windows-conpty-spike.md` | Supported with acceptance gate | Native ConPTY backend; narrow affected interactive feature if Windows 11 real-provider acceptance fails | 2026-07-14 |
| Windows Named Pipe IPC | Windows `10.0.26100.32995`; Go `1.26.5` | GitHub `windows-latest`, amd64 | protected current-logon DACL, anonymous/remote denial, 100 independent reconnects, 71,741-byte message boundary, bounded teardown | `spikes/windows/named-pipe-result.json`, `spikes/windows/2026-07-14-windows-named-pipe-spike.md` | Supported with security and acceptance gates | authenticated loopback only if Windows 11 multi-session/service acceptance fails; no silent downgrade | 2026-07-14 |
| Codex app-server account APIs | CLI `0.142.5`, `0.143.0`, `0.144.2` | macOS 26.5.2 arm64 | versioned schema generation; initialize; account, rate-limit, and usage reads | `spikes/codex/app-server-account-matrix.json`, `spikes/codex/2026-07-14-auth-refresh-spike.md` | Supported for exact tested versions | probe exact schema and disable unsupported capabilities; never fabricate official usage | 2026-07-14 |
| Codex file auth and managed refresh | CLI `0.144.2` macOS; `0.144.4` Linux | macOS 26.5.2 arm64 + Linux 5.4.0 x86_64 | `0600` file store, proactive refresh, four short-run dual-device reads | `spikes/codex/two-device-short-run.json`, ADR 0014 | Supported only with one canonical refresh writer | revisioned-CAS single writer; quarantine/re-login on ambiguity; no multi-writer or 48h claim | 2026-07-14 |
| Codex device auth | CLI `0.144.2` macOS; `0.144.4` Linux | macOS + headless Linux | isolated device-auth initiation and authorization URL | `spikes/codex/app-server-account-matrix.json` | Experimental; completion not verified | official interactive login required | 2026-07-14 |
| Windows Tauri sidecar | Tauri `2.11.5`; CLI `2.11.4`; shell plugin `2.3.5`; Rust `1.91.1`; Go `1.26.5` | Windows `10.0.26100.32995`, GitHub x64 | NSIS externalBin, fixed Rust launch, graceful tree stop, Desktop-abort survival, restart reuse, wrong-owner denial, pre-existing-daemon separation | `spikes/windows/tauri-sidecar-result.json`, `spikes/windows/2026-07-14-windows-tauri-sidecar-spike.md` | Supported with security and Windows 11 acceptance gates | separately installed signed Daemon service; Desktop remains Experimental; never spawn beside service | 2026-07-14 |
| Claude profile auth health | CLI `2.1.207` macOS; `2.1.132` Linux | macOS 26.5.2 arm64 + Linux 5.4.0 x86_64 | macOS Config Dir/Keychain credential-slot isolation and scoped logout; seven-key `auth status --json`; Linux empty-root isolation | `spikes/claude/auth-profile-matrix.json`, `spikes/claude/2026-07-14-config-keychain-spike.md` | Supported for exact versions with one-account scope | official interactive login on each target profile; unknown schema fails as `auth_health_unknown` | 2026-07-14 |
| Claude setup-token and long session | CLI `2.1.207` macOS | macOS 26.5.2 arm64 | PTY initiation/resize only; hCaptcha not bypassed; token issuance/injection/long-session/per-token revocation not verified | `spikes/claude/profile-session-control.json`, ADR 0016 | Unsupported as stable capability | target-profile official interactive login; no Claude setup-token CredentialGrant | 2026-07-14 |

## Pending Phase 0.5 gates

| Gate | Owner | Status | Compatibility claim |
|---|---|---|---|
| [Codex auth and refresh](workflow/features/spike-codex-auth-refresh/dev_log.md) | `provider` | `GATE_RESOLVED` | exact app-server methods; one canonical writable app-server/auth home under ADR 0014; interactive-login fallback; no multi-writer claim |
| [Claude config and keychain](workflow/features/spike-claude-config-keychain/dev_log.md) | `provider` | `GATE_RESOLVED` | ADR 0016 target-local interactive login, isolated Config Dir, version-gated/redacted auth JSON; setup-token grant and distinct-account/long-session claims excluded |
| [Browser key storage](workflow/features/spike-browser-key-storage/dev_log.md) | `web` | `GATE_RESOLVED` | Chrome/Edge/Firefox native; Safari/WebKit software-wrapped fallback; unknown failures metadata-only |
| [E2EE protocol vectors](workflow/features/spike-e2ee-protocol-vectors/dev_log.md) | `security` | `GATE_RESOLVED` | pairwise roots, HPKE Auth wrapping, deterministic nonce derivation, and Go/TypeScript vectors accepted under ADR 0011 |
| [Windows ConPTY](workflow/features/spike-windows-conpty/dev_log.md) | `provider` | `GATE_RESOLVED` | native ConPTY selected under ADR 0012; Windows 11 real-provider acceptance retained |
| [Windows Named Pipe IPC](workflow/features/spike-windows-named-pipe-ipc/dev_log.md) | `core` | `GATE_RESOLVED` | native message-mode Named Pipes selected under ADR 0013; protected current-logon DACL plus protocol authentication/authorization; Windows 11 multi-session/service acceptance retained |
| [Windows desktop sidecar](workflow/features/spike-windows-desktop-sidecar/dev_log.md) | `desktop` | `GATE_RESOLVED` | ADR 0015 discover-first crash-surviving sidecar; fixed Rust launch, authenticated ownership/stop, signed package; Windows 11 acceptance retained |
