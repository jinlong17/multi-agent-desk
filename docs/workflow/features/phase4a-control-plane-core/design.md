# Design: Phase 4a Control Plane Core

## Decision snapshot

- **Owner:** `control-plane`; secondary impacts are `security`, `core`, `web`,
  `desktop`, and `project-system`.
- **Product boundary:** Phase 4a delivers one self-hosted, single-user Control
  Plane, authenticated metadata REST, device enrollment/presence/revocation,
  revisioned metadata sync, asynchronous Session Commands, and a metadata-only
  Web surface. It does not deliver terminal/events WSS, Pairwise Roots, HPKE,
  Approvals, Credential Grants, or Provider plaintext.
- **Transport:** HTTPS REST only. Daemons use signed authenticated
  pull/push/ack/heartbeat requests and bounded long-poll. Delivery is
  at-least-once; daemon-side durable idempotency prevents duplicate execution.
  Phase 4a never claims exactly-once execution.
- **Identity IDs:** every identifier created for the server or placed on the
  network is a canonical UUIDv7. Existing local prefixed opaque IDs remain
  unchanged. A durable device-side mapping binds each local opaque ID to one
  UUIDv7; migrations never rewrite existing primary keys.
- **Trust anchor:** a pure Web client cannot bootstrap trust. The initial anchor
  is a Daemon whose newly generated remote Ed25519 signing and X25519 exchange
  private keys are encrypted in the already-shipped portable password-derived
  Vault v1. That Vault is not OS-backed. OS Keychain, DPAPI/Credential Manager,
  and Secret Service wrapping remain Phase 5.
- **Client key boundary:** Phase 4a pulls forward Web Device Ed25519/X25519
  generation, ADR 0010 storage-mode probing/storage, proof of possession, and
  metadata-only enrollment/revocation. Pairwise Roots, HPKE, realtime payloads,
  and decryption stay in Phase 4b. The server accepts and contract-tests
  `kind=desktop`, but the Desktop product keychain client stays in Phase 5.
- **Pin display:** the full domain-separated SHA-256 pin digest is stored and
  attested. Humans compare only its first 120 bits, encoded as six groups of
  four uppercase unpadded Base32 characters. Truncation is presentation only.
- **API authority:** `api/openapi/control-plane-v1.yaml` is the sole REST type
  authority. Generated Go and TypeScript artifacts have fixed paths and a
  regeneration drift gate.
- **Authentication:** Passkeys authenticate a user; Device keys authenticate a
  Device. Full WebAuthn `SessionData` remains server-side, bounded, expiring,
  and atomically one-shot. It is never serialized to the browser.
- **Phase discipline:** P0 through P6 are built one at a time. Each build stops
  at `READY_FOR_VERIFY`; an independent `feature-verify` must verify that phase
  before the next build starts. The final Security Gate is open.

## Baseline and reconciliation

Planning is based on `origin/main@e3578390a23ddcf805ceb0bad24b1c41d36977fb`
and the committed intake at
`71e0448de1624ae3c00cec82f800d0e5425a4dc5`. The current checkout contains:

- prefixed local `domain.ID` values such as `device_<32 hex>`;
- a shipped portable Argon2id/AES-GCM Vault v1;
- only an Ed25519 local-IPC Daemon identity, not the remote signing/exchange
  identity required here;
- Device migrations through `0007`, an empty `migrations/server` scaffold,
  placeholder Control Plane/Web packages, and no public OpenAPI contract;
- P1 Usage metadata with optional source/confidence/availability/freshness and
  window values.

P0 must reconcile authoritative documentation before product implementation:

1. Update the implementation plan, ADR 0010/0011, E2EE protocol specification,
   vectors/harnesses, data model, and threat model so the pin digest remains a
   full 256-bit domain-separated value while the human fingerprint is exactly
   its first 120 bits in six four-character Base32 groups.
2. Freeze `DeviceAttestationV1` as JCS (RFC 8785) signed bytes containing the
   full signing-key digest, full exchange-key digest, canonical capabilities,
   issued/expiry timestamps, subject/approver IDs, attestation ID, type, and
   version. The signature input is domain-separated and length framed. Reuse
   and upgrade the existing independent Go/TypeScript restricted RFC 8785
   vector codecs for this typed subset (known members, strings, non-negative
   integers, and arrays only); floats, arbitrary maps, duplicate/unknown keys,
   and build-time canonicalizer selection are forbidden.
3. Replace every claim that the initial anchor must already be OS-keychain
   backed with the executable portable Vault v1 Daemon path. Preserve the
   prohibition on pure-Web bootstrap and preserve Phase 5 OS wrapping.
4. Record the exact Phase 4a/4b/5 split and remove Phase 4a WSS implications.
5. Freeze UUIDv7 network IDs without invalidating the existing local ID
   contract or silently rewriting installed databases.

P0 changes protocol vectors because the old vector displays all 256 digest
bits as hexadecimal and the old attestation object does not carry both full
digests. P0 does not implement Pairwise Roots or HPKE production code.

## Components and ownership

### Control Plane server

`cmd/multidesk-server` owns validated configuration, graceful lifecycle,
health/version handlers, HTTP bounds/timeouts, and the application wiring.
`internal/controlplane` owns bootstrap/auth, sessions, device directory,
metadata projections, sync, commands, audit, and retention. `internal/transport`
owns reusable HTTP signature/canonical-request and middleware mechanics, not
business authorization. `migrations/server` owns only the server database.

The server uses one SQLite database in WAL mode with foreign keys enabled, a
bounded busy timeout, ordered forward migrations, an exclusive migration lock,
and refusal of unknown future schema versions. Cleanup jobs operate in bounded
batches and cannot delay request shutdown indefinitely.

### Device integration

`core` extends the local Device store without changing existing IDs:

- `controlplane_id_mappings(entity_type, local_id, server_id, created_at,
  updated_at)` has unique `(entity_type, local_id)` and unique
  `(entity_type, server_id)` constraints. `server_id` is UUIDv7. A new mapping
  is committed before its first network operation and then reused forever.
- P2 Device migration `0008_control_plane_remote_identity.sql` creates both a
  separate `remote_device_identities` table and the generic
  `controlplane_id_mappings` table before any bootstrap handler is enabled. It
  does not overload credential-bound `vault_items`. Each identity row stores
  the local opaque Device ID, server UUIDv7, public keys/full digests,
  `key_revision`, CAS `record_revision`, lifecycle, Vault-v1 payload/wrap
  algorithm names, independent nonces/ciphertexts, AAD digest, plaintext
  digest, and timestamps. P2 uses the mapping table for the anchor Device;
  later resource types reuse the already-verified table without changing its
  uniqueness contract.
- P3 migration `0009_remote_device_trust.sql` adds local peer pins, activation
  receipts, and remote-identity lifecycle evidence without sync behavior.
- P4 migration `0010_control_plane_sync.sql` adds sync outbox/inbox and
  acknowledged cursors; they remain device-local and commit atomically with
  local projection/mapping changes.
- P5 migration `0011_remote_command_receipts.sql` adds
  `remote_command_receipts`. It durably stores command/target/request digest,
  immutable local operation/session identity, claim attempt, receipt revision,
  reconciliation state, safe result, and timestamps. Its state machine is
  frozen below.

The remote identity can be generated and used only while portable Vault v1 is
unlocked. Existing explicit headless auto-unlock policy remains an operator
choice; Phase 4a does not create an OS-keychain dependency or silently weaken
the Vault.

#### `DeviceKeyEnvelopeV1`

The strict UTF-8 JSON plaintext is at most 4096 bytes, rejects unknown or
duplicate fields, and is exactly:

```text
DeviceKeyEnvelopeV1 {
  version: 1
  serverOrigin: CanonicalServerOriginV1
  serverDeviceId: UUIDv7
  ed25519Seed: base64url(32)
  x25519PrivateKey: base64url(32)
  signingPublicKey: base64url(32)
  exchangePublicKey: base64url(32)
  signingKeyDigest: base64url(SHA-256(signingPublicKeyRaw))
  exchangeKeyDigest: base64url(SHA-256(exchangePublicKeyRaw))
  keyRevision: 1
  status: pending | active | retired
  createdAt: RFC3339 UTC
  updatedAt: RFC3339 UTC
}
```

The existing Argon2id-derived portable Vault-v1 KEK is reused; no second
password/KDF is introduced. Every seal/reseal generates a random 32-byte DEK,
encrypts the payload with AES-256-GCM, and wraps that DEK with AES-256-GCM under
the KEK using independent random 12-byte nonces. Ciphertexts include their
16-byte tags. Canonical length-framed AAD is:

```text
frame("multidesk-device-key-envelope-v1", "1", serverOrigin,
      localRemoteIdentityId, serverDeviceId, signingKeyDigestRaw,
      exchangeKeyDigestRaw, decimalKeyRevision)
```

The row stores SHA-256 of the AAD and plaintext. Open recomputes public keys
from private material, recomputes both full digests/AAD/plaintext digest, and
requires exact row/payload/CAS revision agreement. Any mismatch, AEAD failure,
bad length/JSON/status, or relation mismatch fails closed, marks the row
quarantined in a separate safe error field when possible, and never signs.

P2 generation commits the envelope, public metadata, `pending` lifecycle, and
the local-to-server Device mapping in one `BEGIN IMMEDIATE` transaction before
`bootstrap/options` can run. The P2 build gate does not expose bootstrap until
migration `0008`, envelope create/open, mapping lookup, record-revision CAS, and
the pending-to-active bootstrap transition all pass. Crash before commit leaves
no identity; crash after commit recovers the same pending identity. Bootstrap
activation reseals the identical key material as `active` with a fresh
DEK/nonces and record-revision CAS in the same transaction that stores the
public `BootstrapCommitReceiptV1`. P3 reuses the same API for enrollment and
extends it to revocation/replacement. Revocation/replacement reseals as
`retired`; retired keys cannot sign.
There is no in-place key-material mutation or `keyRevision > 1` in v1. Key
replacement generates new keys, a new server Device UUIDv7/mapping, and normal
re-enrollment, then retires the old row only after the new identity is durable.
Ambiguous partial replacement leaves the old identity active and the new one
pending; it never swaps keys under an existing server UUID.

### Web and Desktop

`web` owns the React/PWA application. `openapi-typescript` generates types only;
it does not generate a runtime client. `packages/protocol/src/control-plane-client.ts`
is the first-party typed runtime client and is exhaustively tied to the
generated `paths`/operations. It exposes named operations only, always uses
same-origin `credentials: include`, JSON, safe error mapping, in-memory CSRF
injection, caller `AbortSignal`, and a bounded long-poll timeout; it has no
arbitrary-path escape hatch. `apps/web/src/api/control-plane.ts` is the sole Web
configuration/composition point. No `openapi-fetch` dependency is added.

Before enrollment, an authenticated Web origin is metadata-capable only. It
generates Ed25519/X25519 keys and runs the ADR 0010 storage probe:

- `native` stores reusable non-exportable WebCrypto keys;
- `software_wrapped` wraps the software X25519 private key with a
  non-exportable AES-256-GCM key and shows the downgrade;
- `metadata_only` cannot complete key-bearing enrollment, approve another
  Device, or receive any Phase 4b capability.

Phase 4a uses these keys for proof of possession and signed enrollment metadata
only. No Pairwise Root or protected payload is created. Site-data/key loss
creates a new UUIDv7 Device identity and requires re-enrollment.

`desktop` is limited to shared Web build/render smoke and server contract
fixtures for `kind=desktop`. Phase 4a must not add the real Desktop private-key
store, sidecar lifecycle, packaging, or OS Keychain integration.

### Versioned capabilities and later elevation

Capability strings match
`^mad\.v[1-9][0-9]*\.[a-z][a-z0-9]*(?:\.[a-z][a-z0-9_]*)+$`.
Parsers preserve well-formed unknown strings in `declaredCapabilities` for
forward compatibility, but only the current server's explicit allowlist enters
`effectiveCapabilities`; unknown/reserved strings are never granted, delegated,
or acted upon. Phase 4a reserves 4b realtime/decrypt/terminal/Approval and Phase
5 credential-store/grant names in documentation, but they stay ineffective
until an upgraded server recognizes them.

The kind/storage matrix is closed in `api.md`. Daemon +
`portable_vault_v1` can receive metadata/sync/presence/enrollment/revocation/
command-delivery capabilities. Active Web + `native|software_wrapped` can
receive metadata/Profile/command-create and, only by explicit grant,
enrollment-approve/revoke. `metadata_only` can read metadata as an authenticated
browser but is not an active key-bearing Device and cannot approve/elevate.
Desktop capability rows are server fixtures only in Phase 4a.

Same-key capability elevation uses a new signed
`DeviceCapabilityAttestationV1`: exact current key digests, sorted complete
proposed capabilities, directly pinned approver, explicit user confirmation,
`previousCapabilityRevision`, `capabilityRevision = previous + 1`, issued/
expiry, IDs, type/version, and restricted JCS signature. The server requires
every capability to be recognized and eligible for kind/storage/delegation.
Elevation changes neither `keyRevision` nor Device UUID; key replacement still
requires a new identity/re-enrollment. Future 4b/5 builds must use this flow
after their server version recognizes the reserved name.

## Cryptographic identity and enrollment

### Pin digest and fingerprint

All byte fields below use raw bytes, not textual encodings, inside the framing
function. `frame` is the existing uint32-big-endian-length framing from the
E2EE protocol:

```text
pinDigest = SHA-256(frame(
  "multidesk-device-pin-v1",
  deviceId,                 // canonical lowercase UUIDv7 text
  signingPublicKeyRaw,      // 32 bytes
  exchangePublicKeyRaw      // 32 bytes
))
fingerprint = Base32NoPadding(pinDigest[0:15])
display = fingerprint[0:4] + "-" + ... + fingerprint[20:24]
```

The stored `pinDigest` is always all 32 bytes. The fingerprint is uppercase
RFC 4648 Base32, exactly 24 characters plus five display hyphens. Parsers
normalize display hyphens/case only for comparison and then compare decoded
15 bytes in constant time.

### Attestation

`DeviceAttestationV1` contains canonical UUIDv7 IDs, both full Base64url
SHA-256 key digests, a sorted/deduplicated capability array, `issuedAt`,
`expiresAt`, `type=device_attestation`, and `version=1`. It may also carry raw
public keys outside the signed object for lookup, but verification recomputes
both digests from the submitted 32-byte keys and compares them to the signed
digests. The approver signs:

```text
Ed25519.Sign(approverPrivateKey,
  frame("multidesk-device-attestation-v1", JCS(attestation)))
```

The enrollment verifier requires proof of possession for both subject keys,
an active directly pinned approver, capability delegation subset, a maximum
ten-minute attestation lifetime, unconsumed enrollment/attestation IDs, exact
digest matches, and an unrevoked subject. Server directory keys never satisfy
the local pin check.

### Signing and exchange-key proof of possession

Ed25519 proof is a signature over the transcript below. X25519 cannot sign, so
bootstrap and enrollment use the same exact ephemeral Diffie-Hellman proof:

```text
popContext = frame(
  "multidesk-x25519-pop-context-v1", apiVersion,
  purpose,                       // "bootstrap" or "enrollment"
  ceremonyId, subjectDeviceId,
  subjectSigningPublicKeyRaw, subjectExchangePublicKeyRaw,
  storageMode, storageAssertionDigestRaw,
  serverEphemeralX25519PublicKeyRaw,
  challengeRaw, expiresAtRFC3339UTC
)
sharedSecret = X25519(subjectExchangePrivateKey,
                      serverEphemeralX25519PublicKey)
popSalt = SHA-256(frame("multidesk-x25519-pop-salt-v1",
                       ceremonyId, challengeRaw))
popKey = HKDF-SHA256(sharedSecret, popSalt, popContext, 32)
exchangeProof = HMAC-SHA256(popKey,
  frame("multidesk-x25519-pop-proof-v1", popContext))
signingProof = Ed25519.Sign(subjectSigningPrivateKey,
  frame("multidesk-ed25519-pop-proof-v1", popContext))
```

The server creates a fresh ephemeral X25519 pair and 32-byte challenge for each
ceremony, retains the private key in memory only, rejects an all-zero shared
secret or any mismatch, uses constant-time proof comparison, and erases the
private key on consume/expiry best-effort. The server never persists/logs the
ephemeral private key, shared secret, pop key, proof, or challenge. Server
restart invalidates all incomplete bootstrap/enrollment ceremonies because the
ephemeral private keys are gone; their stored WebAuthn SessionData cannot be
reused. A still-valid bootstrap token may begin a new ceremony.

### Enrollment actors and durable activation

The Daemon CLI application-service flow is fixed:

1. Candidate runs `multidesk devices pair start --server <https-url>
   [--ca-file <absolute-file>]`. It loads or atomically creates its pending
   remote envelope/mapping, submits keys/storage assertion/capabilities, and
   prints enrollment ID, expiry, and its six-group fingerprint.
2. A directly pinned anchor runs `multidesk devices pair approve
   <enrollment-id> --server ...`. It fetches the complete pending transcript,
   recomputes the full digest/fingerprint, displays candidate kind/name/keys/
   capabilities/expiry, and requires the operator to retype the exact six-group
   fingerprint. It commits the candidate pin locally before signing/sending the
   attestation and public `ActivationReceiptV1`.
3. Candidate runs `multidesk devices pair activate <enrollment-id>`. It proves
   both keys, obtains the public receipt, verifies the approver signature and
   exact enrollment/transcript/digests/capabilities, displays/requires the
   approver fingerprint, then atomically pins the approver and marks its
   envelope active before acknowledging activation.

`ActivationReceiptV1` contains version/type, enrollment/subject/approver IDs,
both parties' public-key digests, request/attestation digests, granted
capabilities and capability revision, activated/expiry timestamps; the
approver signs its restricted JCS bytes. It contains no secret or connection
credential. Later `/device-auth/challenges` key PoP is the only way to obtain a
short device session.

A Web candidate performs start/prove/activate in the Devices UI using its ADR
0010 keys; the Daemon CLI approves. Web verifies/stores the same public receipt
and approver pin before active state. `metadata_only` cannot enter this flow.
All start/prove/approve/activate/cancel/resume operations require
Idempotency-Key and return the existing state for an exact replay. `resume`
returns the public transcript/state only; it never returns a private value.
Candidate or authorized approver can cancel before activation; terminal
cancelled/expired enrollments cannot reactivate.

Production URLs must be HTTPS with exact configured host/RP deployment; an
optional CA file must be an absolute owner-readable regular file and never
disables hostname verification. TTY approval requires interactive retyping.
Noninteractive approval requires both `--fingerprint <six-groups>` and
`--confirm-fingerprint`; no `--yes` bypass exists. Stable CLI exit classes are:
`0` success, `2` input/fingerprint, `3` auth/pin/capability, `4` expired/
cancelled, `5` network/TLS, `6` state/replay/conflict, `7` Vault/local-store.

### Bootstrap

Server startup creates 32 random bytes of bootstrap material, prints Base64url
plaintext exactly once, and stores exactly its SHA-256 digest plus a ten-minute
expiry. Validation decodes exactly 32 bytes, hashes them, and constant-time
compares the digest. The 256-bit random token does not use a password hash.
Successful commit deletes the digest; expiry/replay fails without partial state.
The bootstrap flow has two calls:

1. `bootstrap/options` validates the token, records full go-webauthn
   `SessionData`, creates the in-memory server ephemeral X25519 key and
   challenge, binds expected fixed RP ID/origin, and returns the public
   transcript fields with a one-shot ceremony UUIDv7.
2. `bootstrap/verify` consumes that ceremony and atomically creates the single
   user, Passkey, hashed recovery codes, Daemon anchor directory row/pin,
   proof-of-possession record, audit facts, and initial browser session; then it
   deletes the bootstrap-token hash. Recovery plaintext is returned once only
   after commit.

The anchor request must be `kind=daemon`, include newly generated Ed25519 and
X25519 public keys whose private halves are already committed to the unlocked
portable Vault v1, and prove possession of both using the exact transcript.
Its signed PoP transcript contains `storageMode=portable_vault_v1`. This is an
asserted client fact: PoP proves key possession, not encryption at rest. The
server records the assertion but never describes it as server-verified.
Official Daemon integration tests that inspect the local envelope transaction
are the evidence for at-rest storage. `kind=web` and `kind=desktop` are rejected
for the initial anchor. Failure, timeout, replay, or concurrency leaves no
active user or anchor.

#### Exact Daemon/bootstrap actors

Bootstrap is an operator-mediated Browser + Daemon ceremony; no browser can
invent the anchor and no private key or activation credential crosses between
them:

1. With Vault v1 unlocked, the operator runs `multidesk devices bootstrap
   prepare --server <https-url> --out <absolute-file> [--ca-file
   <absolute-file>]`. P2 first applies migration `0008`, then atomically creates
   or reuses the pending envelope and Device mapping. The owner-only output is
   a public `BootstrapAnchorDescriptorV1` containing server origin, Device ID,
   public keys/digests, storage assertion, capabilities, and fingerprint; it
   contains no token, proof, or private material.
2. The operator imports that descriptor into the same-origin Bootstrap Web
   page and enters the bootstrap token there. Web calls `bootstrap/options`,
   creates the Passkey options, and exports the returned public
   `BootstrapAnchorChallengeV1` for the Daemon. The token never enters the
   descriptor or Daemon database.
3. The operator runs `multidesk devices bootstrap prove --challenge
   <absolute-file> --out <absolute-file>`. Before signing, the CLI fetches the
   same ceremony transcript from the configured HTTPS origin, requires byte-
   identical IDs/keys/storage assertion/challenge/expiry, opens the pending
   envelope, and produces both PoP values. It never changes envelope state.
4. Web imports the public proof, performs WebAuthn, and calls
   `bootstrap/verify`. The server atomically commits the user, Passkey,
   recovery hashes, anchor directory row/pin, session, audit, and a public
   `BootstrapCommitReceiptV1`. The response displays recovery plaintext once
   and exports the receipt; replay can return only the public receipt/state.
5. The operator runs `multidesk devices bootstrap activate --receipt
   <absolute-file>`. The CLI re-fetches the committed receipt over the exact
   configured HTTPS origin, matches ceremony/Device/key/assertion/proof
   digests, and atomically stores its digest while resealing the same pending
   key material as `active`. Only then may later Device-auth PoP run.

The challenge/proof/receipt transfer objects are public, strict, bounded JSON;
owner-only files and the re-fetch checks prevent local substitution from being
silently accepted. `BootstrapCommitReceiptV1` is a TLS-authenticated server
commit fact, not a Device attestation or new trust anchor. There is no
activation secret, connection credential, or bootstrap device-session
material. A crash before step 4 leaves the same pending envelope; a crash after
server commit but before step 5 re-fetches the same public receipt and completes
the CAS transition. Mismatch quarantines the pending row and never signs.

#### Lost/expired bootstrap-token recovery

The only reset surface is local CLI:

```text
multidesk-server bootstrap rotate --config <absolute-path> \
  --confirm-uninitialized
```

It is never an HTTP/API operation. The command requires the server to be
stopped, secure owner-only config/database paths, the explicit confirmation
flag, and an exclusive database/process lock. In one transaction it verifies
there is no active user and no active anchor, expires any previous bootstrap
token and incomplete ceremonies, records a redacted pre-user audit event, and
stores SHA-256 of one new random 32-byte token with a ten-minute expiry. Only
after commit does it print the Base64url plaintext once to stdout. If stdout is
lost, the operator must run rotate again; the old/new plaintext is never
recoverable. It refuses an initialized database, a running/concurrent server,
unsafe ownership/permissions, an unknown schema, or a missing confirmation.
An expired/lost token is therefore recoverable before initialization without
creating a network reset or a post-bootstrap account takeover path.

### Passkeys, recovery, browser sessions, and CSRF

Phase 4a pins `github.com/go-webauthn/webauthn v0.17.4`: the 2026-05-22 release
tag is GitHub-verified, BSD-3-Clause, declares Go 1.25 and toolchain 1.26.3,
and is compatible with the repository Go 1.26.5 line. Full `SessionData` is
stored in a bounded server row for at most five minutes. Begin creates it;
finish consumes it once in the same transaction that commits the credential or
assertion result. Failed verification also consumes the challenge.

Production config requires one fixed HTTPS origin and RP ID; wildcard origins,
runtime origin mutation, IP RP IDs, and non-HTTPS origins fail startup.
`localhost` is an explicit development-only mode. User verification is
required. A nonzero authenticator counter that regresses triggers
`passkey_counter_regressed`, revokes existing browser sessions, and requires a
different Passkey or Recovery Code; authenticators that consistently use zero
remain supported and are not presented as clone-detected.

Each recovery batch has exactly ten codes. Each code is 20 random bytes encoded
as 32 uppercase unpadded RFC 4648 Base32 characters and displayed as
`MAD-RC1-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX-XXXX`. Parsing removes only the
exact prefix and seven group hyphens, folds ASCII case, and rejects whitespace,
Unicode confusables, alternative separators, wrong group/count/length, or
non-Base32 input. Each row has a random 16-byte salt and Argon2id (`time=3`,
`memory=64 MiB`, `parallelism=1`, 32-byte output). Verification is globally and
per-source rate limited and always uses the same-cost path.

Atomic consumption creates a 15-minute restricted recovery session with its
own cookie/CSRF pair that can only register a replacement Passkey. Successful
recent-user-verification
registration atomically creates a normal rotated browser session/CSRF value,
revokes every other browser session, and ends the recovery session; only then
may the user list/delete an old Passkey. A normal Passkey session with recent UV
(at most five minutes old) may rotate recovery codes: the transaction
invalidates the prior batch and commits ten new hashes before returning the ten
plaintext codes once. Exact idempotent replay returns
`one_time_result_unavailable`, not the plaintext; the authenticated user can
perform another recent-UV rotation. Passkey deletion also requires recent UV
and can never remove the last active Passkey.

Normal browser sessions use a random 256-bit server-side token, store only its
SHA-256 digest, rotate on authentication/privilege change, expire after 12
hours absolute and 30 minutes idle, and use a `Secure`, `HttpOnly`,
`SameSite=Strict`, `Path=/` cookie. A session also creates a random 32-byte CSRF
value and stores only SHA-256 of it. `GET /auth/current` and successful normal
authentication responses return the raw CSRF value in `Cache-Control:no-store`
JSON; the Web client holds it in memory only and sends `X-CSRF-Token`.
Login, recovery-to-normal transition, and any privilege/session rotation issue
a new CSRF value. Exact Origin, same-origin Fetch Metadata, JSON content type,
cookie, and CSRF checks follow the endpoint matrix in `api.md`. Pre-auth browser
mutations have no CSRF/session yet, but still require exact Origin, same-origin
Fetch Metadata, JSON, and strict rate limits. Signed Device APIs never use CSRF.
Cookies, CSRF values, challenges, and recovery plaintext never enter
logs/audit payloads.

## Device authentication and Phase 4a transport

An enrolled Device obtains a short-lived device session only after a server
challenge and an Ed25519 proof. Each subsequent request carries that token and
a signature over the versioned canonical request:

```text
frame("multidesk-device-request-v1", apiVersion, deviceId, method,
      canonicalPathAndQuery, contentSHA256, timestamp, nonce, sessionId)
```

The server enforces an exact method/path/query allowlist, a five-minute clock
window, unique 128-bit nonce within the session, body digest, request/body
bounds, revocation, and endpoint capability before dispatch. Token or signature
alone is insufficient. Nonce insertion and mutation commit atomically when
required; replays return a stable error without re-running the mutation.

There is no Phase 4a WebSocket. Daemons use:

- `POST /v1/device/presence/heartbeat` every 20 seconds; presence becomes
  offline after 60 seconds and remains advisory.
- `POST /v1/device/sync/push`, `GET /v1/device/sync/pull` with at most a
  25-second wait, and `POST /v1/device/sync/ack`.
- `GET /v1/device/session-commands` with at most a 25-second wait, followed by
  explicit claim, receipt acknowledgement, and terminal result mutations.

Long-poll responses are bounded to 100 sync changes or 50 commands, honor
server shutdown/cancellation, and return immediately on revocation. Retry uses
exponential backoff with jitter. Local CLI/Daemon operations do not depend on
the Control Plane and continue during outage.

## Metadata classification and projection

The server accepts only typed allowlists; no endpoint accepts arbitrary domain
objects or a generic `metadata` map.

| Projection | Allowed fields | Explicit exclusions |
|---|---|---|
| Device | server ID, kind, display name, platform, architecture, client version, public keys, full key digests, storage mode, capabilities, status, key revision, last authenticated/seen timestamps | private keys, Vault state/details, local root/path, local IPC identity |
| Account | server ID, provider, display name, provider-subject digest, subscription hint, enabled, revision, timestamps | secretRef, auth file/token/cookie, raw subject/email, secret digest |
| Credential status | server ID, Account/Device server IDs, auth method, status, availability, credential revision, last validated/expiry timestamps | secretRef, Provider home/path, secret digest/body |
| Profile | server ID, Device/Account server IDs, provider, name, selector alias, enabled, model preference, bounded non-secret environment keys/values, MCP/Skill/Hook reference keys, revision, timestamps | credential binding, filesystem/config-home paths, secret-like environment keys, arbitrary JSON |
| Workspace | server ID, Device server ID, label, tags, bounded non-secret provider defaults, revision, timestamps | real workspace path, Home path, mount/share location |
| Session | server ID and mapped relation IDs, provider, status, capability snapshot, start/end/exit/failure metadata, revision, timestamps | Provider session ID, prompt/model/terminal text, Approval body, local path |
| Usage | mapped IDs, provider/version, optional source/confidence/availability/capability status/error code, observedAt, staleAt, and typed windows with optional numeric/reset values | raw Provider payload, credential material, invented quota |

Environment keys are limited to `^[A-Z][A-Z0-9_]{0,63}$`, values to 2048
UTF-8 bytes, at most 64 entries, and a denylist covering token/key/secret/
password/cookie/auth/proxy credential names. Reference keys and labels are
bounded UTF-8 strings, not filesystem paths. Unknown fields fail validation.

Usage preserves the shipped P1 provenance model: source is
`official|cli_derived|local_estimate|unofficial|unavailable`; confidence is
`high|medium|low|none`; availability is
`available|limited|unavailable|unknown`; `observedAt` and `staleAt` are required;
each window has kind/label, explicit `unit`, `scale`, optional decimal-string
scaled used/limit quantities, optional remaining basis points, and reset time.
Optional means unknown, never zero. USD requires unit `usd`, scale 6, and an
explicit official source; provider units are never converted to dollars.
Claude data cannot be labeled official remaining quota without a verified
official source; unavailable Claude quota is rendered unavailable.

## Revisioned sync and tombstones

Each mapped resource has a server revision starting at one and an append-only
server change revision. The P4 wire carries an exact `fullBase`, `fullNext`, and
diagnostic `patch`; the patch is never mutation authority. The authoritative
candidate is `fullNext`. `CanonicalSyncRevisionV1` is the strict object:

```text
CanonicalSyncRevisionV1 {
  schemaVersion: 1
  resourceType: account | credential_status | profile | workspace |
                session | usage
  resourceId: UUIDv7
  revision: integer[1, 2^53-1]
  operation: upsert | delete
  value: exact type-specific SyncValueV1 | null
}
```

`upsert` requires the matching exact type-specific value; `delete` requires
`value=null`. Sync values are the corresponding complete projection schemas
with `id` and the server revision removed; relation IDs and timestamps remain.
There is no generic map or unknown member. Optional members are omitted when
absent; explicit `null` is accepted only where the type schema says nullable.
Every full revision is at most 192 KiB after canonical encoding.

Canonical bytes are RFC 8785 JCS after strict OpenAPI/type validation: object
keys use RFC 8785 UTF-16 ordering, arrays retain order, strings receive no
Unicode normalization, finite IEEE-754 numbers use the RFC 8785/ECMAScript
shortest representation with negative zero encoded as `0`, and duplicate/
unknown keys, non-finite values, unsafe integers, or lossy numeric conversion
fail. The revision digest is exactly:

```text
SHA-256(frame("multidesk-sync-resource-revision-v1", "1",
                 resourceType, resourceId, decimalRevision,
                 JCS(CanonicalSyncRevisionV1)))
```

For create, and only create, `baseRevision=0`, `fullBase=null`, and
`baseDigest` must equal:

```text
SHA-256(frame("multidesk-sync-create-base-v1", "1",
                 resourceType, resourceId))
```

There is deliberately no revision-zero history row. Create additionally
requires `fullNext.revision=1`, `operation=upsert`, no live resource/history,
and no deletion watermark. Existing update/delete requires
`baseRevision>=1`, a `fullBase` whose type/ID/revision match, its computed
digest equal to both `baseDigest` and the lifetime
`resource_revision_digests(type,id,revision,digest)` row, and
`fullNext.revision=baseRevision+1`. Delete uses a full canonical next revision
with `operation=delete,value=null`; update uses `operation=upsert`. If the
required history digest row is absent, the whole batch returns
`sync_history_missing`, writes nothing, advances no cursor, and marks that
Device `snapshot_required`; the server never guesses or reconstructs history.

`SyncPatchV1` is a deterministic checksum/diagnostic of
`fullBase.value -> fullNext.value`. Each operation is
`{op:add|remove|replace,path,beforeDigest?,afterDigest?}`; paths are canonical
RFC 6901 pointers (`~` -> `~0`, `/` -> `~1`, root is the empty string) and each
present subtree digest is
`SHA-256(frame("multidesk-sync-patch-value-v1","1",JCS(value)))`. Create is
one root `add`; delete is one root `remove`. Otherwise equal canonical values
emit nothing; fixed-schema objects and dynamic schema maps recurse over the
union of keys in RFC 8785 order; missing/present keys emit add/remove; scalars,
type changes, and every array at any nesting depth are atomic and emit one
replace at that path. Thus ordered arrays never use an implementation-chosen
LCS, while nested maps are deterministic. A patch is limited to 128 operations,
256-byte pointers, and 16 KiB canonical bytes; overflow rejects the entire
batch as `sync_patch_too_large`. The client supplies the patch and
`patchDigest`; the server recomputes both byte-for-byte and rejects
`sync_patch_mismatch` before conflict evaluation.

On a stale valid base, `SyncConflict` returns the exact canonical `fullBase`,
current accepted `fullCurrent`, submitted `fullNext`, and deterministic
`baseToCurrentPatch` plus `baseToNextPatch` and their digests. This replaces the
old scalar-only difference tuple and covers maps, nested fixed objects, and
arrays without ambiguity. Sync conflict detail is capped at 768 KiB inside the
1 MiB response limit; exceeding the already-frozen 192-KiB per-revision or
patch bounds is rejected before persistence rather than silently truncated.

A device commits mapping + local mutation + outbox entry before transmit. The
server first validates the whole request. Any malformed, forbidden-field,
relation-invalid, history-missing, base-digest-invalid, next-digest-invalid, or
patch-invalid change rejects the entire batch with zero writes. Otherwise, in
one transaction it applies all nonconflicting changes, emits only their change
rows/cursor positions, returns conflicts for the remainder, and stores the
complete deterministic batch result for replay. Changes whose dependency
conflicted are conflicts, not partial orphans. No cursor advances for conflict
rows; all nonconflicting applies commit together.

Pull uses an opaque, endpoint-bound cursor encoding the last server change
revision and tie-breaker. The server may redeliver until `sync/ack` commits the
cursor. The daemon applies each change and advances its inbox cursor in one
local transaction. A stale but fully verified `baseRevision` returns the
bounded full revisions and typed patches above; it never silently overwrites or
exposes an excluded field.

A delete creates a tombstone with resource type, UUIDv7 resource ID, final
revision, deletion time, and a frozen set of then-active eligible Device IDs.
Offline eligible Devices block collection until they acknowledge the deletion
or are explicitly revoked. After every remaining eligible acknowledgement plus
30 days, the payload/ack tombstone may be collected, but a compact
`resource_deletion_watermarks(type,id,final_revision,digest,deleted_at)` row is
retained for the server database lifetime. A deleted UUID can never be created
again; a logically new resource must use a new UUIDv7. Every upsert checks the
watermark before revision logic. Revoked/re-enrolled/restored Devices therefore
cannot resurrect a collected tombstone; their stale outbox entries are
quarantined as `stale_resurrection`.

### Mapping ownership, snapshots, and restore

Device-originated resources allocate/commit their UUIDv7 mapping with the local
row and outbox entry in one transaction before push. A browser-created Profile
is server-owned for one explicit active target Daemon: the server allocates its
UUIDv7 only after validating target ownership plus Account/provider relations.
On delivery, that target Daemon allocates a new correct prefixed local ID via
the existing domain generator and commits local Profile + mapping in one
transaction before ack. Other Devices can view authorized projections but must
not materialize a false local owner. A server UUID already mapped to another
type/local row, a missing parent, relation mismatch, or conflicting binding
blocks ack and quarantines the change; it never overwrites/rebinds.

Initial enrollment, re-enrollment, and Device-backup restoration enter
`snapshot_required`. The target Device is an out-of-band prerequisite, not a
snapshot resource: before page zero, the server requires the signed Device
session to resolve to the same active enrolled `targetDeviceId`, and the Daemon
requires its P2/P3 envelope, Device mapping, pin, and activation receipt to be
open and consistent. `CanonicalSyncRevisionV1` therefore remains the six-type
union. Snapshot topology is exactly Account -> Credential status -> Profile ->
Workspace -> Session -> Usage, then canonical lowercase UUIDv7 ascending within
each type rank. Only the latest authorized upsert/delete revision for a resource
appears, exactly once.

Snapshot creation freezes a server `snapshotEpoch`, the target, a 1..4
`pageSize`, expiry ten minutes after creation, an `incrementalBaseCursor`, and
the complete ordered resource set. `SnapshotManifestV1` contains schema version
1, snapshot/epoch/target IDs, page size/count, resource count, expiry, base
cursor, and ordered entries `{ordinal,resourceType,resourceId,revision,
revisionDigest}`. The manifest digest is:

```text
SHA-256(frame("multidesk-sync-snapshot-manifest-v1", "1",
                 JCS(SnapshotManifestV1)))
```

It uses the same strict RFC 8785 rules as canonical sync revisions. Page count
is `max(1, ceil(resourceCount/pageSize))`; page `i` is exactly the manifest
slice `[i*pageSize, min((i+1)*pageSize, resourceCount))`. Thus no interior page
is empty, a non-final page is full, a non-empty final page has 1..pageSize
resources, and an empty snapshot is one page at index zero with no resources.

Every response carries the exact `SnapshotPageDigestInputV1`: schema version,
snapshot ID/epoch/target, manifest digest, page index/count, resource count,
prior page digest (`null` only on page zero), that page's full canonical
resources, exactly one continuation union (`next` with the persisted opaque
next-page token, or `final`), the shared expiry, and the shared incremental base
cursor. Its digest is:

```text
pageDigest = SHA-256(frame("multidesk-sync-snapshot-page-v1", "1",
                          JCS(SnapshotPageDigestInputV1)))
```

For page `i>0`, `priorPageDigest` must equal page `i-1`'s digest. The final page
also returns:

```text
SnapshotFinalDigestInputV1 {
  schemaVersion, snapshotId, snapshotEpoch, targetDeviceId, manifestDigest,
  pageCount, resourceCount, firstPageDigest, lastPageDigest, expiresAt,
  incrementalBaseCursor
}
finalSnapshotDigest = SHA-256(frame(
  "multidesk-sync-snapshot-final-v1", "1",
  JCS(SnapshotFinalDigestInputV1)))
```

For the empty snapshot, first and last page digests are the same. The last-page
chain plus manifest binds every page, resource, boundary, token, and final
cursor. The server permits one unexpired uncommitted snapshot per target. A
cursorless request with the same page size returns byte-identical
`SyncSnapshotPage` data for page zero;
another page size returns `snapshot_in_progress`. Every persisted next token is
bound to target/snapshot/epoch/manifest/next index/prior digest. Replaying a
valid token before expiry returns byte-identical `SyncSnapshotPage` data;
the outer success-envelope request ID may differ and is never digested.
Substitution, mixed
epoch/snapshot/target, reorder, omission, duplication, truncation, wrong prior
digest, premature final marker, or cursor substitution returns a stable snapshot
error and cannot advance state. Expiry invalidates uncommitted pages/commit and
releases the active slot; a fresh snapshot is required.

Pages stage locally. The Daemon verifies the manifest order/slices, page chain,
resource digests, topology/parents/mappings, final digest, and base cursor before
one transaction applies mappings/projections and installs that cursor. Existing
dirty local outbox rows are not overwritten: matching bases replay afterward;
mismatches or watermarked IDs quarantine and block commit. Missing parents/type
conflicts abort the staged snapshot. Commit binds target, snapshot/epoch,
manifest/final/last-page digests and base cursor, requires Idempotency-Key, and
atomically marks the snapshot committed. Exact replay returns the original
success. Reusing the same Idempotency-Key with a changed body is
`idempotency_key_reused`; using a fresh key with any different body/digest/
cursor for that already committed snapshot is `snapshot_commit_conflict`.
Committed-result replay follows the ordinary 24-hour idempotency retention even
after page expiry. Newly enrolled Devices do not join old tombstone ack sets.

A supported Device backup is an atomic copy of DB plus matching Vault material;
it preserves mappings, receipts, outbox/inbox, and cursors. Restore never
regenerates or reassigns server IDs and always performs the authoritative
snapshot gate before incremental replay. Server backups must include revision
digests/deletion watermarks; restoring a database without them or mixing epochs
is schema-incompatible rather than reconstructing IDs from stale Device state.

## Asynchronous Session Commands

Commands are metadata/control intent, not terminal transport. Supported kinds
are `start`, `stop`, `kill`, `resume`, `acquire_control`, and
`release_control`. The Web phase exposes start/stop/kill/resume only;
acquire/release remain protocol contracts for later controller UI.

State transitions are:

```text
queued -> claimed -> acknowledged -> succeeded | failed | unsupported
queued | claimed | acknowledged -> expired
claimed --claim lease expiry--> queued
terminal states are immutable
```

Creation returns `202` and a UUIDv7 command ID. Default TTL is five minutes;
the accepted range is 30 seconds to 15 minutes. One eligible target Daemon may
claim for 30 seconds. There is no claim token. Claim response and ack/result
DTOs bind authenticated target Device, command ID, request digest, monotonically
increasing attempt, claim expiry, and request idempotency key. Only an unacked
expired claim returns to `queued`. Ack is valid only for the current unexpired
attempt and means the daemon committed a matching `reserved` receipt; it freezes
that attempt. An acknowledged command never requeues and reaches a terminal
result or TTL `expired`. Stale/wrong Device/digest/attempt calls fail.

Daemon receipt state is:

```text
reserved -> executing -> local_committed -> completed
             |                    |
             +---- ambiguous <----+
```

The daemon transactionally inserts `reserved` before ack. A receipt digest is
`SHA-256(frame("multidesk-daemon-command-receipt-v1","1",
JCS(strict receipt fields)))`; the strict fields include command/Device/request
digest, attempt, receipt revision, state, immutable local operation/session
identities, and safe result. The daemon must receive successful ack, or fetch an
authoritative already-acknowledged state for the same attempt, before CASing to
`executing`. It then calls the existing local application service with
`commandId + requestDigest` as its durable idempotency identity. For
`start|resume`, the initial `reserved` transaction also reserves one new local
Session ID, its server UUID mapping, and the command binding before any Provider
side effect; replay returns that same reservation/Session. For stop/kill/
acquire/release it persists the exact target and pre-state before the local
call. Successful local commit records the resulting Session/state as
`local_committed` before network result upload; accepted terminal server result
marks `completed`.

Claim expiry uses an exact boundary. The server serializes the lease reaper and
ack CAS: if ack commits first the command is `acknowledged` and never requeues;
if expiry commits first it records the expired attempt in append-only claim
history, returns the command to `queued`, and the next winning claim is exactly
attempt `N+1`. On redelivery the daemon applies these state rules:

- An old `reserved` receipt may be rebound, and only while still `reserved`.
  One local transaction CASes `(command,device,requestDigest,state=reserved,
  deliveryRevision=D,attempt=N,receiptRevision=R)` to the current append-only
  delivery revision and server attempt, increments only `receiptRevision`, and
  recomputes the receipt digest. Local operation ID, Session reservation/
  mapping, request digest, and all command/reservation fields are byte-identical.
  It then acks the current attempt. Repeated expiry may perform
  the same reserved-only CAS again; concurrent/stale rebind loses and reloads.
- An old `executing` receipt is never rebound and the application service is
  never invoked again. The daemon reconciles the already-reserved local
  operation only: proof of commit moves it to `local_committed`; inability to
  prove whether a side effect occurred moves it to `ambiguous`.
- An old `local_committed` or `ambiguous` receipt is never rebound. The daemon
  uses the explicit reconcile operation with the current claim attempt plus the
  immutable receipt attempt/digest. The server verifies both attempts were
  issued to the same Device/command/digest, then atomically records
  acknowledgement and the stored terminal outcome. `ambiguous` can produce
  only failed `command_execution_ambiguous`.
- An old `completed` receipt is never rebound. It is a duplicate only when the
  authoritative server command is already terminal with the same outcome;
  any nonterminal/mismatched state is quarantined as
  `command_receipt_inconsistent` and cannot execute automatically.

The reconcile endpoint never accepts `reserved` or `executing`, never invokes a
local service, and requires a live current claim. If that claim expires first,
the next attempt may submit the same immutable later-state receipt. Lost ack
request therefore yields a safe reserved rebind; lost ack response yields an
idempotent ack/query of the already acknowledged attempt; claim/ack/reaper races
have one server CAS winner. On ordinary restart, current `reserved` retries ack,
`local_committed` resends result, and `completed` returns the recorded result.
Different Device/command/digest/local binding always fails. This is
at-least-once transport with a durable idempotency/reconciliation boundary,
never distributed exactly-once or a claim that ambiguous Provider effects are
at-most-once.

## Plan v0.7 decision-complete amendments

These rules replace conflicting pre-v0.6 wording. They do not reopen verified P0
or authorize P1 runtime behavior beyond health/readiness/version.

### P1 contract reconciliation boundary

P1 checks in the complete Phase 4a OpenAPI, generated Go strict server/client/
models, generated TypeScript types, and the exhaustive first-party runtime
client for every P1-P6 operation. Before P1 verification, those artifacts must
be regenerated from v0.7 and byte-verified twice. Only `healthz`, `readyz`, and
`version` are mounted as product handlers in P1. Bootstrap, auth, enrollment,
metadata, sync, and command routes are contract-only and must return the common
unavailable response if accidentally reached; no P2+ row, token, cookie,
identity, or mutation is created.

### P2 server-origin, migration, and WebAuthn boundary

`CanonicalServerOriginV1` is the immutable remote-identity namespace. It is
ASCII `https://host[:port]` with lowercase scheme/IDNA ASCII host, no userinfo,
path, query, or fragment, and no explicit default port. Parsing rejects IP
literals in production, trailing-dot ambiguity, percent-encoded host bytes,
wildcards, and any value whose parse/serialize round trip changes. Localhost is
allowed only under the existing explicit development flag.

The origin is stored in `remote_device_identities`, mapped with the remote
identity, included in `DeviceKeyEnvelopeV1`, its AAD, the bootstrap descriptor/
challenge/commit receipt, and every later activation receipt. An existing row
can be opened only for that byte-identical origin. A different origin always
creates new remote keys, a new local remote-identity ID, a new server UUIDv7,
and a new enrollment; no cross-server rebind API exists.

Before Device schema v7 becomes v8/migration 0008, the stopped or exclusively
locked Daemon uses SQLite Online Backup into
`<device-data>/backups/schema-v7/<UTC-basic>-<db-sha256-prefix>/device.sqlite`
and writes a restricted-JCS `manifest.json` containing schema version, size,
SHA-256, created time, and binary version. It fsyncs file and parent directory,
reopens the copy read-only with integrity/FK checks, and only then migrates.
Unix directory/file modes are `0700/0600`; Windows grants the current logon SID
and SYSTEM only and denies inherited/network access. Unsafe paths, symlinks,
digest mismatch, missing Vault rows, or unverifiable backup abort migration.
Recovery requires the Daemon stopped: verify the manifest/digest/permissions,
copy to a same-directory temporary file, fsync, atomically replace, then open
with the exact prior binary. No down migration or partial-table copy exists.

The browser cookie is exactly `__Host-mad_session`; it is Secure, HttpOnly,
SameSite=Strict, Path=/, and has no Domain attribute. Authentication counter
updates use a credential-revision CAS in the same transaction as assertion use:
`0->0`, `0->N (N>0)`, and `N->M (M>N)` succeed; `N->N` and `N->M (M<N)` for
nonzero N are clone/regression failures. A losing concurrent CAS reloads and
re-evaluates these rules. Clone/regression rejects the assertion and atomically
revokes every browser session whose authentication credential is that Passkey.
Deleting a non-last Passkey similarly revokes all sessions authenticated by it;
if that includes the current session the response clears the host cookie and
cannot return an authenticated continuation. The last active Passkey remains
undeletable.

Real-browser P2 acceptance is not a floating “current browser” claim. At P2
build start one receipt freezes exact browser version plus OS build for Chrome
and Safari on macOS arm64 and Edge and Firefox on Windows 11 x64. The same
frozen binaries run registration, login, counter fixtures where injectable,
logout, recovery replacement, and passkey deletion. Safari's platform Passkey
ceremony is manual/real-browser evidence; protocol fixtures or WebKit emulation
cannot replace it. Any browser upgrade invalidates that row and requires rerun.

### P3 remote identity, device auth, and enrollment boundary

Migration ownership is now: `0008_control_plane_remote_identity.sql` (P2 base
envelope/mapping), `0009_remote_device_trust.sql` (P3 local peer pins,
activation receipts, remote identity lifecycle), `0010_control_plane_sync.sql`
(P4), and `0011_remote_command_receipts.sql` (P5). P3 must not create sync
tables or snapshot payload/page/commit behavior; activation only sets the
server/local `snapshot_required` flag consumed by P4.

The singleton local IPC Device ID is never a remote identity ID. Each remote
row has a generated `remote_identity_<32 lowercase hex>` local ID, its canonical
server origin, and its own server UUIDv7 mapping. Local IPC identity may be a
relation/audit fact but never occupies that mapping key. New key material means
a new local remote-identity ID and server UUID; mappings cannot rebind.

Device lifecycle is `pending|active|revoked`; presence is a separate
`online|offline` projection. At every server start a random UUIDv7 `bootEpoch`
is generated. Online is derived only when lifecycle is active, the last
authenticated heartbeat was accepted under the current boot epoch, and
`now-lastSeenAt <= 60s`; restart therefore renders all devices offline until a
new authenticated heartbeat. Presence never changes lifecycle.

Device-auth challenge rows and request nonces are durable. A 60-second signed
challenge can survive restart because the server challenge-signing key and
row are durable; one exchange CAS consumes it. Concurrent exchanges have one
winner. The returned device-session token is 32 random bytes, only its SHA-256
digest is stored, and it expires after 15 minutes. Request nonces are stored by
session until session expiry, so restart cannot reset replay protection.
Validity intervals are half-open (`issuedAt <= now < expiresAt`).

Candidate authorization is distinct from active Device auth. Daemon candidates
use `EnrollmentPreAuthV1`: every create/prove/activate/resume/cancel request is
signed by the pending subject Ed25519 key over the canonical request plus
enrollment transcript/challenge revision; Web candidates additionally require
the normal same-origin browser cookie/CSRF class. No opaque activation bearer
secret is introduced. Approve requires an active signed approver Device with
the recognized approval capability and a locally pinned subject.

Enrollment state is exactly `pending_proof -> proof_verified -> approved ->
activated`, with terminal `cancelled|expired`. Mutations use state/revision CAS
and Idempotency-Key. Restart before proof invalidates only the memory-only
ephemeral X25519 challenge; resume increments `challengeRevision`, returns a
fresh challenge, and makes all earlier proofs invalid. `proof_verified` and
`approved` persist and resume byte-identically. Approve stores the public
attestation/receipt package but does not activate. The candidate first obtains
the package including raw approver public keys, recomputes their digests,
verifies the attestation and receipt under its locally supplied/pinned
approver key, displays/requires approver fingerprint confirmation, persists the
pin, and only then sends the final subject activation signature. Server
directory keys are lookup data and can never satisfy this local verification.

Browser `AuthCapabilityV1` and signed `DeviceCapabilityV1` are separate closed
types and authorization evaluators. Browser authentication never delegates a
Device capability. `mad.v1.session.command_create` is a recognized Device
capability for a signed eligible key-bearing client, while the browser command
endpoint separately requires its server-derived Auth capability. Unknown
well-formed Device strings are preserved but ineffective; malformed strings
reject. An approver can delegate only the intersection of its recognized
delegable set, kind/storage eligibility, and explicit user confirmation.
Revocation authority is not implied by enrollment approval.

For `software_wrapped`, P3 pins `@noble/curves@2.2.0` and performs real X25519
key generation/shared-secret proof in browser code. The 32-byte private key is
wrapped immediately with a non-exportable WebCrypto AES-256-GCM key; transient
buffers are zeroed best-effort. `WebDeviceStorageAssertionV1` binds origin,
device/key IDs and digests, storage mode, wrapping-key algorithm/extractability,
ciphertext digest, key revision, IndexedDB schema version, and probe time; the
subject signs its digest in enrollment PoP. A storage label without successful
X25519 PoP is rejected.

### P4 metadata and synchronization boundary

The 1-MiB response limit is absolute. A canonical sync revision remains capped
at 192 KiB, so snapshot `pageSize` is exactly 1..4 and every page response is
measured before send; overflow is `snapshot_page_too_large` with no cursor/state
advance. Ordinary pull remains count-bounded to 100 and additionally stops at
900 KiB canonical JSON, returning `hasMore=true`. It never emits an oversized
response.

Network `SessionProjectionV1.provider` is only `codex|claude`; `fake` is an
in-process deterministic test adapter and never appears in OpenAPI, sync rows,
overview, or audit projections. Tests may drive the local service with Fake but
must assert the serialized provider remains an allowed fixture value or that
no network Session row is emitted.

A browser-created Profile is always committed `enabled=false`. The server wire
contains only allowed model/environment/reference intent. The target Daemon
stores that intent in `controlplane_profile_materializations` with
`pending_local_completion`, allocates the local prefixed Profile/mapping, and
requires the local operator to set local-only approval and sandbox policies and
pass local Provider validation. Those policy values never enter outbox,
OpenAPI, sync digest, conflict, log, or server DB. Only a signed Daemon
`materialization_ready` revision permits a later browser CAS to enable it.

`serverSyncRevision` belongs to the mapping/projection/outbox protocol and is
never copied into the Device-local Workspace/Session/domain `revision` fields.
The local entity CAS and server sync CAS commit together where needed but are
separate columns/counters; rollback or conflict in either aborts the whole
transaction. Server restore cannot rewrite local revisions.

### P5 command execution boundary

The server preallocates command UUIDv7 and, for start/resume, one immutable
`resultSessionId` UUIDv7 in the creation transaction. The strict
`CanonicalSessionCommandRequestV1` binds schema version, command/result IDs,
creator class/ID, target Device, kind, typed parameters, referenced server
resource revisions, created/expiry times, and the default 300-second TTL. Its
domain-separated digest is the only command identity sent to a Daemon; the
browser's raw creation Idempotency-Key never crosses the server boundary.

Delivery offers are append-only `(commandId, deliveryRevision,
expectedNextAttempt)` rows. Listing creates/returns an offer but never allocates
an attempt/lease, changes command state, or advances the committed cursor.
Requeue appends a new revision; it never mutates/reuses an old offer. Claim is
the sole CAS that allocates the expected attempt and 30-second lease, but still
does not advance the cursor. After claim, the Daemon locally commits the exact
`ReservedReceiptV1`; ack validates it and atomically marks the delivery accepted
plus advances the server cursor only across a contiguous accepted/terminal/
superseded prefix. Responses are ordered by delivery revision, expose
`hasMore`, and may redeliver an uncommitted offer. The ack result's
`DeviceCommandCursorCommitV1` is the only server wire cursor-commit fact; the
Daemon's local reserved-receipt transaction precedes that request and is not
misdescribed as a server transaction. A signed authoritative Device query
returns command, current attempt/delivery, claim expiry, receipt revision/
digest, committed cursor, terminal result, and command revision so restart
never infers state from a stale offer.

All time checks are half-open: a claim/command is valid only while
`now < expiresAt`; equality is expired. Transaction priority is target
revocation, existing terminal state, command TTL, feature-disable policy, claim
expiry/reaper, then the requested mutation. Revocation blocks every new signed
call and terminalizes nonterminal commands with the stable revoked outcome.
Feature disable blocks create/offer/new claim but permits an already
acknowledged current attempt to report/reconcile until command TTL. Claim
attempts cap at eight; exhaustion terminates `delivery_attempts_exhausted`.
Terminal command/delivery/claim/receipt metadata is retained 30 days,
idempotency results 24 hours, then bounded FK-safe GC removes payload rows while
retaining compact audit digests for 365 days.

`DaemonCommandReceiptV1` is a discriminator oneOf by execution state and
command kind. `integrityStatus=verified|quarantined` is separate from execution
state. Reserved binds the immutable local operation and, for start/resume,
local/server result Session mapping; executing adds start time and exact
pre-state; local_committed adds a kind-specific durable operation proof and
safe result; ambiguous is reachable only from executing when restart cannot
prove commit; completed adds accepted server result revision. A
`local_committed` receipt can never become ambiguous. Quarantined receipts
cannot execute or report success.

Per-kind restart proof is mandatory: start/resume proves command binding,
reserved result Session mapping, local Session row, and P4 outbox revision;
stop/kill proves the dedicated local operation record plus before/after Session
revision/status. Generic post-effect idempotency is insufficient. A dedicated
`RemoteCommandService` owns reserve/execute/reconcile and calls local Session
services through a deterministic derived key committed with the receipt CAS:

```text
base64url(SHA-256(frame("multidesk-remote-command-call-v1", "1",
  commandId, requestDigest, decimalDeliveryRevision, decimalAttempt,
  callKind, decimalReceiptRevision)))
```

The terminal outcome is also a closed discriminator union. `CommonFailureCode`
is `target_revoked|feature_disabled|delivery_attempts_exhausted|
daemon_shutting_down|command_execution_ambiguous`; it may appear only in a
failed variant:

```text
StartSucceededV1 {kind:start,state:succeeded,code:session_started,
  resultSessionId:UUIDv7,sessionStatus:starting|running}
StartFailedV1 {kind:start,state:failed,
  code:CommonFailureCode|vault_locked|profile_disabled|
       credential_unavailable|mapping_invalid,
  resultSessionId:UUIDv7}
StartUnsupportedV1 {kind:start,state:unsupported,
  code:provider_session_start_unsupported,resultSessionId:UUIDv7}

ResumeSucceededV1 {kind:resume,state:succeeded,code:session_resumed,
  sourceSessionId:UUIDv7,resultSessionId:UUIDv7,
  sessionStatus:starting|running}
ResumeFailedV1 {kind:resume,state:failed,
  code:CommonFailureCode|vault_locked|profile_disabled|
       credential_unavailable|mapping_invalid|session_not_found|
       session_state_conflict,
  sourceSessionId:UUIDv7,resultSessionId:UUIDv7}
ResumeUnsupportedV1 {kind:resume,state:unsupported,
  code:provider_resume_unsupported,sourceSessionId:UUIDv7,
  resultSessionId:UUIDv7}

StopSucceededV1 {kind:stop,state:succeeded,code:session_stopped,
  sessionId:UUIDv7,sessionStatus:exited}
StopFailedV1 {kind:stop,state:failed,
  code:CommonFailureCode|session_not_found|session_state_conflict,
  sessionId:UUIDv7}
StopUnsupportedV1 {kind:stop,state:unsupported,
  code:provider_stop_unsupported,sessionId:UUIDv7}

KillSucceededV1 {kind:kill,state:succeeded,code:session_killed,
  sessionId:UUIDv7,sessionStatus:killed}
KillFailedV1 {kind:kill,state:failed,
  code:CommonFailureCode|session_not_found|session_state_conflict,
  sessionId:UUIDv7}
KillUnsupportedV1 {kind:kill,state:unsupported,
  code:provider_kill_unsupported,sessionId:UUIDv7}

AcquireUnsupportedV1 {kind:acquire_control,state:unsupported,
  code:phase4b_controller_required,sessionId:UUIDv7}
ReleaseUnsupportedV1 {kind:release_control,state:unsupported,
  code:phase4b_controller_required,sessionId:UUIDv7}

SessionCommandOutcomeV1 = oneOf(
  StartSucceededV1,StartFailedV1,StartUnsupportedV1,
  ResumeSucceededV1,ResumeFailedV1,ResumeUnsupportedV1,
  StopSucceededV1,StopFailedV1,StopUnsupportedV1,
  KillSucceededV1,KillFailedV1,KillUnsupportedV1,
  AcquireUnsupportedV1,ReleaseUnsupportedV1)
```

No design alias or shortened allowlist is permitted. Acquire/release are never
delivered in Phase 4a because their exact branches terminate unsupported before
offer/claim.

Workers default to four and cap at sixteen per Daemon, with one singleflight per
command and serialization per local Session. Start/resume reservations also
serialize on result Session mapping. Shutdown stops polling/claiming first,
allows at most ten seconds for in-flight local DB commits, never begins a
Provider call after shutdown starts, and leaves durable reserved/executing
state for the exact restart proof path.

### P6 browser and presentation boundary

P6 exposes bounded enrollment list/filter state and a server-computed Overview
aggregate with `generatedAt`, per-section `observedAt|staleAt`, counts, and at
most five recent items per section. The Web must not full-page every resource
to calculate Overview. Usage window quantities use decimal-string scaled
integers plus an explicit unit and scale; missing stays absent. USD is legal
only as unit `usd` with scale 6 from an explicit official source. Provider
units are never converted to dollars and no absent value is rendered zero.

Production browser crypto lives in browser-safe modules under
`packages/protocol`: no Node `Buffer`, `crypto`, or P0 harness import. It owns
restricted JCS, framing, pin/attestation/PoP verification, real X25519, and
constant-time digest comparison where possible. IndexedDB schema v1 has
separate identity, wrapped-key, local-pin, receipt, and CAS-metadata stores;
every write checks `(deviceId,keyRevision,recordRevision,serverOrigin)` and
never replaces a pin from the server directory.

The service worker caches only content-addressed static assets. `/v1/**`, auth,
bootstrap, enrollment, recovery, health/version, and mutation requests are
network-only and are never cached, background-synced, replayed, or served by SPA
fallback. SPA fallback applies only to same-origin navigation with an HTML
Accept header outside reserved server paths. Logout revokes/clears the browser
session and in-memory CSRF but retains the enrolled Web Device keys and local
pins; an explicit UV-protected Forget Device/revocation flow is the only key
deletion path.

Command polling stops on a terminal command, command expiry, logout/session
expiry, device revocation, or unrecoverable API version error; retry uses
bounded jittered backoff and honors `Retry-After`. Recovery plaintext remains
memory-only. Copy, user-chosen download, and print are explicit actions with a
privacy warning; no screenshots, traces, service-worker cache, analytics,
local/session storage, crash report, or test artifact may contain it. Clipboard
or printed/downloaded copies are described as user-controlled and not remotely
erasable.

The real acceptance matrix freezes exact browser/OS builds in receipts:
Chrome+Safari on macOS arm64 and Edge+Firefox on Windows 11 x64, including a
real Safari Passkey/storage run. Desktop evidence launches the macOS Tauri shell
and verifies visible shared-UI rendering and navigation; `cargo check` alone is
not a render receipt. Any matrix row not executed is a release blocker, not a
structural pass.

Registry metadata was checked on 2026-07-21 for the exact candidate pins:
React 19.2.8, React Router 7.18.1, TanStack Query 5.101.4,
SimpleWebAuthn/browser 13.3.0, noble-curves 2.2.0, idb 8.0.3,
vite-plugin-pwa 1.3.0, Vite React plugin 5.2.0, Vitest 4.1.10, jsdom 29.1.1,
Testing Library React 16.3.2/user-event 14.6.1/jest-dom 7.0.0,
Playwright 1.61.1, axe-core 4.12.1, and selenium-webdriver 4.46.0. P3/P6 must
lock the applicable exact versions/integrities and run the full transitive
license gate; registry existence/license metadata is not transitive approval.

## Phase plan

### P0 — Contract freeze

Update authoritative protocol/data/security documents and vectors, freeze this
API/test plan, pin dependencies/toolchains/licenses, and add no product
behavior. Acceptance requires both vector implementations to agree on the full
digest, six-group fingerprint, and signed JCS attestation; docs must state the
portable-Vault/Phase 4a split consistently.

### P1 — Server, storage, and OpenAPI foundation

Implement server lifecycle/config, forward server migrations, health/version,
OpenAPI authority, deterministic generated Go server/client plus TS types,
first-party exhaustive typed runtime client, middleware bounds, UUIDv7/cursor/
idempotency primitives, and exact drift/tool-license gates. No user or Device
becomes active yet. The full contract must be reconciled to plan v0.7 before P1
verification; runtime remains health/readiness/version only.

### P2 — Bootstrap, Passkey, recovery, and browser session

Before enabling bootstrap, implement Device migration 0008, the generic mapping
table, exact envelope create/open/CAS API, pending-to-active bootstrap lifecycle,
and the prepare/import/prove/verify/activate Daemon + Web actor flow. Then
implement atomic Daemon-anchor bootstrap, WebAuthn registration/login, one-shot
server-side SessionData, local pre-user token rotate, exact recovery/Passkey/UV/
session lifecycle, cookie/CSRF endpoint matrix, and auth/current DTO. Pure Web
bootstrap fails; P2 cannot verify on a mock envelope assertion.

### P3 — Remote Device identity, enrollment, presence, and revocation

Reuse the P2-verified migration/envelope/mapping API for additional Daemon
identities; add Web ADR 0010 key storage, Desktop-kind server contracts,
actor-complete pin/attestation/public-receipt activation, versioned capability
evolution, signed device REST, heartbeats, key-change rejection, revocation,
and the enrollment `snapshot_required` gate. No Pairwise Root/HPKE/WSS.
Migration 0009 owns local pins/activation receipts; it adds no snapshot
implementation.

### P4 — Metadata projection and sync

Add typed Account/Credential-status/Profile/Workspace/Session/Usage
projections, CRUD/list filtering, revision/If-Match conflicts, device
push/snapshot/pull/ack, exact canonical full-base/full-next/patch wire and
revision/create-sentinel digests, Account-first snapshot resources with an out-
of-band target Device and exact manifest/page/final digest chain, migration
0010, replay-safe cursors, tombstones plus lifetime deletion watermarks, and
cleanup. Snapshot pages are capped at four, network Session
provider excludes Fake, and Profile/local-vs-server revision rules above apply.

### P5 — Async Session Commands

Add durable command creation/query, bounded long-poll delivery,
tokenless claim/ack/result/expiry/reclaim DTOs, daemon reserved/executing/local-
committed/ambiguous/completed reconciliation, reserved-only attempt rebind,
later-state reconcile DTO, migration 0011, restart/offline behavior, and mapped
local actions. No terminal input or Approval response.

### P6 — Web metadata UI and integration/security handoff

Build the responsive/PWA shell plus Bootstrap/Passkey/Recovery, Overview,
Devices, Accounts, Profiles, Sessions, and Usage flows. Add loading/empty/error/
offline/revoked/conflict/stale states, WCAG 2.2 AA keyboard/screen-reader gates,
supported-browser tests, Desktop shared build/render smoke, end-to-end
integration, security evidence, threat-model updates, and final handoff. No
Terminal, Approvals, Credential Grant, or release claim.

P6 may be split by `feature-review` into P6A Web flows and P6B final
integration/security handoff if that makes independent verification tractable;
the scope and final exit do not change.

## Rollback and recovery

- Every migration is forward-only, transactional, backed up before upgrade,
  and tested from the exact prior schema. Unknown/future/partial schema fails
  closed. Rollback uses the verified pre-migration backup with the prior binary,
  never a destructive down migration.
- P1-P5 server capabilities have explicit configuration gates. Disabling a
  capability stops new operations while preserving rows for a compatible newer
  binary. Auth/device revocation checks remain active when higher-level sync or
  command features are disabled.
- Device migrations preserve existing local rows/IDs. A rollback may stop using
  remote mappings/keys but must not delete or reinterpret them. Vault envelopes
  remain readable by the introducing/newer binary.
- Sync disablement leaves local Device state authoritative and retains outbox
  entries. It does not apply server state destructively or mark queued changes
  acknowledged.
- Command disablement stops creation/claim and lets claims expire; it never
  stops a local Session merely because the Control Plane is rolled back.
- Web assets are independently replaceable with the last verified static
  bundle. An older Web bundle must negotiate API version and fail closed on an
  unsupported server contract.

## Risks and gates

- **Security Gate: open.** Final independent review must cover bootstrap,
  WebAuthn/recovery, cookies/CSRF, Device signatures, pinning/attestation,
  revocation, metadata allowlists, sync replay/tombstones, command idempotency,
  logging, migrations, and dependency provenance.
- **Provider Gate: none.** Fake/local deterministic Sessions are sufficient;
  no Provider token or live Claude/Codex assertion is required.
- Human fingerprint comparison remains fallible. The server verifies proofs and
  signatures but cannot prove the human compared the right screens.
- Control Plane compromise exposes metadata/timing and can deny service. It
  still cannot replace local pins or receive Provider/terminal plaintext.
- Active same-origin Web compromise can use enrolled browser keys. Phase 4a's
  metadata-only boundary reduces impact but does not turn Web storage into an
  XSS boundary.
- UUID mapping, sync conflicts, tombstone eligibility, long-poll cancellation,
  claim expiry, and restart recovery are concurrency-critical and require race
  and failure-injection evidence before their phases verify.
