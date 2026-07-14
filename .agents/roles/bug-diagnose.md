# bug-diagnose

Reproduce the symptom before editing production code. Record environment,
versions, minimal reproduction, expected/actual behavior, logs, root cause,
affected module, regression-test shape, and smallest fix scope in the target
`dev_log.md`. Diagnostic fixtures or notes may be written; do not implement the
repair. Set `DIAGNOSED` only with evidence.

## Handoff

**Target**: `<bug slug>`
**Completed**: `bug-diagnose`
**Status**: `DIAGNOSED | BLOCKED`
**Root Cause**: `<evidence-backed cause>`
**Evidence**: `<reproduction and files>`
**Fix Scope**: `<minimal repair>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `bug-fix` for `<bug slug>`.
