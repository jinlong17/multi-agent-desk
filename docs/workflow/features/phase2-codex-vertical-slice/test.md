# Test strategy: Codex Vertical Slice

The test suite separates deterministic Provider-contract evidence from the
credentialed live Linux exit. A green fixture suite never upgrades an
unsupported Provider version or authorizes Ship.

## Acceptance matrix

| Requirement | Level | Scenario/evidence | Expected result |
|---|---|---|---|
| Provider schema migration | Integration/compatibility | upgrade current `0001`-`0003` database with Fake rows; apply Codex migration; restart; interrupt/future-version cases | Fake rows round-trip; Codex Account/Profile/CredentialInstance/Session/Approval/Usage records are valid; interruption/future version refuses safely |
| Store/provider allowlist | Unit/contract | create/read `fake` and `codex`; invalid Provider/AuthMethod; unknown Account linkage | known rows pass; unknown values and invalid links fail without partial writes |
| Local account/profile management | CLI/contract | list/create/edit/disable Account/Profile metadata; inspect Credential status; legacy Fake null Account/revision zero | metadata operations are bounded and secret-free; Codex requires Account/positive revision; Fake compatibility remains intact |
| Local IPC capability surface | Contract/E2E | `provider.describe/health`, `profile.validate`, `auth.*`, `usage.read`, `approval.list/respond`, `session.start/resume` with observer/controller identities | exact capability is required; bounded response; unknown methods and unauthorized clients fail closed |
| Local CLI surface | CLI/contract | `run codex`, `usage --provider codex`, `approvals list/respond`, explicit resume; retain `run fake` | commands use request-bound idempotency and never put secrets in argv; Fake commands remain compatible |
| Binary discovery | Unit/contract | missing, duplicate, non-executable, timeout, version mismatch | typed diagnostic; no shell execution or Session start |
| Schema gate | Contract | replay `0.142.5`, `0.143.0`, `0.144.2`; alter fingerprint/method list | exact rows pass; changed/unknown versions fail closed |
| JSONL framing | Unit/fuzz | partial frames, oversized frame, duplicate keys, trailing JSON, invalid UTF-8 | bounded parser rejects unsafe input without panic or secret echo |
| Initialize ordering | Contract | app-server notification before/after handshake; wrong protocol version | only negotiated sessions proceed; invalid order is `provider_protocol_error` |
| Account pinning | Unit/integration | change requested Account/Profile after start; rate-limit failure | request rejected; Session remains pinned; no auto-rotation |
| Auth-home isolation | Unit/security | two Profiles/instances, permissions, cleanup, inherited environment | distinct homes, restrictive modes, no cross-profile reads or residue |
| Single writer/CAS | Unit/concurrency | two materializers, refresh mutation, stale revision, same-account short fixture | one owner; conflict/CAS errors are deterministic; no mtime winner |
| Materialization boundary | Contract/security | call materializer from IPC/adapter with raw bytes, locked Vault, release/restart/quarantine | raw bytes rejected; only typed Vault source/handle works; Fake materializer cannot serve Codex |
| Vault v1 crypto and bounds | Unit/integration/security | fixed Argon2id/AES-256-GCM vectors; fresh DEK/nonces; canonical AAD field/revision changes; wrong password, tamper, truncated/oversized/non-object JSON, hostile KDF parameters | exact vectors interoperate; every wrong-key/AAD/tamper/bounds case returns one secret-free failure; no unbounded allocation |
| Vault first-use lifecycle | CLI/integration/concurrency/failure | empty `0005` schema/status; two-entry mismatch; initialize/lost response/same-key retry; two different concurrent initializers; crash before/after commit; restart; partial/duplicate/corrupt config; Fake metadata; existing Codex secret/enrollment/binding | migration remains uninitialized; mismatch sends no request; same request replays locked success; exactly one different init wins; pre-commit stays uninitialized; committed restarts locked; corruption/dependent-state init fails closed; Fake does not block; no password/body digest is persisted |
| Vault v1 atomic CAS/recovery | Integration/failure | expected/stale revision, crash before/after transaction commit, key-check creation, lock/restart, old binary on `0005`, backup restore | Vault item and CredentialInstance revision/digest/status move atomically; old revision survives pre-commit crash; committed revision is authoritative; old binary refuses schema |
| Logout/start exclusion | Integration/concurrency/failure | active Session logout; deterministic Session insert between durable revocation reservation and filesystem cleanup; crash/cleanup failure; lost response/retry | active Session preserves home and Vault; reservation rejects the racing start; interrupted logout remains fail-closed; retry idempotently removes home and atomically revokes Vault metadata/material |
| Official enrollment broker | CLI/integration/security | one-active-per-profile, owner mismatch, idempotent begin/complete/cancel, exact `codex login`, disallowed credential flags, success/cancel/timeout/child failure/restart | only metadata crosses IPC; official UI remains local; validated credential is atomically encrypted then staging removed; failure preserves prior healthy revision |
| Portable Vault platforms | CI/contract | native macOS and CI Linux/Windows round-trip using the password-derived backend; permission/DACL abstraction; unsupported OS-keyring migration | identical v1 envelope semantics on all three platforms; no Keychain/DPAPI/Secret Service or real Windows Codex claim |
| Crash recovery | Failure injection | kill app-server during auth-file mutation; malformed post-crash file | home quarantined, new starts blocked, official re-login required |
| Secret safety | Adversarial/static | scan logs, fixtures, errors, command args, audit records | no token, auth file, URL, device code, email, or account identity |
| Session lifecycle | Integration | start/output/stop/kill/Provider exit with Phase 1 store and ring | bounded events, correct Session transitions, no orphan writer |
| Runtime registration/ownership | Integration | authenticated `session.start` through daemon with fixture app-server; two compatible same-CredentialInstance Sessions; mismatched descriptor/revision; stop/kill one binding; last release; daemon close/early exit | one shared CredentialRuntime/child/materialization owner; distinct SessionBindings/threads; stopping one preserves the other; last release finalizes once; crash fans out once |
| JSON-RPC multiplexer | Unit/concurrency/fuzz | interleave responses, notifications, server requests, Usage calls, cancellation, unknown IDs, EOF, queue pressure | one reader preserves routing/order; calls do not starve events; all waiters fail boundedly on protocol exit |
| Approval control | Integration/security | observer response, stale lease, duplicate idempotency key, valid lease | only lease holder mutates; duplicates replay; stale/unknown requests reject |
| Approval exact response table | Contract/security | exact 0.144.2 command/file approve, deny, cancel; `acceptForSession`; policy amendments; permissions request; older compatibility rows | only command/file `accept|decline|cancel` responses are emitted; disabled decisions/methods fail closed with no Provider write |
| Approval dispatch transaction | Integration/failure | approve/deny/cancel durable claim and exact dispatch digest; request ID only in owning binding; successful write; same/different duplicate key; write failure; child exit; restart while dispatching | success pairs are approved/denied/cancelled with written; cancel duplicate returns stored result; different digest conflicts; uncertainty becomes expired/ambiguous; no replay or cross-binding response |
| Usage/Rate Limits | Contract/integration | supported response, absent method, changed field, timeout | source/freshness/confidence visible; unsupported is unknown/best-effort |
| Approval persistence/restart | Integration/failure | pending Approval, daemon restart, terminal response, duplicate key | bounded metadata survives; pending becomes expired/cancelled; no mutation replay; duplicate same digest returns ACK |
| Daemon-owned start policy | Contract/adversarial | caller attempts binary/version/capability/account/workspace override; unknown profile field; `danger-full-access`; oversized model/input; active-turn input; `turn/steer`/interrupt without exact row | daemon derives all Provider facts and canonical workspace; only `codex.v1` allowlist reaches `thread/start`/`turn/start`; every override/unsupported control fails before Provider write |
| Codex input/resize semantics | Contract/integration | idle input, active-turn input, conversation resize | idle input starts bounded turn; active input conflicts while steer is disabled; resize is typed unsupported with no Provider write |
| Second CLI | Live E2E | Linux server + second local CLI attach, replay, acquire/heartbeat, turn input, conversation resize | controller lease/event flow work; turn starts; resize returns typed unsupported; stale client cannot mutate |
| Resume | Live E2E/failure | controlled app-server restart without a proven continuation capability; snapshot Provider writes and local Session count before/after | typed `provider_resume_unsupported`; no Provider frame, no new local Session, no state transition, and no Fake/local Session masquerades as Provider history; verified continuation remains optional future evidence |
| macOS matrix | Live smoke | exact matrix versions and isolated `CODEX_HOME` | supported rows pass; new versions require new evidence |
| Windows posture | CI/contract | Go tests, Named Pipe/local IPC and cross-compile/build checks | green baseline; no real Windows Codex support claim is added |

## Unit and property tests

- Version parser and path allowlist, including platform-specific separators and
  no shell interpolation.
- JSONL frame decoder/encoder with size bounds, duplicate-key rejection, and
  deterministic error classification.
- Schema fingerprint/allowlist selection and capability downgrade.
- Event mapping, Provider item IDs, ordering, truncation, and redacted summary
  generation.
- Materialization path permissions, cleanup, digest comparison, structure
  validation, monotonic CAS, writer lease expiration, and quarantine state.
- Fixed Argon2id/AES-256-GCM vectors; independent payload/wrap nonces; AAD
  binding of format/device/provider/credential/account/revision; hostile KDF and
  64-KiB JSON bounds; per-CredentialInstance DEK separation; best-effort
  zeroization/release boundaries.
- Atomic VaultItem/CredentialInstance CAS and crash boundaries; key-check,
  forward-only schema refusal, backup/restore, enrollment owner/state/deadline/
  idempotency, exact login argv, forbidden flags, and staging cleanup.
- Empty-schema Vault status and one-time initialize transaction; local password
  confirmation, absent/valid/corrupt singleton classification, concurrent
  insert-if-absent, before/after-commit crash, restart locked state, refusal with
  existing dependent state, and explicit no-rekey behavior.
- JSON-RPC response correlation and ordered Provider queue under concurrent
  `ReadEvent`, Usage, Approval, interrupt, cancellation, EOF, and queue pressure.
- Migration table rebuilds, schema version monotonicity, Fake-row preservation,
  Account linkage, and future-schema refusal.
- Local IPC method-to-capability mapping, bounded response schemas, CLI request
  identity, Approval retention, UsageSnapshot freshness, and restart handling.
- Approval authorization, lease revision, idempotency digest, and replay.
- Approval cancel claim, `cancelled/written` commit, same-digest stored ACK,
  different-digest conflict, migration check constraints, and restart ambiguity.
- Usage/Rate Limit freshness and unknown/best-effort projection.

Property invariants:

1. No arbitrary app-server field can enter a persisted/audited event without an
   allowlist mapping.
2. A CredentialInstance never has two active canonical writers.
3. A stale revision cannot overwrite a newer credential revision.
4. A client without the current ControllerLease cannot cause a Provider
   mutation.
5. Unknown version/schema never enables a real Provider Session.
6. A local Approval cannot become approved, denied, or cancelled before the
   exact Provider response bytes are successfully written.
7. No Codex conversation resize emits a Provider request.
8. One daemon owns exactly one active CredentialRuntime/app-server writer per
   CredentialInstance; each local Session owns only a SessionBinding.
9. Stopping or killing one binding cannot stop the shared child while another
   binding is live; last release and crash fan-out finalize exactly once.
10. Callers cannot choose Provider binaries, versions, capabilities, Account
    identity, workspace path, arbitrary app-server configuration, or disabled
    control methods.
11. A typed unsupported Resume emits no Provider frame and creates no new local
    Session or Provider-history claim.

## Contract and fixture tests

Store only sanitized JSONL/JSON fixtures. For each enabled version, record:

- initialize handshake and negotiated version;
- account, rate-limit, and usage response key shapes;
- Approval request/response examples with payloads redacted or synthetic,
  including exact command/file `accept|decline|cancel` and negative
  session-persistent/amendment/permissions cases;
- normal output/turn completion, Provider error, and restart sequences.
- daemon-generated thread/start and turn/start allowlisted parameters,
  turn/interrupt only for an exact enabled row, disabled turn/steer, and the
  typed absence of a conversation-resize mapping.

Replay must run on macOS, Linux, and Windows CI using the same deterministic
parser and event mapper. A fixture is invalid if it contains a credential,
account identifier, authorization URL, device code, or unbounded payload.

## Integration and live E2E

1. Prepare a disposable Linux server with a pinned Codex binary, a test
   CredentialInstance, and an isolated `CODEX_HOME`; capture only sanitized
   command/version/evidence metadata.
2. Start one real Codex Session through the local CLI; verify structured event
   mapping, Usage/Rate Limits source, and Approval arrival.
3. Connect a second local CLI, observe/replay, acquire the ControllerLease,
   start a turn, confirm conversation resize returns
   `provider_control_unsupported` without a Provider frame, and respond to the
   Approval through the dispatch transaction.
4. Restart the app-server under a controlled failure, snapshot the local
   Session count and Provider writes, then execute the frozen resume contract.
   Unless an independently reviewed exact continuation capability exists,
   require `provider_resume_unsupported`, zero Provider frames, zero new local
   Sessions, and no Provider-history claim. Do not infer from Fake behavior.
5. Repeat the compatibility smoke on the exact macOS matrix where the Phase
   0.5 evidence exists.

The live test is a release-blocking Phase 2 exit criterion but is not itself a
release authorization. If no credentialed Linux environment is available, the
feature remains `BLOCKED` at verification with a named clearing role.

### P3B sanitized live receipt (2026-07-16)

- Linux x86_64 ran exact Codex `0.144.2` with canonical schema fingerprint
  `a1a35476587fe9bbfbe9e291b5200b8bc541df8c00241fe578d285ff26996e1c`.
- Official owner-bound login imported only bounded private `auth.json` into the
  local Vault at credential revision 2; login/app-server residue was ignored
  and the complete staging directory was removed.
- A second CLI attached, replayed, acquired/heartbeated the lease, sent a turn,
  and observed chunks reconstructing exactly `P3B_LINUX_OK`; resize returned
  `provider_control_unsupported`.
- Official Usage persisted a high-confidence `0.144.2` snapshot. A standard
  read-only fileChange Approval reached pending, was claimed and written once,
  completed as approved, and produced the exact disposable file while the
  Session stayed running.
- Binding stop produced `exited`; binding kill produced `killed`; Resume returned
  `provider_resume_unsupported` with local Session count unchanged (`16 -> 16`).
  The final Profile was restored to `on-request` / `workspace-write`, temporary
  shape diagnostics were removed, and the final daemon log was empty.

## Security/adversarial tests

- Attempt path traversal, symlink replacement, permission weakening, inherited
  secret environment, and shell metacharacter injection in binary/profile paths.
- For `NO_PROXY`, accept only a bounded list of domain, IPv4, IPv6, CIDR, and
  optional-port network entries. Reject empty entries, userinfo, key/value,
  URL/path/query/fragment syntax, whitespace/control characters, excessive
  entry count/entry size, and ambiguous ports. Prove login, validation, and
  runtime all consume the same validator.
  Positive rows: `*`, `localhost`, `.example.com`, `*.example.com`, IPv4,
  unbracketed IPv6, IPv4/IPv6 CIDR, `host:1`, `host:65535`, IPv4+port, and
  `[IPv6]:port`. Negative rows: 0/65 entries, 0/256-byte entry, 4097-byte total,
  empty list item, ASCII/Unicode whitespace, control, Unicode domain, empty /
  oversized/hyphen-ended label, schemes, userinfo, `=`, path, query, fragment,
  zone ID, invalid CIDR, unbracketed IPv6+port, and port `0`/`65536`/non-decimal.
- Start a second writer, mutate auth state after lease acquisition, return a
  stale CAS revision, and kill the writer at each commit boundary.
- Send malformed/oversized/duplicate-key app-server frames and unknown Approval
  IDs; verify bounded failure and no leaked body in diagnostics.
- Attempt Approval/terminal mutation from an observer or stale lease holder;
  verify no Provider request is emitted.
- Restart or kill the daemon after Approval dispatch claim but before/while the
  response write; verify ambiguous/expired state and no replay.
- Substitute the P2 test CredentialSource for production registration; verify
  daemon Codex start refuses it.
- Attempt to override binary path/version/capabilities/Account/workspace,
  inject unknown RuntimeProfile fields or `danger-full-access`, enable
  `turn/steer`, or answer permissions/session-persistent Approval; verify each
  fails closed before a Provider frame.
- Search every modified and untracked artifact plus generated dashboard state
  for token/email/account-display-name/phone-or-MFA metadata and forbidden
  multi-writer/48-hour/device-auth claims. The scan reports only file names and
  classifications, never matching secret content.
  The only allowed identifier-shaped hit is the exact synthetic rejection
  fixture in `internal/providers/codex/environment_test.go`, reported as
  `synthetic-security-fixture`; any other hit fails the phase.

## Failure injection and recovery

The test harness must cover binary disappearance, app-server early exit,
protocol mismatch, Usage method absence, auth-file unreadability, Vault lock,
writer conflict, CAS loss, quarantine, migration interruption/future schema,
unauthorized/duplicate Approval responses, unresolved Approval restart,
reconnect, and duplicate idempotency requests. Recovery must preserve Fake
Provider usability and require explicit operator action for re-login,
reconciliation, or a new Provider turn.

## Cross-platform matrix

| Platform | Automated | Live Provider claim in this feature |
|---|---|---|
| macOS | Go/unit/fixture/security + exact supported Codex smoke | supported only for recorded versions/paths |
| Linux | Go/unit/fixture/security + required real vertical-slice exit | Phase 2 exit platform; exact version/schema must be recorded |
| Windows | Go/unit/fixture/security, Named Pipe/CLI build, cross-compile/CI | no real Codex support claim; Phase 1 baseline remains required |

### P4 platform-matrix receipt (2026-07-16)

- macOS 26.5.2 arm64: an isolated official npm Codex `0.144.2` binary passed
  `TestConfiguredCodexBinaryCanonicalSchemaProbe` and
  `TestConfiguredCodexBinaryEmptyHomeHandshake`; the current ChatGPT-bundled
  `0.144.5` is outside the allowlist and remains unsupported.
- Windows amd64: current Provider, runtime, app, and device test packages all
  compile as Windows test executables; `go build ./cmd/...` passes with
  `GOOS=windows GOARCH=amd64 CGO_ENABLED=0`. The unchanged native Phase 1 IPC
  baseline is retained by successful Actions run `29469271422`, Windows job
  `87528995056`. This is build/protocol evidence only, not real Windows Codex
  execution.
- Linux x86_64: exact credentialed `0.144.2` P3B live receipt remains the only
  real Provider support claim in this feature.
- P4 documentation, ADR, threat-model, compatibility, README, secret-like diff
  scan, and Security Review handoff must remain consistent with these limits.

## Verification commands

The final feature-verify record must include the repository's actual commands,
not invented npm scripts. At minimum:

```bash
go test ./...
go vet ./...
git diff --check
```

If the implementation adds a provider-specific command or fixture runner, its
exact invocation, version, platform, and sanitized output must be recorded in
the feature log and compatibility matrix.

## Rollback acceptance

After disabling/removing Codex registration, Phase 1 Fake Provider sessions,
local IPC, Vault lock/unlock, and existing device data must continue to pass.
`0005` is forward-only: an old binary must refuse the newer schema, and recovery
uses a verified backup/restore or newer binary, never a destructive down
migration. A failed live Provider gate must not be hidden by deleting evidence.

## Handoff

Next role: `feature-review`.
