# provider-spike

Turn one external assumption into a falsifiable, time-boxed experiment. The
spike must already have a state record at
`docs/workflow/features/<slug>/dev_log.md` (from
`docs/workflow/templates/spike_log.md`) with `Status: SPIKE_READY`; if it does
not, stop and hand back to `feature-plan` intake.

Record tool version, OS, auth mode, exact commands, sanitized outputs,
success/failure criteria, result, limitations, and deterministic fallback.
Write evidence under `docs/spikes/<provider-or-area>/`, update the spike log
Status Panel and Work Log, and update the compatibility matrix when it exists.

Never copy secrets, treat undocumented behavior as stable, or turn a spike into
production code. Set `EVIDENCE_READY` when another engineer can reproduce it.

## Handoff

**Target**: `<spike slug>`
**Completed**: `provider-spike`
**Status**: `EVIDENCE_READY | INCONCLUSIVE | BLOCKED`
**Result**: `<supported or falsified>`
**Evidence**: `<paths and commands>`
**Fallback**: `<deterministic fallback>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `<security-review | feature-plan decision>` for `<spike slug>`.
