# Design: Claude API-key provider vertical slice

## Decision snapshot

- **Owner:** `provider`; secondary impacts are `core`, `security`, `desktop`,
  and `project-system`.
- **Selected product boundary:** a built-in, exact-version-gated Claude Code
  CLI adapter using a user-supplied Claude Console API key. Managed Claude.ai
  subscription OAuth, setup-token, cloud-provider auth, gateways, and direct
  Anthropic API proxying remain outside this feature.
- **Credential boundary:** one `api_key` CredentialInstance belongs to exactly
  one local Account and Device. The key is sealed in the existing Device Vault
  and is never stored in Account/Profile settings, argv, generic idempotency
  records, logs, audit bodies, fixtures, dashboard state, or Git.
- **Secret-entry boundary:** enrollment confirms the non-secret Account,
  Profile, Device, auth source, and `claude_console_api` billing source before a
  dedicated secret-bearing IPC method reads the key from a hidden prompt or
  `--api-key-stdin`. The method bypasses generic request-body hashing, seals the
  key atomically, zeroes transient buffers best-effort, and returns only
  redacted metadata.
- **Runtime boundary:** the Device Daemon launches the official `claude` binary
  through the existing PTY/Session/Attachment/ControllerLease services. Each
  Session gets an empty Daemon-owned `CLAUDE_CONFIG_DIR`, a minimal reviewed
  child environment, and the selected key as the only enabled auth source.
- **Selection boundary:** `sessions.preview` and confirmed `sessions.start`
  bind Account/Profile/Credential/Device/Workspace revisions, `api_key` auth,
  Console billing source, binary/contract fingerprint, capability digest, and
  any usage observation. Stale, ambiguous, or mismatched input fails before
  materialization or process spawn.
- **Usage boundary:** only exact-version, allowlisted CLI/status-line facts may
  be projected. `cost.total_cost_usd` is labelled `cli_derived`, client-side,
  and estimated; context token fields describe the most recent response, not
  organization usage. Console spend, remaining balance, subscription 5h/7d
  windows, and monthly credit are unavailable.
- **Platform boundary:** stable support requires separate live acceptance on
  Linux amd64 and macOS arm64 for exact recorded CLI versions. Windows receives
  compile and existing ConPTY mechanism coverage only and returns typed
  unsupported before key materialization or Provider spawn.
- **First executable work:** `spike-claude-api-key-cli-compatibility`. No
  feature-build phase may start until its Provider Gate reaches
  `GATE_RESOLVED`, its Security Review is accepted, and this plan is
  independently approved.

## Evidence and frozen boundaries

The following are established inputs, not new support claims:

- ADR 0016 disables stable managed Claude subscription integration and requires
  a separate lifecycle for Console API-key or supported-cloud auth.
- The local planning host exposes Claude Code `2.1.207`; a redacted projection
  of `claude auth status --json` reports `claude.ai`/Team subscription auth.
  No API-key, OAuth-token, Bedrock, Vertex, or Foundry override variable is
  present. That subscription state is not API-key acceptance evidence.
- Anthropic documents that `ANTHROPIC_API_KEY` takes precedence over a logged-in
  subscription and is always used in non-interactive `-p` mode when present.
  Exact prompt, empty-config, Keychain/config, and PTY behavior remains a Spike
  assumption until reproduced on both target platforms.
- Anthropic documents `-p`, JSON/stream-JSON output, `--max-budget-usd`,
  `--max-turns`, and `--no-session-persistence`; these can bound a paid probe,
  but the Spike must show which options preserve the runtime behavior this
  product needs.
- Anthropic documents status-line JSON, including client-estimated session cost
  and current-context token counts. The raw JSON also includes paths and
  session/transcript identifiers, so only a minimal typed projection may cross
  the sanitizer boundary.

Official references are the current
[environment-variable contract](https://code.claude.com/docs/en/env-vars),
[CLI reference](https://code.claude.com/docs/en/cli-usage),
[status-line contract](https://code.claude.com/docs/en/statusline),
[hook contract](https://code.claude.com/docs/en/hooks), and
[error reference](https://code.claude.com/docs/en/errors), reviewed on
2026-07-20.

## Rejected alternatives

- Reusing the currently logged-in Claude.ai Team subscription: rejected by ADR
  0016 and the prior Provider/Security decision.
- Copying Keychain/config credentials, using `CLAUDE_CODE_OAUTH_TOKEN`, or
  setup-token: rejected; their issuance, storage, revocation, and product-policy
  boundaries are not approved.
- Putting the API key in argv, shell history, Profile JSON, settings JSON, a
  temporary plaintext file, or dashboard/manual state: rejected because those
  surfaces are durable or broadly observable.
- Inheriting the parent environment and merely overwriting
  `ANTHROPIC_API_KEY`: rejected because OAuth, custom endpoint, cloud-provider,
  proxy, helper, and model variables can silently change identity or billing.
- Calling the Anthropic Messages API directly: rejected because this feature is
  the official Claude Code CLI/PTY vertical slice, not an API proxy or a second
  Provider runtime.
- Treating `--bare` as the stable launch mode without evidence: rejected until
  the Spike proves whether its disabled settings/hooks/session behavior is
  compatible with the required interactive PTY and usage projection.
- Parsing human terminal text or undocumented files for health/usage: rejected;
  schema drift must become typed unavailable/unsupported state.
- Using status-line cost as Console billing or remaining limit: rejected; it is
  a local estimate and may differ from the actual bill.

## Components and ownership

### 1. Provider compatibility gate (`provider`, Security Gate open)

The prerequisite Spike must pin the binary path, version, digest, platform,
architecture, API-key precedence, empty-config isolation, minimal environment,
auth/health projection, PTY semantics, status-line/hook schema, cost semantics,
cleanup, and exact error taxonomy. It uses a dedicated test API key supplied
through a non-recorded channel and runs an ordinary paid request only after the
operator explicitly authorizes the bounded spend.

Accepted evidence is recorded under `docs/spikes/claude-api-key/`, then passes
independent `security-review`. `feature-plan` records the decision in a new
Claude API-key ADR and `docs/PROVIDER_COMPATIBILITY.md`. A failed assumption
selects one of the deterministic fallbacks below; it never silently broadens
auth or platform scope.

### 2. Domain, migration, and storage foundation (`provider` with `core` impact)

A forward-only `0008_claude_api_key_provider.sql` phase is planned after gate
resolution. It must preserve every existing Fake and Codex row while adding:

- `claude` to the public Provider allowlists used by Account, RuntimeProfile,
  CredentialInstance, Session, and UsageSnapshot storage;
- `api_key` to the auth-method allowlist;
- a typed `billing_source = claude_console_api` binding on Claude API-key
  credentials and preview records;
- provider-aware enrollment states, including confirmation-before-secret and
  one-time secret submission, without storing key bytes or a generic body hash;
- a provider-neutral compatibility fingerprint for Session previews while
  preserving the existing Codex schema fingerprint semantics; and
- optional `session_id`, `metric_kind`, and `unit` fields for unambiguous,
  Account/Profile/Credential-bound session metrics.

Migration tests cover empty databases, upgrade from schema 7 with active and
terminal Codex records, interrupted rebuild rollback, restart, invalid rows,
future-schema refusal, and the old-binary refusal contract. No destructive down
migration is introduced.

### 3. Secret-safe API-key enrollment (`provider` with `security` impact)

Enrollment is deliberately split so a secret is never retained while waiting
for user confirmation:

1. `auth.begin` resolves exactly one public Claude Account/Profile on the local
   Device and creates an expiring, owner-bound enrollment in
   `awaiting_confirmation` with `auth_method=api_key` and
   `billing_source=claude_console_api`.
2. `auth.confirm` checks the exact Account/Profile/Device revisions, binary
   compatibility, selected alias, billing source, and client identity, then
   moves the enrollment to `awaiting_secret`.
3. `auth.api_key.submit` reads a bounded key through authenticated local IPC,
   validates only reviewed local shape constraints, seals it directly into the
   Vault with expected-revision CAS, binds the CredentialInstance, and removes
   transient state. It does not make a hidden Provider request.
4. The credential begins as `unknown/unvalidated`. Only an explicitly
   authorized ordinary Session request can change health based on a real
   Provider result.

The CLI uses a no-echo TTY prompt by default and `--api-key-stdin` only when
stdin is not a terminal. It refuses a key in a positional argument, option
value, environment-backed Profile setting, or JSON output mode. Secret-bearing
requests are excluded from the generic idempotency/audit body ledger; a random
submission ID plus Vault revision/digest gives replay safety without persisting
the request body.

Local revoke/logout disables new starts, waits for or explicitly stops affected
Sessions under the reviewed policy, removes the selected Vault item and
ephemeral runtime state, and tells the operator to revoke the key in Claude
Console. It never claims remote deletion.

### 4. Claude binary and child-environment contract (`provider`)

`internal/providers/claude` owns discovery, version parsing, compatibility,
process launch, status projection, PTY events, and Provider error reduction.
The absolute binary path and digest are pinned in each preview. Unknown or
changed versions fail before secret materialization.

The child environment is constructed from a reviewed allowlist instead of the
daemon's inherited environment. Platform necessities such as locale, terminal,
and temporary-directory variables are frozen by the Spike. Before adding the
selected key, the builder removes all inherited Anthropic auth/token/base-URL/
custom-header variables, Claude OAuth/helper/config overrides, Bedrock/Vertex/
Foundry selectors, and relevant AWS/GCP/Azure credential-routing variables.
It then sets:

- one empty Daemon-owned `CLAUDE_CONFIG_DIR` with restrictive permissions;
- the selected Vault key as the child-only `ANTHROPIC_API_KEY`;
- the exact workspace as the process working directory; and
- only the Spike-approved model, permission, locale, terminal, temporary, and
  sanitizer transport settings.

The environment is never serialized for diagnostics. Same-user/root process
inspection, Claude Code itself, crash tooling, and compromised dependencies can
still observe the child environment; this residual host risk must be accepted
by Security Review and documented.

### 5. Preview, confirmation, and Session binding (`provider` with `core` impact)

The existing `sessions.preview`/confirmed `sessions.start` pattern becomes
provider-neutral. A Claude preview contains only non-secret fields and expires
quickly. It binds:

```text
provider = claude
account_id + account_revision
runtime_profile_id + profile_revision + public selector alias
credential_instance_id + credential_revision
device_id
workspace_id + workspace_updated_at
auth_method = api_key
billing_source = claude_console_api
provider_version + binary_fingerprint + compatibility_fingerprint
capability_digest
usage_snapshot_id? + usage_freshness?
client_id + created_at + expires_at
```

The confirmation echoes the exact tuple and `confirmed=true`. Consumption is
transactional and one-time; a different client, stale revision, changed binary,
different billing source, expired preview, or replay with a different request
digest fails without creating a Session or opening the Vault.

The Session record freezes the same Account/Credential/Profile/Workspace and
capability snapshot. A running Session never rotates keys or Accounts. A later
credential revision requires a new preview and a new Session.

### 6. PTY runtime and lifecycle (`provider` with `core` impact)

The Claude runtime uses the existing runtime manager, ring buffer,
SessionAttachment, and ControllerLease rather than creating provider-specific
control paths. For each accepted Session it must support:

- official Claude Code interactive PTY start with exact terminal dimensions;
- one output reader that continuously drains the PTY into bounded replay;
- observer attach/detach without stopping the Provider;
- input and resize only for the current ControllerLease holder and revision;
- reconnect with ring-buffer replay and explicit `truncated` state;
- graceful stop, bounded timeout, process-tree kill escalation, and terminal
  Session state; and
- crash/restart cleanup of orphan process/config state without reopening or
  exporting the Vault key.

Provider output remains terminal data and is not written to SQLite or normal
logs. Session-local structural events contain only allowlisted types and opaque
local IDs.

### 7. Health and usage projection (`provider` with `security` impact)

Auth health is a redacted state machine, not identity discovery:

```text
unvalidated -> healthy | invalid | permission_denied | billing_unavailable
            | rate_limited | network_unavailable | unknown
```

The Account label is operator-managed. Email, organization, raw auth JSON,
provider response bodies, transcript/session paths, prompts, and key-derived
identity are not retained.

If the accepted status-line/helper mechanism is available, a Daemon-owned
sanitizer consumes raw CLI JSON in memory and emits only:

- exact CLI version and local Session ID;
- client-estimated `session_cost_usd`;
- current-context input/output token counts from the most recent response; and
- observation time plus a canonical sample digest.

Each metric has an explicit kind/unit/source/confidence. Identical canonical
samples for one Session are replay-deduplicated; a repeat may refresh
observation metadata but cannot double-count. Missing, null, changed, or
unaccepted fields produce `usage_unavailable` or `usage_schema_changed` with no
zero value. No metric participates in automatic Account rotation.

### 8. Compatibility and documentation (`provider` / `project-system` impact)

Compatibility rows are exact tuples of CLI version, binary digest, OS,
architecture, auth source, PTY mechanism, and evidence. Passing compilation or
schema fixtures is not live Provider support. The intended feature exit is:

| Platform | Required evidence | Result before evidence |
|---|---|---|
| Linux amd64 | exact-version real API-key PTY, control/replay/stop, paid ordinary-work health and sanitized metrics | `provider_platform_unsupported` |
| macOS arm64 | same exact live acceptance plus empty-config/Keychain non-crossover | `schema_compatible_identity_acceptance_pending` |
| Windows amd64 | build, contract, and existing ConPTY mechanism tests only | `provider_platform_unsupported` |
| Other versions/architectures | fresh Spike and review | `provider_version_unsupported` or `provider_platform_unsupported` |

Windows stable support, cloud providers, remote Credential Grant, Control Plane,
Web/Desktop UI, and organization billing APIs require separate feature units.

## Data flow and state transitions

### Enrollment

```text
begin
  -> awaiting_confirmation
  -> confirmed / awaiting_secret
  -> secret submitted + Vault CAS
  -> succeeded (credential unvalidated)
```

Cancel, expiry, client mismatch, revision drift, binary drift, invalid local key
shape, Vault lock, or CAS conflict terminates the enrollment and clears only
transient state. Existing Vault data is never overwritten on ambiguity.

### Session start

```text
resolve @alias
  -> version/platform preflight
  -> daemon-issued preview
  -> exact user confirmation
  -> transactional preview consumption + Session reservation
  -> Vault open + clean environment build
  -> PTY spawn
  -> running
```

Any failure before spawn marks or removes only the reserved Session according
to the existing state contract and cleans the config directory. Any failure
after spawn drains output, terminates the process tree, records a redacted error
code, releases materialization, and reaches `failed` or `killed`.

### Health and usage

A locally sealed key stays `unvalidated`. The first explicitly authorized
ordinary request may produce a redacted health transition and sanitized
session-local metric. Authentication failure never triggers another
CredentialInstance, subscription fallback, cloud routing, or retry that can
incur an unconfirmed billing source.

## Failure taxonomy and deterministic fallback

| Condition | Stable result | Fallback |
|---|---|---|
| no dedicated key or no paid-request authority | Spike cannot run paid arm; feature build remains gated | operator supplies key through non-recorded channel and separately authorizes bounded ordinary work |
| API key missing/locally malformed | `claude_api_key_invalid` before spawn/request | re-enter key; preserve old Vault revision on failed replace |
| Provider rejects credentials | `claude_api_key_rejected` with redacted 401 class | stop; operator verifies/revokes in Console; never use subscription |
| permission/policy denial | `claude_permission_denied` | stop; no retry with broader permissions/auth |
| billing/credit unavailable | `claude_billing_unavailable` | stop; no subscription/cloud fallback |
| rate limit | `claude_rate_limited` | keep selected Account; surface retry metadata only if documented; no rotation |
| network/TLS/timeout | `provider_network_unavailable` / `deadline_exceeded` | clean stop and explicit retry by operator |
| inherited auth/cloud route detected | `claude_auth_source_conflict` before materialization/spawn | fix Profile/environment; no silent scrub if contract cannot prove safety |
| unknown CLI/schema/status field | `provider_version_unsupported` / `usage_schema_changed` | diagnostics only; fresh Spike required |
| unsupported platform | `provider_platform_unsupported` before Vault access | use an accepted target platform |
| preview or revision drift | existing confirmation/profile/credential conflict code | create a fresh preview and reconfirm |
| sanitizer/metric channel fails | `usage_unavailable`; Session may continue if auth/runtime are healthy | show unavailable with source/version/freshness |
| PTY launch/control failure | `provider_failed` or existing lease/control code | bounded cleanup; Fake Provider remains diagnostic-only |
| cleanup ambiguity | `credential_recovery_required` / quarantine | fail closed and require operator recovery |

The Spike must replace provisional Claude-specific code names with exact stable
names if official behavior requires a narrower classification.

## Security and privacy

- The API key is high-value plaintext only in the local prompt buffer,
  authenticated IPC buffer, Vault decrypt buffer, and selected child
  environment. These buffers are bounded and zeroed best-effort; language/OS
  copies and same-user/root visibility remain explicit residual risks.
- The Control Plane, Web, Desktop UI, dashboard, Profile JSON, audit body,
  terminal logs, debug bundle, crash report, fixture, test output, and Git must
  never receive the key.
- Billing confirmation is authorization, not presentation. It binds the exact
  Account/Credential/Profile/Workspace and is invalidated by any revision or
  binary change.
- Status-line/hook JSON is untrusted and potentially sensitive. It is bounded,
  strictly decoded, allowlisted, and discarded after typed projection.
- Workspace paths and transcript/session IDs remain local runtime data; they are
  never persisted as usage metadata.
- Local revoke prevents future MultiAgentDesk use but cannot erase a key copied
  by Claude Code, host tooling, backups, or an attacker. Provider-side revoke is
  an operator action in Claude Console.
- No automatic Account selection, key rotation, retry across credentials,
  request proxying, quota probe, or rate-limit evasion is permitted.

## Compatibility and migration

- Provider support is an allowlist keyed by exact binary and platform evidence;
  semver range inference is forbidden.
- The migration is forward-only and preserves Fake/Codex rows and all existing
  selector confirmations. A previous binary must refuse schema 8 rather than
  reinterpret Claude data.
- Profile settings use a versioned, strict, non-secret Claude settings schema.
  Unknown fields or credential-like keys fail validation.
- A Claude API-key CredentialInstance is never reinterpreted as interactive or
  subscription auth. Cloud auth requires a separate type and lifecycle.
- Live evidence and compatibility rows contain only sanitized versions,
  digests, result classes, timing bounds, and synthetic IDs.

## Rollback

1. Disable the Claude capability row and refuse new Claude previews/starts.
2. Gracefully stop active Claude Sessions, then use bounded process-tree kill if
   needed; release PTYs and remove Daemon-owned config directories.
3. Preserve encrypted Vault items and schema 8 for a fixed binary; never export
   or downgrade them to plaintext. The operator may explicitly delete a local
   credential and revoke it in Console.
4. Revert product code only through reviewed forward commits. Old binaries must
   refuse the newer schema; restore a verified pre-migration backup only as an
   explicit full-data rollback.
5. Restore the prior compatibility/documentation row and keep the failed
   evidence in the append-only logs.

Rollback never activates subscription OAuth, a different key, a cloud provider,
or an unsupported platform as a substitute.
