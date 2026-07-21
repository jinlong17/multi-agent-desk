# Claude Console API-key compatibility spike: no-key arm

- Date: 2026-07-20 PDT
- Role: `provider-spike`
- Target: `spike-claude-api-key-cli-compatibility`
- Owner: `provider`
- Result: `BLOCKED` after completing every authorized no-key arm

## Verdict

The no-key experiment is reproducible but cannot resolve the Provider or
Security Gate. Exact Claude Code `2.1.207` on the macOS arm64 planning host
exposes `--bare`, print/JSON, budget, and no-session-persistence controls. In a
disposable empty environment, a deliberately invalid sentinel API key failed
as authentication, did not fall through to the existing Team subscription,
and did not echo or persist the sentinel or an email-like value.

This is negative/fail-closed evidence only. No valid key was available, no
ordinary billed request or PTY session ran, and no Linux target was supplied.
The planned stable API-key capability therefore remains unsupported and must
not advance to implementation.

## Official contract review

Reviewed current official documentation on 2026-07-20:

- [Environment variables](https://code.claude.com/docs/en/env-vars) documents
  API-key/environment selection and the relevant Claude Code controls.
- [CLI reference](https://code.claude.com/docs/en/cli-usage) documents print,
  structured output, budget, and session-persistence options.
- [Status line](https://code.claude.com/docs/en/statusline) describes local
  status data, including client-estimated cost semantics that cannot be
  represented as a Console bill or remaining balance.
- [Models, usage, and limits](https://support.claude.com/en/articles/14552983-models-usage-and-limits-in-claude-code)
  distinguishes API-key pay-as-you-go usage from subscription use.

Documentation narrows the experiment but is not runtime compatibility proof.

## Sanitized environment and CLI evidence

- Host: Darwin `26.5.2`, arm64.
- Binary: Claude Code `2.1.207`, Mach-O arm64, SHA-256
  `1397a062c6889675055e3314dd956376ac51262a7734ad9e819c26975d71547a`.
- Auth projection retained only `loggedIn=true`, `authMethod=claude.ai`,
  `apiProvider=firstParty`, and `subscriptionType=team`. Email and organization
  fields were neither selected nor emitted.
- Presence-only checks found no API-key/auth-token, endpoint/header, Bedrock,
  Vertex, Foundry, AWS, Google, or config-root override in the parent process.
- Exact `--help` surface contains print, output format, max budget,
  no-session-persistence, and bare mode. It does not contain `--max-turns`, so
  the plan must not require that flag for this exact version.

## Invalid-sentinel experiment

The child ran under `env -i` with a disposable `HOME` and
`CLAUDE_CONFIG_DIR`, the exact binary, `CLAUDE_CODE_SIMPLE=1`, `--bare`, print
JSON, a USD `0.01` maximum budget, and no session persistence. The API-key
value was an unmistakably invalid sentinel supplied only to the child
environment. The prompt and raw error were not retained.

The streaming sanitizer recorded only:

- Claude exit `1` and sanitizer exit `0`;
- `752` output bytes classified as authentication failure;
- no billing, rate-limit, or network class;
- no sentinel echo and no email-like value;
- two files / `28,970` bytes under the disposable root, with neither sentinel
  nor email-like persistence.

No successful Provider response or ordinary billed request occurred. After
the content-only scan, the disposable root was removed from the active temp
location by a recoverable move to Trash; it contained no real credential.

## Reproduction contract

Reproduce only with a new disposable root. Retain the JSON booleans/classes,
not raw child output, environment values, prompts, responses, identities,
request IDs, paths, or a real key. A valid-key arm must be a separate
operator-authorized run with the secret injected outside chat, shell history,
argv, Git, and recorded tool output.

## Missing decisive arms

The following prerequisites are external and were not inferred from the broad
repository/merge authorization:

1. a dedicated Claude Console test API key delivered through a non-recorded
   local mechanism;
2. an operator-selected Linux amd64 host with the exact binary pinned;
3. explicit per-request authorization and a stated bound for an ordinary paid
   print request and, separately, an interactive PTY request.

Until all three are supplied, the deterministic fallback is direct official
Claude Code outside MultiAgentDesk or no Claude managed Session. The current
Team subscription, another ambient key, cloud credentials, setup-token,
gateway, or hidden quota probe must never be used as fallback.

## Handoff

**Target**: `spike-claude-api-key-cli-compatibility`
**Completed**: `provider-spike`
**Status**: `BLOCKED`
**Result**: `The no-key arm supports a fail-closed invalid-key mechanism on exact macOS Claude Code 2.1.207, but the two-platform API-key, billing, PTY, usage, and cleanup hypothesis is unresolved and no product support claim is authorized.`
**Evidence**: `docs/spikes/claude-api-key/2026-07-20-no-key-probe.json; exact version/digest/help, redacted auth projection, presence-only environment checks, and isolated invalid-sentinel command`
**Fallback**: `No managed Claude Session; use direct official Claude Code or later resume this Spike with a dedicated Console key, exact Linux target, and explicit bounded billing authorization.`
**Blockers**: `Dedicated non-recorded test API key, operator-selected Linux amd64 target, and explicit authorization/bound for each ordinary billed request.`

### Next Step

`Clear the three external gates, then resume provider-spike for spike-claude-api-key-cli-compatibility; do not run security-review or feature-build from partial evidence.`
