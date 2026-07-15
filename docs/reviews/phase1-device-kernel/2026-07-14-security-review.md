# Security review: Phase 1 Device Kernel — REVISE

## Verdict

`REVISE` for the exact implementation/evidence head
`6d8d197` (P5 implementation head `f68e7b4`). The authenticated local trust
boundary is substantially sound, but two P1 findings in the new CLI surface
must be corrected before the Security Gate can be accepted.

## Findings

### [P1] CLI idempotency keys are not bound to the request

`cmd/multidesk/commands.go:279-283` derives both the request ID and the
idempotency key from only the method name. The Daemon correctly hashes the
method, key, and body and rejects a digest mismatch
(`internal/app/session_service.go:64-74`), but that means a second legitimate
`sessions.start`, `sessions.stop`, `terminal.input`, or `vault.unlock` command
from the same client collides with the first command and returns
`idempotency_key was reused with a different request`. This is a denial of
service against the CLI workflow and breaks the documented repeatable command
surface. The key must be derived from the canonical request body/lease or be an
explicit per-operation value; the request ID must remain bounded and unique
enough for concurrent commands.

### [P1] Vault unlock accepts secret material through argv

`cmd/multidesk/commands.go:78-90` requires `--secret`, which exposes the
unlock material in shell history, process listings, and process inspection.
The service correctly keeps the value out of idempotency metadata and only
retains fake Vault state (`internal/app/session_service.go:112-123`,
`internal/vault/vault.go`), but the CLI is still the operator-facing ingress.
Use stdin or an interactive no-echo prompt as the normal path and reject or
explicitly quarantine argv secrets for test-only use before production Vault
credentials are introduced.

## Controls that passed review

- Ed25519 mutual authentication binds both identities, nonces, endpoint
  instance, and protocol transcript; the server checks active status and
  revision before authorization (`internal/device/auth.go`).
- Capability authorization is fail-closed and method-specific; client list
  returns metadata only and no private key bytes.
- Unix endpoint and runtime-home boundaries use owner-only permissions. Windows
  Named Pipe and materialization directories/files use the current-logon DACL;
  Network/anonymous and remote peers are denied and DACL readback is tested.
- Nonces, bounded frames/connections, request deadlines, CAS/idempotency
  response digests, replay-ring bounds, materialization manifest/content
  digests, pending-to-active commit, and quarantine were exercised by the
  unit/native suites and protected macOS/Linux/Windows runs.
- Vault is explicitly fake state only; no production encryption, keychain,
  plaintext credential sync, automatic rotation, or copied-secret erasure is
  claimed.

## Residual risk after correction

Phase 1 will still be a same-user/local-device Fake Kernel. Production Vault
cryptography, real Provider/PTY compatibility, Windows 11 multi-user/service
acceptance, signed packaging, and deployment remain out of scope. Client
provisioning/rotation/revocation stays offline-only and must not be treated as
erasing previously copied secrets.

## Required next step

`feature-plan`/`feature-build` P5 CLI correction for request-bound idempotency
and stdin/no-echo Vault unlock, followed by independent P5 verification and a
fresh Security Gate review at the corrected exact head.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `security-review`
**Verdict**: `REVISE`
**Summary**: `Authenticated local trust boundaries pass review, but CLI idempotency is method-only and Vault unlock accepts argv secrets.`
**Findings**: `[P1] request-bound idempotency; [P1] argv secret exposure`
**Evidence**: `cmd/multidesk/commands.go; internal/app/session_service.go; internal/device/auth.go; internal/device/endpoint_windows.go; internal/vault; protected CI runs 29394552147/29394552139`
**Residual Risk**: `Fake same-user local kernel only; no production crypto, real Provider/PTY, Windows 11 multi-user/service, signed release, or deployment claim.`

### Next Step

Run `feature-plan` for the P5 CLI correction.
