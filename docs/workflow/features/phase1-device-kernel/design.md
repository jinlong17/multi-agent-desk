# Design: Phase 1 Device Kernel

## Decision snapshot

- Selected option: build one Go `multidesk` binary around a single-writer
  application service, ordered Device SQLite migrations, authenticated local
  IPC, a deterministic Fake Provider subprocess, bounded runtime state, and
  thin CLI/TUI clients that never read the database directly.
- Review evidence: Phase 1 Feature Brief; ADR 0002, 0005, 0009, 0012, 0013,
  and 0015; the implementation-plan domain, storage, terminal, and platform
  contracts; the Phase 0.5 Windows IPC evidence; and the dependency evaluation
  recorded in this feature's Evidence Ledger.
- Frozen assumptions: Phase 1 uses fake credential bytes only; the Daemon is
  the only persistent/runtime writer; Windows Server CI proves the Named Pipe
  contract but not Windows 11 release acceptance; and no Fake Provider result
  is promoted to a real Provider or PTY compatibility claim.
- Rejected alternatives: CGO SQLite; `ncruces/go-sqlite3` while its Wasm module
  remains unidentified by the exact governance scanner; a public or
  unauthenticated loopback listener; filesystem permissions as client identity;
  `go-winio` without the required live-DACL checks; direct CLI database access;
  in-process Fake Provider execution; unbounded terminal history; and
  reactivating a terminal Session record during resume.

## Context and boundaries

Phase 1 turns the empty scaffold into the first usable Device Kernel. It owns
local domain behavior, durable state, Daemon lifecycle, local IPC, fake Vault
state, fake credential materialization, Fake Provider process ownership, and
CLI/TUI integration. It does not implement real Provider discovery, login,
authentication, PTY/ConPTY, Control Plane, browser, Desktop, E2EE, release, or
deployment.

The Daemon is the sole authority for Device SQLite, identity allowlists, Vault
state, runtime homes, child processes, Session transitions, attachments, and
leases. A CLI or TUI process is always an authenticated client. Tests may call
the same application-service interface in process, but production clients may
not bypass IPC.

## Components and ownership

| Component | Package/path | Responsibility |
|---|---|---|
| Domain | `internal/domain` | IDs, entities, state transitions, capabilities, lease invariants, stable errors |
| Device store | `internal/storage`, `migrations/device` | SQLite open/configure/migrate, repositories, transactions, schema recovery |
| Application service | `internal/app` | authorization, idempotency, use cases, audit decisions, runtime/storage coordination |
| Local IPC and identity | `internal/device` | paths, Ed25519 identities, framing, handshake, Unix socket/Windows Named Pipe adapters, service specifications |
| Runtime | `internal/runtime` | process ownership, Fake Provider protocol, ring buffer, attachments, leases, materialization, recovery |
| Vault runtime | `internal/vault` | Phase 1 `locked`/`unlocked` state and fake-secret access boundary; no production encryption claim |
| Command | `cmd/multidesk` | bootstrap, Daemon commands, JSON/human CLI, minimal TUI, hidden Fake Provider child mode |
| CI/governance | `.github`, scripts, workflow/dashboard documents | three-platform execution, licenses, protected checks, verified status only |

`core` owns this feature. `security` independently reviews the trust boundary;
`provider` receives the frozen runtime interface but no real adapter change;
`desktop` later consumes service/IPC specifications; `project-system` owns CI
and dashboard surfaces.

## Data flow and state transitions

### Bootstrap and identity

`multidesk init --root <dir>` creates a private Device root, an Ed25519 Daemon
identity, a default local-owner client identity, a pinned Daemon public key,
and the Device database. Private files are created atomically with restrictive
permissions. The client public key and allowed Phase 1 capabilities are stored
in the Device database; a client private key is never stored there.

Additional test/operator clients are provisioned explicitly. Rotation replaces
an allowlisted public key at a monotonic identity revision and invalidates the
old key. Revocation is fail closed. Lost-key recovery is not performed over an
untrusted IPC connection: it requires the Daemon to be stopped, exclusive
ownership of the private Device root, and an explicit offline rotate command.
There is no automatic reset or permissive fallback.

### IPC authentication and requests

The server first owns and verifies the OS endpoint, then performs a versioned
Ed25519 challenge handshake before returning metadata:

1. client sends protocol version, client ID, ephemeral nonce, and requested
   capability set;
2. Daemon returns its ID, endpoint-instance nonce, ephemeral nonce, negotiated
   version, and a signature over the canonical transcript;
3. client verifies the pinned Daemon key and signs the complete transcript;
4. Daemon verifies the allowlisted client key, intersects requested and stored
   capabilities, and binds the result to the connection context.

Every request carries a bounded request ID, method, body, idempotency key when
required, and lease revision when required. The application service maps the
method to its required capability; it never trusts a client-supplied capability
label. Mutations are executed through an idempotency record and an explicit
SQLite transaction. Authentication failures are generic and expose no device,
client, or capability inventory.

Frames use a four-byte unsigned big-endian length followed by canonical JSON.
The maximum encoded frame is 256 KiB. Handshake, idle, read, write, and request
deadlines are mandatory. Connection and in-flight request counts are bounded.
Unix uses a private `0600` socket inside a `0700` root. Windows uses the ADR
0013 message-mode Named Pipe with a protected current-logon DACL, Network deny,
remote rejection, first-instance ownership, and live-DACL verification. No
transport fallback is automatic.

### Persistence and runtime

The selected SQLite driver is `modernc.org/sqlite v1.53.0`. It is CGO-free,
declares Go 1.25, ran a WAL/foreign-key/busy-timeout smoke test under Go 1.26.5,
cross-built production programs for darwin/arm64, linux/amd64, and
windows/amd64, and its complete production dependency set is identified by the
exact `go-licenses v2.0.1` scanner as allowed MIT/BSD-3-Clause. The database
uses WAL, foreign keys, a bounded busy timeout, one writer pool, ordered
embedded migrations, and explicit serializable transactions. A future schema
version aborts startup; there is no destructive downgrade.

The Fake Provider is a child mode of the same binary with a narrow line-framed
test protocol. The Process Manager owns its process handle, stdin/stdout,
bounded termination, and exit observation. Session transition legality lives
in the domain layer. Detaching removes an Attachment only. A default 4 MiB
chunk-aligned ring buffer emits monotonic sequence numbers and an explicit
`truncated` marker when replay starts after discarded content.

One Session has at most one ControllerLease. The default duration is 30
seconds and heartbeat interval 10 seconds. Acquire, heartbeat, release, input,
resize, stop, and kill use a monotonic lease revision; stale revisions fail.
Observer clients cannot mutate. Stop is graceful and bounded, Kill is forced,
and both are idempotent. Resume creates a new Session linked by
`resumedFromSessionId`; a terminal Session never transitions back to running.

### Vault and materialization

Phase 1 persists Vault metadata but keeps the unlock state only in Daemon
memory, so every Daemon restart is locked. Metadata reads remain available;
Session start and fake materialization return `vault_locked` until an authorized
client explicitly unlocks. No real Provider credential is accepted.

The materialization manager is the only runtime-home writer. It uses a
monotonic `credentialRevision`, compare-and-swap, staging directories,
same-filesystem atomic rename, restrictive permissions, reference counts, and
explicit cleanup. Before filesystem commit, the database records a pending
materialization containing revision and the SHA-256 digest of a deterministic
canonical JSON manifest. The version-1 manifest contains only lease ID,
CredentialInstance ID, revision, fake-content digest, and completion state.
Startup may promote a complete directory only when its version, revision, and
manifest digest match that pending database row. Missing, incomplete, unknown,
or conflicting residue is quarantined and never overwrites a newer database
revision. This unkeyed digest detects partial or accidental residue; it is not
an authenticity control against a same-user attacker. Authenticated production
materialization remains part of the later security-owned Vault implementation.

## Failure and recovery

- Endpoint already exists, ownership is ambiguous, or policy readback differs:
  fail Daemon startup without choosing another endpoint.
- Unknown client, bad signature, transcript mismatch, version mismatch, slow
  peer, malformed/oversized frame, or resource limit: close the connection and
  retain only bounded redacted decision metadata.
- SQLite busy beyond deadline or future schema: return a stable unavailable or
  incompatible error; never delete or recreate the database.
- Daemon restart: mark formerly active Sessions failed with a recovery reason;
  never adopt an unauthenticated surviving child. Promote only a complete
  materialization matching a pre-registered pending row; quarantine ambiguous
  runtime homes and start locked.
- Fake Provider exits unexpectedly: persist `failed`, retain bounded replay,
  release its lease, and clean or quarantine materialization.
- Graceful stop exceeds its deadline: remain `stopping` until an explicit Kill
  or configured escalation; Kill persists `killed` only after process teardown.
- Dashboard or documentation disagrees with the feature log: verification
  fails and the operator-directed writer refreshes from persisted facts.

## Security and privacy

- Ed25519 keys identify peers; endpoint ACLs and file modes only narrow access.
- Canonical handshake transcripts bind both identities, both nonces, endpoint
  instance, protocol version, and requested capability set to prevent replay
  or downgrade.
- Authorization is server-derived and deny-by-default. Lease-protected methods
  require the current holder and revision in addition to a capability.
- Logs/audit rows exclude private keys, fake credential bytes, terminal input
  and output, raw frames, paths outside the Device root, and unbounded errors.
- The Phase 1 manifest digest is integrity-only and carries no authenticity or
  production Vault-encryption claim.
- Frame, replay, process, connection, request, and disk usage are bounded.
- Same-logon malware and administrators remain residual host risks. Windows 11
  multi-user/service acceptance remains a later platform gate.
- The final implementation must receive an independent security-review verdict
  before ship.

## Compatibility and migration

Protocol major version `1` rejects unsupported majors. Minor additions are
optional fields only; unknown methods return `method_not_found`. JSON output is
versioned and additive. Database migration IDs are monotonic and immutable.
Phase 1 starts at schema version 1 and tests empty, current, interrupted, and
future-schema cases.

The Fake Provider protocol is internal and explicitly not a Provider adapter
compatibility result. Windows CI must run the native pipe E2E; Unix CI must run
the native socket E2E. Cross-compilation alone cannot satisfy Phase 1 exit.

## Rollback

Each phase is a signed commit whose changes can be reverted before ship. Schema
migrations are forward-only; a rollback binary must refuse a newer schema
rather than mutate it. Before protected-main merge, abandon the feature branch.
After merge, use a signed correction PR. Do not delete Device data, keys,
runtime quarantine, or audit evidence automatically. No release, tag, or
deployment is created by Phase 1.
