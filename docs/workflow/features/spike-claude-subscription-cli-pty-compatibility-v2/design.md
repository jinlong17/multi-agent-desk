# Design: Claude Team-subscription PTY-only compatibility v2

- Plan version: `0.1`
- Workflow: `SPIKE`
- Owner: `provider`
- Security Gate: `open`
- Execution target: exact Claude Code `2.1.207`, macOS `26.5.2` arm64

## Decision to test

Falsifiable hypothesis:

> On the pinned macOS host, one and only one official Claude Code interactive
> PTY process using the existing Claude.ai Team subscription can receive a
> public positional marker request, expose real TTY transport, survive three
> exact resizes, return the marker, and exit within 75 seconds without tools,
> credential/billing overrides, transcript persistence, forbidden local-state
> changes, raw durable capture, or retry.

`SUPPORTED_EXACT_PTY` requires every conjunct. Any failure remains useful safe
negative or inconclusive evidence. Success remains exact external mechanism
evidence and does not establish stable MultiAgentDesk-managed Claude support.

## Prior evidence boundary

The 2026-07-20 Spike is immutable and must not be retried. Its print ledger,
runner, report, JSON, compatibility row, and security verdict remain historical
authority. v2 uses a new slug, new ledger, new runner/evidence names, and a
PTY-only hypothesis. The v2 provider-spike must never execute print mode.

## Time and request box

- Hands-on time-box: `30 minutes` after preflight is ready.
- Model-bearing process budget: `0 or 1` Claude CLI process spawn.
- Print budget: `0`.
- Retry budget: `0` for every result, timeout, crash, limit, or ambiguity.
- PTY deadline: `75 seconds` from spawn through cleanup.
- Observed PTY byte cap: `256 KiB`; rolling detection buffer: `64 KiB`.
- Resizes: exactly `(30,100)`, `(40,120)`, `(24,80)` after spawn while no
  abort condition is present.
- Provider HTTP request count is not inferred from process count.
- Dollar budget: not applicable. Existing included Team subscription usage and
  usage-credits-disabled are the only billing boundary.

## Preflight state machine

All stages before `LEDGER_CLAIMED` are non-model checks. A failure ends as
`BLOCKED_PRE_REQUEST` and creates no attempt ledger.

1. `HOST_PINNED`
   - `Darwin`, macOS `26.5.2`, `arm64`.
   - Resolved executable is a regular executable file owned by the current
     user/root, not a symlink that changed after resolution.
   - Version exactly `2.1.207`; SHA-256 exactly the accepted prior digest.
   - The resolved inode/device/digest tuple is held and rechecked before spawn.
2. `ENV_CLEAN`
   - Abort if the parent environment contains a non-empty API-key, bearer/OAuth
     token override, base URL, custom headers, model override, setup-token,
     Bedrock/Vertex/Foundry/Mantle/AWS selector, credential helper, gateway, or
     upper/lower-case HTTP(S)/ALL proxy variable.
   - Build the child environment from an allowlist only: `HOME`, `PATH`,
     `TMPDIR`, locale, `SHELL`, `USER`, `LOGNAME`, macOS text-encoding variable,
     plus the frozen safe controls. Never pass `CLAUDE_CONFIG_DIR` or a secret.
   - Safe controls: safe mode, skip prompt history, disable nonessential
     traffic/telemetry/error reporting/feedback, subprocess env scrub, no
     color, and `TERM=xterm-256color`.
3. `SETTINGS_CLEAN`
   - Inspect only known JSON settings paths for credential-bearing key names;
     parse in memory and persist only counts/booleans.
   - Any parse failure, credential key, settings helper, cloud selector,
     gateway, or proxy selector blocks the request.
   - Do not inspect or copy Keychain contents.
4. `AUTH_CLASS_PINNED`
   - Run only the non-model official `auth status --json` surface.
   - Reduce in memory to exactly `loggedIn=true`, `authMethod=claude.ai`,
     `apiProvider=firstParty`, `subscriptionType=team`.
   - Discard all other fields and raw bytes. Any mismatch or schema drift blocks.
5. `FLAGS_PINNED`
   - Exact help surface must contain the flags used by the PTY command:
     safe mode, no Chrome, disabled slash commands, strict MCP, empty tools,
     disallowed MCP tools, `dontAsk`, model, system prompt, screen-reader mode,
     and positional prompt support.
   - `--print`, continuation, resume, session-id, fork, and session reuse must
     not appear in the actual PTY argv.
6. `TTY_FIXTURE_PASSED`
   - Use a local no-network/no-Claude fixture through the same PTY-open,
     fork/spawn, standard-stream wiring, resize, process-group, and cleanup path.
   - The fixture must attest only booleans that fd 0, 1, and 2 are TTYs and that
     the three window sizes are observed. Persist no raw fixture terminal bytes.
7. `STATE_BASELINE_FROZEN`
   - Capture the state classes below. Per-file raw paths may be used transiently
     for policy evaluation; durable evidence records only category/count/bounds.
8. `OPERATOR_BOUNDARY_RECORDED`
   - Record the existing explicit direction that the Claude.ai Team
     subscription is used and usage credits are disabled. Do not request a
     dollar cap or alternative credential.
9. `LEDGER_CLAIMED`
   - Atomically create the fresh v2 PTY ledger with `O_CREAT|O_EXCL`,
     `O_NOFOLLOW` where available, mode `0600`, file fsync, and parent-directory
     fsync. Existing ledger means no spawn.
10. `PRESPAWN_REVALIDATED`
    - Recheck host/binary tuple, child-env exactness, settings/auth projection,
      flag surface, TTY fixture contract, and unchanged protected state. Any
      failure after ledger claim stops with zero model process and no retry.

## Frozen PTY command contract

The v2 provider-spike may choose a new runner implementation, but the spawned
Claude argv must be semantically equivalent to:

```text
<pinned-claude>
  --safe-mode
  --no-chrome
  --disable-slash-commands
  --strict-mcp-config
  --tools ""
  --disallowedTools "mcp__*"
  --permission-mode dontAsk
  --model haiku
  --system-prompt <public no-tools marker-only instruction>
  --ax-screen-reader
  <public positional marker fixture>
```

The public fixture joins four fixed non-secret tokens and expects one exact
ASCII marker. The prompt is delivered in argv to avoid typing into an unknown
menu. The argv is not persisted as a runtime capture; the deterministic public
fixture definition and SHA-256 may be retained. After marker detection, send a
single EOF. If it does not exit, escalate once through `SIGINT`, `SIGTERM`, then
`SIGKILL` within the total deadline. Cleanup class is evidence, not retry
authorization.

## TTY and transcript controls

- The child PTY slave is stdin/stdout/stderr; the parent owns one master reader.
- No keyboard input, approval response, trust confirmation, slash command, or
  tool response is sent. Only the post-marker EOF is allowed.
- PTY bytes stay in bounded memory. They may be normalized only for exact
  marker matching and the frozen abort-pattern classes. Durable evidence keeps
  byte count, marker boolean, abort class, exit/timeout/cleanup class, and resize
  count, never the bytes or text.
- `CLAUDE_CODE_SKIP_PROMPT_HISTORY=1` is mandatory. No resume/continue/session
  identifier is used. A new session/history/transcript artifact is forbidden.
- The runner must not claim zero Provider-side retention. Anthropic and the
  official CLI remain processing/retention surfaces.

## Local-state diff policy

The state comparison is intentionally stricter for content than the old
aggregate metadata test. It may hash local files in a streaming comparison but
must never persist raw content or per-sensitive-file digests.

### Allowed writes

Only these repository artifacts may be created/changed by the provider-spike:

- the new v2 runner owned by that role;
- `2026-07-21-pty-v2-attempt-claimed.json` (fresh `0600` ledger);
- the new sanitized v2 report and JSON;
- the v2 `dev_log.md` transition and later security-review report/log update.

During the Claude process, the only non-evidence local difference allowed is a
metadata-only touch on a pre-existing regular file or directory under one of:

- `~/.claude/cache/`
- `~/Library/Caches/Claude Code/`
- `~/Library/Caches/com.anthropic.claudecode/`
- the pre-existing `~/.claude.json` file

`metadata-only` is exact: entry set, path type, symlink target, byte size,
streaming content SHA-256, mode, owner, and group are unchanged; only mtime or
ctime may differ. A root that did not exist before the request may not be
created under this allowance. No cache file content create/modify/delete is
allowed.

### Always-forbidden differences

- Any workspace entry or content change.
- Any create/delete/content/mode/owner/link-target change under `~/.claude`,
  `~/.claude.json`, or the listed Library cache roots.
- Any difference in settings, settings-local, projects, history, sessions,
  transcripts, todos/plans, shell snapshots, hooks, MCP, plugins, auth,
  credentials, tokens, logs, or crash files, regardless of location inside a
  nominal cache root.
- Any repository change outside the new v2 evidence files authorized to the
  executing/reviewing roles.
- Any permission broadening, symlink, device, FIFO, socket, or ownership change.

If a forbidden difference is observed, retain only its category/count/bound and
classify `LOCAL_STATE_POLICY_FAIL`. Do not delete, repair, inspect content, or
retry. The report must say that local side effects were observed without
claiming per-path content or Provider cause beyond the evidence.

## Immediate abort conditions

Before spawn: wrong host/version/digest; executable race; existing ledger;
missing flag; auth mismatch; raw JSON parse failure; credential/token/cloud/
gateway/proxy/settings override; usage-credits-disabled direction unavailable;
TTY fixture failure; protected-state drift; or inability to enforce bounds.

After spawn: billing/limit/upgrade/extra-usage choice; login/auth selection;
trust prompt; tool/permission prompt; browser launch; marker mismatch; output
cap; deadline; unexpected child/process-group behavior; forbidden local-state
difference; or evidence sanitizer failure. Terminate and do not retry.

## Outcome taxonomy

- `SUPPORTED_EXACT_PTY`: exact marker, three resizes, TTY fixture pass, bounded
  clean exit, only allowlisted metadata touches, and every safety assertion.
- `PROVIDER_LIMIT_NO_RETRY`: included-usage/session/rate limit or extra-usage
  surface after the one spawn. This is not auth failure.
- `LOCAL_STATE_POLICY_FAIL`: marker may or may not arrive, but forbidden local
  state changed.
- `PTY_NEGATIVE`: process/marker/resize/cleanup/timeout/output bound failed
  without ambiguity.
- `BLOCKED_PRE_REQUEST`: no model process spawned because a preflight or
  revalidation gate failed.
- `INCONCLUSIVE_SAFE_STOP`: a bounded observation cannot distinguish the
  outcome without raw retention, extra input, a second request, or weakened
  policy. Stop safely.

All outcomes state: macOS-only, exact-version-only, direct official CLI,
subscription-only, no stable managed support, no Phase 3 completion, no
Linux/Windows claim, and no Provider HTTP-request-count claim.

## Rollback and fallback

There is no execution rollback: evidence and the attempt ledger are append-only
history. Do not delete side effects or reset user state. The deterministic
fallback for every non-success, and the product fallback even after success, is
direct use of official Claude Code outside MultiAgentDesk's managed surface.
Any further request needs a new plan, new ledger, and new Security Gate.
