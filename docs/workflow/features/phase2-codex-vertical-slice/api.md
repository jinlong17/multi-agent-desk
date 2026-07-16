# Contracts: Codex Vertical Slice

This is the revised v0.5 internal contract for the `provider` module. It is not
a public Control Plane API and does not authorize implementation before
independent review. Existing P0-P2A names reflect the current Go packages;
P2B/P3A names remain reviewable semantic contracts.

## Device record and migration contract

P0's verified `0004_codex_foundation.sql` preserves all existing Fake rows and
expands the explicit Provider allowlist to `fake` and `codex`. SQLite table
rebuilds remain transactional; an interrupted or unknown-future migration
refuses to open the database rather than partially rewriting a row. P2B must
add forward-only `0005_codex_vault_and_approval_dispatch.sql`, not rewrite
`0004`, for Vault v1, enrollment, and Approval dispatch-state fields.

The Codex-capable records are:

```text
Account {
  id
  provider: "codex"
  display_name                 // operator metadata, not raw Provider identity
  provider_subject_digest?     // one-way digest only
  created_at
  updated_at
}

CredentialInstance {
  id
  account_id?                  // required for codex; nullable for legacy fake rows
  device_id
  provider: "fake" | "codex"
  auth_method: "fake" | "interactive" | "device_code"
  secret_ref                   // Vault reference; never token material
  status
  credential_revision
  secret_digest
}

RuntimeProfile { id, device_id, provider: "fake" | "codex", account_id?, settings_json, ... }
Session { id, account_id?, credential_instance_id, runtime_profile_id,
          provider: "fake" | "codex", provider_session_id?, ... }

Approval { id, session_id, provider_approval_id, kind, payload_digest,
           status, response_state, requested_decision?,
           responded_by_device_id?, idempotency_key, requested_at,
           dispatch_started_at?, responded_at?, dispatch_error_code? }

UsageSnapshot { id, account_id, device_id, source, confidence, window_kind,
                used_value?, limit_value?, used_percent?, resets_at?,
                observed_at, raw_reference_hash? }

VaultConfigV1 { singleton_id, format_version, kdf_name, kdf_salt,
                argon_time, argon_memory_kib, argon_parallelism,
                key_check_nonce, key_check_ciphertext, initialized_at,
                initialized_by_device_id, init_request_digest,
                created_at, updated_at }

VaultItemV1 { credential_instance_id, account_id, device_id, provider,
              envelope_version, credential_revision, cipher_name,
              payload_nonce, payload_ciphertext, wrap_name, wrap_nonce,
              wrapped_dek, aad_digest, secret_digest, created_at, updated_at }

AuthEnrollment { id, client_device_id, runtime_profile_id,
                 credential_instance_id?, binary_fingerprint, staging_path,
                 state, idempotency_digest, expires_at, created_at, updated_at }
```

Approval payloads, auth files, raw email/subject, terminal text, and Provider
response bodies are not persisted. Approval and Usage metadata are bounded and
retained according to the Device policy; unknown/future fields fail closed.
Legacy Fake rows may retain `account_id = null` and revision `0`; Codex rows
require a linked Account and a positive `credential_revision`. `device_code`
remains a reserved persisted enum from P0 but no Phase 2 capability or
enrollment path may create it.

## Authenticated local IPC and CLI contract

The new methods use the existing Device protocol major version and are mapped
server-side to new capabilities. The aliases are deliberate: the provider
adapter protocol names the operation, while the local CLI may expose the
existing `sessions.*`/`terminal.*` vocabulary.

| Method | Capability | State/lease/idempotency rule |
|---|---|---|
| `vault.status`, `vault.initialize`, `vault.unlock`, `vault.lock` | `metadata.read` / `vault.control` | initialize is authenticated, local, one-time, and request-idempotent without hashing/storing the password/body; unlock/lock use existing secret-safe local transport |
| `accounts.list/show/create/disable`, `profiles.list/create/edit/delete`, `credentials.status` | `metadata.read` / `provider.metadata.read` | local metadata only; no secret or raw Provider identity; mutations are offline/request-bound |
| `provider.describe`, `provider.health`, `profile.validate` | `provider.metadata.read` | authenticated read; bounded diagnostics; no secrets |
| `auth.begin`, `auth.complete`, `auth.cancel`, `auth.status`, `auth.logout` | `provider.auth` | production Vault source required; official local interactive enrollment; no raw secret in request; owner-bound state and request-bound idempotency |
| `usage.read` | `provider.usage.read` | bounded persisted snapshot; source/freshness/confidence required |
| `session.start` / local `sessions.start` | `session.start` | explicit Account/Profile; Vault/materialization; idempotency |
| `session.input` / local `terminal.input` | `terminal.control` | current ControllerLease and revision; Codex maps to structured turn start/verified steer, not stdin |
| `session.resize` / local `terminal.resize` | `terminal.control` | current ControllerLease and revision; Codex conversation returns typed unsupported without Provider mutation |
| `session.stop` / local `sessions.stop` | `session.control` | current ControllerLease and revision |
| `session.resume` / local `sessions.resume` | `session.resume` | terminal source; creates a new Session only after verified Provider continuation; unsupported fails before mutation |
| `approval.list` / `approval.observe` | `approval.read` | attached observer; bounded replay; no mutation |
| `approval.respond` | `approval.respond` | current ControllerLease, provider Approval ID, matching revision, request-bound idempotency |

Approval response and Usage read are not provider-only side channels: they go
through the authenticated local IPC authorizer, Device Store, audit metadata,
and the same request/replay limits as Phase 1 operations. The thin CLI must
provide explicit `run codex`, `usage --provider codex`, and
`approvals list|respond` surfaces or their documented equivalent; `run fake`
remains unchanged.

## Capability and version contract

```text
CodexCapabilitySet {
  provider: "codex"
  binary_path: string              // diagnostic path, never a secret
  version: string
  platform: string
  architecture: string
  schema_fingerprint: string
  methods: set<string>              // exact, fixture-backed allowlist
  experimental: set<string>
  status: supported | downgraded | unsupported
}
```

Rules:

- `initialize` and `initialized` are required before any capability call.
- A version is enabled only when its schema fingerprint and replay fixtures
  match a compatibility row. A missing or changed row returns
  `provider_version_unsupported`.
- The adapter exposes `account/read`, `account/rateLimits/read`, and
  `account/usage/read` only when each exact method is enabled. `refreshToken:
  true` is routed through the canonical writer only.
- Device-auth initiation is disabled for Phase 2 even if the binary advertises
  it; no capability marker or completed-login state is exposed until a later
  independently reviewed exact completion contract exists.

## Internal adapter operations

```text
Discover(ctx) -> BinaryDescriptor | ProviderError
Probe(ctx, BinaryDescriptor) -> CodexCapabilitySet | ProviderError
Describe(ctx) -> ProviderDescriptor | ProviderError
Health(ctx, AccountID?) -> ProviderHealth | ProviderError
BeginAuth(ctx, RuntimeProfileID, mode) -> AuthOperation | ProviderError
AuthStatus(ctx, CredentialInstanceID) -> AuthStatus | ProviderError
Logout(ctx, CredentialInstanceID) -> AuthResult | ProviderError
Materialize(ctx, CredentialInstanceID, RuntimeProfileID) -> MaterializedHome
Start(ctx, SessionStartRequest) -> ProviderSession
StartThread(ctx, ThreadStartRequest) -> ProviderThread
StartTurn(ctx, ProviderThreadID, TurnInput) -> ProviderTurn
SteerTurn(ctx, ProviderThreadID, ProviderTurnID, TurnInput) -> ProviderTurn
InterruptTurn(ctx, ProviderThreadID, ProviderTurnID) -> ProviderTurn
ReadAccount(ctx) -> AccountSnapshot
ReadRateLimits(ctx) -> RateLimitSnapshot
ReadUsage(ctx) -> UsageSnapshot
ListApprovals(ctx, SessionID) -> []ApprovalRequest
RespondApproval(ctx, ApprovalResponse) -> ApprovalResult
Stop(ctx, ProviderSessionID, mode) -> ProviderExit
Resume(ctx, SessionID) -> ProviderSession
```

`MaterializedHome` owns cleanup and contains no serialized secret in its
diagnostic form. The lease must be held for every operation that can mutate a
refreshable auth file. `ProviderSession` is pinned to the discovered binary,
schema fingerprint, Account/Profile, CredentialInstance, and materialization
revision.

`SteerTurn` is disabled unless the exact compatibility row and fixtures enable
`turn/steer`. No adapter operation maps a conversation resize to
`command/exec/resize`; command execution is a different Provider object and is
outside this slice.

## Session start request

```text
SessionStartRequest {
  device_id
  credential_instance_id
  runtime_profile_id
  workspace_id
  provider: "codex"
  resumed_from_session_id?
}
```

The request is accepted only after the daemon has an unlocked Vault, a valid
CredentialInstance/Profile, a verified materialization lease, and a compatible
binary. The caller cannot supply binary path/version, schema fingerprint,
Account identity, or capabilities. The daemon derives those fields from
discovery, Store linkage, the exact compatibility row, and its profile policy,
then persists the resulting immutable capability snapshot. Account selection
is explicit through the linked CredentialInstance and remains pinned.
The local Session becomes `running` only after app-server initialize and
`thread/start` return a bounded Provider thread ID. The shared
`CredentialRuntime` owns the child process, materialization handle, protocol
multiplexer, and final cleanup state; its `SessionBinding` owns the returned
thread ID and all per-Session turn/Approval/event state.

## Shared runtime ownership contract

```text
CredentialRuntime {
  credential_instance_id
  materialization_revision
  binary_descriptor
  capability_set
  child_process
  protocol_multiplexer
  materialization_handle
  bindings: map<SessionID, SessionBinding>
  ref_count
  finalization_state
}

SessionBinding {
  session_id
  account_id
  runtime_profile_id
  workspace_id
  provider_thread_id
  active_turn_id?
  pending_approval_request_ids
  next_event_sequence
  state
}
```

The runtime manager keeps one CredentialRuntime per CredentialInstance. A
second compatible Session creates another binding/thread and increments the
reference count. Stopping/killing a binding never terminates the shared child;
the last release performs process stop, final digest validation, Vault CAS, and
materialization release once. Child failure fails all bindings exactly once.

## Daemon-owned profile and Provider mapping

P3A accepts RuntimeProfile settings schema `codex.v1` only:

```text
{ model?: string<=128,
  approval_policy: "untrusted" | "on-request" | "never",
  sandbox: "read-only" | "workspace-write" }
```

Unknown fields and `danger-full-access` are rejected in this slice. The daemon
canonicalizes the Store-owned Workspace path and builds `thread/start` with
`cwd`, `approvalPolicy`, `sandbox`, optional `model`, and `ephemeral=false`.
It omits base/developer instructions, arbitrary config, service tier,
personality, model provider, and environment overrides. `turn/start` contains
only the bound thread ID and one bounded text input (maximum 64 KiB). Active-turn
input is rejected; `turn/steer` remains disabled until a separate exact fixture
and live gate. `turn/interrupt` is enabled only by an exact compatibility row
and fixture. Every event/request must match the binding's thread and active turn.

## Provider event envelope

```text
ProviderEvent {
  session_id
  provider: "codex"
  provider_version
  sequence
  kind: output | turn_started | turn_completed | approval_requested |
        approval_resolved | account | rate_limits | usage | warning | exit | error
  provider_item_id?
  body                         // schema-specific, bounded and redacted
  source: app_server | adapter
  observed_at
}
```

The adapter must not forward arbitrary app-server JSON. Each mapped body has a
bounded size and an allowlisted field set. Terminal/output data is subject to
the Phase 1 ring-buffer and truncation rules.

## Approval contract

```text
ApprovalRequest {
  id
  session_id
  provider_approval_id
  kind
  payload_digest
  summary: redacted string
  status: pending | approved | denied | expired | cancelled
  response_state: idle | dispatching | written | ambiguous
  requested_decision?: approve | deny | cancel
  requested_at
  responded_at?
  responded_by_device_id?
}

ApprovalResponse {
  session_id
  approval_id
  provider_approval_id
  lease_revision
  decision: approve | deny | cancel
  idempotency_key
}
```

The request is observable by attached clients and is persisted as bounded
metadata. A response requires the current ControllerLease, matching revision,
authenticated local IPC client, and an idempotency key. Repeating the same key
with the same digest returns the stored result; reusing it with a different
decision is a conflict. The Store first atomically claims
`pending/idle -> pending/dispatching`; only then may the live runtime write the
exact JSON-RPC server-request response. A successful write commits
`approved|denied|cancelled/written` according to the stored decision. A write
error, lost process/request ID, or restart
from `dispatching` commits `expired/ambiguous` and is never replayed. A stale
lease, terminal Approval, or unknown Provider request is rejected without
sending a Provider mutation.

Exact v0.144.2 Approval mapping:

| Server request | Local decisions | JSON-RPC result | Status |
|---|---|---|---|
| `item/commandExecution/requestApproval` | `approve`, `deny`, `cancel` | `{"decision":"accept"}`, `{"decision":"decline"}`, `{"decision":"cancel"}` | enabled only for exact 0.144.2 row/fixture |
| `item/fileChange/requestApproval` | `approve`, `deny`, `cancel` | same three exact decision strings | enabled only for exact 0.144.2 row/fixture |
| `item/permissions/requestApproval` | none | no response emitted | disabled; fail closed until granted-profile fixture/live evidence |

`acceptForSession`, exec-policy amendments, network-policy amendments, and
permission grants are disabled. The dispatch digest is SHA-256 over local
Session ID, Approval ID, Provider Approval ID, exact method, payload digest,
local decision, responder ID, lease revision, and idempotency key. The raw
JSON-RPC request ID remains only in the owning SessionBinding.

`0005` enforces these durable state constraints: `idle` requires pending status
and null requested decision/dispatch time; `dispatching` requires pending status,
a non-null decision, and dispatch time; `written` requires responded time and
the exact pair `(approve, approved)`, `(deny, denied)`, or
`(cancel, cancelled)`; `ambiguous` requires expired status, a requested
decision, and a bounded error code. A duplicate idempotency key with the same
dispatch digest returns the stored `{decision,status,response_state,
provider_dispatched:true}` result, including `cancel/cancelled/written`; a
different digest is a conflict.

## Usage and Rate Limit contract

```text
UsageSnapshot {
  id
  provider: "codex"
  account_id
  device_id
  source: official | cli_derived | local_estimate | unofficial
  confidence: high | medium | low
  window_kind
  used_value?
  limit_value?
  used_percent?
  resets_at?
  observed_at
  raw_reference_hash?
  source_version
  capability_status: supported | unavailable | schema_changed | error
  error_code?
}
```

Only fields present in the verified app-server response shape are populated;
`raw_reference_hash` is an integrity reference and never a raw response. The
public CLI must identify source, confidence, and observation time and must not
turn a missing value into a zero or a quota estimate.

## CredentialMaterializationManager contract

The Phase 1 `vault.Materializer` remains a Fake Provider test boundary. Codex
uses this provider/core interface; its handle is process-local and cannot be
serialized over IPC:

```text
Acquire(ctx, CredentialInstanceID, RuntimeProfileID) -> MaterializationHandle
MaterializationHandle.AuthHomePath() -> private path
MaterializationHandle.RefreshLease(ctx) -> LeaseState
MaterializationHandle.ObserveAndCommit(ctx) -> CommitResult
MaterializationHandle.Quarantine(ctx, reason) -> error
MaterializationHandle.Release(ctx) -> error
```

The manager obtains secret input from the unlocked Vault through a typed
provider-specific source. Neither the CLI nor an adapter request accepts raw
credential bytes. It owns shutdown ordering: stop new Provider mutations,
drain/stop the canonical app-server, digest and validate the auth file, CAS the
next revision, and quarantine when the sequence is ambiguous.

The production source is distinct from the Phase 1 lock-state manager and test
double:

```text
VaultCredentialSource {
  ReadCredential(ctx, CredentialInstanceID, expectedRevision) -> SecretHandle
  ReplaceCredentialCAS(ctx, CredentialInstanceID, expectedRevision,
                       validatedSecret) -> nextRevision
}

AuthEnrollmentBroker {
  Begin(ctx, RuntimeProfileID, mode=interactive) -> EnrollmentHandle
  Complete(ctx, EnrollmentHandle) -> CredentialInstance
  Cancel(ctx, EnrollmentHandle) -> void
}
```

`SecretHandle` is memory/process-local, zeroized/released after staging, and
never JSON-serialized. Enrollment uses daemon-owned `0700`/user-DACL staging;
the official login UI may be attached locally but Provider secret output is not
returned as an IPC result or stored in idempotency metadata.

Vault v1 freezes AES-256-GCM for both payload encryption and DEK wrapping, with
fresh 32-byte DEKs, independent random 12-byte nonces, and the 16-byte GCM tag
appended to each ciphertext. A 32-byte KEK is derived from raw password bytes
(1..1024 bytes, never normalized) with Argon2id version 0x13, time=3,
memory=65536 KiB, parallelism=min(4, online CPUs) and at least 1, salt=16 bytes,
output=32 bytes. Canonical AAD is each tuple field encoded as unsigned 32-bit
big-endian byte length followed by UTF-8 bytes, in this order:
`format_version`, `device_id`, `provider`, `credential_instance_id`,
`account_id`, `credential_revision`; numeric fields use base-10 ASCII. The
key-check encrypts the fixed UTF-8 bytes `MultiAgentDesk Vault v1 key check`
under the KEK with AAD field `vault_config:v1`. Payloads are strictly decoded
JSON objects of at most 64 KiB. All parameter and size bounds are verified
before allocation/decryption.

Vault lifecycle is explicit:

```text
schema absent -> migration creates empty Vault tables -> uninitialized
uninitialized --vault.initialize--> locked
locked --vault.unlock--> unlocked --vault.lock/restart--> locked
```

Migration `0005` creates only schema and constraints; it never inserts a
password-bound config row. `vault.status` reports `uninitialized`, `locked`, or
`unlocked` without secret material. `vault.initialize` is the sole first-use
operation. The CLI/UI must collect the password twice locally and compare it
(`--password-stdin` consumes exactly two newline-terminated entries and rejects
trailing input);
only the single matched 1..1024-byte value crosses authenticated local IPC, is
never logged or stored in idempotency/audit bodies, and is cleared best-effort.
The daemon derives a fresh salt/KEK and key-check envelope, then executes one
`BEGIN IMMEDIATE` transaction with `INSERT ... WHERE NOT EXISTS` for the fixed
`singleton_id=1`. The row stores the initializer Device ID and SHA-256 of the
length-prefixed `(client_device_id, random_idempotency_key)`, never a password
digest. A same-client/same-key retry returns the stored locked success (useful
after a lost response) without unlocking; exactly one different request wins
and all other callers receive `vault_already_initialized` and must call normal
unlock.

Crash before commit leaves no row and state `uninitialized`; crash after commit
leaves one valid row and restart state `locked`. `NOT NULL`, length/range,
algorithm, and singleton checks make partial/duplicate rows invalid. A missing
row is uninitialized; duplicate rows or structurally missing/invalid fields are
`vault_corrupt`; a structurally valid key-check authentication failure is the
generic `vault_unlock_failed` and remains locked. Initialization is refused
while any Codex CredentialInstance already has a nonempty secret reference or
positive revision, or any Codex enrollment/Provider binding exists; legacy Fake
metadata does not block first use. Password change/rekey is outside Phase 2;
backup/restore or a later atomic rekey contract is required.

The Store operation is one transaction:

```text
ReplaceVaultCredentialCAS(ctx, expectedRevision, VaultEnvelopeV1,
                          nextDigest, nextStatus) -> nextRevision
```

It CAS-checks CredentialInstance and atomically writes VaultItem plus revision,
digest, status, and timestamp. No standalone revision update is valid.

Enrollment IPC returns only metadata:

```text
auth.begin(profile_id) -> {enrollment_id, binary_path, argv:["login"],
                           staging_path, expires_at}
auth.complete(enrollment_id) -> CredentialMetadata
auth.cancel(enrollment_id) -> EnrollmentMetadata
auth.status(enrollment_id | credential_id) -> EnrollmentOrCredentialMetadata
auth.logout(credential_id) -> RevokedCredentialMetadata
```

`auth.logout` atomically reserves revocation before deleting any credential
home. `session.start` rejects a reserved credential in its Session-insert
transaction. Finalization deletes the Vault item, records `revoked`, and clears
the reservation atomically; an interrupted logout stays reserved and can be
retried without exposing a check/delete/start race.

The authenticated CLI launches exact `codex login` with the returned private
staging home and its own local browser/TTY. `--with-api-key`,
`--with-access-token`, and `--device-auth` are rejected. Begin is owner-bound
and one-active-per-profile; complete/cancel are idempotent. Restart expires
nonterminal enrollments and cleans staging. Complete runs daemon-owned bounded
initialize/account validation before the atomic Vault/credential commit.

## Materialization and revision contract

```text
MaterializationLease {
  credential_instance_id
  owner_id
  lease_revision
  acquired_at
  expires_at
}

AuthMutation {
  credential_instance_id
  observed_digest
  expected_revision
  validated_structure
  next_revision
}
```

The save path is compare-and-swap on `expected_revision`. A competing writer,
invalid structure, unreadable file, or ambiguous crash returns a typed error and
quarantines the materialized home as required. `mtime` is never a conflict
resolver.

## Error semantics

| Code | Meaning | Recovery |
|---|---|---|
| `provider_not_found` | binary discovery failed | install/configure Provider or use Fake Provider |
| `provider_version_unsupported` | no exact schema/fixture row | pin a supported version; do not start |
| `provider_schema_mismatch` | live schema differs from recorded fingerprint | disable affected capabilities and record a new Spike |
| `provider_protocol_error` | malformed/duplicate/trailing frame | stop Provider; preserve bounded redacted evidence |
| `credential_writer_conflict` | another canonical writer owns the CredentialInstance | wait/reconcile; never take over by mtime |
| `credential_revision_conflict` | CAS lost to a newer revision | reload/reconcile or quarantine |
| `credential_recovery_required` | ambiguous mutation/crash residue | block new starts and require official re-login |
| `approval_lease_required` | client is not current ControllerLease holder | observe or acquire lease |
| `approval_request_unknown` | Provider request ID is not pending | return conflict; do not send mutation |
| `approval_dispatch_ambiguous` | response write or runtime ownership became uncertain | expire request; require a new Provider Approval; never replay |
| `usage_unavailable` | Usage/Rate Limit method absent or failed | show unknown/best-effort with source |
| `provider_auth_unavailable` | production encrypted credential source is absent | keep Codex start disabled; configure/implement the Vault source |
| `vault_unlock_failed` | key-check, password, or Vault envelope authentication failed | remain locked; do not distinguish wrong password from tamper in user output |
| `vault_already_initialized` | another authenticated initializer committed first or init is repeated | remain locked/unlocked as current state; use status then normal unlock |
| `vault_corrupt` | singleton/config structure violates frozen v1 constraints | refuse initialize/unlock/materialization; restore backup or use a repair workflow |
| `auth_enrollment_conflict` | another enrollment owns the Profile/Credential | observe/cancel the existing owner-bound operation |
| `provider_control_unsupported` | operation has no exact Provider semantic, including Codex conversation resize | report unsupported without mutation |
| `provider_resume_unsupported` | exact Provider continuation is absent or unverified | preserve source state; return before mutation; operator may separately start an unrelated new Session |
| `provider_failed` | app-server exited or session failed | preserve Session failure and allow explicit resume if valid |

## Ordering, replay, and idempotency

- One reader goroutine owns app-server stdout. It correlates responses by
  JSON-RPC ID, routes notifications/server requests into a bounded ordered
  queue, and fails all pending calls on EOF/protocol failure. Writes use a
  separate mutex; no call or event consumer reads the stream directly.
- App-server responses and notifications are converted into one monotonically
  sequenced internal stream per Session.
- The Phase 1 ring buffer stores bounded output/event summaries and advertises
  truncation on replay. A second CLI must receive replay before live events.
- Start, attach, control, input, Approval response, stop, and resume use
  the existing idempotency/lease semantics. A Provider request ID is not itself
  a substitute for a local idempotency key.
- Resize keeps the existing lease/idempotency validation but returns
  `provider_control_unsupported` for Codex conversation sessions before any
  Provider write.
- Reconnect does not create a second refresh writer or silently select another
  Account/Profile. `sessions.resume` creates and links a new local Session only
  after exact fixture/live continuation evidence proves the Provider thread can
  be resumed; otherwise it returns `provider_resume_unsupported` before any
  local or Provider mutation.
- For the Phase 2 exit, `provider_resume_unsupported` is an accepted result only
  when no Provider mutation and no new local Session record occur. The product
  must not label that result Provider recovery. Continuation success is not
  required by this feature.

## Authentication and authorization

- The local daemon authenticates CLI clients using Phase 1 Device IPC identity
  and grants capabilities; it never accepts a Provider token in a request body.
- Credential materialization requires an unlocked Vault and the target
  CredentialInstance's explicit authorization state.
- Approval mutation, terminal input/resize, and stop/kill require the current
  ControllerLease and revision. Read-only events can be observed by an attached
  client.
- Control Plane/Web/Desktop interfaces are not added by this feature.

## Compatibility and migration

The contract is internal and versioned by the existing Device protocol major
version. `0005` is forward-only and atomic, preserves Fake rows, and adds Vault
v1, enrollment, and Approval dispatch state. Feature disablement leaves
ciphertext readable by the current binary; an old binary refuses the newer
schema. Recovery uses backup/restore or a newer binary, not destructive down
migration. Public REST/WebSocket and Control Plane contracts remain out of
scope.

## Handoff

Next role: `feature-review`.
