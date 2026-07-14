# Review record R3-plan: lifecycle-readiness P3 plan — APPROVED

- Date: 2026-07-11
- Role: `feature-review` (independent session; verdict persisted by this role)
- Target: `lifecycle-readiness`, phase P3 "security-gate execution paths and
  dashboard truth"
- Object under review: the revised plan (design.md "Revision R3", api.md,
  test.md, dev_log.md at `NEEDS_REVIEW`). The P3 implementation has not been
  built; this review approves the plan, not a diff.
- Verdict: **APPROVED** — P3 is executable by `feature-build` without
  inventing decisions, subject to the derivable implementation notes below.

## Scope reviewed

- `docs/workflow/features/lifecycle-readiness/design.md` §"Revision R3"
- `docs/workflow/features/lifecycle-readiness/{api,test,dev_log}.md`
- Triggering verdict: `docs/reviews/lifecycle-readiness/2026-07-11-feature-review.md`
- Context: `docs/workflow/project/workflow.md`, `AGENTS.md`, `CLAUDE.md`,
  `docs/workflow/SOP_SPIKE.md` rule 5, `.agents/registry.json`,
  `.agents/roles/{feature-review,security-review}.md`,
  `scripts/workflow/{verify-workflow,render-mirrors}.mjs`,
  `scripts/dashboard/verify-static.mjs`,
  `docs/workflow/project/{module-registry,dashboard-state}.json`,
  `docs/workflow/project/dev-dashboard.md`, `docs/workflow/FILE_STRUCTURE.md`,
  `docs/prototypes/dev-dashboard/index.html` (static fallback, lines ~590–700),
  `docs/adr/README.md`, the `phase0-repository-layout` and
  `lifecycle-readiness` briefs, all 12 current feature/spike dev_logs, and the
  R2 plan-review notes N1–N6
  (`docs/reviews/lifecycle-readiness/2026-07-10-feature-review-r2-plan.md`).

## Finding-by-finding resolution check

### P0-A (no legal security-review path for gated Features) — resolved by plan

The proposed edge set is complete and unambiguous:

- `(FEATURE_DEV, READY_TO_SHIP, security-review) → ACCEPTED | REVISE |
  BLOCKED` (gated), `(FEATURE_DEV, READY_TO_SHIP, ship) → SHIPPED | BLOCKED`
  (gate none/resolved), `(FEATURE_DEV, ACCEPTED, ship) → SHIPPED | BLOCKED`,
  and security `REVISE` reusing `(FEATURE_DEV, REVISE, feature-plan)`.
- Mirrors the proven `SPIKE`/`EVIDENCE_READY` two-row gate pattern, including
  keeping the gate condition as prose so each row keeps exactly one writer.
- Verified against the parser in `verify-workflow.mjs`: each new Writer cell
  matches exactly one registry agent name by substring (`security-review`
  contains no other agent name; `ship with human authorization` contains only
  `ship`); no new status tokens, so canonical-set equality (15/15) holds both
  directions; `BLOCKED` never becomes a Current row.
- Terminal-Next interaction is correct: adding `(FEATURE_DEV, ACCEPTED, ship)`
  makes `ACCEPTED` a Current (non-terminal) status of FEATURE_DEV;
  FEATURE_DEV terminals remain {SHIPPED, BLOCKED}. Per-workflow legal resting
  sets only grow, so all 12 existing logs (11 × DRAFT, 1 × NEEDS_REVIEW)
  remain legal — walked the full corpus.
- Role-handoff/edge consistency: `security-review` already emits
  `ACCEPTED | REVISE | BLOCKED`; the existence-check ("≥1 edge naming that
  role") is satisfied by both SPIKE and the new FEATURE_DEV rows. Adding
  `security-review` to registry `workflows.feature` touches only the
  known-agent membership check (passes) and does not enter mirror rendering
  (`render-mirrors.mjs` uses only per-agent name/description/sandbox/tools),
  so no mirror drift or verifier break.
- Gate-to-`resolved` on `ACCEPTED` is already inside security-review's
  documented write scope: its role file line "update the Status Panel status
  and Security Gate field" predates this plan.
- This makes `phase0-threat-model`'s declared acceptance (`security-review
  ACCEPTED`) executable. The rejected alternative (gating at
  `READY_FOR_VERIFY`) is recorded with a sound reason (two verdict writers on
  one state).

### P0-B (gate-selected transitions not enforced) — resolved by plan

- The generic Suggested-Next legality check is executable and was walked
  against every current log: all 11 DRAFT logs name `feature-plan` (legal
  writer of `(FEATURE_DEV|SPIKE, DRAFT)`); `lifecycle-readiness` at
  `NEEDS_REVIEW` names `feature-review` (legal); post-verdict `APPROVED` →
  `feature-build` is legal; historical values like `ship (human gate)` at
  `READY_TO_SHIP` and `feature-build for the next approved phase` at
  `VERIFIED` also parse to legal writers. No log sits at `EVIDENCE_READY` or
  `READY_TO_SHIP` today, so the gate-linkage checks cannot fire spuriously.
  Exempting terminal statuses (SHIPPED, GATE_RESOLVED, BLOCKED) keeps
  BLOCKED's prose clearing-role convention from failing the check.
- Gate linkage is well-specified: gate prefix `open` vs `none`/`resolved` is
  machine-classifiable from the existing Security Gate field (present in all
  templates and all 12 logs; values are `none…` or `open — …`).
- Extended keyword regex (`…|remote control|trust boundar`) produces zero
  false positives on the current corpus **provided** it keeps the R2-N6
  surface (Title + Hypothesis only): no spike Title/Hypothesis contains
  "remote control" or "trust boundar"; the phrases do appear in Security Gate
  values ("no credentials or trust boundaries in scope") and fallback prose
  ("remote-control scope"), which are outside the surface. See note N5.

### P1-C (Windows dual ownership) — resolved by plan

The re-split matches `module-registry.json` signals exactly: `provider`
signals list `PTY` and `ConPTY`; `core` signals list `daemon` and `IPC`.
`spike-windows-conpty` (`provider`, impacted `core, desktop`) and
`spike-windows-named-pipe-ipc` (`core`, impacted `desktop`) are each
single-owner, and the mad-module-classify definitions ("provider: … PTY";
"core: Daemon, local IPC") agree. The successor Titles/Hypotheses as scoped
trip none of the gate keywords, so `Security Gate: none` remains valid for
both.

### P1-D (dashboard truth/authority drift) — resolved by plan

The chosen single rule (`dashboard-state.json` is operator judgment; refresh
executable by an operator-directed writer session recorded in the target Work
Log; never by a verdict writer) is coherent and matches recorded practice
(P2 close-out row; P3 plan row). It requires reconciling three currently
conflicting texts, all named by the plan: AGENTS.md ("operator or the next
writer role"), `dev-dashboard.md` ("Only humans update"), and
`FILE_STRUCTURE.md` edit-ownership row ("operator"). The static-fallback fix
targets the real stale claims (verified in `index.html`: `只读 Agent` modes at
the feature/bug/spike steps, `FIX_READY`/`FIX_READY_FOR_VERIFY` in the bug
flow, `spike-intake` and `REVISE → provider-spike` in the spike flow), and the
blacklist + required `DIAGNOSED`/`READY_FOR_VERIFY` tokens in
`verify-static.mjs` make those exact regressions impossible. The blacklist is
enumerative, not semantic — acceptable for a static fallback, since
workflow.md remains the authority; see note N6 for copy the blacklist cannot
catch.

### P1-E (stale successor references) — resolved by plan

Grep across the tree confirms the plan's cleanup list covers everything
actionable: `docs/adr/README.md:8` (`phase0-security-docs-adrs` as the
deliverable unit), `docs/reviews/phase0-repository-layout/2026-07-10-feature-brief.md:27`
(non-goals pointer), and the lifecycle brief's acceptance counts ("Four
`phase0-*` … five … spike units" → five and seven, which matches the actual
post-P3 unit inventory). All remaining hits are historical: review/verdict
records, Work Log provenance rows, design.md revision-history sections, and
the "Requested by" provenance lines in the two split-unit briefs — correctly
left unrewritten per the plan's append-only rule. See note N7 for making the
test.md grep row deterministic.

## Non-regression check (P1/P2 outcomes)

R3 is strictly additive to the R2 edge table and verifier v2 checks; no P2
edge, template, gate, or focus-binding behavior is removed. Replacing
`spike-windows-pty-ipc` with two successors follows the established R2-N3
supersede pattern (directory removed, successor briefs carried). Process
conformance (R2 item 7) is honored: this is the committed independent review
of the P3 plan, self-persisting its verdict. Scope stays governance-only; no
CLAUDE.md security invariant is touched.

## Implementation notes (binding but derivable — not plan defects)

- **N1 (rows + prose + diagrams).** Implement item 1 as two four-column rows
  at `READY_TO_SHIP` with exactly one agent token per Writer cell, plus a
  mutual-exclusion prose paragraph parallel to the existing
  `EVIDENCE_READY` one. Also update the workflow.md §2 Feature diagram and
  the AGENTS.md "Document-driven lifecycle" Feature line to show the gated
  `security-review` step, or the prose authorities will contradict the table.
- **N2 (security-review role file).** Document the feature-gate duty without
  changing the Handoff Verdict tokens (`ACCEPTED | REVISE | BLOCKED` must
  stay bidirectionally equal to registry `output`); extend its "### Next
  Step" options to include `ship` (after feature `ACCEPTED`). Gate→`resolved`
  reuses the existing "Status Panel status and Security Gate field" write
  scope — do not add a third writable surface.
- **N3 (Suggested-Next parsing).** Detect agent names in Suggested Next the
  same way the edge parser detects writers (substring match against the 10
  registry names); apply the check only when `(Workflow, Status)` is
  non-terminal, defining terminal exactly as verifier v2 does (a status that
  is no row's Current in that workflow: SHIPPED, GATE_RESOLVED, BLOCKED).
- **N4 (gate-linkage symmetry).** Specify the ungated FEATURE_DEV branch
  symmetrically with the SPIKE rule: gate `none`/`resolved` at
  `READY_TO_SHIP` must name `ship` and must not name `security-review`.
  Classify the gate by prefix (`/^open/i` vs `/^(none|resolved)/i`).
- **N5 (keyword surface).** Extend the regex in place but keep the scan
  surface Title + Hypothesis only (reaffirms R2 N6). Scanning the Security
  Gate field would self-trigger on the Windows spikes' "no credentials or
  trust boundaries in scope" wording. Prefer `remote[ -]control` so the
  hyphenated form used elsewhere in the repo cannot slip through a future
  Title.
- **N6 (fallback sweep beyond the blacklist).** While updating `index.html`,
  also fix read-only claims the `只读 Agent` token will not catch — the
  borrowed-patterns line ("Review/Verify 默认只读", ~line 595), the skill
  registry rows for feature-review/feature-verify (~lines 657/659), and the
  checklist row (~line 679) — and add the gated `READY_TO_SHIP →
  security-review → ACCEPTED` branch to the feature `contract` array
  (~line 628) so the fallback matches the new state machine.
- **N7 (re-split mechanics and grep scope).** Instantiate both successor
  spike logs from `spike_log.md` at `DRAFT` / Suggested Next `feature-plan`
  with briefs, and remove `docs/workflow/features/spike-windows-pty-ipc/`
  (R2-N3 pattern). For the test.md "no stale successor references" row,
  define the grep scope explicitly: exclude `docs/reviews/**` records, Work
  Log provenance rows, design.md revision-history sections, and test.md's own
  matrix row — otherwise the row can never read clean, since the plan itself
  legitimately names the superseded slugs as history.

## Nits (no action required before build)

- dev_log Phase Plan and design R3 say "8 P3 rows"; the test.md acceptance
  matrix adds 7 new P3 rows (lines 21–27). Count either way at verification;
  do not let the label drift the evidence (same class as the R2 "9 rows"
  nit).
- The two split-unit briefs' "Requested by … split of
  phase0-security-docs-adrs" lines are provenance and stay as history —
  confirm that reading at build time rather than rewriting them.

## Evidence

- Files listed under "Scope reviewed", read in full in this session.
- Parser interaction traced line-by-line in `verify-workflow.mjs` (writer
  uniqueness l.67–68, canonical equality l.82–84, BLOCKED-not-Current l.85,
  legal resting statuses l.100–108, handoff/edge existence l.128–132,
  keyword surface l.163/l.188).
- Corpus walk of all 12 feature/spike dev_logs for planned Suggested-Next
  legality, gate linkage, and extended keywords: zero spurious failures.
- Grep sweeps for `phase0-security-docs-adrs`, `spike-windows-conpty-sidecar`,
  `spike-windows-pty-ipc`: only the three planned actionable targets plus
  historical records match.
- `render-mirrors.mjs` confirmed to render from per-agent registry fields
  only (registry `workflows` change cannot drift mirrors).

## Handoff

**Target**: `lifecycle-readiness`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: Revision R3 fully resolves all five R3 findings: the gated
FEATURE_DEV ship path is complete, unambiguous, and verifier-compatible; the
Suggested-Next and gate-linkage checks are executable against the current
corpus with no spurious failures; the Windows re-split matches registry
signals; the single dashboard-authority rule is coherent across all three
documents; the reference cleanup covers everything actionable. P3 is
buildable without inventing decisions.
**Findings**: No blocking findings. Binding-but-derivable implementation
notes N1–N7 and two nits recorded above.
**Evidence**: Parser trace of `verify-workflow.mjs`, 12/12 dev_log corpus
walk, keyword false-positive scan, stale-reference greps, module-registry
signal check, static-fallback inspection.
**Blockers**: none

### Next Step

Run `feature-build` for `lifecycle-readiness`.
