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
| Current Phase | `PROVIDER_SPIKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex (GPT-5), feature-plan` |
| Updated | `2026-07-14 15:59 -0700` |
| Suggested Next | `provider-spike` |
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

## Result, limitations, and fallback

Pending. Fallback: reduce v0.1 remote-control scope rather than weaken the protocol.

## Risks and Blockers

- Blocks Phase 4b design freeze; independent reviewer availability is external.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-14 15:59 -0700 | Codex (GPT-5), feature-plan | Classified owner as `security`; froze the cross-language vector hypothesis, pinned toolchain expectations, retained the open cryptographic security gate, and recorded operator waiver of human review without waiving the security-review role | this file | `SPIKE_READY` | provider-spike |
