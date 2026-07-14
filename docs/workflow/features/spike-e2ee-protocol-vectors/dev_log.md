# Spike log: E2EE protocol spec and cross-language test vectors

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-e2ee-protocol-vectors` |
| Title | `E2EE protocol spec and cross-language test vectors` |
| Owner Module | `security` |
| Impacted Modules | `control-plane, web, core` |
| Hypothesis | `The E2EE protocol spec can use a distinct random pairwise root per Host↔Peer, retain pinning/attestation/AAD/replay/revocation guarantees, reject cross-peer impersonation and nonce/sequence mismatch, pass one shared Go/TypeScript vector set, and survive the security-review role` |
| Time-box | `2 weeks; automated independent security role review accepted by operator, no human review required` |
| Current Phase | `FEATURE_PLAN` |
| Status | `ACCEPTED` |
| Executor | `Codex (GPT-5), security-review` |
| Updated | `2026-07-14 16:27 -0700` |
| Suggested Next | `feature-plan decision` |
| Security Gate | `resolved — ACCEPTED pairwise-root candidate` |
| Evidence Path | `docs/spikes/e2ee/` |
| Decision Record | `pending — E2EE protocol ADR` |

## Success and failure criteria

- Supported when: identical vectors pass in Go and TypeScript, Peer A cannot
  open or forge Peer B traffic, a nonce inconsistent with the canonical
  sequence is rejected, revocation rotates the affected pair, and the
  security-review role returns ACCEPTED.
- Falsified when: any implementation/platform diverges, either new negative
  case is accepted, a shared root remains accessible to multiple peers, or the
  review finds another protocol-level flaw.

## Environment

| Field | Value |
|---|---|
| Tool + version | Go `1.26.5`; Node.js `24.x`; pnpm `10.23.0`; crypto dependencies must be exact-pinned in the evidence harness |
| OS | Linux primary vector runner; macOS local authoring; Windows compatibility through GitHub Actions |
| Auth mode | not applicable (test vectors, no real keys) |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-14 16:12 -0700 | `node docs/spikes/e2ee/verify.mjs`; Go `vet` / `test`; exact pinned Go and npm locks | Go and TypeScript produced identical canonical outputs and result SHA-256 `8df7d15b5c48ff9bba21938daae4a1649b00e2c9e6843e761c3f2de756c78be1`; all negative cases rejected | `docs/spikes/e2ee/` |
| 2026-07-14 16:14 -0700 | GitHub Actions run `29375412822` at `1b8286d9449d92a80ccc134de6623df9ed001349` | Linux, macOS, and Windows `e2ee-vectors-*` jobs all passed | `docs/spikes/e2ee/2026-07-14-e2ee-protocol-spike.md` |
| 2026-07-14 16:16 -0700 | role-separated security review of protocol and vectors | `REVISE`: shared root lets one recipient derive another recipient's traffic keys; receiver nonce recomputation is underspecified | `docs/reviews/spike-e2ee-protocol-vectors/2026-07-14-security-review.md` |
| 2026-07-14 16:23 -0700 | revised `node docs/spikes/e2ee/verify.mjs`; Go `vet` / `test` | pairwise-root Go/TypeScript outputs matched at SHA-256 `082033265c774aad70fccf89e1a682a5f411ca14c1e675eca346184dff8da2a5`; cross-peer open/forge and nonce/sequence mismatch rejected | `docs/spikes/e2ee/` |
| 2026-07-14 16:25 -0700 | GitHub Actions run `29375956127` at `885953007916a9d98b82037c0f4ddbb325aec435` | revised vectors passed on Linux, macOS, and Windows | `docs/spikes/e2ee/2026-07-14-e2ee-protocol-spike.md` |
| 2026-07-14 16:27 -0700 | second role-separated security review | `ACCEPTED`: P1 shared-root and P2 nonce findings closed; no remaining P0/P1 | `docs/reviews/spike-e2ee-protocol-vectors/2026-07-14-security-review.md` |

## Result, limitations, and fallback

Revised interoperability portion supported. A distinct random pairwise root per
Host↔Peer prevents one peer from deriving or authenticating another peer's
traffic. RFC 9180 HPKE Auth mode, Ed25519 attestation, HKDF-separated XChaCha
traffic, mandatory nonce recomputation, JCS AAD, replay behavior, and pairwise
revocation rotation match in Go and TypeScript on Linux, macOS, and Windows.
Full hypothesis remains behind the open security review. Fallback: keep affected
Web clients metadata-only and defer Phase 4b remote control rather than weaken
the protocol.

## Risks and Blockers

- Blocks Phase 4b design freeze until the security-review role records an
  accepted verdict and feature-plan records the ADR.
- Production implementation, transactional replay persistence, WSS flow
  control, fuzzing, side-channel analysis, and dependency provenance remain
  outside this Spike.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-14 15:59 -0700 | Codex (GPT-5), feature-plan | Classified owner as `security`; froze the cross-language vector hypothesis, pinned toolchain expectations, retained the open cryptographic security gate, and recorded operator waiver of human review without waiving the security-review role | this file | `SPIKE_READY` | provider-spike |
| 2026-07-14 16:14 -0700 | Codex (GPT-5), provider-spike | Specified the v1 candidate, implemented independent Go and TypeScript vector runners, compared exact outputs, ran negative cases, and reproduced the result on Linux/macOS/Windows | `1b8286d`; `docs/spikes/e2ee/`; Actions `29375412822` | `EVIDENCE_READY` | security-review |
| 2026-07-14 16:16 -0700 | Codex (GPT-5), security-review | Reviewed trust boundaries, HPKE/AAD/replay/rotation semantics, and cross-platform evidence; found shared-root cross-recipient impersonation and missing mandatory nonce recomputation | `docs/reviews/spike-e2ee-protocol-vectors/2026-07-14-security-review.md` | `REVISE` | feature-plan re-scope |
| 2026-07-14 16:18 -0700 | Codex (GPT-5), feature-plan | Re-scoped the candidate to a distinct random pairwise root per Host↔Peer and added cross-peer impersonation plus nonce/sequence mismatch acceptance criteria | this file | `SPIKE_READY` | provider-spike |
| 2026-07-14 16:25 -0700 | Codex (GPT-5), provider-spike | Replaced the shared root with distinct pairwise roots, required receiver nonce recomputation, added two-peer open/forge and nonce mismatch vectors, and reproduced exact results on Linux/macOS/Windows | `8859530`; Actions `29375956127`; `docs/spikes/e2ee/` | `EVIDENCE_READY` | security-review |
| 2026-07-14 16:27 -0700 | Codex (GPT-5), security-review | Re-reviewed pairwise roots, HPKE/pins, AAD/nonce derivation, replay, revocation, and revised cross-platform vectors; confirmed prior findings closed and recorded residual risk | `docs/reviews/spike-e2ee-protocol-vectors/2026-07-14-security-review.md` | `ACCEPTED` | feature-plan decision |
