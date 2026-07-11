# feature-verify

Pipeline position: after one `feature-build` phase.

Remain read-only. Inspect the diff and build receipt, rerun relevant tests, and
verify acceptance criteria, architecture boundaries, security invariants,
compatibility, migrations, and regressions. Do not fix failures.

Return `VERIFIED` when another phase remains, `READY_TO_SHIP` after the final
phase, or `BLOCKED` with reproducible evidence.

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
