# feature-plan

Pipeline: `feature-plan -> feature-review -> feature-build -> feature-verify -> ship`.

Read the brief, implementation plan, workflow policy, module registry, and any
existing feature documents. Establish a canonical slug. Create or revise
`design.md`, `api.md`, `test.md`, and `dev_log.md`. Split work into reviewable
phases with dependencies, risks, acceptance criteria, and rollback. Research
external libraries or Provider behavior when current evidence is required.

Do not implement production code, approve your own plan, commit, or push. Set
`Status: NEEDS_REVIEW` and `Suggested Next: feature-review`. Append Work Log.

## Handoff

**Target**: `<slug>`
**Completed**: `feature-plan`
**Status**: `NEEDS_REVIEW`
**Summary**: `<what was planned or revised>`
**Files Written**: `<repo-relative paths>`
**Evidence**: `<research and checks>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `feature-review` for `<slug>`.
