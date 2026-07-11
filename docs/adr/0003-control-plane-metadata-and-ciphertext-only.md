# ADR 0003: Control Plane routes metadata and ciphertext only

- Status: Accepted
- Date: 2026-07-11
- Owner module: `control-plane`
- Impacted modules: `security`, `core`, `web`, `desktop`

## Context

Remote coordination requires device discovery, metadata sync, commands, and
message relay without turning the service into a Provider credential proxy or
terminal-content store.

## Decision

The Control Plane stores identity, approved device directory data, non-secret
configuration, session metadata, usage summaries, and audit metadata. It routes
E2EE envelopes and may queue short-lived ciphertext. It does not receive Vault
keys, Provider plaintext credentials, or plaintext terminal/model content, and
its public-key directory is not the trust anchor for pinned device keys.

## Spike-gated details

Envelope formats, algorithms, replay controls, browser key storage, and test
vectors remain pending `spike-e2ee-protocol-vectors` and
`spike-browser-key-storage`. No protocol suite or verified browser support is
frozen here.

## Consequences

Approved clients need separate device enrollment and pinned keys in addition
to user authentication. Server compromise must not reveal protected content,
but metadata exposure and availability risks remain.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§4.1, 5.3, 9
- [Project security invariants](../../CLAUDE.md)
