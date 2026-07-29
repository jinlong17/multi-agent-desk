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
- Provider and Security Gates are open. Provider owners must fact-check
  substantive Provider scope; independent security review is required before
  ship for substantive credential, cryptographic, trust, revocation, or
  residual-risk wording.

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

## Claim vocabulary

Every product-facing capability statement must use one of these meanings.

| Label | Meaning | Required evidence/wording |
|---|---|---|
| `target` / `planned` | reviewed desired v0.1 behavior not yet proven as current product behavior | implementation-plan or approved feature-plan link; no runnable instruction |
| `source-built preview` | runnable development path from source, not an installation/release promise | exact feature state and command scope; identify omissions |
| `supported` | exact behavior independently evidenced for a specified version, platform, and capability | exact evidence link, fallback, and date; no generalization |
| `experimental` | intentionally non-stable behavior with stated limits | explicit platform/scope and non-stable label |
| `unsupported` | intentionally not available or ruled out by evidence/policy | preserve reason and safe fallback where one exists |
| `unknown` / `pending` | no current reproducible evidence or a gate remains open | name the gate/owner; never present as success |

Phase labels, green CI, a build, a configured binary, a dashboard snapshot,
and a feature branch are not standalone support labels.

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

## Reconciliation design

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
- `docs/ROADMAP.md` — target order without current-support upgrades.

Each inventory entry records the claim, source text, class, evidence target,
required reviewer, proposed state, and destination. The implementation must
change a claim's evidence first when needed, then reconcile all derived text
in the same build phase. A placeholder may remain only if it explicitly
declares its limited scope and links to the canonical product document.

## Phased implementation and rollback

| Phase | Scope | Dependencies | Acceptance | Rollback |
|---|---|---|---|---|
| P1 evidence inventory | Build a dated claim ledger; resolve every current statement as target, preview, supported, experimental, unsupported, or unknown | approved plan; access to linked logs/reviews | no unbound substantive claim; conflict list names owner/gate | remove the new inventory artifact; do not rewrite evidence |
| P2 canonical product document | Author `docs/PRODUCT.md` with authority map, snapshot, capability matrix, trust summary, and evidence links | P1; Provider fact review for Provider statements | all canonical claims match ledger; no root `PRODUCT.md` | revert document-only commit |
| P3 derived document reconciliation | Update the README, user guide, and specialist documents from the accepted ledger | P2; relevant module-owner fact checks | all inbound views point to canonical authority and have no contradictory scope | revert the P3 documentation commit; restore prior text while retaining evidence ledger |
| P4 independent documentation verification | Run structural checks; manually trace product, Provider, and trust claims; perform required reviews | P2/P3; Provider Gate evidence; Security Gate review | no broken local links, stale authority map, unsupported upgrade, or trust-boundary loss | fix via a new reviewed documentation phase; do not alter evidence retroactively |

No production code, dashboard-state judgment, release status, or remote action
is part of any phase. A broken claim is corrected by narrowing or removing the
documentation claim; it is not corrected by relabeling a feature as complete.
