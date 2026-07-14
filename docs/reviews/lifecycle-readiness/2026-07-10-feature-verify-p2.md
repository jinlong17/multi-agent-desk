# Verification record P2: lifecycle-readiness — READY_TO_SHIP

- Date: 2026-07-10 23:41 -0700
- Role: `feature-verify` (independent session; verdict persisted by this role)
- Target: `lifecycle-readiness`, phase P2 "state machine closure and semantic
  verifier v2" (final phase)
- Object under verification: the uncommitted working tree on `main` (24
  modified + 22 untracked paths, all this governance feature), against
  design.md §"Revision R2" items 1–7, the approved R2-plan review notes
  N1–N6 (`2026-07-10-feature-review-r2-plan.md`), and the test.md acceptance
  matrix.
- Verdict: **READY_TO_SHIP**

## Commands and results

| # | Command | Result |
|---|---|---|
| 1 | `npm run project:verify` | pass — `verified workflow: agents=10, skills=3, docs=17, edges=18, statuses=15`; `verified dashboard: branch=main, dirty=46, phases=9, agents=10, skills=3` |
| 2 | NEG-1: workflow.md `(SPIKE, REVISE)` Next edited `SPIKE_READY`→`NEEDS_REVIEW`; `npm run workflow:verify` | fail — `Error: (SPIKE, REVISE) must re-enter SPIKE_READY and never NEEDS_REVIEW`; restored from cp backup, `cmp` byte-identical |
| 3 | NEG-2: `spike-codex-auth-refresh/dev_log.md` Security Gate set to `none`; `npm run workflow:verify` | fail — `Error: spike-codex-auth-refresh/dev_log.md mentions credentials/keys/auth but Security Gate is none (SOP_SPIKE rule 5)`; restored, `cmp` byte-identical |
| 4 | NEG-3: removed row `(BUGFIX, DRAFT, bug-diagnose)`; `npm run workflow:verify` | fail — `Error: missing entry edge (BUGFIX, DRAFT, bug-diagnose -> DIAGNOSED)`; restored, `cmp` byte-identical |
| 5 | NEG-4: `spike-windows-pty-ipc/dev_log.md` Owner Module set to `frontend`; `npm run workflow:verify` | fail — `Error: spike-windows-pty-ipc/dev_log.md Owner Module frontend is not a module-registry key`; restored, `cmp` byte-identical |
| 6 | `npm run project:verify` after all restores | pass — identical output to command 1; `git status --short` count back to 46 |

All negative injections used `cp` backups under the session scratchpad and
`cmp`-verified byte-identical restores; `git checkout` was never used.

## Acceptance checks

1. **project:verify** — pass (command 1).
2. **State machine (R2 item 1, N1)** — workflow.md §3 is a strict four-column
   `(Workflow, Current, Writer, Next)` table with 18 edges over
   FEATURE_DEV/BUGFIX/SPIKE. The two `SPIKE`/`EVIDENCE_READY` rows split the
   conditional writer per N1 (`security-review` → ACCEPTED/REVISE/BLOCKED;
   `feature-plan` → GATE_RESOLVED/BLOCKED) with the gating condition as prose.
   `BLOCKED` never appears as a Current row (prose recovery rule retained;
   verifier asserts it). `(SPIKE, REVISE)` targets only
   `SPIKE_READY`/`BLOCKED`. BUGFIX entry edge present. Manual union of
   Current+Next = exactly the 15 canonical statuses, independently confirmed
   and verifier-asserted in both directions.
3. **Templates (R2 item 2, N4)** — `docs/workflow/templates/bug_log.md`
   exists: Workflow `BUGFIX`, Suggested Next `bug-diagnose`, reproduction
   fields, and both Provider Gate and Security Gate fields (N4). All three
   templates (dev_log, bug_log, spike_log) carry the nine Status Panel fields;
   the verifier enforces per-template Workflow values
   (dev_log→FEATURE_DEV, bug_log→BUGFIX, spike_log→SPIKE) and Security Gate
   presence.
4. **Spike security gates (R2 item 3)** — `spike-codex-auth-refresh` and
   `spike-claude-config-keychain` both carry
   `Security Gate: open — ... (SOP_SPIKE rule 5); security-review required on
   evidence`, each with an appended Work Log row dated 21:50 recording the
   gate opening. NEG-2 proves the gate is machine-enforced, not decorative.
5. **Single-owner splits (R2 item 4, N3)** — `phase0-security-docs-adrs` and
   `spike-windows-conpty-sidecar` directories are gone from both
   `docs/workflow/features/` and `docs/reviews/`. Successors exist with
   parseable dev_logs: `phase0-architecture-adrs` (owner `project-system`,
   brief present), `phase0-threat-model` (owner `security`, Security Gate
   `open`, brief present), `spike-windows-pty-ipc` (owner `core`, impacted
   provider/desktop), `spike-windows-desktop-sidecar` (owner `desktop`,
   impacted core). All owners are module-registry keys; spikes carry
   hypothesis/time-box/evidence-path per the spike template.
6. **Verifier v2 (R2 item 5, N2, N6)** — `scripts/workflow/verify-workflow.mjs`
   implements: edge-table parsing with four-column and single-writer
   assertions; canonical-set equality both directions; required entry edges
   for all three workflows; spike-REVISE target rule; role-emitted handoff
   statuses each backed by an authoring edge naming that role;
   registry `output` ↔ role handoff bidirectional token equality;
   feature-log `(Workflow, Status)` legality using Current plus terminal-Next
   (terminal computed per workflow exactly as N2 specifies); owner-module
   validity; security-gate heuristic (credential/auth/key/token/secret/
   keychain/e2ee, case-insensitive) applied to Title+Hypothesis panel fields
   only (N6) plus the security-owner rule. Negative injections NEG-1..4
   exercise four distinct check families with the specific expected messages.
7. **Dashboard focus (R2 item 6, N5)** — `scripts/dashboard/verify-static.mjs`
   asserts every `manual.focus` `{slug, expected_status}` matches the
   generated feature logs, with a failure message naming the resolver.
   `dashboard-state.json` `focus` binds `lifecycle-readiness` →
   `READY_FOR_VERIFY` (true at verification time). AGENTS.md names the
   refresh actor ("the operator or the next writer role refreshes `focus`")
   and its State authority section now names `bug_log.md` alongside dev_log
   and spike_log. The build's Evidence Ledger records a live stale-focus
   catch during the P2 transition.
8. **Scope** — the diff touches only `.agents/`, `.claude/agents/`,
   `.codex/agents/`, `AGENTS.md`, `CLAUDE.md`, `docs/adr/`, `docs/reviews/`,
   `docs/workflow/`, `scripts/workflow/`, `scripts/dashboard/`. No product
   code (`cmd/`, `internal/`, `apps/`, `packages/`). The CLAUDE.md diff is
   solely the architecture-boundaries realignment to ADR 0009 (P1 scope of
   this feature); the Security invariants section and human gates are
   unchanged. Registry verdict-writer modes match the role contracts;
   `bug-verify` output is `READY_TO_SHIP or BLOCKED`.
9. **Process conformance (R2 item 7)** — the R1 deviation (operator-approved
   review skip; parent-persisted verify verdict) remains on record in the
   Work Log rows of 2026-07-10 20:56 and 21:10 and in the R2 review row; it
   is not erased. The P2 cycle ran plan (21:20) → independent feature-review,
   self-persisted APPROVED (21:40) → build (21:55) → this independent
   feature-verify, self-persisted.

## Non-blocking observations

- The dev_log Phase Plan says "9 new rows" in test.md while the acceptance
  matrix adds 8 new rows plus one broadened template row. Already flagged as
  a counting nit in the R2-plan review; evidence is unaffected.
- Expected post-verdict state: once this verdict sets the dev_log to
  `READY_TO_SHIP`, the `focus` binding (`READY_FOR_VERIFY`) is stale by
  design and `npm run dashboard:verify` will fail with the named-resolver
  message until the operator or the next writer role refreshes
  `dashboard-state.json`. This verdict writer may not touch that file
  (two-file write scope).

## Handoff

**Target**: `lifecycle-readiness`
**Completed**: `feature-verify / P2 state machine closure and semantic verifier v2`
**Verdict**: `READY_TO_SHIP`
**Summary**: P2 satisfies design R2 items 1–7 and binding notes N1–N6: the
workflow-typed edge table closes all 15 canonical statuses with one writer per
row, verifier v2 enforces the edge semantics end to end (four negative
injections caught with specific messages), spike security gates and
single-owner splits are in place, and the dashboard focus binding is live.
Final phase complete; ship is a human gate.
**Evidence**: `npm run project:verify` pass (edges=18, statuses=15, dirty=46);
NEG-1..4 injection failures and byte-identical restores; file checks in this
record.
**Findings**: none blocking; two non-blocking observations (test.md row-count
nit; by-design stale focus after this verdict, resolver named in AGENTS.md).
**Blockers**: none

### Next Step

Run `ship` for `lifecycle-readiness` (explicit human authorization required);
operator or next writer refreshes `dashboard-state.json` focus first.
