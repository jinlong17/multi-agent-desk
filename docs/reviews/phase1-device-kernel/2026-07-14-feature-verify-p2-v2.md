# Feature verification v2: Phase 1 Device Kernel P2

- Date: 2026-07-14
- Role: `feature-verify`
- Phase: `P2 identity, IPC, and Daemon lifecycle`
- Verified head: `c9cb5c66698698dc00d8a8086a491da96ea53edf`
- Pull request: `#13` (Draft)
- Verdict: `VERIFIED`

## Conclusion

P2 now meets its identity, framing, mutual-authentication, authorization,
bounded-server, service-specification, and native local-IPC acceptance. The
v1 shutdown finding is closed: `Server` tracks bounded active connections,
closes them on shutdown, and uses context-aware accept cancellation. The fresh
macOS, Ubuntu, and Windows runners all pass the complete Device test suite,
including the Windows Named Pipe round-trip and shutdown test. P3 is unlocked;
the final Security Gate remains open.

## Finding closure

1. `Server` retains accepted connections in a bounded set and closes every
   active connection when `Close` or `Serve` shutdown runs.
2. `acceptWithContext` prevents a blocking native `Accept` from outliving the
   Serve context; `net.ErrClosed` is treated as normal endpoint shutdown.
3. The test clients use the public `Server.Close` lifecycle, and the same
   authenticated request/response path is re-exercised after the correction.

## Evidence

- Exact PR head `c9cb5c66698698dc00d8a8086a491da96ea53edf`; PR #13 remains Draft,
  mergeable, and clean against `main`.
- CI run `29390736909`: `project-verify` passed in 14s, `build-macos` passed in
  1m41s, `build-ubuntu` passed in 2m31s, and `build-windows` passed in 3m43s.
- Governance run `29390736750`: `license-gate`, `dco`, and `link-check` all
  passed.
- Actual Windows Server log job `87273433004` reports `ok` for
  `internal/device` in 2.419s, with `internal/app`, `internal/domain`,
  `internal/storage`, and migrations also passing. This includes
  `TestNamedPipeAuthenticatedDaemon` and the cross-platform shutdown journey;
  no failure output remains.
- Local full tests, race suites, vet, three-target Device compile, exact Go
  license scan, project/CI/scaffold, and service/authorization tests pass.

## Gates and scope

- Provider Gate: none for the deterministic first-party Fake Provider.
- Security Gate: remains open for the final Phase 1 security review.
- P2 blockers: none.
- Windows Server CI still does not prove Windows 11 multi-user/service,
  signed packaging, ConPTY, IME, or accessibility behavior.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-verify / P2 identity, IPC, and Daemon lifecycle`
**Verdict**: `VERIFIED`
**Summary**: Identity, strict authenticated protocol, authorization, native Unix/Named Pipe transport, and Daemon shutdown now pass on actual macOS, Ubuntu, and Windows runners.
**Evidence**: Exact head c9cb5c6; CI run 29390736909; Governance run 29390736750; Windows job 87273433004; retained local full/race/license/project evidence.
**Findings**: None blocking P2; Windows 11 release and multi-user acceptance remain later platform gates.
**Blockers**: None for starting P3; the final Phase 1 Security Gate remains open.

### Next Step

Run `feature-build` P3 Fake runtime and Session control for `phase1-device-kernel`.
