# Feature review: Phase 1 Device Kernel

- Date: 2026-07-14
- Role: `feature-review`
- Verdict: `REVISE`
- Reviewed state: `NEEDS_REVIEW`
- Owner module: `core`

## Conclusion

The feature is correctly scoped to the Device Kernel, preserves the real
Provider/PTY/Control Plane/Desktop/release boundaries, defines a credible
single-writer architecture, and makes macOS, Linux, and Windows native IPC E2E
evidence mandatory. The five phases are ordered and independently verifiable.

Implementation must not begin yet because two plan details would require the
builder to invent compliance or integrity decisions. Both are local planning
corrections; no external blocker or new Spike is required.

## Findings

### P1 — replace or explicitly clear the SQLite module with an unidentified license

Files: `design.md` (Persistence and runtime), `dev_log.md` (Evidence Ledger).

The selected `github.com/ncruces/go-sqlite3 v0.35.2` pulls
`github.com/ncruces/go-sqlite3-wasm/v3 v3.2.35303`. The repository contains an
MIT No Attribution license, but the exact CI scanner
`github.com/google/go-licenses/v2@v2.0.1` reports that module as
`Unknown,Unknown` and emits `no license found`. Its `check` command currently
returns success, which is not sufficient evidence for the plan's claim that
the full boundary is recognized and allowed.

The planner must either select a driver whose complete transitive production
set is recognized by the exact governance scanner, or add an explicit,
reviewable governance exception/override with authoritative license evidence.
The simpler reviewed option is `modernc.org/sqlite v1.53.0`: the same scanner
recognizes its production dependency set as MIT/BSD-3-Clause and the allowlist
check passes. The revised choice still requires three-platform compile and P1
runtime WAL/restart evidence.

### P2 — freeze fake materialization integrity semantics without an undefined signature

Files: `design.md` (Vault and materialization), `test.md` (failure injection).

The phrase “signed/hashed fake manifest” does not define the signing identity,
canonical encoding, key storage, trust source, or what a valid signature proves.
Phase 1 explicitly defers production Vault cryptography, so a builder cannot
choose a signature scheme without expanding the security design.

The planner should define a deterministic versioned canonical manifest with an
unkeyed SHA-256 integrity digest recorded in the database, state that it detects
accidental/partial residue but does not prove authenticity against a same-user
attacker, and retain production authenticated materialization for the later
security-owned Vault feature. Tests must cover manifest version/digest/revision
mismatch and quarantine. If authenticity is required now, the planner must
instead specify the full key and verification lifecycle and keep it in the
Security Gate.

## Non-blocking builder notes

1. Store the authenticated client identity revision in each connection and
   revalidate it before every mutation, or provide an equivalent revocation
   broadcast; otherwise the promised immediate rotation/revocation semantics
   are not testable.
2. Bound connection count, in-flight requests, handshake duration, idle time,
   event queue, and audit/idempotency retention with named constants so the
   adversarial suite can assert exact limits.
3. Keep offline client recovery a distinct command that requires the Daemon
   lock; do not let ordinary `client rotate` silently become a bypass around
   local IPC authentication.
4. The Windows adapter should reuse the accepted Spike's Win32 control set but
   not its probe-only command/process harness. Live-DACL readback and
   first-instance behavior remain production acceptance conditions.

## Evidence reviewed

- Feature Brief and all four canonical workflow documents.
- `docs/IMPLEMENTATION_PLAN.md`, `docs/ARCHITECTURE.md`,
  `docs/DATA_MODEL.md`, and `docs/workflow/project/workflow.md`.
- ADR 0002, 0005, 0009, 0012, 0013, and 0015.
- Upstream module metadata and license files for both SQLite candidates.
- Go 1.26.5 cross-compilation evidence for the selected candidate.
- Exact `go-licenses v2.0.1 check/csv` runs for
  `github.com/ncruces/go-sqlite3/driver` and `modernc.org/sqlite`.
- `npm run workflow:verify`, `npm run project:verify`, and `git diff --check`.

## Gates and blockers

- Provider Gate: none; Fake Provider only.
- Security Gate: remains open for the final implementation.
- External blockers: none.
- Clearing role: `feature-plan` resolves both findings, restores
  `NEEDS_REVIEW`, and resubmits the complete plan.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: The architecture and phase order are sound, but the SQLite dependency has an unidentified transitive license in the exact scanner and the materialization signature semantics are undefined.
**Findings**: P1 select a fully recognized SQLite dependency or document an explicit reviewed exception; P2 replace the undefined signature with a frozen integrity-only manifest or specify a complete signing lifecycle.
**Evidence**: Feature documents, ADRs, Go 1.26.5 cross-compilation, exact go-licenses v2.0.1 check/csv, workflow/project verification, and diff integrity.
**Blockers**: No external blocker; `feature-plan` can clear both findings.

### Next Step

Run `feature-plan` for `phase1-device-kernel`.
