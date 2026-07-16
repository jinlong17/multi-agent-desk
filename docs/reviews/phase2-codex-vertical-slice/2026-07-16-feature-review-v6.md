# Feature Review: Codex Vertical Slice v0.6 P4S

## Verdict

`REVISE`

The remediation scope and ownership are correct, but the proposed structural
`NO_PROXY` contract is not yet executable without implementation-time policy
decisions. Exact numeric bounds and token grammar must be frozen before build.

## Findings

### P1 — The network-entry grammar remains underspecified

`design.md`, `api.md`, and `test.md` say bounded domain/IP/CIDR/optional-port
entries, but do not define the maximum entry count, maximum entry length,
domain-label rules, wildcard/leading-dot behavior, port range, bracketed IPv6
behavior, or whether whitespace normalization is accepted. Different reasonable
implementations would expose different Provider environments, so the builder
cannot choose safely.

Freeze one exact parser contract and positive/negative table. It must preserve
the real Linux `NO_PROXY` shape required by P3B or explicitly remove inheritance
and rerun the live gate.

### P1 — Artifact-scan exceptions are not frozen

The plan correctly expands scanning to modified and untracked files, but the
tree intentionally contains synthetic adversarial strings such as credentialed
proxy fixtures. Define whether such hits are forbidden or manually classified,
and require the scanner to emit file/class only. Otherwise a clean result can
either hide a real identifier or fail on an intentional test fixture.

## Evidence

- v0.6 feature brief, design, API, test strategy, and state authority
- 2026-07-16 Security Review clearing conditions
- current `NetworkEnvironment` implementation and adversarial fixture strings
- workflow/module/security boundaries

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: `P4S has the correct narrow scope but must freeze exact NO_PROXY numeric/token grammar and artifact-scan exception handling before build.`
**Findings**: `P1 parser bounds and syntax are underspecified; P1 synthetic-fixture scan handling is underspecified.`
**Evidence**: `v0.6 brief/design/api/test; Security Review; current environment validator and fixture strings.`
**Blockers**: `feature-plan must publish an exact parser and scanner decision table.`

### Next Step

Run `feature-plan` for `phase2-codex-vertical-slice` v0.7.
