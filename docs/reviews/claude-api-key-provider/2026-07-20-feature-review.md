# Feature Review: Claude API-key provider vertical slice

- Date: 2026-07-20 PDT
- Role: `feature-review`
- Target: `claude-api-key-provider`
- Owner module: `provider`
- Verdict: **BLOCKED**

## Review scope

I independently reviewed the Feature Brief, `design.md`, `api.md`, `test.md`,
`dev_log.md`, child API-key compatibility Spike evidence, ADR 0016, the
Provider Compatibility Matrix, the implementation plan, and live Draft PR #25
state. The controlling operator scope is now: no API key or cloud
authentication, ignore Linux, and use only the existing macOS Claude.ai Team
subscription.

This review ran no Provider request and did not modify the design, API, test,
Feature Brief, child Spike, compatibility matrix, implementation, dashboard,
branch, commit, or PR. Historical API-key/no-key evidence remains intact.

## Module classification

Owner: `provider`

Confidence: high

Why: the reviewed feature is an official Claude Code adapter plan whose
defining contracts are Provider authentication, billing source, CLI/PTY
compatibility, health, and usage projection.

Impacts: `security` for credential and billing boundaries; `project-system` for
workflow truth. No second owner is inferred.

Branch: `codex/provider/claude-api-key-provider`

Workflow: `feature`

Gates: Provider and Security Gates remain unresolved; the selected operator
scope removes the planned API-key/cloud and Linux prerequisites rather than
clearing them.

## Positive findings

- The plan is internally explicit about the API-key trust boundary, Console
  billing confirmation, secret-safe enrollment, exact-version compatibility,
  redaction, rollback, and the absence of subscription fallback.
- The no-key Spike evidence is preserved as a truthful negative mechanism
  result and does not claim valid-key, paid, PTY, Linux, or stable support.
- Draft PR #25 remains a non-implementation review surface and its seven checks
  previously passed; structural green checks do not override the changed
  product scope.

## Findings

### P0 — The selected product scope directly contradicts the plan boundary

Evidence:

- `design.md` selects a user-supplied Claude Console API key and explicitly
  excludes managed Claude.ai subscription OAuth.
- `api.md` defines `auth_method=api_key`,
  `billing_source=claude_console_api`, secret submission, Vault storage, and
  errors that forbid subscription fallback.
- `test.md` requires a dedicated key and bounded ordinary billed requests; it
  treats the current Team subscription only as a negative crossover canary.
- The operator has now explicitly selected the inverse scope: no API key or
  cloud auth, macOS Team subscription only.

Impact: no G0 or P1-P7 phase can run without discarding the reviewed feature's
identity, credential, billing, security, migration, API, and acceptance
contracts. A builder would have to invent a different feature.

Clearing condition: a future explicit operator decision to resume API-key or
supported-cloud product work. After that decision, `feature-plan` must re-read
current Provider policy and repository state, reconcile or replace this plan,
and return it to `NEEDS_REVIEW`. The prior request for a key is not itself the
current clearing condition.

### P0 — Ignoring Linux removes a mandatory feature exit

Evidence:

- The Phase Plan makes P5 exact Linux amd64 live acceptance a predecessor of
  macOS P6.
- The Feature Brief, design compatibility table, test matrix, manual
  acceptance, and implementation plan all require real Linux PTY evidence for
  this candidate vertical slice.
- The operator now explicitly excludes Linux.

Impact: even if an API key existed, this exact plan could not reach its defined
exit under the selected platform scope. Removing Linux would require a new
feature-plan decision; review cannot rewrite the plan.

### P1 — Subscription-only testing cannot be credited to this API-key feature

ADR 0016 and the compatibility matrix retain exact-version macOS
`CLAUDE_CONFIG_DIR` and auth-health mechanism evidence while marking stable
managed subscription integration unsupported without a fresh Provider/Security
lifecycle and explicit policy approval. The current API-key plan goes further:
it expressly rejects subscription use as acceptance evidence.

Impact: a bounded macOS subscription CLI/PTY experiment may be useful, but it
cannot clear this feature's Provider Gate, validate its API-key contracts, or
authorize `feature-build`. It requires a separate operator-selected unit and
lifecycle if the product is to claim more than direct official Claude Code
usage outside MultiAgentDesk's managed surface.

## Security and compatibility assessment

The changed scope removes the API key and Console billing risk from the
requested experiment, but it does not transform the reviewed API-key feature
into a subscription feature. Reusing this plan would erase the distinction
between subscription OAuth and API-key/cloud authentication that its own
security contracts enforce. The safe verdict is therefore terminal-until-
cleared `BLOCKED`, with no Provider call and no implementation authorization.

The child no-key Spike remains historical evidence for the superseded
API-key hypothesis. It is not rewritten, promoted, or treated as subscription
acceptance.

## Verdict

**BLOCKED.** The API-key/Console-billing plan is internally coherent but no
longer executable under the operator-selected macOS Team subscription-only
scope. Neither a builder nor reviewer may silently replace its auth, billing,
platform, and policy contracts. Only a future explicit operator decision to
resume API-key/cloud work allows `feature-plan` to clear this feature's block.

## Evidence

- `docs/reviews/claude-api-key-provider/2026-07-16-feature-brief.md`
- `docs/workflow/features/claude-api-key-provider/{design,api,test,dev_log}.md`
- `docs/workflow/features/spike-claude-api-key-cli-compatibility/dev_log.md`
- `docs/spikes/claude-api-key/2026-07-20-compatibility-spike.md`
- `docs/spikes/claude-api-key/2026-07-20-no-key-probe.json`
- `docs/adr/0016-claude-profile-interactive-login-boundary.md`
- `docs/PROVIDER_COMPATIBILITY.md`
- `docs/IMPLEMENTATION_PLAN.md` Phase 3 and platform acceptance contracts
- Draft PR #25, head `7f4c09da8e56512a487a5bf07c18ef570548163e`,
  observed open, Draft, behind `main`, with its prior seven checks successful
- Operator-selected scope supplied to this review: subscription-only macOS;
  no API key/cloud auth; Linux excluded

## Handoff

**Target**: `claude-api-key-provider`
**Completed**: `feature-review`
**Verdict**: `BLOCKED`
**Summary**: `The API-key/Console-billing plan is no longer executable because the operator selected macOS Claude.ai Team subscription-only testing, excluded API key/cloud authentication, and removed the mandatory Linux exit.`
**Findings**: `P0: every G0/P1-P7 contract assumes API-key/Console billing that the selected scope excludes; P0: Linux is a mandatory predecessor and exit but is now excluded; P1: subscription-only evidence cannot validate or be credited to this API-key feature.`
**Evidence**: `All feature artifacts, child no-key Spike evidence, ADR 0016, Provider Compatibility Matrix, implementation plan, and live Draft PR #25 state; no Provider call was run.`
**Blockers**: `A future explicit operator decision to resume API-key/cloud work; feature-plan is the clearing role and must re-plan against current evidence before returning to review.`

### Next Step

Run `feature-plan` for `claude-api-key-provider` only after the operator explicitly resumes API-key/cloud work.
