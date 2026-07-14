# Spike log: Browser non-exportable key storage

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-browser-key-storage` |
| Title | `Browser non-exportable key storage` |
| Owner Module | `web` |
| Impacted Modules | `security, control-plane` |
| Hypothesis | `Chrome/Edge, Safari, and Firefox can hold a non-exportable WebCrypto device key usable for E2EE, with a documented IndexedDB encrypted-key fallback where non-exportable storage is unavailable` |
| Time-box | `4 days` |
| Current Phase | `PROVIDER_SPIKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex web provider-spike` |
| Updated | `2026-07-14 15:00 -0700` |
| Suggested Next | `provider-spike` |
| Security Gate | `open — security-review must judge the fallback` |
| Evidence Path | `docs/spikes/browser/` |
| Decision Record | `pending — feeds E2EE protocol ADR` |

## Success and failure criteria

- Supported when: each browser demonstrably stores and uses a key that cannot be exported, or falls back per documented matrix.
- Falsified when: any target browser can neither hold a non-exportable key nor support the fallback safely.

## Environment

| Field | Value |
|---|---|
| Tool + version | Chrome `150.0.7871.116`; Safari `26.5.2`; Edge/Firefox absent locally and require external evidence |
| OS | macOS 26.5.2 arm64 primary + Windows runner required |
| Auth mode | WebCrypto, IndexedDB |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-14 15:28 -0700 | Playwright and Selenium, Chrome 150, two independent browser processes over one temporary profile | native Ed25519/X25519 and encrypted fallback passed; all stored private/wrapping keys remained non-extractable | `docs/spikes/browser/chrome-macos.json` |
| 2026-07-14 15:31 -0700 | GitHub Actions engine matrix defined for Edge/Windows, Firefox/Linux, and Safari/macOS | external evidence queued on branch push | `.github/workflows/browser-key-spike.yml` |

## Result, limitations, and fallback

Chrome passes native and encrypted-fallback paths across a full process restart.
Edge, Firefox, and Safari remain pending real-engine CI evidence. The encrypted
fallback reduces raw-key-at-rest exposure but does not protect against
same-origin script; browsers failing both paths must be metadata-only.

## Risks and Blockers

- Blocks Phase 4b E2EE design freeze.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-14 15:00 -0700 | Codex web provider-spike, feature-plan | Froze per-browser non-exportable CryptoKey, IndexedDB persistence, encrypted fallback, and metadata-only downgrade criteria; pinned locally available browsers | this file | `SPIKE_READY` | provider-spike |
| 2026-07-14 15:31 -0700 | Codex web provider-spike | Implemented two-process WebCrypto/IndexedDB PoC, reproduced Chrome through Playwright and Selenium, and added a real-engine CI matrix | `docs/spikes/browser/`, `.github/workflows/browser-key-spike.yml` | Chrome `PASS`; external browsers queued | push branch and collect artifacts |
