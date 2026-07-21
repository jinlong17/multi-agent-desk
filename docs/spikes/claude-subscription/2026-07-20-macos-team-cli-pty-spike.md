# Claude Team-subscription CLI/PTY compatibility Spike on macOS

- Date: 2026-07-20 PDT
- Owner module: `provider`
- Target: `spike-claude-subscription-cli-pty-compatibility`
- Result: **strict hypothesis falsified; negative evidence ready for Security Review**

## Result

The exact macOS subscription-only print mechanism worked: Claude Code returned
the fixed synthetic marker with exit code `0` through the existing
`claude.ai` Team subscription OAuth login held by the official CLI. No API-key,
bearer-token, or cloud-credential override, dollar budget flag, tool, MCP
server, workspace write, retry, or runtime raw-output capture was used.

The strict Spike nevertheless failed. Claude Code changed metadata inside the
selected default configuration scopes during the print arm, so the required
`selectedConfigScopesMetadataUnchanged` assertion was false. The one-shot
harness therefore stopped. It did not claim the PTY ledger and did not start a
PTY process. Exactly one model-bearing print CLI invocation was started and no
retry occurred. This is an invocation/process-spawn count, not network
instrumentation; the evidence does not claim an exact Provider HTTP-request
count.

This is decisive negative evidence for the frozen hypothesis. It is not an
authentication failure and does not show an API-key or billing-source
crossover. It shows that exact Claude Code `2.1.207` cannot satisfy this
Spike's zero-config-metadata-write contract while making the tested official
Team-subscription print request.

## Frozen environment and preflight

| Field | Sanitized result |
|---|---|
| OS | macOS `26.5.2` arm64 |
| Claude Code | `2.1.207` |
| Binary SHA-256 | `1397a062c6889675055e3314dd956376ac51262a7734ad9e819c26975d71547a` |
| Auth | `loggedIn=true`, `authMethod=claude.ai`, `apiProvider=firstParty`, `subscriptionType=team` |
| Credential overrides | API-key, bearer-token/OAuth-token, gateway, Bedrock, Vertex, Foundry, Mantle, AWS and proxy selectors all absent; the official CLI's existing subscription OAuth login remained in use |
| Billing gate | Team Owner usage-credits-disabled attestation recorded before execution |
| CLI controls | exact pinned host/binary, documented flag surface, safe mode, no tools, no MCP, no slash commands, no print session persistence |
| Preflight side effects | selected config-scope metadata unchanged across initial and per-arm preflight |

The official authentication documentation distinguishes Claude for Teams
login from Console API-based billing, and the live allowlisted projection
matched the Team path. The current official subscription update also states
that `claude -p` continues to draw from subscription usage limits. Team usage
credits were externally confirmed disabled because Anthropic documents that
enabled credits can continue past included limits at standard API rates.

- [Claude Code authentication](https://code.claude.com/docs/en/authentication)
- [Current subscription use for `claude -p`](https://support.claude.com/en/articles/15036540-use-the-claude-agent-sdk-with-your-claude-plan)
- [Team usage credits](https://support.claude.com/en/articles/12005970-manage-usage-credits-for-team-and-seat-based-enterprise-plans)

## Execution

The operator attested that Team usage credits were disabled. The provider-spike
then ran exactly once:

```text
python3 -B docs/spikes/claude-subscription/run_macos_team_cli_pty_probe.py --execute --usage-credits-disabled-operator-attested
```

The harness claimed a `0600` print attempt ledger before process spawn and
revalidated the exact SHA, auth class, override absence, settings, and selected
config metadata before process spawn. It retains the deterministic synthetic
fixture definition and hash for reproduction, but no runtime prompt/response
capture, raw auth JSON, terminal content, session identifier, email,
organization identifier, or credential value.

| Arm | Model-bearing CLI invocations | Marker | Exit | Local result | Classification |
|---|---:|---|---:|---|---|
| print | 1 | matched | 0 | empty workspace unchanged; selected config metadata changed | `UNEXPECTED_LOCAL_WRITE` |
| PTY | 0 | not run | n/a | print failure stopped the harness before PTY claim/spawn | `NOT_RUN_AFTER_PRINT_FAILURE` |

The sanitized JSON records `2422 ms`, `27` retained-in-memory stdout bytes and
`206` retained-in-memory stderr bytes for limit enforcement only. Neither byte
buffer nor its text was emitted or persisted.

## Sanitized write diagnosis

The durable before/after manifest proves that aggregate metadata changed in the
selected default configuration scopes. No historical per-path diagnostic
command or output was retained, so this evidence does not attribute the drift
to a particular cache, settings, or session path. No file content was parsed,
emitted, or retained by the durable evidence. The claim does not extend to
uninspected local paths, system storage, or Provider storage.

These writes may be ordinary official CLI cache refreshes, but the intake did
not authorize any config-write allowlist. Treating them as acceptable now
would rewrite the acceptance criteria after observing the result. A future
feature-plan may separately decide whether an exact, reviewable cache-metadata
allowlist is appropriate; this Spike cannot retry or continue into PTY.

## Security and product boundary

- The successful marker supports only the narrow mechanism fact that the
  pinned official CLI completed one Team-subscription print response.
- The failed no-write assertion and absent PTY evidence prevent a positive
  compatibility claim under this Spike.
- No MultiAgentDesk-managed subscription credential, login, routing, usage,
  quota, billing, or account capability is authorized.
- ADR 0016 remains unchanged. Provider-side Team retention still applies; the
  harness controls only project evidence and the inspected local scopes.
- Linux and Windows were not tested. Phase 3 is not satisfied.
- `SIGKILL`, host power loss, same-user host compromise, Keychain behavior and
  Provider-internal processing remain residual risks.

## Deterministic fallback

Continue using the official Claude Code CLI directly outside MultiAgentDesk's
managed surface. Do not delete the attempt ledger, retry the print arm, inject
an API key/cloud credential, or run PTY under this frozen Spike. A separate
cache-write or PTY experiment requires an explicit feature-plan decision and a
new one-shot authorization boundary.

## Evidence

- `docs/spikes/claude-subscription/2026-07-20-macos-team-cli-pty.json`
- `docs/spikes/claude-subscription/2026-07-20-print-attempt-claimed.json`
- `docs/spikes/claude-subscription/run_macos_team_cli_pty_probe.py`
- `docs/workflow/features/spike-claude-subscription-cli-pty-compatibility/dev_log.md`
