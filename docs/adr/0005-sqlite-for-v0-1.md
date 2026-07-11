# ADR 0005: SQLite for v0.1; PostgreSQL deferred

- Status: Accepted
- Date: 2026-07-11
- Owner module: `core`
- Impacted modules: `control-plane`

## Context

v0.1 is single-user and self-hosted. Both the Device Daemon and the initial
Control Plane need transactional persistence without an external database
requirement.

## Decision

Use separate SQLite databases in WAL mode for device-local and Control Plane
state in v0.1, with ordered migrations and explicit transaction boundaries.
PostgreSQL and multi-user tenancy are deferred until after v0.1.

## Spike-gated details

No Phase 0.5 Provider or security assumption is frozen by this storage choice.
E2EE payload formats remain opaque blobs to storage pending
`spike-e2ee-protocol-vectors`.

## Consequences

Installation and local development remain simple. Schemas must avoid relying
on SQLite quirks that would make later migration needlessly difficult, but no
PostgreSQL compatibility claim or migration schedule is made now.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§7, 14, 19
