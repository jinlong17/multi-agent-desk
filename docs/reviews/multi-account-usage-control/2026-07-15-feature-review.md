# Feature review: 多账号用量看板与显式调用 — APPROVED (plan v0.3 rereview)

## Verdict

`APPROVED` for P1 only: manual Account/Profile/alias registry, forward
migration, stored generic Usage contracts, authenticated local IPC/CLI, and
explicit Profile preview binding without a real Provider launch.

Plan v0.3 closes all three findings from the initial `REVISE` verdict. The P1
builder can now implement the next phase without deciding how Fake data maps,
whether metadata creation allocates credentials/Homes, or how update,
pagination, and deletion behave.

This approval does not resolve either Provider Spike or the feature Security
Gate. P2/P3 real login, Home allocation, quota collection, Provider process
launch, and any stable Claude subscription surface remain gated.

## Finding closure

### [CLOSED P0] Internal Fake migration

`design.md` now reserves `fake` as internal-only migration state. It freezes one
deterministic synthetic Account per Device, preserves all existing Profile,
CredentialInstance, and Session IDs, assigns null aliases to internal Profiles,
backfills `sessions.account_id` without changing the Profile/Credential tuple,
and keeps `run fake` on its legacy explicit-ID surface.

The table rebuild and recovery contract is executable: preflight, fixed
parent/child copy and swap order, connection-scoped foreign-key suspension
outside the transaction, count/relationship checks, `foreign_key_check`,
ledger idempotence, rollback, and restart/crash expectations are specified.
`test.md` covers multiple Fake rows, collisions/check failures, restart, exact
tuple preservation, and the existing native Fake Session scenario.

### [CLOSED P0] P1 creation, auth-state, and Home ownership

P1 creation is now exactly one SQLite transaction that creates one public
Account and one default RuntimeProfile at revision 1. It creates no
CredentialInstance, Vault item, Provider Home, Keychain item, directory, or
Provider process. `login_required`/`unknown`/null validation time are derived
views for the missing credential, so filesystem rollback is not invented.

Home allocation is explicitly deferred to P2/P3 and distinguishes Codex
`owner_kind=credential_instance` from Claude `owner_kind=runtime_profile`,
preserving ADR 0014 and ADR 0016. P1 tests require an unchanged filesystem
snapshot and capability-unavailable results for all real Provider operations.

### [CLOSED P1] CRUD, pagination, revision, and deletion

`api.md` now exposes Account/Profile update commands, expected revisions, and
bounded limit/cursor arguments for both lists. It freezes ascending
`(created_at,id)` keyset ordering, cursor filter binding, malformed/reused
cursor errors, public filtering of internal rows, and monotonic revision
semantics.

P1 deletion is an atomic metadata transaction with disabled-state and expected-
revision preconditions, Session/materialization checks, minimal tombstones,
alias release after commit, internal Fake protection, and fail-closed
`provider_cleanup_required` for an unexpected credential. CredentialGrant is
truthfully deferred: P5 must install its delete-side reference protection
before enabling Grant creation, and P1 makes no Grant cleanup claim.

## Scope, security, and phase ordering

- The feature remains Provider-owned with explicit impacts on Core, Web,
  Desktop, Security, Control Plane, and Project System.
- P1 maps metadata reads to the shipped `metadata.read` capability and
  mutations to `client.admin`; it does not rewrite existing client identities
  or prematurely introduce unusable fine-grained capabilities.
- Alias canonicalization, expected revisions, immutable confirmation tuples,
  and explicit capability-unavailable errors keep account selection fail-
  closed. No automatic rotation, fallback to another Profile, or Fake
  fallthrough is authorized.
- Generic `UsageWindow[]`, optional values, source/confidence/freshness, and
  unknown-kind round-trip remain truthful and Provider-neutral.
- P0 Spikes may proceed independently. P2 requires the distinct-account Codex
  decision plus ADR 0014; P3 requires the Claude identity/usage/policy decision
  plus ADR 0016. P4/P5 remain downstream and gated.
- P1 stops at `READY_FOR_VERIFY`; this review does not authorize ship, merge,
  push, Provider compatibility changes, or Security Gate closure.

## Evidence and checks

- Governance and role: `AGENTS.md`, `CLAUDE.md`, implementation plan, workflow
  policy, module registry, and `.agents/roles/feature-review.md`.
- Plan v0.3: Feature Brief and
  `docs/workflow/features/multi-account-usage-control/{design,api,test,dev_log}.md`.
- Provider boundaries: both new Spike intakes, resolved predecessor Spike logs,
  ADR 0014, ADR 0016, and `docs/PROVIDER_COMPATIBILITY.md`.
- Shipped baseline: Device migrations, Store migration runner, domain types,
  authorization mapping, repositories, and Session service on `cb93c02`.
- Fresh checks before this verdict: workflow verification passed (10 Agents,
  3 Skills, 17 docs, 20 edges, 15 statuses); dashboard static verification
  passed (9 phases); 207 local Markdown files passed link validation;
  `git diff --check` passed.
- This rereview changes only this report and the parent feature `dev_log.md`.

## Handoff

**Target**: `multi-account-usage-control`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: `Plan v0.3 closes the Fake migration, metadata-only creation/Home, and CRUD/pagination/delete findings; P1 is executable while all real Provider and Security gates remain open.`
**Findings**: `none; the three prior findings are closed by the frozen v0.3 contracts.`
**Evidence**: `Feature Brief; plan v0.3 design/api/test/dev_log; child and predecessor Spikes; ADR 0014/0016; Provider compatibility; shipped Store/domain/auth/session baseline; workflow/dashboard/link/diff checks.`
**Blockers**: `none for P1; P2/P3 remain gated and are not authorized by this verdict.`

### Next Step

Run `feature-build` for `multi-account-usage-control` P1.
