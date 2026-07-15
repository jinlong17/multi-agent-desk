# Security review: Claude profile authentication boundary

- Date: 2026-07-14
- Role: `security-review`
- Target: `spike-claude-config-keychain`
- Evidence commit: `13b5448`
- Verdict: **ACCEPTED**

## Scope

Reviewed macOS `CLAUDE_CONFIG_DIR`/Keychain slot isolation, scoped logout,
macOS/Linux `auth status --json`, setup-token PTY initiation, the sanitized
quota-limited real-request control, the one-account operator scope, and the
selected official-interactive-login fallback. This verdict accepts only the
constrained profile-login boundary. It does not accept setup-token distribution,
distinct-account isolation, long-session survival, or per-token revocation.

Evidence reviewed:

- `docs/spikes/claude/2026-07-14-config-keychain-spike.md`
- `docs/spikes/claude/auth-profile-matrix.json`
- `docs/spikes/claude/profile-session-control.json`
- `docs/spikes/claude/run_setup_token_pty_probe.py`
- `docs/spikes/claude/run_profile_session_probe.py`
- `docs/THREAT_MODEL.md` credential, Provider, log, and revocation boundaries
- `docs/IMPLEMENTATION_PLAN.md` Claude adapter, Vault, and CredentialGrant rules

## Verdict rationale

**ACCEPTED.** The evidence is secret-safe and supports a narrow stable design:
each target profile/device performs Claude's official interactive login; the
adapter reads a pinned-version JSON health contract; and a target without that
login fails closed. Separate macOS profiles did not inherit the default
credential, Profile A login did not authenticate Profile B, and scoped logout
did not log out the default profile.

The unsupported arms are removed from the stable design rather than inferred:
the setup-token flow did not issue a credential, hCaptcha was not automated or
bypassed, no long session completed, and official evidence does not define a
targetable per-setup-token revocation operation. Consequently normal
CredentialGrant must not carry `CLAUDE_CODE_OAUTH_TOKEN` in v0.1.

## Findings

- P0: none.
- P1: none, provided setup-token distribution remains unavailable to stable
  product flows and targets without an interactive login fail closed.
- P2: `auth status --json` contains email and organization fields. Normal logs,
  audit, telemetry, errors, dashboard facts, and support bundles must allowlist
  only non-identifying status/capability fields and never persist the raw JSON.
- P2: scoped local logout proves profile separation, not Provider-wide
  revocation or remote erasure. UI wording must distinguish local logout,
  account/admin revocation, and removal from another device.

## Required implementation obligations

1. Pin the Claude binary path/version and validate the exact `auth status`
   schema before using it. Unknown or incompatible schemas produce an explicit
   `auth_health_unknown`, not a guessed logged-in state.
2. Create each managed `CLAUDE_CONFIG_DIR` under a Daemon-owned restrictive
   directory. Do not accept an untrusted path, follow an unsafe symlink, copy
   the macOS Keychain database, or search unrelated profiles for credentials.
3. Invoke official interactive login for the exact target profile. Bind the
   resulting profile/account selection to the RuntimeProfile and Session; never
   auto-rotate or silently fall back to the default profile.
4. Persist only allowlisted health facts such as logged-in state, auth method
   class, provider class, CLI version, and validation time. Redact email,
   organization name/ID, raw status payloads, browser URLs/codes, terminal
   captures, credentials, and Provider response content.
5. Do not expose setup-token as a stable capability or CredentialGrant source.
   `CLAUDE_CODE_OAUTH_TOKEN` injection remains disabled until separate evidence
   proves issuance, target injection, session behavior, expiry, revocation, and
   secret-safe process handling, followed by another security review.
6. A quota/session-limit response is not authentication success and not an auth
   failure. Surface it as Provider availability/usage state while retaining the
   independently validated auth health and avoiding automatic account changes.
7. Local logout clears only the selected local profile. Revocation guidance
   must direct the user to official account/admin controls when broader access
   removal is required and must not promise deletion from another machine.

## Residual risk

Claude, same-user processes, host administrator/root, malware, Keychain or
credentials-file access, browser compromise during login, backups, and crash
collectors may expose or use authenticated state. Upstream Keychain service
naming, JSON fields, login flows, and credential storage may change across CLI
versions. A compromised authorized target can retain credentials and plaintext;
local logout and MultiAgentDesk revocation cannot erase copied material.
Account quota prevented a successful long-session control, so session
continuity remains a Phase 3 real-Provider acceptance item rather than a Spike
support claim.

These risks are accepted only for the constrained compatibility decision.
Production implementation must satisfy the obligations above and pass its own
security and release verification.
