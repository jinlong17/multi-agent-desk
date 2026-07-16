# Feature Review: Codex Vertical Slice

- Date: 2026-07-15
- Role: `feature-review`
- Target: `phase2-codex-vertical-slice`
- Owner module: `provider`
- Verdict: **REVISE**

## Review scope

I independently reviewed the Feature Brief, `design.md`, `api.md`, `test.md`,
and `dev_log.md` against the Phase 2 implementation plan, Phase 1 Device
Kernel contracts, ADR 0014, the Codex Spike artifacts, the Provider
Compatibility Matrix, and the current SQLite/domain implementation. The
review did not modify plan files, implementation files, dashboard judgment, or
the Phase 0.5 evidence.

## Positive findings

- Ownership is correctly routed to `provider`; `core` and `security` impacts
  are explicit and Web/Desktop/Control Plane are kept out of the vertical-slice
  exit.
- The plan faithfully carries ADR 0014's conservative boundary: one canonical
  refresh writer, revisioned CAS, quarantine on ambiguity, interactive-login
  fallback, and no multi-writer/48-hour/device-auth completion claim.
- Version/schema gating, bounded JSONL framing, fail-closed unknown versions,
  secret-safe fixtures, lease-bound Approval mutation, and Windows' explicit
  “no real Codex support claim” are strong and testable directions.
- Phase ordering, rollback intent, and the requirement for a real Linux exit
  are appropriate. The plan does not mistake Phase 0.5 compatibility evidence
  for production implementation evidence.

## Findings

### P0 — The current Device schema and Store cannot represent Codex

Evidence:

- `migrations/device/0001_device_identity.sql:35-50` constrains both
  `runtime_profiles.provider` and `credential_instances.provider`/`auth_method`
  to `fake`.
- `migrations/device/0002_sessions.sql:1-5` constrains `sessions.provider` to
  `fake`.
- `internal/storage/repository.go:272-290` and `:324-347` reject every
  non-fake profile or CredentialInstance before SQLite is reached.
- `internal/runtime/manager.go:75-101` creates only `Provider: "fake"` and
  invokes `StartFake`.

Impact: the advertised P2/P3 path cannot create a Codex Profile,
CredentialInstance, or Session from the current main schema. `api.md` says a
database migration is not planned unless later proven necessary, but this is
already a reproducible necessity. Approving the plan would force the builder
to invent an unreviewed persistence migration and compatibility policy.

Required revision:

1. Add an explicit schema/domain compatibility phase before P2/P3 (or make P1
   explicitly include it) that replaces the fake-only checks while preserving
   Fake Provider behavior.
2. Define allowed Provider/AuthMethod values, upgrade/future-schema refusal,
   forward/backward/restart tests, and rollback behavior for existing Phase 1
   fake data.
3. Update the API, test matrix, Phase Plan, and acceptance criteria so the
   Codex Session can be created through the real Store and Device IPC path.

### P0 — Approval and Usage surfaces are not present in the Phase 1 contract

Evidence:

- `internal/app/authorization.go:20-42` has no capability or method for
  `approval.respond`, Usage, or Rate Limits.
- `internal/app/session_service.go:55-62` and `:103-173` have no idempotency or
  dispatch path for Approval response, Usage, or Rate Limits.
- `cmd/multidesk/commands.go:119-131` accepts only `run fake`; there is no
  Codex start/Usage/Approval CLI surface.
- The domain/storage schema has runtime events but no pending Approval request,
  response result, or UsageSnapshot contract.

Impact: the plan's headline exit (“second local CLI ... respond to Approval” and
“inspect Usage/Rate Limits”) is not executable from the current authenticated
local CLI. The proposed `ApprovalRequest`/`UsageSnapshot` types are semantic
sketches, but they do not define capability registration, request routing,
pending-request lifetime, durable versus in-memory state, or replay behavior.

Required revision:

1. Add the exact local IPC methods/capabilities and CLI commands, including
   authorization and request-bound idempotency.
2. Decide and document whether pending Approvals and Usage snapshots are
   persisted, bounded in memory, or both; add the required migration/retention
   and restart semantics.
3. Add synthetic Approval fixtures and a supported Codex schema mapping before
   P3, with negative tests proving observers and stale leases cannot respond.

### P1 — The materialization dependency is named but not contractually mapped

Evidence:

- ADR 0014 requires a `CredentialMaterializationManager`, but the current code
  exposes `vault.Materializer` (`internal/vault/materialization.go:40-158`).
- The Phase 1 materializer writes `credential.fake` from caller-provided bytes;
  it does not read a Provider credential through a Vault API, manage
  `CODEX_HOME/auth.json`, validate Codex structure, or CAS a Provider mutation
  back into a real secret store.
- The plan's P2 dependency says “Vault and CredentialMaterializationManager
  contract” while also treating the existing boundary as sufficient.

Impact: a builder must decide ownership, secret input, lease duration,
materialized-home lifecycle, and CAS/recovery semantics while writing code. It
could also accidentally reuse the fake materializer with raw credential bytes,
contrary to the security review.

Required revision: define the provider/core interface before P2: the secret
source and non-serializable handle, writer lease acquisition/renewal/release,
auth-file digest/structure validation, revision commit, quarantine state, and
process shutdown ordering. Explicitly state which Phase 1 fake materializer
behavior remains test-only.

### P1 — Resume semantics are not frozen enough for an acceptance verdict

`design.md` lists `Resume` as a P3 acceptance, but its open decisions leave
“reuse Provider history or start a new app-server turn” unresolved. Phase 1's
`runtime.Manager.Resume` currently creates a new Fake process and only preserves
the domain source linkage. That is not evidence for Codex history/session
resume. Before P3 is approved, the plan must select a normative Codex resume
contract, state what survives an app-server restart, and add a fixture/live
scenario whose pass/fail result is unambiguous.

### P1 — Phase status is inconsistent across authority documents

The feature plan and `dashboard-state.json` treat Phase 1 as shipped, and
`docs/workflow/features/phase1-device-kernel/dev_log.md` is `SHIPPED`; however,
`docs/IMPLEMENTATION_PLAN.md:1148` still says `Phase 1` status is `ACTIVE`.
This does not invalidate the code, but it creates a project-system truth drift
at the exact dependency boundary used by this feature. Reconcile the baseline
before feature-build and record which document owns the current phase label;
otherwise future dashboard/plan checks can reintroduce a false gate.

## Security and compatibility assessment

- The Provider Spike and ADR 0014 evidence are sufficient for the constrained
  single-writer design decision, not for the implementation acceptance.
- The plan correctly keeps Linux schema replay open and does not overclaim
  Windows Codex support. Those gates should remain open after revision.
- The secret-safety and quarantine requirements are directionally strong, but
  they cannot be verified until the materialization and IPC contracts above are
  concrete.

## Verdict

**REVISE.** The plan is well-scoped and security-conscious, but the two P0
findings are reproducible current-repository incompatibilities, not optional
implementation details. `feature-plan` must add the persistence/migration and
Approval/Usage contract phases, reconcile the materialization boundary, freeze
resume semantics, and resolve the Phase 1 status drift before a builder can
execute the next phase without inventing decisions.

## Evidence

- `docs/reviews/phase2-codex-vertical-slice/2026-07-15-feature-brief.md`
- `docs/workflow/features/phase2-codex-vertical-slice/{design,api,test}.md`
- `docs/IMPLEMENTATION_PLAN.md` §§8, 19
- `docs/adr/0014-codex-app-server-single-writer-auth.md`
- `docs/PROVIDER_COMPATIBILITY.md`
- `docs/spikes/codex/app-server-account-matrix.json`
- `docs/spikes/codex/two-device-short-run.json`
- `migrations/device/0001_device_identity.sql`, `0002_sessions.sql`
- `internal/storage/repository.go`, `internal/runtime/manager.go`,
  `internal/vault/materialization.go`, `internal/app/authorization.go`,
  `internal/app/session_service.go`, `cmd/multidesk/commands.go`
- `docs/workflow/features/phase1-device-kernel/dev_log.md`

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: `The provider boundary and evidence discipline are sound, but the current fake-only SQLite/Store schema and missing Approval/Usage IPC surfaces make the planned Codex exit non-executable without new decisions.`
**Findings**: `P0: add the Codex-compatible domain/SQLite migration and Store path; P0: define and implement the Approval/Usage local IPC, capability, persistence, and CLI contract; P1: map the real materialization manager boundary, freeze resume semantics, and reconcile Phase 1 status drift.`
**Evidence**: `Current main migrations and Store validators, Phase 1 runtime/app/device/vault code, ADR 0014, Codex compatibility artifacts, and all Phase 2 plan files.`
**Blockers**: `No external blocker; the plan must be revised by feature-plan before feature-build.`

### Next Step

Run `feature-plan` for `phase2-codex-vertical-slice`.
