# Contracts: Phase 4a Control Plane Core

## Authority, versions, and generated artifacts

`api/openapi/control-plane-v1.yaml` is the canonical OpenAPI 3.0.3 contract.
Phase 4a does not use OpenAPI 3.1 features; any later upgrade needs a separately
reviewed contract/tool change. Go server/client/models and TypeScript OpenAPI
types are generated. The TypeScript runtime client is a named first-party layer
tied exhaustively to generated types; it is not described as generated.

Pinned dependency decision:

| Dependency | Exact pin | License/toolchain evidence | Use |
|---|---|---|---|
| `github.com/go-webauthn/webauthn` | `v0.17.4` | GitHub-verified tag published 2026-05-22; BSD-3-Clause; Go 1.25/toolchain 1.26.3 | Passkey ceremonies |
| `github.com/oapi-codegen/oapi-codegen/v2` | `v2.8.0` | Apache-2.0; Go 1.25; compatible with repo Go 1.26.5 | Go models, strict `net/http` server, client |
| `openapi-typescript` | `7.13.0` | MIT; npm integrity locked; compatible with Node 24/pnpm 10 | TypeScript protocol types |
| `github.com/getkin/kin-openapi` | `v0.142.0` through the pinned tool graph | MIT; explicitly license-scanned as a tool dependency | schema validation used by oapi-codegen v2.8.0 |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause; already indirect, promoted to direct | UUIDv7 generation with `uuid.NewV7` |

P0/P1 must run the repository license gate over the complete resolved graph
before either generator is added. Pins live in `go.mod/go.sum` and the frozen
pnpm lockfile; `latest`, floating actions, remote schemas, or unpinned generator
containers are forbidden.

Deterministic paths are:

```text
api/openapi/control-plane-v1.yaml
api/openapi/oapi-codegen-v1.yaml
internal/controlplane/api/generated/control_plane_v1.gen.go
packages/protocol/src/generated/control-plane-v1.ts
packages/protocol/src/control-plane-client.ts
apps/web/src/api/control-plane.ts
```

Root scripts `api:generate` and `api:verify` invoke
`node scripts/api/generate.mjs`. The generator runs exactly:

```text
go tool oapi-codegen -config api/openapi/oapi-codegen-v1.yaml \
  api/openapi/control-plane-v1.yaml
pnpm exec openapi-typescript api/openapi/control-plane-v1.yaml \
  --output packages/protocol/src/generated/control-plane-v1.ts
```

The `go tool` directive pins oapi-codegen v2.8.0. Its v2.8 pre-generation
validation through kin-openapi v0.142.0 is the schema validation pass; no
second unpinned validator is introduced. Generation uses repository Go 1.26.5,
Node 24, pnpm 10.23.0, fixed locale/timezone, and generated headers without
host/time. `api:generate` writes fixed paths. `api:verify` uses a fresh temp
directory, runs the same commands, byte-compares Go/TS outputs, compiles the
first-party client, and fails on drift/nondeterminism. `ci:licenses` explicitly
scans the `go tool` graph plus the frozen pnpm graph. `openapi-fetch` is not
added.

`control-plane-client.ts` exposes one named method for every Phase 4a operation
and a compile-time exhaustive map satisfying generated `paths`/`operations`;
adding/removing an operation fails typecheck until the runtime map changes. It
has no public `request(path, ...)` escape hatch. It enforces same-origin base
URL, `credentials:"include"`, JSON request/response only, generated request/
response types, stable error mapping, in-memory CSRF injection for browser
mutations, caller `AbortSignal`, and a 30-second client timeout for a maximum
25-second long poll. `apps/web/src/api/control-plane.ts` validates/configures
the same-origin instance and is the only Web import point.

## Common wire rules

- API version is the literal `v1`. Every JSON success and error body contains
  `apiVersion: "v1"`; health/version do too.
- All server/network resource, ceremony, batch, change, command, attestation,
  and audit IDs are lowercase canonical UUIDv7. Existing local prefixed IDs are
  never accepted on public/network ID fields.
- JSON uses UTF-8, rejects duplicate keys/non-finite numbers/unknown fields, and
  is limited to 1 MiB by default. Endpoint-specific limits may only be smaller.
- Times are UTC RFC3339 with no leap-second acceptance and microseconds or less.
- Binary fields use unpadded Base64url. SHA-256 digests decode to exactly 32
  bytes. Ed25519/X25519 public keys decode to exactly 32 bytes.
- Mutations require `Idempotency-Key` (16..128 visible ASCII characters).
  Update/delete additionally require `If-Match: "rev-<positive integer>"`.
- Idempotency scope is principal + API version + method + canonical path + key.
  The server stores request digest and result for 24 hours. Same key/body returns
  the original result; same key/different body returns
  `idempotency_key_reused`.
- One-time plaintext responses are the explicit exception: successful
  bootstrap recovery-code issuance and recovery-code rotation persist only a
  redacted success marker. Exact replay returns `one_time_result_unavailable`
  while preserving committed state; plaintext is never cached. After normal
  Passkey authentication/recent UV, the user can rotate to a fresh batch.
- List `limit` defaults to 50 and is 1..100. `cursor` is an opaque Base64url
  endpoint/filter/sort-bound token. Unknown, cross-endpoint, or tampered cursors
  return `invalid_cursor`. Responses are stable under `(sortField, id)`.
- Filters/sorts are endpoint allowlists below. Unknown names/operators fail;
  there is no arbitrary SQL/query language.
- `Cache-Control: no-store` applies to auth/bootstrap/recovery/device-auth;
  authenticated metadata uses explicit ETags and no shared caching.

Success envelope:

```json
{
  "apiVersion": "v1",
  "data": {},
  "meta": { "requestId": "uuidv7", "nextCursor": null }
}
```

Error envelope:

```json
{
  "apiVersion": "v1",
  "error": {
    "code": "stable_code",
    "message": "bounded safe text",
    "requestId": "uuidv7",
    "details": {}
  }
}
```

`details` is a per-code schema, never arbitrary data. Validation details contain
at most 32 `{field,rule}` entries. Ordinary conflict details contain at most 64
allowed fields and 64 KiB. The sole larger typed exception is `SyncConflictV1`,
whose three bounded full revisions plus two digest-only patches are capped at
768 KiB inside the 1 MiB response limit. Internal errors, SQL, paths, raw
bodies, credentials, cookies, challenges, signatures, and private material
never appear.

## Authentication and capability DTOs

### `CurrentAuth`

```text
CurrentAuth {
  userId: UUIDv7
  browserSessionId: UUIDv7
  authenticationMethod: passkey | recovery
  authenticatedAt: time
  recentUvAt?: time
  expiresAt: time
  idleExpiresAt: time
  csrfToken: base64url(32) // any browser session; no-store; memory-only client
  capabilities: AuthCapability[]
  webDevice?: {
    deviceId: UUIDv7
    enrollmentStatus: unregistered | pending | active | revoked | key_lost
    storageMode: native | software_wrapped | metadata_only
    keyRevision?: integer
  }
}

AuthCapability =
  mad.v1.metadata.read | mad.v1.profile.write |
  mad.v1.device.enroll_request | mad.v1.device.enroll_approve |
  mad.v1.device.revoke | mad.v1.session.command_create |
  mad.v1.passkey.manage | mad.v1.session.revoke
```

A recovery session receives only `passkey.manage` and `session.revoke` until a
replacement Passkey registration succeeds. That success atomically replaces it
with a normal rotated session/CSRF token and revokes all other browser sessions.
Capabilities are server-derived from authentication method, recent UV, device
state, and endpoint; clients never submit them as authority.

### Browser endpoints

```text
GET    /v1/auth/current
POST   /v1/auth/passkeys/options
POST   /v1/auth/passkeys/verify
POST   /v1/auth/passkeys/registration/options
POST   /v1/auth/passkeys/registration/verify
GET    /v1/auth/passkeys
DELETE /v1/auth/passkeys/{passkeyId}
POST   /v1/auth/uv/options
POST   /v1/auth/uv/verify
POST   /v1/auth/recovery/verify
POST   /v1/auth/recovery-codes/rotate
POST   /v1/auth/logout
GET    /v1/auth/sessions
DELETE /v1/auth/sessions/{sessionId}
```

Options responses are exactly `WebAuthnCreationOptionsV1` or
`WebAuthnRequestOptionsV1` (defined below) and include UUIDv7 `ceremonyId`;
they do not expose serialized go-webauthn `SessionData` or a generic library
map.
Verification accepts only `ceremonyId` and the bounded browser credential
response. The server consumes the full stored SessionData exactly once.

`auth/recovery/verify` accepts one recovery code over HTTPS, consumes a match
atomically, and sets the restricted recovery-session cookie. It returns no
stored code or hash. Enumeration and rate-limit responses are indistinguishable.

`GET /auth/passkeys` returns ID, user-visible name, created/last-used time,
transport hints, and whether it is current; never credential public-key bytes or
attestation objects. Delete requires normal session, CSRF, If-Match, and UV no
older than five minutes, and returns `last_passkey_required` if it would remove
the last active Passkey. UV options/verify perform a Passkey assertion and set
`recentUvAt` without broadening capability; verify rotates session/CSRF.

A recovery batch is exactly ten records. Plaintext format is
`MAD-RC1-` plus eight groups of four uppercase unpadded RFC 4648 Base32
characters representing exactly 20 random bytes. Parser normalization is
limited to the exact prefix/group hyphens and ASCII case. Each record stores a
16-byte salt, frozen Argon2id parameters (`t=3,m=64MiB,p=1,out=32`), hash,
batch UUIDv7, ordinal, status, and consumed time. `recovery-codes/rotate`
requires normal session plus UV within five minutes, invalidates the old batch
and commits ten new hashes atomically, and returns plaintext once. An exact
Idempotency-Key replay returns `one_time_result_unavailable`; it never caches
or repeats plaintext. The user may perform a new rotation with recent UV.

### Browser security matrix

| Endpoint class | Cookie | CSRF | Origin / Fetch Metadata | Content |
|---|---|---|---|---|
| `GET /healthz|readyz|version`, bootstrap status/public ceremony read | none | none | no mutation; CORS disabled; ceremony read is strictly rate-limited | no body |
| pre-auth `bootstrap/options|verify`, login Passkey options/verify, recovery verify | not required and not authority | none (no session yet) | exact configured Origin; `Sec-Fetch-Site:same-origin`, mode `cors|same-origin`, destination `empty`; strict per-IP/global rate limits | `application/json` only |
| authenticated reads including `GET /auth/current` | valid browser cookie | none | same-origin only; no CORS | no body |
| authenticated browser mutations, including registration/UV options+verify, logout/session/passkey delete, recovery rotate, enrollment/Profile/device/command mutations | valid browser cookie | raw 32-byte value in `X-CSRF-Token`, constant-time digest match | exact Origin and same-origin Fetch Metadata as above | `application/json` only; no form/multipart/text |
| signed Device endpoints | Device session + request signature | none; browser cookie ignored | not browser-CSRF authority | operation-declared JSON only |

Successful normal login, recovery verification, recovery-to-normal
registration, UV/session privilege rotation responses and `GET /auth/current`
return the current raw CSRF value in
`Cache-Control:no-store` JSON. The server stores only SHA-256 digest; the
first-party client stores it in memory, never cookie/localStorage/IndexedDB.

## Bootstrap contract

```text
GET  /v1/bootstrap/status
POST /v1/bootstrap/options
POST /v1/bootstrap/verify
GET  /v1/bootstrap/ceremonies/{ceremonyId}
```

`bootstrap/status` reveals only `uninitialized|in_progress|initialized` and
expiry when applicable. It never returns token material.

`bootstrap/options` requires `Authorization: Bootstrap <value>` and:

```text
BootstrapOptionsRequest {
  displayName: string(1..128)
  anchor: {
    deviceId: UUIDv7
    kind: daemon
    name: string(1..128)
    platform: darwin | linux | windows
    architecture: string(1..32)
    clientVersion: string(1..64)
    signingPublicKey: base64url(32)
    exchangePublicKey: base64url(32)
    signingKeyDigest: base64url(32)
    exchangeKeyDigest: base64url(32)
    pinDigest: base64url(32)
    storageMode: portable_vault_v1
    keyEnvelopeAssertion: {
      formatVersion: 1
      keyRevision: 1
      recordRevision: integer>=1
      status: pending
      sealedAt: time
    }
    capabilities: DeviceCapability[]
  }
}
```

The anchor object above is also the exact `anchor` member of the strict public
`BootstrapAnchorDescriptorV1 {version:1,serverOrigin,anchor}` emitted by
`multidesk devices bootstrap prepare`. `serverOrigin` must exactly match the
configured HTTPS origin; it is not accepted as an origin override.

`kind=web|desktop`, missing portable-Vault-v1 Daemon metadata, digest mismatch,
or duplicate Device ID fails. The response returns `ceremonyId`, Passkey
creation options, 32-byte anchor challenge, server ephemeral X25519 public key,
expiry, full pin digest, and six-group fingerprint.

The options response is the strict public `BootstrapAnchorChallengeV1`:

```text
BootstrapAnchorChallengeV1 {
  version: 1
  ceremonyId: UUIDv7
  serverOrigin: exact configured HTTPS origin
  anchor: exact BootstrapAnchorDescriptorV1.anchor echo
  passkeyCreationOptions: bounded PublicKeyCredentialCreationOptions
  challenge: base64url(32)
  serverEphemeralExchangePublicKey: base64url(32)
  storageAssertionDigest: base64url(32)
  expiresAt: time
}
```

`bootstrap/verify` accepts the ceremony ID, Passkey response, 64-byte Ed25519
`signingProof`, and 32-byte X25519-HKDF-HMAC `exchangeProof` over the exact
`ExchangeKeyProofV1` transcript below. A single transaction consumes
SessionData/token, creates the user/Passkey/anchor/pin/recovery hashes/audit/
session and the public receipt below, and commits. The response returns recovery
codes once plus the receipt. Any failure creates none of those active records.

```text
BootstrapCommitReceiptV1 {
  version: 1
  type: bootstrap_commit_receipt
  ceremonyId: UUIDv7
  userId: UUIDv7
  anchorDeviceId: UUIDv7
  signingKeyDigest: base64url(32)
  exchangeKeyDigest: base64url(32)
  storageMode: portable_vault_v1
  storageAssertionDigest: base64url(32)
  signingProofDigest: base64url(32)
  exchangeProofDigest: base64url(32)
  activatedAt: time
}
```

`GET /bootstrap/ceremonies/{ceremonyId}` is strictly rate-limited and returns
only the exact public challenge while pending or the exact public commit receipt
after success; it never returns the bootstrap token, WebAuthn SessionData,
Passkey response, recovery plaintext, proofs, cookie, or CSRF value. The
Daemon `prove` command must re-fetch and byte-match the challenge over its
configured HTTPS/CA connection before signing. `activate` must re-fetch and
match the committed receipt before a local pending-to-active envelope CAS. The
receipt is a TLS-authenticated server commit fact, not a Device attestation.
There is no activation secret, connection credential, or device-session
material; later `/device-auth/*` PoP is mandatory.

Bootstrap token plaintext decodes to exactly 32 random bytes. The server stores
only `SHA-256(token)` and expiry, compares the recomputed digest in constant
time, deletes it on successful commit, and rejects expiry/replay. Password
hashing is not used for this high-entropy token.

`storageMode` and SHA-256 of restricted-JCS `keyEnvelopeAssertion` are signed in
both PoP proofs. They are a client assertion, not cryptographic evidence the
server can inspect at rest. Only official Daemon tests opening the local
`DeviceKeyEnvelopeV1` establish the invariant.

The P2 application commands are exactly `multidesk devices bootstrap prepare
--server ... --out ...`, `multidesk devices bootstrap prove --challenge ...
--out ...`, and `multidesk devices bootstrap activate --receipt ...` with the
absolute owner-only file/CA rules in `design.md`. Prepare must complete
migration `0008`, envelope creation/open, and Device mapping before options;
activate stores the receipt digest and reseals the unchanged key material in one
transaction. No later phase supplies a missing bootstrap dependency.

Lost/expired pre-user tokens are rotated only by the local executable command:

```text
multidesk-server bootstrap rotate --config <absolute-path> \
  --confirm-uninitialized
```

There is no corresponding HTTP operation. With server stopped and an exclusive
process/DB lock, it requires secure owner-only paths, confirms zero active user/
anchor, and atomically expires tokens/ceremonies, stores the new token digest/
ten-minute expiry, and appends a redacted pre-user audit row. It prints the new
32-byte token once after commit and refuses initialized/concurrent/unsafe/
unknown-schema states.

## Device, pin, and attestation DTOs

### Local `DeviceKeyEnvelopeV1` storage contract

P2 Device migration `0008_control_plane_remote_identity.sql` creates
`remote_device_identities` separately from credential `vault_items` plus the
generic `controlplane_id_mappings` table. Later trust/receipt/sync tables are not
a hidden bootstrap dependency: P3 migration `0009_remote_device_trust.sql`, P4
migration `0010_control_plane_sync.sql`, and P5 migration
`0011_remote_command_receipts.sql` add them in their own verified phases. The
strict JSON plaintext is no more than 4096 bytes and contains only
`version=1`, server Device UUIDv7, 32-byte
Ed25519 seed, 32-byte X25519 private key, both 32-byte public keys/full SHA-256
digests, `keyRevision=1`, `pending|active|retired`, and created/updated times.

The row contains a local `remote_identity_<hex>` ID distinct from the singleton
local IPC Device ID, server UUIDv7, canonical server origin, public metadata,
key/record revisions, lifecycle, AES-256-GCM payload/wrap names, independent
12-byte nonces and tagged ciphertexts, AAD/plaintext digests, timestamps, and a
safe quarantine reason. It reuses Vault-v1 KEK; every write uses a random
32-byte DEK and independent payload/wrap encryption. AAD is exactly:

```text
frame("multidesk-device-key-envelope-v1", "1", serverOrigin,
      localRemoteIdentityId, serverDeviceId, signingKeyDigestRaw,
      exchangeKeyDigestRaw, decimalKeyRevision)
```

Envelope + public metadata + Device UUID mapping commit with record-revision CAS
in one transaction before bootstrap/options. P2 owns and verifies that migration,
create/open API, mapping lookup, prepare/prove/activate actor, public receipt
storage, and pending -> active bootstrap reseal. Open recomputes keys/digests/
AAD and rejects/quarantines any mismatch. P3 reuses the API for enrollment and
adds active -> retired/replacement; every reseal uses fresh DEK/nonces and v1
never changes key material in place. Replacement creates a new UUID/row/
enrollment and then retires the old identity.

```text
DeviceKind = daemon | web | desktop
DeviceLifecycle = pending | active | revoked
DevicePresence = online | offline
DeviceStorageMode = portable_vault_v1 | native | software_wrapped |
                    metadata_only | desktop_key_store_deferred

DeviceCapability =
  mad.v1.metadata.read | mad.v1.metadata.write |
  mad.v1.sync.pull | mad.v1.sync.push |
  mad.v1.presence.write | mad.v1.device.enroll_request |
  mad.v1.device.enroll_approve | mad.v1.device.revoke |
  mad.v1.session.command_create | mad.v1.session.command_claim |
  mad.v1.session.command_ack | mad.v1.session.command_result

Device {
  id: UUIDv7
  kind: DeviceKind
  name: string
  platform: string
  architecture: string
  clientVersion: string
  signingPublicKey: base64url(32)
  exchangePublicKey: base64url(32)
  signingKeyDigest: base64url(32)
  exchangeKeyDigest: base64url(32)
  pinDigest: base64url(32)
  fingerprint: "XXXX-XXXX-XXXX-XXXX-XXXX-XXXX"
  storageMode: DeviceStorageMode
  declaredCapabilities: string[]
  effectiveCapabilities: DeviceCapability[]
  lifecycle: DeviceLifecycle
  presence: DevicePresence
  serverBootEpoch?: UUIDv7
  keyRevision: integer>=1
  capabilityRevision: integer>=1
  revision: integer>=1
  lastAuthenticatedAt?: time
  lastSeenAt?: time
  createdAt: time
  updatedAt: time
}

DeviceAttestationV1 {
  version: 1
  type: device_attestation
  attestationId: UUIDv7
  approverDeviceId: UUIDv7
  subjectDeviceId: UUIDv7
  subjectSigningKeyDigest: base64url(32)
  subjectExchangeKeyDigest: base64url(32)
  capabilities: DeviceCapability[] // canonical sorted
  issuedAt: time
  expiresAt: time                // <= issuedAt + 10 minutes
}

DeviceCapabilityAttestationV1 {
  version: 1
  type: device_capability_attestation
  attestationId: UUIDv7
  approverDeviceId: UUIDv7
  subjectDeviceId: UUIDv7
  subjectSigningKeyDigest: base64url(32)
  subjectExchangeKeyDigest: base64url(32)
  previousCapabilityRevision: integer>=1
  capabilityRevision: previousCapabilityRevision + 1
  capabilities: DeviceCapability[] // complete, canonical, recognized set
  issuedAt: time
  expiresAt: time
}

ExchangeKeyProofV1 {
  purpose: bootstrap | enrollment
  ceremonyId: UUIDv7
  subjectDeviceId: UUIDv7
  subjectSigningPublicKey: base64url(32)
  subjectExchangePublicKey: base64url(32)
  storageMode: DeviceStorageMode
  storageAssertionDigest: base64url(32)
  serverEphemeralExchangePublicKey: base64url(32)
  challenge: base64url(32)
  expiresAt: time
  signingProof: base64url(64)
  exchangeProof: base64url(32)
}
```

All declared capability strings must match
`^mad\.v[1-9][0-9]*\.[a-z][a-z0-9]*(?:\.[a-z][a-z0-9_]*)+$`.
Well-formed unknown strings are preserved in `declaredCapabilities` but omitted
from `effectiveCapabilities` and never authorized/delegated. Phase 4b reserves
`mad.v1.session.realtime`, `mad.v1.session.decrypt`,
`mad.v1.terminal.control`, and `mad.v1.approval.respond`; Phase 5 reserves
`mad.v1.credentials.store`, `mad.v1.credentials.grant`, and
`mad.v1.credentials.receive`. Phase 4a does not recognize/effect them. A later
server must recognize the name and then use capability-attestation elevation;
it may not reinterpret an already preserved unknown string as previously
granted.

| Kind/storage | Phase 4a maximum effective set |
|---|---|
| Daemon / `portable_vault_v1` | metadata read/write, sync pull/push, presence, enroll request/approve, revoke, command claim/ack/result |
| Web / `native|software_wrapped` | metadata read, Profile write, command create, enroll request; approve/revoke only when separately attested |
| Web / `metadata_only` | browser-session metadata read only; cannot become an active Device or attest/elevate |
| Desktop / `desktop_key_store_deferred` | server contract fixture accepts Web-like metadata capabilities; no Phase 4a product/client support claim |

Same-key elevation endpoint requires directly pinned approver signature,
explicit user confirmation, eligible recognized complete capability set, and a
monotonic capability revision. It changes no key revision/UUID. Key replacement
always creates a new Device identity.

The detached signature is Ed25519 over the domain-framed JCS object. P0 reuses
and upgrades the existing independent Go/TypeScript restricted RFC 8785 vector
codecs; production imports those reviewed codec implementations for this typed
attestation subset only. The subset permits schema-known object members,
strings, non-negative safe integers, and arrays; it forbids floats, arbitrary
maps, duplicate/unknown keys, and caller-selected member names. Negative vectors
cover ordering, escaping, Unicode, integer boundaries, unknown members, and
cross-language byte equality. No general canonicalizer is selected during
build. Raw subject keys accompany enrollment and must hash to the signed full
digests.

The proof transcript and calculations are exactly:

```text
popContext = frame("multidesk-x25519-pop-context-v1", "v1", purpose,
  ceremonyId, subjectDeviceId, subjectSigningPublicKeyRaw,
  subjectExchangePublicKeyRaw, storageMode, storageAssertionDigestRaw,
  serverEphemeralExchangePublicKeyRaw,
  challengeRaw, expiresAtRFC3339UTC)
sharedSecret = X25519(subjectExchangePrivateKey,
                      serverEphemeralExchangePublicKey)
popSalt = SHA-256(frame("multidesk-x25519-pop-salt-v1",
                       ceremonyId, challengeRaw))
popKey = HKDF-SHA256(sharedSecret, popSalt, popContext, 32)
exchangeProof = HMAC-SHA256(popKey,
  frame("multidesk-x25519-pop-proof-v1", popContext))
signingProof = Ed25519.Sign(subjectSigningPrivateKey,
  frame("multidesk-ed25519-pop-proof-v1", popContext))
```

The server ephemeral private key is memory-only. All-zero shared secrets and
non-constant-time proof checks are forbidden. Restart invalidates every
incomplete ceremony and its stored WebAuthn SessionData; clients must begin a
new ceremony. No proof secret/challenge is logged or persisted after consume.

### Enrollment/device endpoints

```text
GET    /v1/devices
GET    /v1/devices/{deviceId}
POST   /v1/device-enrollments
GET    /v1/device-enrollments/{enrollmentId}
POST   /v1/device-enrollments/{enrollmentId}/prove
POST   /v1/device-enrollments/{enrollmentId}/approve
GET    /v1/device-enrollments/{enrollmentId}/activation-package
POST   /v1/device-enrollments/{enrollmentId}/activate
POST   /v1/device-enrollments/{enrollmentId}/cancel
GET    /v1/device-enrollments/{enrollmentId}/resume
POST   /v1/devices/{deviceId}/capabilities
POST   /v1/devices/{deviceId}/revoke
```

```text
EnrollmentCreate {
  subject: Device key/public metadata + storage assertion
  requestedCapabilities: string[]
}
EnrollmentCreated {
  enrollmentId: UUIDv7
  subjectTranscript: exact public transcript
  pinDigest: base64url(32)
  fingerprint: six groups
  challenge: base64url(32)
  serverEphemeralExchangePublicKey: base64url(32)
  expiresAt: time
  state: pending_proof
}
EnrollmentProofRequest { ExchangeKeyProofV1 }
EnrollmentApproveRequestV1 {
  enrollmentId: UUIDv7
  transcriptDigest: base64url(32)
  fingerprintConfirmed: true
  attestation: DeviceAttestationV1
  attestationSignature: base64url(64)
  receipt: ActivationReceiptV1
  receiptSignature: base64url(64)
  approverKeys: ApproverPublicKeysV1
}
ActivationReceiptV1 {
  version: 1
  type: device_activation_receipt
  enrollmentId: UUIDv7
  serverOrigin: CanonicalServerOriginV1
  subjectDeviceId: UUIDv7
  approverDeviceId: UUIDv7
  subjectSigningKeyDigest: base64url(32)
  subjectExchangeKeyDigest: base64url(32)
  approverSigningKeyDigest: base64url(32)
  approverExchangeKeyDigest: base64url(32)
  transcriptDigest: base64url(32)
  attestationDigest: base64url(32)
  capabilities: DeviceCapability[]
  capabilityRevision: 1
  approvedAt: time
  expiresAt: time
}
ApproverPublicKeysV1 {
  signingPublicKey: base64url(32)
  exchangePublicKey: base64url(32)
}
EnrollmentActivationPackageV1 {
  version: 1
  type: enrollment_activation_package
  enrollmentId: UUIDv7
  state: approved
  enrollmentRevision: integer>=1
  transcript: EnrollmentTranscriptV1
  transcriptDigest: base64url(32)
  attestation: DeviceAttestationV1
  attestationDigest: base64url(32)
  attestationSignature: base64url(64)
  receipt: ActivationReceiptV1
  receiptDigest: base64url(32)
  receiptSignature: base64url(64)
  approverKeys: ApproverPublicKeysV1
}
EnrollmentActivationPackageResultV1 {
  activationPackage: EnrollmentActivationPackageV1
  activationPackageDigest: base64url(32)
}
EnrollmentActivateRequestV1 {
  ack: SubjectActivationAckV1
  subjectActivationSignature: base64url(64)
  expectedEnrollmentRevision: integer>=1
}
EnrollmentActivateResultV1 {
  enrollmentId: UUIDv7
  state: activated
  device: Device
  receipt: ActivationReceiptV1
  receiptDigest: base64url(32)
  activationPackageDigest: base64url(32)
  activatedAt: time
  enrollmentRevision: integer>=1
}
```

Every object above has `additionalProperties:false`. `ActivationReceiptV1` is
the complete signed restricted-JCS payload: it contains digests but no raw
public key, detached signature, or transport wrapper. The raw approver keys,
attestation payload/signature, receipt signature, and transcript are carried
only by `EnrollmentActivationPackageV1`. Before returning or accepting the
package, the server and candidate decode both raw approver keys, recompute their
SHA-256 digests, and require exact equality with the signed receipt's
`approverSigningKeyDigest`/`approverExchangeKeyDigest`; the attestation and
receipt subject/approver/transcript digests must also agree.

The package digest is external to the package to avoid self-reference:

```text
activationPackageDigest = SHA-256(frame(
  "multidesk-enrollment-activation-package-digest-v1", "1",
  JCS(EnrollmentActivationPackageV1)))
```

Enrollment expiry is ten minutes. Candidate Daemon CLI or Web invokes create,
prove, resume/activate/cancel; a directly pinned eligible Device invokes
approve after recomputing the transcript and persisting the candidate pin
locally. Every mutation uses Idempotency-Key; exact replay returns current
state/receipt, not a new secret. `resume` returns public state/transcript only.
`activate` accepts only `EnrollmentActivateRequestV1` and returns only
`EnrollmentActivateResultV1`—no connection credential. Candidate verifies the
package/receipt/attestation/raw-key digests and signatures and persists the
approver pin before signing the exact SubjectActivationAck and before local
active state. Device-auth
challenge/PoP later creates the short session. Changed keys under an existing
Device ID always return `device_key_changed`; v1 has no key-update mutation.

Daemon CLI sequence and flags are exactly the `pair start`, anchor `pair
approve`, candidate `pair activate` flow in `design.md`. HTTPS/CA and TTY/
noninteractive fingerprint rules are application-service acceptance, not an
alternate API. Web uses the same DTOs and a Daemon approver. Cancelled/expired
states are terminal. Server restart invalidates proof ceremonies; `resume`
instructs the candidate to restart proof without changing the pending identity.

Capability elevation accepts `DeviceCapabilityAttestationV1` plus signature,
requires `If-Match` and Idempotency-Key, a directly pinned eligible approver,
explicit user confirmation, current key digests, `capabilityRevision + 1`, and
recognized kind/storage-eligible complete capabilities. It changes no
keyRevision/UUID.

Device list filters: `kind`, `status`, `capability`; sorts: `name`,
`lastSeenAt`, `createdAt`. Default sort is `createdAt asc, id asc`.

## Signed Device authentication

```text
POST /v1/device-auth/challenges
POST /v1/device-auth/exchange
```

`challenges` accepts Device ID and returns a 32-byte challenge plus UUIDv7 ID
valid for 60 seconds. `exchange` supplies the Device signature and returns a
15-minute opaque device-session token. Only its digest is stored.

Authenticated Device requests send:

```text
Authorization: Device <opaque-session-token>
X-MAD-Device-ID: <UUIDv7>
X-MAD-Timestamp: <RFC3339>
X-MAD-Nonce: <base64url-16-random-bytes>
X-MAD-Content-SHA256: <base64url-32>
X-MAD-Signature: <base64url-64>
```

The signed input is the framed canonical request defined in `design.md`.
Canonical query sorts percent-decoded names/values and then emits RFC 3986
encoding; duplicate parameters are rejected unless the operation explicitly
defines an array. Proxy-forwarded host/proto is accepted only from configured
trusted proxy addresses and never changes the RP origin dynamically.

## Metadata resource DTOs

All fields below are the complete public allowlist. Generated schemas set
`additionalProperties: false`.

```text
AccountProjection {
  id: UUIDv7
  provider: codex | claude
  displayName: string(1..128)
  providerSubjectDigest?: hex-sha256
  subscriptionHint?: string(0..64)
  enabled: boolean
  revision: integer>=1
  createdAt: time
  updatedAt: time
}

CredentialStatusProjection {
  id: UUIDv7
  accountId: UUIDv7
  deviceId: UUIDv7
  authMethod: interactive | device_code | api_key | setup_token
  status: healthy | expired | revoked | unknown
  availability: available | limited | unavailable | unknown
  credentialRevision: integer>=0
  lastValidatedAt?: time
  expiresAt?: time
  updatedAt: time
}

ProfileProjection {
  id: UUIDv7
  deviceId: UUIDv7
  accountId?: UUIDv7
  provider: codex | claude
  name: string(1..128)
  selectorAlias?: string(1..32)
  enabled: boolean
  modelPreference?: string(1..128)
  environmentNonSecret: map<string,string> // bounded/denylisted
  mcpRefKeys: string[]
  skillRefKeys: string[]
  hookRefKeys: string[]
  revision: integer>=1
  createdAt: time
  updatedAt: time
}

WorkspaceProjection {
  id: UUIDv7
  deviceId: UUIDv7
  label: string(0..256)
  tags: string[]
  providerDefaults: map<codex|claude, UUIDv7> // mapped Profile IDs only
  revision: integer>=1
  createdAt: time
  updatedAt: time
}

SessionProjection {
  id: UUIDv7
  deviceId: UUIDv7
  provider: codex | claude
  accountId?: UUIDv7
  credentialStatusId?: UUIDv7
  runtimeProfileId: UUIDv7
  workspaceId: UUIDv7
  resumedFromSessionId?: UUIDv7
  status: starting | running | stopping | exited | failed | killed
  capabilitySnapshot: string[]
  startedAt: time
  endedAt?: time
  exitCode?: integer
  failureCode?: string(0..64)
  revision: integer>=1
  updatedAt: time
}

UsageProjection {
  id: UUIDv7
  provider: codex | claude
  providerVersion: string(1..128)
  accountId: UUIDv7
  credentialStatusId?: UUIDv7
  deviceId: UUIDv7
  source: official | cli_derived | local_estimate | unofficial | unavailable
  confidence: high | medium | low | none
  availability: available | limited | unavailable | unknown
  sourceVersion?: string(0..128)
  capabilityStatus: supported | unavailable | schema_changed | error
  errorCode?: string(0..64)
  observedAt: time
  staleAt: time
  windows: UsageWindow[]
}

UsageWindow {
  providerLimitId?: string(0..128)
  kind: rolling | calendar | spend_control | sdk_credit | unknown
  label: string(1..128)
  unit: request | token | second | usd | provider_unit | percent
  scale: integer[0,9] // USD requires 6
  usedScaled?: canonical non-negative decimal string <= int64 max
  limitScaled?: canonical non-negative decimal string <= int64 max
  remainingBasisPoints?: integer[0,10000]
  resetsAt?: time
}
```

`secretRef`, `secretDigest`, real workspace path, Provider auth path/home,
Provider session ID, raw Provider response, terminal/model/Approval text, and
unknown settings are forbidden at decode, domain, store, logging, and response
boundaries.

### Metadata endpoints and allowlists

```text
GET    /v1/accounts
GET    /v1/accounts/{id}
GET    /v1/credential-statuses
GET    /v1/profiles
GET    /v1/profiles/{id}
POST   /v1/profiles
PATCH  /v1/profiles/{id}
DELETE /v1/profiles/{id}
GET    /v1/workspaces
GET    /v1/sessions
GET    /v1/sessions/{id}
GET    /v1/usage
GET    /v1/audit-events
```

| Resource | Filters | Sorts (default first) |
|---|---|---|
| Accounts | `provider`, `enabled` | `displayName`, `updatedAt` |
| Credential statuses | `accountId`, `deviceId`, `status`, `availability` | `updatedAt` |
| Profiles | `provider`, `accountId`, `deviceId`, `enabled` | `name`, `updatedAt` |
| Workspaces | `deviceId`, `tag` | `label`, `updatedAt` |
| Sessions | `deviceId`, `provider`, `accountId`, `status` | `startedAt desc`, `updatedAt` |
| Usage | `deviceId`, `provider`, `accountId`, `source`, `availability` | `observedAt desc` |
| Audit | `actorId`, `action`, `targetType`, `decision`, time range | `createdAt desc` |

Profiles are the only browser-editable metadata in Phase 4a. Device-originated
sync owns Account/Credential/Workspace/Session/Usage facts. Browser attempts to
mutate those projections return `projection_read_only`.

`ProfileCreateRequest` requires `targetDeviceId`, provider, mapped Account ID
when applicable, and only the allowlisted Profile fields. The server allocates
the Profile UUIDv7 after verifying the target is an active owned Daemon, the
Account is visible/eligible on that target, provider relations match, and no
secret/path field exists. The Profile is server-created/target-owned; only the
target Daemon materializes a new prefixed local Profile ID and mapping. Patch/
delete require the same target ownership/relation validation.

## Sync DTOs and endpoints

```text
POST /v1/device/sync/push
GET  /v1/device/sync/snapshot?cursor=&limit=
POST /v1/device/sync/snapshot/commit
GET  /v1/device/sync/pull?cursor=&limit=&waitSeconds=
POST /v1/device/sync/ack
```

```text
SyncPushRequest {
  batchId: UUIDv7
  changes: SyncChange[1..100]
}

SyncChange {
  changeId: UUIDv7
  baseRevision: integer>=0
  fullBase: CanonicalSyncRevisionV1 | null
  baseDigest: base64url(32)
  fullNext: CanonicalSyncRevisionV1
  nextDigest: base64url(32)
  patch: SyncPatchV1
  observedAt: time
}

CanonicalSyncRevisionV1 {
  schemaVersion: 1
  resourceType: account | credential_status | profile | workspace |
                session | usage
  resourceId: UUIDv7
  revision: integer[1, 9007199254740991]
  operation: upsert | delete
  value: AccountSyncValueV1 | CredentialStatusSyncValueV1 |
         ProfileSyncValueV1 | WorkspaceSyncValueV1 |
         SessionSyncValueV1 | UsageSyncValueV1 | null
}

SyncPatchV1 {
  operations: SyncPatchOperationV1[0..128]
  patchDigest: base64url(32)
}
SyncPatchOperationV1 {
  op: add | remove | replace
  path: canonical RFC6901 pointer, 0..256 UTF-8 bytes
  beforeDigest?: base64url(32)
  afterDigest?: base64url(32)
}

SyncPushResult {
  batchId: UUIDv7
  results: {
    changeId: UUIDv7
    status: applied | duplicate | conflict | rejected
    revision?: integer
    error?: Error
    conflict?: SyncConflict
  }[]
  serverCursor: string
}

SyncConflict {
  resourceType: allowed resource type
  resourceId: UUIDv7
  baseRevision: integer
  currentRevision: integer
  fullBase: CanonicalSyncRevisionV1 | null
  fullCurrent: CanonicalSyncRevisionV1
  fullNext: CanonicalSyncRevisionV1
  baseToCurrentPatch: SyncPatchV1
  baseToNextPatch: SyncPatchV1
}

ServerChangeV1 {
  serverChangeRevision: integer>=1
  changeId: UUIDv7
  baseRevision: integer>=0
  fullBase: CanonicalSyncRevisionV1 | null
  baseDigest: base64url(32)
  fullNext: CanonicalSyncRevisionV1
  nextDigest: base64url(32)
  patch: SyncPatchV1
}

SyncPullResult {
  changes: ServerChangeV1[0..100]
  nextCursor: string
  hasMore: boolean
}

SyncAckRequest { cursor: string }

SnapshotManifestEntryV1 {
  ordinal: integer[0, 2^53-1]
  resourceType: allowed sync resource type
  resourceId: UUIDv7
  revision: integer[1, 2^53-1]
  revisionDigest: base64url(32)
}

SnapshotManifestV1 {
  schemaVersion: 1
  snapshotId: UUIDv7
  snapshotEpoch: UUIDv7
  targetDeviceId: UUIDv7
  pageSize: integer[1,4]
  pageCount: integer>=1
  resourceCount: integer>=0
  resources: SnapshotManifestEntryV1[] // complete ordered manifest
  expiresAt: time
  incrementalBaseCursor: string
}

SnapshotContinuationV1 =
  { kind: next, nextPageToken: base64url(32) } |
  { kind: final }

SnapshotPageDigestInputV1 {
  schemaVersion: 1
  snapshotId: UUIDv7
  snapshotEpoch: UUIDv7
  targetDeviceId: UUIDv7
  manifestDigest: base64url(32)
  pageSize: integer[1,4]
  pageIndex: integer>=0
  pageCount: integer>=1
  resourceCount: integer>=0
  priorPageDigest: base64url(32) | null
  resources: CanonicalSyncRevisionV1[0..4]
  continuation: SnapshotContinuationV1
  expiresAt: time
  incrementalBaseCursor: string
}

SnapshotFinalDigestInputV1 {
  schemaVersion: 1
  snapshotId: UUIDv7
  snapshotEpoch: UUIDv7
  targetDeviceId: UUIDv7
  manifestDigest: base64url(32)
  pageCount: integer>=1
  resourceCount: integer>=0
  firstPageDigest: base64url(32)
  lastPageDigest: base64url(32)
  expiresAt: time
  incrementalBaseCursor: string
}

SyncSnapshotPage {
  page: SnapshotPageDigestInputV1
  pageDigest: base64url(32)
  finalSnapshotDigest?: base64url(32) // required only for final continuation
}

SyncSnapshotCommit {
  snapshotId: UUIDv7
  snapshotEpoch: UUIDv7
  targetDeviceId: UUIDv7
  manifestDigest: base64url(32)
  finalSnapshotDigest: base64url(32)
  lastPageDigest: base64url(32)
  pageCount: integer>=1
  resourceCount: integer>=0
  appliedBaseCursor: string
}

SyncSnapshotCommitResult {
  snapshotId: UUIDv7
  finalSnapshotDigest: base64url(32)
  appliedBaseCursor: string
  committedAt: time
}
```

The six named `*SyncValueV1` schemas are exact OpenAPI objects, not arbitrary
JSON: each equals its matching projection above with `id` and its server
`revision` member removed, while retaining all relation IDs and timestamps.
For CredentialStatus and Usage, which have no projection-level server revision
member, only `id` is removed. The `resourceType` discriminator must select that
one value schema. Every value schema has `additionalProperties:false`; a
canonical full revision is limited to 192 KiB.

Canonicalization is RFC 8785 JCS after exact-schema validation. Object keys use
RFC 8785 UTF-16 ordering, arrays preserve order, strings are not Unicode-
normalized, and finite binary64 numbers use the RFC 8785/ECMAScript shortest
form with `-0` encoded as `0`. Duplicate/unknown members, non-finite numbers,
integers outside the safe range, or lossy conversion fail. Exact revision and
create-base digests are:

```text
revisionDigest = SHA-256(frame(
  "multidesk-sync-resource-revision-v1", "1", resourceType,
  resourceId, decimalRevision, JCS(CanonicalSyncRevisionV1)))

createBaseDigest = SHA-256(frame(
  "multidesk-sync-create-base-v1", "1", resourceType, resourceId))
```

Create, and only create, requires `baseRevision=0`, `fullBase=null`, the exact
create-base digest, `fullNext.revision=1`, `operation=upsert`, no live/history
row, and no deletion watermark. There is no revision-zero history row. Update
or delete requires `baseRevision>=1`, exact matching type/ID/revision in
`fullBase`, computed base digest equal to the request and lifetime stored
`resource_revision_digests` row, and `fullNext.revision=baseRevision+1`.
Update uses `upsert` plus the matching value; delete uses `delete,value=null`.
`nextDigest` is the exact revision digest of `fullNext`.

Patch is a checksum/diagnostic, not an apply instruction. Create emits one root
`add`; delete one root `remove`. Otherwise canonical-equal values emit nothing;
fixed-schema objects and schema-declared maps recurse over the union of keys in
RFC 8785 order; add/remove represents absent/present members; scalars, type
changes, and arrays at every nesting depth are atomic `replace` operations.
Paths use RFC 6901 escaping and root `""`. `beforeDigest` is required only for
remove/replace, `afterDigest` only for add/replace, and each is
`SHA-256(frame("multidesk-sync-patch-value-v1","1",JCS(subtree)))`.
`patchDigest` is
`SHA-256(frame("multidesk-sync-patch-v1","1",JCS(operations)))`. The patch is
limited to 128 operations, 256-byte pointers, and 16 KiB canonical bytes. The
server recomputes it byte-for-byte; mismatch/overflow rejects the whole batch.

`waitSeconds` is 0..25. The server retains the revision digest for every
accepted revision for the database lifetime. A missing required history row is
`sync_history_missing`: the whole batch writes nothing, advances no cursor, and
the authenticated Device becomes `snapshot_required`; no base or diff is
guessed. Any malformed, forbidden, relation-invalid, base/next-digest-invalid,
or patch-invalid change likewise rejects the whole request. Otherwise one
transaction applies every nonconflicting change, emits cursor rows only for
those applies, returns conflicts (including dependency conflicts) for the rest,
and stores the complete batch result for dedupe. A stale but valid base returns
the exact full base/current/next and two deterministic patches above. An
explicit pull ack advances only applied rows. Credential grants and secret-
bearing fields have no representation.

Initial enrollment, re-enrollment, and restored Device state require snapshot
before incremental pull. The signed caller must already be the same active,
enrolled `targetDeviceId`; its envelope, Device mapping, pin, and activation
receipt are an out-of-band prerequisite. Device is not a
`CanonicalSyncRevisionV1` member and never appears in snapshot resources. The
exact resource order is Account, Credential status, Profile, Workspace,
Session, Usage, then lowercase UUIDv7 ascending within each type rank. The
latest authorized upsert/delete revision appears exactly once.

At creation the server freezes a ten-minute snapshot, the current server
`snapshotEpoch`, target, requested page size, `incrementalBaseCursor`, and the
complete ordered manifest. `pageCount` is
`max(1,ceil(resourceCount/pageSize))`; page `i` is exactly the manifest slice
`[i*pageSize,min((i+1)*pageSize,resourceCount))`. Non-final pages are full; a
non-empty final page has 1..pageSize resources; an empty snapshot is exactly one
final page at index zero with an empty resource array. Manifest entry digest is
the canonical revision digest already frozen above. Exact digests are:

```text
manifestDigest = SHA-256(frame(
  "multidesk-sync-snapshot-manifest-v1", "1", JCS(SnapshotManifestV1)))

pageDigest = SHA-256(frame(
  "multidesk-sync-snapshot-page-v1", "1",
  JCS(SnapshotPageDigestInputV1)))

finalSnapshotDigest = SHA-256(frame(
  "multidesk-sync-snapshot-final-v1", "1",
  JCS(SnapshotFinalDigestInputV1)))
```

All three use the exact strict RFC 8785 rules frozen for sync revisions.
`priorPageDigest` is null only for page zero and otherwise equals the preceding
page digest. Every page binds the same manifest, epoch, target, expiry, and base
cursor. A non-final page has only the `next` continuation with its persisted
opaque token; the last page has only `final` and the final digest. For an empty
snapshot first/last page digest are equal. The final input binds first and last
page digests; the last-page chain plus manifest binds every page/resource/
boundary/token and the incremental cursor.

Only one unexpired uncommitted snapshot exists per target Device. A cursorless
request with the same `limit` returns byte-identical `SyncSnapshotPage` data for
page zero; another limit
returns `snapshot_in_progress`. Each next-page token is persisted and bound to
target/snapshot/epoch/manifest/next page index/prior digest. Its replay before
expiry returns byte-identical `SyncSnapshotPage` data. The outer success-
envelope `requestId` may differ and is excluded from all snapshot digests.
Mixed epoch/snapshot/target, page
reorder/omit/duplicate/truncate, wrong prior digest, premature final marker,
resource/manifest mismatch, or token/base-cursor substitution fails without
advancing state. Expiry returns `snapshot_expired`, releases the active slot,
and requires a new snapshot.

The daemon stages all pages, reconstructs and verifies the manifest/order/
slices, page chain, canonical resource digests, topology/parents/mappings,
final digest, and base cursor, then atomically applies projections/mappings and
installs that cursor before commit. Commit requires signed Device auth and
Idempotency-Key and binds target, snapshot/epoch, manifest/final/last-page
digests, counts, and applied cursor. First valid commit atomically marks the
snapshot committed. Exact request replay returns the same
`SyncSnapshotCommitResult`. Same Idempotency-Key plus a changed body is
`idempotency_key_reused`; a fresh key plus any changed epoch/digest/count/cursor/
body for an already committed snapshot is `snapshot_commit_conflict`.
Committed-result replay uses the ordinary 24-hour idempotency retention even
after page expiry. Missing parent, wrong target/type, or existing UUID binding
mismatch blocks commit and quarantines locally.
Browser-created Profile includes `targetDeviceId`; server validates its Account/
provider/target relations, and only that target allocates the correct prefixed
local ID + mapping + projection in one transaction.

Delete pulls contain:

```text
Tombstone {
  resourceType: allowed type
  resourceId: UUIDv7
  finalRevision: integer>=1
  digest: base64url(32)
  deletedAt: time
}
```

After all frozen eligible Device acks plus 30 days, tombstone payload/ack rows
may be collected. `resource_deletion_watermarks(resourceType,resourceId,
finalRevision,digest,deletedAt)` remains for the server database lifetime.
Every create/upsert checks it; a deleted UUID is never reused and stale restored
outbox entries are quarantined. Server backups must preserve watermarks and
revision digests; missing/mixed-epoch restore is schema-incompatible.

## Presence and revocation

```text
POST /v1/device/presence/heartbeat
POST /v1/devices/{deviceId}/revoke
```

Heartbeat body contains client version, current server-sync cursor, command
cursor, and bounded capability health only. It never contains Provider health
payloads or local paths. Authenticated heartbeat sets `lastSeenAt`; only an
authenticated request within 60 seconds renders online.

Revocation requires browser CSRF + `If-Match` + Idempotency-Key, writes the
revocation/audit transaction, invalidates device sessions/challenges/claims,
wakes long polls, blocks future sync/command/auth, and marks presence revoked.
It does not delete metadata or claim remote erasure.

## Session Command v0.7 DTOs and endpoints

```text
POST /v1/session-commands
GET  /v1/session-commands
GET  /v1/session-commands/{commandId}

GET  /v1/device/session-commands?cursor=&limit=&waitSeconds=
GET  /v1/device/session-commands/{commandId}
POST /v1/device/session-commands/{commandId}/claim
POST /v1/device/session-commands/{commandId}/ack
POST /v1/device/session-commands/{commandId}/result
POST /v1/device/session-commands/{commandId}/reconcile
```

```text
SessionCommandKind = start | stop | kill | resume |
                     acquire_control | release_control
SessionCommandState = queued | claimed | acknowledged |
                      succeeded | failed | unsupported | expired

SessionCommandCreateV1 {
  targetDeviceId: UUIDv7
  kind: SessionCommandKind
  expiresInSeconds?: integer[30,900] // default 300
  request:
    start { provider, accountId?, credentialStatusId?, runtimeProfileId,
            workspaceId }
    stop|kill|acquire_control|release_control { sessionId }
    resume { sessionId }
}

CanonicalSessionCommandRequestV1 {
  schemaVersion: 1
  commandId: UUIDv7
  resultSessionId?: UUIDv7 // required for start|resume; forbidden otherwise
  creatorClass: browser | signed_device
  creatorId: UUIDv7
  targetDeviceId: UUIDv7
  kind: SessionCommandKind
  request: exact SessionCommandCreateV1.request branch selected by kind
  resourceRevisions: ResourceRevisionRefV1[1..16]
  createdAt: time
  expiresAt: time
  ttlSeconds: integer[30,900]
}

ResourceRevisionRefV1 {
  resourceType: account | credential_status | profile | workspace | session
  resourceId: UUIDv7
  serverSyncRevision: integer>=1
}

SessionCommandV1 {
  id: UUIDv7
  targetDeviceId: UUIDv7
  kind: SessionCommandKind
  state: SessionCommandState
  requestDigest: base64url(32)
  currentAttempt: integer[0,8]
  currentDeliveryRevision?: integer>=1
  claimExpiresAt?: time
  acknowledgedAt?: time
  result?: SessionCommandOutcomeV1
  expiresAt: time
  createdAt: time
  updatedAt: time
  revision: integer>=1
}

SessionCommandOfferV1 {
  command: CanonicalSessionCommandRequestV1
  requestDigest: base64url(32)
  deliveryRevision: integer>=1
  expectedNextAttempt: integer[1,8]
  offeredAt: time
  commandExpiresAt: time
}

SessionCommandDeliveryListResultV1 {
  committedCursor: opaque command cursor
  scanCursor: opaque command scan cursor
  offers: SessionCommandOfferV1[0..50]
  nextScanCursor: opaque command scan cursor
  hasMore: boolean
}

SessionCommandClaimRequestV1 {
  targetDeviceId: UUIDv7
  commandId: UUIDv7
  requestDigest: base64url(32)
  deliveryRevision: integer>=1
  expectedNextAttempt: integer[1,8]
  expectedCommandRevision: integer>=1
  derivedIdempotencyKey: base64url(32) // formula uses receiptRevision=0
}
SessionCommandClaimResultV1 {
  targetDeviceId: UUIDv7
  commandId: UUIDv7
  requestDigest: base64url(32)
  deliveryRevision: integer>=1
  attempt: integer[1,8]
  claimExpiresAt: time
  command: CanonicalSessionCommandRequestV1
  commandRevision: integer>=1
  committedCursor: opaque command cursor // unchanged by claim
}

SessionCommandAckRequestV1 {
  targetDeviceId: UUIDv7
  commandId: UUIDv7
  requestDigest: base64url(32)
  deliveryRevision: integer>=1
  attempt: integer[1,8]
  claimExpiresAt: time
  receiptRevision: integer>=1
  receipt: ReservedReceiptV1
  receiptDigest: base64url(32)
  expectedCommandRevision: integer>=1
  expectedCommittedCursor: opaque command cursor
  derivedIdempotencyKey: base64url(32)
}
SessionCommandAckResultV1 {
  command: SessionCommandV1
  deliveryRevision: integer>=1
  attempt: integer[1,8]
  receiptRevision: integer>=1
  receiptDigest: base64url(32)
  cursorCommit: DeviceCommandCursorCommitV1
}

DeviceCommandCursorCommitV1 {
  previousCommittedDeliveryRevision: integer>=0
  committedDeliveryRevision: integer>=previous
  committedCursor: opaque command cursor
}

SessionCommandResultRequestV1 {
  targetDeviceId: UUIDv7
  commandId: UUIDv7
  requestDigest: base64url(32)
  deliveryRevision: integer>=1
  attempt: integer[1,8]
  claimExpiresAt: time
  receiptRevision: integer>=1
  receipt: LocalCommittedReceiptV1 | AmbiguousReceiptV1
  receiptDigest: base64url(32)
  outcome: SessionCommandOutcomeV1 // byte-equal to receipt safe result;
                                   // ambiguous has its sole failed outcome
  expectedCommandRevision: integer>=1
  derivedIdempotencyKey: base64url(32)
}
SessionCommandResultResultV1 {
  command: SessionCommandV1
  deliveryRevision: integer>=1
  attempt: integer[1,8]
  receiptRevision: integer>=1
  receiptDigest: base64url(32)
  acceptedCommandRevision: integer>=1
  outcome: SessionCommandOutcomeV1
}

SessionCommandReconcileRequestV1 {
  targetDeviceId: UUIDv7
  commandId: UUIDv7
  requestDigest: base64url(32)
  currentDeliveryRevision: integer>=1
  currentClaimAttempt: integer[1,8]
  currentClaimExpiresAt: time
  receiptDeliveryRevision: integer>=1
  receiptAttempt: integer[1,8]       // historical attempt; < current
  receiptRevision: integer>=1
  receipt: LocalCommittedReceiptV1 | AmbiguousReceiptV1
  receiptDigest: base64url(32)
  outcome: SessionCommandOutcomeV1
  expectedCommandRevision: integer>=1
  derivedIdempotencyKey: base64url(32)
}
SessionCommandReconcileResultV1 {
  command: SessionCommandV1
  currentDeliveryRevision: integer>=1
  currentClaimAttempt: integer[1,8]
  receiptDeliveryRevision: integer>=1
  receiptAttempt: integer[1,8]
  receiptRevision: integer>=1
  receiptDigest: base64url(32)
  acceptedCommandRevision: integer>=1
  outcome: SessionCommandOutcomeV1
}

DeviceCommandStateV1 {
  command: SessionCommandV1
  requestDigest: base64url(32)
  latestDeliveryRevision?: integer>=1
  currentAttempt: integer[0,8]
  claimExpiresAt?: time
  acknowledgedReceiptRevision?: integer>=1
  acknowledgedReceiptDigest?: base64url(32)
  committedCursor: opaque command cursor
  terminalOutcome?: SessionCommandOutcomeV1
  commandRevision: integer>=1
}
```

Create returns HTTP 202 whether the target is currently online or offline.
Authorization validates mapped resources and target capability but does not
claim execution. Listing allocates/persists an immutable append-only
`deliveryRevision` offer and computes `expectedNextAttempt`, but does not change
command state, increment attempt, allocate a lease, or advance the Device's
committed delivery cursor. `claim` is the only operation that CASes
queued/reclaimable -> claimed, assigns that expected attempt, and sets the
30-second `claimExpiresAt`; claim leaves the cursor unchanged. The Daemon then
commits `ReservedReceiptV1` locally. `ack` verifies that exact receipt/digest/
revision and current claim, atomically changes claimed -> acknowledged, marks
that delivery accepted, and advances the server Device cursor only through the
highest contiguous accepted/terminal/superseded delivery. Thus an out-of-order
ack cannot skip an earlier live offer. The returned
`DeviceCommandCursorCommitV1` is the only cursor-advance fact.

Each claim/ack/result/reconcile mutation requires its derived idempotency value
inside the body and the identical value as the HTTP Idempotency-Key. The
browser's creation key is never forwarded. The server retains a bounded
append-only claim-attempt history row containing Device/command/digest/attempt/
delivery revision/issued/expired/ack time for command retention. Only an
unacked expired claim
requeues; the lease reaper and ack serialize on one CAS. Ack-first freezes the
attempt; expiry-first records attempt N and the next winning claim is exactly
N+1. Acknowledged never requeues and ends by result or command TTL. Stale
attempt, wrong Device/digest/expiry, conflicting idempotency, or skipped
transition fails. `result` accepts only terminal
`succeeded|failed|unsupported`; terminal states cannot change.

Receipt canonical bytes are restricted JCS over exactly the listed fields;
omitted optional fields stay absent. `receiptDigest` is
`SHA-256(frame("multidesk-daemon-command-receipt-v1","1",JCS(receipt)))`.
The daemon commits `reserved` before ack and must observe successful ack or the
same authoritative acknowledged attempt before CAS `executing`. `start|resume`
atomically reserve one local Session ID, server Session UUID mapping, and
command binding in the `reserved` transaction before Provider work. Other kinds
persist target/pre-state first. Local commit records `local_committed`; accepted
server result records `completed`.

When an expired unacked attempt N is redelivered under a new append-only
delivery revision as current N+1, only an exact old `reserved` receipt may
rebind. One local CAS changes `deliveryRevision` and `attempt` to the current
claim and increments `receiptRevision`; it changes no local operation ID,
Session ID/mapping, reservation, request digest, or command field, then
recomputes the digest and acks. It may repeat across further expiries. `executing`,
`local_committed`, `ambiguous`, and `completed` are forbidden from attempt
rebind.

An old `executing` receipt never invokes the local service again; local-only
reconciliation must move it to `local_committed` when commit is proven or
`ambiguous` otherwise. Old `local_committed|ambiguous` uses `reconcile` against
a live current claim. The server verifies the current claim plus the immutable
receipt attempt in claim history and atomically records acknowledgement and the
stored terminal result without executing anything. Reconcile rejects
`reserved|executing`; ambiguous permits only failed
`command_execution_ambiguous`. If the current claim expires first, the same
immutable later-state receipt may reconcile against the next claim. An old
`completed` receipt is a duplicate only when server terminal state/outcome
matches; otherwise `command_receipt_inconsistent` quarantines it. Lost ack
request therefore takes the reserved-rebind path; lost ack response retries or
queries the already acknowledged attempt. Concurrent reclaim/rebind has one CAS
winner. No state ever creates a second local reservation or auto-reexecutes an
uncertain Provider effect.

The request cannot pass server-supplied binary paths, secretRefs, workspace
paths, raw Provider settings, terminal input, Approval decisions, or capability
snapshots.

## Plan v0.7 normative contract replacements

This section supersedes conflicting pre-v0.6 schemas or bounds above. The complete
OpenAPI and both generated languages must express these replacements before P1
verification, although only health/readiness/version are mounted in P1.

### Canonical origin, cookie, and WebAuthn wire

```text
CanonicalServerOriginV1 = "https://" + lowercase IDNA-ASCII host +
                          optional non-default decimal port

DeviceKeyEnvelopeV1 adds:
  serverOrigin: CanonicalServerOriginV1

BootstrapAnchorDescriptorV1,
BootstrapAnchorChallengeV1,
BootstrapCommitReceiptV1,
ActivationReceiptV1 add the same serverOrigin.
```

The Device envelope AAD becomes:

```text
frame("multidesk-device-key-envelope-v1", "1", serverOrigin,
      localRemoteIdentityId, serverDeviceId, signingKeyDigestRaw,
      exchangeKeyDigestRaw, decimalKeyRevision)
```

The browser cookie is named `__Host-mad_session` and has Secure, HttpOnly,
SameSite=Strict, Path=/, and no Domain. Clearing it uses the same name/path,
`Max-Age=0`, and an already-expired date.

Browser credential bytes are JSON Base64url, never numeric arrays or browser-
specific objects:

```text
WebAuthnExtensionResultsV1 {} // empty in v1; additionalProperties:false

WebAuthnCredentialDescriptorV1 {
  type: public-key
  id: base64url(decoded 1..1024 bytes)
  transports?: (ble|cable|hybrid|internal|nfc|smart-card|usb)[0..7]
}

WebAuthnCreationOptionsV1 {
  ceremonyId: UUIDv7
  publicKey: {
    challenge: base64url(32)
    rp: {id:string(1..253),name:string(1..128)}
    user: {
      id: base64url(decoded 1..64 bytes)
      name: string(1..128)
      displayName: string(1..128)
    }
    pubKeyCredParams: [
      {type:public-key,alg:-7},
      {type:public-key,alg:-8},
      {type:public-key,alg:-257}
    ]
    timeout: 60000
    excludeCredentials: WebAuthnCredentialDescriptorV1[0..64]
    authenticatorSelection: {
      authenticatorAttachment?: platform | cross-platform
      residentKey: preferred
      requireResidentKey: false
      userVerification: required
    }
    attestation: none
    extensions: {} // WebAuthnExtensionResultsV1 shape
  }
}

WebAuthnRequestOptionsV1 {
  ceremonyId: UUIDv7
  publicKey: {
    challenge: base64url(32)
    timeout: 60000
    rpId: string(1..253)
    allowCredentials: WebAuthnCredentialDescriptorV1[1..64]
    userVerification: required
    extensions: {} // WebAuthnExtensionResultsV1 shape
  }
}

WebAuthnRegistrationCredentialV1 {
  id: base64url(1..1024)
  rawId: base64url(1..1024)
  type: public-key
  authenticatorAttachment?: platform | cross-platform
  response: {
    clientDataJSON: base64url(1..65536)
    attestationObject: base64url(1..262144)
    transports?: (ble|cable|hybrid|internal|nfc|smart-card|usb)[]
  }
  clientExtensionResults: WebAuthnExtensionResultsV1
}

WebAuthnAssertionCredentialV1 {
  id: base64url(1..1024)
  rawId: base64url(1..1024)
  type: public-key
  authenticatorAttachment?: platform | cross-platform
  response: {
    clientDataJSON: base64url(1..65536)
    authenticatorData: base64url(37..65536)
    signature: base64url(1..65536)
    userHandle?: base64url(1..1024)
  }
  clientExtensionResults: WebAuthnExtensionResultsV1
}
```

Every nested object above has `additionalProperties:false`; tuple order and
algorithm values are fixed. v1 requests no WebAuthn extension and accepts only
the exact empty `{}` from `getClientExtensionResults()`. Any member such as
`credProps`, `appid`, `largeBlob`, `prf`, `uvm`, or an unknown future extension
is rejected until a reviewed API version defines it.

Base64url decodes before the library call; padding, whitespace, standard Base64
alphabet, unknown extensions, duplicate JSON members, or noncanonical
credential type reject. Options responses are exactly
`WebAuthnCreationOptionsV1|WebAuthnRequestOptionsV1`, with every binary member
encoded Base64url and `Cache-Control:no-store`; no library-native generic map is
serialized.

Passkey rows store `signCount` and `credentialRevision`. Assertion result and
session issuance use one CAS: `0->0`, `0->N>0`, and `N->M>N` succeed;
nonzero `N->N|M<N` returns `passkey_counter_regressed` and revokes all sessions
authenticated with that passkey. Passkey deletion returns
`PasskeyDeleteResultV1 {passkeyId,revokedSessionCount,currentSessionRevoked}`;
when currentSessionRevoked is true it clears the cookie and returns no
`CurrentAuth`.

### Signed Device-auth contract

```text
DeviceAuthChallengeRequestV1 {
  deviceId: UUIDv7
  serverOrigin: CanonicalServerOriginV1
  signingKeyDigest: base64url(32)
  clientNonce: base64url(32)
}

DeviceAuthChallengeV1 {
  version: 1
  type: device_auth_challenge
  challengeId: UUIDv7
  serverOrigin: CanonicalServerOriginV1
  deviceId: UUIDv7
  signingKeyDigest: base64url(32)
  clientNonce: base64url(32)
  challenge: base64url(32)
  issuedAt: time
  expiresAt: time                 // issuedAt + 60 seconds
}

DeviceAuthChallengeResponseV1 {
  challenge: DeviceAuthChallengeV1
  serverSigningPublicKey: base64url(32)
  challengeSignature: base64url(64)
}

DeviceAuthExchangeRequestV1 {
  challenge: DeviceAuthChallengeV1
  challengeSignature: base64url(64)
  subjectSignature: base64url(64)
}

DeviceAuthExchangeResultV1 {
  deviceSessionId: UUIDv7
  deviceSessionToken: base64url(32) // returned once; digest stored
  deviceId: UUIDv7
  issuedAt: time
  expiresAt: time                 // issuedAt + 15 minutes
}
```

Exact signatures are:

```text
challengeSignature = Ed25519.Sign(serverChallengePrivateKey,
  frame("multidesk-device-auth-challenge-signature-v1", "1",
        JCS(DeviceAuthChallengeV1)))

subjectSignature = Ed25519.Sign(subjectSigningPrivateKey,
  frame("multidesk-device-auth-exchange-signature-v1", "1",
        JCS(DeviceAuthChallengeV1), challengeSignatureRaw))
```

Challenge state is `issued|consumed|expired`; consume is one persistent CAS and
uses `issuedAt <= now < expiresAt`. The server signing key and issued row survive
restart. Session token digest is `SHA-256(tokenRaw)`. Request nonces are durable
`(deviceSessionId,nonce)` rows retained through session expiry, so restart does
not reset replay state.

### Enrollment pre-auth, state, and typed signatures

The endpoint set adds list/filter and activation-package retrieval:

```text
GET  /v1/device-enrollments?state=&kind=&subjectDeviceId=&limit=&cursor=
GET  /v1/device-enrollments/{id}/activation-package

EnrollmentState = pending_proof | proof_verified | approved | activated |
                  cancelled | expired

EnrollmentSummaryV1 {
  enrollmentId: UUIDv7
  subjectDeviceId: UUIDv7
  subjectKind: daemon | web | desktop
  subjectName: string
  state: EnrollmentState
  challengeRevision: integer>=1
  capabilityRevision: integer>=1
  fingerprint: six groups
  requestedCapabilities: DeviceCapabilityV1[]
  createdAt: time
  expiresAt: time
  updatedAt: time
}
```

Daemon candidate calls carry `Authorization: Enrollment <enrollmentId>` plus
the ordinary timestamp/nonce/content-digest headers and
`X-MAD-Enrollment-Signature`. Create uses `enrollmentId` from the request body;
all calls sign:

```text
frame("multidesk-enrollment-preauth-request-v1", "1", serverOrigin,
      enrollmentId, subjectDeviceId, decimalChallengeRevision, method,
      canonicalPathAndQuery, contentSHA256Raw, timestamp, nonceRaw)
```

The signature key is the pending subject key in the submitted transcript. Web
candidates require both this signature and the authenticated browser cookie/
CSRF mutation class. Active Device bearer/session headers are not candidate
authority.

The exact typed canonical objects and formulas are:

```text
EnrollmentTranscriptV1 {
  version: 1; type: enrollment_transcript
  enrollmentId; serverOrigin; subjectDeviceId; subjectKind; subjectName
  subjectSigningPublicKey; subjectExchangePublicKey
  subjectSigningKeyDigest; subjectExchangeKeyDigest
  pinDigest; storageMode; storageAssertionDigest
  requestedCapabilities; challengeRevision; challenge; serverEphemeralKey
  issuedAt; expiresAt
}

transcriptDigest = SHA-256(frame(
  "multidesk-enrollment-transcript-digest-v1", "1",
  JCS(EnrollmentTranscriptV1)))

attestationDigest = SHA-256(frame(
  "multidesk-device-attestation-digest-v1", "1",
  JCS(DeviceAttestationV1)))

attestationSignature = Ed25519.Sign(approverKey, frame(
  "multidesk-device-attestation-signature-v1", "1",
  JCS(DeviceAttestationV1)))

capabilityAttestationDigest = SHA-256(frame(
  "multidesk-device-capability-attestation-digest-v1", "1",
  JCS(DeviceCapabilityAttestationV1)))

capabilityAttestationSignature = Ed25519.Sign(approverKey, frame(
  "multidesk-device-capability-attestation-signature-v1", "1",
  JCS(DeviceCapabilityAttestationV1)))

receiptDigest = SHA-256(frame(
  "multidesk-device-activation-receipt-digest-v1", "1",
  JCS(ActivationReceiptV1)))

activationReceiptSignature = Ed25519.Sign(approverKey, frame(
  "multidesk-device-activation-receipt-signature-v1", "1",
  JCS(ActivationReceiptV1)))

SubjectActivationAckV1 {
  version:1; type:subject_activation_ack; enrollmentId; serverOrigin
  subjectDeviceId; approverDeviceId; transcriptDigest; attestationDigest
  receiptDigest; activationPackageDigest; challengeRevision
  confirmedApproverPinDigest; confirmedAt
}

subjectActivationSignature = Ed25519.Sign(subjectKey, frame(
  "multidesk-subject-activation-ack-signature-v1", "1",
  JCS(SubjectActivationAckV1)))
```

`ActivationReceiptV1` is exactly the strict signed payload defined in the
Enrollment section. Raw approver public keys and every detached signature are
outside that payload and exist only in `EnrollmentActivationPackageV1`; their
digests must match its signed receipt and attestation. The activation-package
response is available only in `approved` state. Final activate accepts only
`EnrollmentActivateRequestV1` containing the exact ack above and returns only
`EnrollmentActivateResultV1`, after the client locally verifies/pins the
approver. Each type/domain is distinct; using any valid signature/digest in
another type returns `cross_type_signature_replay`.

```text
AuthCapabilityV1 = metadata_read | profile_write | device_enroll_request |
                   device_enroll_approve | device_revoke |
                   session_command_create | passkey_manage | session_revoke

DeviceCapabilityV1 = mad.v1.metadata.read | mad.v1.metadata.write |
  mad.v1.sync.pull | mad.v1.sync.push | mad.v1.presence.write |
  mad.v1.device.enroll_request | mad.v1.device.enroll_approve |
  mad.v1.device.revoke | mad.v1.session.command_create |
  mad.v1.session.command_claim | mad.v1.session.command_ack |
  mad.v1.session.command_result
```

These enums are never coerced. Device lifecycle is
`pending|active|revoked`; `DevicePresenceV1` is separately
`online|offline` with `serverBootEpoch`, `lastSeenAt`, and `derivedAt`.

```text
WebDeviceStorageAssertionV1 {
  version:1; type:web_device_storage_assertion
  serverOrigin; deviceId; signingKeyDigest; exchangeKeyDigest
  storageMode:native|software_wrapped
  wrappingAlgorithm:AES-256-GCM
  wrappingKeyExtractable:false
  wrappedExchangePrivateKeyDigest?:base64url(32)
  indexedDbSchemaVersion:1
  keyRevision:1
  recordRevision:integer>=1
  probedAt:time
}
```

`software_wrapped` requires @noble/curves 2.2.0 X25519 PoP; an assertion alone
cannot activate.

### Metadata/sync replacements

`SessionProjection.provider` is `codex|claude`; `fake` is removed from all
network schemas. Snapshot `pageSize|limit` is 1..4. Sync pull stops before
900 KiB canonical response bytes as well as 100 changes and sets `hasMore`.
Oversized snapshot construction returns `snapshot_page_too_large` with no
state/cursor write.

Browser `ProfileCreateRequest.enabled` is forbidden; the server always creates
false. `ProfileProjectionV1` adds:

```text
materializationState: pending_local_completion | ready | quarantined
materializationRevision: integer>=1
```

`ProfileMaterializationReadyV1` is a signed Device mutation binding profile ID,
target Device, server sync revision, local prefixed ID mapping digest, Provider
validation digest, and `readyAt`; it contains no local approval/sandbox policy.
Enable PATCH requires state ready. Device storage uses separate
`localEntityRevision` and `serverSyncRevision` columns; only the latter appears
in canonical sync revisions.

Stale browser Profile mutations use:

```text
ProfilePatchMutationV1 {
  operation: patch
  profileId: UUIDv7
  baseRevision: integer>=1
  changes: ProfileMutableChangeV1[1..8] // unique field, enum order
}
ProfileDeleteMutationV1 {
  operation: delete
  profileId: UUIDv7
  baseRevision: integer>=1
}
ProfileMutationV1 = oneOf(ProfilePatchMutationV1,ProfileDeleteMutationV1)

ProfileMutableChangeV1 = oneOf(
  {field:name,value:string(1..128)},
  {field:selector_alias,value:string(1..32)|null},
  {field:enabled,value:boolean},
  {field:model_preference,value:string(1..128)|null},
  {field:environment_non_secret,value:bounded denylisted map<string,string>},
  {field:mcp_ref_keys,value:string[0..64]},
  {field:skill_ref_keys,value:string[0..64]},
  {field:hook_ref_keys,value:string[0..64]})

ProfileConflictFieldNameV1 = resource | name | selector_alias | enabled |
  model_preference | environment_non_secret | mcp_ref_keys |
  skill_ref_keys | hook_ref_keys

ProfileConflictFieldV1 {
  field: ProfileConflictFieldNameV1
  currentState: omitted | null | value
  currentValueDigest: base64url(32)
  submittedState: omitted | null | value
  submittedValueDigest: base64url(32)
}

ProfileConflictV1 {
  profileId: UUIDv7
  submittedIfMatchRevision: integer>=1
  currentRevision: integer>=1
  current: ProfileProjectionV1
  submitted: ProfileMutationV1
  conflictingFields: ProfileConflictFieldV1[1..9] // unique enum order
}
```

Every object/union branch has `additionalProperties:false`; mutation canonical
JSON is capped at 32 KiB and the complete conflict detail at 64 KiB. Create-only
or server-managed `targetDeviceId`, `provider`, `accountId`, every
`materialization*` field, revisions, timestamps, and IDs other than the
path/profile ID are forbidden
in `ProfileMutationV1`. `enabled=true` additionally requires the separately
verified materialization-ready precondition; it is not inferred from conflict
data.

Conflict digests bind the field name and distinguish absence from JSON null:

```text
omittedDigest = SHA-256(frame("multidesk-profile-conflict-field-v1", "1",
  fieldName, "omitted"))
nullDigest = SHA-256(frame("multidesk-profile-conflict-field-v1", "1",
  fieldName, "null", JCS(null)))
valueDigest = SHA-256(frame("multidesk-profile-conflict-field-v1", "1",
  fieldName, "value", JCS(exact typed field value)))
```

For patch, unmentioned mutable fields have submittedState `omitted`; an
explicit nullable clear has `null`; every supplied non-null value has `value`.
For delete, the sole conflict field is `resource`: current is `value` over
JCS(current ProfileProjectionV1), submitted is `omitted`. No digest can be
reused under another field/state.

### Canonical commands, delivery, and receipts

The canonical request, offer/list, claim, ack, result, reconcile, authoritative
query, and cursor-commit DTOs are the exact v0.7 schemas in the preceding
Session Command section. Request digest and derived mutation idempotency are:

```text
requestDigest = SHA-256(frame(
  "multidesk-session-command-request-v1", "1",
  JCS(CanonicalSessionCommandRequestV1)))

derivedCallIdempotencyKey = base64url(SHA-256(frame(
  "multidesk-remote-command-call-v1", "1", commandId, requestDigest,
  decimalDeliveryRevision, decimalAttempt, callKind,
  decimalReceiptRevision)))
```

`callKind` is exactly `claim|ack|result|reconcile`. Claim uses the offered
`expectedNextAttempt` and decimal receipt revision `0`; every later call uses
the current claim attempt and the exact durable receipt revision. The value is
sent both as the JSON `derivedIdempotencyKey` and HTTP Idempotency-Key. Any
field/revision/call-kind mismatch is `idempotency_key_reused` or
`command_digest_mismatch`, never recomputed optimistically by the server.

All schemas below have `additionalProperties:false`. Command reservation is a
closed discriminator union; acquire/release have no reservation or receipt
variant because they terminalize unsupported before delivery:

```text
StartReservationV1 {
  kind: start
  localOperationId: prefixed local ID
  resultSessionId: UUIDv7
  localSessionId: prefixed local ID
  mappingDigest: base64url(32)
}
ResumeReservationV1 {
  kind: resume
  localOperationId: prefixed local ID
  sourceServerSessionId: UUIDv7
  sourceLocalSessionId: prefixed local ID
  sourceLocalSessionRevision: integer>=1
  resultSessionId: UUIDv7
  localSessionId: prefixed local ID
  mappingDigest: base64url(32)
}
StopReservationV1 {
  kind: stop
  localOperationId: prefixed local ID
  targetServerSessionId: UUIDv7
  targetLocalSessionId: prefixed local ID
  preState: {localRevision:integer>=1,status:starting|running|stopping}
}
KillReservationV1 {
  kind: kill
  localOperationId: prefixed local ID
  targetServerSessionId: UUIDv7
  targetLocalSessionId: prefixed local ID
  preState: {localRevision:integer>=1,status:starting|running|stopping}
}
CommandReservationV1 = oneOf(StartReservationV1,ResumeReservationV1,
                             StopReservationV1,KillReservationV1)
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

The proof union exists only after a verified local commit and its `kind` must
equal both the command and outcome kind:

```text
StartProofV1 {kind:start,resultSessionId:UUIDv7,localSessionId:prefixed ID,
  mappingDigest:base64url(32),localSessionRevision:integer>=1,
  outboxChangeId:prefixed ID,outboxServerSyncRevision:integer>=1,
  outboxDigest:base64url(32)}
ResumeProofV1 {kind:resume,sourceServerSessionId:UUIDv7,
  sourceLocalSessionId:prefixed ID,sourceLocalSessionRevision:integer>=1,
  resultSessionId:UUIDv7,localSessionId:prefixed ID,
  mappingDigest:base64url(32),localSessionRevision:integer>=1,
  outboxChangeId:prefixed ID,outboxServerSyncRevision:integer>=1,
  outboxDigest:base64url(32)}
StopProofV1 {kind:stop,targetServerSessionId:UUIDv7,
  targetLocalSessionId:prefixed ID,beforeLocalRevision:integer>=1,
  beforeStatus:starting|running|stopping,
  afterLocalRevision:integer>beforeLocalRevision,afterStatus:exited,
  remoteOperationRevision:integer>=1,serviceResultDigest:base64url(32)}
KillProofV1 {kind:kill,targetServerSessionId:UUIDv7,
  targetLocalSessionId:prefixed ID,beforeLocalRevision:integer>=1,
  beforeStatus:starting|running|stopping,
  afterLocalRevision:integer>beforeLocalRevision,afterStatus:killed,
  remoteOperationRevision:integer>=1,serviceResultDigest:base64url(32)}
KindProofV1 = oneOf(StartProofV1,ResumeProofV1,StopProofV1,KillProofV1)
```

`DaemonCommandReceiptV1` is a closed state discriminator. Every common field is
required: `version=1,type=daemon_command_receipt,commandId,targetDeviceId,
requestDigest,deliveryRevision,attempt,claimExpiresAt,receiptRevision,kind,
integrityStatus=verified|quarantined,reservation,createdAt,updatedAt`.

```text
ReservedReceiptV1 {
  common fields; state:reserved
}
ExecutingReceiptV1 {
  common fields; state:executing; executingAt:time
}
LocalCommittedReceiptV1 {
  common fields; state:local_committed; executingAt:time;
  operationProof:KindProofV1; safeResult:SessionCommandOutcomeV1;
  committedAt:time
}
AmbiguousReceiptV1 {
  common fields; state:ambiguous; executingAt:time;
  ambiguityCode:execution_commit_unprovable;
  safeResult: exact failed outcome for the same kind with
              code=command_execution_ambiguous; detectedAt:time
}
CompletedReceiptV1 {
  all LocalCommittedReceiptV1 fields; state:completed;
  acceptedServerResultRevision:integer>=1; completedAt:time
}
DaemonCommandReceiptV1 = oneOf(ReservedReceiptV1,ExecutingReceiptV1,
  LocalCommittedReceiptV1,AmbiguousReceiptV1,CompletedReceiptV1)
```

`integrityStatus=verified|quarantined` is orthogonal. Only verified receipts
transition. The reservation/proof/outcome kind must match byte-for-byte;
cross-kind or extra/omitted fields fail schema before digest verification.
`local_committed -> ambiguous` is illegal. Acquire/release return their exact
terminal unsupported outcomes without an offer, claim, cursor event, or receipt.

Time comparison is `now < expiry`. Resolution priority is revoked target,
existing terminal state, command TTL, feature disable, claim expiry, then
mutation. Attempt nine is never issued; after eight expired attempts the result
is failed `delivery_attempts_exhausted`. Terminal rows/deliveries/claims/
receipts retain 30 days, idempotency 24 hours, compact audit digests 365 days.

### Overview and Usage units

```text
GET /v1/overview

OverviewV1 {
  generatedAt: time
  freshness: {section,observedAt?,staleAt?,state:fresh|stale|unavailable}[]
  counts: {devices,accounts,profiles,sessions,usageWindows}
  recentDevices: Device[0..5]
  recentSessions: SessionProjection[0..5]
  recentUsage: UsageProjection[0..5]
}

UsageWindowV1 {
  providerLimitId?: string
  kind: rolling | calendar | spend_control | sdk_credit | unknown
  label: string
  unit: request | token | second | usd | provider_unit | percent
  scale: integer[0,9]               // USD requires 6
  usedScaled?: decimal string
  limitScaled?: decimal string
  remainingBasisPoints?: integer[0,10000]
  resetsAt?: time
}
```

`usedValue`, `limitValue`, floating percentages, and implicit unit inference are
removed. Decimal strings have no sign, exponent, leading zeros (except `0`), or
value above signed 63-bit maximum. Missing is unknown, not zero. `usd/scale=6`
requires an explicit official source; `provider_unit` cannot be converted.

### Browser runtime and PWA contract

Production Web cryptography is an exported browser-only module with no Node
imports. IndexedDB schema v1 contains `identities`, `wrapped_keys`,
`pinned_peers`, `activation_receipts`, and `record_cas`; every record is bound
to serverOrigin/deviceId/keyRevision/recordRevision. The service worker route
table is deny-by-default: `/v1/**`, auth/bootstrap/enrollment/recovery,
health/ready/version, and every non-GET are network-only and cannot be cached,
queued, or receive SPA fallback. Static cache entries must be content-hashed.

Logout clears cookie/auth cache/in-memory CSRF only; Web Device keys and pins
remain. Explicit Forget Device is a separate recent-UV revocation/delete flow.
Recovery copy/download/print never enters browser persistence, service worker,
trace, screenshot, analytics, or automated test artifacts.

## Health and version

```text
GET /v1/healthz
GET /v1/readyz
GET /v1/version
```

`healthz` is liveness only. `readyz` verifies configuration and a bounded DB
read without exposing schema/path details. `version` returns API version,
server build version/commit, minimum supported client protocol, and enabled
feature flags. None reports user/device counts, origin secrets, filesystem
paths, or database location.

## Stable errors

The initial v1 set is frozen in OpenAPI:

```text
invalid_argument                 unauthenticated
permission_denied                not_found
conflict                         resource_exhausted
rate_limited                     request_too_large
unsupported_api_version          schema_incompatible
idempotency_key_required         idempotency_key_reused
if_match_required                sync_conflict
sync_history_missing             sync_base_digest_mismatch
sync_next_digest_mismatch        sync_patch_mismatch
sync_patch_too_large             invalid_cursor
stale_resurrection               snapshot_required
snapshot_in_progress             snapshot_expired
snapshot_page_invalid            snapshot_commit_conflict
snapshot_page_too_large          cross_server_identity_rebind
bootstrap_unavailable            bootstrap_expired
bootstrap_replayed               bootstrap_anchor_required
origin_mismatch                  rp_id_mismatch
webauthn_challenge_expired       webauthn_challenge_replayed
webauthn_verification_failed     passkey_counter_regressed
recovery_invalid_or_rate_limited recovery_consumed
one_time_result_unavailable      recent_uv_required
last_passkey_required            recovery_batch_replaced
csrf_invalid                     session_expired
device_not_enrolled              device_revoked
device_key_changed               key_digest_mismatch
device_key_envelope_corrupt      device_key_envelope_conflict
pin_mismatch                     attestation_invalid
attestation_expired              attestation_replayed
cross_type_signature_replay      enrollment_preauth_invalid
approver_not_pinned              capability_denied
capability_revision_conflict     capability_not_recognized
activation_receipt_invalid       enrollment_cancelled
signature_invalid                request_replayed
clock_skew                       command_expired
command_claimed                  command_state_conflict
command_digest_mismatch          projection_read_only
command_attempt_stale            command_execution_ambiguous
command_reconciliation_required  command_receipt_inconsistent
delivery_attempts_exhausted      phase4b_controller_required
mapping_quarantined              forbidden_metadata_field
provider_control_unsupported     provider_resume_unsupported
daemon_unavailable
```

HTTP status is conventional (`400`, `401`, `403`, `404`, `409`, `412`, `413`,
`422`, `429`, `500`, `503`), but clients branch on stable `code`. An unknown
server code maps to `unknown_error`; it never causes optimistic success.

## Audit and retention contract

Audit entries contain UUIDv7 event/actor/target IDs, action, decision, stable
error code, request ID, timestamp, and a small action-specific allowlisted map.
They never contain raw headers/bodies, cookies, recovery values, WebAuthn
SessionData/challenges, signatures, keys beyond public digests, local paths,
Provider content, or terminal text.

Default retention: auth/device/security audit 365 days; ordinary API audit 90
days; idempotency records 24 hours; WebAuthn/device-auth challenges five/one
minutes; browser sessions 12 hours; device sessions 15 minutes; enrollment ten
minutes; presence 60 seconds; command rows 30 days after terminal; tombstones
at least 30 days and until eligible acknowledgements. Resource revision digests
and deletion watermarks are compact server-lifetime records and are not swept
by ordinary cleanup. Cleanup is configurable
only toward longer security retention or smaller operational retention where
the tombstone/incident contract permits.
