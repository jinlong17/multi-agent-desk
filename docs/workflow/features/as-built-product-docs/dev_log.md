# Development log: As-built product documentation authority

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `as-built-product-docs` |
| Title | `As-built product documentation authority` |
| Owner Module | `project-system` |
| Impacted Modules | `core`, `provider`, `control-plane`, `web`, `desktop`, `security` |
| Current Phase | `BUILD P2 CANONICAL PRODUCT DOCUMENT` |
| Status | `READY_FOR_VERIFY` |
| Executor | `Codex as feature-build` |
| Updated | `2026-07-29 PDT` |
| Suggested Next | `feature-verify` |
| Branch / Worktree | `codex/project-system/as-built-product-docs @ /Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/as-built-product-docs` |
| Plan Version | `v0.2` |
| Provider Gate | `open — P2 positive Provider claims require a current immutable provider feature-verify or provider-spike receipt pinned in docs/reviews/as-built-product-docs/claim-ledger.md; stale/missing scope remains unknown and opens a separate provider spike` |
| Security Gate | `open — after final feature-verify reaches READY_TO_SHIP, only independent security-review may accept or return substantive credential, key, E2EE, revocation, trust, or residual-risk wording` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 evidence inventory | create the durable `docs/reviews/as-built-product-docs/claim-ledger.md`, with stable IDs, source/destination headings, full-revision evidence links, conflicts, and freshness bindings | feature-review approval; source evidence available | complete coverage universe is mapped; retained conflict list explicit; receipt gaps are marked; every row uses only planned/preview/supported/experimental/unsupported/unknown | `VERIFIED` |
| P2 canonical `docs/PRODUCT.md` | create dated as-built authority, capability matrix, authority map, non-goals, and trust summary | P1; each positive Provider claim has a current immutable provider feature-verify or provider-spike receipt | all canonical statements match ledger and preserve exact boundaries; no root `PRODUCT.md`; no positive Provider row without a receipt | `READY_FOR_VERIFY` |
| P3 derived-surface reconciliation | update README, user guide, and specialist docs from accepted claim ledger | P2; relevant module-owner fact checks | no contradictory product/support claim across listed surfaces | `PENDING` |
| P4 verification preparation | run structural checks and prepare ledger-to-surface/provider/trust trace material for independent verification | P2/P3; Provider receipts current for positive rows | no broken link, stale positive Provider row, unsupported upgrade, or trust-boundary loss; no review verdict performed in P4 | `PENDING` |

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-29 PDT | PLAN | inspected implementation plan sections 18, 19, and 25; workflow policy; module registry; existing product documentation | `PRODUCT.md` and `docs/PRODUCT.md` are absent; implementation plan names `PRODUCT.md` within its docs set; current surfaces have mixed as-built, preview, and placeholder roles | feature brief; design.md |
| 2026-07-29 PDT | PLAN | inspected README, USER_GUIDE, architecture, data model, provider adapter, compatibility, threat model, roadmap, and feature evidence routing | documentation authority requires an evidence-first canonical synthesis; no existing document may override feature state or compatibility/security evidence | feature brief; api.md; test.md |
| 2026-07-29 PDT | PLAN | inspected branch/worktree topology and created isolated branch from `main@aed5320` | clean isolated planning worktree; no product documentation replaced | this log |
| 2026-07-29 PDT | PLAN | `/opt/homebrew/bin/pnpm run project:verify`; `/opt/homebrew/bin/pnpm run ci:links` | blocked before verifier execution: pnpm 10.23.0 is present but its `env node` launcher cannot find a runnable Node; the discovered Cursor-bundled Node exits `137` in this environment | command output retained in task; this log |
| 2026-07-29 PDT | PLAN | feature-review findings 1-3 resolved in plan artifacts | fixed ledger path/coverage/retention and canonical vocabulary; P2 receipt dependency and P4-to-security-review sequencing are now explicit; freshness policy binds snapshot/receipt dates and revisions with deterministic downgrade | feature brief; design.md; api.md; test.md; this log |
| 2026-07-29 PDT | PLAN | `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run project:verify`; `... pnpm run ci:links`; `git diff --check` | passed: workflow `agents=10, skills=3, docs=17, edges=20, statuses=15`; dashboard verification passed; 308 Markdown files passed link validation; no whitespace errors. Initial rerun correctly rejected `NEEDS_REVIEW -> feature-plan`; after setting Suggested Next to `feature-review`, the legal-state rerun passed | this log; command output retained in task |
| 2026-07-29 PDT | BUILD P1 | inventoried every approved product-facing source heading in the durable append-only claim ledger; bound 20 stable `CLM-###` claims to full-revision evidence, exact Provider receipts where valid, explicit gaps/conflicts, destination anchors, and day-31/day-91 downgrade rules | `docs/reviews/as-built-product-docs/claim-ledger.md`; no commit | P1 complete and `READY_FOR_VERIFY`; no `docs/PRODUCT.md`, README, specialist derived document, runtime, Provider evidence, security verdict, dashboard-state, push, or ship change | independent `feature-verify` must inspect P1 coverage, receipt scope/reachability, conflicts, and stale-data downshifts before P2 |
| 2026-07-29 PDT | BUILD P1 | `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run ci:links`; `... pnpm run project:verify`; `git diff --check` | passed: 309 Markdown files passed local-link validation; workflow `agents=10, skills=3, docs=17, edges=20, statuses=15`; dashboard static verification passed; no whitespace errors. These are documentation/workflow structural results only. | this log; command output retained in task | independent `feature-verify` |
| 2026-07-29 PDT | BUILD P1 REPAIR | Cleared the independent P1 verification blocker by appending `CLM-021` through `CLM-023` and their source/destination rows for User Guide `9.1` Control Plane deployment, `9.2` device pairing, and `9.3` Web/Desktop use; retained all prior claims and conflict history | `docs/reviews/as-built-product-docs/claim-ledger.md`; no commit | restored the last non-blocked status, `READY_FOR_VERIFY`; the three rows remain `planned`, preserve Passkey/pinning/E2EE boundaries, and introduce no P2/P3 or support claim | fresh independent `feature-verify` for P1 |
| 2026-07-29 PDT | BUILD P1 REPAIR | `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run ci:links`; `... pnpm run project:verify`; `git diff --check` | passed: 310 Markdown files passed local-link validation; workflow `agents=10, skills=3, docs=17, edges=20, statuses=15`; dashboard static verification passed; no whitespace errors. These are structural results only. | this log; command output retained in task | fresh independent `feature-verify` for P1 |
| 2026-07-29 PDT | BUILD P2 | Created the canonical dated `docs/PRODUCT.md` from the independently verified P1 ledger: source-built/release boundary, authority map, classed capability matrix, non-goals, trust/residual-risk boundary, and deterministic freshness downgrade are explicit | `docs/PRODUCT.md`; no commit | P2 is `READY_FOR_VERIFY`; only receipt-bound exact Codex `0.144.2` Linux rows are positive, while broad/missing Provider evidence remains `unknown`, `planned`, or `unsupported`; no P3 derived-surface, Provider evidence, security verdict, dashboard-state, push, or ship change | independent `feature-verify` for P2 |
| 2026-07-29 PDT | BUILD P2 | `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run ci:links`; `... pnpm run project:verify`; `git diff --check` | passed: 311 Markdown files passed local-link validation; workflow `agents=10, skills=3, docs=17, edges=20, statuses=15`; dashboard static verification passed; no whitespace errors. These are structural results only. | this log; command output retained in task | independent `feature-verify` for P2 |

## Risks and Blockers

- Provider and security gates are deliberately open. This planning phase creates
  no support or trust claim and cannot resolve either gate.
- A current-support statement without exact evidence must be narrowed or made
  `unknown`; documentation work cannot cure a missing platform/provider test.
- The ledger is fixed as an append-only Markdown review artifact; it must be
  retained even when documentation prose is rolled back.
- Provider evidence still requires a separate valid producer; P1/P2 record or
  consume it but cannot create or accept it. Security acceptance remains the
  independent post-`READY_TO_SHIP` workflow step.
- Dashboard state is operator judgment and is intentionally untouched.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-29 PDT | Codex as feature-plan | Classified documentation-authority drift as `project-system`; created canonical feature brief and planning contracts; resolved target path as `docs/PRODUCT.md`; opened Provider and Security Gates for substantive claims | `docs/reviews/as-built-product-docs/2026-07-29-feature-brief.md`; `docs/workflow/features/as-built-product-docs/{design,api,test,dev_log}.md`; no commit | `NEEDS_REVIEW`; no production documentation, dashboard, Provider evidence, Security evidence, push, or ship action performed | `feature-review` |
| 2026-07-29 PDT | Codex as feature-plan | Attempted required structural project and local-link checks; ran diff hygiene review | structural commands blocked by unavailable/runnable Node; documentation plan remains reviewable; no failed check was reclassified as a product failure | this log; no commit | provision a supported Node 24 runtime, rerun `project:verify` and `ci:links`, then run `feature-review` |
| 2026-07-29 PDT | Codex as feature-review | Independently reviewed plan commit `5e5670a` for authority, evidence boundaries, trust/provider gates, workflow sequencing, and testability | `docs/reviews/as-built-product-docs/2026-07-29-feature-review.md`; `git diff --check 5e5670a^ 5e5670a` passed; current shell has no runnable `node`, so Node-backed structural checks remain unavailable | `REVISE`: P1 inventory location/coverage and freshness policy are undecided; P2/P4 place Provider and Security reviews in contradictory phases and do not bind gate resolution to the workflow | `feature-plan` must resolve the ranked findings, return to `NEEDS_REVIEW`, then request a new feature review |
| 2026-07-29 PDT | Codex as feature-plan | Resolved feature-review findings 1-3 without changing product documentation or verdict reports: fixed an append-only claim-ledger contract, canonical classes/freshness downgrades, valid Provider evidence receipt producers, and post-verification Security Gate sequencing | feature brief; `docs/workflow/features/as-built-product-docs/{design,api,test,dev_log}.md`; no commit | `NEEDS_REVIEW`; Provider Gate remains open unless every positive Provider row later binds a current receipt, and Security Gate remains open until independent review after `READY_TO_SHIP` | rerun supported Node structural checks, then `feature-review` |
| 2026-07-29 PDT | Codex as feature-review | Re-reviewed revised plan commit `a14d395` for prior ledger/retention/coverage, Provider/Security producer ordering, and freshness/vocabulary findings | `docs/reviews/as-built-product-docs/2026-07-29-feature-review.md`; Node `v24.11.1` with `/opt/homebrew/bin/pnpm run project:verify` and `ci:links` passed; `git diff --check a14d395^ a14d395` passed | `APPROVED`: P1 ledger, P2 Provider receipt precondition, P4 verification preparation, post-`READY_TO_SHIP` Security review, and deterministic stale-evidence downshifts are executable; no support/trust claim, gate acceptance, push, merge, or release action was performed | `feature-build` |
| 2026-07-29 PDT | Codex as feature-verify | Independently verified P1 build commit `a54d333` for ledger identity/vocabulary, exact reachable positive Provider receipts, conflicts, freshness rules, support-boundary preservation, structural checks, and complete source-heading coverage | `docs/reviews/as-built-product-docs/2026-07-29-feature-verify.md`; `ci:links` (309 files), `project:verify`, and `git diff --check a54d333^ a54d333` passed; three substantive User Guide 9.1/9.2/9.3 anchors are absent from the ledger coverage map | `BLOCKED`: full source-heading coverage is not established; Provider and Security Gates remain open and no P2/support/security/release acceptance is implied | `feature-build` must append traceable coverage for the three omitted headings and return P1 to `READY_FOR_VERIFY` |
| 2026-07-29 PDT | Codex as feature-verify | Independently re-verified P1 repair commit `1165a13` for closure of F1, planned/no-support boundaries, existing claim/conflict/receipt preservation, and structural/link hygiene | `docs/reviews/as-built-product-docs/2026-07-29-feature-verify.md`; CLM-021/022/023 and all three User Guide 9.1/9.2/9.3 coverage-map anchors present; CLM-001..020 and CON-001..003 unchanged; exact positive Codex receipts remain reachable; `workflow:verify`, `ci:links` (310 files), and `git diff --check 1165a13^ 1165a13` passed | `VERIFIED`: P1 inventory is complete; Provider and Security Gates remain open and no P2/support/security/release acceptance is implied | `feature-build` for P2 canonical `docs/PRODUCT.md` |
