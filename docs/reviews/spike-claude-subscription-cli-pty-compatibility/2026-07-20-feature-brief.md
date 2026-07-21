# Feature Brief: Claude subscription CLI/PTY compatibility evidence on macOS

- Slug: `spike-claude-subscription-cli-pty-compatibility`
- Date: `2026-07-20`
- Owner module: `provider`
- Impacted modules: `security, core, project-system`
- Requested by: `operator scope correction after the Claude API-key Spike`

## Module classification

```text
Owner: provider
Confidence: high
Why: the Spike evaluates exact-version Claude Code CLI, PTY, authentication-mode, and Provider compatibility behavior.
Impacts: security, core, project-system
Branch: codex/provider/claude-subscription-cli-pty-compatibility
Workflow: spike
Gates: Provider compatibility evidence; Security Gate open because authenticated subscription state and trust boundaries are exercised
Docs: docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-feature-brief.md; docs/workflow/features/spike-claude-subscription-cli-pty-compatibility/dev_log.md
```

## Motivation and outcome

The operator corrected the previous Claude API-key scope: the intended test uses
the already authenticated `claude.ai` Team subscription in the user-installed
official Claude Code CLI. It does not use a Claude Console API-key,
bearer-token, or supported-cloud credential override; the official CLI
necessarily uses its existing subscription OAuth login. Linux is explicitly
out of scope.

The measurable outcome is a narrowly sanitized compatibility result for exact
Claude Code `2.1.207` on the operator's macOS arm64 host. The experiment may run
at most one minimal non-interactive print probe and one minimal interactive PTY
probe. Both must use included subscription access only, have tool and workspace
file effects disabled, retain no runtime capture of the raw prompt, response,
transcript, or PII, and stop rather than opt into usage credits when included
usage is unavailable.

A successful result establishes only that the direct official Claude Code
mechanism worked in this exact external/experimental environment. It does not
authorize or establish a stable MultiAgentDesk-managed subscription product.
ADR 0016's 2026-07-16 policy narrowing remains authoritative.

## Scope

- Pin the experiment to macOS `26.5.2` arm64 and exact local Claude Code
  `2.1.207`; record the resolved binary digest before any request.
- Read only a sanitized authentication projection sufficient to confirm
  `loggedIn=true`, `authMethod=claude.ai`, `subscriptionType=team`, and the
  Provider class. Do not retain email, organization, raw JSON, browser state,
  or credential material.
- Confirm that API-key, OAuth-token override, Bedrock, Vertex, and Foundry
  credential selectors are absent. Abort rather than repair or change auth.
- Inspect the exact-version CLI surface and construct a no-tool,
  no-session-persistence, externally time-bounded print probe. Run it at most
  once and retain only exit class, timeout class, and marker-match boolean.
- Construct a no-tool, externally time-bounded interactive PTY probe in an
  empty disposable workspace. Run it at most once, request one fixed marker,
  exit cleanly, and retain only lifecycle/marker classifications.
- Capture before/after manifests for the disposable workspace and the selected
  config scope. Any unexpected write, tool invocation, session transcript, or
  raw terminal capture fails the corresponding criterion; evidence artifacts
  themselves are the only planned repository writes.
- Stop if Claude reports an included-usage or session limit, requests an
  upgrade/extra-usage action, exposes ambiguous billing/auth selection, or
  cannot disable tools and persistence. Do not enable or opt into usage credits.
- Persist reproducible, redacted evidence under
  `docs/spikes/claude-subscription/` and route it through Security Review before
  a feature-plan decision.

## Non-goals

- No Claude Console API key, `ANTHROPIC_API_KEY`,
  `CLAUDE_CODE_OAUTH_TOKEN`, Bedrock, Vertex, Foundry, or other cloud-provider
  credential.
- No Linux or Windows evidence, cross-platform claim, remote server, or second
  account.
- No login, logout, credential export/import, Keychain copying, setup-token,
  browser automation, CAPTCHA interaction, token refresh, or revocation test.
- No stable MultiAgentDesk-managed subscription login, account routing,
  subscription traffic proxy, quota dashboard, CredentialGrant, or billing
  claim.
- No MultiAgentDesk production code, PTY integration implementation, Phase 3
  approval, dashboard priority change, release decision, or compatibility
  matrix decision during intake.
- No dollar-budget flag or separate print/PTY monetary authorization. The
  boundary is the existing included Team subscription allowance, a maximum of
  two minimal requests, and fail-closed handling at the included-usage limit.

## User journeys

1. The operator keeps the existing official Claude Code Team subscription
   login and supplies no API-key, bearer-token, or cloud-credential override.
2. The provider-spike runner verifies the exact binary, sanitized auth class,
   absent override selectors, and no-tool/no-persistence controls without
   changing account or billing state.
3. The runner performs at most one minimal print probe. If the fixed marker is
   returned within the external deadline, only the sanitized result class is
   retained; otherwise the experiment stops without retrying.
4. If the print arm is safe and included usage remains available, the runner
   performs at most one minimal interactive PTY probe in an empty disposable
   workspace and records only sanitized lifecycle and marker classifications.
5. A security reviewer evaluates the resulting trust, PII, billing, and policy
   boundary. Only after an accepted review may feature-plan record a narrowly
   worded compatibility decision.

## Data and trust boundaries

- The user-installed official Claude Code binary and Anthropic receive the
  minimal probe text and may access the already authenticated subscription
  state. MultiAgentDesk does not receive, copy, proxy, or persist the OAuth
  credential.
- macOS Keychain, the default Claude profile, browser state, email,
  organization identifiers, raw auth JSON, runtime prompt/response captures,
  terminal transcript, and subscription usage values are outside retained
  evidence. The deterministic synthetic fixture definition remains reviewable
  in the harness source.
- Tools must be disabled. The process runs in an empty disposable workspace;
  a before/after manifest detects unexpected workspace or config writes. The
  experiment does not grant shell, file, network-tool, MCP, or repository
  capabilities to the model.
- Evidence records only allowlisted metadata: exact CLI version and binary
  digest, OS/architecture, auth/provider/subscription class, override-presence
  booleans, model-bearing CLI invocation count (not a Provider network-call
  count), exit/timeout classes, marker-match booleans, PTY lifecycle classes,
  and side-effect checks.
- Same-user malware, host administrator/root, Claude Code itself, Keychain,
  backups, crash tooling, and Anthropic remain external trust surfaces. This
  Spike neither strengthens nor changes them.

## Provider/external assumptions

- Exact Claude Code `2.1.207` is still installed and the existing account
  remains authenticated through `claude.ai` with Team subscription class. This
  is a precondition to revalidate, not a fact established by intake.
- No API-key/token/cloud override is active. If any override is present, the
  provider-spike must stop rather than guess which billing/auth source wins.
- The exact CLI exposes a way to disable tools and session persistence for both
  arms. If it does not, the hypothesis is falsified or inconclusive; the runner
  must not weaken the boundary.
- Included subscription usage is available for the two minimal probes. A
  quota/session-limit response is Provider availability state, not auth
  failure, and does not authorize a retry, account switch, API key, or usage
  credit opt-in.
- A successful direct official-CLI run is mechanism evidence only. Under ADR
  0016 it cannot be promoted into stable MultiAgentDesk-managed subscription
  login, routing, usage, or billing support without new Provider authorization
  and a fresh product/security lifecycle.

## Dependencies and gates

- ADR 0016 and its 2026-07-16 policy narrowing are authoritative.
- Prior exact-version Config Dir/Keychain and redacted auth-health evidence may
  inform the experiment but does not substitute for the new print/PTY run.
- Provider compatibility gate: exact binary/version, flags, auth source,
  print behavior, PTY behavior, and fail-closed outcomes require reproducible
  evidence under `docs/spikes/claude-subscription/`.
- Security Gate: `open`. Authenticated subscription state, PII handling,
  Provider policy, and possible billing transitions require independent
  `security-review` after `EVIDENCE_READY`.
- No dashboard-state change is authorized by this intake.

## Acceptance criteria

- [ ] Preflight records macOS `26.5.2` arm64, Claude Code `2.1.207`, the
      resolved binary digest, and an allowlisted auth projection showing the
      existing `claude.ai` Team subscription, with no PII or raw JSON retained.
- [ ] Preflight proves API-key, auth-token, Bedrock, Vertex, and Foundry
      overrides absent; any ambiguity stops the experiment without a request.
- [ ] The runner makes no login/logout, account, billing, usage-credit, browser,
      Keychain, or credential mutation and never opts into extra usage.
- [ ] At most one minimal print request runs with tools disabled, session
      persistence disabled, an external deadline, and a fixed response marker;
      retained evidence contains only sanitized outcome classifications.
- [ ] At most one minimal interactive PTY request runs with tools disabled in
      an empty disposable workspace, an external deadline, and a fixed response
      marker; retained evidence contains no runtime prompt/output capture or
      transcript.
- [ ] Before/after checks show no unexpected workspace/config file mutation,
      no model tool invocation, and no persisted session transcript. A failure
      is recorded truthfully and is not repaired by retrying.
- [ ] An included-usage/session-limit, upgrade, extra-usage, or ambiguous
      billing response stops the Spike and is classified separately from auth.
- [ ] The report explicitly labels any success as exact-version direct official
      Claude Code external/experimental mechanism evidence only and preserves
      ADR 0016's prohibition on stable managed subscription claims.
- [ ] Evidence is reproducible and sanitized under
      `docs/spikes/claude-subscription/`; raw capture and PII secret scans pass.
- [ ] Because the Security Gate is open, the Spike proceeds from
      `EVIDENCE_READY` to `security-review`, not directly to a compatibility or
      product decision.

## Risks and open questions

- Claude Code may update local config/cache despite no-session-persistence. An
  unexpected write fails the no-side-effect criterion; it must not be hidden.
- The exact CLI may not expose equally strong tool/persistence controls in
  interactive PTY mode. The provider-spike must inspect the pinned version and
  stop if the control cannot be demonstrated.
- Included usage availability can change between preflight and a request. A
  quota/session-limit result is acceptable negative evidence, not a reason to
  retry or enable usage credits.
- A fixed marker minimizes retained content but cannot prevent Anthropic from
  processing the probe. The operator has selected this bounded direct use of
  the official CLI; evidence still excludes its raw content.
- A successful experiment can be misquoted as product authorization. The
  report, Security Review, ADR, and compatibility decision must keep the
  external/experimental and managed-product boundaries explicit.
- Open question for the provider-spike: can exact `2.1.207` disable every
  model tool and session write in both print and interactive PTY modes without
  changing the existing authenticated profile?

## Evidence

- `docs/adr/0016-claude-profile-interactive-login-boundary.md`
- `docs/PROVIDER_COMPATIBILITY.md`
- `docs/spikes/claude/2026-07-14-config-keychain-spike.md`
- `docs/reviews/spike-claude-distinct-account-usage/2026-07-16-security-review.md`
- `docs/IMPLEMENTATION_PLAN.md` §9, §19 Phase 3, and §20.6
- Planned evidence: `docs/spikes/claude-subscription/`

## Handoff

Next role: `feature-plan`.
