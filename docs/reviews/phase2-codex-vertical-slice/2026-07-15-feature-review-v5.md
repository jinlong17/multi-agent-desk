# Feature Review v5: Codex Vertical Slice

- Date: 2026-07-15
- Role: `feature-review`
- Target: `phase2-codex-vertical-slice`
- Plan version: `v0.5`
- Owner module: `provider`
- Impacted modules: `core`, `security`, `project-system`
- Verdict: **APPROVED**

## Verdict

**APPROVED.** Revision v0.5 closes both findings from the independent v0.4
review and preserves every previously resolved boundary. The next approved
writer phase, P2B only, is executable without inventing a Vault bootstrap,
idempotency, migration, Approval-cancel, runtime, Provider, rollback, or workflow
decision.

Approval is limited to the plan. It is not implementation verification, Linux
Provider evidence, Security Gate acceptance, Ship authorization, or a release
claim. P2B must stop at `READY_FOR_VERIFY`; P3A remains dependent on verified
P2B and P3B remains dependent on an exact credentialed Linux environment.

## Independent classification

```text
Owner: provider
Confidence: high
Why: the outcome is the Codex app-server auth/session/Approval/Usage vertical slice and its exact compatibility gates
Impacts: core (Vault, Store, migration, daemon runtime); security (password-derived key lifecycle, secret transactions, Approval ambiguity); project-system (workflow evidence)
Branch: codex/provider/phase2-codex-vertical-slice
Workflow: feature
Gates: Phase 0.5 exact-version evidence; P2B -> verify -> P3A -> verify -> P3B; exact credentialed Linux evidence in P3B; security-review before Ship
Docs: docs/reviews/phase2-codex-vertical-slice/2026-07-15-feature-brief.md; docs/workflow/features/phase2-codex-vertical-slice/dev_log.md
```

`provider` remains the single owner. The substantial Vault/Store/runtime,
security, and workflow effects remain secondary impacts and do not create a
second owner.

## Review scope

I freshly read the repository and role instructions, project workflow and module
registry, current implementation plan, all v0.5 feature artifacts and authority
state, the v0.4 REVISE report, ADR 0014, the Provider Compatibility Matrix and
schema-reconciliation evidence, plus the relevant current migration, Vault,
Store, local IPC/idempotency, runtime, materialization, protocol,
ProviderSession, Approval, and Resume code.

The review covers both v0.4 findings and the full plan surface: scope, ownership,
trust boundaries, crypto/key lifecycle, enrollment, migration/rollback,
atomicity, concurrency, corruption/restart behavior, shared process ownership,
daemon authority, exact Provider methods/results, control semantics, failure
recovery, compatibility, testing, phase order, and workflow legality. It does
not inherit the planner's approval claim and does not modify plan,
implementation, compatibility, evidence, or dashboard files.

## v0.4 finding closure

### Closed — first-use Vault initialization is now an executable transaction

The contract is consistent across the brief, design, API, tests, implementation
plan, and dev log:

- `0005_codex_vault_and_approval_dispatch.sql` creates empty schema and
  constraints only. No migration attempts to create password-bound data.
- Vault state is explicitly `uninitialized | locked | unlocked`.
  Authenticated `vault.initialize` is the sole first-use transition and moves
  `uninitialized -> locked`; normal unlock is separate.
- The CLI/UI collects two password entries and compares them locally. A mismatch
  emits no IPC request. Only one matched 1..1024-byte value crosses authenticated
  local IPC, and the password/body is excluded from generic idempotency and
  audit persistence.
- The daemon derives the fresh salt, bounded Argon2id KEK, and AES-GCM key-check
  before one `BEGIN IMMEDIATE` insert-if-absent transaction. The singleton stores
  only initializer identity and SHA-256 of length-prefixed client identity plus
  random request key, never a password/body digest or verifier.
- Same-client/same-key retry returns the stored locked success without
  reinitializing or unlocking. Exactly one competing request wins; other callers
  receive `vault_already_initialized` and use normal unlock.
- Crash before commit remains uninitialized; crash after commit restarts locked.
  Missing, valid, partial/duplicate/invalid, and key-check-authentication states
  have distinct fail-closed classifications without revealing wrong password
  versus tamper.
- Initialization refuses existing Codex secret/revision, enrollment, or binding
  state while preserving Fake metadata compatibility. Password change/rekey is
  explicitly outside Phase 2 and requires a later atomic contract.
- The acceptance matrix now tests local confirmation, lost response, same and
  competing retries, both crash boundaries, restart, corrupt singleton,
  dependent state, Fake behavior, no password/body digest, no-rekey, and the
  portable macOS/Linux/Windows backend.

This closes v4 P0. The current lock-state-only Vault and schema through `0004`
are accurately treated as P2B implementation work rather than an unstated
dependency.

### Closed — Approval cancel now has one durable claim and terminal result

The Approval contract now carries `cancel` through every layer:

- `requested_decision` is `approve | deny | cancel`.
- A lease-authorized response claims `pending/idle -> pending/dispatching` with
  the stored decision before any Provider write.
- Exact 0.144.2 command/file responses remain `accept | decline | cancel`;
  permissions, `acceptForSession`, policy amendments, and older unproven rows
  remain disabled with no Provider write.
- A successful cancel response commits the exact pair
  `(cancel, cancelled)` with `response_state=written` and responded metadata.
  `0005` constrains all three successful decision/status pairs.
- An identical idempotency key and dispatch digest returns the stored
  `{decision,status,response_state,provider_dispatched:true}` result, including
  `cancel/cancelled/written`; a different digest conflicts.
- Write/process/request-ID uncertainty and restart from any dispatching decision
  becomes `expired/ambiguous` and is never replayed.
- Tests cover the migration constraints, exact wire result, stored cancel ACK,
  different-digest conflict, response-write failure, child exit, restart, and
  cross-binding denial.

This closes v4 P1 without weakening ControllerLease or ambiguity semantics.

## Recheck of prior resolved findings

- **CredentialRuntime / SessionBinding:** still coherent with ADR 0014. One
  runtime per CredentialInstance owns the child, canonical auth home,
  materialization lease, one-reader multiplexer, refcount, and final CAS; each
  Session binding owns thread/turn/Approval/event state. Binding stop/kill cannot
  terminate a shared child; last release and crash fan-out finalize once.
- **Daemon-owned policy:** callers cannot supply binary/version/fingerprint,
  Account, capability, or workspace authority. The daemon validates the bounded
  `codex.v1` profile, rejects unknown fields and `danger-full-access`, constructs
  the exact `thread/start`/`turn/start` allowlist, and fails cross-thread or
  unsupported control before a Provider write.
- **Input/resize/stop:** idle input starts a bounded turn, active-turn input
  conflicts while steer is disabled, conversation resize is typed unsupported,
  and the nonexistent `session/stop` method is not invented.
- **Resume:** typed `provider_resume_unsupported` remains the accepted Phase 2
  exit when the live test proves zero Provider frame, zero new local Session,
  zero source transition, and no false history-recovery claim.
- **Evidence fidelity:** exact macOS canonical schema rows and actual Approval
  server-request methods remain the only current capability evidence. The plan
  preserves single-writer CAS, interactive-login fallback, and no multi-writer,
  completed-device-auth, 48-hour, unrecorded-Linux, or real-Windows-Codex claim.
- **Enrollment/security:** exact local `codex login`, private staging,
  owner/idempotency/deadline state, validation-before-import, secret-free IPC,
  atomic Vault item/revision CAS, cleanup, and no remote-revocation promise remain
  explicit.
- **Rollback/failure:** `0005` is truthfully forward-only; old binaries refuse
  it, current feature disablement retains readable ciphertext/Fake behavior, and
  recovery is backup/restore or a newer binary. Quarantine, stale-CAS rejection,
  writer conflict, malformed protocol, pending-call failure, and ambiguous
  Approval behavior remain fail closed.
- **Testing/order/workflow:** the deterministic suite covers crypto, bounds,
  initialization, item CAS, enrollment, shared runtime, concurrent JSON-RPC,
  controls, Approval, Resume, Fake regression, and three-platform posture. The
  legal order is P2B build/verify, P3A build/verify, P3B live verification, then
  P4 and final security-review before any human-authorized Ship.

## Findings

None. No remaining plan-level decision blocks P2B implementation.

## Checks

- Read-only source inspection confirmed the current implementation gaps named by
  the plan: no `0005`, no production encrypted Vault or enrollment path, daemon
  Codex start disabled, current per-Session Fake runtime, and Approval persistence
  without Provider dispatch. The v0.5 phase boundaries assign those gaps without
  treating future implementation as present evidence.
- `git diff --check` is run after the verdict write.
- This review writes only this report and the target dev log's Status Panel plus
  one append-only Work Log row.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: `v0.5 closes the first-use Vault and durable Approval-cancel findings and preserves the shared-runtime, daemon-policy, Provider-evidence, Resume, rollback, testing, and workflow boundaries; P2B is executable without unstated decisions.`
**Findings**: `none`
**Evidence**: `v0.5 brief/design/api/test/dev_log; v0.4 review; implementation plan; workflow/registry; ADR 0014; Provider compatibility and auth/schema evidence; current migrations, Vault, Store, IPC/idempotency, runtime, materialization, protocol, ProviderSession, Approval, and Resume code; git diff --check.`
**Blockers**: `none for feature-build P2B; exact credentialed Linux evidence remains a later P3B gate and final security-review remains required before Ship.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice`.
