# Feature Review v2: Codex explicit multi-account selector

- Date: 2026-07-16
- Target: `codex-multi-account-selector`
- Reviewed commit: `1e0d5f8`
- Prior verdict: `REVISE` at `3e2ee90`
- Verdict: **APPROVED**
- Next executable phase: **P1 only**

## Conclusion

Plan v2 closes every ranked v1 finding without weakening the original security
or Provider boundaries. P1 is executable without inventing a preview authority,
debug capability, raw-ID bypass, or impossible cross-system atomicity claim.

The next build is limited to P1: migration 7, persistent one-time Session
previews, enrollment `awaiting_confirmation`, alias-aware auth contracts, the
sole preview/confirm application start path, safe errors/audit, and synthetic
tests. It does not authorize live P2 Account execution, broaden exact Linux
`0.144.2` support, accept macOS/Windows, close the feature Security Gate, or
push/merge/ship.

## Closure of v1 findings

### P1 preview authority — closed

Migration 7 now owns random `session_start_previews` rows bound to authenticated
client, exact tuple/revisions/workspace, Usage selection, compatibility
fingerprints, expiry, and consumption state. One storage transaction validates
and consumes the row while inserting the starting Session. Same-request lost-
response replay returns the recorded Session; forged, cross-client, expired,
or differently consumed previews fail. Restart/cleanup and the adversarial
race/replay matrix are specified.

### P1 raw-ID bypass — closed

There is one application start contract. Every CLI/IPC Codex start, including
the Phase 2 acceptance harness, obtains and consumes a preview. The old raw-ID-
only form returns `identity_confirmation_required` before Session insertion.
Only direct runtime-manager unit tests bypass the application layer, which is
not an authorization surface. No new capability, identity migration, debug
build, or distribution mode is invented.

### P2 compatibility timing — closed

The plan separates the external preflight before preview consumption from the
final runtime fingerprint check after Session reservation. Drift before the
transaction creates no Session; drift after reservation records a failed
Session, performs no credential commit, and releases materialization. Tests
assert both outcomes rather than claiming SQLite can atomically cover the
filesystem and spawned process.

## Full review result

- Ownership and module impacts: correct.
- Exact Provider/platform scope: correct and capability-gated.
- Explicit identity attestation: non-PII, owner/expiry/tuple bound, and not
  misrepresented as automated upstream identity proof.
- Migration and recovery: forward-only, versioned, restart-safe, and covered by
  preservation/rollback tests.
- Session authorization and TOCTOU: server-issued one-time preview plus exact
  transactional revalidation.
- Vault/materialization/logout: ADR 0014 single writer, revision CAS,
  quarantine, active-Session denial, and durable revocation reservation remain.
- Redaction/audit: safe allowlist and prohibited Provider/browser/content data
  are explicit.
- Testing: unit/property, storage/migration, authenticated IPC, A/B isolation,
  live exact-Linux, race, platform build, governance, and rollback coverage are
  proportionate.
- Phase ordering: P1 synthetic contract foundation precedes P2 live selector
  runtime and P3 platform/docs/Security closure.

## Conditions retained

- P1 stops at `READY_FOR_VERIFY` for independent verification.
- P2 starts only after P1 is `VERIFIED` and may require operator participation
  in official login; no hidden quota-only request is allowed.
- Exact Linux CLI `0.144.2` is the only accepted live Provider row.
- The feature Security Gate remains open through P3 and requires an independent
  Security Review before Ship.
- No automatic selection/rotation, mid-Session switch, raw Provider identity,
  credential copy, second writer, or public raw-ID start is permitted.

## Verification evidence

- Plan v2 `design.md`, `api.md`, `test.md`, and `dev_log.md`.
- Prior v1 review and each ranked file-specific finding.
- Feature Brief, parent P1 contracts, shipped Phase 2 runtime, ADR 0014,
  distinct-account Spike/Security Review, and compatibility matrix.
- Workflow verification, dashboard verification, local-link verification, and
  diff integrity passed after the v2 transition.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: `Plan v2 closes preview authority, raw-ID bypass, and compatibility timing findings; P1 contracts are executable.`
**Findings**: `none; prior P1/P2 findings are closed`
**Evidence**: `plan v2, prior review, Feature Brief, P1/Phase 2/ADR 0014/Provider evidence, workflow/dashboard/link checks`
**Blockers**: `none for P1; later live/platform/Security gates remain phase dependencies`

### Next Step

Run `feature-build` P1 for `codex-multi-account-selector`.
