# Feature Brief: Claude Team-subscription PTY-only compatibility evidence v2

- Slug: `spike-claude-subscription-cli-pty-compatibility-v2`
- Date: `2026-07-21`
- Owner module: `provider`
- Impacted modules: `security, project-system`
- Requested by: `operator direction to continue with the existing Claude.ai Team subscription after confirming usage credits are disabled`

## Module classification

```text
Owner: provider
Confidence: high
Why: the bounded Spike evaluates exact-version Claude Code PTY, TTY, subscription-auth, local-state, and Provider compatibility behavior.
Impacts: security, project-system
Branch: codex/provider/claude-subscription-pty-compatibility-v2
Workflow: spike
Gates: Provider compatibility evidence; Security Gate open because authenticated subscription state, Provider retention, and local credential/config boundaries are exercised
Docs: docs/reviews/spike-claude-subscription-cli-pty-compatibility-v2/2026-07-21-feature-brief.md; docs/workflow/features/spike-claude-subscription-cli-pty-compatibility-v2/{design.md,api.md,test.md,dev_log.md}
```

## Motivation and measurable outcome

The operator clarified that this experiment uses the already authenticated
official Claude Code CLI with an existing Claude.ai Team subscription. It does
not use a Claude Console API key, a cloud Provider credential, or an injected
Provider token. The operator also confirmed that Team usage credits are
disabled. That subscription-only boundary is the authorization and billing
control for this Spike; no dollar authorization, budget flag, or separate
print/PTY monetary limit is required or requested.

The previous Spike is immutable negative evidence. It ran one print process,
observed a marker, failed its aggregate selected-config metadata no-write
criterion, and correctly stopped before PTY. This v2 Spike is a new,
independent one-shot experiment. It must not run print, edit or invoke the old
runner, reuse or delete the old ledger, or repair the old result.

The measurable outcome is one sanitized PTY-only compatibility classification
for exact Claude Code `2.1.207` on macOS `26.5.2` arm64. Before the single
model-bearing process is claimed, the provider-spike must prove the exact
binary, subscription auth class, absence of credential/network overrides,
required CLI flags, a real TTY attachment path, and the frozen local-state
policy. The PTY may then request one public synthetic marker, exercise three
window resizes, and exit under a 75-second external deadline. No retry is
allowed.

Even a successful result is only version/platform evidence for direct use of
the official CLI in an external/experimental setting. It is not stable Claude
product support, a MultiAgentDesk-managed subscription integration, Phase 3
completion, or evidence for Linux or Windows.

## Scope

- macOS `26.5.2` arm64 and exact Claude Code `2.1.207`, with the previously
  observed binary SHA-256 revalidated before ledger claim and again immediately
  before spawn.
- Existing `claude.ai` / `firstParty` / `team` subscription login only.
- One PTY model-bearing CLI process maximum; zero print requests; zero retries.
- One fixed public synthetic prompt delivered as the initial positional
  argument, never typed into an unknown interactive menu.
- PTY stdin/stdout/stderr bound to the same slave, local no-model TTY fixture
  preflight, three exact window resizes, bounded rolling memory, process-group
  cleanup, and sanitized classification only.
- Tools, MCP, slash commands, browser launch, prompt history, continuation,
  resume, and session reuse disabled or absent.
- Empty disposable workspace and before/after state comparison under the exact
  allowlist in `design.md`.
- Fresh `0600` attempt ledger created atomically before the only possible
  model-process spawn.
- Independent Security Review before any compatibility decision.

## Non-goals

- No print-mode request and no invocation or modification of
  `run_macos_team_cli_pty_probe.py`.
- No API key, `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`,
  `CLAUDE_CODE_OAUTH_TOKEN`, setup-token, Bedrock, Vertex, Foundry, Mantle, AWS,
  gateway, proxy, credential helper, or alternate billing source.
- No enabling usage credits, upgrade action, purchase, dollar budget, paid
  overage, account switch, login/logout, token refresh, Keychain read/copy, or
  browser automation.
- No Linux, Windows, remote server, second account, distinct-account, quota,
  monthly-usage, or long-session claim.
- No production code, PTY adapter implementation, Phase 3 approval, dashboard
  change, ADR decision, compatibility-matrix decision, commit, push, or merge
  during intake.
- No claim that one CLI process equals one Provider HTTP request; network-call
  count remains unobserved unless the Provider exposes official evidence.

## User journey

1. The provider-spike revalidates the pinned macOS host and CLI without making
   a model request, retains only the allowlisted auth projection, and confirms
   usage credits disabled from the operator's recorded direction.
2. It proves the same runner path attaches all three child standard streams to
   a PTY and rejects any credential, token, cloud, gateway, proxy, settings, or
   flag ambiguity before creating a ledger.
3. It atomically claims a fresh `0600` PTY-v2 ledger, immediately repeats the
   pure pre-spawn gates, and starts at most one Claude process.
4. The process receives one public marker fixture as its positional prompt,
   performs three resizes, and is observed only through bounded in-memory
   marker and abort-pattern classification.
5. The runner records pass, negative, blocked, or inconclusive evidence without
   retrying. Raw auth JSON, prompt/response capture, terminal transcript, PII,
   secrets, and per-path content are not persisted.
6. A security reviewer checks the evidence and wording. Only after an accepted
   review may feature-plan record a narrow compatibility decision.

## Data and trust boundaries

- The official Claude Code binary and Anthropic process the public synthetic
  prompt and response under the user's Team plan. Anthropic may retain or
  process them under its subscription policy; this Spike cannot promise
  Provider-side zero retention.
- The existing subscription credential remains owned by the official CLI and
  macOS Keychain. MultiAgentDesk must not read, copy, export, proxy, persist, or
  route it.
- Raw `auth status --json` and PTY bytes may exist only in bounded process
  memory long enough to reduce them to allowlisted classifications. Durable
  evidence contains no email, organization, credential value, raw JSON,
  terminal text, session identifier, file content, prompt capture, or response
  capture. The public synthetic fixture definition and its digest may remain.
- Same-user malware, host administrator/root, Keychain, backups, crash tooling,
  Claude Code, and Anthropic remain external trust surfaces.
- Local state comparison may stream hashes in memory but must not persist
  per-file content or per-secret-file digests. Only category, count, size bound,
  and equality/allowlist booleans are durable.

## Provider assumptions and gates

- Exact `2.1.207`, macOS `26.5.2` arm64, and the pinned binary digest still
  match. Drift aborts before ledger claim.
- The existing auth projection is exactly logged-in `claude.ai`, `firstParty`,
  `team`. Raw fields outside that projection are discarded.
- The parent and child environments contain no API-key, token, cloud, gateway,
  proxy, or alternate auth/billing selector. Credential-bearing settings are
  absent and all inspected JSON parses successfully.
- Usage credits remain disabled. The operator's explicit statement is the
  required attestation for this authorized one-shot; no dollar amount is
  requested. Any limit, upgrade, extra-usage, or billing-choice surface aborts
  immediately and cannot authorize a retry.
- The exact CLI flag surface can disable tools/MCP/slash commands/browser and
  prompt history and can accept the initial prompt positionally. Otherwise the
  Spike is blocked before the request.
- Security Gate is `open` because authenticated subscription state and local
  auth/config boundaries are involved.
- ADR 0016 remains authoritative. Success cannot promote subscription OAuth
  into a stable managed product surface.

## Acceptance criteria

- [ ] No print command, old-runner execution, old-ledger mutation, API key,
      Provider token override, cloud credential, usage-credit enablement,
      dollar budget, login/logout, or retry occurs.
- [ ] Preflight records only macOS/architecture, exact CLI version and digest,
      allowlisted Team auth class, flag/TTY booleans, and override-absence
      booleans; all ambiguity aborts before ledger claim.
- [ ] A local no-model fixture proves stdin/stdout/stderr are TTYs through the
      same PTY construction used for Claude, and resize/cleanup primitives work.
- [ ] Exactly one fresh `0600` PTY-v2 attempt ledger is atomically claimed
      before any model-bearing process spawn; an existing ledger blocks the run.
- [ ] At most one Claude PTY process starts, receives one positional synthetic
      prompt, performs three resizes, stays within 75 seconds and 256 KiB total
      observed output, and never receives interactive approval/menu input.
- [ ] Tools, MCP, slash commands, browser launch, prompt history, resume,
      continuation, and session reuse are disabled/absent; any trust, tool,
      auth, billing, or usage prompt triggers immediate termination.
- [ ] Workspace, session/history/settings/auth paths, and repository files
      outside the fresh ledger and sanitized evidence remain unchanged. Only
      metadata-only touches that satisfy the exact content/mode/ownership/entry
      invariants in `design.md` may be classified as allowlisted.
- [ ] Durable evidence retains no raw auth JSON, prompt/response capture,
      terminal transcript, PII, secret, session ID, path content, or Provider
      request-count claim.
- [ ] Result wording distinguishes `SUPPORTED_EXACT_PTY`, safe negative,
      pre-request block, and inconclusive outcomes and always states that
      subscription success is exact version/platform external evidence only.
- [ ] Because Security Gate is open, `EVIDENCE_READY` proceeds to
      `security-review`, not directly to a decision.

## Risks and unresolved questions

- Claude Code may persist session or cache content even with prompt-history
  controls. Any content/entry/permission change outside evidence is a truthful
  negative result, not something to delete or retry around.
- The CLI can change between intake and execution. Version or digest drift
  blocks the request; no automatic installation or downgrade is allowed.
- A PTY may emit menus or prompts before the marker. The runner does not answer
  them; it classifies and aborts.
- Usage limits can change. An included-usage limit is Provider availability,
  not auth failure, and cannot trigger an account or billing-source switch.
- One PTY response cannot prove reconnect, replay, stop semantics, long-session
  reliability, stable policy permission, or product support.

## Evidence links

- `docs/adr/0016-claude-profile-interactive-login-boundary.md`
- `docs/PROVIDER_COMPATIBILITY.md`
- `docs/workflow/features/spike-claude-subscription-cli-pty-compatibility/dev_log.md`
- `docs/spikes/claude-subscription/2026-07-20-macos-team-cli-pty-spike.md`
- `docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-security-review.md`
- `docs/workflow/features/spike-claude-subscription-cli-pty-compatibility-v2/{design.md,api.md,test.md,dev_log.md}`

## Handoff

Next role: `provider-spike` after this feature-plan intake is structurally
verified. The provider-spike must implement the new v2 harness/evidence and may
execute only the frozen PTY-only one-shot; it must not run Claude during intake.
