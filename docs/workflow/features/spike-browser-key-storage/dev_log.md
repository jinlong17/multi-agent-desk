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
| Current Phase | `INTAKE` |
| Status | `DRAFT` |
| Executor | `pending assignment` |
| Updated | `2026-07-10 20:56 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `open — security-review must judge the fallback` |
| Evidence Path | `docs/spikes/browser/` |
| Decision Record | `pending — feeds E2EE protocol ADR` |

## Success and failure criteria

- Supported when: each browser demonstrably stores and uses a key that cannot be exported, or falls back per documented matrix.
- Falsified when: any target browser can neither hold a non-exportable key nor support the fallback safely.

## Environment

| Field | Value |
|---|---|
| Tool + version | Chrome/Edge, Safari, Firefox (pin versions at intake) |
| OS | macOS + Windows |
| Auth mode | WebCrypto, IndexedDB |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|

## Result, limitations, and fallback

Pending. Fallback: encrypted key in IndexedDB with explicit residual-risk wording.

## Risks and Blockers

- Blocks Phase 4b E2EE design freeze.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
