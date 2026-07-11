# Verification record: lifecycle-readiness P3 — READY_TO_SHIP

- Date: 2026-07-11 11:45 -0700
- Role: `feature-verify` (independent session; verdict self-persisted)
- Target: `lifecycle-readiness`, phase P3 "security-gate execution paths and
  dashboard truth" (final phase)
- Object verified: uncommitted governance diff on `main` at `342be57`,
  against design.md "Revision R3" items 1–5, the approved P3 plan review
  notes N1–N7 (`2026-07-11-feature-review-p3-plan.md`), the triggering R3
  findings P0-A/P0-B/P1-C/P1-D/P1-E (`2026-07-11-feature-review.md`), and
  the test.md P3 acceptance rows.
- Verdict: **READY_TO_SHIP** — all five R3 findings verified closed; one
  low-severity non-blocking finding recorded below.

## 1. Primary command

```
npm run project:verify
```

Pass (run before injections and re-run after every restore):

```
verified workflow: agents=10, skills=3, docs=17, edges=20, statuses=15
generated docs/prototypes/dev-dashboard/state.generated.js: branch=main, dirty=50, agents=10, skills=3
verified dashboard: branch=main, dirty=50, phases=9, agents=10, skills=3
```

edges=20 as expected (18 P2 edges + gated `(FEATURE_DEV, READY_TO_SHIP,
security-review)` + `(FEATURE_DEV, ACCEPTED, ship)`).

## 2. P0-A — gated FEATURE_DEV ship path (design item 1, N1, N2)

- `workflow.md` §3 lines 70–72: two mutually exclusive
  `(FEATURE_DEV, READY_TO_SHIP)` rows — `security-review → ACCEPTED | REVISE
  | BLOCKED` and `ship with human authorization → SHIPPED | BLOCKED` — plus
  `(FEATURE_DEV, ACCEPTED, ship with human authorization → SHIPPED |
  BLOCKED)`. Each Writer cell names exactly one registry agent by substring.
- Mutual-exclusion prose paragraph (lines 89–94) parallels the
  `SPIKE`/`EVIDENCE_READY` one: open gate → only `security-review`;
  `ACCEPTED` sets gate `resolved` and hands to `ship`; security `REVISE`
  returns through `(FEATURE_DEV, REVISE, feature-plan)`; `ship` writes
  directly only at gate `none`/`resolved`. §2 Feature diagram (line 26) shows
  the gated `security-review` step. AGENTS.md "Document-driven lifecycle"
  Feature line includes `security-review (when gated)` (N1). Pass.
- `.agents/registry.json` `workflows.feature` = `[feature-plan,
  feature-review, feature-build, feature-verify, security-review, ship]`.
  Pass.
- `.agents/roles/security-review.md`: "This role gates both workflows"
  paragraph documents the feature-gate duty; write scope unchanged (review
  record + dev_log Status Panel status/Security Gate + one Work Log row);
  Verdict tokens still exactly `ACCEPTED | REVISE | BLOCKED` (bidirectionally
  equal to registry output); Next Step now `feature-plan | provider-spike |
  feature-build | ship` (N2). Pass.
- Reachability: `phase0-threat-model` (`FEATURE_DEV`, owner `security`, gate
  `open`, acceptance "security-review `ACCEPTED`") now reaches its declared
  acceptance via legal edges DRAFT → … → READY_TO_SHIP → security-review →
  ACCEPTED → ship. Pass.

## 3. P0-B — verifier v3 gate enforcement (design item 2, N3–N5)

Code inspection of `scripts/workflow/verify-workflow.mjs`:

- (a) Suggested-Next legality (lines 208–214): agent names detected by
  substring over the 10 registry names, exactly like the edge parser's writer
  detection (line 67); check applies only when `(Workflow, Status)` is
  non-terminal, terminal defined as "no row's Current in that workflow" via
  `currentsByWorkflow` (N3). Pass.
- (b) Gate linkage (lines 114–117, 215–227): `gatedStates` covers
  `SPIKE|EVIDENCE_READY` and `FEATURE_DEV|READY_TO_SHIP`; gate classified by
  `/^open/i` vs `/^(none|resolved)/i` with an explicit assert that the gate
  starts with one of the three; open → must name `security-review` and not
  the ungated writer; symmetric ungated branch → must name
  `feature-plan`/`ship` and not `security-review` (N4). Pass.
- (c) Keyword regex (line 172):
  `credential|auth|key|token|secret|keychain|e2ee|remote[ -]control|trust boundar`
  (hyphen-tolerant per N5); scan surface is Title + Hypothesis only
  (line 197), so the Windows spikes' gate wording ("no credentials … in
  scope") cannot self-trigger. Pass.

### Independent negative injections (cp backup to scratchpad; restore via cp;
### byte-identical restore proven with cmp; never git checkout)

| # | Injection | Result |
|---|---|---|
| NEG-V1 | `spike-codex-auth-refresh` (gate open) Status → `EVIDENCE_READY`, Suggested Next left `feature-plan` | fail: `spike-codex-auth-refresh/dev_log.md has an open Security Gate at (SPIKE, EVIDENCE_READY); Suggested Next must be security-review, not feature-plan` |
| NEG-V2 | `phase0-threat-model` (gate open) Status → `READY_TO_SHIP`, Suggested Next → `ship` | fail: `phase0-threat-model/dev_log.md has an open Security Gate at (FEATURE_DEV, READY_TO_SHIP); Suggested Next must be security-review, not ship` |
| NEG-V3 | `spike-windows-conpty` (gate none) Hypothesis += "across the trust boundary" | fail: `spike-windows-conpty/dev_log.md mentions credentials/keys/auth/remote control/trust boundaries but Security Gate is none (SOP_SPIKE rule 5)` |
| NEG-V4 | `spike-windows-conpty` at `DRAFT`, Suggested Next → `bug-verify` | fail: `spike-windows-conpty/dev_log.md Suggested Next names bug-verify, which is not a legal writer from (SPIKE, DRAFT)` |
| NEG-V5 | `index.html` bug-flow footer → `FIX_READY` | fail (`dashboard:verify`): `dashboard static fallback contains stale token "FIX_READY"` |

Every restore verified byte-identical with `cmp`; `npm run project:verify`
green after the final restore. Matches the build's NEG-i..v claims.

### Probe: required-edge asserts (low-severity finding F1, non-blocking)

Removing the `(FEATURE_DEV, READY_TO_SHIP, security-review)` row — and,
separately, the `(FEATURE_DEV, ACCEPTED, ship)` row — from `workflow.md`
passes `workflow:verify` (edges=19), unlike the three hard-asserted entry
edges. test.md row "Security-gated Feature can reach ACCEPTED" says the edges
are "required". Assessment: the edges exist and are verified in place; no
gate bypass is possible even under removal, because `gatedStates` is a
verifier constant — a gated unit resting at the boundary then fails either
the gate rule (if it names the ungated writer) or the writer-legality rule
(if it names `security-review`), i.e. the drift is caught, but lazily, at
first use rather than at removal time. Design R3 item 2 never promised a
hard assert, and the acceptance row's "required" is satisfiable by presence
plus lazy contradiction. Non-blocking; recommend adding two `hasEdge` asserts
in a later phase. Both probes restored byte-identically (`cmp`) and re-run
green.

## 4. P1-C — Windows single-owner re-split (design item 3, N7)

- `docs/workflow/features/spike-windows-pty-ipc/` is gone.
- `spike-windows-conpty`: Owner `provider`, Impacted `core, desktop`,
  ConPTY full-screen TUI hypothesis, `DRAFT`, Suggested Next `feature-plan`,
  parseable spike log; gate `none (no credentials in scope)` trips no
  keyword. Pass.
- `spike-windows-named-pipe-ipc`: Owner `core`, Impacted `desktop`, Named
  Pipe IPC hypothesis, `DRAFT`, Suggested Next `feature-plan`, parseable;
  gate `none`. Pass. Both match module-registry signals (ConPTY/PTY →
  provider; IPC → core).
- N7 "with briefs" deviation accepted: workflow.md §6 spike contract requires
  only `dev_log.md` + evidence path; no existing spike (0 of 7) carries a
  brief — briefs exist only for the six feature units under `docs/reviews/`.
  The deviation is recorded in the P3 build Work Log row. Accepted.

## 5. P1-D — dashboard authority and truthful fallback (design item 4, N6)

- One authority rule, identical in substance, in all three documents:
  AGENTS.md "Dashboard contract" (operator owns; operator-directed writer
  session may refresh, recorded in the target Work Log; verdict writers never
  touch it), `dev-dashboard.md` layer 1, and `FILE_STRUCTURE.md`
  edit-ownership row for `dashboard-state.json`. Pass.
- `index.html`: zero hits for `只读`, `FIX_READY`, `spike-intake`
  (`readonly:true` is a CSS-class flag only; visible mode text is
  `裁决 Writer`). Feature contract array includes
  `['READY_TO_SHIP（门禁开）','security-review']`,
  `['ACCEPTED','ship（人工触发）']`, and
  `['READY_TO_SHIP（无门禁）','ship（人工触发）']`; bug flow uses
  `DIAGNOSED`/`READY_FOR_VERIFY`; spike flow shows feature-plan intake and
  decision, gated `security-review` at EVIDENCE_READY, and
  `REVISE → SPIKE_READY`. Pass.
- `verify-static.mjs` blacklists `FIX_READY` (substring also covers
  `FIX_READY_FOR_VERIFY`), `spike-intake`, `只读`, and requires canonical
  tokens `DIAGNOSED`, `READY_FOR_VERIFY`, `SPIKE_READY`, `GATE_RESOLVED`,
  `ACCEPTED`. Exercised by NEG-V5. Pass.
- Nit (F2, cosmetic): the focus-staleness hint string at
  `verify-static.mjs:68` still says "(operator or next writer role)" — old
  phrasing in an error-message hint, not one of the three authority
  documents. Non-blocking.

## 6. P1-E — successor-reference cleanup (design item 5, N7 grep scope)

Repo-wide greps for `phase0-security-docs-adrs`,
`spike-windows-conpty-sidecar`, `spike-windows-pty-ipc`: every remaining hit
is historical per the N7 grep-scope definition — `docs/reviews/**` verdict
records, Work Log provenance rows (lifecycle-readiness,
phase0-architecture-adrs, phase0-threat-model, spike-windows-desktop-sidecar,
and the two new spike logs), design.md revision-history sections (R2/R3), and
test.md's own grep-scope row. No actionable document names a superseded slug.
`docs/adr/README.md:8` names `phase0-architecture-adrs`; the
`phase0-repository-layout` brief points to `phase0-ci-governance`,
`phase0-architecture-adrs`, `phase0-threat-model`; the lifecycle brief reads
"Five `phase0-*` feature units and seven Phase 0.5 spike units", matching the
actual inventory (5 features + 7 spikes under `docs/workflow/features/`).
Pass.

## 7. Scope

`git status --porcelain` (50 paths): only `.agents/**`, generated
`.claude/agents/**` + `.codex/agents/**` mirrors, `AGENTS.md`, `CLAUDE.md`,
`docs/adr/`, `docs/reviews/**`, `docs/workflow/**`,
`docs/prototypes/dev-dashboard/index.html`, and `scripts/{workflow,
dashboard}/*.mjs`. No product code. `git diff CLAUDE.md` touches only the
Architecture-boundaries section (P1 layout-authority work); the Security
invariants section is unchanged; ship/push/merge remain explicit human gates.
Pass.

## 8. Process conformance

Work Log shows P3 ran plan (2026-07-11 09:05) → independent self-persisted
`APPROVED` plan review (10:30) → build (11:00) → this independent
self-persisted verification. R1/R2 history rows are intact and unrewritten;
the R1 deviation note remains on record. Pass.

## Findings (ranked, non-blocking)

- F1 (low): the two new FEATURE_DEV edges are not hard-asserted by
  `verify-workflow.mjs` (`hasEdge`-style), unlike the three entry edges;
  removal is caught only lazily via the `gatedStates` contradiction when a
  gated unit rests at the boundary. No bypass is possible. Recommend two
  asserts in a future phase.
- F2 (cosmetic): stale "(operator or next writer role)" phrasing inside the
  `verify-static.mjs:68` error-message hint.

## Post-verdict note

This verdict sets `lifecycle-readiness` to `READY_TO_SHIP`, making the
`dashboard-state.json` focus binding (expects `READY_FOR_VERIFY`) stale by
design; the operator refreshes it. This role does not touch that file.
Security Gate is `none`, so per the new gated-ship rule the legal next writer
is `ship` (human gate).
