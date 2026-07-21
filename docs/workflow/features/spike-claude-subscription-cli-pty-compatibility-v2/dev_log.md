# Spike log: Claude Team-subscription PTY-only compatibility on macOS v2

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-claude-subscription-cli-pty-compatibility-v2` |
| Title | `Claude Team-subscription PTY-only compatibility on macOS v2` |
| Owner Module | `provider` |
| Impacted Modules | `security, project-system` |
| Hypothesis | `On macOS 26.5.2 arm64, exact Claude Code 2.1.207 authenticated only through the existing Claude.ai Team subscription can complete one and only one bounded interactive PTY marker request with a proven TTY path, three resizes, no print, API-key/token/cloud override, usage-credit enablement, dollar budget, tool, transcript, forbidden local-state change, raw durable capture, or retry; success is exact version/platform direct-official-CLI external evidence only and not stable MultiAgentDesk-managed Claude support or Phase 3 completion` |
| Time-box | `30 hands-on minutes after preflight; zero or one model-bearing PTY process; 75-second live deadline; no retry` |
| Current Phase | `INTAKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex (GPT-5) as feature-plan spike intake` |
| Updated | `2026-07-21 03:04 PDT` |
| Suggested Next | `provider-spike` |
| Security Gate | `open — existing subscription auth, Provider retention, local config/session state, and evidence minimization require independent review after EVIDENCE_READY` |
| Evidence Path | `docs/spikes/claude-subscription/` |
| Decision Record | `pending — after provider-spike evidence and security-review only; ADR 0016 and the prior negative compatibility decision remain unchanged during intake` |

## Success and failure criteria

- Supported when: exact host/binary/auth and no-override preflight passes; a
  local no-model fixture proves fd 0/1/2 TTY wiring and resize semantics; a
  fresh `0600` ledger is claimed before at most one model-bearing Claude PTY
  process; the positional synthetic marker returns with three exact resizes and
  bounded clean exit; workspace/protected content is unchanged; only the exact
  metadata-only state allowlist is observed; durable evidence contains no raw
  auth/prompt/response/terminal/PII/secret/path content; and the report limits
  success to exact macOS `2.1.207` direct official-CLI external evidence.
- Falsified when: the single PTY process starts but marker, TTY/resize,
  deadline/output/cleanup, no-tool/no-input, transcript, or state-diff criteria
  fail. A limit/credit/upgrade surface is Provider availability and stops with
  no retry. Blocked pre-request when host/version/digest/auth/flags/env/settings,
  TTY fixture, ledger, usage-credits-disabled direction, or safety bounds are
  ambiguous. Inconclusive when resolution would require raw retention, extra
  input, a second request, or weakened criteria. No outcome authorizes print,
  API-key/cloud substitution, usage credits, dollar spend, account switching,
  or stable product claims.

## Environment

| Field | Value |
|---|---|
| Tool + version | `Claude Code 2.1.207; prior binary SHA-256 1397a062c6889675055e3314dd956376ac51262a7734ad9e819c26975d71547a; both must be revalidated before ledger and spawn` |
| OS | `operator-selected macOS 26.5.2 arm64 only; Linux and Windows excluded` |
| Auth mode | `existing Claude.ai Team subscription included usage only; usage credits disabled by operator direction; no API key, Provider-token override, cloud credential, setup-token, login/logout, or dollar budget` |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-21 03:04 PDT | Feature-plan intake against current `origin/main@e3578390a23ddcf805ceb0bad24b1c41d36977fb`, workflow/roles, ADR 0016, the full prior Spike log/brief/report/JSON/ledger/runner/security verdict, and operator direction that the experiment is subscription-only with usage credits disabled; no Claude or Provider request executed | New PTY-only hypothesis, exact one-process ledger, no-secret preflight, real-TTY fixture, transcript/state-diff allowlist, immediate aborts, zero retry/print/API-key/cloud/dollar path, Provider-retention disclosure, and exact-version non-product conclusion frozen | `docs/reviews/spike-claude-subscription-cli-pty-compatibility-v2/2026-07-21-feature-brief.md`; `docs/workflow/features/spike-claude-subscription-cli-pty-compatibility-v2/{design.md,api.md,test.md,dev_log.md}` |

## Result, limitations, and fallback

No Provider result exists at intake and no Claude command was run. The planned
experiment is intentionally narrower than the old Spike: PTY only, one process
maximum, no print, no retry, and a content-aware state policy that permits only
metadata-only touches when content, size, mode, ownership, type, link target,
and directory membership remain identical.

The deterministic fallback before and after the experiment is direct official
Claude Code use outside MultiAgentDesk's managed surface. Even a successful
result cannot supersede ADR 0016, establish stable managed subscription auth or
traffic, prove Usage/billing/account capability, satisfy Phase 3, or support
Linux/Windows.

## Risks and Blockers

- No current blocker to the next `provider-spike` role. Execution remains
  fail-closed on environment, auth, flag, TTY, ledger, or state ambiguity.
- Claude Code or Anthropic can change version, policy, billing, cache, prompt,
  or retention behavior. Exact drift blocks or narrows the evidence.
- The official CLI and Anthropic will process the public synthetic request and
  may retain it under Team policy; local evidence minimization is not
  Provider-side zero retention.
- Same-user malware, administrator/root, Keychain, backups, crash tools, and
  uninspected paths remain residual trust surfaces.
- Any local content/session/history/settings/auth change is negative evidence;
  the executing role must not delete, repair, or retry around it.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-21 03:04 PDT | Codex (GPT-5) as feature-plan spike intake | Classified the new bounded experiment as provider-owned with security and project-system impacts; froze macOS/exact-version Team-subscription-only PTY scope, operator-confirmed usage-credits-disabled billing boundary, no dollar budget, zero print/retry/API-key/token/cloud paths, exact ledger/TTY/transcript/state-diff/evidence contracts, immediate aborts, Provider retention, and non-product claim limits after reading the full prior authority; ran structural/project/CI checks without a Claude/Provider call and retained no generated dashboard change; made no script/product/operator-dashboard/Git/remote write, compatibility decision, or self-approval | `docs/reviews/spike-claude-subscription-cli-pty-compatibility-v2/2026-07-21-feature-brief.md`; `docs/workflow/features/spike-claude-subscription-cli-pty-compatibility-v2/{design.md,api.md,test.md,dev_log.md}` | `SPIKE_READY`; decision-complete provider-spike intake with Security Gate open; workflow/project/CI/diff checks passed | `provider-spike` |
