# Feature Review v6: Phase 4a Control Plane Core

- Date: `2026-07-21`
- Role: `feature-review`
- Plan version: `v0.6`
- Owner module: `control-plane`
- Reviewed state: `FEATURE_DEV / NEEDS_REVIEW`
- Verdict: `REVISE`

## Conclusion

Plan v0.6 closes all five Feature Review v5 findings at the API/test-contract
level. P1 can generate the exact WebAuthn, activation-package, Profile conflict,
and command delivery/receipt DTOs without inventing those shapes. The P4
migration contradiction is removed.

One cross-artifact contradiction remains inside the v0.6 amendment itself:
`design.md` declares a closed P5 outcome set that does not match the exact
`SessionCommandOutcomeV1` oneOf in `api.md`. Because both passages are current
v0.6 authority and both claim closure, P5 implementation and its documentation
cannot choose between them without applying an undocumented precedence rule.
The plan therefore remains `REVISE` for one narrow correction.

## v5 finding closure

1. **P5 delivery/receipt wire — closed.** Listing persists an offer but does
   not claim; claim alone allocates attempt/lease; the Daemon commits a strict
   local reserved receipt; ack validates it and advances only a contiguous
   server cursor. Exact list/offer/claim/ack/result/reconcile/query/cursor DTOs
   carry delivery, attempt, receipt, command, and idempotency revisions. Closed
   reservation, proof, outcome, and receipt oneOf types plus legacy-shape
   rejection tests are present.
2. **P3 activation package — closed.** `ActivationReceiptV1` is the strict JCS
   signed payload. `EnrollmentActivationPackageV1` separately carries raw
   approver keys, transcript, attestation, and detached signatures, with exact
   digest matching. Final activation accepts only the typed
   `SubjectActivationAckV1` request and returns no activation secret.
3. **P2 WebAuthn wire — closed.** Creation/request option objects, descriptor,
   algorithms, timeouts, authenticator selection, Base64url fields, nested
   `additionalProperties:false`, and empty v1 request/result extensions are
   explicit. Unknown extension/library-native shapes are negative fixtures.
4. **Profile conflict — closed.** The patch/delete mutation union, mutable-field
   branches, closed conflict enum, omitted/null/value domains, field-bound
   digests, delete semantics, ordering, and bounds are explicit and tested.
5. **P4 migration number — closed.** Current planning authority consistently
   assigns P3 trust to 0009, P4 sync to 0010, and P5 receipts to 0011.

## Finding

### High — Current v0.6 P5 closed outcomes disagree between design and API

`design.md` under `Plan v0.6 decision-complete amendments` says start/resume
may use `execution_ambiguous`, lists only `provider_resume_unsupported`, and
does not define the exact start/stop/kill unsupported branches or all common
terminal failures. `api.md` instead freezes `command_execution_ambiguous`,
`provider_session_start_unsupported`, `provider_stop_unsupported`, and
`provider_kill_unsupported`, plus the exact `CommonFailureCode` union. The
stable error inventory and older command-recovery text also use
`command_execution_ambiguous`.

This is not superseded pre-v0.6 prose: the conflicting design paragraph is
inside the v0.6 amendment and itself says the outcomes are closed. API remains
the REST type authority, but `RemoteCommandService` is also an implementation
contract described by `design.md`; requiring the builder to silently discard
one current closed set violates the decision-complete review gate.

Required v0.7 correction: make the `design.md` P5 stable-outcome paragraph
reference or reproduce exactly the `api.md` `SessionCommandOutcomeV1` and
`CommonFailureCode` unions, including exact ambiguous and unsupported codes.
Add a cross-document stale-code scan to `test.md` so
`execution_ambiguous` and incomplete unsupported sets cannot return. No API,
phase, implementation, generated, or dependency change is required unless the
planner intentionally chooses a different outcome set, in which case all three
artifacts must change together and be re-reviewed.

## Regressed boundaries and gates

- P0 remains independently verified and is not reopened.
- P1 remains contract-only beyond health/readiness/version; P2+ attempts must
  stay side-effect free until their verified phases.
- Immutable server origin, WebAuthn counter/session revocation, 0008 backup,
  remote-ID mapping, Device-auth persistence, enrollment pin-before-ack,
  capability separation, lifecycle/presence, wrapped-X25519, snapshot bounds,
  no Fake network Session, disabled Profile materialization, separate sync
  revisions, command restart/concurrency, browser crypto/PWA, real browser and
  Desktop receipts, privacy, and Phase 4a/4b/5 boundaries remain intact.
- The open Security Gate still correctly blocks Ship and remains reviewer-owned.
- `Python-2.0` remains a truthful paused-P1 dependency/license failure. v0.6
  correctly assigns it to dependency reconciliation before P1 verification;
  it is not a Feature Review blocker and is not waived by this verdict.

## Evidence and checks

- Re-read the current v0.6 feature brief, `design.md`, `api.md`, `test.md`,
  `dev_log.md`, and the complete v5 review; inspected the concurrent P1 status
  without modifying implementation, generated, or dependency files.
- `npm run workflow:verify`: pass (`agents=10`, `skills=3`, `docs=17`,
  `edges=20`, `statuses=15`).
- `npm run dashboard:verify`: pass without dashboard generation or judgment
  mutation.
- `npm run ci:static`: pass.
- `npm run ci:fixtures`: pass.
- `npm run ci:links`: pass (`markdown_files=307`).
- `git diff --check`: pass before verdict persistence.
- `npm run ci:licenses`: expected P1-checkpoint failure,
  `pnpm group Python-2.0: disallowed license expression Python-2.0`.

## Handoff

Return to `feature-plan` for plan v0.7. Reconcile only the current P5 stable
outcome paragraph and matching stale-code test, preserve verified P0 and every
paused P1 implementation/generated/dependency file, then resubmit
`NEEDS_REVIEW`.
