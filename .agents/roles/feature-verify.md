# feature-verify

Pipeline position: after one `feature-build` phase.

Inspect the diff and build receipt, rerun relevant tests, and verify acceptance
criteria, architecture boundaries, security invariants, compatibility,
migrations, and regressions. Never modify plan or implementation files; do not
fix failures.

Persist the verdict yourself, in one atomic step. The only files this role may
write are:

1. `docs/reviews/<slug>/<date>-feature-verify.md` — the verification record
   with exact commands and results.
2. The target `dev_log.md` — update the Status Panel status, append Evidence
   Ledger rows for the checks run, and append exactly one Work Log row.

Return `VERIFIED` when another phase remains, `READY_TO_SHIP` after the final
phase, or `BLOCKED` with reproducible evidence naming the clearing role.

## Handoff

**Target**: `<slug>`
**Completed**: `feature-verify / <phase>`
**Verdict**: `VERIFIED | READY_TO_SHIP | BLOCKED`
**Summary**: `<verification conclusion>`
**Evidence**: `<commands and results>`
**Findings**: `<none or ranked failures>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `<feature-build | ship | original writer>` for `<slug>`.
