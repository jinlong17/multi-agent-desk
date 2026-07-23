# Feature Review v3: Phase 4a Control Plane Core

- Date: 2026-07-21
- Role: `feature-review`
- Owner module: `control-plane`
- Reviewed plan version: `v0.3`
- Verdict: `REVISE`

## Conclusion

Plan v0.3 closes all four findings from review v2. P2 now owns and verifies the
complete Device envelope/migration/bootstrap prerequisite before exposing
bootstrap; the sync change wire has exact canonical revision, create sentinel,
history-missing, nested-map, atomic-array, and conflict contracts; command
receipts have a reserved-only attempt-rebind plus later-state reconciliation;
and the activation/no-secret and storage-assertion negative-vector tests now
match the design.

The complete P0-P6 plan is still not approved because the authoritative P4
snapshot is not wire-complete. Its declared topology includes a Device
resource that `SyncSnapshotPage.resources` cannot encode, and the page/final
snapshot digests have no canonical construction. Those decisions affect
restore/re-enrollment integrity and cross-language interoperability and cannot
be left to the P4 builder.

## V2 finding closure audit

1. **P2 envelope dependency — closed.** Migration
   `0008_control_plane_remote_identity.sql`, the generic mapping, exact
   `DeviceKeyEnvelopeV1` create/open/CAS contract, pending-to-active receipt
   lifecycle, and prepare/import/prove/verify/activate actors are all owned and
   accepted by P2 before bootstrap handlers become available. P3 explicitly
   reuses that verified foundation.
2. **Canonical sync wire — closed for push/pull/conflict.** The plan freezes
   strict typed `CanonicalSyncRevisionV1`, RFC 8785 behavior, domain-separated
   revision and create-base digests, revision-zero absence, missing-history
   failure, deterministic recursive-object/atomic-array patches, subtree and
   patch digests, and bounded full-revision conflict responses.
3. **Expired command claim — closed.** Attempt N to N+1 recovery is an atomic
   `reserved`-only receipt CAS preserving the local operation identity.
   `executing` and later states cannot rebind; local commit proof,
   `ambiguous`, reconcile, completed-state comparison, and race/restart tests
   are explicit.
4. **Activation and PoP vectors — closed.** Enrollment acceptance now requires
   only the signed public `ActivationReceiptV1` and Device metadata, explicitly
   rejects every activation/connection/session secret, and requires later
   Device-auth PoP. P0 negative vectors mutate both `storageMode` and
   `storageAssertionDigest`.

## Ranked finding

### 1. High — the authoritative snapshot resource set and digest wire are not executable

Files: `design.md` "Mapping ownership, snapshots, and restore"; `api.md`
`CanonicalSyncRevisionV1`, `SyncSnapshotPage`, and snapshot prose; `test.md`
P4 snapshot gate.

The design and API say snapshot resources are topologically ordered `Device ->
Account -> Credential status -> Profile -> Workspace -> Session -> Usage`.
However, `SyncSnapshotPage.resources` contains only
`CanonicalSyncRevisionV1[]`, and that union permits only `account`,
`credential_status`, `profile`, `workspace`, `session`, and `usage`. It has no
`device` discriminator or Device snapshot value. An implementation must guess
whether Device is an actual first resource, an out-of-band prerequisite, or a
documentation error.

The same DTO declares `pageDigest` and `finalSnapshotDigest`, and the design
calls the snapshot digest-bound, but no domain-separated input or canonical
algorithm binds snapshot ID/epoch, page sequence, resources, cursors, and final
incremental base. Implementations can therefore disagree about page reorder,
omission, duplication, cross-snapshot mixing, empty pages, or the final cursor
while still satisfying the prose.

Freeze one exact model across design/API/test: either add a typed Device
snapshot member or explicitly make the already-authenticated target Device an
out-of-band precondition and remove it from the resource topology. Define
canonical page and final digest formulas, ordering/page-continuity rules,
snapshot expiry/replay/commit behavior, and which IDs/epoch/cursors are bound.
Add Go/TypeScript goldens plus reorder, omission, duplication, mixed-epoch,
truncation, cursor-substitution, and commit-replay negatives.

## P0-P6 and boundary review

- P0-P3 are phase-ordered and directly executable: contract/vector authority,
  generator/runtime ownership, bootstrap storage prerequisite, browser auth,
  enrollment, capabilities, signed REST, presence, and revocation have named
  gates and failure behavior.
- P4 push/pull/conflict/tombstone/mapping rules are substantially complete, but
  the snapshot finding above blocks P4 approval and therefore the complete
  sequential plan.
- P5 now has an executable at-least-once receipt and reconciliation boundary,
  including exact no-reexecution behavior for uncertain effects.
- P6 remains metadata-only and has concrete browser, accessibility, responsive,
  Desktop-smoke, end-to-end, secret-scan, and security-handoff gates.
- The 4a/4b/5 boundary remains truthful: no WSS, HPKE, Pairwise Root, terminal,
  Approval response, Credential Grant, Desktop key store, or release claim is
  pulled into Phase 4a.
- Rollback remains forward migration plus verified backup/prior binary, with
  local state, mappings, envelopes, outboxes, and command receipts preserved.
  Provider Gate remains `none`; the independent Security Gate remains open.

## Evidence

- Re-read repository governance, the `feature-review` role, workflow policy,
  module registry/classification instructions, implementation-plan authority,
  the complete v0.3 feature brief, `design.md`, `api.md`, `test.md`,
  `dev_log.md`, and the v2 review report.
- Audited all four v2 findings against v0.3 phase ownership, exact DTOs,
  negative tests, failure/restart paths, and acceptance gates.
- Cross-checked P0-P6 ordering, security and Provider gates, rollback, Phase
  4a/4b/5 exclusions, snapshot/mapping topology, and command recovery.
- Coordinator-recorded `project:verify`, `ci:verify`, and `git diff --check`
  success is acknowledged as structural baseline evidence and was not
  represented as rerun by this reviewer.

## Verdict

`REVISE`. Return to `feature-plan` to close the snapshot resource/digest wire
finding and resubmit a new plan version. No product build phase is authorized
by this verdict.
