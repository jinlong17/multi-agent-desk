# E2EE protocol and cross-language vector Spike

- Date: 2026-07-14
- Workflow target: `spike-e2ee-protocol-vectors`
- Owner module: `security`
- Branch: `codex/security/spike-e2ee-protocol-vectors`
- Draft PR: [#7](https://github.com/jinlong17/multi-agent-desk/pull/7)
- Security gate: open; role-separated security review required

Revision history:

- Candidate 1 at `1b8286d` proved cross-language interoperability but used one
  shared session root. The security review at `13cec27` returned `REVISE`
  because one peer could derive another peer's traffic keys.
- Candidate 2 at `8859530` replaces the group root with a distinct random
  Host↔Peer root and adds cross-peer plus nonce/sequence negative vectors.
- Phase 4a P0 (2026-07-21) preserves Candidate 2 and extends the same independent
  harness with the full pin digest/six-group fingerprint, typed restricted-JCS
  `DeviceAttestationV1`, and exact enrollment X25519/HKDF/HMAC/Ed25519 PoP.

## Verdict

**EVIDENCE READY — the interoperability portion is supported.** The revised
pairwise protocol is reproducible in independent Go and TypeScript
implementations and the tested tamper, pin, replay, cross-peer impersonation,
nonce/sequence, and revocation-rotation failures fail closed.
The broader hypothesis is not gate-resolved until the security-review role
accepts the protocol and the feature-plan role records the ADR.

This evidence does not claim that production E2EE is implemented. It freezes a
reviewable v1 candidate and test-vector contract for later Phase 4b code.

## Falsifiable hypothesis

The E2EE protocol candidate covering local pinning, device attestation, AAD
binding, authenticated session-key wrapping, direction/stream traffic keys,
replay state, and revocation rotation can produce exactly the same outputs in
Go and TypeScript from one shared input set, while negative cases are rejected.

Supported when:

- all public keys, canonical bytes, signatures, HPKE results, derived keys,
  nonces, XChaCha ciphertexts, pin digests, and rotation outputs match;
- attestation mutation, AAD mutation, wrong pinned sender, duplicate/too-old
  replay, cross-peer open/forge, nonce/sequence mismatch, and
  old-key-after-rotation cases fail; and
- Linux, macOS, and Windows runners return the same result hash.

The Phase 4a P0 extension additionally requires both implementations to reject
attestation schema/JCS violations; changes to either full key digest,
capabilities, IDs, or expiry; all-zero X25519; and any PoP purpose/ID/key/
`storageMode`/`storageAssertionDigest`/challenge/expiry/server-ephemeral
mutation, replay, or restart.

Falsified when any implementation or platform diverges, any negative case is
accepted, or the security review finds a protocol-level flaw.

## Candidate decision under test

The candidate in [PROTOCOL.md](PROTOCOL.md) uses:

- Ed25519 for device attestations;
- RFC 9180 HPKE Auth mode with X25519, HKDF-SHA-256, and
  ChaCha20-Poly1305 for wrapping a fresh, distinct 32-byte pairwise root for
  each Host↔Peer relationship;
- HKDF-SHA-256 for direction/stream/epoch-separated traffic material;
- XChaCha20-Poly1305 for protected WebSocket payloads;
- RFC 8785 JCS for signatures and AAD;
- decimal strings for `uint64` sequence/epoch fields; and
- a durable direction-scoped replay window plus mandatory epoch rotation on
  revocation or state ambiguity.

The host encrypts fan-out content separately for each peer. A peer receives
only its own pairwise root and therefore cannot derive or authenticate another
peer's traffic context. Capability and ControllerLease checks remain mandatory
after cryptographic authentication.

The Control Plane directory remains an untrusted index. The recipient supplies
the sender public key from its local pin; the header digest alone never becomes
a trust anchor.

## Environment and pinned dependencies

| Surface | Version / dependency | License | Role |
|---|---|---|---|
| Go | `1.26.5` | BSD-3-Clause | Go vector runner |
| Node.js | `24.14.0` local; repo contract `24.x` | MIT | TypeScript runner and comparator |
| pnpm | `10.23.0` | MIT | exact dependency install |
| Cloudflare CIRCL | `1.6.4` | BSD-3-Clause | RFC 9180 HPKE Auth mode in Go |
| `golang.org/x/crypto` | `0.54.0` | BSD-3-Clause | XChaCha20-Poly1305 in Go |
| `@hpke/core` | `1.9.0` | MIT | HPKE core in TypeScript |
| `@hpke/dhkem-x25519` | `1.8.0` | MIT | X25519 KEM in TypeScript |
| `@hpke/chacha20poly1305` | `1.8.0` | MIT | HPKE AEAD in TypeScript |
| `@noble/ciphers` | `2.2.0` | MIT | XChaCha20-Poly1305 in TypeScript |

Exact Go checksums are in `go/go.sum`; exact npm integrity values are in
`typescript/pnpm-lock.yaml`. No real key, credential, account, or Provider
secret is used. All seeds in `vectors.json` are deterministic public fixtures.

## Reproduction

From the repository root with Go 1.26.5 and Node.js 24 on `PATH`:

```bash
pnpm --dir docs/spikes/e2ee/typescript install --frozen-lockfile --ignore-workspace
node docs/spikes/e2ee/verify.mjs
```

Direct implementation checks:

```bash
cd docs/spikes/e2ee/go
go vet ./...
go test ./...
go run . ../vectors.json

cd ../typescript
node validate.mjs ../vectors.json
```

The comparator runs both implementations, parses their results, requires deep
equality, asserts every negative result, and emits only a sanitized manifest.

## Result

Local macOS result:

```json
{
  "schemaVersion": 1,
  "result": "pass",
  "implementations": ["go", "typescript"],
  "resultSha256": "55bff1decd0b3419df4d43e32fe933e397a9167253c89f7a7d71552c178520f5"
}
```

The earlier Candidate 2 cross-platform receipt remains historical evidence for
`082033265c774aad70fccf89e1a682a5f411ca14c1e675eca346184dff8da2a5`.
The P0 hash above is the current local contract result and requires a fresh
cross-platform CI receipt before it can be described as cross-platform evidence.

### Phase 4a P0 dependency readiness record

P0 adds no product dependency or lockfile entry. It records the exact inputs
that P1 must lock and license-scan before any import or generated artifact:

| Input | Provenance and integrity | License/toolchain finding |
|---|---|---|
| `github.com/go-webauthn/webauthn v0.17.4` | [immutable verified release](https://github.com/go-webauthn/webauthn/releases/tag/v0.17.4), commit `bc5e90d68ad5afd2a8aeef1a7af80f493c14526b`, Go sum `h1:KFTSz3R2RYDiUn/0cDi3XTJgFenSG74eKTTHlqWhlxk=` | BSD-3-Clause; module declares Go 1.25 and toolchain 1.26.3 |
| `github.com/oapi-codegen/oapi-codegen/v2 v2.8.0` | [immutable release](https://github.com/oapi-codegen/oapi-codegen/releases/tag/v2.8.0), commit `de2d8b2b0afb287198554eb305bb0d2687d26a85`, Go sum `h1:s4hxMxuqtR8jPzXkBTtFwY/SBuj3gEAYikmbBSdtLMM=` | Apache-2.0; module declares Go 1.25 and pins `kin-openapi v0.142.0`; release validates the specification before generation |
| `github.com/getkin/kin-openapi v0.142.0` | [tagged source](https://github.com/getkin/kin-openapi/tree/v0.142.0), commit `1223a0f215d2cf9beb2d9eb9ea2649d001c21388`, Go sum `h1:izj0vBdFprMhitfzaX8sTqztsEQyvwhssBoB6n8NO7w=` | MIT; module declares Go 1.25; tool graph must be included in the P1 license scan |
| `github.com/google/uuid v1.6.0` | [tagged source](https://github.com/google/uuid/tree/v1.6.0), commit `0f11ee6918f41a04c201eceeadf612a377bc7fbc`, Go sum `h1:NIvaJDMOsjHA8n1jAhLSgzrAzy1Hgr+hNrb57e+94F0=` | BSD-3-Clause; direct P1 UUIDv7 dependency |
| `openapi-typescript 7.13.0` | [npm registry manifest](https://registry.npmjs.org/openapi-typescript/7.13.0), integrity `sha512-EFP392gcqXS7ntPvbhBzbF8TyBA+baIYEm791Hy5YkjDYKTnk/Tn5OQeKm5BIZvJihpp8Zzr4hzx0Irde1LNGQ==` | MIT; types only, no runtime client |

The repository pins Go 1.26.5, Node 24, and pnpm 10.23.0, which satisfy these
declared minimums. P0's current resolved repository license scan must pass, but
the proposed dependencies are deliberately not added here; P1 must add exact
locks and scan the complete resulting runtime and tool graphs before enabling
generation. `openapi-fetch` is intentionally absent and remains prohibited by
the approved first-party runtime-client decision.

Revised cross-platform GitHub run
[`29375956127`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375956127)
at commit `885953007916a9d98b82037c0f4ddbb325aec435`:

| Runner | Job | Result |
|---|---|---|
| Linux | [`e2ee-vectors-ubuntu`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375956127/job/87229354426) | pass |
| macOS | [`e2ee-vectors-macos`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375956127/job/87229354460) | pass |
| Windows | [`e2ee-vectors-windows`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375956127/job/87229354410) | pass |

All three jobs installed the exact nested lockfile and independently executed
the same comparator.

## Negative evidence

| Case | Expected | Observed |
|---|---|---|
| Change signed attestation bytes | Ed25519 verification fails | rejected in Go and TypeScript |
| Change HPKE wrap AAD target | HPKE open fails | rejected in Go and TypeScript |
| Supply a sender key different from local pin | HPKE Auth open fails | rejected in Go and TypeScript |
| Change protected envelope `kind` | XChaCha AEAD open fails | rejected in Go and TypeScript |
| Peer A opens Peer B ciphertext | pairwise traffic key mismatch | rejected in Go and TypeScript |
| Peer A forges a frame claiming Peer B | Host's Peer B key/nonce rejects it | rejected in Go and TypeScript |
| Carry nonce for sequence 101 while header says 100 | receiver recomputation detects mismatch | rejected in Go and TypeScript |
| Replay sequence `100` | replay window rejects duplicate | rejected in Go and TypeScript |
| Supply sequence below 64-entry vector window | replay window rejects too-old value | rejected in Go and TypeScript |
| Open epoch-2 frame with epoch-1 traffic key | XChaCha AEAD open fails | rejected in Go and TypeScript |
| Open epoch-2 frame with epoch-2 key | plaintext matches | accepted in Go and TypeScript |

The vector uses a compact 64-entry bitmap to make boundary behavior visible.
The protocol requires a 1024-entry production window with identical semantics.

## Primary-source checks

- [RFC 9180](https://www.rfc-editor.org/rfc/rfc9180.html) defines HPKE and
  includes X25519/HKDF-SHA-256/ChaCha20-Poly1305 suites and Auth mode.
- [RFC 7748](https://www.rfc-editor.org/rfc/rfc7748.html) defines X25519 and
  requires safe handling of all-zero shared secrets.
- [RFC 8032](https://www.rfc-editor.org/rfc/rfc8032.html) defines Ed25519.
- [RFC 5869](https://www.rfc-editor.org/rfc/rfc5869.html) defines HKDF and the
  separation role of `salt` and `info`.
- [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785.html) defines deterministic
  JSON serialization and I-JSON constraints; verified errata motivate explicit
  rejection of negative zero.
- [RFC 5116](https://www.rfc-editor.org/rfc/rfc5116.html) defines AEAD and
  nonce-uniqueness requirements.
- [Libsodium's XChaCha20-Poly1305 documentation](https://doc.libsodium.org/secret-key_cryptography/aead/chacha20-poly1305/xchacha20-poly1305_construction)
  documents the 192-bit extended nonce construction.
- [Cloudflare CIRCL HPKE](https://pkg.go.dev/github.com/cloudflare/circl/hpke)
  exposes RFC 9180 Auth-mode sender and receiver contexts.
- [hpke-js documentation](https://dajiaji.github.io/hpke-js/docs/) exposes the
  corresponding browser/TypeScript contexts and deterministic test-only EKM.

## Limitations and residual questions

1. This is deterministic protocol evidence, not production code, fuzzing,
   side-channel analysis, memory-zeroization proof, or formal verification.
2. The selected HPKE and XChaCha libraries are version-pinned but still require
   normal dependency review, vulnerability monitoring, and SBOM/provenance work
   before release.
3. An active XSS in an enrolled Web Device can use its keys and decrypted
   content while the origin is compromised; non-exportable storage does not
   prevent authorized key use.
4. Metadata, timing, size, traffic patterns, and denial of service remain
   visible to the Control Plane.
5. Device revocation cannot erase plaintext or keys already copied by an
   authorized or compromised recipient.
6. Sequence persistence, transactional replay state, WSS flow control, queue
   expiry, and redaction must be implemented and tested in Phase 4b.
7. The first role-separated security review correctly rejected the shared-root
   candidate. The revised pairwise candidate still requires a new explicit
   security-review verdict before the ADR is accepted. The operator waived a
   separate human reviewer, not the automated security gate.

## Deterministic fallback

If the security review returns `REVISE` or any later implementation diverges
from the shared vectors:

- do not freeze or ship Phase 4b remote terminal, approvals, remote control, or
  Credential Grant;
- keep affected Web clients `metadata_only`;
- allow local CLI/Daemon operation and non-secret metadata sync only; and
- re-enter the Spike with a versioned protocol change and a new vector set.

The fallback never removes AAD binding, local pinning, replay protection,
revocation rotation, or ciphertext-only relay constraints.
