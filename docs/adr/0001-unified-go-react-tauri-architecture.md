# ADR 0001: Unified Go, React, and Tauri architecture

- Status: Accepted
- Date: 2026-07-11
- Owner module: `project-system`
- Impacted modules: `core`, `provider`, `control-plane`, `web`, `desktop`, `security`

## Context

MultiAgentDesk needs one terminal-first device kernel, a browser UI, and a
desktop shell without duplicating domain behavior across runtimes.

## Decision

Use Go for CLI/TUI, Device Daemon, Control Plane, and built-in Provider logic;
React and TypeScript for Web/PWA and shared UI/protocol packages; and Tauri 2
as the thin desktop shell around the shared Web UI and Go Daemon sidecar. The
physical layout is governed by [ADR 0009](0009-repository-layout-authority.md).

## Spike-gated details

ADR 0015 resolves the Windows Tauri sidecar lifecycle as discover-first,
crash-surviving, fixed Rust-side launch with authenticated ownership; signed
Windows 11 release acceptance remains pending. ADR 0010 resolves browser key
storage modes and ADR 0011 resolves the E2EE protocol candidate. Production
implementation and release acceptance remain gated.

## Consequences

Business and domain rules live in Go or shared contracts, not Tauri commands.
The UI can be reused across Web and Desktop while platform integration stays
thin. Cross-language contracts require fixtures and compatibility tests.

## References

- [Implementation plan](../IMPLEMENTATION_PLAN.md) §§4, 5, 17
- [Module registry](../workflow/project/module-registry.json)
