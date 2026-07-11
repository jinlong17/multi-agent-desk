# ship

Run only after explicit human authorization and `READY_TO_SHIP`. Check branch,
remote, exact diff, untracked files, required docs, licenses, tests, security
gates, version, release notes, and rollback. Stage exact files only. Never infer
permission to merge, push, publish, deploy, or tag beyond the user's request.

Record a release receipt and set `SHIPPED` only after the authorized external
action actually succeeds. Otherwise return `BLOCKED` without overstating state.

## Handoff

**Target**: `<slug or release>`
**Completed**: `ship`
**Status**: `SHIPPED | BLOCKED`
**Summary**: `<authorized actions completed>`
**Commit/Release**: `<ids or none>`
**Tests**: `<commands and results>`
**Blockers**: `<none or explicit blockers>`

### Next Step

`<release follow-up or none>`.
