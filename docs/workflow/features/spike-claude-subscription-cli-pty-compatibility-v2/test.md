# Test plan: Claude Team-subscription PTY-only compatibility v2

No Claude or Provider command is executed by feature-plan. Commands in this
document are acceptance instructions for the later `provider-spike` role.

## Static intake checks

| Check | Expected |
|---|---|
| Module classification | owner `provider`; impacts `security, project-system` |
| Workflow state | `SPIKE_READY`; Suggested Next `provider-spike`; Security Gate `open` |
| Scope scan | PTY-only, macOS-only, exact CLI; no print execution, API key, token override, cloud, Linux/Windows, dollar budget, retry, product claim |
| Old evidence references | old log/report/JSON/ledger/runner are read-only authority and never execution targets |
| Git diff | only new v2 planning/intake files before handoff |

Required intake verification (the project verifier may create the ignored
generated dashboard snapshot transiently; feature-plan removes it after the
check and retains no dashboard change):

```text
npm run workflow:verify
npm run project:verify
npm run ci:verify
git diff --check
git diff --no-index --check /dev/null <each-new-intake-file>
```

The dashboard commands refresh generated machine facts only; feature-plan must
not edit operator-owned `dashboard-state.json`.

## Provider-spike preflight tests (zero model requests)

1. Host/binary pin test: exact Darwin/macOS/arm64/version/digest/inode tuple.
2. Environment denial table: every auth/token/cloud/gateway/proxy variable
   independently causes `BLOCKED_PRE_REQUEST` and no ledger.
3. Settings denial table: credential key or parse failure causes
   `BLOCKED_PRE_REQUEST`; no raw settings/auth output is emitted.
4. Auth projection table: only exact logged-in Claude.ai/firstParty/Team passes;
   every mismatch or unknown field schema blocks.
5. Flag/argv test: PTY argv contains all frozen safety flags and positional
   prompt; print/resume/continue/session reuse flags are absent.
6. TTY fixture test: fd 0/1/2 are TTYs on one slave; exact three resize values
   are observed; fixture performs no network or Claude call.
7. State classifier fixtures:
   - identical content plus mtime-only touch under an existing allowlisted root
     is accepted;
   - content, size, mode, owner, type, target, entry-set, or protected-path
     change is rejected;
   - new cache root, symlink, FIFO, socket, device, transcript/history/session,
     settings/auth/credential, workspace, or unrelated repo change is rejected;
   - durable output includes only category/count/bound booleans.
8. Ledger tests: exclusive create, `0600`, fsyncs, existing-ledger refusal, and
   claim-before-spawn ordering.
9. Sanitizer fixtures: representative auth JSON, PII, ANSI output, marker,
   billing, auth, trust, and tool prompts reduce to allowlisted classifications;
   secret/PII/raw-capture scans remain empty.
10. Timeout/output/process cleanup fixtures: dedicated process group terminates
    under bounded escalation without orphan; no retry path exists.

All fixture tests must pass before the attempt ledger can be claimed.

## One-shot live matrix

Exactly one row is executed once. No print row exists.

| Arm | Max process spawns | Deadline | Output cap | Input | Expected observation |
|---|---:|---:|---:|---|---|
| Claude PTY | 1 | 75 s | 256 KiB | one public positional marker fixture; EOF only after marker | TTY transport, three resizes, exact marker, bounded exit, state policy |

Immediate safe stops: limit/credit/upgrade, auth/login selection, trust prompt,
tool/permission prompt, browser launch, output cap, timeout, process-tree
ambiguity, sanitizer failure, or protected-state drift. None authorize retry.

## Result assertions

### `SUPPORTED_EXACT_PTY`

- one ledger, one model-bearing process, zero print, zero retry;
- exact pinned host/binary/auth and no override;
- local TTY fixture passed and all three resizes occurred;
- marker matched and bounded exit/cleanup accepted;
- workspace/protected state unchanged and all observed metadata touches meet
  the exact allowlist;
- durable report/JSON contain only the evidence contract fields;
- wording explicitly denies stable managed support and Phase 3 completion.

### Safe negative or inconclusive

- outcome maps to the frozen taxonomy in `design.md`;
- process and ledger counts are truthful;
- included-usage limit is not described as auth failure;
- forbidden writes are not deleted or repaired;
- no retry, credential substitution, usage-credit enablement, or weaker
  post-hoc criterion occurs;
- fallback remains direct official CLI outside MultiAgentDesk.

## Security review gate

From `EVIDENCE_READY`, only `security-review` may write while the gate is open.
The reviewer must verify ledger ordering/mode, exact argv/env assertions, TTY
evidence, transcript/state-diff classifications, secret/PII scans, Provider
retention wording, no retry, and the exact-version/non-product conclusion.
