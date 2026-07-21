# Remote Ship receipt: Codex explicit multi-account selector

## Verdict

`INTEGRATED_ON_REMOTE_MAIN`

The selector remains workflow status `SHIPPED`. Its verified source,
documentation and local Ship history are integrated on the protected remote
`main` branch. This receipt does not create a tag, package, release,
publication or deployment claim.

## Authorization and classification

The operator explicitly authorized the feature-branch push, protected pull
request, rebase merge to `main`, post-merge reconciliation, and this durable
receipt. No tag, package publication, release or deployment was authorized or
performed.

- Owner: `project-system` (remote integration governance and dashboard state).
- Secondary impact: `provider`; the already accepted `security` boundary is
  preserved.
- Reconciliation branch:
  `codex/project-system/codex-selector-remote-receipt`.
- Product-code changes in this reconciliation: none.

## Git and pull request receipt

- Pull request: [#22](https://github.com/jinlong17/multi-agent-desk/pull/22)
- Base before merge: `96ba36de0dababf0c74817885efcfaa74ab01877`
- Original pull-request head:
  `cc17c9275c41cb90d30b7c34025cbe20b391a6f0`
- Merge method: rebase merge.
- Selector source-integration `main` SHA:
  `ad420cd62683b4628906ad8c74c95274ab3e5d3d`
- Merged at: `2026-07-21T00:00:28Z` (`2026-07-20 17:00:28 PDT`).

The protected merge replayed the 32 signed feature-branch commits onto the
then-current `main`; the integrated tip is a one-parent commit. A direct tree
comparison between the original PR head and `main@ad420cd` is empty, so the
rebase changed commit identities but not the reviewed source content.

## Protected pull-request checks

All seven checks required by `main` protection passed on original PR head
`cc17c9275c41cb90d30b7c34025cbe20b391a6f0`:

| Required check | Result | Receipt |
|---|---|---|
| `project-verify` | pass | CI run `29788780639`, job `88505865044` |
| `build-ubuntu` | pass | CI run `29788780639`, job `88505865090` |
| `build-macos` | pass | CI run `29788780639`, job `88505865005` |
| `build-windows` | pass | CI run `29788780639`, job `88505865029` |
| `license-gate` | pass | Governance run `29788780641`, job `88505865016` |
| `dco` | pass | Governance run `29788780641`, job `88505865006` |
| `link-check` | pass | Governance run `29788780641`, job `88505865072` |

## Post-merge exact-main audit

The automatic push workflows at exact selector source-integration commit
`ad420cd62683b4628906ad8c74c95274ab3e5d3d` also passed all seven checks. PR
green was not substituted for final-main evidence:

| Required check | Result | Receipt |
|---|---|---|
| `project-verify` | pass | CI run `29788998523`, job `88506516237` |
| `build-ubuntu` | pass | CI run `29788998523`, job `88506516303` |
| `build-macos` | pass | CI run `29788998523`, job `88506516244` |
| `build-windows` | pass | CI run `29788998523`, job `88506516274` |
| `license-gate` | pass | Governance run `29788998525`, job `88506516234` |
| `dco` | pass | Governance run `29788998525`, job `88506516201` |
| `link-check` | pass | Governance run `29788998525`, job `88506516214` |

## Reconciliation verification

- Workflow generation and verification passed: agents `10`, skills `3`, docs
  `17`, edges `20`, statuses `15`.
- Dashboard generation and static verification passed on the dedicated
  reconciliation branch with exact `SHIPPED` focus binding: phases `9`,
  agents `10`, skills `3`.
- `project:verify` passed through the repository's fixed Node 24/pnpm runtime.
- Actions/CODEOWNERS and positive/negative governance fixtures passed:
  required checks `7`, Actions uses `15`, owner `@jinlong17`.
- Local links passed across `280` Markdown files; licenses passed for `5` pnpm
  groups and `418` Cargo packages.
- Scaffold layout and Go-format checks passed: directories `27`, files `49`,
  modules `7`, Go files `90`.
- JSON parse, exact three-file scope, working-tree status and
  `git diff --check` passed.

The first plain `npm` invocation did not start because the ambient shell did
not expose `npm`; no project test ran or failed. The same scripts passed
unchanged after using the repository's established fixed Node/pnpm entrypoint
with its npm-compatible wrapper.

## Compatibility and residual risk

- Stable live selector support remains exact Linux `amd64` Codex CLI
  `0.144.2` only.
- macOS remains `schema_compatible_identity_acceptance_pending`; Windows and
  other platforms remain `provider_platform_unsupported` for the selector.
- Operator alias confirmation is explicit internal target attestation, not
  automated upstream Provider identity proof.
- Automatic Account rotation, default fallback, quota bypass and mid-Session
  credential switching remain excluded.
- Provider-readable runtime plaintext, same-user/root/Provider/browser
  compromise, already-copied credentials, upstream semantic drift and the
  accepted non-post-quantum SSH KEX residual risk remain unchanged.
- Packaging, tagging, release publication and deployment are not claimed.

## Rollback

Do not rewrite or reset protected `main`. If the selector must be withdrawn,
use reviewed revert commits over the rebase-landed selector range and disable
the exact `0.144.2` compatibility row to block new starts while leaving
existing Sessions pinned. Migration 7 is forward-only: retain Vault items,
Session history and Usage evidence, and do not attempt an in-place schema
downgrade. Revert this documentation-only reconciliation separately if its
record is incorrect; it has no runtime rollback effect.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `ship`
**Status**: `SHIPPED`
**Summary**: `The operator-authorized feature branch was integrated by protected rebase PR #22, and all seven required checks passed both on the original PR head and on exact remote main ad420cd; this receipt and dashboard reconciliation preserve the narrow Linux 0.144.2 support boundary.`
**Commit/Release**: `PR #22 head cc17c927; selector source-integration main ad420cd; no tag, package, release or deployment`
**Tests**: `PR CI/Governance 29788780639 and 29788780641; exact-main CI/Governance 29788998523 and 29788998525; workflow/dashboard/project, static governance, links, licenses, layout, Go format and diff integrity — all pass`
**Blockers**: `none for selector remote integration; macOS identity acceptance, Windows selector support, broader versions, packaging, release and deployment remain outside this Ship claim.`

### Next Step

`None for the selector integration; subsequent product phases require their own reviewed plan and human gates.`
