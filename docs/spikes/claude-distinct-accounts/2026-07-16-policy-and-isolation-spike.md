# Claude distinct-account isolation and subscription-policy Spike

Date: 2026-07-16  
Owner: `provider`  
Workflow target: `spike-claude-distinct-account-usage`

## Verdict

The stable-product hypothesis is falsified at the Provider policy boundary.
Anthropic's current login guidance says subscription usage is designed for
native Anthropic applications, identifies open-source and other software as
third-party tools, recommends API-key or supported-cloud authentication for
those tools, and specifically directs developers building a product or tool
for others to use that path. Subscription access from some third-party tools
may be allowed at Anthropic's discretion and may draw from usage credits; that
is not a stable entitlement MultiAgentDesk can promise.

Because policy failure is an explicit Spike failure criterion, no second
distinct-account login and no quota-consuming request were run. A technically
successful login experiment could not close the stable product gate. Existing
same-account slot-isolation evidence and current non-secret health checks remain
useful for an experimental direct-CLI pass-through, but they do not authorize a
managed subscription-account dashboard or subscription traffic surface.

## Current official sources

- [Log in to your Claude account](https://support.claude.com/en/articles/13189465-log-in-to-your-claude-account), reviewed 2026-07-16: native subscription use,
  third-party-tool discretion, prohibited misrepresentation/limit routing, and
  the API-key/cloud-provider direction for products and tools.
- [Claude Code environment variables](https://code.claude.com/docs/en/env-vars),
  reviewed 2026-07-16: `CLAUDE_CONFIG_DIR` overrides settings, history,
  plugins, and Linux/Windows credentials and is documented as useful for
  running multiple accounts side by side; macOS credentials remain in
  Keychain.
- [Claude Code status line](https://code.claude.com/docs/en/statusline), reviewed
  2026-07-16: optional `rate_limits.five_hour` and `rate_limits.seven_day`
  percentage/reset fields appear for Claude.ai subscribers after the first API
  response; the local status-line command itself does not consume API tokens.
- [Use the Claude Agent SDK with your Claude plan](https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan), reviewed 2026-07-16: the
  announced June 15 change is paused, third-party/`claude -p` usage still draws
  from subscription limits, and the proposed monthly credit is unavailable.
- [Manage API key environment variables in Claude Code](https://support.claude.com/en/articles/12304248-manage-api-key-environment-variables-in-claude-code), reviewed 2026-07-16:
  API-key environment variables take precedence over an authenticated
  subscription and can charge a different Console account.

## Environment and sanitized checks

| Target | Claude Code | Binary SHA-256 | Default auth | Empty `CLAUDE_CONFIG_DIR` inherited auth | Overrides |
|---|---|---|---|---|---|
| macOS 26.5.2 arm64 | `2.1.207` | `1397a062c6889675055e3314dd956376ac51262a7734ad9e819c26975d71547a` | logged in | no | no API key, OAuth token, or cloud-provider override |
| Linux 5.4.0-148-generic x86_64 | `2.1.132` | `623086f65cfd9c3aff0c8a5125087f8aea3100aa92bf3f0533b2bea5e5d69e8d` | logged in | no | no API key, OAuth token, or cloud-provider override |

Both current `auth status --json` payloads retained the same seven-key contract
previously recorded: `apiProvider`, `authMethod`, `email`, `loggedIn`, `orgId`,
`orgName`, and `subscriptionType`. Only key names and booleans were retained;
no value identifying an account, person, organization, plan, or credential was
persisted.

The remote SSH service again warned that its negotiated connection lacked a
post-quantum key exchange. That is a deployment hardening item and does not
change the Provider policy result.

## Reproduction commands

These commands read only version, schema shape, login boolean, override
presence, and empty-profile behavior. Do not print the raw JSON in evidence.

```bash
claude --version
sha256sum <resolved-claude-binary>
claude auth status --json | <sanitizer-that-emits-keys-and-booleans-only>
CLAUDE_CONFIG_DIR=<new-private-empty-directory> \
  claude auth status --json >/dev/null
```

Policy reproduction uses the dated official links above. No network endpoint,
browser cookie, OAuth token, API key, credential file, Keychain item, raw auth
JSON value, transcript, status-line payload, or Usage percentage is copied.

## Technical result retained from prior evidence

The accepted ADR 0016 evidence already established that clean config roots do
not inherit the default login, macOS credential slots can be scoped by
`CLAUDE_CONFIG_DIR`, a same-account Profile login/logout does not log out the
default Profile, and Linux/Windows credentials belong under the selected
config root. Current documentation continues to describe the side-by-side
account use case.

That mechanism evidence does not prove two distinct identities, a long-running
PTY, or stable third-party subscription use. The current Spike deliberately
does not spend subscription quota to generate optional 5h/7d status-line data
after the policy gate has already failed.

## Product decision candidate and fallback

- Do not ship stable MultiAgentDesk-managed Claude subscription accounts,
  subscription OAuth enrollment, or a subscription quota dashboard without an
  explicit Anthropic integration approval that defines the allowed identity,
  traffic, billing, and Usage surfaces.
- Do not proxy, impersonate, reuse, export, pool, rotate, or route subscription
  credentials/traffic, and do not parse private endpoints or browser state.
- A user may continue to run the official Claude Code CLI directly with their
  own subscription. MultiAgentDesk may document that external fallback without
  claiming to manage it.
- For a stable product integration, use a user-supplied Claude Console API key
  or supported cloud provider with explicit billing-source confirmation,
  isolated non-secret Profile settings, and Provider-native usage/cost data.
- Keep the generic P1 Account/Profile/Usage contracts, but mark subscription
  windows unavailable for the stable API-key/cloud path unless that provider
  exposes an official corresponding contract.

## Limitations

- No two-distinct-identity login was executed after the policy gate became
  decisive; technical distinct-account behavior remains unclaimed.
- No real request was issued, so no 5h/7d values or actual-account binding was
  collected. This avoids a hidden quota-consuming probe.
- Policy pages can change. Any future attempt to reopen subscription support
  requires a fresh official-policy review and, where appropriate, written
  Provider approval before technical account testing resumes.
