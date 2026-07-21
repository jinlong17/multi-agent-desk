# Contracts: Claude API-key provider vertical slice

This document defines the review candidate for the internal Device IPC,
Provider, storage, and CLI contracts. It does not authorize implementation and
must be revised if `spike-claude-api-key-cli-compatibility` selects a narrower
exact-version mechanism.

## Public interfaces

The stable user surface remains local `multidesk` CLI plus authenticated Device
IPC. The feature does not add a Control Plane, Web, Desktop, external Provider
SDK, or direct Anthropic API.

Planned CLI surfaces:

```text
multidesk accounts create --provider claude --name <label>
multidesk profiles create --provider claude --account <id> --alias @<alias> ...
multidesk login claude --profile @<alias>
multidesk login claude --profile @<alias> --api-key-stdin
multidesk auth status --profile @<alias> [--json]
multidesk usage --provider claude --account <id> [--json]
multidesk sessions preview --provider claude --profile @<alias> --workspace <id> [--json]
multidesk run claude --profile @<alias> --workspace <id>
multidesk attach <session-id>
multidesk control acquire|release <session-id>
multidesk sessions stop|kill <session-id>
multidesk logout claude --profile @<alias>
```

Rules:

- `login claude` always means Console API-key enrollment in this feature. It
  never opens or reuses Claude.ai subscription login.
- Interactive login shows a non-secret enrollment preview, requires explicit
  confirmation, then reads the key with terminal echo disabled.
- `--api-key-stdin` reads the entire bounded key from stdin and is rejected when
  stdin is a terminal. No `--api-key <value>` option exists.
- JSON mode never accepts or echoes a key. Automation first confirms the
  non-secret enrollment through RPC, then submits the key on the dedicated
  secret channel.
- `run claude` is interactive and always displays the daemon-issued Session
  preview and Console/API billing warning before confirmation. JSON callers use
  `sessions.preview` plus confirmed `sessions.start` directly.

## Data contracts

### Credential and profile

```text
ClaudeCredentialBinding {
  credential_instance_id: ID
  account_id: ID
  device_id: ID
  provider: "claude"
  auth_method: "api_key"
  billing_source: "claude_console_api"
  secret_ref: opaque Vault reference
  status: unknown | healthy | expired | revoked
  health_class: unvalidated | healthy | invalid | permission_denied |
                billing_unavailable | rate_limited | network_unavailable |
                unknown
  credential_revision: positive integer
  secret_digest: SHA-256 digest used only for integrity/CAS
  last_validated_at?: RFC3339
}

ClaudeProfileSettingsV1 {
  schema_version: 1
  model?: allowlisted model identifier
  permission_mode?: exact Spike-approved mode
  environment_non_secret?: strict allowlisted map
  terminal?: { columns, rows, term }
  status_projection: "sanitized" | "disabled"
}
```

`ClaudeProfileSettingsV1` rejects fields whose names or values represent
credentials, auth tokens, API/base URLs, custom headers, cloud-provider routing,
credential helpers, or a `CLAUDE_CONFIG_DIR`. Provider credentials never enter
settings JSON.

The encrypted Vault payload is versioned and Provider-specific:

```text
ClaudeAPIKeySecretV1 {
  format_version: 1
  kind: "claude_console_api_key"
  api_key: secret bytes
}
```

Only the Vault manager and Claude materialization code may deserialize this
payload. Diagnostic formatting, JSON output, errors, audit events, and tests use
redacted metadata only.

### Provider compatibility

```text
ClaudeCapabilitySet {
  provider: "claude"
  binary_path: absolute path
  binary_digest: SHA-256
  version: exact string
  platform: exact OS
  architecture: exact architecture
  compatibility_fingerprint: SHA-256 over the accepted contract tuple
  capabilities: sorted set
  status: supported | identity_acceptance_pending | unsupported
}
```

Candidate capabilities are:

```text
auth.api_key
auth.status.redacted
session.pty
session.remote_attach
session.resize
session.stop
usage.session_estimated
```

No capability is enabled merely because the binary version parses. The Spike
and compatibility row must enable the exact version/platform tuple.

### Enrollment

```text
ClaudeAPIKeyEnrollment {
  id: ID
  client_id: ID
  account_id: ID
  account_revision: integer
  runtime_profile_id: ID
  profile_revision: integer
  device_id: ID
  auth_method: "api_key"
  billing_source: "claude_console_api"
  provider_version: string
  binary_fingerprint: SHA-256
  compatibility_fingerprint: SHA-256
  alias_digest: SHA-256
  state: awaiting_confirmation | awaiting_secret | succeeded |
         cancelled | expired | failed
  confirmed_by_client_id?: ID
  confirmed_at?: RFC3339
  submission_id_digest?: SHA-256
  credential_instance_id?: ID
  expires_at: RFC3339
  created_at: RFC3339
  updated_at: RFC3339
}
```

The record contains neither the key nor a digest of the generic secret-bearing
IPC body. `secret_digest` exists only inside the existing Vault/Credential CAS
contract and is never returned to ordinary clients.

### Session preview and confirmation

```text
ClaudeSessionPreviewV1 {
  schema_version: 1
  preview_id: ID
  provider: "claude"
  profile_alias: string
  account_id: ID
  account_label: string
  account_revision: integer
  runtime_profile_id: ID
  profile_revision: integer
  credential_instance_id: ID
  credential_revision: integer
  device_id: ID
  workspace_id: ID
  workspace_updated_at: RFC3339
  auth_method: "api_key"
  billing_source: "claude_console_api"
  provider_version: string
  binary_fingerprint: SHA-256
  compatibility_fingerprint: SHA-256
  capability_digest: SHA-256
  credential_health: redacted health class
  usage_snapshot_id?: ID
  usage_source?: "cli_derived" | "unavailable"
  usage_observed_at?: RFC3339
  created_at: RFC3339
  expires_at: RFC3339
}

ClaudeSessionConfirmationV1 {
  confirmed: true
  account_id: ID
  account_revision: integer
  runtime_profile_id: ID
  profile_revision: integer
  credential_instance_id: ID
  credential_revision: integer
  device_id: ID
  workspace_id: ID
  workspace_updated_at: RFC3339
  auth_method: "api_key"
  billing_source: "claude_console_api"
  provider_version: string
  binary_fingerprint: SHA-256
  compatibility_fingerprint: SHA-256
  capability_digest: SHA-256
  usage_snapshot_id?: ID
}
```

The preview never contains key material, Provider email/organization, raw auth
JSON, endpoint, transcript path, prompt, or Console balance. `credential_health
= unvalidated` is valid and must not be rewritten as healthy.

### Session-local metrics

The existing `UsageSnapshot` projection is extended with optional `session_id`,
`metric_kind`, and `unit`. Claude accepts only these initial metric rows:

| `metric_kind` | `unit` | Meaning | Source / confidence |
|---|---|---|---|
| `session_cost_estimate` | `usd` | client-estimated cost for this local CLI Session | `cli_derived` / `medium` |
| `context_input_tokens_current` | `tokens` | current context input tokens from the most recent API response | `cli_derived` / `medium` |
| `context_output_tokens_current` | `tokens` | current context output tokens from the most recent API response | `cli_derived` / `medium` |

All values are non-negative and optional. There is no `limit_value`,
`used_percent`, or `resets_at` for these metric kinds. Null/missing data is
represented by `capability_status=unavailable`, never numeric zero. The row is
bound to Provider, Account, CredentialInstance, Device, Session, CLI version,
observation time, source, confidence, and a canonical sanitized sample digest.

## Requests, events, and responses

### Enrollment methods

| Method | Capability | Request | Response / transition |
|---|---|---|---|
| `auth.begin` | `provider.auth` | Provider `claude`, exact profile selector, billing source, expiry | redacted enrollment in `awaiting_confirmation` |
| `auth.confirm` | `provider.auth` | enrollment ID, selector, `confirmed=true`, exact tuple | owner-bound `awaiting_secret` |
| `auth.api_key.submit` | `provider.auth` | enrollment ID, random submission ID, bounded secret bytes | Vault CAS + `succeeded`; response contains credential ID/revision and `unvalidated` only |
| `auth.cancel` | `provider.auth` | enrollment ID | terminal `cancelled`; clears transient state only |
| `auth.status` | `provider.metadata.read` | profile/credential selector | redacted credential/compatibility/health metadata |
| `auth.logout` | `provider.auth` | exact profile/credential revision and confirmation | local disable/delete result plus Console-revocation guidance |

`auth.api_key.submit` is a dedicated secret-bearing method. The request frame is
bounded to the reviewed key length plus protocol overhead, is never passed to
generic idempotency or audit-body serialization, and is zeroed best-effort after
Vault sealing. The key is not validated by a hidden network request.

### Provider methods

```text
Discover(ctx) -> ClaudeBinaryDescriptor
Probe(ctx, descriptor) -> ClaudeCapabilitySet
Describe(ctx) -> redacted ProviderDescriptor
ValidateProfile(ctx, profile) -> redacted validation result
BuildEnvironment(ctx, binding, secretHandle) -> non-serializable ChildEnvironment
StartPTY(ctx, reservedSession, childEnvironment) -> ClaudeProviderSession
ReadHealth(ctx, credential, providerResult?) -> ClaudeHealthProjection
ReadSessionMetrics(ctx, session) -> []UsageSnapshot
Stop(ctx, session, mode) -> ProviderExit
```

`ChildEnvironment` and the key-bearing Vault handle are non-serializable types.
`StartPTY` accepts a pre-reserved Session whose immutable tuple matches the
preview. There is no method that selects a default credential or scans the
ambient environment for auth.

### Session methods

| Method | Rule |
|---|---|
| `sessions.preview` | metadata read; resolves exactly one explicit Claude alias/profile and performs compatibility preflight without Vault access |
| `sessions.start` / `session.start` | `session.start`; consumes matching preview once, reserves Session, then materializes/spawns |
| `sessions.attach` / `sessions.detach` | existing authenticated Attachment semantics |
| `sessions.observe` | observer access to bounded PTY replay and structural state |
| `control.acquire/heartbeat/release` | existing revisioned ControllerLease semantics |
| `terminal.input` / `session.input` | current ControllerLease holder only; bounded byte stream |
| `terminal.resize` / `session.resize` | current holder only; exact positive dimensions; applies to PTY |
| `sessions.stop` / `session.stop` | graceful stop with bounded escalation |
| `sessions.kill` | explicit process-tree kill; terminal `killed` state |
| `sessions.resume` / `session.resume` | typed `provider_resume_unsupported` until exact live evidence approves official continuation; reconnect/replay is not Provider resume |

### Structural events

Allowed Provider-derived event kinds are initially:

```text
claude.session.started
claude.health.changed
claude.usage.observed
claude.usage.unavailable
claude.session.stopping
claude.session.exited
claude.session.failed
```

Event metadata contains local opaque IDs, exact version/fingerprint, redacted
status/error code, metric kind/source/freshness, exit classification, and
timing. It never contains terminal content, prompts, paths, Provider raw JSON,
email/organization, key material, request/response bodies, or raw hook data.

## Error semantics

Existing generic confirmation, lease, Vault, frame, deadline, and revision
errors remain authoritative. The Spike must confirm the final Claude-specific
mapping before implementation.

| Code | Meaning | Retry / mutation rule |
|---|---|---|
| `claude_api_key_invalid` | local key shape rejected before seal or request | no Vault mutation for a new key; replace retains old revision |
| `claude_api_key_rejected` | exact Provider auth failure class | selected credential becomes invalid; no alternate auth/retry |
| `claude_permission_denied` | Provider/policy permission rejection | terminal for request; no broader permission fallback |
| `claude_billing_unavailable` | billing/credit class prevents request | terminal for request; no subscription/cloud fallback |
| `claude_rate_limited` | documented rate-limit class | keep selected Account; optional documented retry metadata only |
| `claude_auth_source_conflict` | inherited/configured auth or routing conflicts with selected key | fail before Vault materialization or spawn |
| `provider_network_unavailable` | bounded DNS/TLS/network class | clean failure; explicit operator retry |
| `provider_version_unsupported` | exact CLI tuple absent or changed | diagnostics only; new Spike required |
| `provider_platform_unsupported` | live acceptance absent | fail before Vault access/spawn |
| `schema_compatible_identity_acceptance_pending` | macOS mechanism compatible but API-key identity/billing acceptance incomplete | fail before Vault access/spawn |
| `usage_unavailable` | accepted CLI exposes no safe metric | Session may continue; no zero/fabricated value |
| `usage_schema_changed` | status-line/helper contract drift | stop projection; Session may continue if auth/runtime remain safe |

Errors include only a stable code, safe human message, retry class, and local
opaque IDs. Provider stderr/body, endpoint, request ID, account identity, key
prefix/suffix, and environment are not returned or logged.

Authentication failure, billing failure, and rate limit are distinct. None
authorizes switching Account, key, Provider, subscription, endpoint, or cloud
route.

## Authentication and authorization

- All methods use the authenticated local Device protocol and current client
  identity. Secret entry is local-only.
- `provider.auth` is required for begin/confirm/submit/cancel/logout.
- `provider.metadata.read` is sufficient only for redacted auth status and
  compatibility diagnostics.
- `session.start` is required for preview consumption/start; metadata readers
  may request a preview but cannot consume it.
- Existing Attachment, ControllerLease, terminal, and Session-control
  capabilities govern attach/input/resize/stop/kill. An alias or preview is not
  authorization.
- The confirmed client ID owns the enrollment and preview. A different client
  cannot submit the key or consume the Session preview.
- Billing confirmation expires with the preview and is invalid after any
  Account/Profile/Credential/Workspace/binary/capability revision change.

## Idempotency, ordering, and replay

- `auth.begin`, `auth.confirm`, cancel/logout, and Session mutations use the
  existing request-bound idempotency ledger with non-secret bodies.
- `auth.api_key.submit` never stores or hashes its serialized request body in
  that ledger. It uses an operator-client-generated random submission ID,
  enrollment ownership, expected Vault revision, and the Vault secret digest to
  distinguish a same-secret replay from a conflicting replacement.
- Losing the submit response and replaying the same submission/key returns the
  committed redacted result. Reusing the submission ID with different secret
  bytes returns a conflict and preserves the committed Vault item.
- A Session preview is single-consumer. Same-request replay returns the original
  Session ID; different-request replay fails.
- Input/resize/stop use existing lease revision and idempotency rules. Duplicate
  input is acknowledged once; it is never sent twice to the PTY.
- Sanitized metric samples are deduplicated by
  `(session_id, metric_kind, canonical_sample_digest)`. Repeated samples cannot
  increase totals or create a different Account binding.
- Restart recovery expires incomplete enrollment/preview records, removes only
  Daemon-owned transient directories, and never replays a paid request or
  Provider mutation.

## Versioning and compatibility

- All new request/response objects carry `schema_version: 1` where not already
  covered by the Device protocol major.
- Profile settings reject unknown schema versions and unknown fields.
- Compatibility is an exact allowlist, not a semver range. The matrix records
  binary version/digest, platform/architecture, auth source, mechanism, evidence
  and fallback.
- Linux and macOS become supported independently only after live acceptance on
  their exact rows. Passing one platform does not enable the other.
- Windows and untested architectures return typed unsupported before secret
  materialization or process spawn even when the repository cross-builds.
- Status-line/hook field drift disables only the affected metric projection if
  the runtime/auth contract remains accepted; auth/environment/PTY drift
  disables Session start.
- Provider claims are updated in `docs/PROVIDER_COMPATIBILITY.md` only after the
  gated Spike decision and independent feature verification.

## Data retention and deletion

- The Vault retains only the encrypted key envelope and existing integrity/CAS
  metadata. SQLite never stores plaintext key, key prefix/suffix, or raw child
  environment.
- Account/Profile/enrollment/preview/Session records contain only the typed
  non-secret fields above. Expired enrollments/previews are removed under the
  existing bounded cleanup policy.
- Terminal content remains in the bounded in-memory ring buffer and official
  Provider runtime/history according to the selected CLI mode; it is not written
  to SQLite, audit events, or usage snapshots by this feature.
- Raw status-line/hook input is memory-only and discarded immediately after
  strict allowlist projection. Transcript and workspace paths are not retained
  in the metric row.
- Local logout/revoke deletes or disables the selected local Vault item and
  ephemeral materialization after Session policy permits. It does not delete
  the key from Claude Console or promise erasure from child memory, OS tooling,
  backups, or crash artifacts.
- Debug bundles and test artifacts run a secret/identity/path scanner and retain
  only synthetic IDs, exact versions/digests, stable result classes, and bounded
  timings.
