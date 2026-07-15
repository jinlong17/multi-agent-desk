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

ADR 0014 verifies Codex `account/rateLimits/read` and `account/usage/read` on
the exact tested versions and selects a single-writer refresh boundary. Claude
usage accuracy and other Provider selection signals remain pending their own
evidence. This ADR still does not authorize automatic rotation or account
switching.

## Consequences

Recommendations must expose their evidence and freshness. Failure of a pinned
credential stops or fails the session rather than silently selecting another
account.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§6.7, 10.3, 12.3
- [Project security invariants](../../CLAUDE.md)
