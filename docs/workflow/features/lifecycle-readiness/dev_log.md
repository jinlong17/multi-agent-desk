# Development log: Lifecycle readiness hardening

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `lifecycle-readiness` |
| Title | `Lifecycle readiness hardening` |
| Owner Module | `project-system` |
| Impacted Modules | `core, provider, control-plane, web, desktop, security` (docs/registry only) |
| Current Phase | `P3 security-gate execution paths and dashboard truth` |
| Status | `READY_TO_SHIP` |
| Executor | `Claude Code (Fable 5) as feature-verify` |
| Updated | `2026-07-11 11:45 -0700` |
| Suggested Next | `ship (human gate)` |
| Branch / Worktree | `main (operator kept single branch for governance fix)` |
| Plan Version | `v0.2` |
| Provider Gate | `none` |
| Security Gate | `none (no trust-boundary change)` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 governance hardening | ADR 0009 + verdict writers + spike closure + phase breakdown + semantic verifier | 2026-07-10 REVISE review | brief acceptance criteria; `npm run project:verify` passes | superseded by R2 review — `REVISE` |
| P2 state machine closure and semantic verifier v2 | workflow-typed edge table, bug_log template, spike REVISE loop, spike security gates, single-owner splits, verifier v2, dashboard focus binding | R2 review (2026-07-10-feature-review-r2.md) | test.md acceptance matrix incl. 8 new + 1 modified rows; `npm run project:verify` passes | superseded by R3 review — `REVISE` |
| P3 security-gate execution paths and dashboard truth | FEATURE_DEV gated ship path, gate-selected transition enforcement + full SOP keywords, Windows PTY/IPC re-split, single dashboard authority + truthful static fallback, successor-reference cleanup | R3 review (2026-07-11-feature-review.md) | test.md acceptance matrix incl. 7 new + 1 modified P3 rows; `npm run project:verify` passes | `READY_TO_SHIP` |

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-10 20:58 -0700 | P1 | `npm run workflow:generate` | pass — 10 Codex + 10 Claude agents, 3 skill mirrors | `.codex/agents/*`, `.claude/agents/*` |
| 2026-07-10 20:58 -0700 | P1 | `npm run project:verify` | pass — agents=10, skills=3, docs=16, statuses=15; dashboard feature_logs=10 | terminal output |
| 2026-07-10 21:00 -0700 | P1 | Negative check: appended junk to `.claude/agents/ship.md` | `workflow:verify` fails with "drifted from generator output" (restored) | terminal output |
| 2026-07-10 21:00 -0700 | P1 | Negative check: registry `bug-verify` output set to old `VERIFIED or BLOCKED` | `workflow:verify` fails with "missing status READY_TO_SHIP" (restored) | terminal output |
| 2026-07-10 21:00 -0700 | P1 | Negative check: empty `docs/workflow/features/tmp-negcheck/` | `workflow:verify` fails with "missing dev_log.md" (removed) | terminal output |
| 2026-07-10 21:55 -0700 | P2 | `npm run workflow:verify` (verifier v2) | pass — agents=10, skills=3, docs=17, edges=18, statuses=15 | terminal output |
| 2026-07-10 21:55 -0700 | P2 | Live focus check: dev_log still `APPROVED` while focus expected `READY_FOR_VERIFY` | `dashboard:verify` failed with stale-binding message, proving P1-E check; cleared by this transition | terminal output |
| 2026-07-10 22:00 -0700 | P2 | Negative checks NEG-A..E: spike REVISE→NEEDS_REVIEW, removed BUGFIX entry edge, credential spike gate `none`, illegal (FEATURE_DEV, SPIKE_READY) state, invalid owner `frontend` | all five fail `workflow:verify` with the specific message, restored; `project:verify` green after restore | terminal output |
| 2026-07-10 23:41 -0700 | P2 verify | `npm run project:verify` (independent feature-verify session) | pass — edges=18, statuses=15, docs=17; dashboard dirty=46, phases=9 | terminal output |
| 2026-07-10 23:41 -0700 | P2 verify | Independent negative injections NEG-1..4 (cp backup + cmp restore): spike REVISE→NEEDS_REVIEW; codex spike gate `none`; removed BUGFIX entry edge; owner `frontend` | each fails `workflow:verify` with the specific expected message; restores byte-identical; `project:verify` green after | `docs/reviews/lifecycle-readiness/2026-07-10-feature-verify-p2.md` |
| 2026-07-10 23:41 -0700 | P2 verify | File checks: §3 four-column table incl. N1 row split, BLOCKED never Current, 15/15 status closure; bug_log.md N4 gates; 2 spike gates open w/ SOP_SPIKE rule 5; 4 split units + 2 briefs, superseded dirs gone; verifier v2 covers N2/N6; focus binding + AGENTS.md actor; scope governance-only, CLAUDE.md invariants untouched; R1 deviation still on record | all pass | `docs/reviews/lifecycle-readiness/2026-07-10-feature-verify-p2.md` |
| 2026-07-10 23:45 -0700 | P2 close-out | Focus refreshed to READY_TO_SHIP; Phase Plan row and row-count label aligned (operator-directed next writer per AGENTS.md) | `dashboard-state.json`, this file | `npm run project:verify` green | ship (human gate) |
| 2026-07-11 11:00 -0700 | P3 | `npm run workflow:verify` (verifier v3) | pass — agents=10, skills=3, docs=17, edges=20, statuses=15 | terminal output |
| 2026-07-11 11:00 -0700 | P3 | Negative checks NEG-i..v: gated spike EVIDENCE_READY→feature-plan; gated feature READY_TO_SHIP→ship; "trust boundary" hypothesis with gate none; Suggested Next naming non-writer; FIX_READY token in index.html | all five fail the respective verifier with the specific message, restored byte-identically; `workflow:verify` green after | terminal output |
| 2026-07-11 11:45 -0700 | P3 verify | `npm run project:verify` (independent feature-verify session) | pass — edges=20, statuses=15, docs=17; dashboard dirty=50, phases=9 | `docs/reviews/lifecycle-readiness/2026-07-11-feature-verify-p3.md` |
| 2026-07-11 11:45 -0700 | P3 verify | Independent negative injections NEG-V1..V5 (cp backup + cmp restore): gated spike EVIDENCE_READY→feature-plan; gated feature READY_TO_SHIP→ship; trust-boundary hypothesis with gate none; non-writer Suggested Next at (SPIKE, DRAFT); FIX_READY in index.html | each fails the respective verifier with the specific expected message; restores byte-identical; `project:verify` green after | `docs/reviews/lifecycle-readiness/2026-07-11-feature-verify-p3.md` |
| 2026-07-11 11:45 -0700 | P3 verify | File checks: §3 gated ship rows + prose + §2 diagram + AGENTS.md line (N1); registry feature pipeline + security-review role tokens/ship (N2); verifier v3 N3/N4/N5 code paths; phase0-threat-model acceptance reachable; Windows re-split single-owner, old dir gone, no-brief reading accepted per §6 (N7); one authority rule ×3 docs + index.html canonical flows + blacklist (N6); P1-E greps historical-only, 5 features + 7 spikes; scope governance-only, CLAUDE.md invariants untouched; process plan→review→build→verify intact | all pass; findings F1 (edge rows not hard-asserted, caught lazily via gatedStates contradiction — non-blocking) and F2 (stale hint phrasing in verify-static.mjs:68 — cosmetic) | `docs/reviews/lifecycle-readiness/2026-07-11-feature-verify-p3.md` |

## Risks and Blockers

- Verdict writers hold workspace-write sandboxes; scope enforced by role
  contract and diff review (accepted in brief).
- Remote GitHub governance (P1-7) deferred to `phase0-ci-governance`.
- Open non-blocking follow-ups from P3 verify: F1 hard-assert the two gated
  FEATURE_DEV edge rows in verify-workflow.mjs; F2 fix the stale
  "(operator or next writer role)" hint in verify-static.mjs. Fold into the
  next project-system phase; do not patch post-verification.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 | feature-review (read-only session) | Readiness review recorded | `docs/reviews/lifecycle-readiness/2026-07-10-lifecycle-readiness-review.md` | `REVISE` | feature-plan |
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5) as feature-plan | Brief, design, api, test, dev_log created; ADR 0009 decision framed | `docs/reviews/lifecycle-readiness/2026-07-10-feature-brief.md`, `docs/workflow/features/lifecycle-readiness/*` | plan complete | operator gate |
| 2026-07-10 20:56 -0700 | operator | Authorized combined plan+build in one session for this governance fix (explicit human gate) | chat request | `APPROVED` | feature-build |
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5) as feature-build | ADR 0009; CLAUDE.md + module registry realigned; workflow.md state machine closed; 4 verdict-writer roles + feature-plan/provider-spike updated; registry outputs fixed (incl. bug-verify READY_TO_SHIP); spike_log.md added; dev_log template Title added; render-mirrors.mjs extracted; semantic verifier; phase0/spike state records | see git diff of this change | `READY_FOR_VERIFY` | feature-verify |
| 2026-07-10 21:10 -0700 | feature-verify (independent read-only agent; verdict persisted by operator-directed parent) | Verified all 8 acceptance criteria; project:verify pass; 3 negative tests caught; no scope creep | `docs/reviews/lifecycle-readiness/2026-07-10-feature-verify.md` | `READY_TO_SHIP` | ship (human gate) |
| 2026-07-10 21:15 -0700 | feature-review (independent session via operator) | R2 review: bug entry, spike REVISE loop, spike security gates, single-owner, dashboard staleness, process bypass (P1-F: R1 skipped independent review and parent persisted the verify verdict — deviation acknowledged, not repeated), shallow semantics | `docs/reviews/lifecycle-readiness/2026-07-10-feature-review-r2.md` | `REVISE` — READY_TO_SHIP withdrawn | feature-plan |
| 2026-07-10 21:20 -0700 | Claude Code (Fable 5) as feature-plan | P2 revision planned: design.md Revision R2, api.md contracts, test.md acceptance rows | `docs/workflow/features/lifecycle-readiness/{design,api,test,dev_log}.md` | `NEEDS_REVIEW` | feature-review |
| 2026-07-10 21:40 -0700 | Claude Code (Fable 5) as feature-review (independent) | R2 plan reviewed: all 7 R2 findings resolved by plan; edge table closes 15/15 statuses with prose BLOCKED recovery; verifier v2 checks executable (registry↔handoff equality 10/10, spike gate corpus 5/5); notes N1–N6 recorded for builder | `docs/reviews/lifecycle-readiness/2026-07-10-feature-review-r2-plan.md` | `APPROVED` | feature-build (P2) |
| 2026-07-10 21:55 -0700 | Claude Code (Fable 5) as feature-build | P2 built per plan + N1–N6: workflow-typed edge table (N1 row split), bug_log.md template (N4 gates), spike REVISE→SPIKE_READY, Codex/Claude spike gates opened, single-owner splits (phase0-architecture-adrs, phase0-threat-model, spike-windows-pty-ipc, spike-windows-desktop-sidecar; superseded units removed per N3), verifier v2 (N2 terminal-Next, N6 Title+Hypothesis keywords), dashboard focus binding + AGENTS.md actor text (N5) | workflow.md, templates/bug_log.md, 6 feature dirs, verify-workflow.mjs, verify-static.mjs, dashboard-state.json, AGENTS.md | `READY_FOR_VERIFY` | feature-verify |
| 2026-07-10 23:41 -0700 | Claude Code (Fable 5) as feature-verify (independent session; verdict self-persisted) | P2 verified against design R2 items 1–7, notes N1–N6, and test.md matrix: project:verify pass, 4 negative injections caught with specific messages (cp/cmp restore), splits/gates/templates/focus/scope all confirmed; final phase — post-verdict stale focus is by design, resolver named in AGENTS.md | `docs/reviews/lifecycle-readiness/2026-07-10-feature-verify-p2.md` | `READY_TO_SHIP` | ship (human gate) |
| 2026-07-11 02:26 -0700 | Codex (GPT-5) as feature-review (independent session; verdict self-persisted) | R3 review found no legal security-review path for gated Features, bypassable gated-Spike decision routing, remaining Windows PTY/IPC dual ownership, dashboard truth/authority drift, and stale successor references | `docs/reviews/lifecycle-readiness/2026-07-11-feature-review.md` | `REVISE` — READY_TO_SHIP withdrawn | feature-plan |
| 2026-07-11 09:05 -0700 | Claude Code (Fable 5) as feature-plan | P3 revision planned per R3: design.md Revision R3 (gated ship path, gate-selected transitions + full SOP keywords, Windows re-split, one dashboard authority rule + truthful fallback, reference cleanup), api.md contracts, test.md 8 P3 rows; focus refresh to NEEDS_REVIEW recorded here (operator-directed) | `docs/workflow/features/lifecycle-readiness/{design,api,test,dev_log}.md`, `dashboard-state.json` | `NEEDS_REVIEW` | feature-review |
| 2026-07-11 10:30 -0700 | Claude Code (Fable 5) as feature-review (independent session; verdict self-persisted) | R3/P3 plan reviewed: all five R3 findings resolved by plan; gated FEATURE_DEV edges verified against parser one-writer/canonical-set/terminal-Next/legal-resting checks; corpus walk 12/12 logs pass planned Suggested-Next, gate-linkage, and extended-keyword checks with no spurious failures; Windows re-split matches registry signals; dashboard authority rule coherent; cleanup list grep-complete; notes N1–N7 + 2 nits recorded for builder | `docs/reviews/lifecycle-readiness/2026-07-11-feature-review-p3-plan.md` | `APPROVED` | feature-build (P3) |
| 2026-07-11 11:00 -0700 | Claude Code (Fable 5) as feature-build | P3 built per plan + N1–N7: gated FEATURE_DEV ship path (2 READY_TO_SHIP rows + ACCEPTED row + prose, §2 diagram, AGENTS.md lifecycle line — N1), security-review role feature-gate duty + ship Next Step, verdict tokens unchanged (N2), registry feature pipeline + security-review, verifier v3 Suggested-Next legality (N3) + gate linkage with prefix classes (N4) + `remote[ -]control|trust boundar` keywords on Title+Hypothesis only (N5), index.html fallback sweep incl. cockpit/borrowed/skills/checklist lines + contract arrays (N6), Windows re-split spike-windows-conpty (provider) / spike-windows-named-pipe-ipc (core) from spike_log at DRAFT, no briefs per workflow.md §6 spike contract, old dir removed (N7), dashboard blacklist + canonical-token checks, one authority rule across AGENTS.md/dev-dashboard.md/FILE_STRUCTURE.md, reference cleanup (adr README, repo-layout brief, lifecycle brief counts); focus refresh to READY_FOR_VERIFY recorded here (operator-directed) | workflow.md, AGENTS.md, registry.json, security-review role, verify-workflow.mjs, verify-static.mjs, index.html, dev-dashboard.md, FILE_STRUCTURE.md, 2 spike dirs, 3 reference docs, dashboard-state.json | `READY_FOR_VERIFY` | feature-verify |
| 2026-07-11 11:45 -0700 | Claude Code (Fable 5) as feature-verify (independent session; verdict self-persisted) | P3 verified against design R3 items 1–5, notes N1–N7, and test.md P3 matrix: project:verify pass (edges=20), 5 independent negative injections caught with specific messages (cp/cmp restore, green after), gated ship path/verifier v3/re-split/authority rule/fallback/reference cleanup/scope/process all confirmed; F1 (no hard edge asserts, lazily caught) and F2 (hint phrasing) recorded non-blocking; final phase — post-verdict stale focus is by design, operator refreshes dashboard-state.json | `docs/reviews/lifecycle-readiness/2026-07-11-feature-verify-p3.md` | `READY_TO_SHIP` | ship (human gate) |
| 2026-07-11 11:55 -0700 | operator-directed close-out (Claude Code) | Focus refreshed to READY_TO_SHIP; Phase Plan P3 row aligned; F1/F2 follow-ups recorded in Risks (deferred, no post-verify patching) | `dashboard-state.json`, this file | `npm run project:verify` green | ship (human gate) |
