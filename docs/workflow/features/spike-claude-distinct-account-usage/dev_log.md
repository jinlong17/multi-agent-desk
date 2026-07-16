# Spike log: Claude distinct-account isolation and usage telemetry

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-claude-distinct-account-usage` |
| Title | `Claude distinct-account isolation and usage telemetry` |
| Owner Module | `provider` |
| Impacted Modules | `core, desktop, web, security` |
| Hypothesis | `Two different operator-owned Claude.ai accounts can remain simultaneously authenticated under separate CLAUDE_CONFIG_DIR profiles with scoped logout and real-request identity isolation, while official status-line events yield account-bound 5-hour/7-day usage windows without hidden quota-consuming probes and the stable product policy boundary can be documented` |
| Time-box | `8 hands-on hours plus Provider policy review; requires two accounts and one Linux target` |
| Current Phase | `PROVIDER SPIKE` |
| Status | `EVIDENCE_READY` |
| Executor | `Codex (GPT-5) as provider-spike` |
| Updated | `2026-07-16 16:43 PDT` |
| Suggested Next | `security-review` |
| Security Gate | `open — distinct OAuth identities, macOS Keychain slots, auth/usage PII, PTY/status-line events, policy and logout are in scope` |
| Evidence Path | `docs/spikes/claude-distinct-accounts/2026-07-16-policy-and-isolation-spike.md`; sanitized JSON sibling |
| Decision Record | `pending — ADR 0016 addendum or replacement plus PROVIDER_COMPATIBILITY.md` |

## Success and failure criteria

- Supported when: two distinct identities complete official login into clean
  Profile A/B on macOS; version-gated `auth status` plus minimal real requests
  prove expected identity and scoped logout; Linux isolation reproduces;
  status-line JSON yields 5h/7d used/reset fields bound to the correct Profile
  after ordinary user work; and an accepted decision defines whether the local
  self-hosted product may expose this subscription metadata.
- Falsified when: Keychain/config state crosses Profiles, logout is global,
  actual request identity differs, usage cannot be assigned to a Profile,
  collection requires parsing private endpoints/browser state or hidden prompts,
  or Provider policy does not permit the proposed stable surface.

## Environment

| Field | Value |
|---|---|
| Tool + version | initial macOS candidate Claude Code `2.1.207`; exact Linux version to pin |
| OS | macOS 26.5.2 arm64 + operator-selected Linux |
| Auth mode | two distinct Claude.ai subscription identities via official login; setup-token remains a separate unsupported stable path under ADR 0016 |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-15 01:18 PDT | official env/auth/CLI/status-line/errors/Agent SDK/legal docs | `CLAUDE_CONFIG_DIR` explicitly supports side-by-side accounts; status-line documents 5h/7d used/reset fields after first response; monthly `claude -p` credit is described but no machine remaining/reset contract established; third-party login/rate-limit policy needs applicability decision | official URLs in parent Feature Brief |
| 2026-07-15 01:20 PDT | prior ADR 0016/Spike audit | same-account Keychain slot isolation and scoped logout passed; distinct identities, completed setup-token, long session and stable subscription dashboard were not claimed | prior Claude Spike and ADR 0016 |
| 2026-07-15 01:31 PDT | feature-plan intake | distinct-account/status-line/no-hidden-probe/policy hypothesis frozen; Security Gate opened | this log |
| 2026-07-16 16:43 PDT | current official login/policy, Agent SDK, environment, status-line and API-key precedence documents; sanitized macOS `2.1.207` and Linux `2.1.132` auth-contract/empty-root/override checks | stable third-party subscription surface falsified at policy gate: products/tools for others are directed to API-key or supported-cloud auth; third-party subscription use is discretionary, can change billing, and is not a stable entitlement; no second login or quota-consuming request run | `docs/spikes/claude-distinct-accounts/2026-07-16-policy-and-isolation-spike.md`; sanitized JSON sibling |

## Result, limitations, and fallback

The stable-product hypothesis is falsified at the Provider policy boundary.
Current official guidance directs developers building a product/tool for
others to API-key or supported-cloud authentication. Subscription use from
third-party tools is discretionary, may draw usage credits, and is not a stable
entitlement. The announced Agent SDK monthly-credit change is paused and that
credit is unavailable.

Technical `CLAUDE_CONFIG_DIR` isolation remains documented and prior
same-account slot/logout evidence remains valid, but two distinct identities
were not tested because a successful technical run cannot close the policy
gate. No quota-only request was issued. Stable fallback is direct official CLI
use outside MultiAgentDesk or user-supplied API-key/cloud-provider integration
with explicit billing source. Subscription login/usage remains outside the
stable product absent explicit Anthropic approval.

## Risks and Blockers

- Requires two operator-owned Claude accounts and explicit participation in
  official login; no hCaptcha/CAPTCHA bypass or browser session manipulation.
- `rate_limits` appears after the first API response. The experiment must
  piggyback on operator-approved real work and must not spend quota merely to
  make the dashboard look fresh.
- The monthly Agent SDK credit may exist only on the account web surface or an
  unsupported internal contract; absence must remain truthful.
- Official policy wording may require Provider approval. This is a release gate,
  not a technical check that can be waived by passing local tests.
- Email, org, raw status JSON, transcript and usage values must be reduced to
  the minimum sanitized evidence before persistence.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-15 01:31 PDT | Codex (GPT-5) as feature-plan spike intake | Reopened the prior one-account Claude boundary for the operator's new multi-account priority, added the newly documented status-line usage contract and Provider policy gate, and froze two-platform/no-hidden-probe acceptance | this log; parent Feature Brief/design/test | `SPIKE_READY` | `provider-spike` after operator supplies/selects two test accounts and Linux target; policy evidence gathered in parallel |
| 2026-07-16 16:43 PDT | Codex (GPT-5) as provider-spike | Revalidated official policy and technical contracts, pinned current macOS/Linux CLI versions and hashes, confirmed sanitized seven-key auth status, empty-root isolation and absent credential-provider overrides, then stopped before second-account login or any quota-only request because the stable-product policy failure was decisive | spike report; sanitized JSON; `docs/PROVIDER_COMPATIBILITY.md` | `EVIDENCE_READY`; stable managed subscription login/Usage surface falsified; technical distinct-account support remains unclaimed; API-key/cloud-provider or direct official CLI fallback is deterministic | `security-review` |
| 2026-07-16 16:47 PDT | Codex root as operator-directed provider-spike writer via `mad-dashboard-sync` | Bound manual dashboard focus to the Claude Spike's exact `EVIDENCE_READY` policy verdict, preserved Codex `GATE_RESOLVED` and parent P1 `VERIFIED`, regenerated machine facts, and verified workflow/dashboard/link/diff integrity | `docs/workflow/project/dashboard-state.json`; generated dashboard unchanged | all checks PASS; the dashboard names Security Review and the API-key/cloud fallback instead of requesting a needless second subscription login | `security-review` |
