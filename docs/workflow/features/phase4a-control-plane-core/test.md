# Test Plan: Phase 4a Control Plane Core

## Verification policy

One approved phase is implemented per `feature-build` run. The writer records
commands/artifacts in `dev_log.md`, sets `READY_FOR_VERIFY`, and stops. An
independent `feature-verify` reruns the phase gate and inspects the diff before
the next phase can start. Unknown, flaky, skipped, partial, or platform-missing
evidence is not a pass.

The Provider Gate is `none`. CI and integration tests use Fake/local Sessions
and synthetic non-secret metadata. No real Codex/Claude credential is required
or permitted in fixtures. The Security Gate stays open until all phases are
verified and an independent `security-review` accepts the final boundary.

## Global deterministic gates

Every product phase runs, as applicable:

```text
gofmt/check-go-format
go test -count=1 ./...
go vet ./...
go test -race on changed concurrency packages
go build ./cmd/... for macOS arm64, Linux amd64, Windows amd64
pnpm install --frozen-lockfile
pnpm recursive typecheck/test/build
OpenAPI validate + deterministic generation + drift check
Web unit/component/browser/a11y/responsive tests
Desktop shared Web build + cargo fmt/check/render smoke
workflow/project/dashboard structural verification when their files are touched
Actions/CODEOWNERS/fixture/link/license/DCO gates
git diff --check
secret/prohibited-field scans
```

Generated and migration tests run with the pinned repository toolchains: Go
1.26.5, Node 24, pnpm 10.23.0, and Rust 1.91.1. Exact dependency pins and full
resolved license graphs are recorded; adding a tool dependency without an
explicit license scan fails P0/P1.

Coordinator structural baseline at 2026-07-21 01:55 PDT is retained in the
Evidence Ledger: bundled Node/pnpm `project:verify`, `ci:verify`, and
`git diff --check` passed, including workflow/dashboard generation/verification,
Actions, CODEOWNERS, fixtures, links, and licenses. Each later phase reruns the
applicable checks; operator-owned `dashboard-state.json` is never rewritten by
this plan revision.

## P0 — Contract freeze

### Required tests

| Area | Positive case | Negative case / required result |
|---|---|---|
| Pin vector | both Go and TS compute the same 32-byte domain-separated SHA-256 digest and six four-character Base32 groups from its first 15 bytes | old eight-group full-hex display is rejected as the Phase 4a fingerprint |
| Attestation vector | existing restricted Go/TS RFC 8785 codecs produce identical bytes and Ed25519 signature for the typed strings/integers/arrays object containing both full digests, canonical capabilities, IDs, issued/expiry | changed digest/capability/expiry/ID, float/arbitrary map, duplicate/unknown member, escaping/Unicode/order/integer boundary, or signature fails |
| Key PoP vector | both implementations agree on the framed transcript, X25519 shared secret, HKDF-SHA256 pop key, HMAC-SHA256 exchange proof, and Ed25519 signing proof | all-zero/mismatched shared secret; changed purpose/ID/key/`storageMode`/`storageAssertionDigest`/challenge/expiry/server ephemeral; replay; or restart fails in both Go and TypeScript |
| Presentation | uppercase/lowercase and display hyphens decode to the same exact 15 bytes for comparison | 23/25 chars, non-Base32, altered group, or using the truncated value as stored digest fails |
| Scope docs | implementation plan, ADR 0010/0011, protocol, vectors, threat model, API/design agree on portable Vault anchor and 4a/4b/5 split | any OS-backed claim for portable Vault, pure-Web anchor, WSS/HPKE/terminal/grant in 4a fails link/contract review |
| Dependencies | exact tag/license/toolchain provenance is recorded and full graphs pass license policy | floating versions, missing lock, unknown/incompatible license, unsupported toolchain fail |

### P0 acceptance

- `docs/spikes/e2ee/verify.mjs` passes on Linux/macOS/Windows with revised
  vectors and the prior E2EE negative cases remain green.
- Protocol/vector result hashes are intentionally updated and recorded; both
  independent outputs are byte-identical.
- `go-webauthn v0.17.4`, `oapi-codegen v2.8.0`,
  `openapi-typescript 7.13.0`, tool-graph `kin-openapi v0.142.0`, and direct
  `google/uuid v1.6.0` provenance, licenses, locks, and toolchain compatibility
  pass before product imports. `openapi-fetch` is absent.
- No product package, migration, or runtime behavior is added in P0.

## P1 — Server/storage/OpenAPI foundation

### Server lifecycle and configuration

- Empty server startup creates the DB/migration ledger once, enables WAL and
  foreign keys, sets the bounded busy timeout, and persists after restart.
- Two concurrent starts against the same DB have one migration owner; the
  loser waits/fails safely without partial schema.
- Signal shutdown cancels long operations, drains within the configured
  deadline, closes DB/listeners, and exits without corrupting state.
- Invalid listen address, wildcard/mutable RP origin, production HTTP, invalid
  trusted proxy, unsafe file permissions, missing secrets, and unknown config
  keys fail before listening.
- `healthz`, `readyz`, and `version` enforce exact response schema/bounds and do
  not expose paths, counts, secrets, or DB internals.

### Migration matrix

| Scenario | Required result |
|---|---|
| empty -> current | ordered once; constraints/indexes/WAL/FK read back |
| current -> restart | no migration replay; data unchanged |
| exact prior schema -> current | data preserved and new defaults explicit |
| interrupted before commit | prior schema/data authoritative |
| interrupted after commit | new schema authoritative and restartable |
| partial/duplicate ledger | `schema_incompatible`, no repair-by-deletion |
| unknown future schema | startup refusal, zero writes |
| corrupt DB/FK violation | fail closed with safe error |
| busy/locked DB | bounded timeout/cancellation, no unbounded hang |
| verified backup restore | prior binary opens restored backup; no down migration |

### OpenAPI and middleware

- Schema validation, operation-ID uniqueness, `additionalProperties:false`,
  security coverage, response `apiVersion`, stable errors, UUIDv7 formats,
  cursor/list bounds, mutation headers, and examples are contract-tested.
- Pinned generators run twice from clean trees and byte-match. Regeneration to
  a temporary directory matches checked-in Go/TS artifacts. Altering the schema
  without generation fails CI.
- Generated Go compiles in server and client modes. Generated TS compiles under
  TypeScript 5.9/Node 24. `api:generate` runs exact `go tool oapi-codegen`
  (whose v2.8 validation is the schema validation pass) and pinned
  openapi-typescript; `api:verify` temp-generates/byte-compares. A golden fixture
  verifies enum/nullable/optional semantics agree across Go and TS.
- Compile-time coverage proves the first-party
  `packages/protocol/src/control-plane-client.ts` names every generated
  operation/`paths` entry. Tests prove same-origin base URL,
  `credentials:include`, JSON-only transport, in-memory CSRF injection, stable
  error mapping, AbortSignal propagation, 25-second long poll/30-second client
  timeout, and no arbitrary path. Web imports only its configuration module.
- Hostile JSON covers duplicate/unknown keys, excessive depth, invalid UTF-8,
  huge arrays/strings/numbers, malformed UUID/time/Base64url, content-type
  confusion, compressed-body bombs, slow body, header count/size, query
  duplication, and deadline cancellation.
- Idempotency stores exact response for same digest and rejects key reuse with a
  different digest. Entries expire safely. `If-Match` parsing rejects weak,
  wildcard, zero, malformed, and overflow revisions.
- Cursor tests cover endpoint/filter/sort binding, tamper, expiry, deletion
  between pages, stable tie-breaking, and no duplicate/skip within a frozen
  result ordering.

### P1 acceptance

The server runs only health/version/foundation handlers; no user or Device can
activate. Three-platform builds/tests, API drift, resolved license scan, DB
failure matrix, race tests, and secret scan pass.

## P2 — Bootstrap, Passkey, recovery, and session

### Verified Device identity prerequisite

- P2 applies `0008_control_plane_remote_identity.sql` before enabling any
  bootstrap handler. Upgrading the exact prior Device DB preserves every
  prefixed primary key/relation and creates only the separate
  `remote_device_identities` plus generic `controlplane_id_mappings` contract;
  receipt/sync tables remain the later 0009/0010 migrations.
- Golden envelope tests enforce <=4096-byte strict JSON and exact version,
  UUID, Ed25519 seed, X25519 private/public keys, full digests, key revision,
  lifecycle, and timestamps. Vault tests prove the reused Vault-v1 KEK,
  per-write random DEK, independent AES-256-GCM payload/wrap nonces/tags, exact
  framed AAD, plaintext/AAD digests, and quarantine on every mismatch.
- Failure injection proves envelope + public metadata + Device mapping commit
  before `bootstrap/options`; pre-commit leaves none and post-commit recovers
  the same pending identity. Create/open/mapping lookup and record-revision CAS
  work only while Vault is unlocked. No private key reaches DB/files/logs.
- The exact prepare/import/prove/verify/activate flow passes: prepare reuses the
  same pending envelope/mapping and emits only the public descriptor; the Web
  token/options step exports the public challenge; prove re-fetches and byte-
  matches it over configured HTTPS before signing; verify commits server state;
  activate re-fetches the public `BootstrapCommitReceiptV1` and atomically
  stores its digest while resealing unchanged keys pending -> active. Crash
  before/after server commit resumes the same identity. No activation secret,
  connection credential, or device-session material exists; later Device-auth
  PoP is separately required.
- Wrong AAD/private-public/digest/revision/status/size/JSON/tag, changed transfer
  object, wrong HTTPS origin/CA, receipt mismatch, locked Vault, concurrent CAS,
  or lost/replayed files fail closed. Pure-Web and mock-only envelope assertions
  cannot satisfy P2 acceptance.

### Bootstrap transaction

- Token plaintext is exactly 256 random bits and emitted once; restart retains
  only `SHA-256(token)`/expiry, compares it constant-time, and
  never prints it again.
- Lost/expired token recovery runs only through local `bootstrap rotate` with
  server stopped, exclusive DB lock, secure config/DB ownership, explicit
  `--confirm-uninitialized`, zero user/active anchor, atomic old-token/ceremony
  expiry + new digest + pre-user audit, and one post-commit print. Initialized,
  concurrent server, unsafe path, unknown schema, and network-reset attempts
  fail; lost stdout can only be recovered by another allowed rotation.
- Correct Daemon anchor + Passkey commits user, credential, recovery hashes,
  pin/anchor, audit, and browser session atomically; codes appear only in the
  successful response.
- Bootstrap verifies the signed `portable_vault_v1`/envelope assertion but UI/
  API calls it asserted, not server-proven. An official Daemon integration test
  exercises the P2 migration/store/application service, proves envelope+
  metadata+mapping were committed before options, and decrypts the exact local
  invariant after unlock.
- Wrong/expired/replayed token, expired challenge, wrong origin/RP ID,
  `kind=web|desktop`, missing key proof, digest mismatch, Vault-unbacked anchor
  marker, malformed credential, and concurrent second bootstrap leave zero
  partial active rows.
- Kill/failure injection before every transaction commit point proves either no
  active bootstrap state or the complete committed state after restart.
- The server ephemeral X25519 private key exists only in memory. Restart
  invalidates incomplete bootstrap/enrollment and stored SessionData; a valid
  unconsumed bootstrap token can begin a fresh ceremony. Proof/challenge/shared
  secret/HKDF/HMAC values are absent from durable/logged state.

### WebAuthn

- Registration/authentication fixtures verify challenge, type, origin, RP ID,
  user handle, allowed credential, UV requirement, signature, credential
  public key, and timeout.
- Full library `SessionData` is present only server-side, expires at five
  minutes, is atomically consumed once, and cannot be replayed across begin
  endpoints, users, sessions, origins, or restarts.
- Parallel finishes permit one result. A failed verification consumes the
  challenge. Cleanup does not delete a concurrently claimed ceremony.
- Counter cases cover increasing nonzero, consistent zero, transition to
  supported library state, nonzero regression/clone suspicion, and browser
  session revocation on regression.
- Fixed-origin deployment tests cover valid HTTPS, `localhost` development
  exception, wildcard/multiple/mutable origin rejection, untrusted forwarded
  host/proto, and trusted proxy normalization.

### Recovery, cookie, and CSRF

- Every browser endpoint is table-tested against its exact class: pre-auth
  mutations require Origin + same-origin Fetch Metadata + JSON + rate limit but
  no CSRF; authenticated mutations additionally require cookie + current CSRF;
  reads need cookie but no CSRF; signed Device APIs reject browser authority and
  need no CSRF. Missing/wrong Origin, fetch headers, content type, cookie, CSRF,
  and cross-class credential use fail before side effects.
- `GET /auth/current` and successful normal auth/UV/recovery-transition responses
  deliver a 32-byte no-store CSRF token; only its SHA-256 digest is stored.
  Frontend memory-only storage/injection passes, and login, UV/privilege change,
  recovery->normal, logout/expiry rotate or invalidate it.
- Recovery generation creates exactly ten independent 20-byte codes formatted
  `MAD-RC1-` + eight four-character Base32 groups. Parser accepts only exact
  prefix/hyphens and ASCII case normalization; whitespace, Unicode, ambiguous
  characters/separators, wrong length/groups fail. Each hash uses a unique
  16-byte salt and frozen Argon2 parameters. Plaintext is
  displayed once and absent from DB/log/audit/debug/error/metrics.
- Correct code atomically consumes once; duplicate/concurrent use has one
  winner. Wrong code, malformed code, source/account rate limit, restart, and
  cleanup use indistinguishable safe behavior and bounded Argon2 resource use.
- Recovery session has only passkey-management capability, lasts 15 minutes,
  and cannot read metadata or enroll/revoke/devices or rotate codes. Replacement
  Passkey success atomically creates a normal rotated session/CSRF, revokes all
  other browser sessions, and ends recovery; the old Passkey remains visible
  until an explicit recent-UV delete.
- Passkey list/delete and UV step-up cover redacted metadata, <=5-minute UV,
  current/other credential, concurrent deletes, and refusal to delete the last
  active Passkey. Recovery rotation requires normal recent UV, invalidates the
  prior batch atomically, returns ten plaintext values once, and exact
  Idempotency-Key replay returns `one_time_result_unavailable`; a new rotation
  remains available.
- Browser cookie attributes are `Secure`, `HttpOnly`, `SameSite=Strict`, and
  `Path=/`; session ID rotates; absolute/idle expiry and explicit/global logout
  revoke deterministically across restart.
- Every mutation tests missing/wrong CSRF, wrong Origin, cross-site fetch,
  simple form/content-type, replay after logout, and legitimate same-origin
  request. Read-only GET has no side effects.

### P2 acceptance

End-to-end bootstrap + login + logout + recovery + replacement Passkey passes
in supported browsers against HTTPS test origin only after the 0008/envelope/
mapping/Daemon-actor gate above passes. Pure Web initial anchor is rejected.
DB/log secret scans and auth concurrency/rate-limit tests pass.

## P3 — Device identity, enrollment, presence, revocation

### Reuse of the P2 identity foundation

- P3 begins only after P2 independently verifies migration 0008, the envelope
  store/API, Device mapping, and bootstrap pending-to-active lifecycle. A phase-
  isolation test fails if P3 adds, repairs, or mocks any of those prerequisites.
- Additional Daemon enrollment creates a new pending envelope/mapping through
  the same P2 API, then tests enrollment activation, active -> retired
  revocation, and replacement. Replacement always uses new keys/server UUID/
  enrollment; ambiguous replacement preserves old active/new pending state and
  never mutates key material under an existing UUID.
- Enrollment sets `snapshot_required`; retry/restart/concurrency reuses the one
  verified mapping, while type/UUID collision quarantines without rebinding.
  Device local IPC identity and Provider CredentialInstances remain unchanged.

### Web storage and Desktop boundary

- Chrome/Edge/Firefox native-path tests generate, persist across process
  restart, use, and reject export of Ed25519/X25519 keys.
- Safari/WebKit fixture/browser tests select `software_wrapped` only after
  native X25519 failure and prove AES wrapping key non-exportability, bounded
  raw-key lifetime, ciphertext tamper rejection, and user-visible downgrade.
- Failed/unknown probe selects `metadata_only`; it cannot submit key-bearing
  activation/approval or receive future E2EE capability.
- Site-data loss creates a new UUIDv7 identity and enrollment; it never silently
  recreates the old ID/key revision.
- Desktop `kind` server fixtures with valid keys pass; product code audit proves
  no Phase 4a Desktop keychain/store/client implementation was added. Shared
  Web build/render smoke passes in the Desktop shell.

### Enrollment, pin, and attestation

- Enrollment happy path proves both keys, displays six Base32 groups derived
  from the full digest, accepts an active directly pinned approver signature,
  activates once, and returns only the approver-signed public
  `ActivationReceiptV1` plus Device metadata. It returns no activation secret,
  connection credential, bootstrap token, or device-session material; later
  Device-auth challenge/PoP is mandatory.
- Full digest is stored; truncated fingerprint is never used as a key/digest or
  signature input.
- Negative cases: changed Device ID/key/key order/full digest/fingerprint/
  capability/expiry, invalid JCS, noncanonical capability order, unknown field,
  bad signature, invalid X25519 input/proof, unpinned/revoked approver,
  capability escalation, reused attestation/enrollment/challenge, ten-minute
  expiry, concurrent approval/activation, and existing-ID changed key.
- Fake compromised server-directory substitution cannot satisfy a local pin.
  Key change yields `device_key_changed` and a new enrollment requirement.
- Candidate Daemon `pair start`, anchor `pair approve`, and candidate `pair
  activate` application flows pass with exact HTTPS/CA handling, transcript
  fetch/recompute, candidate pin persisted before signing, approver-signed
  public ActivationReceipt, candidate receipt verification/approver pin before
  active state, and no activation secret/credential. Device-auth works later by
  key PoP alone. Web candidate follows the same DTO flow with Daemon approval.
- TTY requires exact six-group retype; noninteractive requires fingerprint +
  explicit confirmation and has no yes-bypass. Invalid input/auth/expiry/TLS/
  conflict/Vault cases return stable exit classes. Cancel/resume/retry/restart
  tests preserve public state, idempotency, and terminal expiry/cancellation.
- Capability parser preserves well-formed unknown strings but never grants/
  acts/delegates them; malformed strings fail. Kind/storage maximum matrices are
  exhaustive. Reserved 4b/5 names remain ineffective in 4a.
- Same-key capability elevation requires current full key digests, directly
  pinned eligible approver, explicit confirmation, recognized eligible complete
  set, signed `DeviceCapabilityAttestationV1`, and revision +1; stale/escalated/
  unknown/ineligible requests fail. Key revision/UUID do not change. Active Web
  native/software-wrapped may approve only if separately granted;
  metadata-only never activates/approves.

### Signed REST, presence, and revocation

- Signature canonicalization has Go/TS golden cases for method, normalized path
  and query, body digest, timestamp, nonce, API/device/session IDs.
- Mutation of every signed component, wrong Device/token/key, expired token,
  nonce replay, clock skew, body-after-sign, duplicate query, wrong capability,
  and revoked identity fails before handler side effects.
- Nonce insert and mutation concurrency allow one effect. Nonce storage and
  cleanup remain bounded under hostile load.
- Heartbeat alone after authentication marks online; unauthenticated socket/
  request never does. Online becomes offline after 60 seconds and after restart
  until a new authenticated heartbeat.
- Revocation transaction immediately invalidates sessions/challenges/claims,
  wakes/denies long polls, blocks sync/commands/new auth, and renders revoked.
  Previously copied data is not described as erased.

### P3 acceptance

Daemon bootstrap anchor plus second Daemon and Web enrollment pass; mismatch,
key change, replay, expiry, metadata-only, offline/restart, and revocation tests
pass. No WSS, HPKE, Pairwise Root, terminal/Approval/grant code path exists.

## P4 — Metadata projection and sync

### DTO allowlists and data leakage

- Golden round trips cover every allowed Account/Credential status/Profile/
  Workspace/Session/Usage field in OpenAPI, Go, DB, sync, and Web types.
- Unknown/additional fields are rejected. Direct and nested attempts to include
  `secretRef`, secret digest/body, token/cookie/auth, Provider/home/config path,
  workspace path, Provider session ID, terminal/model/Approval text, or
  secret-like environment keys fail before persistence.
- Response, DB, structured log, audit, metrics, trace, debug bundle, panic, and
  error scans use canary secrets/paths and find none.
- Usage tests cover every source/confidence/availability/capability enum,
  required observedAt/staleAt, missing optional window numbers, stale data,
  unavailable source, multiple windows, percentage bounds, and raw payload
  rejection. Missing is not rendered zero. Claude never receives fabricated
  official remaining quota.

### CRUD, pagination, revision, and conflicts

- Each list filter/sort allowlist has positive and unknown/operator/injection
  negatives. Pagination remains stable at equal sort values and page edges.
- Profile create/update/delete enforces Idempotency-Key and If-Match. Stale
  mutations return current allowed resource plus bounded field diff and make no
  write. Account/Credential/Workspace/Session/Usage browser mutation is denied.
- Concurrent updates at one revision have one winner. Retry returns prior
  result. Restart preserves revisions/idempotency.
- Browser Profile creation requires one active target Daemon and valid mapped
  Account/provider relations. Server allocates UUIDv7; only target Device
  allocates correct prefixed local Profile ID and commits projection+mapping in
  one transaction. Wrong owner/target/relation or other Device materialization
  fails.

### Push/pull/ack and tombstones

- P4 migration `0009_control_plane_sync.sql` upgrades the P2 Device DB without
  changing migration 0008 envelope/mapping rows; exact-prior/restart/failure/
  rollback-backup cases pass before sync handlers enable.
- Push commits valid typed batches, maps UUIDv7 relations, deduplicates batch
  and change IDs, and emits change log atomically. Duplicate at-least-once push
  creates no second row/revision.
- Cross-language Go/TypeScript golden vectors encode every exact
  `CanonicalSyncRevisionV1` type with RFC 8785 key/string/number rules and
  byte-match the domain-framed digest. Hostile vectors cover duplicate/unknown
  keys, optional-absent versus nullable, UTF-16 key order, no Unicode
  normalization, array order, `-0`, binary64 boundaries, unsafe integer,
  non-finite/lossy numbers, wrong discriminator/type/ID/revision, and the
  192-KiB bound.
- Create requires exactly `baseRevision=0`, `fullBase=null`, the domain-
  separated create sentinel, no revision-zero history row/live row/watermark,
  and `fullNext` upsert revision 1. Update/delete require a full typed base,
  base revision >=1, matching computed + lifetime history digest, and full next
  revision +1; delete is the canonical null-value delete revision. Missing
  history returns `sync_history_missing`, marks `snapshot_required`, and writes/
  advances nothing. Wrong base/next digest rejects the whole batch.
- Patch goldens prove root add/remove for create/delete; recursive fixed-object
  and dynamic-map keys in RFC 8785 order; RFC 6901 escaping; absent add/remove;
  scalar/type replacement; and atomic replacement of arrays at every nested
  depth. Client/server operations, subtree digests, and patch digest byte-match.
  Wrong order/path/digest/op, >128 operations, >256-byte path, or >16-KiB patch
  returns `sync_patch_mismatch|sync_patch_too_large` with zero writes. No
  implementation-selected array LCS is permitted.
- Stale valid bases return exact `fullBase/fullCurrent/fullNext`,
  `baseToCurrentPatch`, and `baseToNextPatch`; round-trip conflict fixtures for
  Profile maps/reference arrays, Workspace maps/tags, Session capability arrays,
  and nested Usage windows stay within the typed response bound and expose no
  forbidden field.
- Malformed/forbidden/relation/history/base/next/patch-invalid batch rejects
  all. Otherwise all nonconflicting changes apply atomically together,
  dependency conflicts join the conflict result, conflict rows emit no cursor,
  and full deterministic result dedupe replays exactly.
- Pull returns ordered bounded changes, supports 0..25-second long-poll, wakes
  on change/revocation/shutdown, cancels promptly, and redelivers until ack.
- Device applies inbox record + local projection + cursor in one transaction;
  crash before/after each commit point proves replay safety.
- Outbox ack cannot advance past an uncommitted local change. Offline backlog,
  reconnect, server restart, device restart, duplicate/reordered response, and
  cursor tamper are covered.
- Tombstone prevents stale resurrection; its frozen eligible set includes
  active devices at delete, offline devices block, newly enrolled devices do
  not join, and explicitly revoked devices stop blocking. Collection occurs
  only after all remaining eligible acks plus 30 days. Payload collection leaves
  lifetime deletion watermark. Deleted UUID create/upsert always fails; a new
  logical resource requires new UUID. Revoked/re-enrolled/restored stale outbox
  hits watermark and quarantines. Server backup missing/mixing watermark or
  revision-digest epochs fails schema compatibility. Boundary/race tests use a
  controllable clock.
- Initial enrollment, re-enrollment, and Device-backup restore require a scoped
  authoritative snapshot before incremental cursor. The authenticated/enrolled
  target Device plus matching envelope/mapping/pin/receipt is an out-of-band
  precondition and never appears in `CanonicalSyncRevisionV1`; golden topology
  starts Account -> Credential status -> Profile -> Workspace -> Session ->
  Usage and orders UUIDv7 within each rank.
- Independent Go/TypeScript goldens byte-match strict RFC 8785
  `SnapshotManifestV1`, `SnapshotPageDigestInputV1`, and
  `SnapshotFinalDigestInputV1` plus all three domain-separated digests. Fixtures
  cover one-resource, exact-page-boundary, multi-page, and the one empty final
  page; verify page slices/counts, null first prior digest, chained later priors,
  next-versus-final continuation, shared expiry/base cursor, and empty
  firstDigest=lastDigest.
- One-active-snapshot tests prove same-limit cursorless replay and every valid
  page-token replay have byte-identical `SyncSnapshotPage` data while allowing
  only the outer request ID to differ; changed limit conflicts; ten-minute
  expiry releases the slot. Mixed target/snapshot/epoch/manifest, reordered/
  omitted/duplicated/truncated page or resource, wrong prior/page/final digest,
  premature/absent final marker, token substitution, and incremental-cursor
  substitution all return typed errors and never install/ack a cursor.
- Commit tests require Device auth + Idempotency-Key and exact target/snapshot/
  epoch/manifest/final/last-page digests, counts, and cursor. First commit is
  atomic and exact replay returns the identical result through 24-hour
  retention; same key/changed body is `idempotency_key_reused`, while a fresh
  key/changed committed body is `snapshot_commit_conflict`. Staging/final atomic
  apply, dirty outbox replay,
  parent-before-child, missing parent, wrong type/target, mapping collision,
  cursor rollback, backup mapping preservation, and quarantine/block-commit
  paths are tested. Other Devices never materialize target-owned rows.

### P4 acceptance

Two Daemons plus Web exchange only allowlisted metadata across retry, conflict,
delete, offline, restart, and revocation scenarios. Local paths/secretRefs never
cross the network. Snapshot manifest/page/final cross-language goldens and all
continuity/expiry/replay/commit negatives pass. Usage UI data matches P1
semantics exactly.

## P5 — Asynchronous Session Commands

### State machine and HTTP behavior

- Creation validates mapped resource ownership/capability, returns 202 + UUIDv7
  command, persists before response, and remains queued when target is offline.
- Default/floor/ceiling TTL and exact boundary times pass with a fake clock.
- Legal transitions `queued -> claimed -> acknowledged -> terminal` pass;
  every skipped/backward/terminal mutation fails. Nonterminal commands expire.
- Claim is target-only, 30 seconds, one winner under concurrency, increments
  attempt, and only an unacked claim returns to queued after expiry. There is no
  claim token. Revoked/wrong Device cannot claim/ack/result/reconcile.
- Claim response and ack/result request golden tests bind Device, command ID,
  request digest, monotonic attempt, claim expiry, receipt digest/state and
  per-call Idempotency-Key. Ack requires current unexpired claim plus durable
  `reserved` receipt and freezes attempt; acknowledged never requeues. Stale
  attempt/digest/Device/expiry and skipped states fail. Duplicate calls return
  prior state; conflicting calls fail.
- Reaper/ack deterministic races prove one CAS winner: ack-first remains
  acknowledged; expiry-first records attempt N and the next winner is exactly
  N+1. Claim history retains the issued Device/command/digest/attempt needed for
  reconcile. Lost ack request, lost ack response, expiry before/while ack,
  repeated expiry, concurrent reclaim, restart, and stale client attempts have
  exact expected states.
- Long-poll limit/wait/cursor/cancel/backpressure and fairness across devices
  are bounded; no slow Device blocks another.

### Daemon idempotency and failures

- P5 migration `0010_remote_command_receipts.sql` preserves migration 0008/0009
  rows and adds the exact strict receipt/claim-reconciliation storage before
  command delivery enables. Go golden tests compute the domain-framed receipt
  digest over every listed field and receipt revision.
- Receipt state tests cover `reserved -> executing -> local_committed ->
  completed` and fail-closed `ambiguous`. Restart scans/resends ack/result from
  nonterminal rows. Duplicate delivery before execution, during execution,
  after local commit, after ack, and after result returns the same durable
  reservation/outcome.
- An expired old `reserved` receipt alone can CAS-rebind N -> current attempt,
  increment only receipt revision, retain byte-identical operation/Session/
  mapping/request identity, recompute its digest, and ack. Concurrent or stale
  rebind has one winner. Tests assert `executing|local_committed|ambiguous|
  completed` can never use this transition and no second reservation appears.
- Redelivery of old `executing` invokes no application service; proof of local
  commit moves to `local_committed`, otherwise it becomes `ambiguous`. Old
  `local_committed|ambiguous` reconciles only against a live current claim and
  verified immutable claim-history attempt; it atomically terminalizes from the
  stored outcome without execution. `reserved|executing` reconcile is rejected,
  ambiguous only yields `command_execution_ambiguous`, and a claim expiring
  during reconcile safely retries against the next attempt. Old `completed`
  requires matching server terminal state or quarantines as
  `command_receipt_inconsistent`.
- Failure injection at every receipt/local service/server result boundary proves
  exact ordering. `start|resume` reserve command binding + one local Session ID +
  server mapping before Provider work; other operations persist target/pre-state
  first. Executing restart reconciles durable local state; if external effect
  cannot be proved it records `ambiguous`, returns typed failure, and never
  auto-reexecutes. No exactly-once or ambiguous at-most-once claim is made.
- `start` maps only server UUIDs to existing local opaque IDs, uses the existing
  local service, and returns the mapped Session ID. It cannot carry a local
  path, binary path, secretRef, raw settings, terminal input, or Provider body.
- Stop/kill/resume/acquire/release respect existing local authorization,
  capability, Session state, and typed Provider unsupported behavior.
- Offline/locked Vault/disabled Profile/revoked credential/unknown mapping/
  stale Session/unsupported resume/daemon shutdown return stable safe results
  without optimistic success.
- At-least-once tests intentionally redeliver. Documentation/UI contains no
  `exactly once` claim.

### P5 acceptance

Web/API creates a command, target Daemon later claims/acks/results it, and
claim expiry/rebind/reconcile plus restart/retry cannot duplicate local
execution. Queued/offline/expired/failed/unsupported/ambiguous states remain
truthful. No terminal or Approval transport exists.

## P6 — Web metadata UI and final integration

### Functional pages

| Surface | Required states/actions |
|---|---|
| Bootstrap | token entry, Passkey registration, Daemon-anchor fingerprint, one-time recovery display/confirm; no pure-Web completion |
| Passkey/Recovery | login, CSRF rotation, UV step-up, Passkey list/delete/last-key guard, exact ten-code display/consume/rotate/one-time replay, recovery restriction and replacement transition |
| Overview | device/account/session/usage summaries with freshness/offline and no fabricated values |
| Devices | pending/online/offline/revoked, six-group mutual confirmation, start/approve/activate/cancel/resume, public activation receipt, capability elevation, storage-mode downgrade |
| Accounts | provider/auth availability metadata only; no token/auth import |
| Profiles | list/create/edit/delete with conflict/validation/If-Match handling |
| Sessions | metadata and start/stop/kill/resume command state; no terminal/controller UI |
| Usage | source, confidence, availability, observed/stale times, window values, unavailable/unknown |

Every page covers loading, empty, partial, error, offline, revoked, stale,
conflict, retry, and unauthorized/session-expired states. The bundle contains no
Terminal, xterm.js, Approval response, CredentialGrant, Pairwise Root, HPKE,
Provider plaintext, or generic secret input.

### Responsive, accessibility, and browser gates

- Layout is tested at 320, 375, 768, 1024, and 1440 CSS pixels, 200% zoom, long
  labels, reduced motion, high contrast, dark/light modes, and touch target
  sizes. No essential horizontal scroll exists except bounded data tables with
  an accessible alternative.
- Keyboard-only flows cover navigation, dialogs, tables, forms, recovery-code
  confirmation, fingerprint comparison, conflict resolution, and focus return.
- Automated axe has zero serious/critical findings; semantic headings,
  landmarks, labels, descriptions, live regions, error association, status not
  conveyed by color alone, and screen-reader names are manually checked.
- Current Chrome, Edge, Firefox, and Safari run auth/metadata flows. Key-storage
  behavior must match runtime probe, not user-agent guesses. PWA offline shell
  never presents stale data as current or queues a security mutation invisibly.
- CSP, Trusted Types where supported, no unreviewed third-party scripts, SRI/
  dependency lock, XSS payload encoding, open redirect, clickjacking,
  autocomplete, cache, and browser-storage inspection gates pass.
- Desktop builds/renders the same metadata UI and passes a smoke test only; no
  Desktop enrollment/keychain/product capability claim is made.

### End-to-end exit matrix

1. Fresh HTTPS server bootstraps atomically with a portable-Vault-v1 Daemon
   anchor, Passkey, and recovery codes.
2. User logs in; an unapproved/new Web origin sees permitted metadata but has no
   key-bearing/realtime capability.
3. Second Daemon and Web Device complete fingerprint/attestation enrollment;
   key mismatch/replay/expiry fail.
4. Presence is truthful across disconnect/restart; revoke closes access and
   prevents new signed REST operations.
5. Account/Profile/Workspace/Session/Usage metadata syncs between two Daemons
   and Web with idempotent replay, conflict diff, tombstone, offline recovery,
   and no prohibited fields.
6. Web creates Fake Session Commands; offline queue, claim, ack, success,
   failure, unsupported, expiry, daemon restart, and duplicate delivery behave
   according to the state machine.
7. Control Plane shutdown leaves existing local CLI/Daemon Sessions running;
   Web shows offline and does not imply local termination.
8. Database/log/audit/debug/artifact scans find no Provider credential,
   secretRef, real workspace path, terminal/model content, recovery plaintext,
   cookie/CSRF value, challenge, or private Device key.

### Final security evidence

- Update threat model with bootstrap-token theft/partial bootstrap, Passkey
  counter/recovery abuse, CSRF/origin/proxy confusion, Device signature replay,
  fingerprint truncation/human error, Web key storage/XSS, metadata leakage,
  sync resurrection/tombstone starvation, command replay/claim races, long-poll
  exhaustion, migration rollback, and dependency/codegen supply chain threats.
- Fuzz parsers/canonicalization/cursors/JCS/signatures/OpenAPI inputs; run race
  suites repeatedly on sync/command/revocation/cleanup; run hostile resource
  limit tests; preserve failure evidence.
- Run full repository, three-platform, browser, dependency/license, secret
  scan, migration/restart/concurrency/failure, documentation/link, and project
  structural gates against the final phase diff.

## Explicit exclusions from a pass

- A generated build alone is not browser accessibility or security evidence.
- Passing HTTP tests is not proof of human fingerprint comparison.
- Signed REST metadata is not E2EE terminal/realtime support.
- Web Device key generation/enrollment is not Pairwise Root or decryption
  support.
- Desktop server-kind fixtures/build smoke are not Desktop keychain/client,
  packaging, signing, or platform acceptance.
- At-least-once delivery plus daemon idempotency is not distributed exactly
  once.
- Fake Session command integration is not a live Provider compatibility claim.
- Security tests do not resolve the open Security Gate; only the independent
  `security-review` verdict can do that.
