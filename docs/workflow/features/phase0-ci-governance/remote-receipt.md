# Phase 0 P2 remote governance receipt

> Status: `SHIPPED`
> Repository: `jinlong17/multi-agent-desk`
> Branch: `codex/project-system/phase0-ci-governance`
> Pull request: [#1](https://github.com/jinlong17/multi-agent-desk/pull/1)
> Last updated: `2026-07-14 12:26 -0700`

This is a sanitized, append-only-oriented P2 evidence record. It contains no
tokens, cookies, authorization headers, secrets, or environment contents.
Fields marked `pending` or `unknown` are not accepted as pass.

## Remote identity

| Field | Evidence | Conclusion |
|---|---|---|
| Feature head | `22e2240fa16c4066a276c5469c8c0255ed7e64f8`; `git ls-remote` matches `refs/heads/codex/project-system/phase0-ci-governance` and `refs/pull/1/head` | proven |
| Base head | `refs/heads/main` = `928a2909c738406bbb0d089481d03dd0b1f927d2` | proven |
| Generated merge ref | `refs/pull/1/merge` = `b82eb7710cc1ced16c48fab699ce733848c93262`, parents base + feature head | PR has a generated merge result; this does not prove checks |
| Remote actor | `jinlong17` in the authenticated GitHub session | observed |
| Authenticated API | `gh api user` = `jinlong17`; repository permission = `ADMIN`; token scopes include `repo` and `workflow` | proven with a real API call |
| Local evidence branch | contains unpushed documentation receipts ahead of the remote PR head | retained locally because feature-build role does not push |
| Repository visibility | operator changed repository from private to public; authenticated readback returns `PUBLIC` | plan/visibility blocker cleared by operator |
| Merge | not performed | P2 stops at an unmerged test PR; merge remains a later ship gate |

## Actions evidence

| Head / run | Observed result | Conclusion |
|---|---|---|
| `521c8fa`; CI [`29314251246`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314251246), Governance [`29314251259`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314251259) | `project-verify`, Ubuntu, macOS, `license-gate`, `dco`, and `link-check` passed; Windows failed because checkout line endings made `gofmt` fail | retained negative platform evidence |
| `521c8fa`; CI [`29314803988`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314803988), Governance [`29314804058`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314804058) | Governance and non-Windows CI passed; Windows advanced past formatting and failed because Tauri required `icons/icon.ico` | retained second negative platform evidence |
| `0bce526260600d9976df5d4a0e7e90abdf029ab0`; Governance [`29315247924`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29315247924), CI [`29315247926`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29315247926) | GPL-3.0-only fixture from `6811788` remained present; `license-gate` failed with `Error: pnpm group GPL-3.0-only: disallowed license expression GPL-3.0-only` and exit code 1. `project-verify`, Ubuntu, macOS, DCO, and link passed; Windows was cancelled by the superseding recovery push | required GPL rejection proven; cancelled Windows is not represented as pass |
| `22e2240fa16c4066a276c5469c8c0255ed7e64f8`; Governance [`29315826964`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29315826964), CI [`29315826965`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29315826965) | `project-verify`, Ubuntu, macOS, Windows, `license-gate`, DCO, and link all completed with `success`; PR reports `MERGEABLE` / `CLEAN` | clean seven-check recovery proven for the remote PR head |

The conclusions above come from authenticated check-run and workflow-run API
readback, not from the merge ref or local checks.

## Repository settings readback

Authenticated API readback after operator visibility change and authorized rule
mutation at `2026-07-14 02:32 -0700`:

```json
{
  "repository": {
    "private": false,
    "visibility": "public",
    "viewer_permission": "ADMIN"
  },
  "actions": {
    "enabled": true,
    "allowed_actions": "all",
    "sha_pinning_required": true,
    "default_workflow_permissions": "read",
    "can_approve_pull_request_reviews": false,
    "active_workflows": ["CI", "Governance"]
  },
  "main_branch_protection": {
    "required_status_checks": {
      "strict": true,
      "contexts": [
        "project-verify",
        "build-ubuntu",
        "build-macos",
        "build-windows",
        "license-gate",
        "dco",
        "link-check"
      ]
    },
    "enforce_admins": true,
    "required_pull_request_reviews": {
      "dismiss_stale_reviews": true,
      "require_code_owner_reviews": true,
      "required_approving_review_count": 1,
      "require_last_push_approval": false
    },
    "restrictions": null,
    "required_linear_history": true,
    "required_conversation_resolution": true,
    "allow_force_pushes": false,
    "allow_deletions": false,
    "block_creations": false,
    "lock_branch": false,
    "allow_fork_syncing": false
  }
}
```

- Actions defaults meet the required read-only token and no-approval contract.
- Only the read-only CI and Governance workflows exist; no release or deployment
  workflow exists.
- The first mutation response and a separate authenticated GET returned the
  same protection subset shown above.
- No push restriction list is configured; all required review/check/admin and
  history safeguards are enforced independently of push restrictions.
- The pre-mutation value for `require_full_length_action_sha` was not
  persisted. It remains `unknown`; rollback parity cannot be claimed.

## Initial strict review rule applied and read back (superseded)

`main` must require the exact checks `project-verify`, `build-ubuntu`,
`build-macos`, `build-windows`, `license-gate`, `dco`, and `link-check`, plus
strict up-to-date branches, one approval, CODEOWNER review, stale-review
dismissal, conversation resolution, linear history, admin enforcement, and
disabled force-push/deletion. The authenticated readback matches every required
field and exact check name; no weaker rule was substituted.

## Operator-approved single-account rule

At `2026-07-14 11:48 -0700`, the operator explicitly stated that one account is
sufficient, no review is required, direct completion into `main` is allowed,
and this is the highest priority. The original writer therefore changed only
the pull-request review subset and immediately performed an independent GET.

```json
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "project-verify",
      "build-ubuntu",
      "build-macos",
      "build-windows",
      "license-gate",
      "dco",
      "link-check"
    ]
  },
  "enforce_admins": true,
  "required_pull_request_reviews": {
    "dismiss_stale_reviews": true,
    "require_code_owner_reviews": false,
    "required_approving_review_count": 0,
    "require_last_push_approval": false
  },
  "required_conversation_resolution": true,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false
}
```

The seven required checks and every non-review safeguard remained unchanged.
After the mutation, PR #1 changed from `BLOCKED` / `REVIEW_REQUIRED` to
`MERGEABLE` / `CLEAN` with no review decision. This resolves the single-owner
CODEOWNER deadlock without disabling CI, admin enforcement, conversation
resolution, linear history, or destructive-update protections.

## Reproducible blocker

Chrome, the ChatGPT Chrome Extension, and the native-host manifest all pass
their installation checks, but two initial browser-client `openTabs()` attempts
timed out. At `2026-07-14 01:11 -0700`, the operator authorized the documented
recovery: a fresh Chrome window was opened for the selected profile, the client
waited two seconds, reconnected once, and retried `openTabs()`. That sole retry
also timed out. The Chrome troubleshooting contract now requires reinstalling
the Chrome plugin from the ChatGPT/Codex plugin UI; it forbids fallback through
scripts, cookies, browser storage, or another automation surface.

The operator then reinstalled the plugin and enabled the extension. At
`2026-07-14 01:32 -0700`, post-reinstall diagnostics proved all four local
prerequisites again: Chrome `150.0.7871.115` was running; ChatGPT Chrome
Extension `1.2.27203.26575` was installed and enabled in the selected `Default`
profile; the expected native host `com.openai.codexextension` existed; and its
allowed origin exactly matched the extension. Browser setup still returned a
Chrome binding, but both controlled-tab listing and the single documented
two-second retry timed out. Therefore reinstall did not clear the authenticated
readback blocker, and no GitHub result or setting is inferred from local
installation health.

Authenticated `gh` access cleared the browser-readback blocker. The operator's
public-visibility change cleared the GitHub-plan blocker. The authorized
protection mutation and independent GET now satisfy the remaining P2 remote
configuration criterion.

## Historical post-protection PR state (superseded)

PR #1 retains seven successful checks and is structurally `MERGEABLE`, while
GitHub reports `BLOCKED` / `REVIEW_REQUIRED` because the newly enforced review
rule has no approval. The generated CODEOWNERS file currently assigns all paths
only to `@jinlong17`, who is also the PR author. This is not a P2 build failure:
it proves the rule is active, and P2 intentionally uses an unmerged test PR. It
is a real later ship gate; merge must not be attempted until an eligible
CODEOWNER approval path exists and the independent feature verifier accepts the
P2 evidence.

The operator-approved single-account rule above supersedes that merge-path
requirement. The historical blocked state is retained as evidence and is not
rewritten as a pass.

## Authorized merge and DCO follow-up

At `2026-07-14 12:01 -0700`, ship preflight proved final PR head `b70e258`, all
seven successful checks, exact single-account protection, and a clean worktree.
GitHub rejected the requested rebase merge with `This branch can't be rebased`.
The ship role then used the enabled squash strategy with a signed commit body.
PR #1 merged at `2026-07-14T19:01:44Z` as
`ba6909449de53db604eaa25c8d7b1f9726446503`; its tree is byte-for-byte equal to
the verified PR head, and its parent is the prior `main` head `928a290`.

The first `main` Governance run
[`29360235017`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29360235017)
retained `license-gate` and `link-check` success but failed `dco`. The exact
failure was not a missing trailer: live config still named feature-only policy
commit `ff4c2ad`, which is not an ancestor of the squashed `main` commit. The
original writer re-anchored the policy to signed `main` commit `ba69094` and
removed three exceptions whose original commits were also absent after the
squash. This failure remains failed until the correction passes both PR and
`main` push checks.

## Final ship completion

Correction PR #2 merged by rebase as signed commits `9ba99f1`, `165fa75`, and
`750b435`. On final main head `750b435`, Governance run `29361556828` completed
successfully for DCO, licenses, and links; CI run `29361556914` completed
successfully for project verification, Ubuntu, macOS, and Windows. Independent
protection and Actions readback remained exact. The Phase 0 CI/governance unit
is therefore shipped. No tag, release, deployment, or package publication was
performed.
