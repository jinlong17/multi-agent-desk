# Contract: Phase 0 repository identity and root governance

This feature exposes governance and repository-state contracts, not a runtime
API.

## Root document contract

- `LICENSE`: Apache License 2.0 full text.
- `CONTRIBUTING.md`: DCO sign-off required; no CLA; local verification entry.
- `SECURITY.md`: private reporting guidance without invented SLA or contact.
- `THIRD_PARTY_NOTICES.md`: notice ledger and update rule; no dependency claims
  before dependencies exist.
- `README.md`: product identity, current maturity, development/document links,
  and non-goals consistent with the implementation plan.

## Verifier contract

- Workflow verification must explicitly require both gated FEATURE_DEV edges:
  `READY_TO_SHIP + security-review -> ACCEPTED|REVISE|BLOCKED` and
  `ACCEPTED + ship -> SHIPPED|BLOCKED`.
- A stale dashboard focus error must identify the operator-directed writer
  refresh rule and must not imply any arbitrary next writer owns manual state.

## Repository identity contract

- Final checkout basename: `multi-agent-desk`.
- Final origin repository basename: `multi-agent-desk.git` (or equivalent SSH
  URL for that repository), verified from Git rather than assumed.
- Directory rename, GitHub rename/settings, push, and merge remain separate
  operator gates.
