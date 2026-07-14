# Feature review: `user-operations-guide`

- Date: 2026-07-14
- Role: `feature-review`
- Phase reviewed: `P1`
- Owner: `project-system`
- Verdict: `APPROVED`

## Conclusion

P1 is executable without inventing product behavior or unresolved architecture
decisions. The plan consistently treats the guide as pre-release documentation,
keeps every product command behind its owning Phase evidence, and confines the
dashboard work to discoverability and machine facts. It is approved for one
`feature-build` run.

## Classification

```text
Owner: project-system
Confidence: high
Why: the changed contracts are documentation, implementation-plan discovery,
README discovery, dashboard static content, generator facts, and verification
Impacts: core, provider, control-plane, web, desktop, security
Branch: codex/project-system/user-operations-guide
Workflow: feature
Gates: workflow and dashboard verification; no new Provider or Security Gate
Docs: docs/reviews/user-operations-guide/2026-07-14-feature-brief.md and
docs/workflow/features/user-operations-guide/dev_log.md
```

## Review findings

No blocking or revision findings.

### Scope and truthfulness

- The brief, design, contracts, and test strategy all say that product code is
  not currently usable and that only developer workflow/dashboard commands are
  executable today.
- Planned CLI commands come from the reviewed implementation baseline and must
  remain visibly marked as planned until the owning Phase is verified.
- Installer URLs, default ports, service names, output transcripts, Provider
  versions, and Phase 1-6 support claims are explicitly excluded when evidence
  does not exist.
- The current repository corroborates the premise: README still reports Phase
  0, and the planned product directories contain no product source files in this
  checkout.

### Contracts and phase ordering

- One canonical `docs/USER_GUIDE.md` avoids competing user-state authorities.
- README, the implementation-plan document inventory, the dashboard static
  docs section, `required_docs`, and generated feature facts have explicit,
  non-overlapping responsibilities.
- `docs/workflow/features/<slug>/dev_log.md` remains readiness authority; the
  dashboard does not infer approval, priority, risk acceptance, Ship, or release.
- P1 is a coherent documentation/discoverability slice with no dependency on
  unfinished Provider or E2EE implementation. Provider behavior remains gated
  by Phase 0.5 evidence and later feature logs.

### Failure modes, compatibility, and rollback

- Missing/empty-guide and missing-static-entry failures have named verifier
  behavior.
- Link drift and planned-command drift have explicit recovery rules.
- The stable/experimental platform matrix is represented as a target, not as
  current runtime evidence.
- Rollback is limited to the guide and its discovery/assertion references; no
  schema, runtime data, Provider state, or credential migration exists.

### Security and privacy

- The plan preserves Passkey-versus-Device-Key, enrollment, ControllerLease,
  explicit target-scoped grant, and revocation-versus-remote-erasure boundaries.
- Examples require placeholders and prohibit secrets, credential files, local
  secret paths, and realistic tokens.
- No protocol, cryptography, credential, key, or remote-control behavior changes,
  so `Security Gate: none` is appropriate. The same reasoning supports no new
  Provider Gate for this documentation slice.

### Testing executability

- Acceptance covers pre-release truthfulness, journey completeness, discovery,
  required-document facts, link health, negative missing/empty behavior, and the
  repository verifier.
- The negative case is safely scoped to an isolated copy or in-memory fixture;
  the shared guide need not be renamed or deleted.
- The current generator already emits `exists` and `bytes`, and the verifier
  already validates required docs, so adding the guide-specific non-empty and
  static-marker assertions is a bounded implementation.
- `npm run project:verify` was not executed during this review because it runs
  `npm run dashboard` and would modify generated implementation state outside a
  verdict writer's allowed files. It remains required build/verify evidence.

## Build constraints

1. Use `codex/project-system/user-operations-guide` for the build; the current
   `codex/project-system/phase0-repository-layout` checkout is only the planning
   location recorded in the log.
2. Do not modify operator-owned `docs/workflow/project/dashboard-state.json` or
   imply a priority, release, Ship, or phase-completion decision.
3. Regenerate `state.generated.js` only as part of the operator-directed P1
   writer run, record the refresh in the Work Log, and verify both `exists` and
   non-zero `bytes` for the guide.
4. Keep every product command and platform flow visibly planned until its owning
   Phase has verified evidence.

## Evidence reviewed

- `AGENTS.md`
- `CLAUDE.md`
- `.agents/roles/feature-review.md`
- `docs/IMPLEMENTATION_PLAN.md`
- `docs/workflow/project/workflow.md`
- `docs/workflow/project/module-registry.json`
- `docs/reviews/user-operations-guide/2026-07-14-feature-brief.md`
- `docs/workflow/features/user-operations-guide/{design,api,test,dev_log}.md`
- `README.md`
- `package.json`
- `scripts/dashboard/generate-state.mjs`
- `scripts/dashboard/verify-static.mjs`
- `docs/prototypes/dev-dashboard/index.html`
- `git status --short`, current branch, and worktree topology
