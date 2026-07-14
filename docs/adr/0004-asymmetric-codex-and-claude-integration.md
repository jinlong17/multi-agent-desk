# ADR 0004: Codex and Claude integrations remain asymmetric

- Status: Accepted
- Date: 2026-07-11
- Owner module: `provider`
- Impacted modules: `core`, `security`

## Context

Codex and Claude Code expose different official integration surfaces. A forced
lowest-common-denominator protocol would hide capabilities or depend on
unsupported private behavior.

## Decision

Represent Provider capabilities explicitly. The planned Codex built-in adapter
uses the official `codex app-server` structured surface; the planned Claude
Code adapter uses the official CLI through a PTY plus documented hooks and
status commands. Provider-specific events remain in their adapters and are
normalized only where semantics genuinely match.

## Spike-gated details

Codex schemas, usage methods, credential behavior, and refresh safety remain
pending `spike-codex-auth-refresh`. Claude config/keychain isolation, auth
status, setup-token, hooks, and PTY behavior remain pending
`spike-claude-config-keychain`. Windows PTY transport uses the native ConPTY
backend under [ADR 0012](0012-windows-conpty-pty-backend.md); Windows 11
real-provider and UI acceptance remains an implementation/release gate. No
undocumented Provider behavior is asserted by this ADR.

## Consequences

Capability snapshots are attached to sessions and UI behavior degrades
explicitly when a Provider lacks a capability. Provider integration cannot be
implemented or frozen ahead of the owning Spike evidence.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§10–12
- [Provider compatibility placeholder](../PROVIDER_COMPATIBILITY.md)
