# Lifecycle readiness review — REVISE

- Date: 2026-07-10
- Role: `feature-review` (read-only assessment of the project system)
- Target: `lifecycle-readiness`
- Verdict: **REVISE**

## Conclusion

The architecture, workflow, agent, skill, and security-governance baseline is
strong enough to start Phase 0, but the system is not yet a closed loop for
the full lifecycle and must not be run end-to-end to v0.1 in one pass.
Phase 0.5 spikes will overturn assumptions by design, and the workflow itself
mandates per-phase build/verify with human gates.

## P0 findings (block product skeleton work)

1. **Repository layout authority conflict.** `docs/IMPLEMENTATION_PLAN.md` §17
   defines `cmd/ + internal/ + apps/{web,desktop}`; `CLAUDE.md` and
   `docs/workflow/project/module-registry.json` treated `apps/daemon`,
   `apps/cli`, `apps/server`, `packages/provider-*` as authoritative. Affects
   module ownership, branch naming, CODEOWNERS, CI path filters, and agent
   write scopes. Resolve by ADR.
2. **Read-only review roles cannot persist state transitions.** The workflow
   requires every transition to be written to `dev_log.md`, but
   `feature-review`, `feature-verify`, `bug-verify`, and `security-review`
   were fully read-only, so `APPROVED`/`REVISE`/`VERIFIED`/`READY_TO_SHIP`
   had no defined on-disk writer.
3. **Spike state machine is not closed.** Intake and the final decision had no
   owner; there was no spike log template; `security-review` returns
   `ACCEPTED` but the state machine had no such transition; `BLOCKED` had no
   recovery rule.
4. **Macro phases are not broken into executable work units.**
   `docs/workflow/features/` had no real feature/spike state directories;
   Phase 0 cannot be handed to `feature-build` as-is.

## P1 findings (block Phase 0 exit)

5. **Verification proves structure, not semantics.** No mirror content
   comparison, no role/registry/state-machine consistency checks. Known
   drift: registry said `bug-verify` outputs `VERIFIED` while the role
   contract requires `READY_TO_SHIP`; the dashboard verifier did not fail on
   `MISSING_DEV_LOG`; the dev_log template lacked the `Title` field the
   generator reads.
6. **Phase 0 engineering baseline not implemented** (monorepo skeleton, CI,
   ADR 0001–0008, threat model, compatibility matrix, research log,
   CONTRIBUTING/SECURITY/Notices).
7. **Remote governance unverified** (branch protection, required checks,
   Actions, release permissions).

## P1/P2 (before release)

Release-candidate/rollback process, backup/restore drills, upgrade and
migration-failure policy, performance budgets and SLOs, incident response,
artifact signing/SBOM/provenance, deployment verification, and maintenance
workflows must be split into executable gates before v0.1 ships.

## Handoff

**Target**: `lifecycle-readiness`
**Completed**: `feature-review`
**Verdict**: `REVISE`
**Summary**: Governance baseline supports starting Phase 0, but state
persistence, layout authority, spike closure, semantic verification, and
release operations do not yet meet full-lifecycle execution standards.
**Findings**: P0 items 1–4; P1 items 5–7 above.
**Evidence**: `npm run workflow:verify` and `npm run dashboard:verify` passed;
working tree clean; no product code, monorepo, or CI yet.
**Blockers**: Do not approve continuous product implementation until the P0
items are fixed.

### Next Step

Run `feature-plan` for `lifecycle-readiness`.
