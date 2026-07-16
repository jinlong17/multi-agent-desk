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
| Current Phase | `INTAKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex (GPT-5) as feature-plan spike intake` |
| Updated | `2026-07-15 01:31 PDT` |
| Suggested Next | `provider-spike` |
| Security Gate | `open — distinct OAuth identities, macOS Keychain slots, auth/usage PII, PTY/status-line events, policy and logout are in scope` |
| Evidence Path | `docs/spikes/claude-distinct-accounts/` |
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

## Result, limitations, and fallback

Not run. Until accepted evidence exists, stable MultiAgentDesk must retain ADR
0016's target-local interactive-login boundary and must not claim distinct
Claude identities, setup-token CredentialGrant, official monthly remaining
credit or a stable subscription rate-limit dashboard. P1 may implement generic
metadata/Usage contracts. Missing windows display unknown/unavailable. If the
policy gate fails, Claude subscription login/usage stays outside the stable
product; direct official CLI use and API-key/provider integrations remain the
documented alternatives.

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
