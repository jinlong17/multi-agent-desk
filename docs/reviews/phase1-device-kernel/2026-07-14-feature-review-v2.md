# Feature review v2: Phase 1 Device Kernel

- Date: 2026-07-14
- Role: `feature-review`
- Verdict: `APPROVED`
- Reviewed state: `NEEDS_REVIEW` after revision
- Owner module: `core`

## Conclusion

The revised plan is executable without inventing a dependency-compliance,
integrity, protocol, authorization, migration, platform, or phase-order
decision. Both prior findings are fully resolved and no new blocking finding
was introduced.

The feature may enter P1 only. P2 remains dependent on an independent P1
verification, and the final open Security Gate remains mandatory before ship.

## Prior finding closure

1. `modernc.org/sqlite v1.53.0` replaces the candidate whose Wasm dependency
   was unidentified. The exact `go-licenses v2.0.1` scanner identifies the
   production dependency set as allowed MIT/BSD-3-Clause. A Go 1.26.5 smoke
   read back WAL, foreign keys, and the busy timeout, and production programs
   cross-built for all three Phase 1 targets. P1 still correctly requires full
   migration, locking, restart, and future-schema runtime evidence.
2. Materialization now uses a versioned deterministic canonical manifest,
   database pre-registration, SHA-256 integrity digest, exact promotion rule,
   and quarantine for all ambiguous cases. The plan explicitly denies an
   authenticity claim against a same-user attacker and defers authenticated
   production materialization to the security-owned Vault feature. Negative
   tests cover version, digest, revision, and missing-pending-row failures.

## Review coverage

- Scope and non-goals: complete and aligned with Phase 1.
- Module ownership: one `core` owner with explicit secondary impacts.
- Public contracts: versioned, bounded, authenticated, authorized, and
  idempotent where required.
- Failure/recovery: fail closed for endpoint, schema, process, and materialized
  state ambiguity.
- Security/privacy: peer identity is cryptographic; endpoint permissions are
  only narrowing; capability and lease checks are independent; logging and
  resource limits are explicit; residual host risk remains visible.
- Migration/compatibility: forward-only schema and protocol behavior, no
  downgrade or real Provider/Windows 11 inference.
- Test strategy: unit, contract, adversarial, failure-injection, subprocess,
  native-IPC, and three-platform E2E evidence are required.
- Rollback: correction PRs and future-schema refusal preserve data.
- Phase order: P1 through P5 are independently verifiable and sequential.

## Non-blocking builder notes retained

1. Revalidate the connection's client identity revision on every mutation or
   provide an equivalent immediate revocation mechanism.
2. Give every documented resource bound a named constant and direct test.
3. Keep offline recovery separate from authenticated online rotation and
   require exclusive Daemon ownership.
4. Reuse the Windows Spike control requirements, not its probe-only harness.

## Evidence

- Complete revised Feature Brief, design, contracts, tests, and development log.
- Prior `REVISE` report and exact closure diff.
- Go 1.26.5 modernc SQLite runtime smoke and three-platform production builds.
- Exact `go-licenses v2.0.1 check/csv` dependency comparison.
- `npm run project:verify` and `git diff --check` after revision.

## Gates and blockers

- Provider Gate: none; Fake Provider only.
- Security Gate: open and mandatory after final feature verification.
- Current blockers: none.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: The revised five-phase Device Kernel plan is executable; the SQLite dependency is fully recognized by governance and fake materialization now has explicit integrity-only recovery semantics.
**Findings**: No blocking findings; retain the four builder notes on identity revision, named bounds, offline recovery, and Windows probe isolation.
**Evidence**: Revised feature artifacts, prior finding closure, Go 1.26.5 SQLite smoke/cross-builds, exact license scan, project verification, and diff integrity.
**Blockers**: none

### Next Step

Run `feature-build` for `phase1-device-kernel` P1 only.
