# Claude Code config-dir and Keychain isolation Spike

Status: **in progress**. Config-slot isolation and the machine-readable auth
contract have reproducible evidence. A second distinct Claude account,
completed setup-token issuance/injection, a long session, and revocation remain
open.

## Safety boundary

No Keychain database, Keychain secret, email address, organization identifier,
OAuth token, authorization URL, authorization code, or terminal capture is
stored in this repository. Identity values are compared only in process memory
and reduced to booleans.

## Current official contract

- [Environment variables](https://code.claude.com/docs/en/env-vars) documents
  `CLAUDE_CONFIG_DIR` as the configuration root and explicitly calls it useful
  for multiple accounts. It also says macOS credentials live in Keychain while
  Linux/Windows credentials live under the selected config directory.
- [CLI reference](https://code.claude.com/docs/en/cli-reference) documents
  `claude auth status` as JSON with exit `0` when logged in and exit `1` when
  logged out, and documents `claude setup-token` as an interactive command.
- [Authentication](https://code.claude.com/docs/en/authentication) documents a
  one-year, inference-only setup token, precedence through
  `CLAUDE_CODE_OAUTH_TOKEN`, and that the token is printed but not saved.
- The official authentication page does not provide a per-token revocation
  command or API. General documentation only makes global sign-out/admin access
  removal observable as revoked OAuth. This is an open operational gate.

## Environments

| Device | Platform | Claude Code | Auth |
|---|---|---:|---|
| Local | macOS 26.5.2 arm64 | `2.1.207` | Claude.ai login in Keychain |
| Remote | Linux 5.4.0 x86_64 | `2.1.132` | Claude.ai login in mode-`0600` credentials file |

The two `auth status --json` payloads had the same seven keys and represented
the same account and organization. Values were not persisted.

## Config Dir / Keychain experiment

1. Two new empty macOS `CLAUDE_CONFIG_DIR` paths returned exit `1` and did not
   inherit the default profile's authenticated identity.
2. Profile A completed the official browser login with Claude Code `2.1.207`.
   The default and A then represented the same identity, while empty Profile B
   remained logged out.
3. No `.credentials.json` appeared under the macOS profile, matching the
   official Keychain contract.
4. `claude auth logout` scoped to Profile A logged out A only. The default
   profile remained logged in and Profile B remained logged out.
5. On Linux, two empty config roots also remained logged out and did not inherit
   the default credentials.

This proves independent credential slots and scoped logout on the tested
macOS/CLI version. It does **not** prove two distinct account identities can be
held simultaneously because only one Claude account is available in the
current environment.

## JSON health-check experiment

`claude auth status --json` parsed successfully on macOS `2.1.207` and Linux
`2.1.132` with keys:

```text
apiProvider, authMethod, email, loggedIn, orgId, orgName, subscriptionType
```

The structure is suitable for a pinned-version adapter. Only `loggedIn`,
`authMethod`, and provider classification should be exposed to normal product
logs; email/org fields must stay redacted.

## setup-token PTY experiment

The sanitized harness
[run_setup_token_pty_probe.py](run_setup_token_pty_probe.py) allocated a PTY at
`24x80`, launched `claude setup-token`, detected an authorization URL without
emitting it, resized the PTY to `50x140`, delivered `SIGWINCH`, and verified the
process remained alive. The authorization attempt was stopped before a token
was issued. The process did not exit within three seconds of `SIGTERM`, so the
probe escalated to `SIGKILL`; the future Process Manager must retain graceful
stop plus kill escalation.

## Real-request control

Both macOS and Linux were authenticated to the same account, but one-turn
requests returned a usage/session-limit message with a reset time of 15:40
America/Los_Angeles. That is not an authentication failure. A successful
post-reset inference and long-session run remain required.

## Current result and fallback

The exact tested version supports separate macOS credential slots keyed by
`CLAUDE_CONFIG_DIR` and a stable machine-readable health check. Distinct-account
isolation is not yet proven. The deterministic fallback remains user-generated
`setup-token` injection via `CLAUDE_CODE_OAUTH_TOKEN`, with the token held by the
Vault and passed only to the child environment. Because documented per-token
revocation is absent, this fallback cannot be marked stable until issuance,
long-session, and revocation behavior are completed or the product explicitly
accepts global sign-out/admin removal as the only revocation path.

Machine-readable sanitized results are in
[auth-profile-matrix.json](auth-profile-matrix.json).
