# Feature Review: Codex explicit multi-account selector

- Date: 2026-07-16
- Target: `codex-multi-account-selector`
- Reviewed commit: `46bff9e`
- Verdict: **REVISE**

## Conclusion

The feature is correctly owned by `provider`, is bounded to the accepted exact
Linux `0.144.2` row, preserves ADR 0014, and correctly replaces automated
identity inference with an explicit operator attestation that persists no
Provider PII. The phase ordering, migration intent, scoped logout, redaction,
platform capability gates, tests, and rollback direction are sound.

P1 is not executable yet because two server-side authorization/compatibility
decisions are still left to the builder. The current unkeyed preview digest
does not prove that the daemon issued a preview, and the plan proposes an
undefined internal/debug raw-ID capability that does not exist in the current
domain model. Both affect the central acceptance property: normal Session start
must not bypass preview and confirmation.

## Findings

### P1 — Persist or authenticate Session previews and consume them atomically

Files: `docs/workflow/features/codex-multi-account-selector/design.md`,
`api.md`, `test.md`

`preview_id` is described as a digest of safe fields and explicitly "not an
authorization token." No server-side preview record, keyed MAC, signing key,
or consumption record is defined. A caller that already knows valid internal
IDs/revisions can construct a confirmation-shaped request without proving that
the daemon issued the preview or that it remains unconsumed. Revalidating tuple
equality protects TOCTOU but does not enforce the human preview step.

Freeze one mechanism before build. Recommended: migration 7 adds a bounded
`session_start_previews` table containing opaque random preview ID, authenticated
client ID, tuple/revisions, workspace, compatibility fingerprint, selected
Usage snapshot, expiry, and consumed/session ID. `session.start` must validate
and consume the row in the same transaction that reserves/creates the Session;
idempotent replay returns the same Session, while cross-client, expired,
changed, or consumed-by-another-request previews fail. Define restart/expiry
cleanup and retention. A daemon-keyed MAC is acceptable only if the exact key
authority, restart behavior, replay store, and one-time consumption are frozen.

Add storage and authenticated-IPC tests for forged/random preview IDs,
cross-client use, two starts racing one preview, restart, expiry, idempotent
lost-response replay, and preview cleanup.

### P1 — Remove the undefined internal/debug raw-ID start capability

Files: `docs/workflow/features/codex-multi-account-selector/design.md`,
`api.md`, `test.md`

The plan says legacy raw-ID `run codex` will require an "internal-only
capability and explicit debug build/test plumbing," but no such capability,
identity migration, build mode, CLI contract, or release exclusion exists.
The parent P1 deliberately deferred fine-grained capability migration, and an
invented debug capability would create a second start path that can diverge
from product authorization.

Choose one canonical daemon start contract. Recommended: every public and local
Codex start, including acceptance tests, presents a daemon-issued preview and
full confirmation; opaque IDs remain fields inside that confirmation, not an
alternate bypass. Adapt the Phase 2 harness by seeding a public Account/Profile
alias and obtaining a preview. If a maintenance-only bypass is indispensable,
it needs a separately reviewed capability/identity/build-distribution contract
and must be outside P1.

Specify CLI compatibility: reject the old raw-ID-only invocation with
`identity_confirmation_required` (or a versioned deprecation error) before
Session insert. Add tests proving direct RPC and CLI raw-ID requests cannot
launch, including clients with the existing `session.start` capability.

### P2 — Clarify external compatibility recheck timing

Files: `docs/workflow/features/codex-multi-account-selector/design.md`,
`test.md`

The plan requires compatibility drift to create no Session, but exact binary
discovery/fingerprint/probe is external to the SQLite transaction. Freeze the
sequence: preview records the accepted binary fingerprint/schema; start
re-discovers before consuming the preview; the runtime spawn rechecks the same
fingerprint. A change after Session reservation must transition the Session to
`failed` with no credential write, while a change before reservation creates no
Session. Tests and wording must distinguish these two truthful outcomes rather
than promise impossible atomicity across filesystem/process and SQLite.

## Strengths retained

- Exact Linux-only support and macOS/Windows typed gates match the matrix.
- Explicit alias typing is a clear non-PII operator attestation and avoids
  treating undocumented JWT/email fields as durable identity.
- `awaiting_confirmation` keeps unconfirmed credentials outside the Vault and
  preserves owner/expiry/binary/tuple validation.
- Active-Session logout denial plus durable revocation reservation preserves
  the verified cross-account race boundary.
- The test plan covers migration, restart, redaction, A/B isolation, races,
  rollback, full Go/race/platform builds, and governance.

## Verification

- Reviewed Feature Brief, `design.md`, `api.md`, `test.md`, and state log.
- Compared against parent P1 selector/confirmation contracts, shipped Phase 2
  raw-ID start/enrollment/runtime behavior, ADR 0014, the distinct-account
  Spike and Security Review, compatibility matrix, workflow policy, module
  registry, and implementation plan.
- `workflow:verify`, dashboard verification, local-link verification, and diff
  integrity passed at the planned transition.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: `Exact-version scope and security boundaries are sound, but P1 cannot build until preview issuance/one-time consumption and the sole non-bypass start contract are frozen.`
**Findings**: `P1 persist/authenticate and atomically consume previews; P1 remove the undefined internal/debug raw-ID capability and require preview for every start; P2 distinguish pre-reservation compatibility drift from post-reservation failed Session.`
**Evidence**: `Feature Brief and four feature artifacts, parent P1/Phase 2 contracts, ADR 0014, resolved Spike/Security Review, compatibility/workflow checks`
**Blockers**: `feature-plan must resolve the two P1 contract findings before P1 build`

### Next Step

Run `feature-plan` for `codex-multi-account-selector`.
