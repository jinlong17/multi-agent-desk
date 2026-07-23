# Feature Review v7: Phase 4a Control Plane Core

- Date: `2026-07-21`
- Role: `feature-review`
- Plan version: `v0.7`
- Owner module: `control-plane`
- Reviewed state: `FEATURE_DEV / NEEDS_REVIEW`
- Verdict: `APPROVED`

## Conclusion

Plan v0.7 closes the sole Feature Review v6 finding without changing the API,
phase ordering, or product scope. The complete P5 terminal-outcome block in
`design.md` is byte-identical to the normative `CommonFailureCode` and
`SessionCommandOutcomeV1` block in `api.md`. The plan is decision-complete and
approved for the paused P1 writer to resume reconciliation against v0.7.

No material Feature Review finding remains. The verified P0 result is
preserved, P1 still mounts only health/readiness/version handlers, and the
known `Python-2.0` dependency-license failure remains an explicit P1
verification gate rather than a waived or misclassified plan-review blocker.

## v6 finding closure

1. **P5 closed outcomes — closed.** `design.md` and `api.md` enumerate the
   same fourteen `SessionCommandOutcomeV1` branches with identical fields,
   states, codes, IDs, and status domains. `CommonFailureCode` is exactly
   `target_revoked|feature_disabled|delivery_attempts_exhausted|
   daemon_shutting_down|command_execution_ambiguous` and is permitted only in
   failed variants.
2. **Unsupported outcomes — complete.** Start, resume, stop, and kill use the
   exact respective codes `provider_session_start_unsupported`,
   `provider_resume_unsupported`, `provider_stop_unsupported`, and
   `provider_kill_unsupported`. Acquire/release remain the typed Phase 4b
   unsupported branches and are never delivered in Phase 4a.
3. **Stale-code regression — executable.** `test.md` requires normalization of
   the complete current design and API blocks followed by byte-equal branch,
   field, state, and code-set comparison. It explicitly fails on the old bare
   `execution_ambiguous` token, omission of any common failure, or
   omission/substitution of any of the four provider unsupported codes. The
   bare legacy token appears only as that required negative-test sentinel; it
   does not occur in current design or API authority.

## v5 closure and boundary regression

- All five v5 findings remain closed: exact P5 delivery/receipt wire and
  oneOfs, strict P3 activation receipt/package/final ack, exact P2 WebAuthn
  options plus empty closed extensions, typed Profile mutation/conflict, and
  the 0009/0010/0011 migration ownership.
- P0 remains independently `verified`; v0.7 neither reopens nor weakens its
  portable Vault, digest, proof, time, and three-platform evidence.
- P1 generates the full v0.7 contract but mounts only `healthz`, `readyz`, and
  `version`. Bootstrap, auth, enrollment, sync, commands, and Web behavior
  remain side-effect-free contracts until their independently verified phases.
- Immutable origin, WebAuthn counter/session revocation, backup/recovery,
  remote-ID mapping, pin-before-ack enrollment, capability separation,
  wrapped-X25519, bounded snapshots, no Fake network Session, disabled Profile
  materialization, separate sync revisions, command restart/concurrency,
  browser/PWA privacy, real browser/Desktop receipts, and Phase 4a/4b/5
  boundaries remain intact.
- Provider Gate remains `none`. The open Security Gate still blocks Ship and
  remains independently reviewer-owned.

## Evidence and checks

- Re-read plan v0.7 authority, Feature Review v6, and the v5 closure evidence;
  inspected the concurrent P1 worktree state without modifying implementation,
  generated, dependency, migration, dashboard, or existing review files.
- Direct extraction and `diff -u` of the complete design/API P5 terminal
  outcome blocks: pass with no output.
- Standalone-token scan: no bare `execution_ambiguous` in current design/API;
  one intentional negative-test sentinel in `test.md`.
- `npm run workflow:verify`: pass (`agents=10`, `skills=3`, `docs=17`,
  `edges=20`, `statuses=15`).
- `npm run dashboard:verify`: pass without generation or judgment mutation.
- `npm run ci:static`: pass.
- `npm run ci:fixtures`: pass.
- `npm run ci:links`: pass (`markdown_files=308`).
- `git diff --check`: pass before verdict persistence.
- `npm run ci:licenses`: expected paused-P1 failure,
  `pnpm group Python-2.0: disallowed license expression Python-2.0`; P1 cannot
  verify until its writer removes/replaces or obtains an operator-approved
  compatible resolution through the normal gate.

## Handoff

Resume the existing `feature-build` P1 writer. Reconcile the full OpenAPI,
generated clients/types, runtime client, and foundation implementation to plan
v0.7; preserve the health/readiness/version-only runtime boundary; clear the
`Python-2.0` license gate; then stop at `READY_FOR_VERIFY` for independent P1
verification.
