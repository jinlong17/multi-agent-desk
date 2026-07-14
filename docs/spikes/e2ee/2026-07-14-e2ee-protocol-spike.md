# E2EE protocol and cross-language vector Spike

- Date: 2026-07-14
- Workflow target: `spike-e2ee-protocol-vectors`
- Owner module: `security`
- Branch: `codex/security/spike-e2ee-protocol-vectors`
- Draft PR: [#7](https://github.com/jinlong17/multi-agent-desk/pull/7)
- Security gate: open; role-separated security review required

## Verdict

**EVIDENCE READY — the interoperability portion is supported.** The candidate
protocol is reproducible in independent Go and TypeScript implementations and
the tested tamper, pin, replay, and revocation-rotation failures fail closed.
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
  replay, and old-key-after-rotation cases fail; and
- Linux, macOS, and Windows runners return the same result hash.

Falsified when any implementation or platform diverges, any negative case is
accepted, or the security review finds a protocol-level flaw.

## Candidate decision under test

The candidate in [PROTOCOL.md](PROTOCOL.md) uses:

- Ed25519 for device attestations;
- RFC 9180 HPKE Auth mode with X25519, HKDF-SHA-256, and
  ChaCha20-Poly1305 for wrapping fresh 32-byte session root keys;
- HKDF-SHA-256 for direction/stream/epoch-separated traffic material;
- XChaCha20-Poly1305 for protected WebSocket payloads;
- RFC 8785 JCS for signatures and AAD;
- decimal strings for `uint64` sequence/epoch fields; and
- a durable direction-scoped replay window plus mandatory epoch rotation on
  revocation or state ambiguity.

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
  "resultSha256": "8df7d15b5c48ff9bba21938daae4a1649b00e2c9e6843e761c3f2de756c78be1"
}
```

Cross-platform GitHub run
[`29375412822`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375412822)
at commit `1b8286d9449d92a80ccc134de6623df9ed001349`:

| Runner | Job | Result |
|---|---|---|
| Linux | [`e2ee-vectors-ubuntu`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375412822/job/87227713997) | pass |
| macOS | [`e2ee-vectors-macos`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375412822/job/87227714045) | pass |
| Windows | [`e2ee-vectors-windows`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29375412822/job/87227714005) | pass |

All three jobs installed the exact nested lockfile and independently executed
the same comparator.

## Negative evidence

| Case | Expected | Observed |
|---|---|---|
| Change signed attestation bytes | Ed25519 verification fails | rejected in Go and TypeScript |
| Change HPKE wrap AAD target | HPKE open fails | rejected in Go and TypeScript |
| Supply a sender key different from local pin | HPKE Auth open fails | rejected in Go and TypeScript |
| Change protected envelope `kind` | XChaCha AEAD open fails | rejected in Go and TypeScript |
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
7. The operator waived a separate human reviewer. The required security-review
   workflow role remains mandatory and must record an explicit verdict before
   the ADR is accepted.

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
