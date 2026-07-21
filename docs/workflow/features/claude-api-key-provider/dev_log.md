# Development log: Claude API-key provider vertical slice

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `claude-api-key-provider` |
| Title | `Claude API-key provider vertical slice` |
| Owner Module | `provider` |
| Impacted Modules | `core`, `security`, `desktop`, `project-system` |
| Current Phase | `PLAN` |
| Status | `NEEDS_REVIEW` |
| Executor | `Codex (GPT-5) as feature-plan` |
| Updated | `2026-07-20 17:37 PDT` |
| Suggested Next | `feature-review` |
| Branch / Worktree | `codex/provider/claude-api-key-provider` / `/Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/claude-api-key-provider` |
| Plan Version | `v0.1` |
| Provider Gate | `open — spike-claude-api-key-cli-compatibility must reach Security-accepted GATE_RESOLVED before feature-build` |
| Security Gate | `open — API key/Vault/secret IPC/billing confirmation/environment/status sanitizer/PTY/platform behavior` |

## Phase Plan

The independent plan review may accept or revise this phase decomposition, but
the first executable technical unit is the prerequisite Provider Spike. No
feature-build phase is authorized while G0 remains open.

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| G0 | Run `spike-claude-api-key-cli-compatibility`; independently security-review it; record a new ADR and exact compatibility rows | dedicated test API key through non-recorded channel; explicit authorization for bounded ordinary paid requests; exact Linux/macOS targets | API-key precedence, empty-config isolation, minimal env, redacted health/error, PTY/status/metric behavior, cleanup and fallbacks frozen; Security Review accepted; decision `GATE_RESOLVED` | `SPIKE_READY` in child unit; feature gate open |
| P1 | Forward migration, Claude/api_key/billing/session-metric domain contracts, confirmation-before-secret enrollment, Vault seal/revoke | G0 resolved; plan `APPROVED` | schema-7 upgrade/restart/rollback checks; secret-safe prompt/stdin; submit replay/CAS; no secret in durable surfaces | pending |
| P2 | Binary discovery/exact compatibility, strict Profile validation, clean child-environment builder, redacted health/error/status fixtures | P1 independently `VERIFIED`; accepted G0 mechanism | unknown version/platform and auth-source conflicts fail before Vault/spawn; fixture/adversarial env/redaction tests pass | pending |
| P3 | Provider-neutral Session preview/confirmation, immutable billing/auth tuple, Account-bound health and session-metric projection | P2 independently `VERIFIED` | stale/mismatched tuple fails before materialization; no default/rotation; metric meaning/dedup/unavailable behavior passes | pending |
| P4 | Claude PTY runtime wired through existing Session/Attachment/ControllerLease/ring buffer; deterministic fake/fixture lifecycle | P3 independently `VERIFIED` | input, exact resize, observer/replay, lease, stop/kill, crash/restart cleanup and race suite pass | pending |
| P5 | Exact Linux amd64 live API-key acceptance and sanitized evidence | P4 independently `VERIFIED`; dedicated key and explicit paid-request authority | ordinary API-key PTY task, second CLI control/replay/stop, truthful health/metrics, no retained secret; exact compatibility row only | pending |
| P6 | Exact macOS arm64 live acceptance; Windows build/ConPTY regression and typed unsupported gate | P5 independently `VERIFIED`; same human/secret gates for macOS | no Keychain/subscription crossover; live macOS lifecycle passes; Windows fails before secret/spawn and is not called supported | pending |
| P7 | Full regression, threat/compatibility/user docs, rollback drill, final independent feature verification and Security Review | P6 independently `VERIFIED` | Go/race/cross-build/Web/Rust/governance checks, secret scan, rollback, no open Critical/High issue; `READY_TO_SHIP` then Security Review | pending |

Each P1-P7 build run completes exactly one approved phase, updates this state
authority and evidence, and stops at `READY_FOR_VERIFY`. An independent
`feature-verify` verdict is required before the next phase.

## Acceptance criteria

- [ ] G0 reaches Security-accepted `GATE_RESOLVED` without using the current
      Team subscription as API-key evidence and without storing a real key/raw
      Provider output in Git.
- [ ] One dedicated Console API key enters only through hidden prompt or
      `--api-key-stdin`, is sealed only in the selected Device Vault, and never
      appears in argv, Profile/settings, SQLite plaintext, generic idempotency,
      logs, audit, dashboard, fixtures, test output, or Git.
- [ ] Enrollment confirms Account/Profile/Device/auth/billing before secret
      submission and handles replay, expiry, client/revision drift, Vault lock,
      replacement conflict, cancellation, and cleanup atomically.
- [ ] An exact-version compatibility gate and clean child environment prevent
      inherited subscription OAuth, another key, custom endpoint/header,
      credential helper, Bedrock, Vertex, Foundry, or ambient cloud credential
      from silently changing identity or billing.
- [ ] Session preview/confirmation binds immutable Account/Profile/Credential/
      Device/Workspace revisions, `api_key`, Console billing, binary/contract/
      capability digests, and usage evidence before Vault access or spawn.
- [ ] A real Linux amd64 and separate real macOS arm64 exact-version Session
      each prove input, exact resize, observer replay, ControllerLease, graceful
      stop/kill, reconnect, bounded cleanup, and no subscription crossover.
- [ ] Health failures distinguish auth, permission, billing, rate, network,
      version/schema, and platform classes without raw Provider identity/body or
      automatic retry through another credential.
- [ ] Session cost/context metrics are source/version/freshness labelled,
      metric/unit explicit, Account/Credential/Session bound, replay-deduplicated
      and honestly unavailable on missing/drifted schema; they never claim
      Console bill/balance, subscription windows, monthly credit, or limits.
- [ ] Local revoke blocks future MultiAgentDesk use and removes the selected
      local Vault/materialization state without claiming remote key deletion.
- [ ] Windows cross-build and existing ConPTY mechanism tests pass, but real
      Claude preview/start remains typed unsupported until a separate live gate.
- [ ] Migration/restart/failure injection/rollback, full Go/race/cross-build,
      Web/Rust scaffold, workflow/dashboard/governance, documentation link and
      secret-scan checks pass with no unresolved Critical/High security issue.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-20 17:20 PDT | PLAN | Read `AGENTS.md`, `CLAUDE.md`, implementation plan, workflow policy, module registry, `feature-plan` role, both required Skills, Feature Brief, ADR 0016, templates, current compatibility/state and relevant code/migrations | classified sole Owner `provider`; current Claude adapter remains a placeholder; schema 7 and existing preview/enrollment contracts require a forward provider-neutral extension | repository files; no product mutation |
| 2026-07-20 17:32 PDT | PLAN | Anthropic official environment, CLI, status-line, hook and error documentation | docs establish API-key precedence and bounded CLI options; status-line cost is client-estimated and raw hook/status input carries sensitive fields; exact runtime behavior retained as Spike assumption | official URLs in `design.md` and Feature Brief |
| 2026-07-20 17:37 PDT | PLAN | `/Users/jinlong/.local/bin/claude --version`; redacted `claude auth status --json` projection; presence-only checks for API/OAuth/cloud variables | Claude Code `2.1.207`; current auth is `claude.ai` Team/first-party; all checked overrides absent; no PII/secret emitted and no Provider request run | planning terminal evidence only |
| 2026-07-20 17:37 PDT | PLAN | Created decision-complete design/API/test plan and child Spike intake | feature `NEEDS_REVIEW`; child `SPIKE_READY`; no dashboard/product/generated file edited | four feature documents plus child spike log |

## Risks and Blockers

- **Human/secret gate:** no dedicated API key is currently available. The
  operator must inject one through a non-recorded local mechanism; it must not
  be pasted into chat or repository artifacts.
- **Billing gate:** no paid request is authorized by planning or testing. The
  operator must explicitly authorize each bounded ordinary-work arm and known
  maximum/expected cost.
- **Provider gate:** documented precedence is insufficient for stable support.
  Exact macOS/Linux CLI, empty-config/Keychain behavior, minimal environment,
  PTY/status/error schema and cleanup require reproducible Spike evidence.
- **Security gate:** environment delivery is necessarily visible to the child
  and potentially same-user/root tooling. Secret IPC, Vault, billing
  confirmation, sanitizer, PTY control, local revoke and evidence redaction need
  independent review.
- **Usage risk:** client-estimated cost can differ from the bill and context
  token values are not cumulative organization usage. The accepted fallback is
  explicit unavailable state, never a fabricated value.
- **Platform risk:** passing Linux does not enable macOS; compilation/ConPTY
  does not enable Windows. Unknown tuples fail before secret materialization.
- **Migration risk:** schema 8 is forward-only. Rollback uses capability disable,
  reviewed forward fixes, or full verified backup restore; old binaries must
  refuse the schema.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-20 17:37 PDT | Codex (GPT-5) as feature-plan | Classified the API-key vertical slice as solely `provider`-owned, converted the existing Brief/ADR boundary into phased design/API/test contracts, froze secret/billing/platform/usage/rollback rules, and created the required credential-sensitive Provider Spike intake without touching product code or dashboard state | `docs/workflow/features/claude-api-key-provider/{design.md,api.md,test.md,dev_log.md}`; `docs/workflow/features/spike-claude-api-key-cli-compatibility/dev_log.md`; no commit | feature `NEEDS_REVIEW`; Provider/Security Gates open; first executable technical work is the child `provider-spike` and no feature-build is authorized | independent `feature-review`; execute `provider-spike` for child before any build |
