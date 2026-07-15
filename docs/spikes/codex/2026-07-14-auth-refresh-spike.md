# Codex auth, usage, and concurrent refresh Spike

Status: **conclusive**. The operator cancelled the 48-hour requirement after a
sanitized three-hour short run. The evidence is sufficient to reject a
production multi-writer claim and select the canonical single-writer fallback.

## Scope and decision rule

The original composite hypothesis is supported only if schema discovery, account usage,
file-backed credentials, headless device authentication, and same-account
two-device refresh all reproduce. Any failed sub-claim, or any design that
depends on undocumented concurrent-writer behavior, falsifies the composite
hypothesis and selects the single-writer CAS fallback.

The 48-hour criterion was removed by explicit operator direction on
`2026-07-14`. This does not convert the short run into a passing long-duration
result. Instead, the original composite hypothesis is treated as unsupported,
and the predeclared conservative fallback becomes the decision.

This Spike records no email address, account/workspace identifier, token,
authorization code, usage value, rate-limit value, auth-file content, or
auth-file digest. Account equality and file changes are reduced to booleans in
memory before evidence is written.

## Official contract checked

- [Codex authentication](https://developers.openai.com/codex/auth/) documents
  ChatGPT/API-key login, automatic ChatGPT token refresh, `file | keyring |
  auto` credential storage, `auth.json` under `CODEX_HOME`, and beta
  `codex login --device-auth` for headless systems.
- [Codex app-server](https://developers.openai.com/codex/app-server/) documents
  stdio JSONL, the initialize handshake, version-specific schema generation,
  and the rich-client integration boundary.
- The generated `GetAccountParams` schema says `refreshToken: true` invokes the
  managed proactive refresh flow. Neither current official source specifies
  cross-device refresh-token rotation, last-writer-wins, compare-and-swap, or
  multi-writer safety. The absence is a compatibility constraint, not proof of
  safety.

## Environments

| Device | Platform | Codex | Auth state | Credential evidence |
|---|---|---:|---|---|
| Local | macOS 26.5.2 arm64 | `0.144.2` | ChatGPT login active | `~/.codex/auth.json`, mode `0600`; content never read into evidence |
| Remote | Linux 5.4.0 x86_64 | `0.144.4` | ChatGPT login active | `~/.codex/auth.json`, mode `0600`; content never read into evidence |

The app-server account values were compared only in process memory and proved
that both devices were using the same account before and after the first
concurrent refresh.

## Reproduction commands

Run schema generation and replay with the pinned CLI versions:

```bash
pnpm --silent dlx @openai/codex@0.142.5 app-server generate-json-schema --out "$TMPDIR/codex-0.142.5"
pnpm --silent dlx @openai/codex@0.143.0 app-server generate-json-schema --out "$TMPDIR/codex-0.143.0"
pnpm --silent dlx @openai/codex@0.144.2 app-server generate-json-schema --out "$TMPDIR/codex-0.144.2"
```

For each binary, a JSONL client sent `initialize`, `initialized`,
`account/read`, `account/rateLimits/read`, and `account/usage/read`. For
`0.144.2` it also sent `account/read` with `refreshToken: true`. The client
persisted only response keys and success booleans. The reproducible result is
in [app-server-account-matrix.json](app-server-account-matrix.json).

Device-auth initiation was run with a newly created empty `CODEX_HOME` on both
platforms. Both processes produced a device-auth prompt and authorization URL,
then were terminated while polling; the URL and user code were not recorded.
This proves initiation, not a completed headless login.

The two-device harness is:

```bash
python3 docs/spikes/codex/run_two_device_soak.py \
  --local-codex /Applications/ChatGPT.app/Contents/Resources/codex \
  --remote-host '<linux-host>' \
  --remote-codex /home/'<user>'/.local/bin/codex \
  --remote-home /home/'<user>' \
  --duration-hours 48 \
  --interval-seconds 3600 \
  --output /private/tmp/mad-codex-soak-20260714T2143Z.jsonl
```

The harness was stopped after four samples spanning `10812.845` seconds. It
compared the account only in memory, requested account/rate-limit/usage reads
hourly, and wrote only sanitized booleans/error codes. All four read samples
passed. The first sample also performed a concurrent proactive refresh: both
devices succeeded, both `auth.json` files changed, and both remained readable.
The persisted sanitized result is
[two-device-short-run.json](two-device-short-run.json).

## Results so far

| Sub-claim | Current evidence | State |
|---|---|---|
| Version-specific schema discovery | 267 generated files and combined-schema SHA-256 for each of `0.142.5`, `0.143.0`, `0.144.2` | Passed on macOS |
| `account/rateLimits/read` | Same request and response-key contract succeeded on all three versions | Passed on macOS |
| `account/usage/read` | Same request and response-key contract succeeded on all three versions | Passed on macOS |
| Managed proactive refresh | `account/read {refreshToken:true}` succeeded | Passed on macOS; also passed in first two-device sample |
| File credential store | Official contract plus active `0600` `auth.json` on macOS and Linux | Observed for default homes; isolated explicit-store login still not completed |
| Headless device auth | Isolated device-auth initiation and authorization URL on macOS and Linux | Initiation passed; completed login not claimed; interactive login remains required fallback |
| Same-account concurrent refresh | Four hourly samples passed across about three hours; the first refreshed both devices and retained account equality/readability | Short-run compatibility observed; long-duration and production multi-writer safety not claimed |

## Fallback and limitations

Production design must assume that a refreshable credential has exactly one
canonical writer. Device-specific runtime homes may contain session/config
state, but they must not independently persist refresh-token rotations. The
selected design is the planned `CredentialMaterializationManager`: one lease,
one refresh writer, revisioned CAS back to the Vault, and rejection of a second
writer. Headless device-auth initiation may be offered as an experimental
operator flow, but interactive login remains the required fallback until a
completed isolated device login is evidenced.

The evidence authorizes the exact app-server reads and managed refresh behavior
listed above. It does not authorize claiming multi-writer refresh support,
completed headless device login, 48-hour stability, or support outside the
exact versions and platforms above.
