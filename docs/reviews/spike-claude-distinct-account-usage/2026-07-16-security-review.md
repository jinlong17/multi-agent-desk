# Security Review: Claude distinct-account subscription policy boundary

- Date: 2026-07-16
- Target: `spike-claude-distinct-account-usage`
- Owner module: `provider`
- Reviewed commit: `c8f8ddf`
- Verdict: **ACCEPTED**

## Scope and conclusion

The Spike's falsified stable-product result is accepted. Current Anthropic
guidance does not provide MultiAgentDesk a stable entitlement to manage Claude
subscription OAuth accounts or route subscription traffic as a third-party
product. It directs developers building products/tools for others to Claude
Console API-key or supported-cloud authentication, while describing any
subscription access from third-party tools as discretionary and potentially
charged through usage credits.

Stopping the technical experiment at that decisive policy gate was the safe
choice. A second login or a quota-consuming request could not turn a
discretionary Provider allowance into an accepted product contract. The
evidence truthfully retains prior profile-slot isolation as mechanism evidence,
claims neither two distinct identities nor Usage binding, and provides a
deterministic stable fallback.

This verdict accepts the negative decision. It does not approve an API-key
implementation, cloud-provider integration, subscription wrapper, direct OAuth
token storage, status-line collector, or Phase 3 product build.

## Trust-boundary review

### Subscription identity and traffic

MultiAgentDesk must not present itself as native Claude Code, proxy or pool
subscription credentials, route third-party traffic against subscription
limits, infer permission from a successful OAuth login, or depend on a
discretionary allowance. Direct use of the user-installed official Claude Code
CLI remains an external user action, not a managed MultiAgentDesk stable
credential surface.

The previous ADR 0016 target-profile interactive-login decision predates the
current explicit third-party-product guidance. The feature-plan decision must
amend or supersede that stable claim. Until then, the new compatibility row is
the narrower authority: one-account slot/auth-health evidence remains
reproducible, but managed subscription login, distinct-account support, and a
subscription Usage dashboard are unsupported.

### Config and credential isolation

Current documentation and sanitized checks still support the limited mechanism
claim that `CLAUDE_CONFIG_DIR` separates settings/history/plugins and
Linux/Windows credential files, while macOS credentials use Keychain. Empty
config roots did not inherit the default auth on either tested platform, and no
API-key, OAuth-token, or cloud-provider override was present during the check.

Those facts do not make subscription credentials exportable or grantable. The
stable product must continue to exclude Keychain copying, setup-token grants,
`CLAUDE_CODE_OAUTH_TOKEN` storage, default-profile inheritance, and silent
account switching.

### Usage and status-line data

The official status-line schema documents optional 5h/7d percentage and reset
fields after the first response. Its input also contains sensitive operational
metadata such as paths, session identifiers, transcript location, cost, model,
and token state. A future approved collector would need a local allowlist that
immediately reduces input to Account/Profile-bound window kind, percentage,
reset, source version, and observation time; raw payloads, paths, transcripts,
identity fields, and command output must not enter logs, dashboard state, or
the Control Plane.

No such collector is approved here. Missing subscription windows remain
`unknown`/`unavailable`, and MultiAgentDesk must not spend quota to refresh a
dashboard or fabricate monthly remaining credit.

### API-key and cloud fallback

The policy fallback changes billing and credential boundaries. API-key
environment variables override subscription auth and can charge a different
Console account. A future stable adapter must therefore preview and confirm the
Provider, Account/workspace, auth source, and billing source; isolate the key in
the Device Vault; keep it out of Profile settings/process listings/logs; and
use Provider-native cost/limit data. Existing generic P1 metadata contracts do
not by themselves implement or approve this credential path.

## Findings

### P0

None.

### P1

None for accepting the negative Spike decision. No subscription product build
is authorized.

### P2 / required decision constraints

1. Amend or supersede ADR 0016 so target-local subscription login is no longer
   described as MultiAgentDesk's stable managed authentication path. Preserve
   it only as direct official-CLI/external or explicitly experimental behavior.
2. Treat API-key/cloud integration as a separately planned, reviewed, and
   verified credential/billing feature. Do not interpret this fallback as
   implementation approval.
3. If Provider approval later reopens subscription integration, require fresh
   written policy applicability, two-distinct-identity technical evidence,
   explicit post-login account binding, sanitized status-line collection, no
   auto-rotation, and another Security Review.
4. Keep raw status-line JSON, transcript paths, account/org values, Usage
   values, OAuth artifacts, credentials, and Keychain data out of repository,
   telemetry, dashboards, and remote sync.
5. Upgrade the Linux target's SSH key-exchange policy before treating it as a
   production deployment surface.

## Verification evidence

- `docs/spikes/claude-distinct-accounts/2026-07-16-policy-and-isolation-spike.md`
- `docs/spikes/claude-distinct-accounts/2026-07-16-policy-and-isolation.json`
- `docs/spikes/claude/2026-07-14-config-keychain-spike.md`
- `docs/spikes/claude/auth-profile-matrix.json`
- `docs/adr/0016-claude-profile-interactive-login-boundary.md`
- `docs/THREAT_MODEL.md` invariants 3-5 and threats T-03/T-04/T-06/T-09/T-14/T-17
- [Anthropic login guidance](https://support.claude.com/en/articles/13189465-log-in-to-your-claude-account)
- [Claude Code environment variables](https://code.claude.com/docs/en/env-vars)
- [Claude Code status-line schema](https://code.claude.com/docs/en/statusline)
- [Paused Agent SDK subscription change](https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan)
- [API-key precedence guidance](https://support.claude.com/en/articles/12304248-manage-api-key-environment-variables-in-claude-code)
- `claude --version`, binary SHA-256, sanitized `auth status --json` key/boolean projection, empty-config-root checks, and override-presence checks on macOS and Linux — PASS
- workflow, dashboard, local-link, JSON, and diff checks at the evidence transition — PASS

## Residual risk

Provider policy, billing, SDK, subscription, status-line, credential, Keychain,
and CLI behavior can change. Direct official CLI use still exposes authenticated
state to Claude Code, the browser, same-user malware, host administrators,
backups, and crash tooling. API keys or cloud credentials would create their own
high-value secret and billing risks. A user could misunderstand external CLI
use as MultiAgentDesk support unless UI/docs keep the boundary explicit.

The negative result is safe to accept because it fails closed: no new login was
requested, no quota-only traffic was generated, no secret/identity/Usage value
was persisted, and the stable managed subscription surface remains disabled.

## Handoff

**Target**: `spike-claude-distinct-account-usage`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: `Accepted the policy-gate falsification: MultiAgentDesk must not claim stable managed Claude subscription accounts or Usage; use direct official CLI externally or separately build/review API-key/cloud authentication.`
**Findings**: `P0 none; P1 none for the negative decision; P2 amend ADR 0016, treat API-key/cloud as a separate credential/billing feature, require explicit Provider approval plus fresh evidence to reopen subscription support, sanitize status-line inputs, and harden SSH.`
**Evidence**: `sanitized policy/isolation report and JSON, prior ADR/evidence, current official Anthropic guidance, macOS/Linux version/auth-contract/empty-root/override checks, workflow/dashboard/link checks`
**Residual Risk**: `Provider policy and CLI drift, direct-CLI host/browser exposure, future API-key/cloud secret and billing risk, and user boundary confusion remain.`

### Next Step

Run `feature-plan`.
