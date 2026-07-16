# Claude Code config-dir and Keychain isolation Spike

Status: **conclusive with fallback**. Config-slot isolation and the
machine-readable auth contract have reproducible evidence. The operator does
not require a second account. Completed setup-token issuance/injection, a long
session, and per-token revocation are not supported by this evidence; official
interactive login per target profile is the selected path.

## Safety boundary

No Keychain database, Keychain secret, email address, organization identifier,
OAuth token, authorization URL, authorization code, or terminal capture is
stored in this repository. Identity values are compared only in process memory
and reduced to booleans.

## Current official contract

- The official Environment variables page available during the experiment
  documented `CLAUDE_CONFIG_DIR` as the configuration root and explicitly
  called it useful for multiple accounts. It also said macOS credentials live
  in Keychain while Linux/Windows credentials live under the selected config
  directory.
- The official CLI reference available during the experiment documented
  `claude auth status` as JSON with exit `0` when logged in and exit `1` when
  logged out, and documented `claude setup-token` as an interactive command.
- The official Authentication page available during the experiment documented
  a one-year, inference-only setup token, precedence through
  `CLAUDE_CODE_OAUTH_TOKEN`, and that the token is printed but not saved.
- The official authentication page does not provide a per-token revocation
  command or API. General documentation only makes global sign-out/admin access
  removal observable as revoked OAuth. The stable design therefore does not
  depend on setup-token distribution.

Those exact claims are retained as dated Spike evidence, not as a promise that
the documentation locations are permanent. On 2026-07-16 the prior
`code.claude.com` endpoints redirected outside the documentation site and
failed the repository HTTP-link gate. They were retired from live links; future
compatibility work must revalidate the current official CLI and documentation.
The stable upstream project entry is the
[official Claude Code repository](https://github.com/anthropics/claude-code).

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
current environment. The operator explicitly scoped v0.1 to one account, so
this is a disclosed limitation rather than a remaining exit criterion.

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
process remained alive. The authorization attempt did not issue a token; a
later interactive attempt reached hCaptcha and was allowed to time out without
automation or bypass. The process did not exit within three seconds of
`SIGTERM`, so the probe escalated to `SIGKILL`; the future Process Manager must
retain graceful stop plus kill escalation. No token injection, long-session,
or revocation claim follows from this PTY evidence.

## Real-request control

Both macOS and Linux were authenticated to the same account, but one-turn
requests returned a usage/session-limit message rather than an authentication
error. A later sanitized macOS retry again returned a quota/session limit while
`auth status --json` remained healthy. It did not exercise resume or a long
session. The result is persisted in
[profile-session-control.json](profile-session-control.json); no polling wait is
required because long-session behavior is excluded from the support claim.

## Current result and fallback

The exact tested version supports separate macOS credential slots keyed by
`CLAUDE_CONFIG_DIR`, scoped logout, and a stable machine-readable health check.
Distinct-account isolation is not claimed. The stable v0.1 path is official
interactive login in each target profile/device. `setup-token` distribution by
`CLAUDE_CODE_OAUTH_TOKEN` remains experimental and unavailable to normal
CredentialGrant flows until issuance, injection, long-session, and revocation
semantics have separate accepted evidence. A target without an interactive
Claude login must fail closed with an explicit login-required state.

Machine-readable sanitized results are in
[auth-profile-matrix.json](auth-profile-matrix.json).
