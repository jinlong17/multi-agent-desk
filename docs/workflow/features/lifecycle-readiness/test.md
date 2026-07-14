# Test strategy: Lifecycle readiness hardening

## Acceptance matrix

| Requirement | Level | Command/scenario | Expected evidence |
|---|---|---|---|
| Mirrors match generator output byte-for-byte | static | `npm run workflow:verify` | pass; fails on any hand-edited mirror |
| Role handoff statuses ⊆ registry output ∧ state machine | static | `npm run workflow:verify` | pass; fails if e.g. bug-verify output drops READY_TO_SHIP |
| Templates carry all dashboard Status Panel fields | static | `npm run workflow:verify` | pass; fails if Title removed |
| Every feature dir has parseable dev_log | static | `npm run workflow:verify` + `npm run dashboard:verify` | pass; fails on MISSING_DEV_LOG or unknown fields |
| Layout authority is single | manual | grep for `apps/daemon`, `apps/cli`, `apps/server`, `packages/provider-` in CLAUDE.md and module registry | no hits |
| Dashboard reflects new feature/spike units | static | `npm run dashboard && npm run dashboard:verify` | feature_logs lists all units, all parseable |
| Bug workflow has legal entry | static | `npm run workflow:verify` | edge (BUGFIX, DRAFT, bug-diagnose, DIAGNOSED) required; bug_log.md template verified |
| Spike REVISE returns to spike pipeline | static | `npm run workflow:verify` | edge (SPIKE, REVISE) must contain SPIKE_READY and not NEEDS_REVIEW |
| Role-emitted statuses have authoring edges | static | `npm run workflow:verify` | every handoff status appears as Next on an edge naming that role |
| Registry output ≡ handoff statuses | static | `npm run workflow:verify` | bidirectional token equality per agent |
| Feature log states are legal | static | `npm run workflow:verify` | (Workflow, Status) of every log is a valid Current/terminal Next |
| Owner modules valid | static | `npm run workflow:verify` | Owner Module of every log is a module-registry key |
| Credential/auth spikes carry security gate | static | `npm run workflow:verify` | keyword heuristic + security-owner rule; `none` fails |
| Manual dashboard focus matches logs | static | `npm run dashboard:verify` | each manual.focus {slug, expected_status} matches feature_logs |
| Security-gated Feature can reach ACCEPTED | static | `npm run workflow:verify` | edges (FEATURE_DEV, READY_TO_SHIP, security-review) and (FEATURE_DEV, ACCEPTED, ship) required |
| Gated logs cannot bypass security-review | static | `npm run workflow:verify` | open gate at EVIDENCE_READY/READY_TO_SHIP forces Suggested Next = security-review; negative injection fails |
| Suggested Next names only legal writers | static | `npm run workflow:verify` | non-terminal log naming a non-writer agent fails |
| Full SOP keyword coverage | static | `npm run workflow:verify` | remote control / trust boundary spikes with gate none fail |
| Windows spikes are single-owner | manual | read spike-windows-conpty (provider) and spike-windows-named-pipe-ipc (core) panels | ConPTY and IPC scopes are in separate units matching registry signals |
| Static dashboard uses canonical semantics | static | `npm run dashboard:verify` | blacklist FIX_READY/FIX_READY_FOR_VERIFY/spike-intake/只读 Agent; DIAGNOSED and READY_FOR_VERIFY present |
| No stale successor references | static | grep for phase0-security-docs-adrs / spike-windows-conpty-sidecar / spike-windows-pty-ipc in actionable docs | only historical verdict records and Work Log rows match |

## Unit and property tests

None (no product runtime). Verifier assertions act as the unit layer.

## Contract and fixture tests

Negative checks executed once during verification: a temporarily edited
mirror, a registry output missing a role status, and a feature directory
without dev_log.md must each fail the verifier.

## Integration and E2E

`npm run project:verify` (workflow + dashboard generate + dashboard verify).

## Security/adversarial tests

Confirm no role gained authority over human gates; verdict-writer scope is
two files by contract.

## Cross-platform matrix

Scripts are Node ≥18, path handling via `node:path`; no platform-specific
behavior introduced.

## Failure injection and recovery

BLOCKED recovery rule exercised on paper via state machine review; runtime
exercise deferred until first real BLOCKED occurrence.

## Manual acceptance

Operator reads `workflow.md` §2–§3 and confirms every transition has exactly
one named writer.
