# Security review: E2EE protocol candidate v1

- Date: 2026-07-14
- Role: `security-review`
- Target: `spike-e2ee-protocol-vectors`
- Verdict: **REVISE**
- Evidence commit: `865af5d`

## Scope

Reviewed the candidate protocol, deterministic Go and TypeScript harness,
cross-platform run `29375412822`, threat-model invariants, local pinning,
attestation, HPKE Auth-mode session-key wrapping, JCS AAD, replay handling,
nonce construction, and revocation rotation.

Primary references checked:

- [RFC 9180](https://www.rfc-editor.org/rfc/rfc9180.html)
- [RFC 7748](https://www.rfc-editor.org/rfc/rfc7748.html)
- [RFC 8032](https://www.rfc-editor.org/rfc/rfc8032.html)
- [RFC 5869](https://www.rfc-editor.org/rfc/rfc5869.html)
- [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785.html)
- [RFC 5116](https://www.rfc-editor.org/rfc/rfc5116.html)

## Positive findings

- The Control Plane directory remains an index rather than a trust anchor.
- HPKE Auth mode uses the locally pinned sender X25519 public key; changing the
  sender key or wrap AAD fails in both implementations.
- Attestation bytes are domain-separated and Ed25519 signed.
- JCS AAD binds source, target, purpose, kind, stream, direction, sequence,
  epoch, timestamp, and message identity.
- Traffic material is HKDF-separated by direction, stream, endpoint pair,
  session, and epoch.
- Sequence-based deterministic XChaCha nonces are safe if sequence reservation
  is durable; crash ambiguity correctly requires key rotation.
- Duplicate/too-old replay and old-key-after-rotation negative vectors fail on
  Linux, macOS, and Windows.
- Residual-risk wording correctly avoids claims that revocation erases already
  copied plaintext or keys.

## Findings

### P1 — Shared session root enables cross-recipient impersonation

`PROTOCOL.md` currently generates one `sessionRootKey` per session/epoch and
wraps the same root to every approved recipient. Each recipient can therefore
derive traffic keys for every public source/target/direction/stream context,
not only its own pair. A device granted only `session.decrypt` could derive the
traffic key for another device's `client_to_device` control context and emit a
valid AEAD frame with that other device's identity. The symmetric AEAD would
authenticate the shared root holder, not the claimed source device.

This violates device-scoped capabilities and the threat-model requirement that
a compromised approved endpoint remain bounded to its identity and grant. AAD
does not solve the issue because every root holder can compute valid AAD and a
valid tag.

Required correction:

1. Generate a distinct, uniformly random `pairwiseRootKey` for every
   `(sessionId, hostDeviceId, peerDeviceId, pairEpoch)`.
2. Wrap only that pairwise root to that peer with HPKE Auth mode.
3. Derive both directions and all streams from the pairwise root, retaining
   direction/stream separation.
4. Keep receiver authorization and ControllerLease checks; possession of a
   pairwise key never grants a missing capability.
5. Add at least two peer devices to the shared vectors and prove that peer A
   cannot open or forge peer B traffic.
6. Define revocation/rotation per pair. Revoking peer A must stop future A
   traffic without forcing reuse or exposure of peer B material.

Alternative: retain a group content key but add an independently verified
per-sender signature/MAC key that recipients cannot derive. This is more
complex and is not recommended for v0.1.

### P2 — Receiver must recompute the nonce

The protocol derives `nonce = noncePrefix || uint64be(sequence)` and also
carries `nonce` in the authenticated header, but the receiver procedure does
not explicitly require recomputation and byte-for-byte equality before AEAD
open. A buggy or compromised sender could otherwise choose a nonce inconsistent
with durable sequence state and risk reuse.

Required correction: the receiver derives the traffic material, recomputes the
nonce from the canonical sequence, rejects a mismatch as
`nonce_sequence_mismatch`, and only then attempts AEAD open. Add a negative
vector for a non-derived nonce.

## Verdict rationale

The vector implementation is reproducible and its existing negative cases are
credible, but it faithfully validates a group-root design that does not
preserve sender identity among multiple recipients. Because this is a protocol
authorization flaw, the security gate cannot accept the candidate.

The deterministic fallback remains safe: keep protected Web/remote-control
surfaces `metadata_only` and do not freeze Phase 4b until corrected vectors and
another security review pass.

## Residual risk after correction

Pairwise keys will prevent one peer from cryptographically impersonating a
different peer, but a compromised peer can still act within its own granted
capabilities, use plaintext it legitimately received, and attack availability.
Application-layer capability and ControllerLease checks remain mandatory. An
active same-origin Web compromise can use that Web Device's own keys while
active. Revocation cannot erase previously copied data.

## Required next evidence

- Revised protocol text using pairwise roots.
- Two-peer Go/TypeScript vectors with exact cross-language equality.
- Negative cases for cross-peer open/forge and nonce/sequence mismatch.
- Linux, macOS, and Windows rerun.
- A new role-separated security-review verdict.
