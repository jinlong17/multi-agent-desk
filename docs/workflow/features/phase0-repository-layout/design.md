# Design: Phase 0 repository identity and root governance

## Decision snapshot

- Owner: `project-system`; no Provider or Security Gate.
- Execute in two phases so local governance can be verified before the
  operator-gated repository identity maintenance window.
- P1 owns root governance documents plus the explicitly assigned lifecycle
  follow-ups F1 and F2. P2 owns only local directory/origin identity changes.
- The §17 product skeleton remains out of scope for
  `phase0-monorepo-scaffold`.

## P1: governance documents and verifier follow-ups

Create or refresh `LICENSE`, `CONTRIBUTING.md`, `SECURITY.md`,
`THIRD_PARTY_NOTICES.md`, and `README.md`. Links must resolve locally; DCO is
required and no CLA is introduced. Hard-assert the two gated FEATURE_DEV edges
identified as F1 in the lifecycle-readiness verification, and replace the F2
dashboard hint with the single-authority operator-directed wording.

Failure or rollback is file-local: revert the P1 commit if verification finds
incorrect governance text or verifier behavior. No product code or remote
state changes occur.

## P2: repository identity maintenance window

During an operator-approved maintenance window, rename the local checkout from
`agent-deck` to `multi-agent-desk`. Change `origin` only after the operator has
renamed or otherwise prepared the GitHub repository. Re-open the checkout,
verify the absolute worktree path, origin URL, clean status, and all project
checks. Do not push or change GitHub settings without separate authorization.

If the remote repository is not ready, leave origin unchanged and record the
check as blocked/unknown; never substitute a speculative URL.

## Boundaries

No secrets, Provider behavior, runtime code, monorepo directories, branch
protection, required-check settings, push, merge, or risk acceptance are in
scope. ADR 0009 and the module registry remain the layout authorities.
