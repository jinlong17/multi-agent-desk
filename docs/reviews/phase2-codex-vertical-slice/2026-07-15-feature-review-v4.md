# Feature Review v4: Codex Vertical Slice

- Date: 2026-07-15
- Role: `feature-review`
- Target: `phase2-codex-vertical-slice`
- Plan version: `v0.4`
- Owner module: `provider`
- Impacted modules: `core`, `security`, `project-system`
- Verdict: **REVISE**

## Verdict

**REVISE.** Revision v0.4 materially resolves the v0.3 findings around the
portable encrypted-record format, shared app-server lifecycle, daemon-owned
thread/turn policy, exact Provider Approval results, and Resume's no-mutation
exit. The scope, owner, evidence limits, phase order, compatibility posture, and
final Security Gate are also sound.

P2B is still not executable without two security-state decisions. The plan does
not define how an empty database becomes an initialized password-derived Vault:
`0005` is said to create a password-bound key-check row, but a migration has no
password, and the local API exposes only unlock. Separately, `cancel` is enabled
and mapped to the exact Provider result, but is excluded from the durable
dispatch claim and successful terminal transition. Because `0005` owns both
schemas, those decisions must return to `feature-plan` before the migration is
written.

## Independent classification

```text
Owner: provider
Confidence: high
Why: the outcome is the Codex app-server auth/session/Approval/Usage vertical slice and its exact compatibility gates
Impacts: core (Vault, Store, migration, daemon runtime); security (password-derived key lifecycle, secret transactions, Approval ambiguity); project-system (workflow evidence)
Branch: codex/provider/phase2-codex-vertical-slice
Workflow: feature
Gates: independent review before P2B; P2B -> P3A -> P3B; exact credentialed Linux evidence in P3B; security-review before Ship
```

The owning module remains exactly `provider`. `core`, `security`, and
`project-system` are secondary impacts, not additional owners.

## Review scope

I independently read the repository instructions, feature-review role,
implementation plan, project workflow and module registry, all v0.4 feature
artifacts, the v0.3 review, ADR 0014, the Provider Compatibility Matrix, Codex
auth/schema evidence, and the relevant current migration, Store, Vault, runtime,
IPC, materialization, protocol, ProviderSession, Approval, and Resume code.

The review covers scope, ownership, trust boundaries, Vault crypto/storage/key
lifecycle, enrollment, atomic CAS, migration/rollback, shared runtime ownership,
daemon authority, Approval decisions/results, conversation controls, failure
recovery, compatibility, testing, phase order, and workflow legality. It does not
modify plan, implementation, compatibility, evidence, or dashboard files.

## v0.3 finding closure

| v0.3 finding | v0.4 result |
|---|---|
| Vault v1 crypto/storage/platform/enrollment | **Mostly resolved.** Exact Argon2id/AES-256-GCM envelope, AAD, bounds, atomic item/revision CAS, portable three-platform backend, official-login enrollment, forward-only migration, and old-binary refusal are frozen. First-use Vault initialization remains undefined. |
| CredentialRuntime versus SessionBinding | **Resolved.** One runtime per CredentialInstance owns the child, materialization, reader, and final CAS; bindings own thread/turn/Approval/event state; refcount, binding stop/kill, last release, and crash fan-out are explicit and tested. |
| Daemon-owned capability/thread/turn policy | **Resolved.** Caller authority-bearing overrides are removed; `codex.v1` fields, bounds, workspace resolution, sandbox/approval policy, input conflict, disabled steer, and cross-thread failure are explicit. |
| Exact Approval mapping, including permissions and cancel | **Partially resolved.** Command/file wire results and disabled permissions behavior are exact. Cancel's durable claim/terminal state is still contradictory. |
| Resume exit | **Resolved.** Typed `provider_resume_unsupported` is the accepted Phase 2 exit and must produce zero Provider frame, zero local Session, and zero state transition. |

## Ranked findings

### P0 — Vault v1 has no first-use initialization transaction

Evidence:

- `design.md:167-176` says forward migration `0005` creates one `vault_config`
  row containing a random salt and AES-GCM key-check envelope, then defines only
  how unlock decrypts that envelope. A schema migration cannot create a
  password-derived key-check without receiving the user's password.
- `api.md:54-56` makes the key-check fields part of `VaultConfigV1`, while the
  authenticated local surface at `api.md:83-95` and the Vault contract at
  `api.md:396-418` expose lock/unlock and item CAS but no initialize operation,
  absent-config rule, or atomic config-creation transition.
- `test.md:24-27` mentions key-check creation and portable round trips but does
  not state whether first unlock initializes, how confirmation is handled, how
  two initializers race, or what survives a crash during config creation.
- The current `internal/vault/vault.go:16-47` is intentionally lock-state only,
  and the repository has migrations only through `0004`. There is no existing
  lifecycle implementation that removes this decision from P2B.

Impact: the next writer must choose whether `0005`, first unlock, a new init
command, or enrollment creates the Vault; whether an absent or partial singleton
means uninitialized, corrupt, or locked; and how concurrent/crashed first use is
recovered. These choices determine whether the first credential can ever be
encrypted and whether an attacker or racing client can bind the database to an
unexpected password.

Required revision:

1. Define an explicit `uninitialized -> initialized/locked -> unlocked` state
   machine and exactly one authenticated local initialization operation. If
   first unlock initializes, name that behavior explicitly and distinguish it
   from normal unlock without leaking wrong-password/tamper detail.
2. Specify the single SQLite transaction that creates the singleton salt,
   bounded Argon2 parameters, fresh key-check nonce/ciphertext, and initialized
   marker; use insert-if-absent/CAS so exactly one concurrent initializer wins.
3. Define crash/restart outcomes for no row, a committed valid row, duplicate
   rows, missing/partial fields, and a key-check that fails authentication. The
   migration should create schema only unless a password-independent valid row
   format is explicitly selected.
4. Add first-init, duplicate-init, concurrent-init, crash-before/after-commit,
   restart, wrong-password, corrupt-singleton, and three-platform tests. State
   that password change/rekey is either implemented with its own atomic contract
   or explicitly outside Phase 2.

### P1 — Approval `cancel` is wire-mapped but absent from durable dispatch state

Evidence:

- `api.md:294-296` permits local `cancel`, and `api.md:315-316` maps it to the
  exact `{"decision":"cancel"}` result for command/file requests.
- The same contract restricts `requested_decision` to `approve | deny` at
  `api.md:281-284`, and its successful write transition to
  `approved|denied/written` at `api.md:303-307`.
- `design.md:282-291` likewise claims a dispatching row stores the decision but
  says only approved/denied can finalize. The acceptance text enables cancel
  without defining its successful local status, stored idempotent result, or
  migration constraint.
- `test.md:34-37` checks the exact Provider wire result and general terminal
  behavior, but does not assert cancel's durable claim, successful terminal row,
  duplicate ACK, or restart state.

Impact: `0005` must add the Approval dispatch columns and constraints, so P2B/P3A
would otherwise invent whether cancel is stored as `cancelled/written`,
`denied/written`, or another state. That ambiguity affects idempotency, restart
recovery, audit meaning, and the response returned to the controller.

Required revision: include `cancel` in `requested_decision`; select one explicit
successful terminal transition (preferably `pending/dispatching ->
cancelled/written`); define its stored response and duplicate-key behavior; and
repeat the same state in the brief, design, API, migration constraints, and tests.
Permissions Approval should remain disabled with no Provider write, as v0.4
already specifies.

## Assessment of remaining areas

- **Scope and ownership:** correct. Web/Desktop/Control Plane, E2EE Credential
  Grants, Claude, release, and real Windows Codex support remain outside this
  feature.
- **Security and secret handling:** the selected portable password-derived
  backend, strict KDF bounds, fresh DEK/nonces, canonical AAD, 64-KiB object
  bound, secret-free IPC/logs, staging validation, best-effort zeroization, and
  quarantine posture are coherent once initialization is defined.
- **Atomicity and rollback:** Vault item plus CredentialInstance CAS and
  before/after-commit authority are clear. `0005` is truthfully forward-only;
  feature disablement preserves encrypted rows, old binaries refuse the schema,
  and recovery is backup/restore or a newer binary.
- **Enrollment:** exact `codex login`, owner binding, one-active operation,
  deadline/idempotency, forbidden credential flags, validation-before-import,
  restart expiry, cleanup, and no remote-revocation claim are sound. First use
  still depends on Finding 1.
- **Runtime and controls:** CredentialRuntime/SessionBinding separation now
  matches ADR 0014. One reader, response/request routing, binding-scoped
  interrupt/stop/kill, last-reference shutdown, daemon-close cleanup, typed
  resize unsupported, and no-mutation Resume are non-contradictory.
- **Provider compatibility:** v0.4 preserves exact canonical macOS evidence,
  treats Approval as a server request, disables unproven permission grants and
  older actionable rows, and retains exact credentialed Linux Session/Approval/
  Usage/second-CLI evidence as a P3B gate. It does not infer multi-writer,
  completed device-auth, 48-hour, or Windows Codex support.
- **Testing and order:** P2B -> P3A -> P3B is correct, and the suite is broad
  across crypto, concurrency, routing, failures, controls, Fake regression, and
  platforms. Add only the initialization and durable-cancel cases above before
  P2B is approved. Final `security-review` remains mandatory before Ship.

## Checks

- Read-only inspection confirmed the branch/worktree and that current code still
  has a lock-state-only Vault, no `0005`, disabled daemon Codex start, no auth
  enrollment, per-Session Fake runtime ownership, metadata-only Approval response,
  and the pre-v0.4 ProviderSession implementation. These are correctly planned
  implementation gaps, not evidence that the revised runtime design is wrong.
- `git diff --check` is run after the verdict write.
- This review writes only this report and the target dev log's Status Panel plus
  one append-only Work Log row.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: `v0.4 resolves the shared-runtime, daemon-policy, exact Provider mapping, and Resume findings, but P2B still lacks a first-use Vault initialization transaction and a complete durable state for Approval cancel.`
**Findings**: `P0: define atomic first-use Vault initialization, concurrency, crash recovery, and tests; P1: carry cancel through requested_decision, successful terminal state, idempotent stored result, migration constraints, and tests.`
**Evidence**: `v0.4 brief/design/api/test/dev_log; v0.3 review; implementation plan; workflow/registry; ADR 0014; Provider compatibility and auth/schema evidence; current migrations, Vault, Store, runtime, IPC, materialization, protocol, ProviderSession, Approval, and Resume code; git diff --check.`
**Blockers**: `The plan must resolve the ranked P0/P1 findings before feature-build P2B; exact credentialed Linux P3B evidence and final security-review remain later gates.`

### Next Step

Run `feature-plan` for `phase2-codex-vertical-slice`.
