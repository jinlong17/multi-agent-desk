# Security Review: Codex P3 schema reconciliation

## Verdict

`ACCEPTED` for the narrow Spike evidence and canonical-schema decision. This
does not accept a credentialed Codex Session, live Approval mutation, Linux
deployment, or Ship readiness.

The experiment used an empty disposable `CODEX_HOME`, retained no raw app-server
responses containing identifiers, read no auth file, and persisted only method
names, bounded result keys, error classes, file counts, and hashes. Canonical
JSON hashing is safer than raw generated-byte hashing because it removes
nondeterministic object ordering without ignoring semantic keys or values.

## Findings

### P0 — accepted boundary

The compatibility decision may adopt canonical JSON fingerprints only if the
implementation continues to:

- reject duplicate keys, invalid JSON, symlinks, non-regular files, excessive
  file counts/sizes, and unknown versions;
- bind the fingerprint to every relative schema path plus canonical content;
- keep experimental methods disabled unless separately evidenced;
- fail closed when the exact canonical fingerprint or allowlisted method shape
  is absent.

### P1 — implementation correction required

The current synthetic initialize and Approval method assumptions are falsified.
They must not be grandfathered as aliases. The adapter must implement the exact
`clientInfo` initialize request/current result shape and distinguish server
Approval requests from client notifications. A Provider server request must
retain its JSON-RPC request ID and accept a response only after local
ControllerLease and idempotency authorization.

### P2 — live risk remains open

No credentialed Account, Usage value, Approval, turn, refresh, Linux binary, or
second CLI was exercised. ADR 0014's single writer, CAS, isolated auth home,
quarantine, pinned Account/Profile, and no-rotation rules remain mandatory.

## Evidence reviewed

- `docs/spikes/codex/p3-schema-reconcile.json`
- `docs/adr/0014-codex-app-server-single-writer-auth.md`
- `docs/THREAT_MODEL.md` T-05, T-06, T-09, T-14, and T-17
- two same-binary schema generations, exact npm version generations, and an
  empty-home initialize/Account replay
- evidence JSON parse validation and repository secret-safety constraints

## Residual risk

- A malicious or compromised Provider binary can emit a valid schema while
  behaving differently at runtime; schema fingerprints are compatibility
  evidence, not binary attestation.
- Canonicalization cannot prove undocumented refresh or Approval semantics.
- Provider-readable plaintext exists in a credentialed auth home at runtime;
  root/admin, malware, the Provider, backups, or crash tooling may copy it.
- Approval payload fields may change across versions; any unmapped field must
  remain unavailable rather than entering logs or audit records.
- Device revocation cannot erase credentials already copied by an authorized or
  compromised host.

## Handoff

**Target**: `spike-codex-p3-schema-reconcile`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: `Canonical JSON schema fingerprinting and the empty-home protocol evidence are acceptable under strict fail-closed parsing and exact-method constraints; this is not live Session acceptance.`
**Findings**: `Replace nondeterministic raw hashing and synthetic initialize/Approval aliases; preserve server-request IDs and ControllerLease/idempotency authorization.`
**Evidence**: `docs/spikes/codex/p3-schema-reconcile.json`; ADR 0014; Threat Model T-05/T-06/T-09/T-14/T-17; empty-home replay.
**Residual Risk**: `Schema identity is not binary attestation; credentialed Linux, real Approval/Usage/turn behavior, Provider compromise, and host plaintext exposure remain open.`

### Next Step

Run `feature-plan` decision for `spike-codex-p3-schema-reconcile`, then return
the accepted corrections to `feature-build` P3.
