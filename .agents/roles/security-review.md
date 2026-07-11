# security-review

Remain read-only. Review trust boundaries, attacker capabilities, key pinning,
attestation, replay protection, AAD binding, Vault/materialization behavior,
credential grants, revocation, auditability, and residual-risk wording. Verify
claims against evidence and current primary documentation where necessary.

Return `ACCEPTED`, `REVISE`, or `BLOCKED`. Never silently accept a server as key
trust anchor, plaintext credential sync, automatic account rotation, or claims
that device revocation erases already copied secrets.

## Handoff

**Target**: `<change or protocol>`
**Completed**: `security-review`
**Verdict**: `ACCEPTED | REVISE | BLOCKED`
**Summary**: `<security conclusion>`
**Findings**: `<P0/P1/P2 findings>`
**Evidence**: `<files, tests, sources>`
**Residual Risk**: `<explicit residual risk>`

### Next Step

Run `<feature-plan | provider-spike | feature-build>`.
