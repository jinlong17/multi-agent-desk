# ADR 0002: Device Daemon owns secrets and Provider processes

- Status: Accepted
- Date: 2026-07-11
- Owner module: `core`
- Impacted modules: `provider`, `security`, `desktop`, `control-plane`

## Context

Provider credentials and interactive coding-agent processes must remain
device-local while multiple local or approved remote clients observe sessions.

## Decision

The Device Daemon is the local fact source for Vault state, CredentialInstance
materialization, Provider processes, sessions, attachments, leases, and the
device database. CLI/TUI and Desktop use local IPC; they do not treat the
database or Vault files as an API. The Control Plane never owns Provider
plaintext credentials or Provider processes.

## Spike-gated details

Credential locations, refresh concurrency, and auth behavior remain pending
`spike-codex-auth-refresh` and `spike-claude-config-keychain`. ADR 0013 resolves
`spike-windows-named-pipe-ipc`: Phase 1 uses protected, local-only Named Pipes
on Windows and Unix-domain sockets on macOS/Linux, with mutual protocol
authentication and per-request capability/lease authorization above both
transports. Provider credential mechanisms remain unfrozen.

## Consequences

Local operation can continue while the Control Plane is offline. The Daemon
must enforce single-writer credential materialization and revision/CAS rules.
Compromise of an authorized device remains a documented residual risk.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§4.1, 5.2, 8
- [Project security invariants](../../CLAUDE.md)
