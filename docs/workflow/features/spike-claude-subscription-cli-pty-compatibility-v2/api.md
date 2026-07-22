# Evidence contract: Claude Team-subscription PTY-only compatibility v2

This Spike changes no product API. This file freezes the durable evidence and
one-shot ledger contracts so a provider-spike runner and security reviewer can
agree on bounded, secret-free facts.

## Ledger

Planned path:

`docs/spikes/claude-subscription/2026-07-21-pty-v2-attempt-claimed.json`

Required mode: `0600`. Required atomicity: exclusive create before process
spawn, file fsync, directory fsync. Schema:

```json
{
  "schemaVersion": 1,
  "target": "spike-claude-subscription-cli-pty-compatibility-v2",
  "arm": "pty",
  "attemptClaimed": true,
  "claimedBeforeProcessSpawn": true,
  "modelBearingCliProcessUpperBound": 1,
  "printAllowed": false,
  "retryAllowed": false,
  "billingSource": "claude_ai_team_included_subscription",
  "usageCreditsDisabledOperatorDirected": true,
  "rawContentRetained": false
}
```

The ledger claims a model-bearing CLI process opportunity, not a Provider HTTP
request. Once present, no invocation under this Spike is allowed again.

## Sanitized evidence JSON

Planned path:

`docs/spikes/claude-subscription/2026-07-21-macos-team-cli-pty-v2.json`

Required top-level fields:

```text
schemaVersion
recordedAt
target
scope
result
platform
claude
auth
billing
controls
attempt
tty
pty
stateDiff
retention
claims
fallback
```

### Allowlisted values

- `platform`: system, OS version, architecture.
- `claude`: version, binary SHA-256, exact tuple revalidation booleans, flag
  surface boolean.
- `auth`: logged-in boolean and allowlisted auth/provider/subscription classes;
  override-presence booleans only.
- `billing`: Team included-subscription class,
  `usageCreditsDisabledOperatorDirected=true`, and
  `dollarBudgetFlagUsed=false`.
- `controls`: tools/MCP/slash/browser/history/session reuse disabled booleans,
  minimal-env boolean, raw-capture false booleans.
- `attempt`: ledger claim/mode, process spawned boolean, model-bearing process
  count `0|1`, print count `0`, retry false, Provider network count `null`.
- `tty`: local fixture fd booleans, same-slave boolean, resize fixture count,
  process-group cleanup primitive boolean.
- `pty`: classification, marker boolean, exit/timeout/output-cap/cleanup class,
  duration milliseconds, total byte count, resize count, positional-delivery
  class, blocker class.
- `stateDiff`: workspace unchanged, protected state unchanged, metadata-only
  allowlist satisfied, forbidden-category count, allowed-touch count, content
  retained false. No raw path or per-file digest.
- `retention`: Provider-side retention disclosure and local durable-capture
  booleans.
- `claims`: exact platform/version evidence boolean; stable managed support,
  Phase 3, Linux, Windows, Usage/billing/account capability all false.
- `fallback`: direct official CLI outside the managed surface; new lifecycle
  required for any further experiment.

## Forbidden durable fields

Evidence must not contain raw auth JSON, email, organization, user-provided
content, raw prompt/response capture, terminal text/bytes, ANSI transcript,
session/conversation ID, credential/token/cookie, Keychain material, local raw
path, file content, per-sensitive-file hash, request/response headers, Provider
endpoint, Provider HTTP request count, or dollar amount.

## Markdown report

The report mirrors the JSON classifications, links the ledger and runner, and
must contain:

- exact version/platform and sanitized auth/billing boundary;
- explicit zero-print/zero-retry statement;
- TTY fixture, PTY lifecycle, marker, resize, cleanup, and state-diff result;
- Provider retention disclosure;
- any limitation or ambiguity without raw evidence;
- deterministic fallback;
- the exact claim boundary: success is not stable MultiAgentDesk-managed Claude
  subscription support and does not satisfy Phase 3.
