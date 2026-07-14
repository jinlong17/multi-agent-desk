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
| Current Phase | `SECURITY_REVIEW` |
| Status | `ACCEPTED` |
| Executor | `Codex security-review` |
| Updated | `2026-07-14 15:42 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `resolved — fallback accepted only with documented software-wrapped downgrade controls` |
| Evidence Path | `docs/spikes/browser/` |
| Decision Record | `pending — feeds E2EE protocol ADR` |

## Success and failure criteria

- Supported when: each browser demonstrably stores and uses a key that cannot be exported, or falls back per documented matrix.
- Falsified when: any target browser can neither hold a non-exportable key nor support the fallback safely.

## Environment

| Field | Value |
|---|---|
| Tool + version | Chrome `150.0.7871.116`; Edge `149.0.4022.98`; Firefox `152.0.4`; Safari runner `26.4`; local WebKit on macOS `26.5.2` |
| OS | macOS 26.5.2 arm64 + GitHub-hosted Windows and Linux runners |
| Auth mode | WebCrypto, IndexedDB |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-14 15:28 -0700 | Playwright and Selenium, Chrome 150, two independent browser processes over one temporary profile | native Ed25519/X25519 and encrypted fallback passed; all stored private/wrapping keys remained non-extractable | `docs/spikes/browser/chrome-macos.json` |
| 2026-07-14 15:31 -0700 | GitHub Actions engine matrix defined for Edge/Windows, Firefox/Linux, and Safari/macOS | external evidence queued on branch push | `.github/workflows/browser-key-spike.yml` |
| 2026-07-14 15:33 -0700 | PR #6 run 29372955467 | Edge/Windows and Firefox/Linux passed native and fallback paths across process restart; Safari cross-session control confirmed WebDriver isolation | `docs/spikes/browser/edge-windows.json`, `firefox-linux.json`, `safari-webdriver-isolation-macos.json` |
| 2026-07-14 15:33 -0700 | Minimal WKWebView harness, three separate app processes | Ed25519 and AES-GCM fallback persisted; X25519 read returned `TypeError`; fallback makes WebKit E2EE-eligible | `docs/spikes/browser/webkit-macos.json` |
| 2026-07-14 15:36 -0700 | PR #6 run 29373451433 | Edge/Windows, Firefox/Linux, Safari 26.4, and separate-process WebKit matrix all passed their declared compatibility paths | `docs/spikes/browser/edge-windows.json`, `firefox-linux.json`, `safari-macos.json`, `webkit-macos-ci.json` |

## Result, limitations, and fallback

Chrome, Edge/Windows, and Firefox/Linux pass native and encrypted-fallback paths
across a full process restart. WebKit/macOS passes only Ed25519 plus the
encrypted fallback across separate app processes because persisted X25519 use
returned `TypeError`. Safari 26.4 and separate-process WebKit evidence agree on
the same fallback boundary. The fallback reduces raw-key-at-rest exposure but does not protect against
same-origin script; browsers failing both paths must be metadata-only.

## Risks and Blockers

- Blocks Phase 4b E2EE design freeze.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-14 15:00 -0700 | Codex web provider-spike, feature-plan | Froze per-browser non-exportable CryptoKey, IndexedDB persistence, encrypted fallback, and metadata-only downgrade criteria; pinned locally available browsers | this file | `SPIKE_READY` | provider-spike |
| 2026-07-14 15:31 -0700 | Codex web provider-spike | Implemented two-process WebCrypto/IndexedDB PoC, reproduced Chrome through Playwright and Selenium, and added a real-engine CI matrix | `docs/spikes/browser/`, `.github/workflows/browser-key-spike.yml` | Chrome `PASS`; external browsers queued | push branch and collect artifacts |
| 2026-07-14 15:33 -0700 | Codex web provider-spike | Collected Edge/Windows and Firefox/Linux passes, identified Apple WebDriver isolation, and added a separate-process WKWebView harness | PR #6 run `29372955467`, `docs/spikes/browser/` | Edge/Firefox `PASS`; WebKit `PASS_WITH_FALLBACK`; Safari session rerun required | push corrected Safari probe and rerun matrix |
| 2026-07-14 15:36 -0700 | Codex web provider-spike | Completed the real-engine matrix and stored sanitized artifacts for every declared platform path | PR #6 run `29373451433`, `docs/spikes/browser/` | `EVIDENCE_READY`; Chrome/Edge/Firefox `PASS`, Safari/WebKit `PASS_WITH_FALLBACK` | security-review |
| 2026-07-14 15:42 -0700 | Codex security-review | Reviewed origin compromise, raw-key exposure, non-exportability limits, storage loss, recovery, trust anchor, revocation, audit binding, and unresolved protocol gates | `docs/reviews/spike-browser-key-storage/2026-07-14-security-review.md`, this file | `ACCEPTED`; Security Gate resolved for the Spike only | feature-plan decision |
