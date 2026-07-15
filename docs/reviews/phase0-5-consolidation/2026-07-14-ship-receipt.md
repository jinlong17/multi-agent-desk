# Ship receipt: phase0-5-consolidation

- Status: `SHIPPED`
- Authorized by: operator sequential-execution directive, including branch,
  push, merge, phase completion, and ship
- Feature pull request: <https://github.com/jinlong17/multi-agent-desk/pull/11>
- Verified feature head:
  `bcaeff6b46fa7a9dd7806e802778128cb19b3cae`
- Main integration:
  `3ee7ee1cbc267b6cf9a7464fcb5bee5f7d0dc5a4`
- Merged: 2026-07-14 20:25 -0700
- Release/tag/deployment: none

## Shipped outcome

Phase 0.5 is complete at the decision and compatibility-evidence level. The
seven Spike authorities, ADR 0010-0016, exact-version compatibility rows,
fallbacks, security boundaries, and deferred platform/provider acceptance
gates are integrated on protected `main`. The plan, README, threat model,
manual dashboard, and static dashboard fallback now identify Phase 1 Device
Kernel as the current implementation phase.

This receipt does not claim that the selected designs are production
implementations, that Windows Desktop is stable, or that a product release or
deployment occurred.

## Pre-merge evidence

The exact final evidence head passed every required check:

| Workflow run | Check | Result |
|---|---|---|
| CI `29386319103` | `project-verify` | pass, 19s |
| CI `29386319103` | `build-macos` | pass, 1m28s |
| CI `29386319103` | `build-ubuntu` | pass, 1m43s |
| CI `29386319103` | `build-windows` | pass, 3m32s |
| Governance `29386319063` | `dco` | pass, 7s |
| Governance `29386319063` | `link-check` | pass, 11s |
| Governance `29386319063` | `license-gate` | pass, 36s |

PR #11 was ready, `MERGEABLE` / `CLEAN`, and based on
`1e027573f401ee8115ba0a5e321a0540052d7a9c`. It was squash-merged only after
the exact-head checks above passed.

## Post-merge main evidence

The resulting `main` head
`3ee7ee1cbc267b6cf9a7464fcb5bee5f7d0dc5a4` passed both push workflows:

| Workflow run | Checks | Result |
|---|---|---|
| CI `29386512031` | `project-verify`, `build-macos`, `build-ubuntu`, `build-windows` | all pass; Windows completed in 3m22s |
| Governance `29386512023` | `dco`, `link-check`, `license-gate` | all pass |

Authenticated protection readback retained strict seven required contexts,
admin enforcement, conversation resolution, linear history, disabled force
push/deletion, and the operator-approved one-account rule of zero reviews and
no CODEOWNER-review requirement.

## Local and lifecycle checks

- Independent P1 verification: exact seven Spikes, seven accepted ADRs, seven
  evidence paths, seven resolved compatibility rows, and matching Go/TypeScript
  E2EE vectors.
- Independent P2 verification: plan/manual/static phase state, README/toolchain
  truth, one-account/two-profile scope, retained Windows/Provider gates, and
  full Go/Web/Tauri scaffold regression.
- Provider Gate: resolved at the decision level.
- Security Gate: none for this consolidation; all underlying accepted security
  verdicts remain in force.
- Required docs, links, DCO, licenses, branch protection, rollback, and exact
  diff were checked before merge.
- No version bump or release notes were required because this was a project
  phase transition, not a software release.

## Residual gates retained

- Phase 1 must implement cross-platform Fake Session, Unix socket/Named Pipe,
  protocol authentication, capability/lease enforcement, daemon lifecycle,
  SQLite, Vault state, and recovery.
- Codex remains one canonical credential writer; no 48-hour, multi-writer, or
  completed headless-auth claim.
- Claude remains target-profile official interactive login; setup-token grant,
  distinct-account isolation, and long-session claims remain unsupported.
- Windows 11 real-device Provider TUI, IME/accessibility, multi-user/service,
  signed packaging, update/rollback/uninstall, logoff/sleep/reboot, and security
  software acceptance remain Phase 3/6 gates.

## Rollback

The merge is protected and history is linear. Any correction uses a new signed
branch and protected pull request; original Spike evidence and accepted ADR
history are not rewritten. If a compatibility assumption regresses, its owning
later phase fails closed or records a superseding ADR instead of reverting to
an unsupported claim.

## Next step

Create the `phase1-device-kernel` Feature Brief, classify it under `core`, and
run `feature-plan` before writing product code.
