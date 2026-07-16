# Security Review: Codex distinct-account managed Homes

- Date: 2026-07-16
- Target: `spike-codex-distinct-account-homes`
- Owner module: `provider`
- Reviewed commit: `f83d06f`
- Verdict: **ACCEPTED**

## Scope and conclusion

The exact Linux `x86_64` Codex CLI `0.144.2` spike evidence is accepted as a
bounded Provider compatibility result. The experiment demonstrates that two
operator-owned official logins can coexist in two CredentialInstance-scoped
managed Homes, that concurrent Sessions and Usage remain account-bound, and
that target-B stop/logout/re-login does not mutate target A. The experiment
also exercised the fail-closed active-session logout rule and monotonic Vault
revision update.

This verdict accepts evidence, not a general multi-platform product claim. It
does not accept macOS distinct-account support, Windows Codex support, a passive
soak claim, automatic account rotation, multi-writer refresh, or identity
guessing. Those exclusions are explicit in the spike report and compatibility
matrix and must remain visible in the feature-plan decision.

## Trust-boundary review

### Official login and callback

`auth.begin` creates a private, Device-root-local staging directory, binds the
enrollment to the authenticated local client, RuntimeProfile, Credential, exact
Provider binary fingerprint, idempotency digest, and ten-minute expiry. The
official Codex binary owns the Provider OAuth/PKCE/MFA interaction. Completion
re-checks the binary fingerprint, validates the staged credential using the
exact app-server, imports only the bounded `auth.json`, seals it into the Vault
with revision CAS, clears the plaintext buffer best-effort, and removes the
staging directory. Expiry/cancel/restart cleanup is fail-closed and does not
install a partial credential.

The browser account choice, MFA, and loopback callback remain operator and
Provider trust surfaces. MultiAgentDesk neither bypassed them nor retained the
callback URL, authorization code, token, cookie, raw claim, email, or display
name in repository evidence.

### Vault, materialization, and refresh

Each CredentialInstance owns a distinct Vault item and a canonical Home named
from the validated Credential ID. The Home root and Home are `0700`; credential
and lock files are `0600`. Materialization validates regular-file shape, size,
strict JSON, digest, revision, and manifest; unexpected or ambiguous residue is
quarantined. The exclusive writer lock, expiring lease, exact revision checks,
and Vault CAS preserve ADR 0014's single-writer rule independently per
CredentialInstance.

The live evidence confirms two materializations can coexist and that final
Session cleanup removes both plaintext auth files. It does not weaken the
known residual risk that the Provider process, same-user malware, root/admin,
backup, crash, or memory tooling can copy authenticated state while a Home or
unlocked Vault is live.

### Account/session/Usage binding

Runtime start requires the Profile, Credential, Workspace, Device, and Account
tuple to match before a Session is created. The runtime map and writer lock are
keyed by CredentialInstance, so different Accounts do not share an app-server
or auth Home. Usage reads are selected by Account ID and stored snapshots are
account-bound. The experiment independently confirmed two distinct Provider
identities, two distinct Provider session IDs, and concurrent account-bound
Usage without persisting raw identity values.

The current P1 selector path still returns a gated error rather than launching
a multi-account Session. That is the correct security posture until the next
approved build adds a user-facing confirmation step and privacy-preserving
post-login identity binding. `ValidateEnrollment` intentionally returns no
Provider identity today; therefore metadata/profile matching alone must not be
presented as proof that the operator selected the intended upstream account.

### Scoped logout and replay/race behavior

Logout first reserves revocation transactionally. The reservation rejects a
new Session, active `starting`/`running`/`stopping` Sessions block logout, and
finalization removes only the selected Credential's Vault item and marks only
that Credential revoked. Replays are idempotent. The live negative and scoped
logout/re-login sequence agrees with the storage tests: target A's auth bytes,
Session, Usage, Vault item, and credential state were unchanged throughout.

## Findings

### P0

None.

### P1

None for accepting the bounded spike evidence.

### P2 / required decision constraints

1. Before the gated selector path becomes a stable user-facing launch path,
   bind the post-login Provider identity to the intended Account using a
   privacy-preserving stable identifier or an explicit operator confirmation.
   Do not store email/display-name PII as the identity key, and do not infer
   correctness only from the internal Account/Profile/Credential tuple.
2. Preserve exact-version fail-closed behavior and the explicit no-auto-
   rotation rule. A Usage recommendation may inform the operator but may not
   switch CredentialInstance or Account during a Session.
3. Keep macOS distinct-identity, real Windows Codex, and passive-soak support
   outside the accepted compatibility claim until separately reproduced and
   reviewed.
4. The Linux SSH target emitted a non-post-quantum key-exchange warning. This
   does not invalidate the local Provider result, but production deployment on
   that host requires an SSH service/client policy upgrade or explicit
   infrastructure risk decision.

## Verification evidence

- `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes-spike.md`
- `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes.json`
- `docs/adr/0014-codex-app-server-single-writer-auth.md`
- `docs/THREAT_MODEL.md` invariants 4-5 and threats T-04/T-05/T-06/T-09/T-14/T-17
- `internal/app/session_service.go`
- `internal/storage/enrollment.go`
- `internal/storage/vault_records.go`
- `internal/providers/codex/enrollment.go`
- `internal/providers/codex/materialization.go`
- `internal/providers/codex/runtime.go`
- `go test ./internal/app ./internal/providers/codex ./internal/storage ./internal/vault` — PASS
- `go test -race ./internal/app ./internal/providers/codex ./internal/storage ./internal/vault` — PASS

## Residual risk

Authenticated Provider state necessarily exists in Provider-readable plaintext
while a managed Home is active. Host compromise, same-user malware, an
operator-approved malicious Provider binary, browser/session compromise, and
crash/backup tooling can copy it. Upstream Codex schema, OAuth, callback,
credential, refresh, and Usage semantics can change outside the pinned version.
Logout and revocation prevent future MultiAgentDesk materialization but cannot
erase an already copied credential. Availability and SSH interception risks
also remain outside the local credential-isolation proof.

These risks are acceptable for recording the exact Linux compatibility result
because the feature remains gated, the fallback is deterministic, no automatic
account selection is enabled, and unsupported platforms/versions fail closed.

## Handoff

**Target**: `spike-codex-distinct-account-homes`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: `Exact Linux 0.144.2 two-identity Home isolation, concurrent account binding, and scoped logout/re-login evidence is accepted; no cross-platform or automatic-selection claim is accepted.`
**Findings**: `P0 none; P1 none for the bounded spike; P2 require privacy-preserving post-login identity binding/confirmation, exact-version fail-closed behavior, no auto-rotation, separate macOS/Windows/soak gates, and SSH KEX hardening before production.`
**Evidence**: `sanitized spike report/JSON, ADR 0014, threat model, auth/Vault/materialization/runtime source, targeted Go and race suites`
**Residual Risk**: `Provider-readable runtime plaintext, host/browser/Provider compromise, upstream semantic drift, already-copied credentials, and transport availability/interception remain.`

### Next Step

Run `feature-plan`.
