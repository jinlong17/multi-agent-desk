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
| Current Phase | `DECISION` |
| Status | `BLOCKED` |
| Executor | `Codex (GPT-5) as feature-plan decision` |
| Updated | `2026-07-21 09:50 PDT` |
| Suggested Next | `none for this Spike — preserve the exhausted ledger and terminal decision; only an explicit operator-authorized new Spike with a new slug, changed trust-input/criteria contract, fresh ledger, and open Security Gate may investigate further` |
| Security Gate | `open and unresolved — INCONCLUSIVE returned directly to feature-plan and did not enter security-review; this exhausted Spike is terminal BLOCKED, so the gate is neither reviewed nor resolved` |
| Evidence Path | `docs/spikes/claude-subscription/` |
| Decision Record | `BLOCKED — the fresh one-shot ledger was consumed by one PTY process that safely stopped at a trust-confirmation class before marker with ambiguous cleanup-path classification; exact macOS 26.5.2 / Claude Code 2.1.207 PTY compatibility remains unproven, ADR 0016 and PROVIDER_COMPATIBILITY.md remain unchanged, and no managed subscription or Phase 3 claim is authorized` |

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
| 2026-07-21 09:35 PDT | New v2 runner zero-model fixtures/preflight and the sole authorized PTY execution after the fresh ledger was atomically claimed | `13/13` zero-model fixtures and exact host/binary/Team-auth/env/settings/flag/state preflight passed; one PTY process performed three resizes, then encountered a trust-confirmation class before marker; runner sent no response, safely stopped without retry, retained no raw content, and observed no workspace/protected/repository change during the request | `docs/spikes/claude-subscription/{run_macos_team_cli_pty_probe_v2.py,2026-07-21-pty-v2-attempt-claimed.json,2026-07-21-macos-team-cli-pty-v2.json,2026-07-21-macos-team-cli-pty-v2-spike.md}` |
| 2026-07-21 09:43 PDT | Post-result scoped verification only; no Claude/Provider command | Runner AST and 13 zero-model fixtures, sanitized JSON contract, `0600` ledger, workflow/project/dashboard/CI script equivalents, link/license/action/CODEOWNERS gate fixtures, diff checks, exact file-scope check, old-evidence no-diff check, and secret/PII/raw-path/raw-capture scans passed; ignored generated dashboard snapshot was not retained as a Git change | v2 runner/report/JSON/ledger; `node scripts/{workflow,dashboard,ci}/...`; `git diff --check`; secret/PII scans |
| 2026-07-21 09:47 PDT | Feature-plan decision from the complete sanitized report/JSON, fresh `0600` ledger, runner source, authoritative log, workflow edge, ADR 0016, and current compatibility matrix; no Claude/Provider command, retry, raw-content inspection, new ledger, security review, or re-scope performed | `BLOCKED`; the only authorized ledger/process opportunity is exhausted and the observed trust-confirmation plus cleanup-path ambiguity cannot be resolved without changed input/criteria or another request; exact PTY support remains unproven and no ADR/compatibility support row changes | this log; v2 sanitized report/JSON/ledger/runner; `docs/adr/0016-claude-profile-interactive-login-boundary.md`; unchanged `docs/PROVIDER_COMPATIBILITY.md` |
| 2026-07-21 09:50 PDT | Post-decision verification only; no Claude/Provider command or runner execution | `project:verify` and `ci:verify` passed, including workflow/dashboard, Actions, CODEOWNERS, gate fixtures, 302 local Markdown files, and licenses; JSON parsing, `0600` ledger mode, tracked/untracked diff checks, one decision Work Log row, protected old evidence, ADR 0016, and compatibility-matrix no-diff checks passed; ignored generated dashboard snapshot was removed | repository verification commands; this log; unchanged v2 evidence and protected authorities |

## Result, limitations, and fallback

Feature-plan records the terminal decision as `BLOCKED`. The
`INCONCLUSIVE_SAFE_STOP` evidence cannot legally re-enter `SPIKE_READY` under
the same frozen scope: its sole `0600` ledger and model-bearing process
opportunity are consumed, while resolving the trust-confirmation surface and
cleanup ambiguity would require extra input, changed criteria, or another
request. None is authorized in this Spike. It does not proceed to
security-review because the workflow routes `INCONCLUSIVE` directly to
feature-plan.

The result is `INCONCLUSIVE_SAFE_STOP`. All zero-model fixtures and exact
preflight gates passed. The fresh `0600` ledger was claimed before exactly one
model-bearing PTY process; zero print processes and zero retries occurred. The
PTY performed the three frozen resizes, then exposed a trust-confirmation class
before the marker. No input or confirmation was sent. The marker was not
observed, cleanup-path classification remained ambiguous although the final
process group was clear, and workspace/protected/repository state remained
unchanged during the request.

The deterministic fallback remains direct official Claude Code use outside
MultiAgentDesk's managed surface. This result cannot supersede ADR 0016,
establish stable PTY or managed subscription support, prove Usage/billing/
account capability, satisfy Phase 3, or support Linux/Windows. Resolving the
trust surface would require extra input, changed policy, or another request;
all are forbidden in this exhausted one-shot lifecycle.

ADR 0016 and the existing compatibility matrix remain unchanged because this
is a terminal blocked decision, not `GATE_RESOLVED` compatibility evidence.
The subscription-only boundary also remains unchanged: Team included usage,
usage credits disabled, no API key/token/cloud override, and no dollar budget.

## Risks and Blockers

- The one-shot ledger is exhausted and this frozen Spike is terminal
  `BLOCKED`. The condition cannot be cleared inside this slug. Only explicit
  operator authorization can start a genuinely new feature-plan Spike intake
  with a new slug, changed trust-input/criteria contract, fresh ledger, and
  open Security Gate; that would be a new unit, not a retry or clearing action
  on this evidence.
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
| 2026-07-21 09:35 PDT | Codex (GPT-5) as provider-spike | Implemented and passed the v2 zero-model environment/settings/auth/argv/TTY/resize/state/ledger/sanitizer/timeout/output/process fixtures; revalidated exact macOS/CLI digest/Team auth and unchanged protected state; atomically claimed the fresh `0600` ledger; ran exactly one PTY process with zero print/retry/API-key/token/cloud/dollar path; stopped without input when a trust-confirmation class appeared; retained only sanitized classifications and counts; made no product/dashboard/policy/compatibility/Git/remote change | v2 runner; fresh ledger; sanitized JSON/report; this log | `INCONCLUSIVE`; trust-class safe stop before marker, three resizes, final process group clear, no forbidden local-state difference, no raw capture, Provider HTTP count unobserved, and no stable support or Phase 3 claim | `feature-plan decision` |
| 2026-07-21 09:50 PDT | Codex (GPT-5) as feature-plan decision | Applied the legal `INCONCLUSIVE -> feature-plan -> BLOCKED` edge after reading the complete sanitized report/JSON, exhausted `0600` ledger, runner, authority log, ADR 0016, compatibility matrix, and workflow; preserved all evidence and the Team included-usage/usage-credits-disabled/no-API-key-or-token-or-cloud/no-dollar boundary; completed project/CI/link/license/JSON/ledger/diff/protected-authority checks and retained no generated dashboard snapshot; made no Claude/Provider call, raw-content inspection, retry, new ledger, re-scope, security-review routing, product/dashboard-state/runner/evidence/ADR/compatibility/Git/remote change | this log | `BLOCKED`; exact macOS `26.5.2` / Claude Code `2.1.207` PTY compatibility and Phase 3/managed subscription support remain unproven; direct official CLI remains the fallback; checks passed | `none for this Spike; a future investigation requires explicit operator authorization for a new Spike lifecycle` |
