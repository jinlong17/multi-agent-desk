# Test strategy: Phase 0.5 compatibility consolidation

## Acceptance matrix

| Requirement | Level | Command/scenario | Expected evidence |
|---|---|---|---|
| Seven Spike decisions resolved | workflow contract | enumerate expected slugs and parse Status Panels | exactly seven `GATE_RESOLVED` results |
| Accepted decision set complete | documentation contract | inspect ADR index and compatibility matrix | ADR 0010-0016 accepted; every gate has result and fallback/boundary |
| Unsupported claims preserved | adversarial documentation review | search Codex, Claude, and Windows conclusions | no 48h/multi-writer/headless completion, setup-token/distinct-account/long-run, or Windows 11 release claim |
| Project phase transition coherent | integration | inspect plan plus manual dashboard state | Phase 0.5 completed; Phase 1 active; concrete next action |
| Dashboard authorities aligned | generated-state contract | `npm run dashboard` then `npm run dashboard:verify` | generator succeeds; no stale focus or generated diff |
| Repository governance intact | regression | `npm run project:verify` | workflow, dashboard, registries, docs, and lifecycle checks pass |
| Local documentation valid | regression | `npm run ci:links` | all local links and anchors pass |
| Dependency policy intact | regression | `npm run ci:licenses` | current pnpm/Cargo/Go policy passes |
| Cross-platform scaffold intact | platform regression | `npm run scaffold:verify` locally and CI on macOS/Linux/Windows | all build jobs pass |
| Protected integration exact | remote integration | GitHub PR/check/protection readback | seven required checks green, strict protection retained, exact verified head merged |

## Unit and property tests

No new runtime unit test is required. Deterministic validators must reject a
temporary stale dashboard focus and must return to a clean state after the
fixture is restored. Repeated dashboard generation with unchanged inputs must
be idempotent.

## Contract and fixture tests

Check the expected slug set explicitly so an extra resolved feature cannot hide
a missing Phase 0.5 Spike. Check both positive claims and negative boundaries.
Existing workflow, registry, CODEOWNERS, DCO, link, and license fixtures remain
part of `project:verify`/CI.

## Integration and E2E

Run the complete local project checks on the consolidation head. Push the
signed branch, open a ready pull request, and require the protected macOS,
Ubuntu, Windows, project, DCO, license, and link checks. Merge only that head,
then require the corresponding main checks to pass.

## Security/adversarial tests

- Search committed changes for tokens, credentials, cookies, private keys,
  Keychain payloads, and unsanitized home paths.
- Confirm ADR 0011, 0013-0016 security constraints remain unchanged.
- Confirm unsupported capabilities remain explicit rather than omitted.
- Confirm Windows runner evidence is not described as real-device acceptance.
- Confirm the dashboard generator does not make operator judgments.

## Cross-platform matrix

| Platform | Required evidence | Retained limitation |
|---|---|---|
| macOS | project and scaffold build; browser/Codex/Claude exact-version artifacts | later real-provider and signed release acceptance |
| Linux | project and scaffold build; Codex/Claude auth-health evidence | later daemon/service and real-provider acceptance |
| Windows | project and scaffold build; ConPTY, Named Pipe, and Sidecar runner artifacts | Windows 11 real-device, multi-session/service, IME/accessibility, lifecycle, signed installer acceptance |

## Failure injection and recovery

Temporarily alter a copied or restored dashboard focus expectation and prove
verification rejects it. A failed remote check leaves the branch unmerged; the
correction receives a new signed commit and a complete check rerun. No negative
fixture may leave the worktree dirty.

## Manual acceptance

Read the final Phase 0.5 and Phase 1 plan/dashboard wording as an operator:
Phase 0.5 must be unambiguously complete at the decision level, Phase 1 must be
the active implementation target, and every unproven production/release claim
must remain visible in the owning later phase.
