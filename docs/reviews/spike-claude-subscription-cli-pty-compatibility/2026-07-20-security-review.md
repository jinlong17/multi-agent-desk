# Security Review: Claude Team-subscription CLI/PTY compatibility evidence

- Date: 2026-07-20
- Target: `spike-claude-subscription-cli-pty-compatibility`
- Owner module: `provider`
- Reviewed branch: `codex/provider/claude-subscription-cli-pty-compatibility`
- Base commit: `2d3b4162b72bff26d203c55bb63782b725464f87`
- Verdict: **ACCEPTED**

## Scope and conclusion

The safety and truthfulness of this Spike's negative result are accepted. One
model-bearing print CLI invocation/process was started with the pinned official
Claude Code `2.1.207` binary on macOS `26.5.2` arm64. The CLI continued using
its existing subscription OAuth login; the allowlisted projection was
`claude.ai`/`firstParty`/`team`, while API-key, bearer-token/OAuth-token, cloud,
gateway, settings-helper, and proxy overrides were absent. The process returned
the fixed marker with exit code `0` and retained only bounded classifications
and counts. No Provider network instrumentation was present, so this review
does not claim an exact HTTP-request count.

The strict compatibility hypothesis nevertheless failed because aggregate
metadata in the selected default config scopes changed during the print arm.
The one-shot ledger was claimed before spawn; no retry occurred; the PTY ledger
was not claimed and no PTY process was started. Treating the observed aggregate
metadata drift as acceptable after the run would weaken the frozen criterion,
so the fail-closed stop is the correct result.

This verdict accepts a sanitized falsification and one narrow direct-CLI print
mechanism fact. It does **not** accept positive CLI/PTY compatibility, a stable
MultiAgentDesk-managed Claude subscription surface, subscription credential or
traffic routing, Usage or billing support, Phase 3 completion, or any Linux or
Windows claim. ADR 0016's managed-product prohibition remains authoritative.

## Trust-boundary review

### Authentication and billing source

Current Anthropic authentication documentation distinguishes Claude for Teams
login from Console API billing and documents credential precedence: cloud,
bearer token, API key, helper, setup-token OAuth, then subscription OAuth. The
harness checks the relevant override selectors and credential-bearing settings
recursively, requires `firstParty` Team auth, and constructs a minimal child
environment while intentionally preserving the official CLI's existing
subscription OAuth login. That is sufficient for this exact one-shot
classification; it is not a reusable proof for later invocations or versions.

The operator attested before execution that Team usage credits were disabled.
Anthropic currently states that `claude -p` still draws from subscription usage
and that fully disabled usage credits prevent continued use after included
limits. The attestation is an external billing control, not a machine-verified
property of the harness. It safely bounded this run but must be re-established
for any separately authorized future experiment.

### Data minimization and local effects

The subprocess output and auth JSON were bounded in memory and reduced to an
allowlist before the evidence was written. Repository scans found no email-like
identity value, API-key prefix, credential value, raw auth JSON, or captured
terminal text. The deterministic public synthetic prompt template remains in
the reviewable harness source, and the corrected evidence explicitly records
`syntheticFixtureDefinitionRetained=true`. The corresponding
`runtimePromptCaptureRetained=false` and
`runtimeResponseCaptureRetained=false` claims mean that no runtime prompt or
response capture and no user content were persisted; they do not claim that the
public fixture definition was hidden.

Official session documentation says `CLAUDE_CODE_SKIP_PROMPT_HISTORY` suppresses
transcript writes in all modes and `--no-session-persistence` suppresses them
for one print run. The harness sets both controls, and the empty disposable
workspace remained unchanged. The durable evidence makes no per-path session,
cache, or settings attribution and does not prove zero Provider-side retention
or zero writes outside the inspected local scopes.

The selected config manifest intentionally failed on aggregate metadata drift.
No historical per-path diagnostic command or output was retained, so the
evidence truthfully declines to attribute the change to a cache, settings, or
session path. The durable evidence parsed, emitted, and retained no file
content. The drift may be ordinary CLI behavior, but this review does not
retroactively allowlist it.

### PTY, tools, and process control

The harness disables tools, MCP, slash commands, browser launch, prompt
history, and print session persistence, uses an empty workspace, bounds output
and time, and terminates a dedicated process group on failure. Its PTY arm would
deliver only a positional synthetic prompt and would not type into or answer an
unknown menu. Because print failed the side-effect gate, that arm remained
unclaimed and unexecuted. Consequently there is no real PTY, resize, lifecycle,
or persistence evidence to accept.

The executed harness source retains legacy internal field names such as
`providerRequestCount` and `requestCount`. The corrected durable report and JSON
do not interpret those labels as network measurements: they represent only a
model-bearing CLI invocation/process-spawn count. No Provider HTTP count was
observed. This naming limitation does not change the one-shot stop or the
negative verdict, and the frozen harness must not be edited and rerun to repair
historical labels.

### Product and policy boundary

The evidence consistently labels the run as direct official-CLI,
external/experimental mechanism evidence. MultiAgentDesk did not receive,
copy, export, proxy, or persist the subscription credential. The result cannot
supersede ADR 0016 or authorize stable subscription OAuth management, account
routing, status-line collection, quota dashboards, CredentialGrant, automatic
switching, or Provider traffic proxying. A new cache-write allowlist or a
separate PTY experiment requires a fresh feature-plan scope and a new one-shot
ledger; this Spike must not be retried or repaired in place.

## Findings

### P0

None.

### P1

None for accepting the sanitized negative evidence.

### P2 / required decision constraints

1. Feature-plan must record this as accepted **negative** compatibility
   evidence: print mechanism observed, strict no-write hypothesis falsified,
   PTY untested. It must not convert `ACCEPTED` into a support claim.
2. Preserve the print attempt ledger. Do not retry, delete the ledger, claim the
   PTY arm, or add a cache-write allowlist under this frozen Spike.
3. Keep ADR 0016's stable managed-subscription prohibition unchanged. Direct
   official CLI remains the deterministic fallback; Phase 3 remains unresolved.
4. Any future PTY or cache-behavior experiment needs a new feature-plan
   decision, exact-version preflight, explicit side-effect allowlist, fresh
   billing attestation, bounded one-shot execution, and another Security Gate.
5. Keep Provider-side retention and same-user/administrator/Keychain/crash-tool
   exposure explicit; local evidence minimization is not zero-data-retention.

## Verification evidence

- `docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-feature-brief.md`
- `docs/spikes/claude-subscription/2026-07-20-macos-team-cli-pty-spike.md`
- `docs/spikes/claude-subscription/2026-07-20-macos-team-cli-pty.json`
- `docs/spikes/claude-subscription/2026-07-20-print-attempt-claimed.json`
- `docs/spikes/claude-subscription/run_macos_team_cli_pty_probe.py`
- `docs/adr/0016-claude-profile-interactive-login-boundary.md`
- `docs/PROVIDER_COMPATIBILITY.md`
- JSON parse, Python AST parse, ledger-mode, PTY-ledger-absence, diff, and
  secret/PII/runtime-capture claim scans performed read-only during this review
- Read-only confirmation that corrected evidence records existing official-CLI
  subscription OAuth, one model-bearing CLI invocation, no observed Provider
  network count, no reproducible per-path attribution, a retained public
  synthetic fixture definition, and no retained runtime prompt/response capture
- [Anthropic authentication documentation](https://code.claude.com/docs/en/authentication)
- [Anthropic session persistence documentation](https://code.claude.com/docs/en/sessions)
- [Current `claude -p` subscription update](https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan)
- [Team usage-credit controls](https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans)

## Residual risk

Claude Code, Anthropic policy, credential precedence, billing, cache layout,
session persistence, and CLI flags can change. The operator attestation cannot
be cryptographically bound to the request, and an administrator could later
enable usage credits. The official CLI and Anthropic processed the synthetic
request and may retain it under Team policy. Same-user malware, host
administrator/root, Keychain, backups, crash tooling, and unexpected paths
remain outside the local evidence boundary. A user could still mistake the
successful print marker for product support unless the feature-plan decision
keeps the failed no-write criterion and absent PTY evidence prominent.

## Handoff

**Target**: `spike-claude-subscription-cli-pty-compatibility`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: `Accepted the safe, sanitized negative result: one model-bearing official-CLI Team-subscription print process returned its marker but violated the frozen aggregate config-metadata no-write criterion, so PTY was not run and no managed subscription or Phase 3 capability is supported; no Provider HTTP-request count is claimed.`
**Findings**: `P0 none; P1 none; P2 preserve the ledger and frozen failure, record only negative compatibility evidence, keep ADR 0016 unchanged, and require a new scoped Spike plus Security Gate for any cache allowlist or PTY run.`
**Evidence**: `feature brief; narrowed sanitized report/JSON; 0600 print ledger; one-shot harness; ADR 0016; compatibility row; read-only JSON/AST/permission/secret and wording confirmation; current official Anthropic auth, session, subscription-usage, and usage-credit documentation`
**Residual Risk**: `Provider policy/CLI/cache/billing drift, external billing attestation, Provider-side retention, same-user/admin/Keychain/crash exposure, and user confusion between one print mechanism fact and product support remain.`

### Next Step

Run `feature-plan`.
