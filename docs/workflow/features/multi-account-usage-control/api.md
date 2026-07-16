# Contracts: 多账号用量看板与显式调用

## Public interfaces

### CLI

```text
multidesk accounts add --provider codex|claude --name <display> --alias <alias> [--json]
multidesk accounts list [--provider <provider>] [--limit <1..200>] [--cursor <cursor>] [--json]
multidesk accounts show <account-or-@alias> [--json]
multidesk accounts update <account-or-@alias> [--name <display>] [--subscription-hint <hint>] --if-revision <n> [--json]
multidesk accounts disable|enable <account-or-@alias> --if-revision <n> [--json]
multidesk accounts delete <account-or-@alias> --if-revision <n> --confirm [--json]

multidesk profiles list [--account <id>] [--limit <1..200>] [--cursor <cursor>] [--json]
multidesk profiles create --account <id> --name <display> --alias <alias> [--json]
multidesk profiles show|validate <@alias> [--json]
multidesk profiles update <@alias> [--name <display>] [--alias <alias>] --if-revision <n> [--json]
multidesk profiles disable|enable <@alias> --if-revision <n> [--json]
multidesk profiles delete <@alias> --if-revision <n> --confirm [--json]

multidesk login <@alias> [--browser-profile <operator-label>] [--json]
multidesk logout <@alias> [--json]
multidesk usage [--profile <@alias>] [--refresh] [--json]

multidesk run --profile <@alias> --workspace <path> [--yes] [-- <provider args>]
multidesk shell <@alias>
```

P1 implements metadata CRUD, alias resolution and stored Usage reads. `login`,
real Provider `run`, live `usage --refresh` and `shell` return a capability-
specific unavailable error until P2/P3 enable the adapter. `profiles validate`
also returns `provider_capability_unavailable` in P1 and does not mutate state.

The CLI strips a single leading `@` for lookup. It never stores or evaluates a
shell alias. JSON output retains the Phase 1 envelope:

```json
{
  "schema_version": 1,
  "request_id": "...",
  "ok": true,
  "result": {}
}
```

### Local IPC application methods

```text
accounts.create
accounts.list
accounts.show
accounts.update
accounts.disable
accounts.enable
accounts.delete

profiles.create
profiles.list
profiles.show
profiles.update
profiles.validate
profiles.disable
profiles.enable
profiles.delete
profiles.resolveAlias

provider.login.begin
provider.login.status
provider.login.cancel
provider.logout
usage.list
usage.refresh

sessions.start     # extended with explicit resolved profile selector
```

The CLI and future Web/Desktop clients call these services; none may read the
Device SQLite database directly.

### Control Plane REST (P4)

```text
GET /v1/accounts?provider=&cursor=&limit=
GET /v1/accounts/{id}
GET /v1/profiles?account_id=&device_id=&cursor=&limit=
GET /v1/usage?account_id=&device_id=&cursor=&limit=
POST /v1/session-commands   # requires account/profile/credential snapshot
```

Provider login/logout and live usage refresh are Device commands, not Control
Plane credential operations. The Server routes an authorized asynchronous
command to the target Device and never receives plaintext credentials.

## Requests, events, and responses

### Account and Profile

```json
{
  "account": {
    "id": "account_<opaque>",
    "provider": "codex",
    "display_name": "Personal Codex",
    "subscription_hint": "plus",
    "enabled": true,
    "revision": 1,
    "created_at": "RFC3339",
    "updated_at": "RFC3339"
  },
  "profiles": [
    {
      "id": "profile_<opaque>",
      "account_id": "account_<opaque>",
      "device_id": "device_<opaque>",
      "name": "Personal Codex",
      "selector": "@A",
      "provider": "codex",
      "enabled": true,
      "revision": 1,
      "auth_status": "login_required",
      "availability": "unknown",
      "last_validated_at": null
    }
  ]
}
```

Normal responses omit `secretRef`, Provider Home paths, auth-file paths, raw
identity fields and raw Provider payloads.

In P1, `accounts.create` accepts only `codex|claude` and atomically creates the
Account plus one default RuntimeProfile on the local Device. The Profile has a
null CredentialInstance; therefore `auth_status=login_required`,
`availability=unknown`, and `last_validated_at=null` are derived response
values. The transaction creates no CredentialInstance, Vault item, Home,
Keychain item, directory or process. `fake` rows are internal migration state
and cannot be created, listed, mutated or alias-resolved through public IPC.

Update requests are explicit patches with `expected_revision`. Account update
may change only `display_name` and `subscription_hint`; Profile update may
change only `name` and `selector_alias`. Provider, Account ID, Device ID and
Credential binding are immutable in P1. A stale revision returns
`sync_conflict` without a write; success increments that entity's revision by
exactly one.

### Usage

```json
{
  "snapshot_id": "usage_<opaque>",
  "account_id": "account_<opaque>",
  "credential_instance_id": "credential_<opaque>",
  "device_id": "device_<opaque>",
  "provider": "codex",
  "provider_version": "0.144.2",
  "source": "official",
  "confidence": "high",
  "availability": "available",
  "observed_at": "RFC3339",
  "stale_at": "RFC3339",
  "windows": [
    {
      "provider_limit_id": "codex",
      "kind": "rolling",
      "label": "5 hours",
      "duration_seconds": 18000,
      "used_percent": 34,
      "remaining_percent": 66,
      "resets_at": "RFC3339"
    }
  ]
}
```

All numeric usage fields are optional. When the Provider omits a value the
field is absent/null and `source`/`confidence` remain truthful. The service
does not synthesize a limit from local token counts.

### Login lifecycle events

```text
provider.login.started
provider.login.browser_required
provider.login.verifying
provider.login.succeeded
provider.login.failed
provider.login.cancelled
provider.auth_status.updated
provider.usage.updated
provider.usage.unavailable
```

Event payloads contain opaque IDs, safe status codes, Provider/version and
timestamps only. Authorization URLs, device codes, email, org, Token, Cookie,
auth-file content and terminal capture are forbidden.

### Session start extension

The client may send `profile_selector` for convenience, but the daemon must
resolve and persist immutable IDs before starting the Provider:

```json
{
  "profile_selector": "@A",
  "workspace_id": "workspace_<opaque>",
  "confirmation": {
    "account_id": "account_<opaque>",
    "credential_instance_id": "credential_<opaque>",
    "runtime_profile_id": "profile_<opaque>",
    "usage_snapshot_id": "usage_<opaque-or-null>"
  }
}
```

If the current resolution differs from the confirmation tuple, return
`profile_binding_changed`; never silently use the new value.

## Error semantics

Stable safe codes:

```text
alias_invalid
alias_conflict
account_not_found
account_disabled
profile_not_found
profile_disabled
profile_binding_changed
profile_in_use
active_sessions
active_credential_grants
login_in_progress
login_required
login_cancelled
login_identity_mismatch
auth_health_unknown
provider_binary_missing
provider_version_unsupported
provider_capability_unavailable
provider_policy_blocked
provider_cleanup_required
usage_unavailable
usage_not_observed_yet
usage_stale
vault_locked
credential_quarantined
sync_conflict
```

`usage_unavailable` is not an authentication failure. `usage_stale` may return
the last snapshot alongside the warning. Provider raw errors are mapped to
safe codes and retained only in an explicitly exported redacted debug bundle.

## Authentication and authorization

New local capabilities:

```text
accounts.read
accounts.manage
profiles.read
profiles.manage
provider.login
provider.logout
usage.read
usage.refresh
```

Those fine-grained names are the P2+ target. To keep existing Phase 1 owner
identities usable, P1 maps public Account/Profile/Usage reads to the shipped
`metadata.read` capability and mutations to the shipped `client.admin`
capability. It does not silently rewrite persisted client capability lists.
Introducing the fine-grained capabilities requires its own identity migration
and compatibility review before Provider login is enabled.

`sessions.start` remains separately authorized. A login or usage refresh must
be authorized for the exact target Device/Profile, serialized by Profile lock
and audited. Remote Web/Desktop requests require an approved Device and the
Control Plane can only relay the command.

Provider credentials are never accepted in these JSON requests. Interactive
secrets stay in the official Provider process; supported non-interactive
secrets use stdin/restricted file descriptors/Vault materialization only.

## Idempotency, ordering, and replay

- All mutations require request-bound `Idempotency-Key`; the local CLI derives
  it from canonical method/body as in Phase 1.
- Account add with the same canonical alias and identical fields returns the
  existing transaction result; different fields return `alias_conflict`.
- Update/enable/disable/delete requires `expected_revision`; a successful
  mutation increments once, while a mismatch is `sync_conflict` and writes no
  audit/tombstone/state row.
- Login attempts have one active attempt per Profile. Retry after terminal
  failure creates a new attempt ID.
- Usage snapshots are append-only observations. `(accountId, deviceId,
  providerVersion, observedAt, rawReferenceHash)` deduplicates replay.
- Alias updates use `If-Match`/revision, replace the canonical key in one
  transaction and release the old key after commit.
- Session start pins the resolved tuple and rejects changes between preview and
  commit.
- Provider rate-limit update notifications merge only into the latest snapshot
  for the same CredentialInstance/version; they cannot clear omitted values.

## Versioning and compatibility

- Public envelope `schema_version` begins at 1.
- List endpoints use cursor pagination and bounded `limit` (default 50, max
  200); no account-count ceiling exists.
- Unknown Usage window kinds/Provider limit IDs are retained as `unknown` with
  the original safe label.
- Codex app-server methods are enabled only after exact schema/fixture checks.
- Claude auth/status-line parsing is enabled only for accepted version fixtures;
  unknown fields are ignored, missing required fields downgrade capability.
- P1 real Provider commands remain explicitly unavailable, not silently mapped
  to Fake.

Both local list methods return `{items, next_cursor}` and use ascending
`(created_at, id)` keyset order. `limit` defaults to 50 and is bounded to
1..200. The opaque base64url cursor encodes schema version, last created-at,
last ID and a digest of the immutable filters (`provider` for Accounts;
`account_id` for Profiles). Malformed cursors or reuse with different filters
returns `invalid_argument`; reaching the end returns an absent/null
`next_cursor`. Public lists always exclude `internal=true` rows. There is no
total Account/Profile count ceiling.

## Data retention and deletion

- P1 deletion is an all-or-nothing SQLite transaction. Profile delete requires
  the Profile disabled and no Session or active materialization reference.
  Account delete requires the Account disabled, checks all child rows, then
  deletes Usage and every public Profile before the Account. It is not necessary
  to disable each child after the Account itself is disabled.
- P1 public records own no CredentialInstance and P1 has no Provider Home
  registry. If an unexpected CredentialInstance is linked, P1 returns
  `provider_cleanup_required` rather than deleting it or partially succeeding.
- Account/Profile tombstones contain only opaque ID, provider, final revision
  and deletion time. Display names/aliases are removed and aliases are reusable
  after commit. Internal Fake rows are non-deletable.
- Usage snapshots default to 30 days with configurable per-account/global
  quotas; newest auth/availability state is retained separately.
- LoginAttempt state expires after 10 minutes unless the official flow defines
  a shorter timeout; terminal state may be retained as a safe audit event.
- Raw Provider responses, browser state, OAuth codes and terminal capture are
  never stored as product data.
- CredentialGrant does not exist and cannot be created in P1. The P5 migration
  that enables it must install the delete reference check before its create API;
  until then Grant deletion tests remain gated rather than falsely passing.
- P2/P3 deletion will add staged Provider logout/Home/Vault cleanup. Even then,
  local deletion does not claim Provider-wide revocation or remote erasure.
