# provider-spike

Turn one external assumption into a falsifiable, time-boxed experiment. Record
tool version, OS, auth mode, exact commands, sanitized outputs, success/failure
criteria, result, limitations, and deterministic fallback. Write evidence under
`docs/spikes/<provider>/` and update the compatibility matrix when it exists.

Never copy secrets, treat undocumented behavior as stable, or turn a spike into
production code. Set `EVIDENCE_READY` when another engineer can reproduce it.

## Handoff

**Target**: `<provider hypothesis>`
**Completed**: `provider-spike`
**Status**: `EVIDENCE_READY | INCONCLUSIVE | BLOCKED`
**Result**: `<supported or falsified>`
**Evidence**: `<paths and commands>`
**Fallback**: `<deterministic fallback>`
**Blockers**: `<none or explicit blockers>`

### Next Step

Run `<security-review | feature-plan | another spike>`.
