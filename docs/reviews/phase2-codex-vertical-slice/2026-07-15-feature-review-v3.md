# Feature Review v3: Codex Vertical Slice

- Date: 2026-07-15
- Role: `feature-review`
- Target: `phase2-codex-vertical-slice`
- Plan version: `v0.3`
- Owner module: `provider`
- Impacted modules: `core`, `security`, `project-system`
- Verdict: **REVISE**

## Verdict

**REVISE.** Revision v0.3 is materially more truthful than the blocked P3 plan:
it identifies the absent production credential source, daemon runtime bridge,
single-reader multiplexer, Approval dispatch transaction, and typed Codex
conversation controls. It also preserves the narrow ADR 0014 and Compatibility
Matrix claims.

The next phase is nevertheless not executable without inventing decisions. P2B
defines interfaces for a production Vault and enrollment broker, but not the
versioned encrypted record, key/unlock lifecycle, atomic secret-plus-revision
transaction, supported platform boundary, or interactive transport/state machine.
P3A also contradicts ADR 0014 by combining a shared per-CredentialInstance
app-server with per-Session process ownership and a Stop path that terminates the
shared child. The ranked findings below must return to `feature-plan`.

## Independent classification

```text
Owner: provider
Confidence: high
Why: the outcome is a Codex app-server adapter, compatibility gate, auth/session/Approval/Usage mapping, and live Provider slice
Impacts: core (Vault runtime, Store, migration, daemon/session manager); security (encrypted credential storage, enrollment, Approval ambiguity, secret recovery); project-system (feature plan, ADR/compatibility and workflow evidence)
Branch: codex/provider/phase2-codex-vertical-slice
Workflow: feature
Gates: Phase 0.5 exact-version Provider evidence; open implementation Security Gate; P2B before P3A; exact Linux schema and credentialed environment before P3B; security-review before Ship
Docs: docs/reviews/phase2-codex-vertical-slice/2026-07-15-feature-brief.md; docs/workflow/features/phase2-codex-vertical-slice/dev_log.md
```

The `provider` owner is not displaced by the large `core` and `security`
impacts. The user-visible outcome and Provider compatibility authority remain
Codex-specific; the secondary modules own their respective runtime and trust
contracts.

## Review scope

I read the complete Feature Brief, `design.md`, `api.md`, `test.md`, and
`dev_log.md`; the full implementation plan; AGENTS/CLAUDE/workflow/module and
feature-review contracts; ADR 0014; the Provider Compatibility Matrix; the auth/
refresh and P3 schema-reconciliation evidence; and the current Device migration,
Store, Vault, runtime, local IPC/CLI, protocol client, ProviderSession, Approval,
and materialization code.

This review covers scope, ownership, trust boundaries, production Vault and
enrollment feasibility, JSON-RPC/process ownership, Approval dispatch, migrations,
input/resize/stop, failure recovery, tests, rollback, and phase ordering. It does
not modify the feature plan, implementation, compatibility evidence, or dashboard.

## Ranked findings

### P0 — P2B names a production Vault and enrollment broker but does not freeze a buildable security transaction

Evidence:

- `design.md:145-161` says SQLite will hold an encrypted envelope, nonce/version,
  wrapped-key metadata, and digest, then defers OS key protection and headless
  unlock to the implementation-plan contract. `api.md:288-308` only sketches
  `ReadCredential`, `ReplaceCredentialCAS`, and `Begin/Complete/Cancel` signatures.
- The implementation plan at `docs/IMPLEMENTATION_PLAN.md:672-693` selects broad
  backends and an envelope-DEK-KEK shape, but it does not select a versioned AEAD/
  wrapping format, associated data, KDF parameters/metadata, master-key creation
  and zeroization lifecycle, payload bounds, or platform fallback behavior.
- The current `internal/vault/vault.go:16-18` explicitly stores only lock state.
  `migrations/device/0004_codex_foundation.sql` stores a `vault:` reference and
  digest but no encrypted Vault item; `migrations/device/README.md:20` confirms
  that no migration stores a real Provider credential.
- The current materializer updates only CredentialInstance revision/digest in
  `internal/providers/codex/materialization.go:233-271` and
  `internal/storage/repository.go:629-653`; it cannot atomically replace encrypted
  bytes. A P2B builder must decide how the ciphertext, wrapped DEK, digest,
  CredentialInstance status, and monotonic revision commit in one crash-consistent
  transaction and how a partial commit is reconciled.
- The request/response Device protocol in `internal/device/protocol.go:54-78` has
  no interactive stream attachment contract, and the current CLI has no auth
  command. The plan says a daemon-owned staging login UI “may attach” locally but
  does not define who owns the TTY, how a headless daemon exposes the official
  flow, or the operation states for begin/status/complete/cancel/timeout/restart.
- `design.md:239` depends on a “security-impacting review,” while
  `dev_log.md:20` places `security-review` before Ship and the workflow permits
  feature `security-review` only from `READY_TO_SHIP`. The plan does not say
  whether this feature-review is the pre-build gate or whether an otherwise
  illegal pre-P2B security transition is expected.

Impact: `feature-build P2B` would have to choose cryptography, persistence,
platform support, unlock semantics, transaction recovery, interactive process/TTY
ownership, and a workflow edge while writing production secret code. The proposed
tests can detect some failures after the fact, but they do not make those choices.

Required revision:

1. Add the P2B forward migration and exact Vault record/CAS contract. Bind the
   encrypted item to Device, Provider, CredentialInstance, envelope version, and
   credential revision; define size limits and one atomic secret-plus-metadata
   commit/recovery rule.
2. Freeze the approved AEAD, DEK wrapping, AAD, randomness, key creation/unlock/
   lock/zeroization, and wrong-key/tamper behavior. State which macOS, headless
   Linux, Linux Desktop, and Windows backends P2B implements, compiles, or fails
   closed on; do not defer a platform required by P2B acceptance to Phase 5.
3. Define the enrollment operation state machine and CLI/IPC contract: start,
   local UI/TTY transport, status, success validation, cancellation, timeout,
   daemon restart, idempotency, one-active-enrollment concurrency, staging cleanup,
   and atomic creation/update of Account/CredentialInstance/Vault records.
4. Define logout/revocation versus local secret deletion and materialized-home
   cleanup. Define rollback after the new schema: feature-gate disablement, readable
   retained ciphertext, and backup/restore or old-binary refusal. A one-way
   migration cannot be described as a reversible schema rollback.
5. Name this `feature-review` as the pre-build design gate if that is intended,
   retain the open final `security-review` at `READY_TO_SHIP`, and avoid inventing
   a workflow transition.

### P0 — P3A's process and Stop model conflicts with ADR 0014's shared single writer

Evidence:

- ADR 0014 at `docs/adr/0014-codex-app-server-single-writer-auth.md:48-58`
  requires one writable app-server/auth home per CredentialInstance and says
  sessions using that credential multiplex through it. The implementation plan
  repeats the rule at `docs/IMPLEMENTATION_PLAN.md:580-583` and the reference-count
  lifecycle at `:697-705`.
- `design.md:19-21` and `:175-178` describe one runtime object containing one
  `ProviderSession`, thread ID, active turn ID, process, and materialization handle,
  without saying whether it is keyed by Session or CredentialInstance.
- `design.md:203-206` says stopping one Session interrupts its active turn and then
  terminates the app-server child. If another Session shares the canonical child,
  that operation terminates a different Session and releases the common writer.
- `api.md:129-148` exposes `Start`/`Stop` through a per-ProviderSession abstraction
  but defines no credential runtime, session-to-thread map, refcount, event/request
  routing, or shared-owner failure fan-out. The current baseline is explicitly
  per-Session (`internal/runtime/manager.go:67-72`, `:134-139`) and the current
  `ProviderSession` owns one Client (`internal/providers/codex/session.go:23-31`).

Impact: a builder must choose between one app-server per Session (violating the
single-writer decision) and a shared app-server whose Stop/Kill/Close behavior can
kill unrelated Sessions. A single `active turn ID` also cannot represent multiple
threads with concurrent turns or route Approval/notification IDs safely.

Required revision: define two explicit layers. A CredentialRuntime keyed by
CredentialInstance owns the process, materialization handle, single-reader client,
writer lease, refcount, and final CAS. A SessionBinding owns local Session ID,
Provider thread ID, active turn, pending Approval IDs, event sequence, and cleanup.
Freeze concurrent start, event demultiplexing, one-Session stop/kill, last-reference
process shutdown, daemon close, owner crash fan-out, and restart behavior. Add a
two-Session/same-credential test proving that stopping or killing one Session does
not corrupt the other or create a second writer.

### P1 — Daemon-owned capability and thread/turn trust policy are still implicit

Evidence:

- `api.md:155-176` places `binary_version` and `capabilities[]` in
  `SessionStartRequest` even though the daemon is supposed to discover, probe, and
  pin them. The contract does not explicitly prohibit the authenticated IPC caller
  from supplying or widening these authority-bearing fields.
- `api.md:130-134` names `ThreadStartRequest` and `TurnInput` without defining the
  allowlisted mappings for workspace path, model, sandbox/approval policy, network,
  profile settings, environment, or content bounds. `design.md:165-180` likewise
  says “starts a thread” and maps events without freezing those inputs.
- `test.md` covers schema/lease/input behavior, but not rejection of caller-supplied
  capability/version elevation, unsafe workspace/profile fields, sandbox/approval
  weakening, or an event carrying a different thread/turn than the local binding.

Impact: authenticated local IPC is not a reason to trust a caller's compatibility
or sandbox claims. The builder would have to invent which RuntimeProfile fields
become exact `thread/start`/`turn/start` parameters and which are daemon-derived.

Required revision: make binary path/version, canonical schema fingerprint, enabled
methods, Account validation, and capability snapshot daemon outputs. Define the
exact allowlisted `thread/start`, `turn/start`, optional `turn/steer`, and
`turn/interrupt` mappings for each enabled schema row, including workspace
resolution and sandbox/approval policy. Add negative trust-boundary and cross-thread
routing tests. Preserve the current typed resize-unsupported rule; it is clear and
does not require a Provider mutation.

### P1 — Approval dispatch has a good transaction shape but no complete exact-method decision contract

Evidence:

- `api.md:199-237` permits `approve | deny | cancel`, but successful dispatch only
  defines `approved | denied/written`; it does not define the local/provider result
  for `cancel`.
- The accepted evidence lists three distinct server requests:
  `item/commandExecution/requestApproval`, `item/fileChange/requestApproval`, and
  `item/permissions/requestApproval` (`docs/spikes/codex/p3-schema-reconcile.json`).
  The plan does not freeze each request's exact response schema and allowed
  decisions, or state which methods remain disabled.
- The current adapter deliberately rejects the permissions response as unmapped
  (`internal/providers/codex/session.go:139-175`). No live Approval was exercised;
  the schema-reconciliation security review preserves this as an open risk.

Impact: persistence-first dispatch and ambiguity handling are directionally sound,
but P3A could still send a syntactically wrong Provider result, expose `cancel`
without semantics, or enable a request kind lacking fixture coverage.

Required revision: add an exact-version table mapping each enabled server-request
method to sanitized summary fields, local decisions, exact JSON-RPC result shape,
and unsupported fallback. Reconcile `cancel`, define the dispatch request digest
used for idempotency conflicts, and bind the in-memory raw request ID to the
CredentialRuntime/SessionBinding lifecycle. Add fixtures for success, partial/
failed write, process exit, restart from dispatching, duplicate key/digest, and
every disabled Approval kind.

### P1 — Resume has contradictory Phase 2 exit semantics

The implementation plan at `docs/IMPLEMENTATION_PLAN.md:1192-1196`, the P3B phase
row in `design.md:241`, and `test.md:37` allow either verified continuation or an
explicit `provider_resume_unsupported` result. But `design.md:299-307` says that
the typed unsupported fallback is correct and then says it does **not** satisfy the
Phase 2 exit. A verifier cannot infer which document wins.

Required revision: select one exit rule and repeat it identically in the brief,
design, API, test strategy, implementation plan, and dev log. If typed unsupported
is accepted, the live test must prove no Provider mutation and no fake/local
Session masquerading as history. If continuation success is mandatory, P3B must
remain blocked until exact fixture and live restart evidence exists.

## Assessment of the remaining areas

- **Scope and ownership:** correct. Web, Desktop, Control Plane, E2EE grants,
  Claude, release, and real Windows Codex support remain out of scope.
- **Provider evidence:** appropriately narrow. Exact canonical schema rows,
  interactive-login fallback, and no multi-writer/48-hour/completed-device-auth
  claims are preserved. Linux credentialed acceptance remains open.
- **Single-reader JSON-RPC:** the proposed one-reader/response-correlation/bounded
  queue direction fixes the current global read-mutex design, but it must live in
  the shared CredentialRuntime described above.
- **Input/resize/stop:** idle input and evidence-gated steer are sound; conversation
  resize is correctly typed unsupported. Stop/Kill must be revised for a shared
  process and per-thread lifecycle.
- **Failure recovery:** quarantine, stale-CAS rejection, no mtime winner, pending
  call failure, ambiguous Approval expiry, and no automatic account rotation are
  correct. Vault commit recovery cannot be tested until Finding 1 defines the
  atomic record and key lifecycle.
- **Testing:** broad and adversarial, but it needs the multi-Session shared-runtime,
  daemon-authority, enrollment transport/state, platform-key-backend, and exact
  Approval cases above.
- **Phase ordering:** P2B before P3A before P3B is correct. P2B must first become an
  executable approved phase; P3B still requires a recorded Linux version/schema
  and credentialed environment. Final security-review remains mandatory before
  Ship.

## Checks

- `git diff --check`: passed before the verdict write.
- Read-only repository inspection confirmed the branch is
  `codex/provider/phase2-codex-vertical-slice`, the current Vault is lock-state
  only, Codex start still returns `provider_unsupported`, `auth.begin` is
  unimplemented, Approval persistence does not dispatch to Provider, and the
  protocol client still serializes reads under one mutex.
- No dashboard, plan, compatibility, evidence, or production file was modified by
  this review.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: `v0.3 correctly exposes the missing production-auth and runtime work, but P2B still lacks an executable Vault/enrollment security transaction and P3A conflicts with ADR 0014's shared per-CredentialInstance app-server lifecycle.`
**Findings**: `P0: freeze the P2B encrypted record, key/unlock lifecycle, atomic CAS, enrollment transport/state, migration/rollback, platform, and legal review edge; P0: define CredentialRuntime versus SessionBinding so one Session stop cannot terminate a shared writer; P1: make capability/thread/turn policy daemon-owned, complete exact Approval response mappings, and reconcile the contradictory Resume exit.`
**Evidence**: `v0.3 brief/design/api/test/dev_log; implementation plan; workflow/registry; ADR 0014; compatibility and auth/schema Spike evidence; current Vault, migrations, Store, runtime, IPC/CLI, Codex protocol/session/materialization, and Approval code; git diff --check.`
**Blockers**: `The feature plan must resolve the ranked P0/P1 findings before P2B feature-build; exact Linux schema/credentialed live evidence and final Security Gate remain later gates.`

### Next Step

Run `feature-plan` for `phase2-codex-vertical-slice`.
