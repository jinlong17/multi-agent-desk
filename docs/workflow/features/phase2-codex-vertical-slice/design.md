# Design: Codex Vertical Slice

## Decision snapshot

- **Selected boundary:** a built-in, version-gated Codex adapter owned by
  `provider`, launched by the Device Daemon over stdio JSONL and integrated
  with the Phase 1 runtime/session/lease services.
- **Foundation boundary:** a versioned Device migration and local IPC contract
  must first expand the Phase 1 Fake-only records and method surface to accept
  Codex while preserving existing Fake data and fail-closed future-schema
  behavior.
- **Credential boundary:** one canonical writable app-server and one isolated
  `CODEX_HOME` per `CredentialInstance`, acquired through the Vault's
  exclusive materialization lease and committed with monotonic revisioned CAS.
- **Production-source boundary:** Phase 1's Vault is only a lock-state
  placeholder. P2B pulls forward the minimum local encrypted credential
  read/replace-CAS source required by Codex; Credential Grant and E2EE remain
  outside this feature.
- **Runtime boundary:** one `CredentialRuntime` per CredentialInstance owns the
  child, materialization handle, single-reader JSON-RPC multiplexer, and writer
  lifecycle. Each local Session has a separate `SessionBinding` with its own
  Provider thread/turn/Approval/event state.
- **Interaction boundary:** Codex conversation input maps to exact-schema
  `turn/start` and, only when verified, `turn/steer`. Conversation resize is
  typed unsupported and is not mapped to command-execution resize.
- **Capability boundary:** only methods and event mappings whose exact CLI
  version has schema and fixture evidence are enabled. Unknown versions are
  probed and then downgraded or rejected; they are never treated as the latest
  known version.
- **Supported login path:** official interactive login. Device-auth initiation
  may be an explicit experimental action, but completion is not a support claim.
- **Exit slice:** real Linux Session + second local CLI attach/control/Approval/
  resume, with macOS compatibility smoke and Windows compile/protocol coverage.

## Context and boundaries

Phase 1's `runtime.Manager.StartFake` and `SessionService` prove the local
lifecycle with a deterministic child process, but the shipped baseline is
intentionally Fake-only: SQLite checks, Store validators, local capabilities,
and CLI commands reject Codex. The first Phase 2 slice therefore has to extend
those contracts before an adapter can create a real Session. The Codex adapter
must fit behind that lifecycle instead of creating a second daemon, database,
IPC, or lease model. The plan intentionally keeps Web/Desktop/Control Plane
out of the exit so that a Provider failure is distinguishable from a remote
transport failure.

P0-P2 contract work and a bounded `internal/providers/codex.ProviderSession`
now exist, but they are not a runnable integration: `runtime.Manager.Start`
still returns `provider_unsupported`, `auth.begin` is unimplemented, the Vault
stores no encrypted secret, and `approval.respond` records
`provider_dispatched=false`. This revision treats those as implementation
dependencies rather than misclassifying the absence of a Linux host as the
only blocker.

## Components and ownership

### 0. Domain, Store, migration, and local IPC foundation (`core` with `provider`)

- Add a versioned Device migration that replaces Fake-only `CHECK` constraints
  for Provider/Profile/CredentialInstance/Session records with an explicit
  allowlist containing `fake` and `codex` values. Preserve existing Fake rows,
  refuse unknown future schema versions, and test upgrade, restart, rollback,
  and interrupted migration behavior.
- Add the local Account projection required by the implementation plan and
  link each Codex CredentialInstance to its Account without storing raw
  Provider identity in logs or Control Plane metadata. Display name and a
  one-way subject digest are local metadata; raw email/subject is not a
  compatibility fixture.
- Add Approval and UsageSnapshot persistence contracts matching
  `docs/IMPLEMENTATION_PLAN.md`: bounded pending/terminal Approval metadata,
  Provider Approval ID, payload digest, response actor, idempotency key, source,
  confidence, freshness, and observed values where the exact schema permits.
  Terminal text and Provider raw payloads remain outside SQLite.
- Extend the authenticated Device IPC authorization map and thin CLI with the
  provider methods needed by this feature. The canonical methods are
  `provider.describe`, `provider.health`, `profile.validate`, `auth.begin`,
  `auth.status`, `auth.logout`, `usage.read`, `session.start`,
  `session.input`, `session.resize`, `session.stop`, `session.resume`, and
  `approval.respond`; local `sessions.*`/`terminal.*` aliases must preserve
  the same capability, lease, and idempotency semantics.
- Add explicit capabilities for Provider diagnostics/profile validation,
  Usage read, and structured Approval response. Read-only Approval/Usage
  observation may be granted separately from mutation. Every new method is
  request-bound, bounded, and versioned with the existing Device protocol.
- Preserve the implementation-plan local management surface: Accounts and
  Profiles can be listed/created/edited/disabled without exposing Provider
  secrets; Credential status is metadata-only until an explicit `auth.begin` or
  `auth.logout` operation. Legacy Fake rows may keep a null Account and
  revision zero, while Codex rows require a linked Account and positive
  revision.
- Decide retention at the contract level: pending Approvals and structural
  UsageSnapshots persist with bounded metadata and restart recovery; terminal
  Provider payloads remain memory-only/ring-buffer or Provider-owned history.
  A restart marks an unresolved Approval as `expired` or `cancelled` according
  to the Provider capability, never silently replays a mutation.

### 1. Binary discovery and compatibility registry (`provider`)

- Search only explicit, platform-appropriate paths and configured overrides;
  never interpolate an untrusted path into a shell command.
- Execute a version probe with an argument vector, bounded output, timeout, and
  secret-safe stderr handling.
- Normalize `{provider, binary_path, version, platform, architecture}` and
  compute a schema/fixture fingerprint.
- Consult the compatibility matrix before enabling a capability. An unknown
  version can expose diagnostics and Fake Provider fallback, but cannot start a
  real Codex Session.

### 2. Schema-gated app-server client (`provider`)

- Spawn `codex app-server` through `exec.Cmd`/equivalent argument vectors,
  using stdio JSONL; do not expose the app-server socket/network surface.
- Perform `initialize`, `initialized`, and negotiated capability checks before
  any Account/Session call.
- Enforce bounded frame size, decode strictly, reject duplicate/unknown fields
  where the selected schema requires it, and classify malformed/unsupported
  messages as provider errors.
- Keep transport, protocol, and Provider event mapping separate so that a
  schema drift cannot corrupt Session state.

### 3. Credential materialization (`provider` with `core`/`security` contracts)

- Resolve a `CredentialInstance`, Account, and RuntimeProfile through the
  provider/core materialization interface. The Phase 1 `vault.Materializer`
  and `credential.fake` path remain Fake-only test support; they are not the
  Codex secret source and must not receive raw Provider bytes from an IPC
  request.
- `CredentialMaterializationManager` owns a non-serializable provider-specific
  handle: acquire/renew/release one writer lease, expose a bounded auth-home
  path to the child process, observe digest/structure changes, commit a
  monotonic revisioned CAS, and quarantine on ambiguity. The manager, not the
  CLI or adapter caller, owns secret input and shutdown ordering.
- Create a unique temporary home, apply restrictive permissions, materialize
  only the expected credential/config inputs, and validate the Provider account
  identity without persisting it in logs.
- Run the canonical app-server writer for that CredentialInstance. A second
  writer receives a deterministic conflict and cannot choose a newer file by
  `mtime`.
- Detect auth-file changes by digest, validate structure, and save with
  monotonic `credentialRevision` CAS. On interruption or ambiguous mutation,
  quarantine the home and require reconciliation or official re-login.
- Separate session/profile state from the single refreshable auth home; two
  sessions may multiplex the canonical owner but may not each persist a
  refresh-token copy.

#### 3.1 Production Vault source and enrollment

- Add a typed local Vault secret interface with per-CredentialInstance
  encrypted read and expected-revision replace-CAS operations. SQLite stores
  only the encrypted envelope, nonce/version/wrapped-key metadata, and digest;
  provider code never receives the Vault master key.
- Official interactive enrollment runs under a one-time daemon-owned staging
  home. The local broker may attach official login UI to the operator, but IPC
  accepts no token/auth-file body and no auth URL, device code, account identity,
  or secret is written to logs, idempotency records, or fixtures.
- After successful account validation, the daemon imports the bounded
  `auth.json` into the encrypted Vault record, destroys staging, and creates or
  advances the CredentialInstance revision atomically. Failed or cancelled
  enrollment removes staging and leaves the prior revision unchanged.
- P2B uses one portable password-derived local Vault backend on macOS, Linux,
  and Windows so the phase is executable without inventing three OS-keychain
  integrations. Phase 5 may migrate wrapping to Keychain/DPAPI/Secret Service;
  Credential Grant, cloud sync, and E2EE transport remain outside this feature.

#### 3.2 Vault v1 cryptographic and storage contract

- Forward migration `0005_codex_vault_and_approval_dispatch.sql` creates empty
  `vault_config`/`vault_items` schema and constraints; a migration never creates
  password-bound data. `vault_config` permits only singleton ID 1 and stores
  format/KDF version, a 16-byte random salt, Argon2id parameters, AES-GCM
  key-check envelope, initialized time, initializer Device ID, and a digest of
  the random init request key; it never stores a verifier or password/body hash.
- Vault state is `uninitialized` when the schema exists with no config row,
  `locked` when exactly one structurally valid row exists without an in-memory
  KEK, and `unlocked` only after authenticated key-check decryption.
  `vault.initialize` is the only first-use transition. The CLI/UI reads and
  compares two password entries locally, then sends one matched value over
  authenticated local IPC without idempotency/audit-body persistence.
- Initialization derives a fresh salt/KEK/key-check before one
  `BEGIN IMMEDIATE` insert-if-absent transaction. The config stores SHA-256 of
  length-prefixed client Device ID plus random idempotency key. Same-client/
  same-key retry returns stored locked success; one different caller wins and
  others return `vault_already_initialized`. Crash before commit leaves uninitialized; crash after commit
  restarts locked. Missing row means uninitialized; duplicate/partial/invalid
  config is `vault_corrupt`; valid-structure authentication failure stays the
  generic `vault_unlock_failed`. Initialization is refused if a Codex
  CredentialInstance already has a secret reference/positive revision or a
  Codex enrollment/binding exists; Fake metadata does not block it.
  Rekey/password change is outside Phase 2 and requires a later atomic contract.
- P2B derives a 32-byte KEK from raw 1..1024-byte password input (no Unicode
  normalization) using Argon2id version 0x13 with time=3, memory=64 MiB,
  parallelism=min(4, online CPUs) but at least 1, 16-byte salt, and 32-byte
  output. Parameters are strictly bounded on read. Unlock decrypts the fixed
  UTF-8 plaintext `MultiAgentDesk Vault v1 key check` with AAD
  `vault_config:v1`; wrong password and tamper return `vault_unlock_failed`.
- Every Vault item uses a fresh random 32-byte DEK. Credential bytes and the
  DEK are separately encrypted/wrapped with AES-256-GCM and independent random
  12-byte nonces; each ciphertext has its 16-byte tag appended. Canonical AAD
  is the ordered format/Device/Provider/CredentialInstance/Account/revision
  tuple encoded as uint32 big-endian byte length plus UTF-8 field bytes;
  numeric fields use base-10 ASCII. Plaintext is a strictly decoded JSON object
  of at most 64 KiB.
- `vault_items` stores envelope version, credential revision, cipher/wrap
  algorithms, both nonces/ciphertexts, AAD digest, secret digest, and timestamps.
  SQLite never stores plaintext, KEK, DEK, password, account identity, or auth
  UI data. Lock cancels new secret operations and overwrites in-memory KEK/
  decrypted buffers best-effort before release.
- One Store transaction CAS-checks the CredentialInstance revision and atomically
  writes the new Vault envelope, digest, next revision, status, and updated time.
  No file/digest-only commit is permitted. Crash before commit preserves the old
  envelope; crash after commit makes the new revision authoritative. A materialized
  file whose digest cannot be reconciled with that transaction is quarantined.
- `0005` is forward-only. Disabling the Codex feature retains readable encrypted
  rows; old binaries refuse the newer schema. Recovery is backup/restore or a
  newer binary, never a destructive down migration.

#### 3.3 Official enrollment state machine

- One active enrollment per RuntimeProfile/CredentialInstance is persisted as
  `begun -> validating -> succeeded | cancelled | expired | failed`, bound to
  the authenticated local client, binary fingerprint, private staging path,
  idempotency digest, and a 10-minute deadline. Daemon restart expires all
  nonterminal operations and removes/quarantines staging without importing.
- `auth.begin` returns only enrollment ID, validated binary path/argv, staging
  path, and deadline. The authenticated CLI launches exact `codex login` with
  `CODEX_HOME` set to that daemon-created private directory and attaches the
  official browser/TTY interaction locally; it never sends auth bytes through
  IPC. API-key/access-token/device-auth flags remain disabled in P2B.
- `auth.complete` carries only enrollment ID. The daemon CAS-claims validating,
  checks exact files/permissions/size, runs bounded app-server initialize plus
  `account/read` in staging, then atomically creates or advances the encrypted
  Vault item and CredentialInstance. Validation failure/cancel/timeout preserves
  the prior healthy revision and removes/quarantines staging.
- `auth.status` is metadata-only. `auth.logout` first atomically writes a
  durable revocation reservation while requiring zero live SessionBindings;
  Session creation rejects that reservation before it can insert. Logout then
  shuts down the last CredentialRuntime, removes the canonical materialized
  home, and transactionally deletes the Vault item, marks the
  CredentialInstance revoked, and consumes the reservation. A crash or
  filesystem failure leaves the reservation fail-closed for an idempotent
  retry. It does not claim remote Provider revocation.
- The normal independent `feature-review` is the P2B pre-build design gate.
  The Security Gate remains open for the workflow-supported final
  `security-review` from `READY_TO_SHIP`; no pre-build security-review edge is
  invented.

### 4. Session/event adapter (`provider` with `core`)

- Register Codex through the existing runtime manager rather than bypassing
  Session/Attachment/ControllerLease. Start validates Account/Profile/
  CredentialInstance/Workspace linkage, acquires the canonical materialization
  handle, discovers and probes the binary, spawns `codex app-server` with a
  minimal environment, handshakes, starts a thread, then transitions the local
  Session to running.
- Replace the protocol client's blocking global read mutex with one bounded
  reader loop. It correlates client responses by JSON-RPC ID, queues exact
  Provider notifications/server requests, serializes writes separately, and
  fails all waiters if the child exits or framing becomes invalid.
- `CredentialRuntime`, keyed by CredentialInstance ID, is the only process and
  writer owner. It retains the child, materialization handle, protocol client,
  exact descriptor/capabilities, binding map, reference count, and one
  idempotent finalization state. Compatible concurrent starts reuse it; a
  mismatched binary/schema/materialization revision returns conflict.
- `SessionBinding`, keyed by local Session ID and Provider thread ID, owns the
  Account/Profile/Workspace snapshot, active turn, pending raw Approval request
  IDs, ring/event sequence, and cancellation state. Notifications are routed by
  exact thread/turn IDs; unknown or cross-binding IDs fail closed.
- Convert app-server lifecycle and turn events into bounded internal events
  understood by the Phase 1 runtime ring and Session state machine.
- Preserve ordering, sequence, and truncation metadata. Provider stdout is
  protocol only; diagnostics go to redacted stderr/audit events.
- Treat Approval as a first-class event with a stable provider item/request ID,
  a redacted action/context summary, and an explicit response result.
- Route Approval responses through the existing ControllerLease and idempotency
  rules. Observers can read; only the lease holder can mutate.
- Keep session account/profile and Provider version pinned for the Session.
  Rate-limit or auth errors never trigger account rotation.
- Use the local Approval/Usage persistence and IPC contracts from component 0;
  do not invent a provider-only side channel that bypasses ControllerLease.

#### 4.1 Input, stop, and resize semantics

- `session.input` from the current ControllerLease holder creates
  `turn/start` when the thread is idle. Input during an active turn is rejected
  unless the exact compatibility row enables and fixtures prove `turn/steer`.
- `terminal.input` may remain a compatibility alias, but its Provider behavior
  is the same structured turn operation; it is not raw stdin.
- `session.resize`/`terminal.resize` returns typed
  `provider_control_unsupported` for a Codex conversation. A future
  command-execution resize requires a verified process ID and exact
  `command/exec/resize` contract and is outside this slice.
- Graceful stop affects only its SessionBinding: interrupt its active turn when
  exact `turn/interrupt` evidence is enabled, then mark/remove that binding.
  Kill force-cancels only that binding; it is not permission to kill a shared
  child. The app-server and materialization handle stop/finalize only after the
  last binding releases. A child crash fails every binding once; daemon close
  drains all bindings, stops each CredentialRuntime once, then performs final
  CAS/release. The nonexistent `session/stop` method must not be invented.

#### 4.2 Approval dispatch transaction

- Persist Provider Approval requests as pending bounded metadata and retain the
  raw JSON-RPC request ID only in the live runtime object, never in durable
  authority state.
- A lease-authorized response first CAS-claims a `dispatching` response state
  with `approve|deny|cancel`, actor, and idempotency key. The runtime writes the exact
  JSON-RPC server-request result. Only a successful bounded write finalizes the
  Approval as `approved|denied|cancelled` with `response_state=written`.
- A write error, process exit, daemon restart, or lost runtime request ID while
  dispatching becomes `ambiguous`/expired and is never automatically replayed.
  Repeating an already finalized identical idempotency key returns the stored
  result; a different digest is a conflict.
- Command-execution and file-change Approvals for exact 0.144.2 map local
  `approve|deny|cancel` to `accept|decline|cancel`. Session-persistent and policy
  amendment variants are disabled. Permissions Approval remains disabled until
  its structured granted-profile contract receives separate fixture/live
  evidence. Older rows expose no actionable Approval capability.
- `0005` constrains successful pairs to `approve/approved`, `deny/denied`, or
  `cancel/cancelled`. A duplicate same-key/same-digest response returns the
  stored decision/status/written result (including cancel); different digest
  conflicts. Restart from any decision in dispatching remains expired/ambiguous.

### 5. Usage and Rate Limit projection (`provider`)

- Expose only the methods present in the exact schema/fixture matrix.
- Store source version, observed time, freshness, confidence (`official/high`,
  or a clearly labelled lower-confidence fallback), and a capability status.
- If a method is absent, fields change, or the call fails, return `unknown` or
  best-effort status with a reason. Never synthesize an official remaining
  quota from terminal text.

## Phase plan and sequencing

| Phase | Scope | Dependencies | Exit evidence | Rollback |
|---|---|---|---|---|
| P0 Domain/Store/IPC foundation | Provider/account/profile/session schema expansion, migration/future-schema policy, Approval/Usage records, capabilities, local methods/CLI, retention and restart semantics | Phase 1 Device Kernel; implementation-plan domain/API tables | Fake data round-trip remains green; Codex rows can be created/read through Store and authenticated IPC; Approval/Usage negative/auth/idempotency tests pass | keep migration unapplied and Fake-only behavior; refuse Codex start; restore prior schema only through a tested migration rollback |
| P1 Contract and fixtures | App-server framing, initialize handshake, schema fingerprints, method allowlist, redacted record/replay fixtures, binary/version probe, Approval/Usage schema mapping | P0; current Codex matrix | fixture replay passes for each enabled version; unknown/malformed input and unmapped Approval fields fail closed | keep Fake Provider as the only real runtime path; remove adapter registration |
| P2A Materialization manager contract | Isolated private home, lease/CAS, digest/structure validation, writer conflict, quarantine/recovery | P0/P1; ADR 0014 | deterministic contract tests prove one writer, restrictive files, no secret output, and fail-closed recovery | keep production source absent and Codex start disabled |
| P2B Production credential source | `0005` empty Vault v1/enrollment/Approval-dispatch schema; atomic one-time Vault initialization; portable Argon2id/AES-GCM Vault; official enrollment; atomic secret+revision CAS | P2A; implementation-plan Vault contract; independent feature-review | first/concurrent/crash init, exact crypto vectors, wrong-key/tamper/bounds, transactional CAS, login success/cancel/timeout/restart, durable Approval-cancel constraints, and macOS/Linux/Windows tests pass without secret output | disable enrollment/source registration; retain forward-readable ciphertext; old binary refuses schema; backup/restore only |
| P3A Daemon runtime bridge | CredentialRuntime/SessionBinding, single-reader JSON-RPC, thread/turn routing, event/Usage persistence, exact Approval dispatch table, daemon-owned capabilities, input/stop/kill and typed resize unsupported | P1/P2B; exact method rows and `0005` dispatch state | two same-credential Sessions share one child; stopping one preserves the other; concurrent calls/events, crash fan-out, dispatch ambiguity, and Fake regression tests pass | unregister Codex runtime; retain readable data; stop shared runtime only after last reference |
| P3B Live Linux exit | pinned real app-server, structured events/Approval/Usage, second CLI attach/replay/lease/turn input, typed resize unsupported, stop/kill, frozen resume result | P3A; exact Linux schema/version; credentialed test environment | reproducible sanitized Linux exit; no unsupported continuation or terminal capability claim | feature-gate Codex; preserve evidence; no remote rollout |
| P4 Matrix and handoff | macOS smoke, Windows Go/IPC CI, exact compatibility rows, security review package, user-facing readiness notes | P3; platform environments; docs | compatibility matrix and verification report match actual evidence; Security Gate accepted | retain only the verified platform/version rows and mark others unsupported |

Only one phase may be built at a time. A phase completes at `READY_FOR_VERIFY`
and stops for an independent `feature-verify`; no test result authorizes Ship.

## Failure and recovery

| Failure | Required behavior | Forbidden behavior |
|---|---|---|
| Binary missing or version unknown | Return capability diagnostic and allow Fake Provider path | Download, silently use another binary, or claim compatibility |
| Schema/method mismatch | Disable that capability; preserve explicit error/source | Parse arbitrary JSON or infer fields by position |
| Auth-home writer conflict | Return conflict; retain current owner and audit metadata | Last-writer-wins, mtime selection, or concurrent token writes |
| Auth file changed unexpectedly | Validate digest/structure and CAS; quarantine on ambiguity | Copy the file into logs or overwrite a newer revision |
| Production Vault source absent | Return `provider_auth_unavailable`; keep metadata/Fake path usable | Use the test CredentialSource or read an operator home implicitly |
| JSON-RPC reader exits or response ID is unknown | Fail pending calls, stop event pump, mark Session failed, finalize/quarantine once | Start a second reader or ignore unmatched responses |
| App-server crash | Mark Provider failure, keep bounded evidence, allow explicit resume if valid | Auto-switch account or silently recreate credentials |
| Approval without lease | Observer-only event; response rejected | Let any attached client approve |
| Approval write is ambiguous | Mark dispatch ambiguous/expired and require a new Provider request | Replay the response automatically or mark it approved locally |
| Codex conversation resize | Return typed unsupported without mutation | Call command-exec resize without a verified process ID |
| Usage/rate-limit unavailable | `unknown`/best-effort with source and freshness | Show fabricated official quota |
| Credential revoke/lock | Block future materialization and stop new starts | Promise remote erasure from a host that already received plaintext |
| Unresolved Approval after restart | Mark expired/cancelled according to the verified capability and require a new request | Replay a mutation or silently approve |
| Resume method absent or unverified | Return `provider_resume_unsupported` before mutation; leave the source unchanged; operator may separately start an unrelated Session | Pretend a new Fake/local Session is Provider history recovery |

## Security and privacy

- No auth file contents, tokens, authorization URLs, device codes, email,
  workspace ID, or account identity in fixtures, logs, screenshots, metrics,
  crash reports, or command-line arguments.
- Use argument vectors and environment/file-descriptor injection with an
  explicit allowlist. Avoid shell expansion and inherited secret-bearing
  environment variables.
- Audit only event kinds, capability decisions, version/fingerprint, redacted
  error codes, lease/revision transitions, and bounded sizes.
- Treat the app-server and host as capable of reading materialized credentials;
  this plan reduces accidental disclosure and stale overwrites, not compromise
  impact.

## Compatibility and platform posture

- **macOS:** live/provider smoke may consume the exact versions already evidenced
  by the Codex Spike; any new version or method needs a new replay row.
- **Linux:** Phase 2's real exit platform. The selected binary's app-server
  schema must be replayed and recorded before P3; the existing `.1444` auth
  observation is not by itself a schema support claim.
- **Windows:** Phase 1 CLI/Daemon/IPC and CI remain required. This feature does
  not claim a real Windows Codex provider because no such acceptance exists in
  the current matrix. Windows Desktop remains Experimental and is out of scope.
- **P2B Vault:** the password-derived Vault v1 backend is the minimum portable
  local-only implementation on all three platforms. Platform Keychain/DPAPI/
  Secret Service wrapping is a Phase 5 enhancement and must migrate, not silently
  reinterpret, Vault v1 rows.

## Frozen Resume contract

`sessions.resume` is a MultiAgentDesk operation that may create a new local
Session only after Provider history/thread continuation is proven. On that
supported branch it preserves the same Account, CredentialInstance,
RuntimeProfile, Workspace, and Provider version, links the source with
`resumedFromSessionId`, and never reactivates the old Session record.
Continuation is an additional capability, not an assumption:

1. The adapter may reuse a Codex Provider thread only when the exact version's
   schema, fixture replay, and live restart scenario prove the continuation
   method and its ordering.
2. If that evidence is absent or the app-server restart loses the Provider
   thread, `sessions.resume` returns `provider_resume_unsupported` without
   creating a Provider mutation; the operator must start an explicit new turn.
3. Phase 2 accepts the typed unsupported branch as its frozen Resume exit when
   the live test proves no Provider mutation, no new local Session creation, and
   no Fake/local Session masquerading as Provider history. It must not claim
   Provider session recovery. Verified continuation remains an optional future
   capability and requires exact fixture/live evidence.

## Rollback and migration

P0 explicitly includes a versioned Device migration. It must preserve all
existing Fake rows, keep the schema version monotonic, reject unknown future
versions, and leave the old Fake-only behavior available if Codex registration
is disabled. Migration failure or interruption leaves the database unopened
until recovery; it must never partially reinterpret a Fake row as Codex. The
adapter remains removable behind a provider registration/feature gate; prior
Device data and Fake Provider sessions remain usable.

## Remaining evidence gates for independent review

1. The Linux Codex version and its schema fingerprint must be selected from a
   reproducible probe; no unrecorded version may enter P3.
2. Approval payload redaction fields must be checked against the exact schema
   before storing any summary; unmapped fields remain unavailable rather than
   being guessed.
3. The frozen Resume contract above requires a live typed-unsupported no-mutation
   result; continuation success is not required for the Phase 2 exit.

## Handoff

Next role: `feature-review`.
