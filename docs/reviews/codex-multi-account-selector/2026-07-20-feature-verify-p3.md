# Feature verification: Codex explicit multi-account selector P3

- Date: 2026-07-20
- Executor: `feature-verify / P3`
- Build commit: `25328b1`
- Verdict: `BLOCKED`

## Scope inspected

The final-phase diff was traced from every public selector entry through
binary discovery, enrollment, preview, Session reservation, runtime
materialization, platform errors, and the updated user/compatibility claims.

The preview and `StartReserved` gates are correctly typed and tested, and the
writer's Go/race/build/Web/Desktop/governance evidence passes. The final phase
cannot be accepted because the same gate is absent from the official selector
enrollment lifecycle.

## Finding

### P1 — enrollment can cross the platform boundary before preview

`auth.begin`, `auth.complete`, and `auth.confirm` each call `codex.Discover`
and proceed directly to fingerprint/validation without
`codex.RequireSelectorPlatform`. On macOS, `auth.begin --profile @A` can create
an enrollment row, a new CredentialInstance placeholder, and a private staging
Home; completion/confirmation can validate and seal that Credential. The
platform is rejected only later by `sessions.preview`.

This conflicts with the approved P3 identity-acceptance boundary and the new
user/compatibility wording that a macOS selector attempt returns
`schema_compatible_identity_acceptance_pending` without creating a
selector-owned credential Home/process. Windows has the same missing defense,
although its normal official binary availability may hide the path.

Evidence:

- `internal/app/session_service.go`: `auth.begin` discovery near line 582,
  `auth.complete` discovery near line 680, and `auth.confirm` discovery near
  line 818 contain no platform check.
- The only application call to `RequireSelectorPlatform` is the Session
  preview preflight near line 80; the second call is inside reserved runtime
  startup.
- `docs/workflow/features/codex-multi-account-selector/design.md` says macOS
  identity acceptance is pending and the multi-account selector path must not
  launch.

## Passing evidence retained

- `git show --check --stat 25328b1` — pass.
- Full Go/vet/race, platform-gate race x10, three-target builds,
  Web/Desktop, workflow/dashboard/CI/governance matrix — writer evidence pass.
- Preview and reserved-runtime platform errors are correctly differentiated;
  no finding applies to those two call sites.

## Required correction

The `feature-build` writer must apply the same selector platform gate before
any enrollment row, placeholder CredentialInstance, staging Home,
validation/seal, or Provider login launch. It must repeat the gate on
complete/confirm to catch platform/binary drift and clean up an already-open
enrollment fail-closed. Add macOS/Windows negative coverage proving zero new
enrollment/Credential/staging artifacts and no validator/spawn reach.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-verify / P3`
**Verdict**: `BLOCKED`
**Summary**: `Preview/runtime platform gates pass, but selector enrollment can still create and seal platform-pending credentials before the later Session gate.`
**Evidence**: `25328b1; auth.begin/complete/confirm discovery paths versus the two existing RequireSelectorPlatform call sites; passing writer matrix.`
**Findings**: `P1: apply the selector platform gate to the entire enrollment lifecycle and prove zero artifact creation on macOS/Windows.`
**Blockers**: `feature-build P3 correction required before final verification`

### Next Step

Run `feature-build / P3 correction` for `codex-multi-account-selector`.
