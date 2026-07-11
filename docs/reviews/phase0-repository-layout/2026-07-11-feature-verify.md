# Feature verification: phase0-repository-layout P1

## Verdict

`VERIFIED`

P1 satisfies its approved acceptance criteria. P2 remains, so this unit is not
ready to ship.

## Evidence

- `npm run project:verify && git diff --check` — pass; workflow verified with
  10 agents, 3 skills, 17 required docs, 20 edges, and 15 statuses; dashboard
  verified on the feature branch with 12 dirty paths before verdict writes.
- Independent local Markdown link scan over README, CONTRIBUTING, SECURITY,
  and THIRD_PARTY_NOTICES — pass, four files and no missing local targets.
- Root-policy marker inspection — pass: Apache-2.0 license exists; DCO sign-off
  and no-CLA language; private security-reporting guidance with no invented
  SLA; pre-dependency notice ledger; truthful pre-release README.
- F1 source and build receipt — pass: both gated edges have exact-set asserts;
  the build ledger records two independent negative mutations, each failing
  with its dedicated missing-edge message before restoration.
- F2 source inspection — pass: stale-focus guidance names an
  operator-directed writer session and target Work Log.
- Scope check — pass: checkout remains under `agent-deck`; no directory move,
  GitHub setting, push, or merge occurred. `origin` already resolves to
  `git@github.com:jinlong17/multi-agent-desk.git`, reducing but not removing P2.

## Findings

No blocking P1 findings. The durable CI link checker remains `unknown` and is
correctly deferred to `phase0-ci-governance`; this verdict covers the local
link scan only.

## Compatibility and security

P1 changes project-system documentation and verifier behavior only. No product
runtime, migration, Provider behavior, credential boundary, or Windows
hardware acceptance is involved.
