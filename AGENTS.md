# MultiAgentDesk agent rules

This file governs Codex and every delegated agent working in this repository.
Shared architecture and development rules also live in `CLAUDE.md`.

## Read first

Before changing project files, read:

1. `docs/IMPLEMENTATION_PLAN.md`
2. `docs/workflow/project/workflow.md`
3. `docs/workflow/project/module-registry.json`
4. the target feature's `docs/workflow/features/<slug>/dev_log.md`, when present

## Route every task

Classify each task into exactly one owning module with `mad-module-classify`:
`core`, `provider`, `control-plane`, `web`, `desktop`, `security`, or
`project-system`. Record secondary impacts without inventing a second owner.
Use a short-lived branch named `codex/<module>/<feature>` unless the operator
specifies another branch.

## Document-driven lifecycle

- Feature: `intake -> feature-plan -> feature-review -> feature-build -> feature-verify -> security-review (when gated) -> ship`
- Bug: `bug-diagnose -> bug-fix -> bug-verify -> ship`
- Spike: `feature-plan intake -> provider-spike -> security-review (when gated) -> feature-plan decision`
- `feature-build` and `bug-fix` are writers. One writer owns one phase at a time.
- Review and verify roles are verdict writers: they never modify plan or
  implementation files, and they persist their own verdict — target
  `dev_log.md` Status Panel, one Work Log row, and a report under
  `docs/reviews/<slug>/`. No other writes.
- `ship` requires an explicit human request. Tests never authorize push or release.
- A build run completes one approved phase, updates evidence, and stops for verification.

## State authority

`docs/workflow/features/<slug>/dev_log.md` is the resumable state authority
for a feature, bug, or spike (features use
`docs/workflow/templates/dev_log.md`, bugs `bug_log.md`, spikes
`spike_log.md`). Maintain its Status Panel and append-only Work Log on every
workflow transition. Chat history is never the state authority.

Allowed status transitions are defined in
`docs/workflow/project/workflow.md`. Never skip review, verification, security
gates, or provider compatibility gates.

## Handoff contract

Agent final output must end with the exact `## Handoff` structure defined in
the role file under `.agents/roles/`. A parent agent must display that block
verbatim so it remains copyable across Codex and Claude Code.

## Dashboard contract

- Manual judgment: `docs/workflow/project/dashboard-state.json`
- Generated machine facts: `docs/prototypes/dev-dashboard/state.generated.js`
- Refresh: `npm run dashboard`
- Verify: `npm run project:verify`

The dashboard may read branch, commit, dirty files, registries, docs, and
feature logs. It must not decide priority, accept risk, create branches, merge,
push, or mark a release shipped.

`dashboard-state.json` is operator judgment and carries a `focus` array
binding that judgment to concrete feature statuses (`{slug,
expected_status}`); `dashboard:verify` fails when a binding is stale. One
authority rule: the operator owns this file; a refresh may be executed by an
operator-directed writer session, which must record the refresh in the
target unit's Work Log; verdict writers never touch it.
