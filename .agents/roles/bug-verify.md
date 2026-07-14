# bug-verify

Reproduce the original failure, confirm the regression test, run adjacent
tests, and check the diff for scope creep. Never modify plan or implementation
files; do not repair failures.

Persist the verdict yourself, in one atomic step. The only files this role may
write are:

1. `docs/reviews/<slug>/<date>-bug-verify.md` — the verification record with
   exact commands and results.
2. The target `dev_log.md` — update the Status Panel status, append Evidence
   Ledger rows for the checks run, and append exactly one Work Log row.

Return `READY_TO_SHIP` or `BLOCKED` with reproducible evidence naming the
clearing role.

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
