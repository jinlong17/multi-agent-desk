# P1 correction build: Codex explicit multi-account selector

- Date: 2026-07-16
- Role: `feature-build / P1 correction`
- Input verdict: `2026-07-16-feature-verify.md`
- Result: **READY_FOR_VERIFY**

## Corrections

All five verification findings are closed in the implementation and regression
suite:

1. A request that omits Provider but supplies a stored Codex RuntimeProfile ID
   now enters the same preview/confirmation boundary and returns
   `identity_confirmation_required`. Explicit non-Codex requests retain their
   existing cross-provider binding failure.
2. Durable same-client/same-selector enrollment attestation is replay-safe
   after a crash before Vault seal. A succeeded confirmation replay revalidates
   the selector digest and client, then retries staging cleanup.
3. Final Credential revocation atomically clears matching public Profile
   bindings and advances their revisions, including already-revoked recovery
   replay, so a later enrollment can bind a new Credential.
4. Expired and consumed Session previews remain available for a 24-hour bounded
   idempotency/audit window before cleanup; expiry still denies consumption.
5. Preview rows pin the Workspace `updated_at` surrogate and consumption
   rechecks it inside the Session-insertion transaction without storing paths.

## Verification evidence

- Targeted storage/application/Vault/migration tests: pass.
- `go test -count=1 ./...`: pass.
- `go vet ./...`: pass.
- `go test -count=1 -race ./...`: pass.
- Darwin arm64, Linux amd64, Windows amd64 `go build ./cmd/...`: pass.
- The full run exposed and closed one intermediate regression: an explicit
  `fake` request carrying a Codex Profile was initially routed to confirmation;
  provider inference is now limited to requests whose Provider is omitted.

No live Provider login, account identity, secret payload, Usage request, push,
merge, release, support broadening, or risk acceptance occurred.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-build / P1 correction`
**Status**: `READY_FOR_VERIFY`
**Summary**: `Closed all five P1 verification findings across raw-ID routing, auth replay/cleanup, logout re-login, preview retention, and Workspace drift.`
**Files Written**: `application/storage/migration tests and contracts; p1-as-built.md; this report; dev_log.md; dashboard-state.json`
**Tests**: `targeted tests; full Go; vet; race; three-OS builds; Web/Desktop and governance matrix`
**Blockers**: `none for independent P1 re-verification; P2 live and platform/Security gates remain unchanged`

### Next Step

Run `feature-verify` for `codex-multi-account-selector` P1 correction.
