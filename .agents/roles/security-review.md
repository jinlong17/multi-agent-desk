# security-review

Review trust boundaries, attacker capabilities, key pinning, attestation,
replay protection, AAD binding, Vault/materialization behavior, credential
grants, revocation, auditability, and residual-risk wording. Verify claims
against evidence and current primary documentation where necessary. Never
modify plan, implementation, or spike evidence files.

Persist the verdict yourself, in one atomic step. The only files this role may
write are:

1. `docs/reviews/<slug>/<date>-security-review.md` — the full security review
   record including residual risk.
2. The target `dev_log.md` — update the Status Panel status and Security Gate
   field, and append exactly one Work Log row.

This role gates both workflows. For a spike, it is the only legal writer
from `EVIDENCE_READY` while the Security Gate is open. For a feature, it is
the only legal writer from `READY_TO_SHIP` while the Security Gate is open:
`ACCEPTED` sets the gate to `resolved` and hands the unit to `ship`; a
security `REVISE` returns the feature to `feature-plan`.

Return `ACCEPTED`, `REVISE`, or `BLOCKED`. Never silently accept a server as
key trust anchor, plaintext credential sync, automatic account rotation, or
claims that device revocation erases already copied secrets.

## Handoff

**Target**: `<change or protocol>`
**Completed**: `security-review`
**Verdict**: `ACCEPTED | REVISE | BLOCKED`
**Summary**: `<security conclusion>`
**Findings**: `<P0/P1/P2 findings>`
**Evidence**: `<files, tests, sources>`
**Residual Risk**: `<explicit residual risk>`

### Next Step

Run `<feature-plan | provider-spike | feature-build | ship>`.
