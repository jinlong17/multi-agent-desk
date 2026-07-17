# Feature verify v2 P1: Codex explicit multi-account selector

- Date: 2026-07-16
- Reviewed commits: `50794df`, `5899e73`, `c062738`
- Role: `feature-verify / P1 correction`
- Prior verdict: `BLOCKED`
- Verdict: **VERIFIED**

## Conclusion

The correction commit closes all five findings from the first independent P1
verification without broadening Provider, platform, identity, or Security
claims. The approved P1 synthetic contract foundation is verified. P2 remains
a separate live phase and was not executed or implicitly authorized here.

## Finding closure

1. **Raw-ID route — closed.** A Provider-omitted request that supplies a stored
   Codex Profile is routed to the confirmation boundary and creates no Session.
   The native Fake cross-provider regression also passes, proving explicit
   non-Codex requests retain their binding semantics.
2. **Auth crash/replay — closed.** Same-client/same-selector attestation replay
   continues after the modeled pre-seal crash. Succeeded replay rejects a wrong
   selector and removes recreated staging residue on exact retry.
3. **Logout/re-login — closed.** Revocation clears and revision-bumps the public
   Profile binding transactionally. The regression performs a complete new
   enrollment, confirmation, Vault seal, Profile bind, and second logout.
4. **Preview retention — closed.** An expired preview remains readable during
   the 24-hour retention interval, still rejects consumption, and is removed
   only after the bounded interval.
5. **Workspace drift — closed.** Migration 7 persists only the Workspace
   `updated_at` surrogate, and transactional consumption rejects a changed
   Workspace with no Session insertion.

## Commands and results

- Targeted auth and selector tests — pass.
- Targeted preview authority/expiry/retention/drift tests — pass.
- Native two-client Fake cross-provider test — pass.
- Device migration suite — pass.
- Preview concurrent-consumer test with `-count=10` — pass.
- `go test -count=1 ./...` — pass.
- `go vet ./...` — pass.
- `go test -count=1 -race ./...` — pass.
- Darwin arm64, Linux amd64, Windows amd64 `go build ./cmd/...` — pass.
- Workflow verification and `git diff --check` — pass before verdict writes.

The static dashboard correctly became commit-stale after `c062738`; the
verdict role did not regenerate or edit dashboard files. The next authorized
writer must refresh generated facts after this verdict. This is deterministic
project-state reconciliation, not a P1 product finding.

No live Provider login, real Account identity, secret material, hidden Usage
request, push, merge, release, platform acceptance, Security acceptance, or
risk acceptance occurred.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-verify / P1 correction`
**Verdict**: `VERIFIED`
**Summary**: `All five prior findings are closed; the approved P1 selector confirmation foundation is verified.`
**Evidence**: `targeted finding regressions; full Go/vet/race; preview race x10; three-OS builds; workflow/diff checks`
**Findings**: `none; dashboard generated-commit reconciliation is reserved for the next writer`
**Blockers**: `none for P1; live P2 still requires explicit phase authorization and its operator/provider gates`

### Next Step

Run the original `feature-plan` / independent `feature-review` gate to authorize
P2 for `codex-multi-account-selector`; do not start the live build from the
P1-only approval.
