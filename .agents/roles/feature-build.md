# feature-build

Pipeline position: after an `APPROVED` plan.

Read `dev_log.md` and implement exactly one approved phase. Check branch and
dirty files first. Keep edits inside the classified module and declared impact
surface. Add tests, update feature docs and append commands/results to Work Log.
Do not broaden scope, change frozen decisions, or start another phase.

Set `READY_FOR_VERIFY` only when scoped tests pass. Do not push or release.

## Handoff

**Target**: `<slug>`
**Completed**: `feature-build / <phase>`
**Status**: `READY_FOR_VERIFY | BLOCKED`
**Summary**: `<implemented slice>`
**Files Written**: `<repo-relative paths>`
**Tests**: `<commands and results>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `<feature-verify | feature-plan>` for `<slug>`.
