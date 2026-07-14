# Phase 0 P2 remote governance receipt

> Status: `BLOCKED`
> Repository: `jinlong17/multi-agent-desk`
> Branch: `codex/project-system/phase0-ci-governance`
> Pull request: [#1](https://github.com/jinlong17/multi-agent-desk/pull/1)
> Last updated: `2026-07-14 01:11 -0700`

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
| Merge | not performed | pending operator-confirmed UI action |

## Actions evidence

| Head / run | Observed result | Conclusion |
|---|---|---|
| `521c8fa`; CI [`29314251246`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314251246), Governance [`29314251259`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314251259) | `project-verify`, Ubuntu, macOS, `license-gate`, `dco`, and `link-check` passed; Windows failed because checkout line endings made `gofmt` fail | retained negative platform evidence |
| `521c8fa`; CI [`29314803988`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314803988), Governance [`29314804058`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29314804058) | Governance and non-Windows CI passed; Windows advanced past formatting and failed because Tauri required `icons/icon.ico` | retained second negative platform evidence |
| `68117886c43bdf9b622d3c95d499e532d740ddc7` | GPL-3.0-only fixture was pushed; local gate failed specifically with `pnpm group GPL-3.0-only: disallowed license expression GPL-3.0-only` | remote run ID/log conclusion pending authenticated readback |
| `22e2240fa16c4066a276c5469c8c0255ed7e64f8` | GPL fixture removed; Windows icon and LF normalization retained; remote head proven by Git | final seven-check run IDs and conclusions pending authenticated readback |

No clean-head success is inferred from the merge ref or from local checks.

## Repository settings readback

Last authenticated UI readback before the browser connection failed:

```json
{
  "actions": {
    "require_full_length_action_sha": true,
    "default_workflow_token": "read contents/packages",
    "read_write_token": false,
    "actions_can_approve_pull_requests": false,
    "fork_pull_request_workflows": {
      "run": true,
      "send_write_tokens": false,
      "send_secrets": false,
      "require_approval": true
    },
    "repository_actions_reusable_externally": false
  },
  "main_branch_protection": null
}
```

- The last branch-protection page readback showed no rule configured.
- The exact required `main` rule is therefore not applied or proven.
- The pre-mutation value for `require_full_length_action_sha` was not
  persisted. It remains `unknown`; rollback parity cannot be claimed.
- Final timestamped readback of Actions settings is still required after the
  authenticated browser session is restored.

## Required rule still pending

`main` must require the exact checks `project-verify`, `build-ubuntu`,
`build-macos`, `build-windows`, `license-gate`, `dco`, and `link-check`, plus
strict up-to-date branches, one approval, CODEOWNER review, stale-review
dismissal, conversation resolution, linear history, admin enforcement, and
disabled force-push/deletion. No weaker rule is accepted.

## Reproducible blocker

Chrome, the ChatGPT Chrome Extension, and the native-host manifest all pass
their installation checks, but two initial browser-client `openTabs()` attempts
timed out. At `2026-07-14 01:11 -0700`, the operator authorized the documented
recovery: a fresh Chrome window was opened for the selected profile, the client
waited two seconds, reconnected once, and retried `openTabs()`. That sole retry
also timed out. The Chrome troubleshooting contract now requires reinstalling
the Chrome plugin from the ChatGPT/Codex plugin UI; it forbids fallback through
scripts, cookies, browser storage, or another automation surface.

Without an authenticated readback, the final clean/GPL run results and
repository rule cannot be proved or changed safely.

Clearing role: the operator reinstalls the Chrome plugin from the ChatGPT/Codex
plugin UI and confirms it is ready (or provides another authenticated GitHub
API/CLI surface). Feature-build then completes readback, performs separately
confirmed merge/settings actions, updates this receipt, and returns
`READY_FOR_VERIFY` only if every P2 criterion is proven.
