# Feature verification: Codex explicit multi-account selector P2

- Date: 2026-07-20
- Executor: `feature-verify / P2`
- Build commit: `4c00aec`
- Verdict: `VERIFIED`

## Scope inspected

The committed diff from `401ea2a..4c00aec` was inspected against the approved
P2 phase, P1 preview/confirmation authority, exact-Linux Provider decision,
Vault/materialization boundary, immutable Session tuple, public raw-ID denial,
Usage truthfulness, scoped logout/re-login, and P3 platform/Security gates.

The preview transaction remains the only application Session-creation path.
`StartReserved` reads that persisted Session, compares every tuple field, and
does not create a second row. It re-discovers and fingerprints the binary,
schema, and method set before materialization; concurrent exact starts are
coalesced, and a shared Credential runtime must match the reservation.

Public selector operations still call `ResolveProfile`; only explicit
administrative Profile operations call `ResolveProfileTarget`. Usage refresh
requires an explicit public selector, a confirmed healthy Credential, and an
active Account-bound runtime. Storage validates that a persisted
CredentialInstance belongs to the same Account, Device, and Provider.

## Independent command evidence

- `git show --check --stat 4c00aec` — pass; committed build receipt is clean.
- `go test -count=1 ./...` and `go vet ./...` — pass.
- `go test -race -count=1 ./...` — pass.
- Targeted reserved-start A/B isolation/drift/Usage, selector/auth isolation,
  and administrative/public resolver tests with `-race -count=10` — pass.
- Darwin arm64, Linux amd64, and Windows amd64 Go builds — pass with hashes
  `8192ea...7d354`, `1ac398...fd107`, and `581383...abe9`. These are compile
  artifacts only for macOS/Windows.
- pnpm `10.23.0`: Web TypeScript/build and Desktop Rust fmt/check — pass.
- Layout, Go format, Actions, CODEOWNERS, CI fixtures, local links, licenses,
  workflow, dashboard, project structural verification, and diff integrity —
  pass while the unit remained at `READY_FOR_VERIFY`.

## Independent live-evidence readback

The stopped Linux acceptance Device database was queried read-only after the
writer run:

- both final A/B Sessions are `exited`, have distinct Account,
  CredentialInstance, and Provider thread IDs;
- the two cited Usage rows map to those distinct Account/Credential tuples,
  report `0.144.2` and `available`, contain 64-character redacted hashes, and
  the hashes are distinct;
- both rows preserve the documented conservative
  `stale_at == observed_at` contract;
- the managed Home contains zero materialized `auth.json` files and the
  acceptance daemon is not running.

The SSH host still reports no post-quantum KEX. This is a disclosed environment
hardening item, not evidence of a product regression or a broader support
claim.

## Acceptance verdict

P2 is `VERIFIED`. No P2 finding remains. The exact Linux `0.144.2` selector,
runtime, Usage, active-logout denial, targeted B re-login, A isolation, and
cleanup contracts have direct automated and live evidence.

P3 remains mandatory before final readiness. It owns the explicit macOS
identity-acceptance-pending and Windows unsupported runtime gates, user and
compatibility documentation, dashboard reconciliation, full platform matrix,
and final Security Review. Cross-compilation in this verdict does not accept
either platform.

## Findings

None.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-verify / P2`
**Verdict**: `VERIFIED`
**Summary**: `The selector-bound exact Linux runtime, credential-bound Usage, scoped logout/re-login, and cleanup contracts pass independent code, race, platform-build, governance, and live-evidence verification.`
**Evidence**: `4c00aec; full Go/vet/race; targeted race x10; three-OS builds; Web/Desktop; CI/project governance; read-only Linux Session/Usage/materialization readback.`
**Findings**: `none`
**Blockers**: `none for P2; P3 platform gates and final Security Review remain planned dependencies`

### Next Step

Run `feature-build / P3` for `codex-multi-account-selector`.
