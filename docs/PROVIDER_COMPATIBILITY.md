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

## Pending Phase 0.5 gates

| Gate | Owner | Status | Compatibility claim |
|---|---|---|---|
| [Codex auth and refresh](workflow/features/spike-codex-auth-refresh/dev_log.md) | `provider` | Pending Spike evidence | none |
| [Claude config and keychain](workflow/features/spike-claude-config-keychain/dev_log.md) | `provider` | Pending Spike evidence | none |
| [Browser key storage](workflow/features/spike-browser-key-storage/dev_log.md) | `web` | `GATE_RESOLVED` | Chrome/Edge/Firefox native; Safari/WebKit software-wrapped fallback; unknown failures metadata-only |
| [E2EE protocol vectors](workflow/features/spike-e2ee-protocol-vectors/dev_log.md) | `security` | `GATE_RESOLVED` | pairwise roots, HPKE Auth wrapping, deterministic nonce derivation, and Go/TypeScript vectors accepted under ADR 0011 |
| [Windows ConPTY](workflow/features/spike-windows-conpty/dev_log.md) | `provider` | `GATE_RESOLVED` | native ConPTY selected under ADR 0012; Windows 11 real-provider acceptance retained |
| [Windows Named Pipe IPC](workflow/features/spike-windows-named-pipe-ipc/dev_log.md) | `core` | DRAFT; not started | none |
| [Windows desktop sidecar](workflow/features/spike-windows-desktop-sidecar/dev_log.md) | `desktop` | DRAFT; not started | none |
