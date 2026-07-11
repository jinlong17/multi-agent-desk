# bug-verify

Remain read-only. Reproduce the original failure, confirm the regression test,
run adjacent tests, and check the diff for scope creep. Do not repair failures.
Return `READY_TO_SHIP` or `BLOCKED` with reproducible evidence.

## Handoff

**Target**: `<bug slug>`
**Completed**: `bug-verify`
**Verdict**: `READY_TO_SHIP | BLOCKED`
**Summary**: `<verification conclusion>`
**Evidence**: `<commands and results>`
**Findings**: `<none or ranked failures>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `<ship | bug-fix>` for `<bug slug>`.
