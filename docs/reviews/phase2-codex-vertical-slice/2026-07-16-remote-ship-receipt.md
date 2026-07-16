# Phase 2 remote integration receipt

## Verdict

`INTEGRATED_ON_REMOTE_MAIN`

Phase 2 remains workflow status `SHIPPED`. Its source and documentation were
integrated through the protected `main` branch; this receipt does not create a
release, package, tag, publication, or deployment claim.

## Git and pull request receipt

- Pull request: [#19](https://github.com/jinlong17/multi-agent-desk/pull/19)
- Base before merge: `cb93c02a535c46d3e8a5873bcc7b7ba7bccdac82`
- Final pull-request head: `29837deded44cbc33e02a3fcd2f60c13402d13eb`
- Squash merge on remote `main`: `250bf57f2ec21beb5f02e03b58f030b9d67e5ff4`
- Merged at: `2026-07-16T19:10:30Z`

## Protected checks

All seven checks required by `main` protection passed on the final pull-request
head:

| Required check | Result | Receipt |
|---|---|---|
| `project-verify` | pass | CI run `29526495324`, job `87716108043` |
| `build-ubuntu` | pass | CI run `29526495324`, job `87716108154` |
| `build-macos` | pass | CI run `29526495324`, job `87716108074` |
| `build-windows` | pass | CI run `29526495324`, job `87716108057` |
| `license-gate` | pass | Governance run `29526495234`, job `87716107792` |
| `dco` | pass | Governance run `29526495234`, job `87716107751` |
| `link-check` | pass | Governance run `29526495234`, job `87716107822` |

The first CI attempt (`29526029227`) is retained as failed evidence. It exposed
a Windows-only test assumption about environment-variable key casing. The
production safe proxy value was present and no secret was inherited. Commit
`29837de` made the assertion case-insensitive for allowlisted key names while
retaining exact value and secret non-inheritance checks; the mandatory fresh
seven-check run above then passed.

## Post-merge final-main audit

The automatic Governance push run `29526956439` at exact squash commit
`250bf57` failed only `link-check`: six historical Claude documentation links
redirected outside the documentation site and returned HTTP 404. The PR link
check had passed minutes earlier, so the final-main run is retained as a real
failure rather than overwritten by the pull-request result. The receipt branch
removes the dead live-link dependency, points to the stable official Claude
Code upstream entry, retains the exact claims as dated Spike evidence, and
requires current CLI/documentation revalidation before Phase 3.

The companion final-main CI push run `29526955379` passed `project-verify` and
the Ubuntu, macOS, and Windows builds. A fresh local receipt-branch audit also
passed full Go tests, vet, full race tests, all three command builds,
workflow/dashboard generation and verification, static governance, local links,
licenses, the stable upstream HTTP probe, and diff integrity.

Receipt PR [#20](https://github.com/jinlong17/multi-agent-desk/pull/20) then
passed all seven required checks at head `48ae9fa` and squash-merged as
`390f4c4387f20c1d4fb32c9c2b529b2c4f632223`. Its exact final-main push runs
closed the audit:

- CI `29527674752`: `project-verify`, Ubuntu, macOS, and Windows passed.
- Governance `29527674854`: DCO, license, and external-link checks passed.

The earlier failed Governance run remains in history as evidence; it is closed,
not erased. Phase 2 remote source integration is complete.

## Preserved concurrent work

The dirty user-guide, old Phase 1 governance, and multi-account worktrees were
not staged, reset, deleted, or absorbed into PR #19. In particular, the
multi-account P1 migration still collides with Phase 2's `0004`/`0005`
migrations and must be reconciled onto this final-main schema before it can be
considered for integration.

## Remaining boundaries

- Linux x86_64 Codex CLI `0.144.2` is the only real credentialed Phase 2
  vertical-slice claim.
- macOS arm64 `0.144.2` remains schema/handshake smoke only.
- Windows remains build/protocol/native-CI evidence only; real Windows Codex is
  unsupported.
- Packaging, tagging, release publication, deployment, Control Plane, Web
  remote terminal, Desktop product completion, and later Provider phases are
  not claimed by this integration.
