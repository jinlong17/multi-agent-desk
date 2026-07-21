# Test strategy: Claude API-key provider vertical slice

No CI command receives a real Provider key. Live Provider acceptance runs only
in an isolated manual environment after a dedicated key is injected through a
non-recorded channel and the operator explicitly authorizes the ordinary billed
requests. Sanitized evidence must be reviewed before it enters Git.

## Acceptance matrix

| ID | Requirement | Level | Command/scenario | Expected evidence |
|---|---|---|---|---|
| G0 | Exact API-key precedence, empty-config isolation, minimal environment, PTY/status/error behavior | Provider Spike + Security Review | `spike-claude-api-key-cli-compatibility` on exact macOS/Linux targets | `GATE_RESOLVED`, accepted security report, new ADR and compatibility rows; otherwise no build |
| P1 | Schema 7 upgrades without changing Fake/Codex rows; Claude/api_key/billing/session-metric constraints enforce | migration/integration | fresh DB, populated schema-7 upgrade, interrupted transaction, restart, future-schema refusal | row-by-row preservation, invalid insert rejection, no destructive down migration |
| P1 | Key entry is hidden/stdin-only and sealed solely in Vault | unit/integration/security | prompt, `--api-key-stdin`, generic JSON/argv/env negative paths | no key in argv/settings/log/audit/idempotency/DB plaintext; Vault revision advances atomically |
| P1 | Confirmation precedes secret submission | contract/integration | wrong client, stale revision, expired enrollment, submit-before-confirm | typed failure, no Vault item or lingering secret state |
| P1 | Secret submission replay is safe | integration/failure injection | lose response; repeat same submission/key; reuse submission ID with different key | same redacted result for same secret; conflict for different secret; one Vault revision |
| P2 | Exact binary/platform compatibility gate | unit/fixture | accepted tuple, digest drift, unknown version, unsupported platform | only exact accepted row enables auth/session; failure occurs before Vault access/spawn |
| P2 | Child environment contains selected key and no conflicting auth/routing | adversarial integration | seed every forbidden Anthropic/Claude/AWS/GCP/Azure variable and config helper | selected key is child-only; conflicts fail closed or are removed exactly as Spike decision; no inherited subscription/cloud route |
| P2 | Empty config does not cross into subscription/Keychain state | live macOS/Linux | accepted Spike/feature harness with isolated `CLAUDE_CONFIG_DIR` | API-key auth class only; no raw identity retained; cleanup passes |
| P3 | Preview binds exact tuple and billing source | contract/integration | preview then mutate Account/Profile/Credential/Workspace/binary/capability | every mutation invalidates start before materialization/spawn |
| P3 | Explicit alias required; no default/rotation | unit/integration | missing alias, ambiguous alias, disabled Account, failed selected key with second healthy key present | typed failure; zero alternate selection or Provider process |
| P3 | Health taxonomy is redacted and truthful | fixture/live | invalid sentinel, Provider 401/403/billing/rate/network fixtures, one approved live request | stable distinct class; no raw body, endpoint, request/account identity, or key fragment |
| P3 | Session metrics preserve meaning | unit/fixture/live | null/status drift/repeated samples/real approved response | separate cost/context metrics, correct unit/source/confidence/version/freshness, replay dedup, no limit/reset fabrication |
| P4 | PTY lifecycle uses existing Session/Attachment/Lease | integration | start, second client observe, acquire, input, three exact resizes, detach/reconnect/replay, stop | immutable binding, exact resize, one controller, bounded replay, clean terminal state |
| P4 | Stop/kill/crash cleanup is bounded | failure injection | normal exit, ignored graceful stop, child tree, daemon crash/restart, sanitizer failure | tree terminated, config removed, key not logged, ambiguous state quarantined, no paid request replay |
| P5 | Linux amd64 real Provider exit | manual isolated live | exact pinned binary, dedicated key, ordinary operator task, second local CLI | real API-key-billed PTY input/resize/replay/lease/stop, sanitized health/metric evidence, no secret retained |
| P6 | macOS arm64 real Provider exit | manual isolated live | exact pinned binary, empty config, existing subscription present but excluded | same lifecycle plus no Keychain/subscription crossover; exact row only |
| P6 | Windows remains truthful | build/mechanism | Windows amd64 cross-build and existing ConPTY suite; attempt Claude preview | build/ConPTY pass; preview returns `provider_platform_unsupported` before secret/spawn |
| P7 | Full regression/security/governance | repository | commands below plus independent feature/security verification | all deterministic checks pass; live evidence stays separately scoped; no High/Critical open issue |
| RB | Rollback is non-destructive and fail closed | integration/manual | disable compatibility row, stop runtime, open schema 8 with old binary, restore backup drill | no new starts, active cleanup bounded, encrypted key preserved or explicitly deleted, old binary refuses schema |

## Unit and property tests

Planned package coverage:

- `internal/domain`: `ProviderClaude`, `AuthMethodAPIKey`, billing-source and
  health enums, metric kind/unit validation, strict Profile settings, and
  canonical compatibility/confirmation digests.
- `internal/providers/claude`: version parser, binary fingerprint, compatibility
  allowlist, child-environment scrub/allowlist, strict redaction, error reducer,
  status-line projection, canonical sample digest, and process exit mapping.
- `internal/vault`: Claude payload size/shape validation, per-item envelope,
  expected-revision replace CAS, same-secret replay, conflicting-secret replay,
  key-buffer cleanup hooks, and local revoke.
- `internal/app`: enrollment state machine, confirmation ownership/revision,
  secret-submit special path, preview tuple, single-use consumption, and no
  alternate Account selection.
- `internal/runtime`: PTY start/input/resize/stop/kill and cleanup reuse the
  existing provider-neutral invariants.

Property/fuzz targets include:

- arbitrary settings keys cannot smuggle a credential, endpoint, custom header,
  config directory, cloud selector, or credential-helper configuration;
- arbitrary Provider error/status payloads reduce to a bounded stable class
  without retaining raw strings;
- arbitrary status-line JSON is size-bounded, rejects duplicate/unknown fields
  under the accepted schema, never emits a negative/NaN/Inf metric, and never
  preserves a path or identity field;
- canonical sample/confirmation digests are deterministic and field/order
  sensitive; and
- arbitrary terminal input cannot bypass frame or ControllerLease bounds.

Representative deterministic commands after implementation:

```bash
go test -count=1 ./internal/domain ./internal/providers/claude ./internal/vault ./internal/app ./internal/runtime
go test -race -count=1 ./internal/providers/claude ./internal/vault ./internal/app ./internal/runtime
go test -run 'Fuzz|Property' -count=1 ./internal/providers/claude ./internal/app
```

## Contract and fixture tests

Fixtures are synthetic or sanitized and versioned by exact CLI/platform tuple.
They must contain no key, email, organization, prompt, response text, transcript
path, workspace path, endpoint, request ID, or live billing value.

Required fixtures:

- version output and binary/compatibility fingerprint;
- redacted auth-health success plus invalid-key, permission, billing, rate,
  network, timeout, and unknown-schema classes;
- status-line samples with null-before-response, valid client-estimated cost,
  current-context tokens, duplicate sample, added unknown field, oversized JSON,
  malformed number, and sensitive-path canaries;
- PTY ANSI/input/resize/exit transcripts made from deterministic local fakes,
  not real Claude conversation content; and
- Linux/macOS compatibility result manifests containing sanitized result classes
  and hashes only.

Tests prove the parser rejects a fixture when its declared version/digest or
schema differs. Updating a fixture never auto-enables compatibility; the matrix
decision is a separate reviewed change.

## Integration and E2E

### Enrollment/Vault

1. Initialize and unlock a disposable Vault.
2. Create one Claude Account/Profile/alias and a second unrelated Account.
3. Begin enrollment and verify the preview is secret-free.
4. Attempt submit before confirmation, from another client, after expiry, and
   after Profile revision drift; verify no Vault item.
5. Confirm the exact tuple, submit a synthetic secret through the dedicated
   method, and inspect database/log/audit/idempotency/debug outputs for canaries.
6. Simulate a lost response, replay the same submission, then a conflicting
   submission; verify one item/revision and deterministic conflict.
7. Restart locked, unlock, locally revoke only the selected credential, and
   prove the unrelated Account/Credential remains byte-for-byte unchanged.

### Preview/Session

1. Create a supported synthetic compatibility row and a Daemon-owned Fake
   Claude child that behaves like a PTY without network access.
2. Request `sessions.preview` with explicit `@alias` and Workspace.
3. Mutate each bound revision/fingerprint independently and prove the old
   preview cannot be consumed.
4. Consume a fresh confirmed preview twice; same-request replay returns the
   original Session, different-request replay fails, and only one child exists.
5. Attach a second client as observer, acquire control, send bounded input,
   apply three resizes, detach/reconnect, replay, and stop.
6. Verify no Provider output or child environment entered SQLite or logs.

### Metrics

1. Feed valid and invalid status-line fixtures through the exact sanitizer.
2. Persist separate cost and current-context metric rows.
3. Replay identical samples and verify no double count; change a value and
   verify a new sample remains bound to the same immutable Session tuple.
4. Feed null or changed schema and verify explicit unavailable/schema-changed
   status without zeros, subscription windows, monthly credit, or limits.

## Security/adversarial tests

- Seed key canaries into prompt input and verify absence from process argv,
  parent/daemon environment, SQLite plaintext, Profile JSON, terminal output,
  ordinary logs, audit metadata, idempotency records, dashboard/generated state,
  crash/error strings, debug bundle, test reports, and Git diff.
- Confirm the selected child necessarily receives the key in its environment,
  and document that same-user/root inspection remains a residual risk rather
  than claiming invisibility.
- Seed all documented conflicting auth and routing variables, including API/
  auth/OAuth tokens, base URLs, custom headers, Bedrock/Vertex/Foundry selectors,
  credential helpers, and common AWS/GCP/Azure credential paths. Prove no
  unconfirmed route reaches the child.
- Place malicious config/settings in default and isolated config roots; prove
  the accepted empty-config contract does not load subscription OAuth,
  Keychain state, hooks, plugins, MCP, or helper configuration beyond the
  reviewed allowlist.
- Race submit/revoke/start and submit/replace operations; CAS and preview
  revisions must prevent stale-key use.
- Attempt symlink/path escape and weak permissions for config/temp/helper paths;
  fail before key write or spawn.
- Inject oversized/malformed PTY, status, hook, and error payloads; maintain
  bounds and redaction.
- Kill the CLI, helper, daemon, and process tree at each lifecycle boundary;
  verify no paid request replay and bounded cleanup/quarantine.
- Search retained artifacts for secret, identity, endpoint, transcript, and
  workspace-path canaries before evidence review.

No test asserts that local logout remotely revokes the key. A separate manual
Console action proves Provider-side revocation guidance only.

## Cross-platform matrix

| Scenario | Linux amd64 | macOS arm64 | Windows amd64 |
|---|---|---|---|
| unit/fixture/race | required | required | required in CI where applicable |
| build | required | required | required |
| PTY mechanism | required | required | existing ConPTY regression required |
| real API-key auth | required exact live row | required exact live row | not in scope |
| real input/resize/replay/stop | required | required | not in scope |
| empty-config subscription crossover | required | required including Keychain boundary | not claimed |
| status/metric projection | required or explicit reviewed unavailable | required or explicit reviewed unavailable | typed unsupported |
| stable support | only after live pass | only after live pass | unsupported |

Other versions and architectures remain unsupported until a fresh gated Spike.

Representative cross-build commands:

```bash
GOOS=linux GOARCH=amd64 go build ./cmd/multidesk
GOOS=darwin GOARCH=arm64 go build ./cmd/multidesk
GOOS=windows GOARCH=amd64 go build ./cmd/multidesk
go test -count=1 ./internal/device ./internal/runtime
```

The final build phase also runs the repository's existing Windows native CI,
not a macOS cross-build presented as Windows runtime evidence.

## Failure injection and recovery

Required interruption points:

- before/after enrollment confirmation;
- during secret frame read, Vault encrypt, Vault CAS commit, and response write;
- after Session reservation but before Vault open, environment build, PTY spawn,
  and running transition;
- during first output, input, resize, status projection, graceful stop, kill
  escalation, materialization release, and config removal; and
- during migration table rebuild and first restart on schema 8.

For each point, tests record Session/enrollment/Vault state, surviving process
tree, config paths, retry behavior, error class, and secret-scan result. Recovery
never retries a Provider request automatically, selects another credential, or
reconstructs a secret from durable plaintext.

## Manual acceptance

### Preconditions

- Operator supplies a dedicated Claude Console test API key through a
  non-recorded local mechanism. The key is never pasted into chat, issue, PR,
  command argument, shell history, or Git artifact.
- Operator separately authorizes each bounded ordinary paid arm and its known
  maximum/expected cost. Absence of this authorization blocks the paid arm; it
  is not implied by plan or test approval.
- Exact official binaries, hashes, OS/architecture, clean test workspaces, and
  disposable Daemon/Vault/config roots are recorded.
- The current local Team subscription identity is treated only as a crossover
  canary; it is never used to satisfy API-key acceptance.

### Provider Spike arm

The Spike first runs documentation/version/hash/config/environment and invalid-
sentinel checks that require no real key. Only after the preconditions are met
does it run bounded API-key requests using the accepted combination of `-p`,
JSON/stream-JSON, `--max-turns`, `--max-budget-usd`, and
`--no-session-persistence`, followed by the smallest interactive PTY arm needed
to observe input/resize/cleanup. The Spike records only sanitized classes,
digests, and bounds.

### Linux feature exit

On the exact accepted Linux amd64 tuple:

1. Enroll through hidden prompt/stdin and confirm Console/API billing source.
2. Preview and confirm `@alias` plus Workspace.
3. Start one ordinary operator-approved Claude Code task.
4. Attach a second local CLI, observe output, acquire control, send input, apply
   exact resizes, detach, reconnect/replay, and stop gracefully.
5. Repeat with kill escalation and daemon restart cleanup where it does not
   create an extra paid request.
6. Verify redacted health and either sanitized session metrics or the exact
   reviewed unavailable state.
7. Scan artifacts, release config/materialization, stop the daemon, and retain
   no key or raw Provider content.

### macOS feature exit

Repeat the Linux lifecycle on the exact accepted macOS arm64 tuple while a
separate Claude.ai subscription login exists as a crossover canary. Prove the
child uses only the selected API key and isolated config, never the default
Keychain/subscription identity. Do not record email, organization, account
subject, prompt, response, or live billing value.

### Rollback drill

Disable the Claude compatibility row, prove new preview/start fails before
Vault access, stop existing test Sessions, remove transient config, and confirm
the encrypted Vault item remains readable only by the current schema/binary.
Then explicitly delete the test credential locally and revoke the dedicated key
in Claude Console. Preserve only sanitized completion evidence.

## Phase verification gates

Every feature-build phase stops at `READY_FOR_VERIFY` and receives an
independent `feature-verify` verdict before the next phase:

| Phase | Verification focus |
|---|---|
| P1 | migration, enrollment, Vault CAS, secret absence |
| P2 | exact compatibility, environment isolation, redaction/error fixtures |
| P3 | preview/confirmation, health/metrics contracts, no selection fallback |
| P4 | deterministic PTY/lease/replay/cleanup and full race regression |
| P5 | sanitized exact Linux live evidence |
| P6 | sanitized exact macOS live evidence plus truthful Windows gate |
| P7 | full regression, docs/compatibility, rollback, Security Review readiness |

Final Security Review is mandatory because the feature handles a Provider key,
billing authorization, child environment, status/hook input, Vault state, and
PTY control.

## Repository verification

Proportionate checks for planning/workflow changes:

```bash
npm run workflow:generate
npm run workflow:verify
npm run dashboard
npm run dashboard:verify
npm run project:verify
```

The feature's final deterministic matrix additionally includes:

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
GOOS=linux GOARCH=amd64 go build ./cmd/multidesk
GOOS=darwin GOARCH=arm64 go build ./cmd/multidesk
GOOS=windows GOARCH=amd64 go build ./cmd/multidesk
pnpm --dir apps/web check
pnpm --dir apps/web build
pnpm --dir apps/desktop exec cargo fmt --check --manifest-path src-tauri/Cargo.toml
pnpm --dir apps/desktop exec cargo check --manifest-path src-tauri/Cargo.toml
```

Exact bundled/system runtime paths may be used as documented by the repository,
but the command and version are recorded in the Evidence Ledger. Structural
verification is necessary but never substitutes for live Provider acceptance.
