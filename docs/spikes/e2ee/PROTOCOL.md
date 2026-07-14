# MultiAgentDesk E2EE protocol candidate v1

Status: Phase 0.5 Spike candidate; not production implementation  
Wire version: `1`  
Owner: `security`  
Affected modules: `control-plane`, `web`, `core`

## 1. Scope and invariants

This candidate protects session content, approvals, remote-control input, and
session-key distribution between enrolled devices when the Control Plane and
ciphertext relay are observable or malicious. It does not hide routing
metadata, message size, timing, or availability.

The following invariants are mandatory:

1. The Control Plane key directory is an index, never a trust anchor.
2. A receiver selects a sender key from its local pin/attestation state. It
   never trusts the key or digest carried by an incoming envelope by itself.
3. Passkeys authenticate a user to the Control Plane but never derive, replace,
   recover, or authorize an E2EE device private key.
4. Ed25519 signing keys and X25519 exchange keys are generated and stored as
   separate key pairs. Implementations must not convert one key type into the
   other.
5. Protected payloads and Provider credentials never appear in Control Plane
   logs, traces, metrics, error messages, queues, or databases as plaintext.
6. Device revocation prevents future access but cannot erase plaintext or
   credentials already copied by an authorized or compromised device.
7. Any key/state ambiguity fails closed and rotates the affected session key
   epoch; it never resets a sequence under the same traffic key.

## 2. Algorithm suite

| Purpose | Algorithm | Identifier |
|---|---|---|
| Device signatures and attestations | Ed25519 | `Ed25519` |
| Session-key wrapping | RFC 9180 HPKE Auth mode | `DHKEM(X25519,HKDF-SHA256)/HKDF-SHA256/ChaCha20Poly1305/Auth` |
| Session traffic derivation | RFC 5869 HKDF-SHA-256 | `HKDF-SHA256` |
| Session payload AEAD | XChaCha20-Poly1305, 32-byte key, 24-byte nonce | `XChaCha20Poly1305` |
| Signed/authenticated object serialization | RFC 8785 JSON Canonicalization Scheme | `JCS` |
| Hashes and key digests | SHA-256 | `SHA-256` |
| Binary JSON fields | unpadded Base64url | `base64url` |

The protocol uses HPKE rather than a project-specific X25519/HKDF wrapping
construction. HPKE Auth mode authenticates possession of the locally pinned
source X25519 private key while encrypting a fresh pairwise root key to one
target X25519 public key. Session traffic uses XChaCha20-Poly1305 because its
extended nonce construction is available in both selected implementation
stacks and already matches the implementation plan.

Algorithm negotiation is not allowed in v1. A peer that does not support the
complete suite cannot process protected content and must remain
`metadata_only`. A future suite requires a new protocol version and new test
vectors; it must not silently downgrade v1.

## 3. Canonical serialization

Every object that is signed or passed as AEAD AAD is serialized as RFC 8785
JCS UTF-8. Implementations must reject before cryptographic processing:

- duplicate JSON object names;
- non-I-JSON strings or numbers;
- negative zero;
- unknown critical fields;
- integers outside the exact IEEE-754 range when represented as JSON numbers;
- malformed UUID, timestamp, enum, Base64url, or key lengths.

All `uint64` values, including `sequence` and `keyEpoch`, are canonical decimal
strings without signs or leading zeroes (`"0"` is the sole zero form). This
avoids JavaScript precision loss. Capability arrays are sorted
lexicographically before signing. Unknown non-critical extension data must be
placed under an explicitly versioned `extensions` object; it is still covered
by the signature or AAD.

Domain-separated byte strings use this framing:

```text
frame(part_1, ..., part_n) =
  uint32be(len(part_1)) || part_1 || ... || uint32be(len(part_n)) || part_n
```

No concatenation without lengths is permitted.

## 4. Device identity, pinning, and attestation

Each enrolled device has:

- one Ed25519 signing key pair;
- one X25519 exchange key pair;
- one immutable Device ID;
- an explicit capability set;
- a local, operator-verified pinned directory.

Private keys remain in the device Vault selected by the platform key-storage
decision. A key change creates a new device identity or explicit re-enrollment;
the server cannot overwrite a pin.

The full pin digest is:

```text
SHA-256(frame(
  "multidesk-device-pin-v1",
  deviceId,
  signingPublicKeyRaw,
  exchangePublicKeyRaw
))
```

The UI displays every byte as eight groups of eight lowercase hexadecimal
characters. The approval ceremony compares the complete value on a previously
pinned device and the enrolling device.

A `DeviceAttestation` contains at least:

```json
{
  "approverDeviceId": "uuidv7",
  "attestationId": "uuidv7",
  "capabilities": ["session.control", "session.decrypt"],
  "expiresAt": "RFC3339",
  "issuedAt": "RFC3339",
  "subjectDeviceId": "uuidv7",
  "subjectExchangeKey": "base64url-32-bytes",
  "subjectSigningKey": "base64url-32-bytes",
  "type": "device_attestation",
  "version": 1
}
```

The approver signs:

```text
Ed25519.Sign(
  approverSigningPrivateKey,
  frame("multidesk-device-attestation-v1", JCS(attestation))
)
```

Verification requires an unrevoked, locally pinned approver signing key,
matching subject key bytes and digest, current validity, permitted capability
delegation, and a previously unseen `attestationId`. Server-provided keys may
help locate candidates but never satisfy this check.

The initial trust anchor must be a Daemon/Desktop device with an OS-backed
Vault. A pure browser cannot bootstrap the trust graph by itself.

## 5. Pairwise root key and HPKE wrap

A session host generates a different uniformly random 32-byte
`pairwiseRootKey` for every
`(sessionId, hostDeviceId, peerDeviceId, keyEpoch)`. A root is never shared by
two peers. The host creates one HPKE Auth-mode wrap for the approved peer when
that peer has `session.decrypt` capability.

The host encrypts fan-out content separately for each peer. This bounded
per-peer cost is intentional: a peer that learns its own pairwise root cannot
derive another peer's traffic keys, impersonate that peer, or bypass
device-scoped capability and ControllerLease checks.

The base wrap context is:

```json
{
  "expiresAt": "RFC3339",
  "keyEpoch": "1",
  "purpose": "session_content",
  "sessionId": "uuidv7",
  "sourceDeviceId": "uuidv7",
  "sourceExchangeKeyDigest": "base64url-sha256",
  "targetDeviceId": "uuidv7",
  "targetExchangeKeyDigest": "base64url-sha256",
  "type": "session_key_wrap",
  "version": 1,
  "wrapId": "uuidv7"
}
```

HPKE `info` is the 32-byte value:

```text
SHA-256(frame(
  "multidesk-hpke-session-wrap-info-v1",
  JCS(baseWrapContext)
))
```

After HPKE setup returns `enc`, AAD is JCS of the base context plus:

```json
{
  "enc": "base64url",
  "hpkeSuite": "DHKEM(X25519,HKDF-SHA256)/HKDF-SHA256/ChaCha20Poly1305/Auth"
}
```

The plaintext is exactly the 32-byte pairwise root key for the source/target
pair. The receiver must:

1. reject a revoked source or target before decryption;
2. load the target private key and source public key from local state;
3. require both locally stored key digests to match the AAD values;
4. validate purpose, audience, epoch, expiry, wrap ID, and capabilities;
5. reject a reused wrap ID;
6. open HPKE in Auth mode; and
7. require that no other peer record references the same root identity; and
8. persist the accepted key and replay state atomically.

Key wraps expire after at most ten minutes. A retry creates a new `wrapId` and
fresh HPKE encapsulation. The relay may cache only ciphertext and routing
metadata until expiry.

## 6. Traffic keys and nonces

Each direction and stream derives independent material from the pairwise root
key. The context is JCS of:

```json
{
  "direction": "device_to_client",
  "keyEpoch": "1",
  "purpose": "session_traffic",
  "sessionId": "uuidv7",
  "sourceDeviceId": "uuidv7",
  "streamId": "terminal",
  "targetDeviceId": "uuidv7",
  "version": 1
}
```

Derivation is:

```text
salt = SHA-256(frame(
  "multidesk-session-traffic-salt-v1",
  sessionId,
  keyEpoch
))

info = frame(
  "multidesk-session-traffic-info-v1",
  JCS(trafficContext)
)

material = HKDF-SHA256(pairwiseRootKey, salt, info, 48)
trafficKey = material[0:32]
noncePrefix = material[32:48]
nonce = noncePrefix || uint64be(sequence)
```

This creates a deterministic 24-byte XChaCha nonce that is unique while the
sequence is unique. The pairwise root prevents one peer from deriving another
peer's context. The derivation context further isolates source, target,
direction, stream, session, and key epoch. A sender must durably reserve the
next sequence before emitting ciphertext. If state may have rolled back, it
rotates the affected pair's key epoch instead of guessing or reusing a
sequence.

Mandatory pairwise rotation or termination occurs when a peer is revoked, a
device key changes, sequence persistence is ambiguous, or a sequence would
overflow. A conservative policy rollover may also occur after 24 hours or
`2^32` envelopes. Rotation creates a fresh random pairwise root; deriving epoch
2 from the epoch 1 root is forbidden. Unaffected peer roots are not exposed or
reused during another peer's rotation.

## 7. Protected WebSocket envelope

The authenticated header is:

```json
{
  "direction": "device_to_client",
  "keyEpoch": "1",
  "kind": "terminal_output",
  "messageId": "uuidv7",
  "nonce": "base64url-24-bytes",
  "sentAt": "RFC3339",
  "sequence": "100",
  "sessionId": "uuidv7",
  "sourceDeviceId": "uuidv7",
  "streamId": "terminal",
  "targetDeviceId": "uuidv7",
  "type": "session_envelope",
  "version": 1
}
```

The receiver recomputes `noncePrefix` and `nonce` from its pairwise root,
canonical traffic context, and parsed sequence. It must reject a byte mismatch
with `nonce_sequence_mismatch` before AEAD open; the transmitted `nonce` is not
an authority for nonce choice.

`ciphertext = XChaCha20Poly1305.Seal(trafficKey, nonce, plaintext,
JCS(header))` in combined ciphertext/tag form. Every field above is AAD-bound.
The Control Plane may read the routing header and ciphertext length but cannot
change source, target, purpose, stream, kind, direction, sequence, epoch,
timestamp, or message identity without authentication failure.

`kind`, `streamId`, and `direction` are strict enums. Remote-control commands
and approvals use dedicated kinds and capability checks; terminal text can
never be reinterpreted as a command or approval.

## 8. Replay, ordering, and persistence

The receiver maintains a replay window per tuple:

```text
(sessionId, keyEpoch, sourceDeviceId, targetDeviceId, direction, streamId)
```

The production window is 1024 sequence values. It accepts authenticated
out-of-order delivery inside the window, rejects duplicates, and rejects
values below the window. `messageId` is an additional idempotency key, not a
replacement for the direction-scoped sequence.

Processing order is:

1. validate syntax, local pins, device status, capabilities, and epoch;
2. perform a non-authoritative replay prefilter for obvious duplicates;
3. authenticate/decrypt the AEAD frame;
4. atomically commit replay state and the resulting command/event; and
5. acknowledge only after commit.

The receiver persists high-water marks and the bitmap before acknowledging.
After crash ambiguity it requires a new key epoch. A Control Plane cursor or
message timestamp cannot reset local replay state.

`sentAt` has a default five-minute clock-skew warning window. It is an
operational freshness signal only; a valid contiguous sequence is not rejected
solely because clocks disagree. An out-of-window timestamp combined with a
discontinuous sequence fails closed and requests resynchronization. Devices
should use NTP and surface `clock_skew` without weakening sequence checks.

## 9. Revocation and session-key rotation

When a device is revoked:

1. the Control Plane rejects and closes that Device ID's WSS connections;
2. senders stop creating wraps for it and drop queued ciphertext addressed to
   it;
3. every affected Host↔Peer pair increments `keyEpoch`, tombstones the old
   pair, and destroys or quarantines its old root;
4. a re-enrolled replacement peer receives a fresh random pairwise root under
   its new identity; roots for other peers remain independent and unchanged;
5. all directions/streams of the affected pair derive new traffic keys and
   restart sequences at zero under the new epoch; and
6. audit records contain identifiers, epoch changes, and decisions only.

Peers reject a lower pair epoch after accepting a newer epoch. An old pairwise
root may decrypt previously captured old-epoch ciphertext for that pair, but it
must fail on every new-epoch frame and on every other peer's frame. Revocation
does not claim remote deletion of plaintext, credentials, or already exported
pairwise roots.

## 10. Failure behavior

The following conditions fail closed with stable, non-secret error codes:

- `pin_mismatch`
- `attestation_invalid`
- `device_revoked`
- `capability_denied`
- `wrap_expired`
- `wrap_replayed`
- `key_epoch_stale`
- `sequence_replayed`
- `sequence_too_old`
- `sequence_state_ambiguous`
- `nonce_sequence_mismatch`
- `aad_authentication_failed`
- `cipher_suite_unsupported`

Errors, traces, diagnostics, and audit events must not include private keys,
pairwise root keys, traffic keys, plaintext, AAD-bearing user content, or full
ciphertext. Unknown versions and suites never downgrade automatically.

## 11. Test-vector contract

`vectors.json` contains only deterministic, non-secret test seeds and message
fixtures. The independent implementations are:

- Go 1.26.5 using Cloudflare CIRCL HPKE 1.6.4 and `x/crypto` 0.54.0;
- Node.js 24 using `@hpke/core` 1.9.0,
  `@hpke/dhkem-x25519` 1.8.0,
  `@hpke/chacha20poly1305` 1.8.0, and `@noble/ciphers` 2.2.0.

`verify.mjs` requires exact equality for public keys, canonical bytes,
signatures, HPKE encapsulation/ciphertext, per-peer derived traffic keys,
nonces, XChaCha ciphertexts, pin digests, and rotation output. It includes two
peer roots and also requires these negative cases to fail:

- changed attestation bytes;
- changed wrap AAD;
- a sender key different from the local pin;
- changed envelope AAD;
- a nonce inconsistent with the canonical sequence;
- Peer A opening Peer B ciphertext;
- Peer A forging a frame that claims Peer B's identity;
- duplicate and too-old sequences; and
- Peer A epoch-2 ciphertext opened with the Peer A epoch-1 key.

The harness runs on Linux, macOS, and Windows in GitHub Actions. Passing these
vectors establishes interoperability and the tested failure properties; it
does not replace production implementation review, side-channel analysis,
dependency review, fuzzing, or an external cryptographic audit before a wider
security claim.

## 12. Primary references

- [RFC 9180: Hybrid Public Key Encryption](https://www.rfc-editor.org/rfc/rfc9180.html)
- [RFC 7748: Elliptic Curves for Security](https://www.rfc-editor.org/rfc/rfc7748.html)
- [RFC 8032: Edwards-Curve Digital Signature Algorithm](https://www.rfc-editor.org/rfc/rfc8032.html)
- [RFC 5869: HMAC-based Extract-and-Expand Key Derivation Function](https://www.rfc-editor.org/rfc/rfc5869.html)
- [RFC 8785: JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html)
- [RFC 5116: An Interface and Algorithms for Authenticated Encryption](https://www.rfc-editor.org/rfc/rfc5116.html)
- [Libsodium XChaCha20-Poly1305 construction](https://doc.libsodium.org/secret-key_cryptography/aead/chacha20-poly1305/xchacha20-poly1305_construction)
- [Cloudflare CIRCL HPKE package](https://pkg.go.dev/github.com/cloudflare/circl/hpke)
- [hpke-js API documentation](https://dajiaji.github.io/hpke-js/docs/)
- [Noble Ciphers project](https://github.com/paulmillr/noble-ciphers)
