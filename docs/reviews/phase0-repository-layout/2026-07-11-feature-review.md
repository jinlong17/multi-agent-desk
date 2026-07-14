# Feature review: phase0-repository-layout

## Verdict

`APPROVED`

P1 is executable without inventing product, Provider, security, remote, or
maintenance-window decisions. The plan correctly separates reversible local
governance/verifier work from the operator-gated P2 identity transition.

## Review findings

No blocking or revision findings.

Builder notes:

1. Keep `SECURITY.md` reporting instructions truthful; do not invent an email,
   response SLA, or enabled GitHub private-reporting setting.
2. Treat link checking as `unknown` if no local checker exists in P1; the CI
   feature owns the durable link-check gate.
3. F1 must assert the two exact gated FEATURE_DEV edges independently, not
   merely rely on canonical-status or gate-linkage side effects.
4. Do not touch origin, the checkout directory, GitHub settings, push, or merge
   in P1.

## Evidence

- `docs/IMPLEMENTATION_PLAN.md` §17 and §19
- `docs/adr/0009-repository-layout-authority.md`
- feature brief, `design.md`, `api.md`, and `test.md`
- `docs/workflow/project/workflow.md` transition and verdict-writer contracts
- `npm run project:verify` passed after planning: 10 agents, 3 skills, 17 docs,
  20 edges, 15 statuses; dashboard focus matched `NEEDS_REVIEW`.

## Scope and gate assessment

Owner `project-system` is unambiguous. No Security or Provider Gate is opened.
The directory maintenance window and GitHub remote preparation remain explicit
human gates in P2, as required. P1 rollback is file-local and testable.
