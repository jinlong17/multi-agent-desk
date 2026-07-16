# Feature verification: `user-operations-guide`

- Date: 2026-07-14
- Role: `feature-verify`
- Phase verified: `P1` (final phase)
- Owner: `project-system`
- Verdict: `READY_TO_SHIP`

## Conclusion

P1 satisfies every approved acceptance criterion. The canonical guide is
truthful about the Phase 0 maturity level, covers the requested user journeys,
keeps all product commands visibly planned, preserves the security boundaries,
and is discoverable from README, the implementation-plan document inventory,
and the dashboard. Static, link, required-document, workflow, and dashboard
checks all pass independently.

No Browser QA was performed in this verification run. The build receipt's
existing DOM/console browser result was reviewed as supplementary evidence;
its timed-out screenshot remains explicitly unverified and is not needed for
the approved static acceptance criteria.

## Classification and scope

```text
Owner: project-system
Confidence: high
Why: all implementation changes are documentation, dashboard static content,
generator facts, and verifier contracts
Impacts: core, provider, control-plane, web, desktop, security
Branch: codex/project-system/user-operations-guide
Workflow: feature
Gates: workflow and dashboard verification; Security Gate none; no new Provider Gate
Docs: docs/reviews/user-operations-guide/2026-07-14-feature-brief.md and
docs/workflow/features/user-operations-guide/dev_log.md
```

The scoped diff contains only the approved guide/discovery/dashboard contract
files plus the feature artifacts. No product runtime, API, database, migration,
credential, crypto, operator-owned `dashboard-state.json`, or generated-state
change is part of the verdict-writer diff. Unrelated untracked `.agents/skills/`
content was not inspected or modified.

## Acceptance results

| Acceptance criterion | Result | Evidence |
|---|---|---|
| Unmistakable pre-release warning and current/planned separation | PASS | Guide opens with Phase 0 / not usable warning and defines current, planned, gated, and Experimental labels |
| Complete user journey coverage | PASS | Install/readiness, init/daemon, Provider login, profiles, sessions, attach/control, pairing, remote UI, grant/revoke, offline behavior, troubleshooting, and safety are present |
| Planned commands remain gated and examples use placeholders | PASS | Every `multidesk` command section says planned/currently unavailable; no fabricated download URL, output, port, service name, or Provider version appears |
| README and implementation-plan discovery | PASS | Both reference `docs/USER_GUIDE.md`; link scanner reports zero broken local links |
| Dashboard static and required-document contract | PASS | Static card is present; generated fact reports `exists: true`, `bytes: 13028`; missing and empty mutations are rejected |
| Workflow/dashboard repository contracts | PASS | Direct current-tree verifiers pass; isolated generator/verification chain also passes |
| Feature state visible without operator judgment changes | PASS | Generated facts contain `user-operations-guide` at `READY_FOR_VERIFY`; operator-owned manual state is unchanged |
| Security and architecture boundaries | PASS | Passkey vs Device Key, explicit target-scoped grants, ControllerLease, Device-owned secrets, and revocation vs remote erasure remain accurate |

## Commands and results

The shell has no `npm` on `PATH`, so the scripts named in `package.json` were
invoked with the bundled Node executable
`/Users/jinlong/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin/node`
(`v24.14.0`). This is command-equivalent because the npm scripts are direct
`node <script>` wrappers with no dependencies or extra arguments.

```text
git diff --check
PASS (no output)

<node> scripts/workflow/verify-workflow.mjs
PASS: agents=10, skills=3, docs=17, edges=20, statuses=15

<node> scripts/dashboard/verify-static.mjs
PASS: branch=codex/project-system/user-operations-guide, dirty=9,
phases=9, agents=10, skills=3

<node> -e '<repository-relative Markdown link scanner>'
PASS: files=8, broken=0

<node> -e '<required-doc positive/missing/empty in-memory fixture>'
PASS: positive=pass, missing=rejected, empty=rejected
```

The complete generation and verification chain was also run in an isolated
temporary clone overlaid with the current working tree, so it could not modify
the protected generated snapshot in this worktree:

```text
<node> scripts/workflow/generate-agent-mirrors.mjs
PASS: generated 10 Codex agents, 10 Claude agents, 3 Claude skill mirrors

<node> scripts/workflow/verify-workflow.mjs
PASS: agents=10, skills=3, docs=17, edges=20, statuses=15

<node> scripts/dashboard/generate-state.mjs
PASS: branch=codex/project-system/user-operations-guide, dirty=9,
agents=10, skills=3

<node> scripts/dashboard/verify-static.mjs
PASS: branch=codex/project-system/user-operations-guide, dirty=9,
phases=9, agents=10, skills=3
```

## Findings

None.

## Residual notes

- The guide is intentionally a pre-release contract, not proof that product
  commands are implemented. Owning phase writers must keep it synchronized as
  real commands and compatibility evidence become available.
- Ship remains a human gate. This verdict does not authorize merge, push,
  release, deployment, or risk acceptance.
