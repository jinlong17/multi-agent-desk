# Feature Review v2: Codex Vertical Slice

- Date: 2026-07-15
- Role: `feature-review`
- Target: `phase2-codex-vertical-slice`
- Owner module: `provider`
- Verdict: **APPROVED**

## Review scope

This is the independent re-review after `2026-07-15-feature-review.md` returned
`REVISE`. I re-read the revised Feature Brief, `design.md`, `api.md`, `test.md`,
and `dev_log.md`, then checked the current Phase 1 schema/Store/IPC code,
`docs/IMPLEMENTATION_PLAN.md`, ADR 0014, the Codex Spike evidence, and the
Provider Compatibility Matrix. This verdict writes only this report and the
target feature log status/work-log row; it does not modify implementation or
plan files.

## Finding closure

### P0 schema and Store compatibility — closed

The revised plan adds mandatory P0 work for the post-`0003` Device migration,
Provider allowlists, Account linkage, legacy Fake-row preservation, positive
Codex revisions, future-schema refusal, restart/rollback/interruption tests,
and Store/domain acceptance. It no longer treats the migration as optional or
asks the adapter to bypass the current Fake-only checks.

### P0 Approval/Usage IPC and CLI — closed

The revised plan now defines the local method/capability map, Account/Profile/
Credential metadata commands, Usage read, Approval list/observe/respond,
request-bound idempotency, ControllerLease enforcement, bounded persistence,
restart expiration/cancellation, and the required CLI surface. The contracts
match the existing implementation-plan Approval and UsageSnapshot models while
keeping raw Provider payloads out of storage.

### P1 materialization boundary — closed

`CredentialMaterializationManager` now has an explicit process-local handle,
typed Vault source, lease lifecycle, auth-home path, digest/structure validation,
revisioned CAS, quarantine, and shutdown ordering. The Phase 1
`vault.Materializer`/`credential.fake` path is explicitly test-only and cannot
receive raw Provider bytes from IPC.

### P1 Resume semantics — closed

The revised contract freezes `sessions.resume` as a new local Session with
source linkage and pinned Account/Credential/Profile. Provider thread
continuation requires exact fixture/live evidence; otherwise the typed
`provider_resume_unsupported` result is returned without mutation. The test and
rollback plans now use this rule instead of leaving history reuse ambiguous.

### P1 phase-status drift — closed

`docs/IMPLEMENTATION_PLAN.md` now records Phase 1 as shipped on 2026-07-15 and
records Phase 2 as `FEATURE_PLAN NEEDS_REVIEW`. This matches the Phase 1 feature
log and operator dashboard without claiming product release or deployment.

## Scope and security assessment

- Ownership remains `provider`; `core`, `security`, and `project-system` impacts
  are explicit. Web/Desktop/Control Plane remain outside the Phase 2 exit.
- ADR 0014's one-writer/revisioned-CAS boundary, interactive-login fallback,
  quarantine rule, and no multi-writer/48-hour/device-auth-completion claims are
  preserved.
- Unknown versions, unmapped Approval fields, missing Usage methods, malformed
  frames, stale leases, unresolved restart Approvals, and absent Provider
  continuation all have fail-closed behavior.
- Windows remains explicitly limited to Phase 1 CLI/Daemon/IPC and CI/protocol
  evidence; no real Windows Codex support is claimed.

## Verification evidence

- `git diff --check`: passed.
- `workflow:generate`: passed; mirrors unchanged and generated cleanly.
- `workflow:verify`: passed — 10 agents, 3 skills, 20 edges, 15 statuses.
- `dashboard`: passed; generated state reports the revised feature as
  `NEEDS_REVIEW` before this verdict transition.
- `dashboard:verify`: passed — 9 phases, 10 agents, 3 skills.
- `ci:links`: passed — 205 Markdown files, no broken local links.
- Production code was not changed and Go/runtime tests are not a feature-plan
  acceptance gate; they belong to the later build/verify roles.

## Verdict

**APPROVED.** The revised plan is executable as a sequence of reviewable build
phases without inventing the previously missing persistence, IPC, materializer,
or resume decisions. Approval does not authorize implementation, credentials,
live Linux testing, security acceptance, Ship, merge, push, release, or deploy.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: `All prior P0/P1 findings are closed in the plan: P0 schema/Store/IPC and Approval/Usage contracts are explicit, the real materialization boundary is mapped, Resume semantics are frozen, and Phase 1 status authority is reconciled.`
**Findings**: `No remaining blocking findings. Linux schema/version, Approval field mapping, Provider continuation evidence, credentialed live exit, and the implementation Security Gate remain required later gates.`
**Evidence**: `Revised Phase 2 brief/design/api/test/dev_log; current Phase 1 migrations, Store, IPC, Vault, and CLI; implementation plan; ADR 0014; Codex Spike artifacts; workflow/dashboard/link checks.`
**Blockers**: `None for feature-build planning; production code and live-provider gates remain unstarted.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice`, starting with P0 only.
