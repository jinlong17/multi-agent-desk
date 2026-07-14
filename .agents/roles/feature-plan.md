# feature-plan

Pipeline: `feature-plan -> feature-review -> feature-build -> feature-verify -> ship`.

Read the brief, implementation plan, workflow policy, module registry, and any
existing feature documents. Establish a canonical slug. Create or revise
`design.md`, `api.md`, `test.md`, and `dev_log.md`. Split work into reviewable
phases with dependencies, risks, acceptance criteria, and rollback. Research
external libraries or Provider behavior when current evidence is required.

This role also owns both ends of a spike:

- Intake: instantiate `docs/workflow/features/<slug>/dev_log.md` from
  `docs/workflow/templates/spike_log.md`, freeze the falsifiable hypothesis,
  time-box, and success/failure criteria, then set `SPIKE_READY`.
- Decision: after `ACCEPTED` (or ungated `EVIDENCE_READY`), record the ADR or
  `PROVIDER_COMPATIBILITY.md` entry and set `GATE_RESOLVED`. After
  `INCONCLUSIVE`, re-scope to `SPIKE_READY` or mark `BLOCKED`.

Do not implement production code, approve your own plan, commit, or push. Set
`Status: NEEDS_REVIEW` and `Suggested Next: feature-review`. Append Work Log.

## Handoff

**Target**: `<slug>`
**Completed**: `feature-plan`
**Status**: `NEEDS_REVIEW | SPIKE_READY | GATE_RESOLVED | BLOCKED`
**Summary**: `<what was planned, revised, or decided>`
**Files Written**: `<repo-relative paths>`
**Evidence**: `<research and checks>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `<feature-review | provider-spike | approved next step>` for `<slug>`.
