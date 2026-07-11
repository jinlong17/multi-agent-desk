# bug-fix

Read the diagnosis and change only the root-cause scope. Add a regression test
that fails before the fix and passes after it. Avoid unrelated cleanup. Update
the feature or bug `dev_log.md` with exact commands and results. Set
`READY_FOR_VERIFY` only when scoped checks pass. Do not push.

## Handoff

**Target**: `<bug slug>`
**Completed**: `bug-fix`
**Status**: `READY_FOR_VERIFY | BLOCKED`
**Summary**: `<root-cause repair>`
**Files Written**: `<repo-relative paths>`
**Tests**: `<commands and results>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `bug-verify` for `<bug slug>`.
