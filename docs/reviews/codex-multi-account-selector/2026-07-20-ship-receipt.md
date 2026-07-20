# Ship receipt: Codex explicit multi-account selector

## Status

`BLOCKED`

## Authorized action

The operator explicitly requested `ship`. Under the repository's local Ship
contract, this authorized final local scope, branch, remote, diff, untracked,
documentation, version, rollback, security, test, license, DCO, dashboard and
receipt checks. It did not authorize push, pull request creation, merge,
history rewrite, tag, package publication, release, or deployment.

## Local result

- Branch: `codex/provider/codex-multi-account-selector`
- Reviewed head: `370aa244a9ebb478430c10c9d8e20a1d14fe0a3c`
- Remote baseline: `origin/main` at
  `96ba36de0dababf0c74817885efcfaa74ab01877`
- Range: 28 local commits ahead and zero behind the fetched remote baseline.
- Worktree was clean before the Ship receipt transition; no untracked product
  files were present.
- Provider Gate is resolved for exact Linux `amd64` Codex CLI `0.144.2`.
- Security Review is `ACCEPTED`; Security Gate is resolved.
- No push, PR, merge, history rewrite, tag, package, release, publication or
  deployment was performed.

The local Ship cannot be marked `SHIPPED` because the exact repository DCO
gate fails for nine commits and the exact base-to-head diff check reports 20
trailing-whitespace lines in five Markdown files. The DCO correction requires
rewriting local commit messages to add valid `Signed-off-by` trailers; Ship
authorization alone does not authorize that history rewrite.

## Blocking evidence

`node scripts/ci/verify-dco.mjs --base origin/main --head HEAD` reports missing
or malformed DCO trailers for:

- `4c00aec38906711c2888eef055783f3c36906445`
- `56571df4f918ced8c38f483b89b4e25a0e4b69e7`
- `1219b54b622264a08c8b5c26bfacb0acdaeaa735`
- `25328b148a440cb57824ec4b8bd92eba33f2b44c`
- `4ba0090bc520fe05946f11710d6428b18da1f06c`
- `ec73d1cff73c8d9687022c37d0a192c233cccf74`
- `ad67bdae499abfe8c09e7d1397898f56ea6da565`
- `1d6f4b68a8da60021ad26f60b78d1bf652c4f678`
- `370aa244a9ebb478430c10c9d8e20a1d14fe0a3c`

`git diff --check origin/main...HEAD` reports 20 trailing-whitespace lines
across:

- `docs/reviews/claude-api-key-provider/2026-07-16-feature-brief.md`
- `docs/reviews/codex-multi-account-selector/2026-07-16-feature-brief.md`
- `docs/spikes/claude-distinct-accounts/2026-07-16-policy-and-isolation-spike.md`
- `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes-spike.md`
- `docs/workflow/features/codex-multi-account-selector/api.md`

The whitespace cleanup can be added non-destructively, but the nine DCO
failures cannot be repaired by a later commit because the governance job checks
each commit in the PR range.

## Passing evidence

- `go test -count=1 ./...` — PASS.
- `go vet ./...` — PASS.
- `go test -race -count=1 ./...` — PASS.
- Darwin arm64, Linux amd64 and Windows amd64 `multidesk` builds — PASS.
- Build SHA-256 values remain `2540b1de...b0d70`,
  `6929ff46...74804`, and `0868ae2e...9becc` respectively.
- Web TypeScript checks/build — PASS.
- Desktop Rust format/check — PASS.
- Workflow verifier — PASS: agents 10, skills 3, docs 17, edges 20,
  statuses 15.
- Actions, CODEOWNERS, positive/negative gate fixtures, 277 local Markdown
  links, pnpm/Cargo licenses, scaffold layout and Go formatting — PASS.
- Exact feature security review — `ACCEPTED`, with P0/P1 none.
- Version posture remains pre-release `0.0.0`; this local Ship creates no
  package or release version and makes no broader platform claim.

## Compatibility and residual risk

- Live support remains exact Linux `amd64` Codex CLI `0.144.2` only.
- macOS selector identity acceptance remains pending; Windows remains
  unsupported for the selector despite successful compilation.
- Operator alias confirmation is an internal target attestation, not an
  upstream identity proof.
- Provider-readable runtime plaintext, same-user/root/Provider/browser
  compromise, already-copied credentials, upstream drift and the Linux host's
  non-post-quantum SSH KEX remain explicit residual risks.
- No automatic account rotation, default fallback or mid-Session identity
  switching is accepted.

## Rollback

No external or release state was created, so no remote rollback is required.
Keep the current branch intact until the operator decides whether to authorize
local history rewriting. If a later corrected Ship is reverted, use reviewed
revert commits rather than resetting shared history. Migration 7 is
forward-only: disable the exact selector capability to stop new starts and
retain existing Vault items, Session history and Usage evidence; do not attempt
an in-place schema downgrade.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `ship`
**Status**: `BLOCKED`
**Summary**: `All product, security, platform-build, UI, license and project checks pass, but local Ship cannot be completed because nine commits fail the exact DCO range and five Markdown files fail exact diff whitespace validation; no remote or release action was performed.`
**Commit/Release**: `reviewed product/security head 370aa24; the signed blocked-receipt commit contains this file; no successful Ship commit, push, PR, merge, history rewrite, tag, release or deployment`
**Tests**: `full Go/vet/race; Darwin/Linux/Windows builds; Web/Desktop; workflow/CI/contracts/links/licenses/layout/format pass; DCO and exact range diff fail as recorded`
**Blockers**: `operator authorization is required to rewrite nine local commit messages with DCO sign-off; then clean 20 Markdown whitespace lines, rerun the complete Ship matrix, and create the signed local Ship receipt commit.`

### Next Step

`Authorize local history rewrite for DCO correction, then rerun ship.`
