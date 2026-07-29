# Feature review: As-built product documentation authority

- Target: `as-built-product-docs`
- Plan commit reviewed: `5e5670a3d4d5be9500a56a30f8d864311c321191`
- Date: `2026-07-29 PDT`
- Owner classification: `project-system` (high confidence). The change governs
  documentation authority, workflow evidence routing, and acceptance contracts;
  Provider and Security remain impacted fact authorities, not co-owners.
- Verdict: `REVISE`

## Conclusion

The plan correctly selects `docs/PRODUCT.md` rather than creating a root-level
duplicate, keeps `dev_log.md`, compatibility evidence, and security evidence
above product prose, and explicitly refuses to turn target, CI, dashboard, or
configuration evidence into product support. Its proposed Provider and trust
boundaries are appropriately narrow.

It is not yet executable without inventing decisions. The planned inventory is
not a fixed review artifact with a measurable coverage set or freshness rule,
and the phase table asks for Provider/Security reviews in phases that conflict
with the documented workflow. Resolve the findings below, then re-enter
`NEEDS_REVIEW`; no product documentation should be built from this revision.

## Ranked findings

### 1. Gate producers, timing, and resolution are not executable

`design.md` P2 makes Provider fact review a dependency, while P4 says that
Provider evidence and Security review occur after P2/P3. `dev_log.md` repeats
that contradiction. The plan also says P4 will "perform required reviews",
but `feature-build` may not act as the independent `security-review` writer.
The workflow permits the Security Gate review only after feature verification
reaches `READY_TO_SHIP`; it must not be silently performed inside a build
phase. "Owning provider reviewer" is likewise not a defined workflow verdict
writer or evidence artifact.

Required revision:

1. Define the provider fact-review producer, its immutable report/receipt
   path, what it validates, and how an unproven claim opens a `provider-spike`.
   State whether that review is a P1/P2 dependency or an input to later
   verification; do not retain both orderings.
2. Make P4 a documentation-build/verification-preparation phase only. Route
   the independent security assessment through the existing
   `READY_TO_SHIP -> security-review` transition, and name the resulting report
   as the Security Gate evidence.
3. Specify how the Provider Gate is resolved or remains open without allowing a
   documentation build to self-accept either Provider or security risk.

Affected files: `docs/workflow/features/as-built-product-docs/design.md`,
`api.md`, `test.md`, and `dev_log.md`.

### 2. The claim inventory cannot prove coverage or reconciliation

The API permits an unspecified Markdown inventory and defers the storage/schema
decision to feature review. P1 nevertheless accepts "no unbound substantive
claim," while P3/P4 require every listed surface to reconcile to it. No path,
versioning rule, initial coverage universe, or stable mapping from an inventory
row to a source/destination assertion is required. A writer would have to
invent all four choices, and a reviewer could not determine that a newly added
or existing support statement was omitted.

Required revision:

1. Choose and name a durable, reviewable artifact path (a Markdown ledger is
   adequate; no machine-readable registry is required for this feature).
2. Define the coverage universe: the complete listed source surfaces plus every
   capability-matrix and newly changed product-facing support/trust statement.
   Require a source heading/anchor and destination mapping for each row.
3. Make P1 acceptance and P3/P4 tests trace the fixed ledger path, stable row
   IDs, and a recorded unresolved-conflict list. State the preservation/rollback
   rule for that ledger instead of allowing its deletion to remove the review
   trail.

Affected files: `docs/reviews/as-built-product-docs/2026-07-29-feature-brief.md`,
`docs/workflow/features/as-built-product-docs/design.md`, `api.md`, and
`test.md`.

### 3. Freshness and vocabulary tests are asserted but undefined

The plan requires a dated snapshot, labels stale Provider evidence as a reason
to open a spike, and says a simulated stale date must fail. It supplies no
staleness policy: no observation-age/review trigger, source-revision binding,
or prescribed downgrade when current external/provider evidence ages out.
The class set is also inconsistent: the API/test contract uses `preview`, while
the design defines `source-built preview` as a distinct label. This prevents a
deterministic stale-evidence simulation and a unique matrix classification.

Required revision:

1. Define snapshot freshness and the provider-evidence revalidation trigger;
   bind the snapshot to a ledger revision and exact evidence dates. Specify the
   required downgrade/gate when that trigger fires.
2. Select one canonical class vocabulary (for example `preview` with a required
   `source-built` scope qualifier) and use it consistently in the brief,
   design, API, dev log, and tests.
3. Replace the prose-only stale simulation with an inspectable pass/fail
   scenario using the selected trigger and expected ledger/matrix result.

Affected files: `docs/reviews/as-built-product-docs/2026-07-29-feature-brief.md`,
`docs/workflow/features/as-built-product-docs/design.md`, `api.md`, and
`test.md`.

## Evidence and checks

- Reviewed every artifact introduced by `5e5670a`: feature brief, design, API
  contract, test plan, and feature state log.
- Checked authority against `docs/IMPLEMENTATION_PLAN.md` section 18 and its
  Phase 2 current-support boundary; the proposed canonical location and
  evidence-first hierarchy are compatible with both.
- Checked workflow state and verdict-writer constraints in
  `docs/workflow/project/workflow.md` and `.agents/roles/feature-review.md`.
- Checked current compatibility and threat-model surfaces. They already carry
  bounded Provider/platform and residual-risk wording, so documentation must
  preserve—not generalize—them.
- `git diff --check 5e5670a^ 5e5670a` passed.
- `node` is unavailable in the current shell; therefore `npm run
  project:verify` and `npm run ci:links` could not be independently run. This
  is a structural-check environment limitation, not product or plan evidence.

## Decision

`REVISE`. The planning owner must resolve findings 1-3 in the feature planning
artifacts, update its state from `REVISE` to `NEEDS_REVIEW`, and request a new
feature review. Provider evidence and Security Gate acceptance remain open;
this review neither resolves them nor authorizes a build, push, merge, release,
or risk acceptance.

## Handoff

**Target**: `as-built-product-docs`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: `Authority and evidence boundaries are strong, but the ledger, freshness policy, and Provider/Security gate sequencing remain undecided or contradictory, so the next build phase is not executable.`
**Findings**: `1) bind Provider/Security reviews to valid workflow producers and order; 2) fix the claim-ledger path, coverage, and retention contract; 3) define freshness and one canonical vocabulary with testable stale handling.`
**Evidence**: `5e5670a artifacts; implementation-plan section 18/Phase 2 boundary; workflow and role contracts; current compatibility/threat-model surfaces; git diff --check passed; Node-backed structural checks unavailable because node is absent.`
**Blockers**: `feature-plan resolution of findings 1-3; a runnable supported Node is still needed before the planned structural checks can run.`

### Next Step

Run `feature-plan` for `as-built-product-docs`.
