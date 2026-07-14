# Feature verification: Phase 0 CI governance P2 DCO anchor correction

## Verdict

`READY_TO_SHIP`

The post-squash DCO ancestor failure is reproduced, correctly diagnosed, and
fixed. The correction PR has all seven required checks successful, including
the live DCO job that previously failed on `main`.

## Scope

- Target: `phase0-ci-governance`, corrective P2 verification.
- Owner: `project-system`; secondary impacts: none.
- Base: signed `main` commit
  `ba6909449de53db604eaa25c8d7b1f9726446503`.
- Verified correction head:
  `1ad1c894e172e417d378a70987192b56f9e39c71`.
- Pull request: [#2](https://github.com/jinlong17/multi-agent-desk/pull/2).

## Reproduction and cause

Main Governance run
[`29360235017`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29360235017)
failed only `dco`; `license-gate` and `link-check` succeeded. The failed command
proved that feature-only policy commit `ff4c2ad` was not an ancestor of the
squash-integrated main commit `ba69094`. The merge commit itself contains a
valid `Signed-off-by` trailer, so missing sign-off was not the cause.

## Correction verification

- `policy_effective_commit` now equals signed `main` commit `ba69094`.
- `git merge-base --is-ancestor ba69094 HEAD` succeeds.
- The exception list is empty; none of the feature-only SHAs are represented as
  ancestors or grandfathered main commits.
- Local live verification passes with `commits=1 grandfathered=0`.
- Workflow verification passes with 10 agents, 3 skills, 17 documents, 20
  edges, and 15 statuses.
- Dashboard verification passes on a clean tree with 9 phases, 10 agents, and
  3 skills.
- Positive/negative DCO and other gate fixtures pass.
- License inventory passes for 5 pnpm groups and 418 Cargo packages.
- Diff checks pass and the worktree is clean before verdict write.

## Remote checks

Current correction head has:

| Check | Result |
|---|---|
| `project-verify` | success |
| `build-ubuntu` | success |
| `build-macos` | success |
| `build-windows` | success |
| `license-gate` | success |
| `dco` | success |
| `link-check` | success |

CI run:
[`29360539860`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29360539860).
Governance run:
[`29360540068`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29360540068).
PR #2 is `OPEN`, `MERGEABLE`, and `CLEAN`, with no review required under the
operator-approved single-account policy.

## Findings

No blocking finding remains. The failed main run is retained as negative
evidence; it is not converted into pass. Ship must merge this exact correction
head, then require a new `main` push Governance run to pass before recording
`SHIPPED`.
