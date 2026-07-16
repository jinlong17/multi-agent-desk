# Design: Codex explicit multi-account selector

## Objective and boundary

Connect the verified P1 Account/Profile/Usage registry to the shipped Phase 2
Codex Vault/runtime for the exact accepted Linux CLI `0.144.2` arm. Product
launch becomes:

```text
resolve @alias -> preview immutable tuple -> explicit user confirmation
  -> transactionally revalidate tuple/revisions/capability -> Session start
```

The feature does not broaden Provider/platform support, create another auth
writer, automate account choice, or persist upstream identity PII. macOS
distinct-account and real Windows Codex remain typed unsupported capabilities.

## Existing authorities reused

- P1 `accounts`, `runtime_profiles`, `usage_snapshots`, alias resolution,
  revisions, pagination, and public/internal tuple isolation.
- Phase 2 `credential_instances`, Vault items, official enrollment,
  `CredentialMaterializationManager`, shared app-server runtime, Session/
  Approval/Usage/ControllerLease, and scoped revocation reservation.
- ADR 0014 exact-version schema, one writer per CredentialInstance, monotonic
  revision CAS, quarantine, and no auto-rotation.
- Local authenticated IPC and existing capability checks.

No new Provider Home abstraction is added. The canonical Home remains keyed by
CredentialInstance, and multiple Sessions for that Credential continue to
share the one app-server owner.

## State additions

### Enrollment confirmation

Add migration `0007_codex_identity_confirmation.sql` with an
`awaiting_confirmation` auth-enrollment state and safe confirmation fields:

```text
confirmation_account_id
confirmation_profile_id
confirmation_credential_id
confirmation_alias_digest
confirmed_by_client_id
confirmed_at
```

The alias digest is SHA-256 over the canonical internal selector plus
enrollment ID; it is not a Provider identity hash. No email, display name,
organization, JWT subject, raw claim, auth JSON, URL, code, or token is stored.

Official login and exact app-server validation occur in the private enrollment
staging Home. Instead of immediately sealing the credential, `auth.complete`
transitions the enrollment to `awaiting_confirmation` and returns only the
internal Account/Profile/Credential labels plus a typed
`identity_confirmation_required` result. The CLI asks the operator to type the
canonical alias shown in the preview. `auth.confirm` revalidates the
enrollment owner, expiry, exact tuple, alias/revisions, binary fingerprint, and
staged credential before the existing atomic Vault seal and staging cleanup.

JSON/non-interactive callers never receive an implicit yes path. They receive
the confirmation-required result and must perform the distinct confirm command.
Cancel/expiry/restart removes staging and does not install a credential.

This is an operator attestation that the official browser authorized the
intended internal Account. It is deliberately not an automated upstream
identity claim because the accepted app-server schema exposes no durable
non-PII identity key.

### Session preview

`sessions.preview` resolves a selector and workspace and returns a redacted
`SessionPreview`:

```text
preview_id / expires_at
provider / account_id / account_revision / account_label
runtime_profile_id / profile_revision / alias / profile_label
credential_instance_id / credential_revision / auth_status
device_id / workspace_id
provider_version / compatibility_status / capability_snapshot
usage_snapshot_id? / usage_observed_at? / usage_stale
```

`preview_id` is a digest of the canonical safe fields plus the authenticated
client ID and a short expiry. It is not an authorization token. Session start
still re-resolves the selector and validates every field/revision in the same
store transaction that reserves/creates the starting Session.

The request contains the selector, workspace, preview ID, and explicit
confirmation. Any drift returns `profile_binding_changed` before runtime
materialization. A disabled Account/Profile, revoked/unhealthy Credential,
Vault lock, unsupported version/platform, or ambiguous selector also fails
before a Session row or writer lock is created.

## CLI and TUI flow

```text
multidesk sessions preview --root <root> --profile @A \
  --workspace <workspace-id> --json

multidesk run codex --root <root> --profile @A \
  --workspace <workspace-id> --preview-id <id> --confirm

multidesk auth begin --root <root> --profile @A
# official login completes; CLI prints only internal @A labels
# operator types @A
multidesk auth confirm --root <root> --profile @A \
  --enrollment-id <id>

multidesk auth status|logout --root <root> --profile @A --json
```

Human mode requires the explicit typed alias. `--json` never prompts and never
accepts `--yes`. TUI uses the same preview/confirm RPCs and displays a blocking
confirmation panel. Shell aliases/display names are data, never evaluated
command fragments.

The legacy raw-ID `run codex` command becomes a compatibility-test surface
requiring an internal-only capability and explicit debug build/test plumbing;
normal public clients must use selector preview/confirmation. This prevents a
raw-ID product bypass while retaining deterministic Provider acceptance tests.

## Compatibility policy

- Linux `x86_64`, Codex CLI `0.144.2`: enabled after exact probe and accepted
  schema fingerprint.
- macOS: report `schema_compatible_identity_acceptance_pending`; do not launch
  the multi-account selector path.
- Windows: report `provider_platform_unsupported`; build/protocol tests remain
  non-live evidence.
- Unknown version/schema/platform: `provider_version_unsupported` or
  `provider_capability_unavailable`; never fall back to another Account or the
  default Home.

## Concurrency and failure handling

- Preview is read-only and bounded to 10 minutes; start performs all authority
  checks again.
- One active enrollment per Profile; the enrollment owner client and exact
  tuple are immutable.
- Logout transactionally reserves Credential revocation, blocks active or
  starting Sessions, removes only the selected Home/lock/Vault item, and is
  idempotent.
- Start cannot race logout because the revocation reservation rejects Session
  insertion. Login confirmation cannot race alias/profile mutation because
  revisions and canonical selector digest are rechecked.
- Daemon restart expires unconfirmed enrollment staging; it never guesses that
  browser success means operator confirmation.
- Provider crash/refresh ambiguity follows ADR 0014 quarantine/re-login.

## Audit and redaction

Audit records may contain opaque IDs, canonical alias digest, action, safe
result code, exact Provider version, and timestamps. They exclude display
labels, Provider identity, auth material, Usage values, browser/callback data,
terminal/model content, and raw errors. Generated dashboard state contains
workflow status only.

## Rollback

Disable the selector capability row for exact `0.144.2`. Existing Sessions
continue pinned to their tuple; no new selector starts are accepted. Migration
7 is forward-only and preserves enrollment history without credential content.
The deterministic fallback is the existing one selected managed Home operated
through reviewed direct maintenance/acceptance tooling, not automatic default
selection. No migration rollback deletes Vault items or Session history.
