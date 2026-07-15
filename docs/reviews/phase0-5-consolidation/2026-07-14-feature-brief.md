# Feature Brief: Phase 0.5 compatibility consolidation

- Slug: `phase0-5-consolidation`
- Date: 2026-07-14
- Owner module: `project-system`
- Impacted modules: `core, provider, control-plane, web, desktop, security`
- Requested by: operator-directed sequential execution of the project audit recommendations

## Motivation and outcome

Phase 0.5 now has reproducible evidence and an accepted decision for every
planned provider, browser, Windows, and E2EE Spike, but the project-level state
still describes the phase as active. The outcome is one auditable transition
from Phase 0.5 to Phase 1 that reconciles the seven Spike logs, ADR 0010-0016,
the provider compatibility matrix, the implementation plan, the threat model,
and the development dashboard without overstating production readiness.

## Scope

1. Audit all seven Phase 0.5 Spike state authorities and their evidence-backed
   compatibility decisions.
2. Verify that ADR 0010-0016 and the provider compatibility matrix preserve the
   accepted fallbacks, version boundaries, and deferred acceptance gates.
3. Record Phase 0.5 completion and make Phase 1 Device Kernel the active project
   phase in the implementation plan and dashboard state.
4. Preserve residual Windows 11, real-provider, credential, quota, and release
   gates as explicit later-phase work.
5. Refresh and verify generated dashboard facts and run the full local project
   governance checks.
6. Verify the protected `main` branch and the merged Phase 0.5 pull requests
   against their required GitHub checks.

## Non-goals

- Implementing Phase 1 or any product code.
- Claiming that accepted Spike designs are production implementations.
- Claiming Codex multi-writer refresh, a completed headless device-auth flow,
  or a successful 48-hour observation period.
- Claiming Claude setup-token CredentialGrant, distinct-account isolation, or
  long-session behavior.
- Removing Windows 11 real-device, multi-session/service, signed-package,
  provider-TUI, IME, accessibility, sleep, reboot, or security-software gates.
- Accepting release risk or changing the established trust boundaries.

## User journeys

- A contributor opening the dashboard can see that Phase 0.5 decisions are
  complete and that Phase 1 is the next executable phase.
- A Phase 1-6 implementer can locate the exact ADR, compatibility result,
  fallback, and retained acceptance gate for each external dependency.
- A reviewer can reproduce the completion verdict from repository state and
  GitHub checks without relying on chat history.

## Data and trust boundaries

This feature changes documentation, workflow state, and generated dashboard
facts only. It must not capture or persist provider credentials, account
identifiers, tokens, private keys, cookies, or unsanitized command output.
Manual judgment remains in `dashboard-state.json`; generated Git and workflow
facts remain in `state.generated.js`.

## Provider/external assumptions

- GitHub remains the synchronization and protected integration authority.
- Exact tested provider/browser/tool versions in
  `PROVIDER_COMPATIBILITY.md` are evidence bounds, not evergreen support.
- The operator has shortened the Codex observation period; ADR 0014's
  single-writer CAS and interactive-login fallback remain mandatory.
- The operator requires only one Claude account; ADR 0016 therefore does not
  claim distinct-account isolation.

## Dependencies and gates

- Depends on all seven Phase 0.5 Spike logs being `GATE_RESOLVED` and ADR
  0010-0016 being accepted on protected `main`.
- Depends on successful macOS, Linux, Windows, governance, DCO, license, and
  documentation checks for their merge pull requests.
- No new security review is required because this feature introduces no new
  trust boundary; it audits and preserves already reviewed boundaries.
- Phase completion, branch creation, push, merge, and ship are explicitly
  operator-authorized for this sequential execution.

## Acceptance criteria

- [ ] All seven Phase 0.5 Spike state authorities are `GATE_RESOLVED` and point
  to reproducible evidence plus a fallback or bounded support decision.
- [ ] ADR 0010-0016 are indexed as accepted and no Spike decision is described
  as a completed production implementation.
- [ ] `PROVIDER_COMPATIBILITY.md` contains no unresolved Phase 0.5 decision and
  keeps every residual acceptance gate explicit.
- [ ] The implementation plan and manual dashboard state mark Phase 0.5
  completed and Phase 1 active, with a concrete Phase 1 next action.
- [ ] Dashboard focus binds to the consolidation feature's verified status and
  does not retain stale active-branch or pending-merge language.
- [ ] Codex 48-hour/multi-writer, Claude setup-token/distinct-account/long-run,
  and Windows 11 release acceptance remain excluded from support claims.
- [ ] `npm run dashboard`, `npm run project:verify`, link checking, license
  checking, and the three-platform scaffold/build verification pass.
- [ ] The Phase 0.5 consolidation pull request passes every protected-main
  required check and is merged to `main`.

## Risks and open questions

- Project-level status can drift if manual dashboard judgment is advanced
  without a matching feature state transition; the focus binding must fail
  closed on stale status.
- Provider and browser releases can invalidate exact-version evidence; later
  implementation phases must probe versions and fail closed rather than infer
  support.
- Windows GitHub-runner evidence cannot replace Windows 11 real-device release
  acceptance; those gates remain scheduled for the owning later phases.
- Claude quota and interactive challenge availability can prevent real-session
  acceptance even though the login boundary is resolved.

## Evidence

- `docs/workflow/features/spike-*/dev_log.md`
- `docs/spikes/`
- `docs/adr/0010-*` through `docs/adr/0016-*`
- `PROVIDER_COMPATIBILITY.md`
- `docs/IMPLEMENTATION_PLAN.md` Phase 0.5 and Phase 1
- `docs/security/THREAT_MODEL.md`
- Protected-main pull-request checks and merge commits for the seven Spikes

## Handoff

Next role: `feature-plan`.
