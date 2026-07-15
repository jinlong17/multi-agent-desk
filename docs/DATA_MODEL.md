# Device data model

This document records the as-built Phase 1 P1 Device schema. Later Phase 1
runtime, IPC, Vault, and CLI behavior remains governed by
[`IMPLEMENTATION_PLAN.md`](IMPLEMENTATION_PLAN.md) and the
[`phase1-device-kernel`](workflow/features/phase1-device-kernel/design.md)
feature until independently verified.

## Authority and storage contract

- The Device Daemon is the only production database writer. CLI/TUI clients
  will use the application service and authenticated local IPC; they never open
  this database directly.
- The driver is `modernc.org/sqlite v1.53.0`, built without CGO.
- A Store uses one connection, WAL mode, foreign keys, a 5-second busy timeout,
  and explicit serializable transactions.
- The database file is `0600` inside a non-symlink `0700` Device root on Unix.
  Windows endpoint and root ACL enforcement enters P2; ordinary Go mode bits
  are not presented as an equivalent Windows security boundary.
- Schema migration filenames, versions, and SHA-256 checksums are immutable.
  A changed, missing, non-contiguous, or future version fails with
  `schema_incompatible`; the Store does not delete or recreate the database.

The current schema is version 3:

| Migration | Tables | Purpose |
|---|---|---|
| `0001_device_identity.sql` | `device_identity`, `client_identities`, `workspaces`, `runtime_profiles`, `credential_instances` | local identity and Fake Provider configuration metadata |
| `0002_sessions.sql` | `sessions`, `session_attachments`, `controller_leases`, `session_events` | Session state, independent client attachment/control, structural events |
| `0003_operations.sql` | `audit_events`, `idempotency_records`, `credential_materializations` | bounded decision/idempotency metadata and the later Phase 1 pending fake-materialization journal |

`schema_migrations` is the bootstrap ledger and stores migration version,
filename, checksum, and application time. `PRAGMA user_version` must match the
contiguous applied ledger.

## Domain invariants

### Device and local clients

`device_identity` contains the one Daemon identity and its Ed25519 public key.
`client_identities` contains only client public keys, monotonically increasing
identity revisions, `active|revoked` status, and a canonical capability set.
Private keys are not database fields.

Cryptographic provisioning, rotation, revocation, pinning, and handshake
behavior are planned for P2 and are not claimed as implemented by P1.

### Workspace, RuntimeProfile, and fake CredentialInstance

- Workspace paths are device-local and unique per Device; they are not portable
  cross-device identifiers.
- Phase 1 RuntimeProfiles are constrained to provider `fake` and retain valid
  JSON settings.
- Phase 1 CredentialInstances are constrained to `provider=fake` and
  `auth_method=fake`. They store a secret reference, SHA-256-shaped digest,
  status, and monotonic revision, never a real Provider credential.

The `credential_materializations` schema pre-allocates the approved later
Phase 1 journal: manifest version 1, expected digest, revision, state, and
reference count. P1 does not yet write runtime homes or claim materialization
recovery.

### Session

```text
starting -> running -> stopping -> exited
    |          |          |
    +----------+----------+-> failed | killed
```

`exited`, `failed`, and `killed` are terminal. Rejected transitions do not
mutate the in-memory record. A failed Session requires a bounded failure code.
Terminal records have an end time; live records do not.

Resume requires the frozen `session.resume` Capability and a terminal source.
It creates a new `starting` Session with a distinct ID and
`resumed_from_session_id`; it never changes the original terminal record. The
new start time may not precede the source end time.

The Session freezes Device, provider, CredentialInstance, RuntimeProfile,
Workspace, and canonical capability snapshot at creation.

### Attachment and ControllerLease

Attachments are separate from Session state. The schema permits multiple
clients per Session but only one Attachment per `(session, client)`.

`controller_leases` has exactly one current/released row per Session. Domain
operations enforce:

- default 30-second duration and 10-second heartbeat expectation;
- no acquisition while a lease is active, including by the same holder;
- holder identity and exact revision on heartbeat, release, and control;
- monotonic operation time and lease revision;
- revision increment on release and on acquisition after expiry/release;
- stale, wrong-holder, expired, or missing control fails before mutation.

P3 will connect these invariants to actual Session input/resize/stop/kill
operations. P1 proves the domain and compare-and-swap repository behavior only.

### Structural events and audit metadata

`session_events` stores monotonic structural metadata per Session, not terminal
payloads. `audit_events` stores bounded actor/action/target/decision/error
metadata. `idempotency_records` is keyed by client, method, and idempotency key
and stores request digest plus bounded response metadata.

Retention/pruning and application-layer redaction enter later approved phases.
The schema and P1 repositories do not authorize storing terminal content,
private keys, fake secret bytes, or raw IPC frames.

## Transaction and recovery evidence

P1 tests prove:

- empty database migration, pragma readback, one-time restart, and restrictive
  Unix file/directory handling;
- complete rollback of a migration containing valid DDL followed by invalid
  SQL, including unchanged ledger and `user_version`;
- refusal of future schema and changed checksum while preserving unrelated
  data;
- foreign-key rollback and stable error codes;
- restart persistence for identities, Workspace/Profile/Credential metadata,
  Session transition, Attachment deletion, ControllerLease CAS, and structural
  event/audit rows;
- race-clean domain, migration, and repository tests on the development macOS
  host.

Native three-platform runtime evidence remains required by the subsequent
phase verifiers and final Phase 1 exit gate.
