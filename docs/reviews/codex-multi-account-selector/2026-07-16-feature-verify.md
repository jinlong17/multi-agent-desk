# Feature verify P1: Codex explicit multi-account selector

- Date: 2026-07-16
- Reviewed commit: `50794df`
- Role: `feature-verify / P1`
- Verdict: **BLOCKED**
- Clearing role: `feature-build / P1 correction`

## Conclusion

The migration, primary preview transaction, explicit enrollment split, public
CLI shape, exact-Linux compatibility gate, redaction boundary, and regression
matrix are substantially implemented and all executed tests pass. P1 is not
yet `VERIFIED`, however, because five approved fail-closed/recovery contracts
remain incomplete in code or tests.

## Findings

### P1 — Raw-ID Codex start can evade the required error branch

`session.start` chooses the preview path only when `profile_selector`,
`provider == codex`, or `account_id` is present. A caller can omit provider and
account while supplying a Codex RuntimeProfile/Credential ID. `runtime.Manager`
then resolves the stored Profile to Codex. The current request still fails when
its empty Account reaches the runtime, but it bypasses the promised sole
application contract and does not return `identity_confirmation_required`.
Determine the stored Profile provider before selecting the public start path,
and add a raw-ID omission test proving zero Session/process/materialization.

### P1 — Auth confirmation crash/replay contract is incomplete

After `ConfirmAuthEnrollmentAttestation` records `confirmed_by_client_id`, a
crash before Vault seal leaves `awaiting_confirmation`. An exact retry attempts
the `confirmed_by_client_id IS NULL` update and returns conflict instead of
continuing idempotently. Separately, a retry after successful Vault seal returns
success before validating the submitted selector and before removing residual
staging. This permits a new-key wrong-selector replay and can retain staged auth
after a post-seal cleanup failure. Treat same-owner/same-digest attestation as a
replay, validate persisted selector digest on succeeded replay, and always
retry safe staging cleanup.

### P1 — Alias logout prevents the planned re-login

Credential revocation leaves `runtime_profiles.credential_instance_id` bound
to the revoked Credential. A later selector `auth.begin` creates a new
Credential, but Vault seal cannot bind it because the Profile still points to
the old ID. Final revocation must atomically clear matching public Profile
bindings and advance their revisions, with a regression test showing another
enrollment can bind safely while unrelated Profiles remain unchanged.

### P2 — Expired previews are deleted before the bounded audit window

Startup cleanup deletes every unconsumed preview as soon as `expires_at` is in
the past. The approved design retains consumed and expired rows for bounded
idempotency/audit time before deletion. Keep expired rows for the chosen
retention interval, continue rejecting them as `confirmation_expired`, and add
restart/cleanup retention tests.

### P2 — Workspace drift is not bound to the preview

The approved TOCTOU matrix requires Workspace mutation after preview to create
no Session. The row stores only `workspace_id`, and consumption rechecks only
the Workspace device. Bind a safe Workspace revision surrogate such as its
`updated_at` value (without storing the path), recheck it transactionally, and
add a drift negative.

## Passing evidence

- `go test -count=1 ./...` — pass.
- `go vet ./...` — pass.
- `go test -count=1 -race ./...` — pass.
- preview concurrent-consumer test repeated 10 times — pass, one Session.
- Darwin arm64, Linux amd64, Windows amd64 `go build ./cmd/...` — pass.
- v6-to-v7 active enrollment preservation and reopen — pass.
- Web TypeScript check/build; Desktop Rust fmt/check — pass.
- workflow/dashboard/layout/format/Actions/CODEOWNERS/fixtures/links/licenses
  and `git diff --check` — pass.

No Provider identity, credential payload, hidden Usage request, live Account
login, support broadening, push, merge, or release occurred during verification.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-verify / P1`
**Verdict**: `BLOCKED`
**Summary**: `Core P1 contracts pass, but raw-ID routing, auth replay/cleanup, logout re-login, preview retention, and Workspace drift remain incomplete.`
**Evidence**: `full Go/vet/race; three-OS builds; migration, preview authority/race, auth, Web/Desktop and governance checks`
**Findings**: `P1 raw-ID route; P1 auth replay/cleanup; P1 logout re-login; P2 preview retention; P2 Workspace drift`
**Blockers**: `feature-build P1 correction must close all five findings before re-verification`

### Next Step

Run `feature-build` P1 correction for `codex-multi-account-selector`.
