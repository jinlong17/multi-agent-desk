# ADR 0011: Pairwise E2EE session protocol

- Status: Accepted
- Date: 2026-07-14
- Owner: `security`
- Security gate: resolved by `docs/reviews/spike-e2ee-protocol-vectors/2026-07-14-security-review.md`

## Context

MultiAgentDesk needs encrypted terminal, approval, and remote-control traffic
between a session host and enrolled Web/Desktop devices while treating the
Control Plane as an untrusted metadata index and ciphertext relay. Passkey
login cannot substitute for an E2EE Device identity, and the relay must not be
able to replace a locally pinned key, replay a command, or reinterpret one
message kind as another.

The first Spike candidate used one session root wrapped to every approved peer.
Although the Go and TypeScript vectors matched, security review rejected that
design: any peer holding the shared root could derive another peer's traffic
keys and authenticate a frame claiming that peer's identity. AAD binds an
identity string but cannot restore sender separation when all recipients share
the symmetric root.

## Decision

### Device identity and trust

- Every Device has separate Ed25519 signing and X25519 exchange key pairs.
- The Control Plane key directory is an index, never a trust anchor.
- A receiver obtains the sender X25519 public key from its local pin or a valid
  attestation signed by a directly pinned approver.
- A key change is a new revision/re-enrollment and cannot silently replace a
  local pin.
- A pure browser cannot be the initial trust anchor.
- The stored pin is the complete domain-separated 32-byte SHA-256 digest.
  Humans compare only its first 15 bytes as six four-character uppercase
  unpadded Base32 groups; this 120-bit display never replaces the full digest.
- `DeviceAttestationV1` is a closed restricted-RFC-8785 object containing both
  full subject key digests, canonical capabilities, IDs, and validity. Raw keys
  travel outside the signed object and must hash to those digests.
- The Phase 4a initial anchor is a Daemon remote identity sealed by portable
  password-derived Vault v1, not an OS-backed or pure-Web anchor. OS wrapping
  and Desktop product key storage remain Phase 5.

### Fixed v1 suite

- Device attestations: Ed25519.
- Pairwise root wrapping: RFC 9180 HPKE Auth mode with
  DHKEM(X25519, HKDF-SHA-256), HKDF-SHA-256, and ChaCha20-Poly1305.
- Traffic derivation: HKDF-SHA-256.
- Payload AEAD: XChaCha20-Poly1305.
- Signed/AAD objects: RFC 8785 JCS UTF-8.
- Hashes and key digests: SHA-256.
- Binary fields: unpadded Base64url.

v1 has no cipher-suite negotiation or downgrade. A future suite requires a new
protocol version and reviewed vectors.

### Enrollment proof of possession

Bootstrap/enrollment additionally use the protocol-vector
`multidesk-x25519-pop-context-v1`: a per-ceremony ephemeral X25519 shared
secret feeds HKDF-SHA-256, then HMAC-SHA-256 proves exchange-key possession and
Ed25519 signs the same length-framed transcript. The transcript binds API
version, purpose, ceremony/subject IDs, both subject public keys, `storageMode`,
the restricted-JCS `storageAssertionDigest`, server ephemeral public key,
32-byte challenge, and expiry. All-zero X25519, any field mutation, replay,
expiry, consume, or server restart fails closed.

### Pairwise roots

The session host generates a different uniformly random 32-byte
`pairwiseRootKey` for every
`(sessionId, hostDeviceId, peerDeviceId, keyEpoch)`. It HPKE-wraps that root
only to that peer. A root is never shared across peers or derived from another
peer/epoch root.

The host encrypts fan-out content separately for each peer. This overhead is
accepted for v0.1 because it preserves device identity and capability
boundaries. Possession of a pairwise root still does not grant an application
capability or ControllerLease.

### Traffic and nonce derivation

Each source/target/direction/stream derives separate 48-byte HKDF material from
the pairwise root. The first 32 bytes are the XChaCha traffic key and the final
16 bytes are the nonce prefix. The nonce is:

```text
noncePrefix || uint64be(sequence)
```

The authenticated header binds protocol version, message ID, source, target,
session, stream, kind, direction, decimal-string sequence, timestamp, key
epoch, and nonce. The receiver recomputes the traffic key and nonce from local
pair state and rejects `nonce_sequence_mismatch` before AEAD open.

### Replay and persistence

- `sequence` and `keyEpoch` are canonical unsigned decimal strings, avoiding
  JavaScript precision loss.
- Replay state is scoped to session, pair, epoch, direction, and stream.
- A 1024-value window accepts authenticated out-of-order frames and rejects
  duplicates and too-old values.
- Replay state and the resulting event/command commit atomically before ACK.
- Sequence reservation is durable. Crash/rollback ambiguity rotates the pair
  epoch rather than reusing a nonce.
- `sentAt` is an operational clock-skew hint, never the sole replay defense.

### Revocation

Revocation closes the Device's WSS, rejects the identity before decryption,
tombstones/rotates each affected pair, invalidates leases, and stops all future
wraps and frames to that Device. Re-enrollment creates a new identity and fresh
pairwise root. Other peer roots remain independent. Revocation never claims to
erase already copied plaintext or keys.

## Consequences

### Positive

- A malicious relay cannot substitute a key or change AAD-bound routing and
  authorization context without failure.
- Peer A cannot open or forge Peer B traffic merely because both participate in
  the same session.
- Deterministic nonce derivation is interoperable and prevents accidental
  random-nonce collision when durable sequence rules are followed.
- The same test inputs produce identical Go and TypeScript outputs on Linux,
  macOS, and Windows.

### Costs and obligations

- The host performs per-peer fan-out encryption instead of one group
  ciphertext.
- Phase 4b must use a conformant JCS parser/canonicalizer, validate X25519
  inputs, persist sequence/replay state transactionally, enforce capabilities
  and ControllerLease after cryptographic authentication, and test WSS flow
  control, queue expiry, redaction, fuzzing, and dependency provenance.
- Production code must never use deterministic test seeds or test-only HPKE
  EKM injection.
- Unknown versions, suites, critical fields, pins, epochs, or state ambiguity
  fail closed.
- Phase 4a consumes only the pin/attestation/PoP vector contract over HTTPS
  REST. Pairwise Roots, HPKE production code, WSS, terminal/Approval payloads,
  and Credential Grants remain Phase 4b/5 obligations.

## Residual risk

Static recipient X25519 key compromise plus captured wraps may expose that
pair's past data; v1 does not claim receiver post-compromise confidentiality.
A compromised approved peer can use its own keys/capabilities and retain
plaintext it legitimately received. Active same-origin Web compromise can use
an enrolled Web key while active. The Control Plane still sees metadata, size,
timing, and traffic patterns and can deny service. Revocation cannot erase
copied data.

## Evidence

- `docs/spikes/e2ee/PROTOCOL.md`
- `docs/spikes/e2ee/2026-07-14-e2ee-protocol-spike.md`
- `docs/spikes/e2ee/vectors.json`
- `docs/spikes/e2ee/go/`
- `docs/spikes/e2ee/typescript/`
- `docs/reviews/spike-e2ee-protocol-vectors/2026-07-14-security-review.md`
- PR #7 vector run `29375956127`
- Result SHA-256:
  `55bff1decd0b3419df4d43e32fe933e397a9167253c89f7a7d71552c178520f5`
