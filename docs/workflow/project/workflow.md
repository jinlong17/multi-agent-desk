# MultiAgentDesk workflow policy

## 1. Principles

1. Classify every task into one owning module before editing.
2. Persist state in files; chat is not continuity.
3. One writer owns one phase. Review and verify roles never modify plan or
   implementation files; they persist exactly their own verdict record
   (see §3, "Verdict writers").
4. Build one phase, verify it, then continue.
5. Provider and security assumptions require reproducible Spike evidence.
6. Ship, push, merge, deploy, risk acceptance, and priority are human gates.
7. Never convert unknown, partial, or blocked evidence into pass.

## 2. Workflows

### Feature

```text
mad-feature-brief
  -> feature-plan (NEEDS_REVIEW)
  -> feature-review (APPROVED | REVISE)
  -> feature-build, one phase (READY_FOR_VERIFY)
  -> feature-verify (VERIFIED | READY_TO_SHIP | BLOCKED)
  -> next feature-build phase, or:
  -> security-review when the Security Gate is open (ACCEPTED | REVISE | BLOCKED)
  -> ship (SHIPPED | BLOCKED)
```

### Bugfix

```text
bug intake: copy docs/workflow/templates/bug_log.md (DRAFT)
  -> bug-diagnose (DIAGNOSED)
  -> bug-fix (READY_FOR_VERIFY)
  -> bug-verify (READY_TO_SHIP | BLOCKED)
  -> ship
```

### Spike

```text
feature-plan spike intake (SPIKE_READY)
  -> provider-spike (EVIDENCE_READY | INCONCLUSIVE)
  -> security-review when trust/credential/crypto is affected (ACCEPTED | REVISE | BLOCKED)
  -> feature-plan records the decision: ADR / compatibility matrix (GATE_RESOLVED)
  -> on REVISE: feature-plan re-scopes and re-enters SPIKE_READY (never NEEDS_REVIEW)
```

`feature-plan` owns both ends of a spike: intake (instantiate the spike log
from `docs/workflow/templates/spike_log.md`, freeze the falsifiable hypothesis
and time-box, set `SPIKE_READY`) and the decision (record the ADR or
`PROVIDER_COMPATIBILITY.md` entry, set `GATE_RESOLVED`). When no security gate
is open, `EVIDENCE_READY` goes directly to the `feature-plan` decision.

## 3. State machine

Every transition is one edge `(Workflow, Current, Writer, Next)`. The table is
machine-parsed by `scripts/workflow/verify-workflow.mjs`; keep the four-column
shape and exactly one writer role per row.

| Workflow | Current | Writer | Next |
|---|---|---|---|
| `FEATURE_DEV` | `DRAFT` | `feature-plan` | `NEEDS_REVIEW` |
| `FEATURE_DEV` | `NEEDS_REVIEW` | `feature-review` | `APPROVED`, `REVISE`, `BLOCKED` |
| `FEATURE_DEV` | `REVISE` | `feature-plan` | `NEEDS_REVIEW`, `BLOCKED` |
| `FEATURE_DEV` | `APPROVED` | `feature-build` | `READY_FOR_VERIFY`, `BLOCKED` |
| `FEATURE_DEV` | `READY_FOR_VERIFY` | `feature-verify` | `VERIFIED`, `READY_TO_SHIP`, `BLOCKED` |
| `FEATURE_DEV` | `VERIFIED` | `feature-build` for the next approved phase | `READY_FOR_VERIFY`, `BLOCKED` |
| `FEATURE_DEV` | `READY_TO_SHIP` | `security-review` | `ACCEPTED`, `REVISE`, `BLOCKED` |
| `FEATURE_DEV` | `READY_TO_SHIP` | `ship` with human authorization | `SHIPPED`, `BLOCKED` |
| `FEATURE_DEV` | `ACCEPTED` | `ship` with human authorization | `SHIPPED`, `BLOCKED` |
| `BUGFIX` | `DRAFT` | `bug-diagnose` | `DIAGNOSED`, `BLOCKED` |
| `BUGFIX` | `DIAGNOSED` | `bug-fix` | `READY_FOR_VERIFY`, `BLOCKED` |
| `BUGFIX` | `READY_FOR_VERIFY` | `bug-verify` | `READY_TO_SHIP`, `BLOCKED` |
| `BUGFIX` | `READY_TO_SHIP` | `ship` with human authorization | `SHIPPED`, `BLOCKED` |
| `SPIKE` | `DRAFT` | `feature-plan` spike intake | `SPIKE_READY`, `BLOCKED` |
| `SPIKE` | `SPIKE_READY` | `provider-spike` | `EVIDENCE_READY`, `INCONCLUSIVE`, `BLOCKED` |
| `SPIKE` | `EVIDENCE_READY` | `security-review` | `ACCEPTED`, `REVISE`, `BLOCKED` |
| `SPIKE` | `EVIDENCE_READY` | `feature-plan` | `GATE_RESOLVED`, `BLOCKED` |
| `SPIKE` | `ACCEPTED` | `feature-plan` | `GATE_RESOLVED` |
| `SPIKE` | `REVISE` | `feature-plan` | `SPIKE_READY`, `BLOCKED` |
| `SPIKE` | `INCONCLUSIVE` | `feature-plan` | `SPIKE_READY`, `BLOCKED` |

The two `SPIKE` / `EVIDENCE_READY` rows are mutually exclusive: when the
spike's Security Gate is open, only `security-review` may write; only when no
security gate is open does `feature-plan` record the decision directly.

The two `FEATURE_DEV` / `READY_TO_SHIP` rows are mutually exclusive in the
same way: when the feature's Security Gate is open, only `security-review`
may write — `ACCEPTED` sets the gate to `resolved` and hands the unit to
`ship`; a security `REVISE` returns through the normal
`(FEATURE_DEV, REVISE, feature-plan)` edge. Only when the gate is `none` or
`resolved` may `ship` write directly from `READY_TO_SHIP`.

A spike returned to `REVISE` re-enters `SPIKE_READY` after `feature-plan`
applies the reviewer findings to the hypothesis, criteria, or time-box; a
spike never enters `NEEDS_REVIEW`.

`BLOCKED` is terminal-until-cleared and never appears as a `Current` row: it
is not a fixed edge. It must name a reproducible condition and the role that
can clear it.

Verdict writers. `feature-review`, `feature-verify`, `bug-verify`, and
`security-review` must not modify plan or implementation files. Each persists
its own verdict atomically in one step: update the target `dev_log.md` Status
Panel, append one Work Log row, and write its report under
`docs/reviews/<slug>/`. No other write is permitted for these roles.

The clearing role resolves the condition, appends a Work Log row with
evidence, and restores the last non-blocked status; it may not skip forward
past states that were never reached. Only the owner may set manual priority or
accept residual risk.

## 4. Feature state contract

Each `docs/workflow/features/<slug>/dev_log.md` contains:

- Workflow, Target, Title, Owner Module, Impacted Modules
- Current Phase, Status, Executor, Updated, Suggested Next
- Branch/Worktree, Plan Version, Security/Provider gates
- Phase plan and acceptance criteria
- Evidence ledger
- append-only Work Log

Update the Status Panel in place and append a Work Log row on every transition.
Do not erase prior failures; append their resolution.

## 5. Branch and worktree

- Stable integration branch: `main`.
- Work branch: `codex/<module>/<feature>`.
- Concurrent writers use separate branches/worktrees and non-overlapping files.
- Before writing: inspect `git status --short`, branch, and worktree topology.
- Stage exact files. Do not absorb another tool's dirty files.
- Merge/push only on explicit request.

## 6. Documentation contract

Feature work requires:

```text
docs/reviews/<slug>/<date>-feature-brief.md
docs/workflow/features/<slug>/design.md
docs/workflow/features/<slug>/api.md
docs/workflow/features/<slug>/test.md
docs/workflow/features/<slug>/dev_log.md
```

Bug work requires:

```text
docs/workflow/features/<slug>/dev_log.md   (from docs/workflow/templates/bug_log.md)
```

plus reproduction and regression evidence in its Evidence Ledger.

Spike work requires:

```text
docs/workflow/features/<slug>/dev_log.md   (from docs/workflow/templates/spike_log.md)
docs/spikes/<provider-or-area>/...         (reproducible evidence)
```

plus the ADR or `PROVIDER_COMPATIBILITY.md` entry recorded at `GATE_RESOLVED`.
Per SOP_SPIKE rule 5, a spike touching credentials, keys, auth, remote
control, or trust boundaries must declare an open Security Gate at intake.

Verdict writers store review and verification reports as
`docs/reviews/<slug>/<date>-<role>.md`.

Security-sensitive decisions additionally update the threat model and ADR.
Provider claims additionally update `docs/PROVIDER_COMPATIBILITY.md` after the
Phase 0.5 compatibility matrix is created.

## 7. Handoff

Use the exact `## Handoff` block from `.agents/roles/<role>.md`. The receiving
tool re-reads repository state and never relies solely on the handoff text.

## 8. Required checks

For workflow/dashboard changes:

```bash
npm run workflow:generate
npm run workflow:verify
npm run dashboard
npm run dashboard:verify
```

Product test commands will be added with the Phase 0 monorepo scaffold and must
be recorded in each feature's `test.md` and `dev_log.md`.
