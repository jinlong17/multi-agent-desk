# Spike log: Codex auth, usage, and concurrent refresh

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-codex-auth-refresh` |
| Title | `Codex auth, usage, and concurrent refresh` |
| Owner Module | `provider` |
| Impacted Modules | `core, security` |
| Hypothesis | `Codex supports app-server schema discovery, usage methods, a file credential store, headless device auth, and two devices refreshing one account concurrently for ≥48h without corruption` |
| Time-box | `5 days (48h soak included)` |
| Current Phase | `INTAKE` |
| Status | `DRAFT` |
| Executor | `pending assignment` |
| Updated | `2026-07-10 20:56 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `open — file credential store, device auth, and concurrent refresh touch credentials (SOP_SPIKE rule 5); security-review required on evidence` |
| Evidence Path | `docs/spikes/codex/` |
| Decision Record | `pending — PROVIDER_COMPATIBILITY.md entry` |

## Success and failure criteria

- Supported when: each sub-claim reproduces on a second machine with pinned Codex version.
- Falsified when: any sub-claim fails or requires undocumented behavior.

## Environment

| Field | Value |
|---|---|
| Tool + version | Codex CLI (pin at intake) |
| OS | Linux + macOS |
| Auth mode | device auth, file credential store |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|

## Result, limitations, and fallback

Pending. Fallback direction per plan: reduce to single-writer refresh with CAS if concurrent refresh is unsafe.

## Risks and Blockers

- Blocks Phase 2 design freeze (not Phase 1).

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-10 21:50 -0700 | Claude Code (Fable 5), lifecycle-readiness P2 build | Security Gate opened per R2 review P0-C (SOP_SPIKE rule 5: credentials/auth in scope) | this file | `DRAFT`, gate `open` | feature-plan |
