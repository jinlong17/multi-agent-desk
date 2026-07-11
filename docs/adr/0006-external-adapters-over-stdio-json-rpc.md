# ADR 0006: External adapters use stdio JSON-RPC, not Go plugins

- Status: Accepted
- Date: 2026-07-11
- Owner module: `provider`
- Impacted modules: `core`, `security`

## Context

Future third-party adapters need process isolation, language independence, and
cross-platform behavior. Go's plugin mechanism is platform- and toolchain-
sensitive and would load untrusted extension code into the Daemon process.

## Decision

Reserve an out-of-process adapter boundary using versioned JSON-RPC messages
over stdin/stdout. Built-in v0.1 adapters remain compiled Go code. The public
external Adapter SDK and marketplace are deferred to v0.2; the v0.1 protocol
namespace is internal and experimental.

## Spike-gated details

No wire schema, sandbox guarantee, credential delegation method, or public
compatibility promise is frozen in Phase 0. Provider-specific capabilities
remain gated by their own Spikes and compatibility evidence.

## Consequences

Adapter crashes can be isolated and implementations can use other languages.
The Daemon must eventually enforce framing, size, timeout, capability, and
secret-handling policy before the boundary becomes public.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§5.2, 11, 17
- [Module registry](../workflow/project/module-registry.json)
