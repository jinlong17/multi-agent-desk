# Spike log: Claude subscription CLI/PTY compatibility on macOS

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-claude-subscription-cli-pty-compatibility` |
| Title | `Claude subscription CLI/PTY compatibility on macOS` |
| Owner Module | `provider` |
| Impacted Modules | `security, core, project-system` |
| Hypothesis | `On macOS 26.5.2 arm64, exact Claude Code 2.1.207 authenticated only through the existing claude.ai Team subscription can complete no more than one minimal print probe and one minimal interactive PTY probe using included subscription access, with API-key/token/cloud overrides absent, tools disabled, no unexpected file or session side effects, and only sanitized outcome evidence retained; success supports direct official-CLI external/experimental mechanism evidence only, not a stable MultiAgentDesk-managed subscription claim` |
| Time-box | `45 hands-on minutes; at most one print request and one PTY request; externally bounded to 120 seconds per arm` |
| Current Phase | `SPIKE` |
| Status | `GATE_RESOLVED` |
| Executor | `Codex (GPT-5) as feature-plan` |
| Updated | `2026-07-20 22:55 PDT` |
| Suggested Next | `none — preserve the resolved negative decision; any cache-write allowlist or PTY experiment starts a new feature-plan Spike intake with a fresh one-shot ledger and Security Gate` |
| Security Gate | `resolved — independent review confirmed the narrowed wording and accepted the safety and truthfulness of the sanitized negative evidence only; one model-bearing print CLI process violated the frozen aggregate config-metadata no-write criterion, PTY was not run, no Provider network-request count was observed, and no stable managed subscription or Phase 3 capability is supported` |
| Evidence Path | `docs/spikes/claude-subscription/` |
| Decision Record | `GATE_RESOLVED — docs/PROVIDER_COMPATIBILITY.md records security-reviewed negative evidence for exact Claude Code 2.1.207 on macOS 26.5.2 arm64: one direct official-CLI print process returned its marker but failed the frozen aggregate selected-config metadata no-write criterion; PTY was not run; Provider HTTP-request count was not observed; no positive CLI/PTY compatibility, stable managed subscription, Usage/billing/account capability, or Phase 3 claim is authorized; ADR 0016 remains unchanged` |

## Success and failure criteria

- Supported when: exact-version preflight proves the macOS arm64 binary and
  sanitized existing `claude.ai` Team auth class with all API-key/token/cloud
  overrides absent; at most one no-tool/no-persistence print arm and one
  no-tool interactive PTY arm each return the fixed marker within their
  external deadline; before/after checks find no unexpected workspace/config
  writes, tool use, or persisted transcript; retained evidence contains only
  allowlisted classifications; and the report explicitly limits the result to
  direct official Claude Code external/experimental mechanism evidence.
- Falsified when: the binary/version/auth source differs; a credential override
  or billing source is ambiguous; tools or persistence cannot be disabled; a
  probe mutates files, invokes a tool, or retains raw prompt/output/PII; Claude
  requests an upgrade or usage-credit opt-in; included usage is unavailable;
  either arm exceeds its one-request/120-second bound; or the evidence cannot
  be stated without implying a stable MultiAgentDesk-managed subscription
  product claim. Quota/session-limit is recorded as Provider availability, not
  authentication failure, and never authorizes a retry or account switch.

## Environment

| Field | Value |
|---|---|
| Tool + version | `Claude Code 2.1.207; binary SHA-256 1397a062c6889675055e3314dd956376ac51262a7734ad9e819c26975d71547a` |
| OS | `operator-selected macOS 26.5.2 arm64 only; Linux and Windows excluded` |
| Auth mode | `existing claude.ai Team subscription only; no API key, CLAUDE_CODE_OAUTH_TOKEN, Bedrock, Vertex, Foundry, setup-token, login, or logout` |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-20 20:53 PDT | Feature-plan intake against operator-corrected subscription-only scope, ADR 0016, exact-version compatibility record, and Spike/security workflow contract; no Provider call executed | Hypothesis, one-print/one-PTY limits, no-tool/no-file/no-raw-evidence controls, included-usage stop rule, macOS-only environment, and open Security Gate frozen | `docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-feature-brief.md`; this log |
| 2026-07-20 22:37 PDT | One-shot harness after Team Owner usage-credits-disabled attestation; exact host/SHA, Team auth, override, settings, flag and selected-config preflights repeated before print spawn | Print invocation returned the fixed marker with exit `0`, proving the narrow direct Team-subscription print mechanism, but selected config metadata changed; strict hypothesis falsified, PTY ledger unclaimed and PTY process not run; one model-bearing CLI invocation, no retry or runtime raw-content capture retained; Provider network-call count not instrumented | `docs/spikes/claude-subscription/2026-07-20-macos-team-cli-pty-spike.md`; sanitized JSON sibling; `2026-07-20-print-attempt-claimed.json`; harness; compatibility matrix |

## Result, limitations, and fallback

The strict hypothesis is falsified with reproducible sanitized negative
evidence. Exact Claude Code `2.1.207` on macOS `26.5.2` arm64, authenticated as
`claude.ai` / `firstParty` / `team` with no API-key, token, cloud, gateway or
proxy override, completed one no-tool print request and returned the fixed
marker with exit code `0`. The empty disposable workspace remained unchanged.

The selected default config scopes did not remain metadata-identical. The
durable evidence proves aggregate metadata drift but retains no historical
per-path diagnostic command or output, so it does not attribute that drift to
specific cache, settings, or session paths. Because intake authorized no
config-write allowlist, the harness classified the print arm
`UNEXPECTED_LOCAL_WRITE` and stopped. It did not claim or spawn PTY. Exactly one
model-bearing print CLI invocation was started and no retry occurred; Provider
network-call count was not instrumented.

This proves only a narrow direct official-CLI Team-subscription print mechanism
fact. It does not establish positive CLI/PTY compatibility under the frozen
criteria and cannot establish a stable MultiAgentDesk-managed subscription
login, credential, traffic, account, Usage, or billing capability. ADR 0016's
policy narrowing remains in force.

The deterministic fallback is to continue using the official Claude Code CLI
directly outside MultiAgentDesk. Do not delete the attempt ledger, retry this
Spike, substitute an API key/cloud source, enable usage credits, or weaken the
criteria after observing the result. A cache-write allowlist or separate PTY
experiment requires a new feature-plan decision.

## Risks and Blockers

- Provider-spike evidence is complete and Security Review accepted its safety
  and truthful negative framing. The strict no-config-write criterion failed;
  this is the final decision, not a reason to delete the ledger or retry.
- An included-usage/session-limit response is a legitimate negative or
  inconclusive outcome. It is not an auth failure and does not authorize a
  retry, extra-usage opt-in, alternate credential, or dollar-budget path.
- The official CLI may write config/cache even when transcript persistence is
  disabled. Before/after manifests make such behavior visible; unexpected
  writes fail the criterion rather than being silently removed or ignored.
- Interactive PTY control may be weaker than print-mode control in exact
  `2.1.207`. If tools or persistence cannot be proven disabled, do not run that
  arm.
- Authentication projection, terminal output, and Provider errors may contain
  PII or raw content. Only allowlisted classifications may be persisted; raw
  captures remain transient and must not enter the evidence artifact.
- A successful direct official-CLI print process does not supersede ADR 0016 or
  authorize stable MultiAgentDesk subscription management. The resolved
  compatibility decision preserves that distinction.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-20 20:53 PDT | Codex (GPT-5) as feature-plan spike intake | Replaced the mismatched API-key/Linux/paid-request hypothesis with the operator-selected macOS-only official Claude Code Team-subscription experiment; froze exact CLI `2.1.207`, one print plus one PTY maximum, no credential override, no tool/file/raw-evidence side effects, included-usage-only stop rules, ADR 0016 product-boundary language, and an open Security Gate | `docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-feature-brief.md`; this log | `SPIKE_READY`; no Provider call, plan self-approval, product edit, commit, push, dashboard change, or compatibility decision performed | `provider-spike` |
| 2026-07-20 22:37 PDT | Codex (GPT-5) as provider-spike | Recorded the operator's Team Owner usage-credits-disabled attestation; executed the independently audited one-shot subscription-only harness; revalidated exact host/SHA, Team auth, absent credential/network overrides, settings and selected config scopes; retained only allowlisted evidence; then stopped without PTY or retry when print changed selected config metadata | report; sanitized JSON; `0600` print attempt ledger; harness; `docs/PROVIDER_COMPATIBILITY.md`; this log | `EVIDENCE_READY`; print marker/exit passed, strict no-write hypothesis falsified, PTY not run, Provider request count `1`, Security Gate remains open | `security-review` |
| 2026-07-20 22:44 PDT | Codex (GPT-5) as security-review | Independently reviewed the frozen brief, one-shot harness, sanitized report/JSON, `0600` print ledger, absent PTY ledger, ADR 0016 boundary, compatibility wording, secret/PII minimization and current official Anthropic auth/session/subscription-usage/usage-credit documentation; made no Provider request and did not modify evidence | `docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-security-review.md`; this log | `ACCEPTED`; accepted the safe and truthful negative evidence only, with P0/P1 none; print's config-metadata write remains a failed criterion, PTY remains untested, ADR 0016 remains authoritative, and no managed subscription or Phase 3 capability is supported | `feature-plan decision` |
| 2026-07-20 22:51 PDT | Codex (GPT-5) as provider-spike evidence correction | Applied an independent read-only evidence audit without any Provider call: clarified that the official CLI used its existing subscription OAuth while credential overrides were absent; relabeled the one-shot counter as a model-bearing CLI invocation rather than a network request count; removed unreproducible per-path metadata attribution; and clarified synthetic-fixture versus runtime-capture retention | feature brief; sanitized report/JSON; this log | Core negative result unchanged; `ACCEPTED` remains pending security-review confirmation of the narrowed wording, with no retry, PTY claim, compatibility support, or product claim | `security-review confirmation, then feature-plan decision` |
| 2026-07-20 22:52 PDT | Codex (GPT-5) as security-review confirmation | Re-read the narrowed report/JSON and one-shot ledger without executing Claude or any Provider call; confirmed existing official-CLI subscription OAuth with only override absence claimed, one model-bearing CLI invocation/process spawn with no Provider network-count claim, aggregate selected-config metadata drift with no historical per-path attribution, and retained synthetic fixture definition without retained runtime prompt/response capture; aligned only the security-review record | `docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-security-review.md`; this log | `ACCEPTED` reaffirmed for safe and truthful negative evidence only; P0/P1 remain none, PTY remains unrun, the frozen no-write criterion remains failed, and no managed subscription, network-count, compatibility-support, or Phase 3 claim is authorized | `feature-plan decision` |
| 2026-07-20 22:55 PDT | Codex (GPT-5) as feature-plan decision | Classified the final decision as provider-owned with security, core, and project-system impacts; recorded the accepted negative evidence in the compatibility matrix while preserving ADR 0016 and the frozen one-shot boundary; made no Claude/Provider call, retry, PTY claim, ledger change, product edit, dashboard change, commit, or push | `docs/PROVIDER_COMPATIBILITY.md`; this log | `GATE_RESOLVED`; exact Claude Code `2.1.207` on macOS `26.5.2` arm64 returned the print marker but failed the aggregate selected-config metadata no-write criterion; PTY and Provider HTTP count remain unobserved, and no positive CLI/PTY compatibility, managed subscription, Usage/billing/account capability, or Phase 3 claim is authorized | `none; a cache-write allowlist or PTY experiment requires a new feature-plan Spike intake, fresh one-shot ledger, and Security Gate` |
