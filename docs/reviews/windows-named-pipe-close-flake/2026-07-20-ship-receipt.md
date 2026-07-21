# Ship receipt: Windows Named Pipe close flake

## Verdict

`SHIPPED`

The diagnosed Windows Named Pipe close race is repaired on protected remote
`main`. The implementation, independent verification record, and bug state
were integrated through PR #26, and all seven required checks passed both on
the final PR head and on the final combined `main` that also contains the
independent Provider test-fixture fix.

This receipt does not claim a tag, package, GitHub Release, publication,
deployment, long-duration Windows soak, or broader Windows 11 release
certification.

## Authorization and classification

The operator explicitly authorized automatic push, PR, protected-check,
rebase-merge and final reconciliation work. This reconciliation is owned by
`project-system`; the shipped bug remains `core`-owned, with `security` and
`project-system` impacts. No dashboard priority or release risk was changed.

## Pull request and commit receipt

- Pull request: [#26](https://github.com/jinlong17/multi-agent-desk/pull/26)
- Base before merge: `154f882e8baea8165b729dfa53d7bdf4d3e546f1`
- Locked final PR head: `3350c8241c5aaadb2a7a4e6439822c7ddd2ccacf`
- Merge method: protected rebase merge
- Rebase-landed implementation commit:
  `0891bdb2d3454cdec747db63b6a77d0e99696086`
- PR #26 integration main:
  `8e15dd13f0a4c79948a360070639b2a17219d73b`
- Merged at: `2026-07-21T02:30:14Z` (`2026-07-20 19:30:14 PDT`)

The direct tree comparison between the locked PR head and
`main@8e15dd1` is empty. Rebase changed commit identities, not the reviewed
source content.

## Verification and protected checks

Independent bug verification at implementation head `4454dfc` found no
finding and recorded `READY_TO_SHIP`. Windows job `88525033621` executed the
complete `internal/device` package twice, covering the authenticated idle-close
path and 64 aggregate pending-read close iterations, plus pending accept,
deadline cancellation and repeated close.

The final PR head then passed all seven required checks:

| Check | Result | Receipt |
|---|---|---|
| `project-verify` | pass | run `29795745504`, job `88526604389` |
| `build-ubuntu` | pass | run `29795745504`, job `88526604262` |
| `build-macos` | pass | run `29795745504`, job `88526604288` |
| `build-windows` | pass | run `29795745504`, job `88526604279` |
| `license-gate` | pass | run `29795745512`, job `88526604647` |
| `dco` | pass | run `29795745512`, job `88526604677` |
| `link-check` | pass | run `29795745512`, job `88526604670` |

Exact final combined `main@621a4a2` also passed all seven checks: CI run
`29796308826` (`project-verify` `88528245797`, Ubuntu `88528245838`, macOS
`88528245839`, Windows `88528245814`) and Governance run `29796308835`
(`license-gate` `88528245890`, DCO `88528245808`, link-check `88528245807`).

## Scope, residual risk, and rollback

The production change remains confined to the Windows endpoint's overlapped
I/O lifecycle; the regression remains Windows-only. Server ownership,
protocol/authentication, DACL, same-session, remote-client rejection, Provider,
SQLite and non-Windows endpoints are unchanged.

Hosted Windows evidence closes this bug but is not a long soak or release
certification. To roll back, use reviewed revert commits for the rebase-landed
implementation and verification commits; do not reset protected `main`. A
rollback restores the prior availability defect and therefore requires an
explicit Windows disable/fallback decision rather than a silent downgrade.

## Handoff

**Target**: `windows-named-pipe-close-flake`
**Completed**: `ship`
**Status**: `SHIPPED`
**Summary**: `Merged the completion-tracked Windows Named Pipe repair through protected PR #26 and verified all seven checks on both the final PR head and final combined remote main.`
**Commit/Release**: `PR #26 head 3350c824; landed implementation 0891bdb; PR #26 integration main 8e15dd1; final combined main 621a4a2; no tag, release, publish, or deploy.`
**Tests**: `Independent Windows-native bug verification, final PR CI/Governance 29795745504 and 29795745512, exact-final-main CI/Governance 29796308826 and 29796308835, plus local Go/race/vet/workflow/CI/scaffold evidence — all required checks pass.`
**Blockers**: `none for this bug ship`

### Next Step

`None for this bug; broader Windows release acceptance remains under the existing product release gates.`
