# Security review: Phase 1 Device Kernel — ACCEPTED

## Verdict

`ACCEPTED` for exact implementation/evidence head
`bf5f5c3787d5b45e830b71ee16485b1e391271af`.

The two original P1 findings and the subsequent parser-boundary gap are closed:
CLI request identity is bound to operation data, and Vault unlock rejects both
flag and positional argv secret material before reading bounded stdin. No new
transport, plaintext credential sync, automatic rotation, or copied-secret
erasure claim was introduced.

## Security controls reviewed

- The CLI derives bounded request ID and idempotency key from method, canonical
  JSON body, and optional lease revision. Exact retries reproduce the same
  identity; distinct operation bodies and revisions do not collide. The daemon
  still recomputes the request digest and rejects key reuse with a different
  body.
- `vault unlock` exposes only `--secret-stdin`; the removed `--secret` flag and
  all positional arguments are rejected at parse time. Stdin is bounded to
  4096 bytes, newline-trimmed, and never printed. The documented production
  invocation is a pipe or other no-echo input source.
- Ed25519 mutual authentication, capability authorization, lease revision
  checks, endpoint ACLs, bounded frames/connections, replay/idempotency
  records, materialization digest/atomicity, and Windows current-logon DACL
  controls remain unchanged and were re-exercised by the existing suites.
- Service-spec commands remain render-only; client provisioning/rotation/
  revocation remains offline-only; fake Vault state is held in daemon memory
  and no production credential bytes are synchronized or persisted.

## Evidence

- Local full Go, vet, race, three-target compile, exact license, project, CI,
  scaffold, workflow, dashboard, and diff checks pass.
- Draft PR #13 CI `29396672544` passed project-verify `87291649879`, Ubuntu
  `87291649887`, macOS `87291649933`, and Windows `87291649897`.
- Governance `29396672597` passed DCO, license-gate, and link-check.
- Focused tests cover request identity binding, bounded stdin, no output of
  the secret, removed `--secret`, and rejected positional argv values.

## Residual risk and explicit non-claims

Phase 1 remains a same-user/local-device Fake Kernel. Production Vault
cryptography, real Provider credentials and PTY/ConPTY behavior, Windows 11
multi-user/service acceptance, signed packaging, deployment, release, and
online control-plane synchronization remain later phases. Offline client
provisioning must not be described as erasing secrets already copied to a
device. The ship step must still verify the protected PR head and merge gate.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: `The corrected CLI request identity and Vault secret ingress satisfy the Security Gate; no new high-risk boundary was introduced.`
**Findings**: `none`
**Evidence**: `bf5f5c3; cmd/multidesk/commands.go; cmd/multidesk/main_test.go; internal/app/session_service.go; CI 29396672544; Governance 29396672597`
**Residual Risk**: `Fake same-user local kernel only; no production crypto, real Provider/PTY, Windows 11 multi-user/service, signed release, deployment, or copied-secret erasure claim.`

### Next Step

Run `ship` for `phase1-device-kernel` with the explicit human merge authorization.
