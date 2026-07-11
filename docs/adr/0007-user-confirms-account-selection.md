# ADR 0007: System recommends; user confirms account selection

- Status: Accepted
- Date: 2026-07-11
- Owner module: `core`
- Impacted modules: `provider`, `web`, `desktop`

## Context

Users may have multiple accounts and runtime profiles, but automatic rotation
or hidden mid-session switching would create identity confusion and could be
used to evade Provider quotas or limits.

## Decision

The system may rank and explain eligible accounts, but the user confirms the
selection before starting a session. A running session pins its Account,
CredentialInstance, RuntimeProfile, Device, and capability snapshot. There is
no automatic account rotation, quota bypass, rate-limit evasion, or transparent
credential switch. Resume creates a new Session linked to the prior one.

## Spike-gated details

Provider usage and health signals used by future recommendations remain
pending Provider-specific Spike evidence. This ADR does not claim an official
quota API or a safe refresh mechanism for any Provider.

## Consequences

Recommendations must expose their evidence and freshness. Failure of a pinned
credential stops or fails the session rather than silently selecting another
account.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§6.7, 10.3, 12.3
- [Project security invariants](../../CLAUDE.md)
