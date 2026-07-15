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
| Time-box | `operator-scoped: one account + conservative interactive-login fallback` |
| Current Phase | `EVIDENCE` |
| Status | `EVIDENCE_READY` |
| Executor | `Claude Code 2.1.207 + Codex provider-spike` |
| Updated | `2026-07-14 19:31 -0700` |
| Suggested Next | `security-review` |
| Security Gate | `open — Keychain, setup-token, and revocation touch credentials (SOP_SPIKE rule 5); security-review required on evidence` |
| Evidence Path | `docs/spikes/claude/` |
| Decision Record | `pending — PROVIDER_COMPATIBILITY.md entry` |

## Success and failure criteria

- Supported when: dual-profile isolation and auth-status parsing reproduce on a clean macOS account.
- Falsified when: Keychain entries collide across profiles or setup-token cannot be driven via PTY.

## Environment

| Field | Value |
|---|---|
| Tool + version | Claude Code CLI `2.1.207` |
| OS | macOS 26.5.2 arm64 primary; Linux control still required |
| Auth mode | active Claude.ai login in macOS Keychain; setup-token is the fallback experiment arm |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-14 14:46 -0700 | Parsed `claude auth status --json` on macOS `2.1.207` and Linux `2.1.132`; compared identity only in memory | same seven-key schema, same account/org, both logged in | `docs/spikes/claude/auth-profile-matrix.json` |
| 2026-07-14 14:51 -0700 | Empty profile A/B status, interactive login into A, then scoped logout from A | A did not inherit login, completed login, B stayed logged out; A logout did not log out default | `docs/spikes/claude/2026-07-14-config-keychain-spike.md` |
| 2026-07-14 14:53 -0700 | `setup-token` in a real PTY with resize and sanitized capture | authorization flow started; process survived resize; no token persisted; stop escalated from TERM to KILL | `docs/spikes/claude/run_setup_token_pty_probe.py` |
| 2026-07-14 14:55 -0700 | Real one-turn probes on macOS and Linux | both reached account quota/session limit until 15:40, not an authentication error | `docs/spikes/claude/2026-07-14-config-keychain-spike.md` |
| 2026-07-14 19:31 -0700 | Sanitized macOS profile-session retry and operator one-account/fallback decision | auth JSON remained healthy; real request remained quota/session limited, not an auth error; no resume/long-session claim; setup-token excluded from stable support and per-profile interactive login selected | `docs/spikes/claude/profile-session-control.json` |

## Result, limitations, and fallback

Evidence ready. Independent macOS Keychain credential slots and scoped logout
were observed for default/Profile A/Profile B, and JSON auth status was stable
across the tested macOS/Linux versions. The operator does not require a second
identity. Completed setup-token issuance/injection, long-session survival, and
per-token revocation are unsupported rather than pending. The selected stable
boundary is official interactive login per target profile/device; setup-token
CredentialGrant remains experimental and fail-closed.

## Risks and Blockers

- Blocks Phase 3 design freeze only until security review accepts the per-profile interactive-login boundary.
- The environment has one Claude account; distinct identities are not proven or claimed, and the operator does not require them for v0.1.
- Official docs do not expose per-setup-token revocation; global sign-out/admin access removal is not equivalent to a targetable product revocation contract.
- Live inference remains account quota/session limited; auth JSON remains healthy and the design does not wait on a quota reset.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-10 21:50 -0700 | Claude Code (Fable 5), lifecycle-readiness P2 build | Security Gate opened per R2 review P0-C (SOP_SPIKE rule 5: credentials/auth in scope) | this file | `DRAFT`, gate `open` | feature-plan |
| 2026-07-14 14:49 -0700 | Codex provider-spike, feature-plan | Froze the macOS Config Dir/Keychain isolation, JSON auth status, setup-token PTY, long-session, and revocation criteria; pinned Claude Code `2.1.207` | this file | `SPIKE_READY` | provider-spike |
| 2026-07-14 14:55 -0700 | Codex provider-spike | Proved same-account macOS Keychain slot isolation/scoped logout, cross-version JSON health checks, and setup-token PTY initiation/resize; recorded quota and remaining gates | `docs/spikes/claude/`; this file | `SPIKE_READY`, experiment incomplete | provider-spike after quota reset / second identity or fallback decision |
| 2026-07-14 14:55 -0700 | Codex provider-spike | Refreshed the operator-owned dashboard to Phase 0.5 active with a status binding to this Spike | `docs/workflow/project/dashboard-state.json` | dashboard focus `SPIKE_READY` | continue provider-spike |
| 2026-07-14 19:31 -0700 | Codex provider-spike | Applied the operator one-account scope, retained only secret-safe auth/session evidence, classified quota as non-auth failure, excluded unverified setup-token/long-session/revocation behavior, and selected official interactive login per target profile | `docs/spikes/claude/`; this file; dashboard | `EVIDENCE_READY`, gate remains `open` | security-review |
