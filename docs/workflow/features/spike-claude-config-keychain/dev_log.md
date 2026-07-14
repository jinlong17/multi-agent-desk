# Spike log: Claude Code config-dir and Keychain isolation

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-claude-config-keychain` |
| Title | `Claude Code config-dir and Keychain isolation` |
| Owner Module | `provider` |
| Impacted Modules | `core, desktop, security` |
| Hypothesis | `On macOS, two CLAUDE_CONFIG_DIR profiles isolate accounts including Keychain entries; auth status is machine-readable JSON; setup-token works in an interactive PTY, survives long sessions, and revocation is observable` |
| Time-box | `4 days` |
| Current Phase | `INTAKE` |
| Status | `DRAFT` |
| Executor | `pending assignment` |
| Updated | `2026-07-10 20:56 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `open — Keychain, setup-token, and revocation touch credentials (SOP_SPIKE rule 5); security-review required on evidence` |
| Evidence Path | `docs/spikes/claude/` |
| Decision Record | `pending — PROVIDER_COMPATIBILITY.md entry` |

## Success and failure criteria

- Supported when: dual-profile isolation and auth-status parsing reproduce on a clean macOS account.
- Falsified when: Keychain entries collide across profiles or setup-token cannot be driven via PTY.

## Environment

| Field | Value |
|---|---|
| Tool + version | Claude Code CLI (pin at intake) |
| OS | macOS (primary), Linux (control) |
| Auth mode | setup-token, Keychain |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|

## Result, limitations, and fallback

Pending. Fallback direction per plan: deterministic setup-token injection instead of Keychain-based multi-profile.

## Risks and Blockers

- Blocks Phase 3 design freeze.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-10 21:50 -0700 | Claude Code (Fable 5), lifecycle-readiness P2 build | Security Gate opened per R2 review P0-C (SOP_SPIKE rule 5: credentials/auth in scope) | this file | `DRAFT`, gate `open` | feature-plan |
