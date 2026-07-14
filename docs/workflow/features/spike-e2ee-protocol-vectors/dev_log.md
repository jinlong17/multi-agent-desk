# Spike log: E2EE protocol spec and cross-language test vectors

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-e2ee-protocol-vectors` |
| Title | `E2EE protocol spec and cross-language test vectors` |
| Owner Module | `security` |
| Impacted Modules | `control-plane, web, core` |
| Hypothesis | `The E2EE protocol spec (pinning, attestation, AAD binding, session keys, revocation rotation) can be implemented in both Go and TypeScript passing one shared test-vector set, and survives one independent cryptographic review` |
| Time-box | `2 weeks; automated independent security role review accepted by operator, no human review required` |
| Current Phase | `SECURITY_REVIEW` |
| Status | `EVIDENCE_READY` |
| Executor | `Codex (GPT-5), provider-spike` |
| Updated | `2026-07-14 16:14 -0700` |
| Suggested Next | `security-review` |
| Security Gate | `open — independent cryptographic review required` |
| Evidence Path | `docs/spikes/e2ee/` |
| Decision Record | `pending — E2EE protocol ADR` |

## Success and failure criteria

- Supported when: identical vectors pass in Go and TypeScript and the independent review returns ACCEPTED.
- Falsified when: implementations diverge on any vector or the review finds a protocol-level flaw.

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

## Result, limitations, and fallback

Interoperability portion supported. RFC 9180 HPKE Auth mode, Ed25519
attestation, HKDF-separated XChaCha traffic, JCS AAD, replay behavior, and
revocation rotation have one shared deterministic vector set that matches in
Go and TypeScript on Linux, macOS, and Windows. Full hypothesis remains behind
the open security review. Fallback: keep affected Web clients metadata-only and
defer Phase 4b remote control rather than weaken the protocol.

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
