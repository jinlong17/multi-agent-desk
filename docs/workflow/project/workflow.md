# MultiAgentDesk workflow policy

## 1. Principles

1. Classify every task into one owning module before editing.
2. Persist state in files; chat is not continuity.
3. One writer owns one phase. Reviewers and verifiers remain read-only.
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
  -> next feature-build phase or ship
  -> ship (SHIPPED | BLOCKED)
```

### Bugfix

```text
bug-diagnose (DIAGNOSED)
  -> bug-fix (READY_FOR_VERIFY)
  -> bug-verify (READY_TO_SHIP | BLOCKED)
  -> ship
```

### Spike

```text
spike intake (SPIKE_READY)
  -> provider-spike (EVIDENCE_READY | INCONCLUSIVE)
  -> security-review when trust/credential/crypto is affected
  -> feature-plan / ADR / compatibility matrix (GATE_RESOLVED)
```

## 3. State machine

| Current | Role allowed to write next | Next statuses |
|---|---|---|
| `DRAFT` | `feature-plan` | `NEEDS_REVIEW` |
| `NEEDS_REVIEW` | reviewer is read-only; planner writes only after `REVISE` | `APPROVED`, `REVISE`, `BLOCKED` |
| `REVISE` | `feature-plan` | `NEEDS_REVIEW` |
| `APPROVED` | `feature-build` | `READY_FOR_VERIFY`, `BLOCKED` |
| `READY_FOR_VERIFY` | verifier is read-only | `VERIFIED`, `READY_TO_SHIP`, `BLOCKED` |
| `VERIFIED` | `feature-build` for next approved phase | `READY_FOR_VERIFY` |
| `READY_TO_SHIP` | `ship`, only with human authorization | `SHIPPED`, `BLOCKED` |
| `DIAGNOSED` | `bug-fix` | `READY_FOR_VERIFY`, `BLOCKED` |
| `SPIKE_READY` | `provider-spike` | `EVIDENCE_READY`, `INCONCLUSIVE`, `BLOCKED` |
| `EVIDENCE_READY` | reviewer is read-only; planner/ADR writer records decision | `GATE_RESOLVED`, `REVISE`, `BLOCKED` |

`BLOCKED` must name a reproducible condition and the role that can clear it.
Only the owner may set manual priority or accept residual risk.

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
