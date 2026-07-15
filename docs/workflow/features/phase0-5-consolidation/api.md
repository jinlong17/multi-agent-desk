# Contracts: Phase 0.5 compatibility consolidation

## Public interfaces

This unit adds no runtime API. Its public contracts are repository state:

- seven Phase 0.5 Spike `dev_log.md` files;
- ADR 0010-0016 and `PROVIDER_COMPATIBILITY.md`;
- `docs/IMPLEMENTATION_PLAN.md` phase status language;
- `docs/workflow/project/dashboard-state.json` manual judgment;
- `docs/prototypes/dev-dashboard/state.generated.js` generated facts;
- this feature's plan, verdict, and ship documents.

## Requests, events, and responses

The consolidation input is a fixed set of seven Spike slugs. A successful
reconciliation produces:

1. a resolved decision record for every slug;
2. an explicit list of deferred acceptance gates;
3. Phase 0.5 `completed` and Phase 1 `active` manual state;
4. generated dashboard facts consistent with Git and workflow authorities;
5. a lifecycle verdict whose status exactly matches the dashboard focus.

## Error semantics

- `unresolved_spike`: expected slug missing or status is not `GATE_RESOLVED`.
- `unbounded_claim`: support claim lacks exact evidence/version or fallback.
- `missing_residual_gate`: deferred Windows/provider/security work disappears.
- `stale_dashboard_focus`: focus status differs from feature authority.
- `remote_governance_mismatch`: required check or protection readback differs.

Every error is fail-closed: Phase 0.5 stays active and ship is forbidden.

## Authentication and authorization

Local verification is read-only. Authenticated GitHub readback may inspect the
public repository, pull requests, checks, Actions settings, and branch
protection. Push, merge, phase completion, and ship are authorized by the
operator's sequential-execution directive. No provider authentication material
is read or retained by this feature.

## Idempotency, ordering, and replay

Reconciliation and dashboard generation are deterministic and repeatable.
Manual state changes occur only after the related lifecycle verdict is
persisted. Work Log and evidence ledgers are append-only. Re-running generators
must produce no diff when Git and workflow state are unchanged.

## Versioning and compatibility

The dashboard uses schema version 1 and Plan version v0.2. Exact tested tool,
browser, OS, and provider versions remain in the compatibility matrix. New
versions require later evidence; they are not inferred compatible here.

## Data retention and deletion

Sanitized reports, JSON evidence, workflow logs, and remote run identifiers are
retained with the repository. Credentials and raw secret-bearing output are
never collected. Historical negative, unsupported, and superseded evidence is
retained rather than deleted.
