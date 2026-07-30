# Feature Review v5: Phase 4a Control Plane Core

- Date: `2026-07-21`
- Role: `feature-review`
- Plan version: `v0.5`
- Owner module: `control-plane`
- Reviewed state: `FEATURE_DEV / NEEDS_REVIEW`
- Verdict: `REVISE`

## Conclusion

Plan v0.5 closes the audit's major architectural omissions: immutable canonical
server-origin binding, exact WebAuthn counter/session-revocation policy,
versioned Device-auth persistence, candidate pre-auth and activation ordering,
0008/0009/0010/0011 migration ownership, immutable remote-ID mapping,
browser/Device capability separation, lifecycle/presence separation, real
wrapped-X25519 proof, bounded sync pages, no Fake network Session projection,
separate local/server revisions, command recovery principles, browser-safe
crypto/PWA behavior, real browser/Desktop receipts, privacy, and the Phase
4a/4b/5 boundary.

It is not yet decision-complete enough for P1 to check in the full P2-P6
OpenAPI without inventing wire contracts. The remaining findings are contract
definition defects, not implementation defects. The paused P1 implementation
and generated files were not modified or treated as plan authority.

## Findings

### 1. High — P5 delivery, claim, result, and receipt schemas are not an executable wire contract

`api.md` adds `deliveryRevision` to `SessionCommandOfferV1` and the receipt
common fields, and requires derived per-call idempotency to bind delivery and
receipt revisions. It does not replace the earlier claim/ack/result/reconcile
request and response DTOs with exact v0.5 schemas that carry those values.
`SessionCommandOfferV1` also contains `attempt` and `claimExpiresAt` before the
document defines whether listing an offer creates a claim or whether the
separate claim mutation creates them. The narrative says the cursor advances
only after a current claim plus durable reserved receipt, but no exact response
or transaction result binds that cursor event.

The receipt contract then references undefined `KindProofV1` and
`SessionCommandOutcomeV1` schemas. Narrative field lists for start/resume and
stop/kill are not sufficient to generate strict OpenAPI oneOf variants or
cross-kind rejection tests. P1 would have to invent fields, required/optional
rules, discriminators, endpoint payloads, and the offer-to-claim transition.

Required revision in `api.md`: define the exact delivery-list, claim, ack,
result, reconcile, authoritative-query, cursor-commit, `SessionCommandOutcomeV1`,
and every per-kind proof schema; state exactly when attempt/claim expiry are
allocated; bind `deliveryRevision` and `receiptRevision` in every applicable
request/response and derived idempotency formula. Update `test.md` with schema
round trips and offer/list -> claim -> reserved-receipt/cursor transaction
goldens. Reconcile the older v0.4 DTO block instead of leaving builders to
apply prose precedence.

### 2. High — P3 activation-package and signed-receipt object boundaries are ambiguous

The v0.5 formula hashes and signs `JCS(ActivationReceiptV1)`, while the next
paragraph says raw approver public keys are "outside its signed JCS object."
No exact activation-package response schema names the signed receipt payload,
detached signature, raw approver keys, attestation, or their relationship.
Likewise, the earlier `EnrollmentActivateRequest` remains present while v0.5
introduces `SubjectActivationAckV1` without an explicit replacement request/
response DTO. This leaves two incompatible ways to shape the OpenAPI and risks
accidentally signing transport-only key fields or omitting digest-bound fields.

Required revision in `api.md`: split and name the strict signed
`ActivationReceiptV1` payload from an exact `EnrollmentActivationPackageV1`
transport wrapper; enumerate every wrapper field and `additionalProperties:
false`; define the final activate request/result using
`SubjectActivationAckV1`; state which raw keys are outside JCS and require their
digests to match the signed payload. Add byte-level package/ack/cross-type
vectors to `test.md`.

### 3. High — P2 WebAuthn JSON is still not fully specified for complete OpenAPI generation

The registration/assertion credential DTOs leave
`clientExtensionResults` as an unnamed "strict allowlisted object" and options
responses as a generic "generated WebAuthn option DTO." The same text rejects
unknown extensions, so the builder must decide which members are known and how
their binary/nested values are encoded. This conflicts with P1's requirement
to check in a complete exact P2-P6 OpenAPI before any P2 behavior exists.

Required revision in `api.md`: enumerate the accepted registration/assertion
extension-result members (or explicitly freeze an empty object for v1), their
types/bounds, and the exact creation/request options schemas including every
Base64url field. Update `test.md` with accepted extension members, rejected
unknown members, and options/credential Go-TypeScript byte-shape fixtures.

### 4. Medium — `ProfileConflictV1` still contains undefined mutation and field types

`ProfileConflictV1.submitted` references an undefined `ProfileMutationV1`, and
`conflictingFields[].field` is only "allowlisted Profile field." This does not
freeze whether create-only fields, target ownership, materialization state, or
server-managed fields can appear, nor the exact digest input for absent versus
nullable values.

Required revision in `api.md`: define the strict submitted mutation union and
closed conflict-field enum, including omitted/null digest semantics and bounds;
add exact conflict vectors in `test.md`.

### 5. Low — the P4 phase summary contains a contradictory migration number

`design.md` says P4 adds "migration 0009" and immediately says the migration is
0010. All normative v0.5 sections, `api.md`, `test.md`, and `dev_log.md` agree on
0010, so this does not create an architectural choice, but it would fail the
stated stale-v0.4 scan and should be removed in the same planning revision.

## Confirmed boundaries and gates

- P0 remains independently verified; v0.5 does not reopen or reinterpret its
  vectors.
- P1 may expose only health/readiness/version. P2-P6 routes remain contract-only
  and side-effect free until their verified phase.
- P2 correctly binds the remote identity to one canonical HTTPS origin in the
  row, envelope, AAD, descriptor, challenge, and receipts; v7 backup/restore,
  exact cookie attributes, counter CAS, passkey-derived session revocation,
  passkey deletion, and frozen real-browser receipts are explicit.
- P3 correctly separates local IPC identity from remote identities, browser
  auth from Device capability, lifecycle from presence, and activation receipt
  verification/pinning from final acknowledgement. It assigns only trust data
  to migration 0009 and only `snapshot_required` to P3.
- P4 correctly freezes 1..4 snapshot pages, the 900-KiB pull budget, no Fake
  network Session, disabled browser-created Profiles pending local completion,
  and separate local/server revisions.
- P5 correctly states half-open time, priority, attempt/retention bounds,
  append-only delivery, preallocated result Session IDs, no raw browser
  idempotency key at the Daemon, no `local_committed -> ambiguous`, and
  acquire/release typed unsupported. Finding 1 is about making those rules
  executable in the wire schema.
- P6 correctly requires browser-only crypto, IndexedDB origin/revision CAS,
  network-only `/v1/**`, bounded Overview, exact Usage units, real Safari and
  cross-browser receipts, launched macOS Tauri render evidence, polling stops,
  recovery privacy, and logout key retention.
- The open Security Gate remains reviewer-owned and correctly blocks Ship, not
  Feature Review.
- The current `Python-2.0` pnpm license failure is truthfully reproducible and
  correctly assigned to P1 dependency reconciliation before P1 verification.
  It is not a reason to block plan review, and this verdict neither waives nor
  accepts that dependency.

## Evidence and checks

- Fully reviewed `docs/IMPLEMENTATION_PLAN.md`, repository workflow/module
  authority, feature brief, `design.md`, `api.md`, `test.md`, and `dev_log.md`.
- `npm run workflow:verify`: pass (`agents=10`, `skills=3`, `docs=17`,
  `edges=20`, `statuses=15`).
- `npm run project:verify`: pass; dashboard generation/verification remained a
  machine-fact check and did not change operator judgment.
- `git diff --check`: pass before verdict persistence.
- `npm run ci:licenses`: expected failure at the paused P1 checkpoint:
  `pnpm group Python-2.0: disallowed license expression Python-2.0`.
- Git status confirmed the P1 implementation/generated files are concurrent
  task-owned changes. This review modified none of them.

## Handoff

Return to `feature-plan` for plan v0.6. Close Findings 1-5 across the existing
planning artifacts only, preserve verified P0 and every paused P1
implementation/generated change, then resubmit `NEEDS_REVIEW`.
