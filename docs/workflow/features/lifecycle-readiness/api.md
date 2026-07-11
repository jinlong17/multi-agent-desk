# Contracts: Lifecycle readiness hardening

## Public interfaces

No runtime API. The public contracts are file contracts:

- `docs/workflow/project/workflow.md` §3 — state machine: a
  `(Workflow, Current, Writer, Next)` edge table over workflows
  `FEATURE_DEV`, `BUGFIX`, `SPIKE` and fifteen canonical statuses; the
  verifier parses this table, so rows must keep the four-column shape
  (`BLOCKED` recovery is prose, not a row).
- `.agents/registry.json` — agent modes: `writer`, `verdict-writer`,
  `diagnostic-writer`, `evidence-writer`, `release-writer`; each agent's
  `output` status tokens must equal its role-handoff status tokens
  (bidirectional).
- `docs/workflow/templates/dev_log.md`, `bug_log.md`, and `spike_log.md` —
  Status Panel must contain: Workflow, Target, Title, Owner Module, Current
  Phase, Status, Executor, Updated, Suggested Next. Workflow values map:
  dev_log→`FEATURE_DEV`, bug_log→`BUGFIX`, spike_log→`SPIKE`.
- `docs/workflow/project/dashboard-state.json` — `focus`: array of
  `{slug, expected_status}` binding manual judgment to feature logs;
  `dashboard:verify` fails when an entry does not match the log. Authority:
  operator judgment; refresh executable by an operator-directed writer
  session (recorded in the target Work Log); never by a verdict writer.
- Security-gate execution contract: when a Status Panel `Security Gate`
  starts with `open`, the gated writer (`security-review`) is the only legal
  Suggested Next at `SPIKE`/`EVIDENCE_READY` and `FEATURE_DEV`/
  `READY_TO_SHIP`; `ACCEPTED` sets the gate to `resolved`. Suggested Next
  must always name only legal writers for the current `(Workflow, Status)`.
- Static dashboard fallback (`docs/prototypes/dev-dashboard/index.html`)
  must use canonical statuses and verdict-writer role semantics;
  `dashboard:verify` blacklists `FIX_READY`, `FIX_READY_FOR_VERIFY`,
  `spike-intake`, and `只读 Agent`.
- Verdict record path: `docs/reviews/<slug>/<date>-<role>.md`.
- Module ownership: `docs/workflow/project/module-registry.json`
  (schema_version 2, `layout_authority` pointer), mapping physical paths per
  ADR 0009.

## Requests, events, and responses

Not applicable (documentation and script contracts only).

## Error semantics

`npm run workflow:verify` and `npm run dashboard:verify` exit non-zero with a
message naming the offending file and, for generated files, the regeneration
command.

## Authentication and authorization

Human gates unchanged: priority, risk acceptance, phase completion, branch
creation, merge, push, ship, release.

## Idempotency, ordering, and replay

`npm run workflow:generate` is idempotent; verifiers are read-only except the
dashboard generator's single output file.

## Versioning and compatibility

Registry `schema_version` 1 (agents), module registry `schema_version` 2.
Canonical status list changes require updating workflow.md, roles, registry,
and verifier together.

## Data retention and deletion

Work Logs and verdict records are append-only; prior failures are never
erased, only resolved.
