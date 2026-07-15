# ADR 0008: Configuration sync is separate from credential grants

- Status: Accepted
- Date: 2026-07-11
- Owner module: `security`
- Impacted modules: `core`, `control-plane`, `web`, `desktop`

## Context

Ordinary profile configuration needs convenient synchronization, while
Provider credentials require explicit, target-device-scoped authorization and
stronger trust semantics.

## Decision

Sync non-secret profile and workspace configuration separately from secrets.
A credential moves only through an explicit CredentialGrant to an eligible,
approved target device; it is encrypted for that target and creates a distinct
CredentialInstance. Revocation stops future use but is never described as
remote erasure of a secret already copied to a compromised device.

## Spike-gated details

ADR 0011 resolves the pairwise envelope protocol, ADR 0010 resolves browser key
storage modes, and ADR 0014 resolves the Codex credential-write boundary
without permitting plaintext config sync or multi-writer refresh. ADR 0016
selects target-local Claude interactive login and explicitly excludes
setup-token from stable v0.1 CredentialGrant until separate evidence and review.

## Consequences

Control Plane configuration sync can remain plaintext/non-secret while grants
follow a separately audited protocol. Grant flows require pinned device keys,
replay protection, revision semantics, and independent security review before
implementation ships.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§8–10
- [Project security invariants](../../CLAUDE.md)
