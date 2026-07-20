# Ship receipt v2: Codex explicit multi-account selector

## Status

`SHIPPED`

## Authorized action

The operator explicitly requested local `ship`, then explicitly authorized
rewriting the current local branch history to add DCO sign-off to nine commits,
cleaning the recorded Markdown trailing whitespace, and rerunning Ship.

This authorization covered the local history correction, exact local staging,
signed correction and receipt commits, the complete Ship verification matrix,
dashboard reconciliation, and the `SHIPPED` state transition. It did not
authorize push, pull request creation, merge, tag, package publication,
release, or deployment. None of those remote or release actions occurred.

## Local result

- Branch: `codex/provider/codex-multi-account-selector`
- Remote baseline after fetch: `origin/main` at
  `96ba36de0dababf0c74817885efcfaa74ab01877`
- Rewritten product/security head: `3afef484f90abb289d9b692f63c075f526965864`
- Whitespace correction: `bdc0aa4faf8c4c33c47b8d400e26992273779f19`
- Final signed Ship receipt/governance commit: the commit containing this file.
- Exact DCO range after the whitespace correction: 30 commits, zero
  exceptions; the signed receipt commit raises the final clean range to 31.
- Exact base-to-head diff check: clean.
- Untracked files: none.
- Version posture: repository pre-release `0.0.0`; no package/release version
  was changed or published.
- Provider Gate: resolved for exact Linux `amd64` Codex CLI `0.144.2`.
- Security Gate: resolved / `ACCEPTED`; P0/P1 none.

The earlier `2026-07-20-ship-receipt.md` remains as the durable first-attempt
`BLOCKED` record. This v2 receipt records its operator-authorized correction
and does not erase the prior failure.

## DCO correction mapping

The nine originally unsigned commits were recreated with unchanged trees and
subjects plus a valid `Signed-off-by: jinlong17 <jinlong.li1@oppo.com>` trailer:

| Original | Corrected | Subject |
|---|---|---|
| `4c00aec` | `25dae0b` | `feat(provider): complete selector P2 runtime` |
| `56571df` | `ebcab9d` | `docs(provider): verify selector P2 runtime` |
| `1219b54` | `04b7f31` | `docs(provider): enter selector P3 phase` |
| `25328b1` | `b1995f2` | `feat(provider): close selector P3 platform gates` |
| `4ba0090` | `d8fae97` | `docs(provider): record selector P3 verification blocker` |
| `ec73d1c` | `b14dce5` | `fix(provider): gate selector enrollment platforms` |
| `ad67bda` | `6ebd0c4` | `docs(provider): verify selector P3 correction` |
| `1d6f4b6` | `52196c8` | `docs(provider): fix selector security handoff state` |
| `370aa24` | `3afef48` | `docs(security): accept selector security gate` |

The already signed first-attempt blocker receipt was replayed as `9e2c889`
without adding a duplicate trailer. No remote ref pointed to the feature branch
before the rewrite.

## Whitespace correction

Signed commit `bdc0aa4` removed the exact 20 trailing-whitespace instances from
five Markdown files without changing product code, Provider behavior,
compatibility claims, tests, secrets, or dependencies. Local link validation
passes with 278 Markdown files, and `git diff --check origin/main...HEAD`
returns no finding.

## Verification evidence

- `go test -count=1 ./...` — PASS.
- `go vet ./...` — PASS.
- `go test -race -count=1 ./...` — PASS.
- Darwin arm64, Linux amd64 and Windows amd64 builds — PASS.
- Build SHA-256 values:
  - Darwin: `2540b1de8d5310e2b23e074daff4445f809628421e04c8bc9f9c9a6cbeab0d70`
  - Linux: `6929ff46ccbb0f2627137e4e5c45fb5fbeb3df7e576de6946b1c62e3b0574804`
  - Windows: `0868ae2eb189541b91e216e95fc9f854fafeeaa6c26e2b4c4e5726d52119becc`
- Web TypeScript checks/build — PASS.
- Desktop Rust format/check — PASS.
- Workflow generation/verification and dashboard generation/verification —
  PASS: agents 10, skills 3, docs 17, edges 20, statuses 15, phases 9.
- Actions/CODEOWNERS and positive/negative governance fixtures — PASS.
- Local links — PASS: 278 Markdown files.
- Licenses — PASS: 5 pnpm groups and 418 Cargo packages.
- Scaffold layout and Go formatting — PASS: 27 directories, 49 files,
  7 modules, 90 Go files.
- DCO — PASS: 30 commits and zero exceptions before the signed final receipt.
- Exact range diff, untracked and working-tree checks — PASS.
- Retained exact Linux A/B live selector, Usage, scoped logout/re-login and
  zero-materialized-auth cleanup evidence — PASS and unchanged by the
  documentation-only correction.

## Compatibility and residual risk

- Stable live selector support remains exact Linux `amd64` Codex CLI `0.144.2`
  only.
- macOS remains `schema_compatible_identity_acceptance_pending`; Windows and
  other platforms remain `provider_platform_unsupported` for the selector.
- Operator alias confirmation is an explicit internal target attestation, not
  an upstream Provider identity proof.
- No automatic Account rotation, default fallback, quota bypass, or
  mid-Session credential switch is accepted.
- Provider-readable runtime plaintext, same-user/root/Provider/browser
  compromise, already-copied credentials, upstream semantic drift and the
  Linux host's current non-post-quantum SSH KEX remain explicit residual risks.
- Local `SHIPPED` means the verified feature and its signed receipt are durable
  on this work branch. It does not mean remote integration, packaging,
  production release or deployment has occurred.

## Rollback

No remote or release state exists, so no external rollback is required. Use
reviewed revert commits on the corrected local history rather than resetting or
restoring the unsigned commits. Disable the exact `0.144.2` selector
compatibility row to prevent new starts while leaving existing Sessions pinned.
Migration 7 is forward-only: retain Vault items, Session history and Usage
evidence, and do not attempt an in-place schema downgrade.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `ship`
**Status**: `SHIPPED`
**Summary**: `The operator-authorized local history correction added valid DCO sign-off to all nine previously unsigned commits, removed all 20 recorded Markdown whitespace findings, reran the complete Ship matrix successfully, reconciled the dashboard, and shipped the exact Linux 0.144.2 selector locally.`
**Commit/Release**: `corrected product/security head 3afef48; whitespace correction bdc0aa4; signed receipt commit contains this file; no push, PR, merge, tag, package, release or deployment`
**Tests**: `full Go/vet/race; Darwin/Linux/Windows builds; Web/Desktop; workflow/dashboard; Actions/CODEOWNERS/fixtures/links/licenses/layout/format; DCO and exact range diff — all pass`
**Blockers**: `none for local Ship; any push, PR, merge, tag, release or deployment requires separate explicit authorization.`

### Next Step

`None for local Ship; optional remote integration requires separate explicit authorization.`
