# Design: As-built product documentation authority

## Decision snapshot

- Owner: `project-system`. This feature governs documentation authority and
  evidence routing; it does not own Provider or Security product behavior.
- Canonical future target: `docs/PRODUCT.md`. The `PRODUCT.md` row in the
  implementation plan's documentation table is resolved as the file named
  `PRODUCT.md` within that table's `docs/` document family. No root
  `PRODUCT.md` will be created.
- `docs/IMPLEMENTATION_PLAN.md` remains the reviewed v0.1 target baseline.
  It is not an as-built capability or release authority.
- `docs/workflow/features/<slug>/dev_log.md` remains the resumable lifecycle
  state authority. Independent review/verification/security reports and exact
  compatibility artifacts remain claim evidence; no prose document overrides
  them.
- Provider and Security Gates are open. A positive substantive Provider claim
  needs the immutable provider evidence receipt defined below before P2;
  independent `security-review` is required only after final feature
  verification reaches `READY_TO_SHIP` and before ship for substantive
  credential, cryptographic, trust, revocation, or residual-risk wording.

## Authority model

| Question | Primary authority | Documentation treatment |
|---|---|---|
| What product is v0.1 intended to be? | `docs/IMPLEMENTATION_PLAN.md` and accepted ADRs | describe as target/planned unless independently implemented and verified |
| What is the state of a feature now? | target feature `dev_log.md` and its workflow verdicts | link rather than duplicate mutable phase state |
| What exact Provider/platform capability has evidence? | `docs/PROVIDER_COMPATIBILITY.md` row plus linked feature/spike/review evidence | preserve exact Provider, version, platform, result, fallback, and date |
| What trust behavior and residual risk apply? | `docs/THREAT_MODEL.md`, accepted ADRs, security-review report, and implementation evidence | retain boundaries and residual risk; do not condense away qualifications |
| What does the product mean to a user at a dated snapshot? | future `docs/PRODUCT.md`, constrained by the authorities above | canonical overview with evidence pointers, not a new source of proof |
| How does a reader get started? | `README.md` and `docs/USER_GUIDE.md` | derived entry points that link to `docs/PRODUCT.md` and exact evidence |
| What does a specialist surface describe? | architecture/data model/adapter/roadmap documents in their stated scope | as-built only inside their specialty; link outward for lifecycle truth |

When sources disagree, the most specific current independently reviewed
evidence controls the support wording. The documentation must downgrade to
`unknown`, `planned`, `experimental`, or `unsupported` until the discrepancy
is resolved; it must never choose the more optimistic claim.

## Claim vocabulary and freshness

Every product-facing capability statement must use one of these meanings.

| Label | Meaning | Required evidence/wording |
|---|---|---|
| `planned` | reviewed target v0.1 behavior not yet proven as current product behavior | implementation-plan or approved feature-plan link; no runnable instruction |
| `preview` | runnable development path from source, not an installation/release promise | `scope` must include the literal qualifier `source-built`; identify omissions |
| `supported` | exact behavior independently evidenced for a specified version, platform, and capability | exact evidence link, fallback, and date; no generalization |
| `experimental` | intentionally non-stable behavior with stated limits | explicit platform/scope and non-stable label |
| `unsupported` | intentionally not available or ruled out by evidence/policy | preserve reason and safe fallback where one exists |
| `unknown` / `pending` | no current reproducible evidence or a gate remains open | name the gate/owner; never present as success |

Phase labels, green CI, a build, a configured binary, a dashboard snapshot,
and a feature branch are not standalone support labels.

`target` is allowed only as a synonym in prose for the canonical ledger class
`planned`; every ledger and capability-matrix row uses exactly one of
`planned`, `preview`, `supported`, `experimental`, `unsupported`, or
`unknown`. `stale` is evidence state, never a seventh capability class.

The future ledger and product snapshot use these deterministic dates and
bindings:

- Ledger header: `Ledger revision`, `Snapshot as of`, `Verified on`, and the
  full Git revision of the reconciliation baseline. Every evidence link is
  recorded as `repo-relative-path@full-commit`, with its exact
  `evidence_date`.
- A current snapshot is fresh only when `Verified on - Snapshot as of <= 30`
  calendar days and every evidence revision is reachable from the verification
  baseline. At day 31, all rows represented as current (`preview`,
  `supported`, or `experimental`) must change to `unknown` with
  `evidence_state=stale` and `fallback_or_gate=snapshot refresh`; the product
  matrix must use the same downgrade.
- A Provider receipt is current only when the full receipt revision is
  reachable from the verification baseline, its exact Provider/tool,
  version, platform, and capability equal the ledger scope, and
  `Verified on - evidence_date <= 90` calendar days. A changed scope,
  unreachable receipt, contradiction, or age of 91 days is stale.
- A stale or missing positive Provider receipt changes that ledger row to
  `unknown`, sets `evidence_state=stale` or `missing`, opens the Provider Gate,
  and names `provider evidence refresh` as the gate. If determining the new
  result requires Provider behavior research, feature-plan creates a separate
  provider-owned spike; it must complete its normal
  `SPIKE_READY -> provider-spike -> ... -> GATE_RESOLVED` path before a fresh
  positive claim can return.

## Future document shape

`docs/PRODUCT.md` will be concise and dated. Its build phase must include:

1. product position and intended user;
2. explicit non-goals and policy boundaries;
3. status snapshot with a visible `as of` date, source-built/release boundary,
   and links to live feature state rather than copied status tables;
4. current capability matrix whose rows include state, exact scope, evidence,
   fallback/gate, and user-safe next action;
5. product-surface map for local CLI/Daemon, Provider adapters, Control Plane,
   Web/PWA, and Desktop, clearly separating target from as-built scope;
6. trust and data-boundary summary that links to the threat model and does not
   replace it;
7. documentation map explaining what each authority owns and how to report
   drift.

The canonical document may summarize evidence but must not copy raw Provider
output, credentials, browser data, private account identifiers, or unredacted
terminal content.

## Claim ledger and reconciliation design

The durable, reviewable ledger is exactly
`docs/reviews/as-built-product-docs/claim-ledger.md`. It is an append-only
review artifact after its initial P1 baseline: resolved rows may be superseded
but are never deleted, and each supersession names the prior stable ID and
reason. Reverting `docs/PRODUCT.md` or a derived-document change never removes
the ledger or its unresolved-conflict history. Its header contains the
freshness bindings above and an `Unresolved conflicts` table; an empty list is
written explicitly as `none`.

P1's complete coverage universe is the source heading/anchor of every
substantive product-facing support or trust statement in:

The future implementation inventories every substantive sentence in the
following surfaces before editing it:

- `README.md` — entry point and concise availability boundary;
- `docs/USER_GUIDE.md` — user-facing operations and current/planned labels;
- `docs/ARCHITECTURE.md` — as-built components/data flows, not target-only
  diagrams presented as operational;
- `docs/DATA_MODEL.md` — implemented schema and explicit later-phase gaps;
- `docs/PROVIDER_ADAPTER.md` and `docs/PROVIDER_COMPATIBILITY.md` — exact
  adapter/compatibility scope only;
- `docs/THREAT_MODEL.md` — security authority and residual risk;
- `docs/ROADMAP.md` — target order without current-support upgrades;
- every capability-matrix row in the future `docs/PRODUCT.md`; and
- every newly changed product-facing support or trust statement in a surface
  outside this list.

Each row uses a stable `CLM-###` ID and records source path plus heading/anchor,
normalized claim, class, scope, authority and full-revision evidence link,
evidence date/state, provider receipt when applicable, required reviewer,
destination heading/anchor, and conflict ID (or `none`). A source statement is
covered only when it has a row and the row maps it to every canonical/derived
destination assertion or records a bounded unresolved conflict. The
implementation must change a claim's evidence first when needed, then
reconcile all derived text in the same build phase. A placeholder may remain
only if it explicitly declares its limited scope and links to the canonical
product document.

## Gate producers, ordering, and resolution

Provider fact review is not a new documentation verdict role. Its valid
producer is either (a) the independent `feature-verify` verdict for the
provider-owned feature that proved the exact scope, at
`docs/reviews/<provider-feature-slug>/<date>-feature-verify*.md`, or (b) a
provider-spike's immutable evidence under
`docs/spikes/<provider-area>/` plus its `GATE_RESOLVED` provider feature log
and compatibility-matrix row. The ledger records the selected receipt at its
full Git revision; this path-and-revision tuple is the immutable receipt for a
Provider row. It validates exact Provider/tool version, platform, capability,
result, evidence date, fallback, and gate wording. A Provider feature plan or
spike decision, never this documentation build, establishes that receipt.

P1 only inventories existing receipts and marks their gaps. P2 has the single
ordering rule: it may publish a positive (`supported` or `experimental`)
Provider claim only after the matching current receipt is recorded in the
ledger. P2 may publish `unknown`, `planned`, or `unsupported` wording while a
Provider Gate is open, but it cannot call this feature's Provider Gate
resolved. An unproven or stale positive claim opens a separately tracked
provider spike; after its decision updates the compatibility evidence, a new
P1 ledger revision may bind it. P4/feature verification merely checks this
contract and must block if a positive row lacks a current receipt; neither is
allowed to accept Provider risk.

The Security Gate remains open through P4. P4 prepares structural and
traceability evidence only; it performs no security verdict. After the final
documentation phase is independently verified and the feature reaches
`READY_TO_SHIP`, the workflow's `security-review` writer independently issues
`docs/reviews/as-built-product-docs/<date>-security-review.md`, updates this
feature log, and either sets the Security Gate resolved (`ACCEPTED`) or returns
`REVISE`/`BLOCKED`. The documentation builder and verifier cannot accept
security risk.

## Phased implementation and rollback

| Phase | Scope | Dependencies | Acceptance | Rollback |
|---|---|---|---|---|
| P1 evidence inventory | Build `docs/reviews/as-built-product-docs/claim-ledger.md` with baseline coverage, revision bindings, conflicts, and receipt gaps | approved plan; access to linked logs/reviews | every coverage-universe source heading has a stable row/mapping; unresolved list is explicit; no row lacks a class | retain append-only ledger; revert no primary evidence |
| P2 canonical product document | Author `docs/PRODUCT.md` with authority map, snapshot, capability matrix, trust summary, and evidence links | P1; a current Provider receipt for each positive Provider claim | all canonical claims match ledger; no root `PRODUCT.md`; positive Provider claims have receipt tuples | revert document-only commit; retain ledger/history |
| P3 derived document reconciliation | Update the README, user guide, and specialist documents from the accepted ledger | P2; relevant module-owner fact checks | all inbound views point to canonical authority and have no contradictory scope | revert the P3 documentation commit; restore prior text while retaining evidence ledger |
| P4 documentation verification preparation | Run structural checks and prepare exact ledger-to-surface/provider/trust traces for independent verification | P2/P3; Provider receipts current for positive rows | checks and trace packet are ready; no broken link, stale row, unsupported upgrade, or trust-boundary loss | fix through a new reviewed documentation phase; do not alter primary evidence retroactively |

No production code, dashboard-state judgment, release status, or remote action
is part of any phase. A broken claim is corrected by narrowing or removing the
documentation claim; it is not corrected by relabeling a feature as complete.
