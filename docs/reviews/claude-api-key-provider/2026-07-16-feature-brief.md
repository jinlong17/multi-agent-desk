# Feature Brief: Claude API-key provider vertical slice

- Slug: `claude-api-key-provider`
- Date: `2026-07-16`
- Owner module: `provider`
- Impacted modules: `core`, `security`, `desktop`, `project-system`
- Requested by: operator continuation of `multi-account-usage-control` P3 after the subscription-policy decision

## Module classification

Owner: `provider`  
Confidence: high  
Why: the feature adapts official Claude Code CLI/PTY behavior, API-key auth,
Provider health, cost/Usage semantics, and explicit Account selection.  
Impacts: `core` for Vault-backed credential/session/PTY contracts, `security`
for high-value key and billing-source boundaries, `desktop` for later platform
capability display, and `project-system` for workflow/dashboard truth.  
Branch: `codex/provider/claude-api-key-provider`  
Workflow: `feature` with a prerequisite Provider Spike  
Gates: open Provider evidence and Security Gate; real Linux PTY acceptance;
macOS/Windows capability acceptance remains separate.  
Docs: this Feature Brief and
`docs/workflow/features/claude-api-key-provider/`.

## Motivation and measurable outcome

Current Anthropic guidance does not support a stable MultiAgentDesk-managed
Claude subscription OAuth/Usage surface for a product/tool used by others. It
directs product developers to Claude Console API-key or supported-cloud
authentication. The parent P3 subscription scope must therefore be replaced,
not implemented.

The first measurable outcome is a narrow Claude Console API-key vertical slice:
one operator-supplied key sealed in the target Device Vault, one explicitly
selected Account/Profile/Workspace and billing source, one real official Claude
Code PTY Session on Linux, truthful auth/health plus session-local cost/usage
metadata, and bounded stop/kill/cleanup. Supported cloud providers, managed
subscription OAuth, and product dashboards remain later capability-gated work.

## Scope

- Run a bounded prerequisite Provider Spike against current official Claude
  Code documentation and exact CLI versions to freeze API-key precedence,
  validation, error taxonomy, PTY/status/hook behavior, and safe cost/Usage
  projection without storing a real key or raw output in Git.
- Add a Claude API-key CredentialInstance stored only in the Device Vault,
  associated with exactly one Account and Device. Profile settings store only
  non-secret model/permission/runtime configuration.
- Accept key input through a secret-safe local channel; never command-line
  arguments, shell history, Profile JSON, dashboard state, logs, IPC error text,
  or process listings. Zero transient buffers best-effort.
- Preview and explicitly confirm Provider, Account/Profile, Device/Workspace,
  `api_key` auth source, billing source, exact CLI compatibility, and current
  Provider health before Session start.
- Launch the official Claude Code CLI through the existing cross-platform PTY
  abstraction with an isolated `CLAUDE_CONFIG_DIR` and a minimal allowlisted
  environment. Prevent inherited subscription/API/cloud credentials from
  silently overriding the selected key.
- Implement real Linux input, resize, observe/replay, ControllerLease, graceful
  stop plus kill escalation, restart/reconnect behavior, and explicit alias
  selection for the first accepted slice.
- Collect only Provider-documented, source-labelled Session health and
  session-local cost/token facts available from the official CLI/status-line or
  hook contract. Do not represent subscription 5h/7d windows or monthly credit
  as API-key limits; unavailable data remains unavailable.
- Render macOS and Windows capability status truthfully. Cross-build and PTY
  mechanism tests are required, but stable real-provider support waits for
  separate live acceptance on each platform.

## Non-goals

- Managed Claude.ai subscription OAuth, distinct subscription accounts,
  subscription 5h/7d Usage dashboard, Agent SDK monthly credit, setup-token,
  `CLAUDE_CODE_OAUTH_TOKEN`, or Keychain copying.
- Amazon Bedrock, Google Cloud/Vertex, Microsoft Foundry, gateways, or all cloud
  providers in the first build; each requires separate auth/billing evidence.
- API proxying, request interception/replay, credential pooling, automatic
  account rotation, mid-Session switching, or quota evasion.
- CredentialGrant, Control Plane secret sync, Web/Desktop product UI, or remote
  browser control in the initial vertical slice.
- Claiming Console organization spend/remaining limit from session-local cost
  estimates or undocumented endpoints.

## User journeys

1. The operator creates a Claude API-key Account/Profile, enters the key through
   a local secret prompt, confirms the Console/API billing source, and sees only
   redacted health metadata.
2. The operator previews `@claude-api`, confirms the exact tuple and billing
   source, then starts a Linux PTY Session and can attach, observe, acquire
   control, send input/resize, stop, and reconnect without exposing the key.
3. If an inherited `ANTHROPIC_API_KEY`, OAuth token, Bedrock/Vertex/Foundry
   selector, or conflicting Profile setting exists, launch fails closed or
   removes it from the child environment according to the reviewed contract;
   it never silently bills another Account.
4. The operator views source-labelled session-local cost/token facts or an
   explicit unavailable state. No subscription window or monthly-credit field
   appears.
5. Logout/revoke removes only the selected local Vault item and prevents future
   Sessions; it does not claim remote key revocation or erasure.

## Data and trust boundaries

- The API key is a high-value Provider secret. It exists encrypted in the local
  Vault and transiently in Daemon/child-process memory/environment only; the
  Control Plane, Web, Git, logs, audit payloads, and Profile settings never
  receive it.
- Environment variables are readable by the child and may be observable to
  same-user/root tooling on some platforms. Same-user malware, host admin,
  official Claude Code, crash/backup tooling, and a compromised dependency
  remain residual risks.
- Explicit billing-source confirmation is security-relevant because API-key
  environment variables override subscription auth and can charge a different
  Console Account.
- Status-line/hook payloads contain paths, transcript/session identifiers,
  model/cost/token data, and possibly future fields. A local sanitizer must
  allowlist minimal typed values before persistence.
- Local IPC capability checks and ControllerLease protect Session control;
  alias selection never replaces authorization.

## Provider assumptions requiring a Spike

- Current official guidance says `ANTHROPIC_API_KEY` takes precedence over an
  authenticated subscription, but exact CLI validation/error/PTY behavior must
  be pinned on the intended versions.
- The exact minimal environment and whether an empty isolated
  `CLAUDE_CONFIG_DIR` plus API key avoids Keychain/subscription crossover must
  be reproduced on macOS and Linux; Windows requires a later real target.
- Status-line/session-local cost and token fields are documented, but their
  availability and binding under API-key auth require a normal operator-
  approved request, not a hidden quota probe.
- Remote key revocation and Console organization limits are outside the CLI
  contract unless a separately documented official API is selected.

## Dependencies and gates

- `multi-account-usage-control` P1 reconciliation: `VERIFIED`.
- `spike-claude-distinct-account-usage`: `GATE_RESOLVED` negative subscription
  decision.
- ADR 0016 policy narrowing and compatibility matrix are authoritative.
- A new Provider Spike and independent Security Review must resolve the API-key
  CLI/PTY/credential/billing assumptions before feature build.
- Feature Security Gate is open for Vault storage, secret input/environment,
  billing confirmation, hook/status redaction, local revocation, PTY control,
  and platform behavior.
- The plan must be independently reviewed; each build phase stops for
  independent verification.

## Acceptance criteria

- [ ] A prerequisite Spike pins exact macOS/Linux CLI versions, API-key
      precedence/validation, empty-config isolation, sanitized auth/health,
      real Linux PTY behavior, ordinary-work cost/token projection, failure
      taxonomy, cleanup, and deterministic fallback; Security Review accepts it.
- [ ] API key enters through a secret-safe local prompt, is sealed only in the
      selected Device Vault, and never appears in argv, Profile settings,
      database plaintext, logs, dashboard, audit payloads, Git, or test output.
- [ ] Preview/confirmation binds Account, Profile, Credential, Device,
      Workspace, auth source, billing source, exact compatibility, and revision;
      stale/mismatched/ambiguous input fails before Session creation.
- [ ] Child environment contains only reviewed variables, uses an isolated
      `CLAUDE_CONFIG_DIR`, and cannot silently inherit subscription OAuth,
      another API key, or cloud-provider routing.
- [ ] One real Linux Session supports input, exact resize, observer replay,
      ControllerLease transfer, graceful stop/kill escalation, reconnect, and
      bounded cleanup with no secret in output or daemon logs.
- [ ] Stored Usage/cost is Account/Profile-bound, source/version/freshness
      labelled, replay-deduplicated, and limited to official API-key/session
      facts; subscription 5h/7d and monthly credit remain unavailable.
- [ ] Revocation blocks new local use and removes the selected Vault item/
      materialization without claiming remote key deletion.
- [ ] macOS/Linux/Windows builds and PTY mechanism tests pass; UI/docs label
      only live-accepted platforms stable and retain typed unsupported states.
- [ ] Full Go/race, migration/restart, secret-redaction, adversarial env, Web/
      Rust scaffold, workflow/dashboard/governance, and rollback checks pass.

## Risks and unresolved questions

- The feature cannot progress to build until the operator supplies a dedicated
  test API key through a non-recorded secret channel and authorizes an ordinary
  request with known billing impact.
- CLI behavior may still consult Keychain/config state even with an API key;
  the Spike must prove the exact precedence/isolation behavior rather than rely
  only on documentation.
- Environment delivery is inherently visible to the child and may be visible
  to same-user OS tooling. The plan must compare environment, inherited file,
  and stdin/FD mechanisms supported by the official CLI.
- A locally reported cost is not an organization invoice or remaining budget.
  Naming and confidence must prevent billing misunderstanding.
- Supporting multiple cloud providers in one phase would combine incompatible
  identity, SDK, billing, and credential contracts; keep the first slice to
  Claude Console API key.

## Evidence

- `docs/reviews/spike-claude-distinct-account-usage/2026-07-16-security-review.md`
- `docs/spikes/claude-distinct-accounts/2026-07-16-policy-and-isolation-spike.md`
- `docs/adr/0016-claude-profile-interactive-login-boundary.md`
- `docs/workflow/features/multi-account-usage-control/p1-as-built.md`
- `docs/spikes/claude/2026-07-14-config-keychain-spike.md`
- [Anthropic login guidance](https://support.claude.com/en/articles/13189465-log-in-to-your-claude-account)
- [API-key precedence guidance](https://support.claude.com/en/articles/12304248-manage-api-key-environment-variables-in-claude-code)
- [Claude Code environment variables](https://code.claude.com/docs/en/env-vars)
- [Claude Code status-line schema](https://code.claude.com/docs/en/statusline)

## Handoff

Next role: `feature-plan`.
