# Feature review: As-built product documentation authority

- Target: `as-built-product-docs`
- Plan commit reviewed: `a14d39558b651cee41beeccc545969af511ad85a`
- Date: `2026-07-29 PDT`
- Owner classification: `project-system` (high confidence). The feature owns
  documentation authority, workflow evidence routing, and acceptance
  contracts. Provider and Security remain affected evidence authorities; they
  are not co-owners of this documentation build.
- Verdict: `APPROVED`

## Conclusion

Plan v0.2 resolves all three findings from the preceding review without
expanding product support or trust claims. The next build phase can implement
the documentation artifact set without inventing ledger, freshness, Provider,
or Security decisions.

The plan is intentionally evidence-conservative: current Provider support may
be published positively only with an upstream immutable receipt, while missing
or expired evidence remains `unknown`, `planned`, or `unsupported`; substantive
trust wording remains subject to independent Security review after final
feature verification. Those open gates constrain the future prose but do not
block this documentation plan from entering `feature-build`.

## Re-review of prior findings

### 1. Ledger authority, retention, and coverage — resolved

`api.md` fixes the durable ledger path as
`docs/reviews/as-built-product-docs/claim-ledger.md`, its required header and
row fields, stable `CLM-###` IDs, full-revision evidence references, source and
destination anchors, and explicit conflict IDs. `design.md` defines the
coverage universe as substantive product-facing support/trust statements in
every named source surface, every future product capability-matrix row, and
any newly changed out-of-universe product-facing support/trust statement.

The append-only/supersession rule, retained unresolved-conflict list, and
no-deletion-on-prose-rollback rule make the ledger a durable review artifact,
not a mutable runtime or dashboard authority. `test.md` supplies direct
inspection acceptance for path, coverage, retention, source-to-destination
mapping, and conflict handling. This is decision-complete and manually
testable without requiring a new product registry or an invented automation
schema.

### 2. Provider and Security producer ordering — resolved

The plan now identifies only valid upstream Provider receipt producers:

- an independent `feature-verify` report for the provider-owned feature that
  proved the exact scope; or
- a provider-spike evidence artifact together with its `GATE_RESOLVED` feature
  log and compatibility-matrix row.

P1 records existing receipts and gaps. P2 may publish a positive Provider
claim only after a matching current receipt is pinned in the ledger; otherwise
the safe classes remain available and a separate provider-owned spike supplies
any needed new evidence. Documentation planning, build, and verification do
not establish or self-accept Provider behavior.

P4 is correctly limited to verification preparation. The independent
`security-review` occurs only after `feature-verify` advances the completed
feature to `READY_TO_SHIP`, consistent with the workflow state machine. This
is the sole feature-level writer that can accept or return substantive trust
wording before ship. The plan therefore does not create an undefined Provider
review role or place a Security verdict inside a build phase.

### 3. Freshness vocabulary and downshift — resolved

The canonical class set is consistently `planned`, `preview`, `supported`,
`experimental`, `unsupported`, and `unknown`; `preview` must include the
literal `source-built` qualifier, and `stale` is an evidence state rather than
a capability class. Snapshot and evidence references are pinned to dates and a
full verification baseline revision.

The plan makes freshness deterministic: a snapshot over 30 calendar days old,
or a positive Provider receipt over 90 days old, out of scope, unreachable,
or contradictory, downshifts the affected row and product-matrix statement to
`unknown` with the prescribed stale/contradictory evidence state and refresh
gate. `test.md` gives concrete day-31 and day-91 manual failure scenarios plus
scope-mismatch and unreachable-revision cases. This prevents an aged snapshot
or structural check from preserving an unsupported positive claim.

## Execution constraints preserved for feature build

1. P2 must not publish a positive substantive Provider/version/platform claim
   without the ledger's matching current receipt tuple; a provider plan, CI,
   branch, or configuration is not a substitute.
2. Any unresolved Provider claim remains visibly bounded and may require a
   separately tracked provider spike; the documentation feature must not mark
   that upstream evidence as accepted.
3. The build must preserve the threat-model boundaries that a Passkey is not
   E2EE decryption authority and revocation cannot erase plaintext already
   copied to a compromised target. Final acceptance of changed substantive
   trust wording remains the later independent Security Gate.
4. `npm run project:verify` and `npm run ci:links` are structural evidence
   only; they do not establish product, platform, deployment, or release
   readiness.

## Evidence and checks

- Read the revised feature brief, `design.md`, `api.md`, `test.md`, and
  `dev_log.md`; compared `a14d395` with the earlier `REVISE` review and checked
  the exact five changed planning artifacts.
- Re-read `docs/IMPLEMENTATION_PLAN.md`, `AGENTS.md`, `CLAUDE.md`, the module
  registry, workflow state machine, and feature-review role contract.
- Checked current `docs/PROVIDER_COMPATIBILITY.md` and `docs/THREAT_MODEL.md`:
  both retain narrow version/platform boundaries and residual-risk wording,
  which the plan correctly requires future documentation to preserve rather
  than broaden.
- With Node `v24.11.1` on `PATH`, passed
  `/opt/homebrew/bin/pnpm run project:verify` (workflow and dashboard checks)
  and `/opt/homebrew/bin/pnpm run ci:links` (308 Markdown files).
- `git diff --check a14d395^ a14d395` passed. The reviewed worktree was clean
  before and after the checks; no product documentation, implementation,
  dashboard judgment, remote, push, merge, or release action was performed.

## Decision

`APPROVED`. The approved plan defines an executable P1 evidence inventory,
P2 canonical document, P3 reconciliation, and P4 verification-preparation
sequence. Proceed with one `feature-build` phase only. Provider receipts and
the Security Gate remain independently governed at their specified workflow
boundaries.

## Handoff

**Target**: `as-built-product-docs`
**Completed**: `feature-review`
**Verdict**: `APPROVED`
**Summary**: `Plan v0.2 is decision-complete and testable: it fixes the durable claim ledger and coverage contract, requires valid upstream Provider evidence before positive claims, and routes Security acceptance only through the post-verification workflow gate.`
**Findings**: `No blocking findings. Feature-build must retain the receipt, freshness-downshift, residual-risk, and structural-check-only constraints recorded above.`
**Evidence**: `a14d395 revised artifacts; implementation-plan, module, workflow, and role contracts; current Provider Compatibility and Threat Model boundaries; project:verify and ci:links passed with Node v24.11.1; git diff --check passed.`
**Blockers**: `none for feature-build; missing or stale future Provider receipts must remain unknown or open a separate provider-owned spike, and substantive trust wording remains subject to the later independent Security Gate.`

### Next Step

Run `feature-build` for `as-built-product-docs`.
