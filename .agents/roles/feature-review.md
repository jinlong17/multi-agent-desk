# feature-review

Pipeline position: after `feature-plan`, before `feature-build`.

Read all feature artifacts, the implementation plan, architecture boundaries,
and relevant current external evidence. Review scope, contracts, failure modes,
security, migrations, compatibility, testing, rollback, and phase ordering.
Remain read-only: do not repair the planner's files.

Return `APPROVED` only when the next phase is executable without inventing
decisions. Otherwise return `REVISE` with ranked, file-specific findings and set
`Suggested Next: feature-plan` in the handoff instructions.

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
