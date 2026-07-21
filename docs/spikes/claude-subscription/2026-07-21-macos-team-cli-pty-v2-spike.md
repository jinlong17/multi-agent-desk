# Claude Team-subscription PTY-only compatibility Spike v2

- Date: 2026-07-21 PDT
- Owner module: `provider`
- Target: `spike-claude-subscription-cli-pty-compatibility-v2`
- Result: **INCONCLUSIVE_SAFE_STOP; no retry permitted**

## Result

The one authorized PTY process reached a trust-confirmation class before the
public marker appeared. The runner sent no keyboard input, trust response,
approval, tool response, login choice, or billing choice. It terminated the
process group and did not retry. The final process-group check was clear, but
the bounded cleanup path itself remained classified `ambiguous`; that
ambiguity independently prevents a positive PTY result.

The exact pinned environment and every zero-model gate passed before the
attempt ledger was claimed. The live process exercised all three frozen
resizes, stayed below the time and output limits, and produced no forbidden
workspace, protected-state, or repository change during the request. No raw
authentication JSON, terminal bytes/text, prompt/response capture, PII,
secret, session identifier, raw local path, file content, or per-sensitive-file
digest was retained.

This result is neither an authentication failure nor a Provider-usage-limit
result. It cannot distinguish whether a future direct invocation could pass
without answering the trust surface, because doing so would require extra
input, a changed policy, or another request. All three are forbidden under the
frozen one-shot contract.

## Frozen environment and zero-model gates

| Field | Sanitized result |
|---|---|
| OS | macOS `26.5.2` arm64 |
| Claude Code | exact `2.1.207`; pinned SHA-256 matched before ledger and immediately before spawn |
| Auth projection | `loggedIn=true`, `authMethod=claude.ai`, `apiProvider=firstParty`, `subscriptionType=team` |
| Billing boundary | existing Claude.ai Team included subscription; usage credits disabled by operator direction; no dollar budget |
| Overrides | API key, auth token, setup token, cloud, model, base URL, custom header, gateway, credential helper, and proxy overrides absent |
| Zero-model fixtures | `13/13` passed, including fd 0/1/2 on one TTY slave, three resize observations, state classifier, ledger atomicity/mode, sanitizer, timeout/output cap, and process-group cleanup |
| Preflight state | protected state and repository unchanged; no raw content retained |

## One-shot execution

The only model-bearing command was launched by the new v2 runner after its
fresh ledger was created atomically:

```text
python3 -B docs/spikes/claude-subscription/run_macos_team_cli_pty_probe_v2.py --execute --usage-credits-disabled-operator-directed
```

| Observation | Sanitized result |
|---|---|
| Attempt ledger | claimed before spawn; mode `0600` |
| Model-bearing Claude processes | `1` |
| Print processes | `0` |
| Retries | `0` |
| Provider HTTP request count | unobserved / `null` |
| Prompt delivery | one public positional fixture |
| Resizes | `3` |
| Marker | not observed |
| Abort classification | `trust` |
| Duration | `5185 ms` |
| Observed byte count | `796`, retained only as a count |
| Deadline / output cap | neither exceeded |
| Cleanup | bounded cleanup class `ambiguous`; final process group clear |
| Local state | workspace, protected state, and repository unchanged during the request; zero allowed or forbidden touches |

## Claim boundary

- The evidence is macOS-only and exact-version-only.
- It covers direct use of the official Claude Code CLI with the existing Team
  subscription, not a MultiAgentDesk-managed subscription surface.
- It proves the exact host, binary, auth-class, TTY fixture, three live resizes,
  safe abort, cleanup end state, and local no-write classification only.
- It does not prove a successful marker response, stable PTY compatibility,
  reconnect/replay/stop semantics, long-session reliability, Usage, billing,
  account capability, Provider HTTP request count, Linux, Windows, or Phase 3.
- Anthropic and the official CLI may process or retain the public synthetic
  request under the Team subscription policy; local minimization is not
  Provider-side zero retention.

## Deterministic fallback

Continue using the official Claude Code CLI directly outside MultiAgentDesk's
managed surface. Preserve the ledger and evidence. Do not retry, delete or
repair observed state, answer the trust surface under this Spike, enable usage
credits, add a dollar budget, or substitute an API key/cloud credential. Any
future experiment requires a new feature-plan scope, a fresh ledger, and the
required Security Gate.

## Verification

After the one-shot result, no Claude or Provider command was run again.
Read-only/scoped verification passed:

```text
python3 -B <v2-runner> --fixtures
python3 -B -c <AST, JSON schema, ledger mode, and sanitized assertion checks>
node scripts/workflow/verify-workflow.mjs
node scripts/dashboard/generate-state.mjs
node scripts/dashboard/verify-static.mjs
node scripts/ci/verify-actions.mjs
node scripts/ci/verify-codeowners.mjs
node scripts/ci/test-gates.mjs
node scripts/ci/check-local-links.mjs
node scripts/ci/verify-licenses.mjs
git diff --check
git diff --no-index --check /dev/null <each new v2 artifact>
```

The dashboard command refreshed only the ignored generated snapshot. Git status
contains exactly the v2 runner, ledger, sanitized JSON/report, and target log
update. Scans for credential prefixes, bearer values, email-shaped PII, local
absolute paths, raw auth fields, raw terminal fields, and session/conversation
identifiers found no retained value. The old runner, old ledger, old evidence,
and old security verdict have no diff.

## Evidence

- `docs/spikes/claude-subscription/run_macos_team_cli_pty_probe_v2.py`
- `docs/spikes/claude-subscription/2026-07-21-pty-v2-attempt-claimed.json`
- `docs/spikes/claude-subscription/2026-07-21-macos-team-cli-pty-v2.json`
- `docs/workflow/features/spike-claude-subscription-cli-pty-compatibility-v2/dev_log.md`
