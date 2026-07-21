# Spike log: Claude API-key CLI and PTY compatibility

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-claude-api-key-cli-compatibility` |
| Title | `Claude API-key auth, billing, CLI, PTY, and usage trust-boundary compatibility` |
| Owner Module | `provider` |
| Impacted Modules | `security`, `core`, `project-system` |
| Hypothesis | `On exact pinned macOS arm64 and Linux amd64 Claude Code versions, a dedicated Claude Console API key injected only after tuple and billing confirmation into an empty CLAUDE_CONFIG_DIR plus reviewed minimal environment is the sole effective auth source, supports a bounded ordinary paid CLI/PTY Session with deterministic redacted health/error and safe session-metric projection, and cleans up without persisting the key or raw identity/output.` |
| Time-box | `8 operator-active hours across documentation, macOS, Linux, sanitization, and cleanup arms; waiting for secret/host/billing authorization is excluded` |
| Current Phase | `INTAKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex (GPT-5) as feature-plan spike intake` |
| Updated | `2026-07-20 17:37 PDT` |
| Suggested Next | `provider-spike` |
| Security Gate | `open — Provider key, billing authorization, child environment, status/hook input, PTY control, local cleanup and retained evidence` |
| Evidence Path | `docs/spikes/claude-api-key/` |
| Decision Record | `pending docs/adr/0017-claude-api-key-cli-boundary.md and docs/PROVIDER_COMPATIBILITY.md exact rows` |

## Success and failure criteria

### Supported when

All criteria below pass with sanitized, reproducible evidence:

1. Official documentation URLs and review date freeze auth precedence, supported
   CLI flags, status/hook input fields, cost semantics, and relevant error
   classes without relying on private endpoints or human terminal parsing.
2. Exact binary path, version, SHA-256 digest, OS, architecture, install source,
   and invocation mode are pinned separately for macOS arm64 and Linux amd64.
3. A dedicated test API key is supplied through a non-recorded local channel;
   no key is printed, logged, persisted in Git, or placed in argv/shell history.
4. The operator explicitly authorizes each ordinary paid arm and its known
   maximum/expected cost before it runs. No request is sent merely to refresh a
   dashboard or probe hidden quota.
5. With an existing macOS Team subscription present as a crossover canary, the
   selected key is the sole effective auth source under an empty isolated
   `CLAUDE_CONFIG_DIR` and the accepted clean environment. Linux proves the same
   isolation without assuming macOS Keychain behavior.
6. Tests cover documented conflicts for inherited API/auth/OAuth tokens,
   endpoint/custom-header settings, credential helpers, Bedrock, Vertex,
   Foundry, and ambient cloud credentials. Each conflict is removed or fails
   closed exactly as recorded; none silently changes billing.
7. A bounded print/JSON arm demonstrates deterministic API-key auth and redacted
   result/error classes using the Spike-approved combination of `-p`, output
   format, `--max-turns`, `--max-budget-usd`, and
   `--no-session-persistence`. An invalid sentinel key covers the non-billed
   auth-error arm.
8. A minimal interactive PTY arm on each target proves input, at least three
   exact resizes, continuously drained output, graceful exit, forced cleanup,
   and no subscription/cloud fallback. Raw prompt/response content is not
   retained.
9. Auth status and Provider failures reduce to a bounded allowlist that
   distinguishes invalid auth, permission, billing/credit, rate, network/TLS,
   timeout, version/schema, and platform without email, organization, endpoint,
   request ID, raw body, or key fragment.
10. The candidate status-line/helper or hook path is strictly bounded and
    allowlisted. It either produces source/version/freshness-labelled
    client-estimated session cost and current-context token fields, or is
    rejected with a deterministic `usage_unavailable` fallback. Raw paths,
    transcript/session IDs, prompts, and unknown fields are not retained.
11. Process exit, cancel, timeout, CLI/helper crash, daemon-style parent crash,
    and config cleanup leave no key/raw identity/output in the sanitized
    evidence. The result explicitly documents same-user/root/child/crash-tool
    visibility as residual host risk.
12. The final report states an exact compatibility allowlist, stable failure
    codes, selected launch/status mechanism, Linux/macOS support limits, Windows
    typed fallback, and a no-request/no-key fallback when gates are unavailable.

### Falsified or inconclusive when

- API-key selection cannot be proven distinct from the existing subscription,
  Keychain/config, another API key, custom endpoint, or cloud-provider route.
- The accepted CLI requires durable plaintext key storage, argv delivery,
  undocumented credential copying, request interception, browser/session data,
  or private endpoints.
- A bounded ordinary request cannot distinguish auth, billing, rate, and
  transport failures without retaining sensitive raw output.
- PTY input/resize/termination or output draining is nondeterministic beyond the
  recorded bounds on either required platform.
- Status/hook data cannot be sanitized before persistence, or its cost/token
  meaning cannot be labelled without implying an actual bill, remaining limit,
  subscription window, monthly credit, or cumulative organization usage.
- Secret, identity, endpoint, transcript/workspace path, prompt, response, or
  live billing value appears in a retained report/JSON/log/Git artifact.
- The dedicated key, exact Linux host, or explicit paid-request authority is
  unavailable. In that case the paid/live arms are `INCONCLUSIVE` or `BLOCKED`;
  documentation and invalid-sentinel results alone cannot resolve the gate.
- Official policy or exact CLI behavior does not permit the proposed product
  surface. Passing a technical probe cannot override that failure.

## Experiment sequence

The sequence is ordered to minimize secret exposure and billed work:

1. Re-read current official environment/authentication, CLI, status-line,
   hooks, errors, cost/usage, and product-policy documentation. Record URLs and
   dates, not long quotations.
2. Pin macOS/Linux binary version, digest, platform/architecture, and sanitized
   dependency/config facts. Confirm the current subscription only through
   allowlisted auth class fields; do not retain email or organization.
3. With no real key, inspect environment-variable **presence only**, create
   disposable empty config roots, run version/help/local-shape checks, exercise
   an invalid sentinel key, and freeze the candidate minimal environment and
   error reducer.
4. Obtain the dedicated key through a non-recorded channel and the operator's
   explicit authorization for the bounded paid print arm. Keep the key outside
   shell history, argv, transcripts, artifacts, and Git.
5. Run exact conflict/isolation arms one at a time. Each child receives only the
   selected test key and accepted minimal environment; the harness records
   booleans/classes/digests/bounds, not raw child environment or output.
6. Run the smallest ordinary print/JSON request needed to bind API-key auth and
   health/error/metric projection. Stop at the approved turn/budget bound.
7. After separate confirmation where required, run the minimal interactive PTY
   arm on macOS and Linux, including input, three resizes, graceful exit and
   kill cleanup. Do not preserve conversation content.
8. Exercise sanitizer null/drift/oversize/canary cases and lifecycle failure
   injection without additional paid requests.
9. Stop all children, remove disposable config roots, scan retained artifacts,
   revoke the dedicated key in Console when the operator ends the experiment,
   and record that local cleanup alone is not remote revocation.
10. Write sanitized report/JSON, update this log to `EVIDENCE_READY` or
    `INCONCLUSIVE`, then hand the open Security Gate to independent
    `security-review`.

## Environment

| Field | Value |
|---|---|
| Tool + version | planning host Claude Code `2.1.207`; exact macOS and Linux experiment binaries/digests must be pinned at execution |
| OS | planning host macOS 26.5.2 arm64; operator-selected Linux amd64 target required; Windows real Provider arm excluded |
| Auth mode | planning host currently `claude.ai` Team subscription and no API-key/cloud override; experiment requires a separate dedicated Claude Console API key |

The current subscription is a negative crossover canary only. It must not send
the Spike request or satisfy API-key acceptance.

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-20 17:20 PDT | Feature Brief, ADR 0016, implementation plan, compatibility matrix, prior Claude Spike/security decision | stable managed subscription surface is disabled; API-key/cloud requires a new lifecycle; prior config/auth evidence is mechanism-only | repository documents |
| 2026-07-20 17:32 PDT | Anthropic official environment, CLI, status-line, hook and error references | docs describe API-key precedence, bounded print-mode flags and client-estimated status-line cost; exact CLI/PTY/isolation/sanitizer behavior remains unproven | official URLs recorded in parent design |
| 2026-07-20 17:37 PDT | `/Users/jinlong/.local/bin/claude --version`; redacted auth-status projection; presence-only checks for API/OAuth/cloud variables | planning host `2.1.207`, current `claude.ai` Team/first-party class, checked overrides absent; no PII/secret emitted and no Provider request made | planning terminal evidence only |
| 2026-07-20 17:37 PDT | `feature-plan` Spike intake | falsifiable two-platform hypothesis, eight-hour active time-box, ordered no-key/paid/PTY arms, Security Gate and deterministic fallbacks frozen | this log |

## Result, limitations, and fallback

No experiment has run yet. `SPIKE_READY` authorizes only the bounded
`provider-spike` procedure above; it is not API-key, PTY, usage, or platform
support evidence.

If the operator cannot yet provide the dedicated key, Linux target, or paid-
request authorization, preserve the completed documentation/no-key evidence and
record the missing arm honestly. The stable fallback remains: no managed Claude
Session, direct official Claude Code outside MultiAgentDesk, or a later
separately reviewed supported-cloud feature. Never fall back to the currently
logged-in subscription, setup-token, another key, a gateway, or hidden quota
probe.

If status/metric projection alone fails but auth/environment/PTY passes, the
candidate feature may use explicit `usage_unavailable` only if Security Review
and the revised feature plan accept that scope. If auth-source isolation or PTY
cleanup fails, the entire Session capability remains unsupported for that exact
tuple.

## Risks and Blockers

- A dedicated test API key is not currently present and must be injected through
  a non-recorded local channel. Never request that it be pasted into chat.
- No billed request is authorized by this intake. Each paid arm needs explicit
  operator approval and a stated bound.
- Environment delivery exposes the key to the child and may expose it to
  same-user/root/crash tooling. This cannot be described as zero exposure.
- `--max-budget-usd` applies to print mode; the interactive PTY arm requires a
  separate bounded task and explicit authorization rather than an assumed hard
  dollar cap.
- `--bare` may disable settings, hooks, MCP, plugins, and session persistence in
  ways that invalidate the required product path. It is a candidate to compare,
  not a preselected answer.
- Status-line/hook JSON contains sensitive paths and identifiers and may change
  across exact versions. Unknown fields or schemas must fail the projection.
- The local cost value is client-estimated and may differ from the bill. It
  cannot populate Console spend/balance or subscription quota fields.
- Passing macOS does not prove Linux, and passing both does not prove Windows or
  another version/architecture.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-20 17:37 PDT | Codex (GPT-5) as feature-plan spike intake | Classified the compatibility question as solely `provider`-owned with an open Security Gate; froze the two-platform API-key/auth/billing/PTY/status hypothesis, eight-hour active time-box, ordered no-key then explicitly authorized paid arms, success/failure criteria, redaction and deterministic fallbacks | this log; parent Feature Brief/design/API/test; no commit | `SPIKE_READY`; no real key supplied, no billed request run, and no compatibility/support claim changed | `provider-spike` after non-recorded dedicated-key injection, Linux target availability, and explicit bounded paid-request authorization |
