# Phase 0 P2 remote governance receipt

> Status: `BLOCKED`
> Repository: `jinlong17/multi-agent-desk`
> Branch: `codex/project-system/phase0-ci-governance`
> Pull request: [#1](https://github.com/jinlong17/multi-agent-desk/pull/1)
> Last updated: `2026-07-14 02:00 -0700`

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
| Merge | not performed | prohibited while the required protection contract is unavailable |

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

Authenticated API readback at `2026-07-14 02:00 -0700`:

```json
{
  "repository": {
    "private": true,
    "visibility": "private",
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
  "main_branch_protection": "HTTP 403: upgrade to GitHub Pro or make this repository public",
  "repository_rulesets": "HTTP 403: upgrade to GitHub Pro or make this repository public"
}
```

- Actions defaults meet the required read-only token and no-approval contract.
- Only the read-only CI and Governance workflows exist; no release or deployment
  workflow exists.
- The fork-PR approval endpoint returns 422 because it is not applicable to
  private repositories.
- Both branch protection and repository rulesets are unavailable for this
  private repository on the current GitHub plan. The exact required `main`
  rule therefore cannot be applied or read back under the current plan/state.
- The pre-mutation value for `require_full_length_action_sha` was not
  persisted. It remains `unknown`; rollback parity cannot be claimed.

## Required rule blocked by GitHub plan

`main` must require the exact checks `project-verify`, `build-ubuntu`,
`build-macos`, `build-windows`, `license-gate`, `dco`, and `link-check`, plus
strict up-to-date branches, one approval, CODEOWNER review, stale-review
dismissal, conversation resolution, linear history, admin enforcement, and
disabled force-push/deletion. No weaker rule is accepted.

Authenticated calls to both `/branches/main/protection` and `/rulesets` return
HTTP 403 with GitHub's explicit remedy: upgrade to GitHub Pro or make the
repository public. Per the approved P2 design, this is an operator-choice gate;
feature-build must not weaken the rule, change visibility, buy a plan, or merge
without that choice.

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

Authenticated `gh` access now clears the browser-readback blocker and proves
the clean/GPL results plus Actions settings. The remaining blocker is solely
the GitHub plan/repository-visibility gate above.

Clearing role: the operator chooses either to make the repository public or to
enable a GitHub plan that supports protection for this private repository.
Feature-build can then apply and read back the exact rule. Merge remains a
separate human gate and must not occur until the protection criterion is
proven.
