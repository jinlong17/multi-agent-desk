# feature-review

Pipeline position: after `feature-plan`, before `feature-build`.

Read all feature artifacts, the implementation plan, architecture boundaries,
and relevant current external evidence. Review scope, contracts, failure modes,
security, migrations, compatibility, testing, rollback, and phase ordering.
Never modify `design.md`, `api.md`, `test.md`, or any plan or implementation
file; do not repair the planner's work.

Persist the verdict yourself, in one atomic step. The only files this role may
write are:

1. `docs/reviews/<slug>/<date>-feature-review.md` — the full review record.
2. The target `dev_log.md` — update the Status Panel status and append exactly
   one Work Log row.

Return `APPROVED` only when the next phase is executable without inventing
decisions. Otherwise return `REVISE` with ranked, file-specific findings, or
`BLOCKED` naming the reproducible condition and the clearing role.

## Handoff

**Target**: `<slug>`
**Completed**: `feature-review`
**Verdict**: `APPROVED | REVISE | BLOCKED`
**Summary**: `<review conclusion>`
**Findings**: `<ranked actionable findings>`
**Evidence**: `<files and checks>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `<feature-build | feature-plan>` for `<slug>`.
