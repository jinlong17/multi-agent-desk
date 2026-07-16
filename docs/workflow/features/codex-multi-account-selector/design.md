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

Add migration `0007_codex_selector_confirmation.sql` with an
`awaiting_confirmation` auth-enrollment state, safe confirmation fields, and
the bounded Session preview table described below:

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

Migration 7 adds `session_start_previews`:

```text
id                              # random preview_<opaque>, primary key
client_id                       # authenticated issuer/consumer
provider                        # codex only in this feature
account_id / account_revision
runtime_profile_id / profile_revision
credential_instance_id / credential_revision
device_id / workspace_id
usage_snapshot_id?              # exact selected snapshot or null
provider_version
binary_fingerprint / schema_fingerprint / capability_digest
created_at / expires_at
consumed_at?
consumed_request_digest? / session_id?
```

`preview_id` is random and server-issued. The row, not a client-computable
digest, proves issuance and binds the authenticated client, tuple, revisions,
workspace, exact compatibility and ten-minute expiry. Preview rows contain
only opaque IDs, revisions, fingerprints and timestamps; internal display
labels are reconstructed for the response and are not duplicated in the row.

Session start performs an exact Provider preflight before opening the store
transaction, then calls one storage operation that validates the preview owner,
expiry, unconsumed state, tuple/revisions, revocation state and preflight
fingerprints; marks the preview consumed with the request digest/Session ID;
and inserts the starting Session. Lost-response replay with the same request
digest returns the same Session. Cross-client use, a forged/random ID, expiry,
or a second different request fails. Consumed/expired previews are retained for
bounded idempotency/audit time, then deleted without affecting Sessions.

The request contains the selector, workspace, preview ID, and explicit
confirmation. Any database or preflight drift returns
`profile_binding_changed` before the preview transaction, Session row, or
runtime materialization. A disabled Account/Profile, revoked/unhealthy
Credential, Vault lock, unsupported version/platform, or ambiguous selector
also fails before preview consumption.

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

There is one daemon start contract: every Codex Session start, including live
acceptance tests, requires a daemon-issued preview and confirmation. Opaque IDs
remain fields inside the preview/confirmation; they are not an alternate start
path. The legacy CLI raw-ID-only invocation is rejected with
`identity_confirmation_required` before Session insert. The Phase 2 acceptance
harness seeds a public Account/Profile alias and obtains a preview instead of
using a debug capability. Internal runtime unit tests may call the runtime
manager directly, but no IPC/CLI identity can bypass preview consumption.

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

- Preview creation is a bounded metadata write with ten-minute expiry; start
  performs all authority checks again and consumes it once transactionally.
- Daemon restart preserves unexpired preview rows; cleanup expires them without
  authorizing a start. Two requests racing one preview yield one Session, while
  idempotent replay of the winning request returns that Session.
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

### External compatibility recheck

Preview preflight records the exact binary fingerprint, canonical schema
fingerprint and capability digest. Start preflights again before the preview
transaction. Drift detected there returns a typed compatibility error with no
Session. After the transaction reserves a Session, `Runtime.StartReserved`
rechecks the same fingerprint immediately before materialization/spawn. Drift
or process/filesystem failure after reservation transitions that Session to
`failed`, consumes no Provider mutation, releases any acquired materialization,
and never pretends that cross-system work was atomic. Tests distinguish
"pre-reservation: no Session" from "post-reservation: failed Session."

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
