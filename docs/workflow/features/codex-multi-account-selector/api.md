# API: Codex explicit multi-account selector

All requests use authenticated local IPC, protocol major compatibility,
request-bound idempotency for mutations, and existing capability checks.

## Preview

Method: `sessions.preview`
Capability: `metadata.read` in this phase; future fine-grained
`profiles.read + usage.read` migration remains separate.

```json
{
  "provider": "codex",
  "profile_selector": "@A",
  "workspace_id": "workspace_<opaque>"
}
```

Response fields are bounded safe metadata:

```json
{
  "schema_version": 1,
  "preview_id": "preview_<opaque>",
  "expires_at": "RFC3339",
  "provider": "codex",
  "account_id": "account_<opaque>",
  "account_revision": 2,
  "account_label": "Work",
  "runtime_profile_id": "profile_<opaque>",
  "profile_revision": 3,
  "profile_alias": "A",
  "profile_label": "Work Linux",
  "credential_instance_id": "credential_<opaque>",
  "credential_revision": 4,
  "auth_status": "healthy",
  "device_id": "device_<opaque>",
  "workspace_id": "workspace_<opaque>",
  "provider_version": "0.144.2",
  "compatibility_status": "supported",
  "capability_snapshot": ["provider.usage.read", "session.control"],
  "usage_snapshot_id": "usage_<opaque>",
  "usage_observed_at": "RFC3339",
  "usage_stale": false
}
```

Labels are internal operator-authored metadata. No upstream identity or auth
payload is returned.

## Confirmed Session start

Method: `session.start`
Capability: `session.start`

```json
{
  "provider": "codex",
  "profile_selector": "@A",
  "workspace_id": "workspace_<opaque>",
  "preview_id": "preview_<opaque>",
  "confirmation": {
    "confirmed": true,
    "account_id": "account_<opaque>",
    "account_revision": 2,
    "credential_instance_id": "credential_<opaque>",
    "credential_revision": 4,
    "runtime_profile_id": "profile_<opaque>",
    "profile_revision": 3,
    "device_id": "device_<opaque>",
    "workspace_id": "workspace_<opaque>",
    "usage_snapshot_id": "usage_<opaque-or-empty>",
    "provider_version": "0.144.2"
  }
}
```

Every field is revalidated. The persisted Session contains immutable IDs and
the existing capability snapshot, never the selector string or display labels.

## Enrollment lifecycle

### `auth.begin`

Accept either `profile_selector` or the existing internal Profile/Credential
IDs, never conflicting forms. Public clients use the selector. It creates one
10-minute owner-bound enrollment and returns the existing official binary/
staging launch contract to the local CLI only.

### `auth.complete`

After official login and exact app-server validation, transition to
`awaiting_confirmation` and return:

```json
{
  "enrollment_id": "enrollment_<opaque>",
  "state": "awaiting_confirmation",
  "profile_selector": "@A",
  "account_id": "account_<opaque>",
  "runtime_profile_id": "profile_<opaque>",
  "credential_id": "credential_<opaque>",
  "confirmation_expires_at": "RFC3339"
}
```

No Provider identity value is included. The CLI asks the operator to type the
shown canonical alias.

### `auth.confirm`

Capability: `provider.auth`. Idempotent mutation.

```json
{
  "enrollment_id": "enrollment_<opaque>",
  "profile_selector": "@A",
  "confirmed": true
}
```

The authenticated client must own the enrollment. Store rechecks expiry,
state, Profile/Account/Credential linkage, alias and revisions, binary
fingerprint, validated staging, and confirmation alias digest before sealing
the Vault item. Success returns Credential ID/revision and safe state only.

### `auth.status` and `auth.logout`

Public clients accept exactly one `profile_selector`; the daemon resolves the
CredentialInstance. Internal opaque-ID forms remain for migration/tests but
cannot conflict with a selector. Logout retains active-Session denial and
revocation reservation.

## Stable errors

```text
alias_invalid
alias_conflict
account_disabled
profile_disabled
profile_binding_changed
identity_confirmation_required
identity_confirmation_mismatch
confirmation_expired
login_in_progress
login_required
auth_health_unknown
provider_version_unsupported
provider_capability_unavailable
provider_platform_unsupported
schema_compatible_identity_acceptance_pending
usage_stale
vault_locked
credential_quarantined
credential_revision_conflict
credential_writer_conflict
active_sessions
```

Raw Provider messages and identity/auth fields never appear in these errors.

## Ordering and idempotency

- Preview IDs are random server-issued records. The stored row binds client,
  tuple/revisions, compatibility fingerprints, latest selected Usage snapshot,
  expiry, and workspace. Client-computable digests are never accepted.
- Start performs Provider preflight before the transaction, then atomically
  validates/consumes the preview and inserts the starting Session. Same-request
  lost-response replay returns the recorded Session; another request/client or
  a forged/expired/consumed preview fails.
- Repeated `auth.confirm` with the same request returns the committed revision;
  a different selector/body returns conflict.
- Start confirmation is single-use only at the Session insertion boundary;
  replay returns the same Session through the existing idempotency record or a
  safe conflict, never a second Provider start.
- Login confirmation and Session confirmation are distinct; neither implies
  the other.
- Alias/profile/credential mutation, Vault revision change, Usage snapshot
  selection change when explicitly bound, or compatibility drift invalidates
  the preview.
- Compatibility drift before preview consumption creates no Session. Drift
  detected by the final runtime fingerprint check after reservation transitions
  the recorded Session to `failed` and performs no credential commit.

## Legacy raw-ID compatibility

There is no public or debug IPC bypass. `session.start` for Provider `codex`
requires `profile_selector`, a valid daemon-issued `preview_id`, and the full
confirmation on every request. The old raw-ID-only CLI/RPC form returns
`identity_confirmation_required` before Session insert. Phase 2 live and
integration harnesses seed a public alias and use preview/confirmation. Direct
runtime-manager unit tests are not an application authorization surface.

## CLI

```text
multidesk sessions preview --profile @A --workspace <workspace-id> [--json]
multidesk run codex --profile @A --workspace <workspace-id>
multidesk auth begin --profile @A
multidesk auth confirm --profile @A --enrollment-id <id>
multidesk auth status --profile @A [--json]
multidesk auth logout --profile @A [--json]
```

Human `run` performs preview, renders the confirmation, and requires explicit
input. JSON mode separates preview/start and requires the full confirmation
object; it never auto-confirms.
