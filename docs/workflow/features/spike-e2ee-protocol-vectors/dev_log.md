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
| Time-box | `2 weeks including external review turnaround` |
| Current Phase | `INTAKE` |
| Status | `DRAFT` |
| Executor | `pending assignment` |
| Updated | `2026-07-10 20:56 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `open — independent cryptographic review required` |
| Evidence Path | `docs/spikes/e2ee/` |
| Decision Record | `pending — E2EE protocol ADR` |

## Success and failure criteria

- Supported when: identical vectors pass in Go and TypeScript and the independent review returns ACCEPTED.
- Falsified when: implementations diverge on any vector or the review finds a protocol-level flaw.

## Environment

| Field | Value |
|---|---|
| Tool + version | Go + TypeScript toolchains (pin at intake) |
| OS | Linux |
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
