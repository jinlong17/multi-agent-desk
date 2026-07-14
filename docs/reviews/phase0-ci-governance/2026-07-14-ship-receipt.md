# Ship receipt: Phase 0 CI and remote governance

## Status

`SHIPPED`

## Authorization

On 2026-07-14 the operator explicitly made single-account, no-review, direct
`main` completion the highest priority. This authorized the review-policy
exception, branch pushes, PR merges, risk acceptance, and Phase 0 completion.

## Delivered integration

- PR [#1](https://github.com/jinlong17/multi-agent-desk/pull/1) merged at
  `2026-07-14T19:01:44Z` as signed squash commit
  `ba6909449de53db604eaa25c8d7b1f9726446503` after GitHub rejected rebase.
- The squash tree exactly matched verified PR head `b70e258`.
- Main Governance run `29360235017` correctly failed because the original DCO
  policy SHA was absent from squash history. That failure remains retained.
- PR [#2](https://github.com/jinlong17/multi-agent-desk/pull/2) corrected the
  DCO anchor and merged by rebase at `2026-07-14T19:22:17Z` as three signed,
  linear commits: `9ba99f1`, `165fa75`, and `750b435`.
- Final shipped `main` head before this receipt is
  `750b4352da815e83b854855dce84f2257e0f6f5a`.

## Final main evidence

- Governance run
  [`29361556828`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29361556828)
  succeeded on `750b435`: `dco`, `license-gate`, and `link-check` all passed.
- CI run
  [`29361556914`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29361556914)
  succeeded on `750b435`: `project-verify`, Ubuntu, macOS, and Windows all
  passed.
- The DCO policy effective commit is signed `ba69094`; exceptions are empty;
  all four current main commits include valid `Signed-off-by` trailers.
- Main protection still requires the exact seven strict checks, enforces rules
  for admins, requires conversation resolution and linear history, and forbids
  force pushes and deletion. The operator-approved single-account subset is
  zero approvals and no mandatory CODEOWNER review.
- Actions defaults remain read-only and Actions cannot approve PR reviews.

## Scope and release posture

This ships the Phase 0 repository/governance baseline. It does not create a
product version, tag, GitHub Release, deployment, or package publication.
Provider and Security Gates remain deferred to Phase 0.5 evidence as planned.

## Rollback

Rollback is a new, reviewed revert sequence on protected `main`: revert
`750b435`, `165fa75`, `9ba99f1`, then `ba69094` in reverse integration order,
and require the same seven checks. Force push, branch deletion, settings
weakening, and history rewriting are not rollback mechanisms.
