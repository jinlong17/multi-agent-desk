# Feature Brief: As-built product documentation authority

- Slug: `as-built-product-docs`
- Date: `2026-07-29`
- Owner module: `project-system`
- Impacted modules: `core`, `provider`, `control-plane`, `web`, `desktop`, `security`
- Requested by: `operator-directed documentation-drift planning`

## Motivation and outcome

The repository has a reviewed product target in `docs/IMPLEMENTATION_PLAN.md`,
feature-level state and verification evidence, a concise README, and several
as-built or placeholder documents. It does not currently contain either
`PRODUCT.md` or `docs/PRODUCT.md`. Without a named as-built product authority,
readers can confuse the v0.1 target with the source-built preview, a structural
workflow check with product acceptance, or a narrow provider result with broad
platform support.

The implementation outcome is a canonical `docs/PRODUCT.md` that states what
is true at a dated snapshot and links every substantive current-support claim
to its evidence. `README.md`, `docs/USER_GUIDE.md`, and the specialist product
documents will become derived, consistent views. This feature plans that work;
it does not replace any product documentation yet.

## Scope

- Establish `docs/PRODUCT.md` as the future canonical as-built product
  document. The unqualified `PRODUCT.md` label in
  `docs/IMPLEMENTATION_PLAN.md` section 18 is resolved as a document name in
  the `docs/` tree, not a new repository-root file.
- Define claim authority, vocabulary, evidence links, review ownership, and
  conflict resolution for product, support, provider, platform, and trust
  statements.
- Plan a claim inventory and reconciliation of `README.md`,
  `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, `docs/DATA_MODEL.md`,
  `docs/PROVIDER_ADAPTER.md`, `docs/PROVIDER_COMPATIBILITY.md`,
  `docs/THREAT_MODEL.md`, and `docs/ROADMAP.md`.
- Require provider-owner fact review for substantive Provider/version/platform
  claims and independent security review for substantive credential, key,
  encryption, revocation, or residual-risk claims.
- Define deterministic structural and manual acceptance checks for the future
  documentation build.

## Non-goals

- Do not change product code, runtime capability, release status, dashboard
  judgment, feature priority, or any existing support claim in this planning
  phase.
- Do not create a root-level `PRODUCT.md`, silently rewrite the reviewed
  implementation plan, or make `docs/PRODUCT.md` a substitute for feature
  `dev_log.md` state authority.
- Do not infer Provider compatibility from configuration, CI, a binary being
  present, an app-server handshake, or a previous platform result.
- Do not imply that revocation erases a credential or plaintext already copied
  to a compromised target, or that Passkey login grants E2EE decryption.
- Do not introduce product telemetry, credentials, browser automation, or
  external Provider research as part of documentation planning.

## User journeys

1. A prospective user opens the README and can distinguish the pre-release
   source-built preview from planned v0.1 capabilities, then follows the
   canonical product document for detail.
2. An operator deciding whether a feature is usable can identify its exact
   platform/version scope, evidence source, and any remaining gate instead of
   reading a phase label as a release claim.
3. A maintainer changes a capability and can update its evidence first, then
   make every product-facing document use the same bounded wording.
4. A security-conscious reader can tell which credential/trust guarantees are
   implemented, which are planned, and what residual risks cannot be removed
   remotely.

## Data and trust boundaries

Documentation is not a runtime authority, but inaccurate text can cause users
to make unsafe credential or deployment decisions. The documentation must not
contain secrets, copied authentication material, private raw Provider output,
or invented operational commands. It must preserve the project invariants:
Device-owned secrets, explicit target-scoped credential grants, local key
pinning, no automatic account rotation, and no claim of remote erasure after a
target has copied plaintext.

## Provider/external assumptions

Provider and platform support statements are valid only when their wording is
bounded by the exact version, platform, capability, evidence artifact, and
fallback recorded in `docs/PROVIDER_COMPATIBILITY.md` and the linked feature
evidence. A Provider owner must fact-check each substantive statement before
the documentation feature can be accepted. A missing, stale, contradictory,
or unreviewed row becomes `unknown`, `planned`, or explicitly unsupported; it
does not become supported through documentation reconciliation.

## Dependencies and gates

- `docs/IMPLEMENTATION_PLAN.md` section 18 supplies the product-document set
  and the reviewed target baseline; it is not current-support evidence.
- Feature `dev_log.md` files and their independent review/verification reports
  remain the lifecycle and implementation evidence source.
- `docs/PROVIDER_COMPATIBILITY.md` is the presentation index for exact
  compatibility evidence, subject to provider-owner review for every rewritten
  substantive claim.
- `docs/THREAT_MODEL.md`, relevant ADRs, and security-review reports supply
  security wording; the Security Gate is open and requires independent review
  before ship if the build changes substantive trust or credential claims.
- The Provider Gate is open. If the inventory reveals an unproven Provider
  assumption, the owning module must open a provider spike rather than approve
  the claim.
- `npm run project:verify` and the local Markdown-link checker must pass after
  documentation implementation. These are structural checks, not support or
  release evidence.

## Acceptance criteria

- [ ] `docs/PRODUCT.md` exists and identifies itself as the canonical as-built
  product document, with a dated status snapshot and evidence links.
- [ ] The document distinguishes product target, current source-built preview,
  exact supported scope, planned work, experimental work, unsupported work,
  and unknown evidence without treating any one label as another.
- [ ] Every substantive Provider/platform statement carries an exact evidence
  pointer and has provider-owner fact review; unsupported and pending cases
  retain their fallback or gate.
- [ ] Every substantive credential/trust statement is reviewed by security,
  preserves the residual-risk boundary, and does not overstate revocation,
  Passkeys, encryption, or platform protection.
- [ ] README, user guide, architecture, data model, provider adapter,
  compatibility, threat model, and roadmap are reconciled as derived views or
  expressly scoped specialist authorities; no contradictory current-support
  statement remains.
- [ ] Local links and `npm run project:verify` pass, with the check results
  recorded as documentation-structure evidence only.

## Risks and open questions

- A single high-level snapshot can become stale quickly. The build must define
  a dated snapshot policy and evidence-first update rule.
- Current documents may contain mutually inconsistent claims. The exact
  evidence wins; unresolved conflicts must be marked, not harmonized upward.
- `docs/PRODUCT.md` may need a compact machine-checkable claim ledger. Feature
  review must choose whether an in-document table is sufficient or a separate
  checked registry is justified; this plan does not pre-approve a new schema.
- Product text spanning Chinese and English must preserve semantic scope, not
  merely literal phrasing.

## Evidence

- `docs/IMPLEMENTATION_PLAN.md` sections 18, 19, and 25 name the intended
  documentation set, phase gates, and release documentation requirement.
- `README.md` and `docs/USER_GUIDE.md` already distinguish a source-built
  preview from planned/release work, but no canonical `PRODUCT.md` exists.
- `docs/ARCHITECTURE.md`, `docs/PROVIDER_ADAPTER.md`, and `docs/ROADMAP.md`
  currently identify themselves as placeholders; `docs/DATA_MODEL.md`,
  `docs/PROVIDER_COMPATIBILITY.md`, and `docs/THREAT_MODEL.md` contain
  specialist scoped evidence.
- `docs/workflow/project/workflow.md` makes `dev_log.md` the resumable state
  authority and prohibits converting partial or blocked evidence into pass.

## Handoff

Next role: `feature-plan`.
