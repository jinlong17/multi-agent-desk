# Ship receipt: Codex reserved start singleflight race

## Verdict

`SHIPPED`

The reserved-session singleflight race is repaired on protected remote
`main`. A late concurrent `StartReserved` caller now competes for the per-Session
gate before reading durable state, so it observes the completed running Session
instead of issuing a duplicate Provider start from a stale snapshot.

This receipt does not claim a tag, package, GitHub Release, publication or
deployment.

## Authorization and classification

The operator explicitly authorized automatic push, PR, protected-check,
rebase-merge and final reconciliation work. The bug is owned by `provider` with
secondary `core` and `project-system` impacts. Provider compatibility, platform
support, credential handling and storage CAS semantics did not change.

## Pull request and commit receipt

- Pull request: [#29](https://github.com/jinlong17/multi-agent-desk/pull/29)
- Locked final PR head:
  `776498b1470f734d515a9ded90fc517736326be4`
- Merge method: protected rebase merge
- Rebase-landed implementation commit / final main:
  `281c151d120f53365ecbc1f9150c084ca28d1205`
- Merged at: `2026-07-21T06:50:46Z` (`2026-07-20 23:50:46 PDT`)

The original commit contains the required DCO signoff. Rebase merge changed the
commit identity but not the content: direct tree comparison between locked PR
head `776498b` and final `main@281c151` is empty.

## Verification and protected checks

Independent bug verification reproduced the exact base failure, reviewed the
two-file implementation scope, and passed the deterministic regression,
single-P stress, target/package race, full Go/vet, workflow, link and license
checks. Root orchestration also passed the repository's complete local
`project:verify` and `ci:verify` gates before remote ship.

The locked final PR head passed all seven required checks:

| Check | Result | Receipt |
|---|---|---|
| `project-verify` | pass | run `29808094137`, job `88562818596` |
| `build-ubuntu` | pass | run `29808094137`, job `88562818586` |
| `build-macos` | pass | run `29808094137`, job `88562818624` |
| `build-windows` | pass | run `29808094137`, job `88562818563` |
| `license-gate` | pass | run `29808094113`, job `88562818520` |
| `dco` | pass | run `29808094113`, job `88562818617` |
| `link-check` | pass | run `29808094113`, job `88562818513` |

Exact final `main@281c151` passed the same seven-check matrix: CI run
`29808322809` (`project-verify` `88563532302`, Ubuntu `88563532297`, macOS
`88563532323`, Windows `88563532301`) and Governance run `29808322805`
(`license-gate` `88563532386`, DCO `88563532383`, link-check `88563532437`).

## Scope, residual risk, and rollback

The shipped diff changes reserved-start gate ordering and one focused
regression only. It does not change Store CAS behavior, SQLite policy, account
selection, Provider discovery, credentials, supported platforms or compatibility
claims. Residual risk is limited to ordinary concurrency-regression risk and is
covered by deterministic exact-once, stress, race and cross-platform CI evidence.

To roll back, create and review a revert of `281c151`; do not reset protected
`main`. No migration or data rollback is required. A rollback restores the
known duplicate-start and destructive-cleanup race, so it should be used only
with an explicit mitigation or replacement fix.

## Handoff

**Target**: `codex-reserved-start-singleflight-race`
**Completed**: `ship`
**Status**: `SHIPPED`
**Summary**: `Merged the reserved-start singleflight repair through protected PR #29 and verified all seven checks on the locked final head and exact remote main.`
**Commit/Release**: `PR #29 head 776498b; final main 281c151; no tag, release, publish, or deploy.`
**Tests**: `Deterministic reproduction/regression, stress/race/full local verification, final PR CI/Governance 29808094137 and 29808094113, and exact-main CI/Governance 29808322809 and 29808322805 all passed.`
**Blockers**: `none`

### Next Step

`Refresh PR #28 onto main@281c151 and require a new complete seven-check matrix before merge.`
