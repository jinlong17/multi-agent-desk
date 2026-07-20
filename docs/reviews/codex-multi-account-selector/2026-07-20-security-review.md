# Security Review: Codex explicit multi-account selector

- Date: 2026-07-20
- Target: `codex-multi-account-selector`
- Owner module: `provider`
- Impacted modules: `core`, `security`, `desktop`, `project-system`
- Reviewed build commits: `4c00aec`, `25328b1`, correction `ec73d1c`
- Functional verification: `1d6f4b6` / `READY_TO_SHIP`
- Verdict: **ACCEPTED**

## Scope and conclusion

The Security Gate is accepted for the bounded selector implementation: Linux
`amd64`, exact Codex CLI `0.144.2`, explicit operator-owned `@alias`
selection, one confirmed CredentialInstance per selected Account, and no
automatic account rotation or mid-Session credential switch.

The reviewed implementation preserves the existing authenticated local IPC,
Vault, single-writer materialization, revision-CAS, revocation reservation,
and immutable Session tuple authorities. Enrollment confirmation and Session
start confirmation are separate owner-bound actions. The public Codex start
surface has no raw-ID or default-Account bypass, and all five selector
platform boundaries fail closed before unsupported work can produce a usable
credential or Provider runtime.

This verdict does not accept macOS distinct-identity operation, real Windows
Codex operation, versions outside the exact compatibility row, upstream
identity inference, multi-writer refresh, automatic fallback, push, merge,
package, release, or deployment.

## Trust-boundary review

### Official login, identity attestation, and platform gate

`auth.begin` discovers the official binary and applies the selector platform
gate before creating an enrollment, placeholder CredentialInstance, staging
root, or auth Home. `auth.complete` claims the owner-bound enrollment, then
repeats the platform and binary checks before Provider validation or reading
the staged credential. `auth.confirm` durably records the exact operator alias
attestation, then repeats the platform/binary checks before validation, auth
read, and Vault seal. Complete/confirm drift terminally fails the enrollment,
deletes the unknown placeholder Credential, and removes staging.

The attestation binds the authenticated client, enrollment, Account, Profile,
Credential, revisions, canonical alias digest, expiry, and binary fingerprint.
It truthfully confirms the operator's internal target; it is not described as
a durable upstream Codex identity proof. No email, display name, organization,
subject claim, callback URL, OAuth code, cookie, token, or raw auth payload is
persisted in selector evidence.

### Preview, replay, and public start authorization

`sessions.preview` issues a random server-side record bound to the
authenticated client, immutable Account/Profile/Credential/Device/Workspace
tuple, revisions, selected Usage row, exact provider/schema/capability
fingerprints, and ten-minute expiry. `session.start` rechecks those fields and
atomically consumes the preview in the same SQLite transaction that inserts
one `starting` Session.

Cross-client, forged, expired, drifted, already-consumed, or differently
replayed requests fail closed. Exact lost-response replay returns the original
Session. Revocation reservation is checked by Session insertion, so logout and
start cannot create a post-revocation Session. Public Codex start always
requires a selector, preview, and full confirmation; opaque IDs remain bound
fields rather than an alternate authorization surface.

### Vault, materialization, and runtime isolation

Credentials are sealed into the local Vault with CredentialInstance,
Account, Device, provider, expected revision, and enrollment ownership
binding. Auth Homes and staging directories are private, auth/lock files are
`0600`, materialization rejects raw paths, malformed/duplicate JSON and
unexpected files, and ambiguous residue is quarantined rather than selected by
mtime. One writer lease and monotonic revision CAS remain authoritative for
each CredentialInstance.

`Runtime.StartReserved` re-discovers the Provider and repeats the platform,
binary, schema, capability, version, and immutable tuple checks after Session
reservation and before materialization/spawn. A mismatch records the reserved
Session failed and never falls back to another Account. Runtime ownership and
bindings are keyed by CredentialInstance; the accepted A/B receipt shows
distinct Homes, app-server processes, Provider thread IDs, Account IDs,
Credential IDs, Sessions, and Usage rows, followed by zero materialized
`auth.json` files after stop and cleanup.

### Usage, logout, and cross-account effects

Usage refresh requires an active running binding for the requested Account and
persists the matching CredentialInstance, Device, exact Provider version,
availability, observation/staleness timestamps, and a redacted response
digest. It does not persist raw responses or fabricate quota windows, and it
never authorizes account switching.

Logout first reserves revocation transactionally. Active or starting Sessions
deny logout; the reservation denies new Sessions; finalization removes only
the selected Vault item and Profile binding and marks only that Credential
revoked. The verified B stop/logout/re-login sequence retained A's Credential,
Session and Usage state unchanged.

### Audit and diagnostic exposure

The selector contracts restrict audit/evidence fields to opaque IDs,
revisions, alias digests, fingerprints, safe error codes, Provider version and
timestamps. Stable errors exclude Provider identity, auth payload, Usage
values and raw Provider messages. A repository scan of the feature documents
and reviews found no operator identity name, phone number, token or OAuth
parameter. References to `auth.json` describe only the bounded file and its
verified absence after cleanup.

## Findings

### P0

None.

### P1

None. The prior P1 platform-gate finding is closed by `ec73d1c` and direct
begin/complete/confirm artifact-ordering and cleanup tests.

### P2 / retained constraints

1. Operator alias confirmation can be mistaken and is not upstream identity
   proof. Product wording must continue to describe it as an explicit internal
   target attestation.
2. macOS remains `schema_compatible_identity_acceptance_pending`; Windows and
   other platforms remain `provider_platform_unsupported`. Cross-compilation
   and schema smoke are not live identity acceptance.
3. Unknown CLI versions, schema drift, binary drift, revision ambiguity, or
   materialization residue must continue to fail closed or quarantine and
   require explicit re-login; no automatic Account fallback is permitted.
4. The Linux SSH target's non-post-quantum KEX warning requires infrastructure
   hardening or an explicit deployment risk decision before production use.

These are residual constraints, not blockers for the exact bounded feature.

## Verification evidence

- `docs/THREAT_MODEL.md` invariants 4-5 and T-04/T-05/T-06/T-09/T-14/T-17.
- `docs/adr/0014-codex-app-server-single-writer-auth.md` and its
  distinct-account addendum.
- `p1-as-built.md`, `p2-as-built.md`, `p3-as-built.md`, selector design/API,
  P2 live receipt, and P3 correction verification.
- Source inspection of `internal/app/session_service.go`,
  `internal/storage/session_preview.go`, `internal/storage/enrollment.go`,
  `internal/storage/vault_records.go`, and Codex compatibility,
  materialization, runtime, Session and Usage paths.
- Auth enrollment, platform drift, confirmation, scoped logout and re-login
  tests under `-race -count=5` — PASS.
- Selector-only public start and owner-bound preview tests, including preview
  race under `-race -count=10` — PASS.
- Materialization permissions/single-writer/recovery and reserved-runtime A/B
  isolation/Usage tests under `-race -count=5` — PASS.
- Independent full Go/vet/race, three-OS build, Web/Desktop, governance and
  exact Linux A/B acceptance evidence — PASS.

## Residual risk

Provider-readable authenticated state necessarily exists while an authorized
Home/runtime is active. Same-user malware, host root/admin, an operator-approved
malicious Provider binary, browser compromise, backup/crash tooling, or live
process compromise can copy or use that state. Vault encryption and `0600`
permissions do not protect an already compromised runtime or administrator.

Logout and revocation prevent future MultiAgentDesk materialization but cannot
erase a credential already copied by an authorized or compromised host.
Upstream Codex OAuth, schema, credential, refresh, Usage and binary semantics
may drift outside the pinned row. Operator confirmation can target the wrong
browser account. SSH interception/availability and the target's current KEX
policy remain deployment risks. Conservative Usage staleness and absent
quota-window projection may reduce recommendation quality but do not authorize
automatic rotation.

These risks are accepted only for the exact Linux `amd64` / Codex `0.144.2`
scope because unsupported states fail closed, account choice remains explicit,
runtime state is CredentialInstance-isolated, and no remote or release action
is implied by this review.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: `The exact Linux 0.144.2 explicit-selector boundary is accepted: owner-bound login and Session confirmations, one-time preview consumption, Vault/revision binding, CredentialInstance-isolated runtime/Usage, scoped revocation, redaction, and five fail-closed platform checks have evidence.`
**Findings**: `P0 none; P1 none; P2 retain operator-attestation wording, exact-version fail-closed behavior, unsupported macOS/Windows gates, no auto-rotation, and SSH KEX hardening before production.`
**Evidence**: `threat model; ADR 0014; selector design/API/as-built and verification receipts; auth/preview/Vault/materialization/runtime/Usage source; targeted race suites; retained full matrix and exact-Linux A/B receipt`
**Residual Risk**: `Provider-readable runtime plaintext, same-user/root/browser/Provider compromise, mistaken operator confirmation, upstream semantic drift, already-copied credentials, and SSH transport availability/interception remain.`

### Next Step

Run `ship`.
