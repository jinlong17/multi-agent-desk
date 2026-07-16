# Feature Brief: Codex Vertical Slice

- Slug: `phase2-codex-vertical-slice`
- Date: 2026-07-15
- Owner module: `provider`
- Impacted modules: `core`, `security`, `project-system`
- Requested by: operator continuation after Phase 1 was shipped to `main`

## Motivation and outcome

Phase 1 delivered the cross-platform Device Kernel with a deterministic Fake
Provider, local IPC authentication, session state, attachments, ControllerLease,
ring-buffer replay, and resume semantics. The next product risk is not another
fake lifecycle: it is proving that the same local contracts can safely own a
real Codex process without leaking or corrupting a refreshable credential.

The outcome is a reviewable, version-gated Codex adapter and one end-to-end
vertical slice. First, the Device domain, SQLite schema, authenticated local IPC,
and CLI must be able to represent both the existing Fake Provider and Codex
without data loss. On a supported Linux server, an operator can then use a
selected Codex Account/Profile to create a real Session, receive structured
app-server events, inspect only evidence-backed Usage/Rate Limit data, respond
to a structured Approval while holding the existing ControllerLease, and
exercise an explicit, truthful Resume result from a second local CLI. The slice must preserve
the Fake Provider path when Codex is unavailable or its schema is unknown.

This is a design and implementation plan, not a claim that the Codex path is
currently runnable. P0-P2 contracts and a bounded protocol adapter now exist,
but the daemon still rejects Codex starts, the Phase 1 Vault stores no encrypted
credential payload, and Approval responses are not dispatched to app-server.
Revision v0.5 freezes the previously missing Vault initialization transaction
and durable Approval-cancel state in addition to the v0.4 Vault/enrollment,
shared runtime ownership, daemon authority, exact Approval response mapping,
and no-mutation Resume exit before another build.

## Scope

1. Add the Device domain/SQLite compatibility foundation for Codex Account,
   RuntimeProfile, CredentialInstance, Session, Approval, and UsageSnapshot
   records while preserving existing Fake Provider data and future-schema
   refusal behavior.
2. Add the authenticated local IPC capabilities, methods, idempotency, replay,
   and thin CLI surfaces required for Codex start, Usage/Rate Limits, and
   structured Approval response.
3. Discover and pin a Codex binary, report its exact version, and reject or
   downgrade unknown versions rather than assuming compatibility.
4. Define a version-specific stdio JSONL app-server client with an
   initialization handshake, schema/method allowlist, bounded frames, and
   secret-safe diagnostics.
5. Add the minimum production local-Vault credential source needed by this
   slice: a portable password-derived Argon2id/AES-256-GCM Vault v1, official
   interactive enrollment into daemon-owned staging, encrypted at rest per
   CredentialInstance, atomic Vault-item plus revision/digest/status CAS, and
   no raw credential bytes in IPC. Then materialize one isolated canonical
   `CODEX_HOME` through the single-writer/revisioned-CAS boundary.
6. Register Codex with the daemon runtime. One `CredentialRuntime` per
   CredentialInstance owns the binary process, single-reader JSON-RPC client,
   materialization handle, capabilities, and finalization; each local Session
   owns a `SessionBinding` with its own thread/turn/Approval/event state.
7. Integrate Codex Session start, structured output/events, Approval request and
   response, Usage/Rate Limit snapshots, binding-scoped stop/kill, and the
   frozen typed-unsupported Resume path with the Phase 1 Device Kernel and CLI.
8. Add sanitized record/replay fixtures, exact-version compatibility entries,
   schema drift detection, and deterministic fallback behavior.
9. Validate the live exit scenario on a pinned Linux environment and perform
   the matching macOS compatibility smoke. Keep Windows CI/build and protocol
   acceptance explicit without claiming a real Windows Codex release path.

## Non-goals

- Claude Code, PTY/ConPTY, Web, Desktop, Control Plane, E2EE relay, or remote
  browser control.
- Multi-writer refresh, 48-hour stability, last-writer-wins, mtime-based token
  selection, automatic account rotation, quota bypass, or transparent account
  switching.
- Treating device-auth initiation as completed headless login. Official
  interactive login remains the supported fallback until new evidence exists.
- Copying a macOS Keychain, browser cookie, Provider auth file, token, device
  code, or account identity into logs, fixtures, telemetry, or the Control Plane.
- Claiming Codex compatibility for versions, operating systems, or app-server
  methods not present in the compatibility matrix and passing fixture/live gates.
- Release packaging, signing, deployment, or production readiness.
- Treating Codex conversation input as a PTY. `session.input` may start or steer
  an exact-schema Codex turn; `session.resize` is typed unsupported unless a
  future command-execution process ID and resize method are independently
  verified.

## User journeys

1. The operator asks the local CLI to inspect Codex. The daemon discovers the
   binary and presents the exact path/version and enabled capabilities; an
   unknown or malformed version is visibly unsupported.
2. The operator creates or selects a Codex Account/Profile through the local
   Device store. Existing Fake Provider records remain readable after the
   schema upgrade, and unknown/future schema versions fail closed.
3. The operator completes official interactive login through a local,
   authenticated enrollment broker. Provider output stays attached to the
   operator's local terminal/browser, while the daemon owns a private staging
   home, validates the result, encrypts it into the local Vault, destroys the
   staging copy, and records only digest/revision metadata.
4. The operator starts a Codex Session from the local CLI. The daemon launches
   the pinned app-server over stdio, maps only the negotiated schema methods,
   and persists bounded structured events alongside the existing session state.
5. A Codex Approval arrives as a structured request. The second local CLI may
   observe it, but only the current ControllerLease holder may respond; duplicate
   responses are idempotent and stale leases are rejected.
6. The second local CLI attaches, replays the ring buffer, acquires or
   heartbeats the ControllerLease, sends Codex turn input, observes an explicit
   typed-unsupported result for conversation resize, and invokes the explicit
   Codex Resume contract after an app-server restart. Unless separately proven,
   Resume returns `provider_resume_unsupported` without a Provider frame, a new
   local Session, or a false Provider-history recovery claim.
7. The operator requests Usage and Rate Limits. Supported fields include source
   version, freshness, and confidence; an unavailable or changed method is
   reported as unknown/best-effort instead of fabricated official data.

## Data and trust boundaries

- The Device Daemon owns the Vault, CredentialInstance, materialized
  `CODEX_HOME`, app-server process, session plaintext, and Provider events.
- The Control Plane is not involved in this slice and must never become a
  credential writer or receive Provider plaintext. Any future grant remains a
  target-device-scoped E2EE operation outside this feature.
- Local CLI requests cross the Phase 1 authenticated IPC boundary. Session
  control and Approval response remain protected by ControllerLease and
  idempotency rules.
- The app-server receives plaintext credentials only inside the authorized
  device process boundary. Auth-home content is never placed in JSON logs,
  fixture artifacts, crash reports, or command arguments.
- A compromised same-user process, administrator, backup, or Provider process
  may still copy a materialized credential. The plan reduces accidental/stale
  exposure; it does not promise erasure after compromise.

## Provider/external assumptions

- Current evidence supports schema generation, `initialize`, `account/read`,
  `account/rateLimits/read`, and `account/usage/read` for Codex CLI `0.142.5`,
  `0.143.0`, and `0.144.2` on the recorded macOS environment. Exact Linux
  runtime versions require their own schema replay before live acceptance.
- File-backed auth and a short same-account two-device refresh run were observed
  on macOS `0.144.2` and Linux `0.144.4`. This does not support a multi-writer
  or 48-hour claim.
- ADR 0014 therefore requires one canonical writable app-server/auth home per
  CredentialInstance, an exclusive materialization lease, monotonic
  `credentialRevision` CAS, and quarantine/re-login on ambiguous mutation.
- Device-auth initiation is experimental. Completed isolated device login is
  not established; interactive login is the required supported path.
- The adapter must use the official `codex app-server` boundary and versioned
  schema, not an undocumented internal API or network proxy.

## Dependencies and gates

- Phase 1 Device Kernel is shipped on `main` and provides the daemon, Vault
  lock-state boundary, local IPC, Session state, Attachment, ControllerLease,
  replay, and resume primitives. It does **not** provide production encrypted
  credential storage; the minimum local-only secret source/read/replace-CAS
  capability must be pulled forward into P2B. Credential Grant/E2EE remains a
  later Phase 5 concern.
- Phase 2 must first extend the Phase 1 fake-only domain/SQLite/IPC contract;
  this compatibility foundation is part of the feature and is not an implicit
  follow-up migration.
- Phase 0.5 Codex Spike is `GATE_RESOLVED` under
  `docs/workflow/features/spike-codex-auth-refresh/dev_log.md`, ADR 0014, and
  `docs/PROVIDER_COMPATIBILITY.md`. The resolution constrains this feature; it
  does not waive implementation acceptance.
- Provider gate: exact binary/schema version, Linux live environment, and
  compatibility-matrix evidence must be recorded before the live phase.
- Security gate: open for implementation review because credential
  materialization, auth refresh, Approval control, and crash recovery are in
  scope. Security review is required before Ship.
- No Control Plane, Web, Desktop, or E2EE dependency is needed for the Phase 2
  exit; those remain later phases.
- The normal independent `feature-review` is the P2B/P3A pre-build gate. The
  final workflow `security-review` remains required from `READY_TO_SHIP`; the
  plan does not invent an unsupported pre-build security-review transition.

## Acceptance criteria

- [ ] Binary discovery reports a pinned path/version and refuses unknown,
      mismatched, or schema-incompatible versions.
- [ ] A versioned Device migration and domain/store contract accepts `fake` and
      `codex` Provider records, preserves existing Fake data, records Account,
      Approval, and UsageSnapshot metadata without secrets, and rejects unknown
      future schemas.
- [ ] Authenticated local IPC and thin CLI expose the planned Codex start,
      Usage/Rate Limit read, Approval observe/respond, and explicit resume
      methods with capability checks, request-bound idempotency, bounded replay,
      and restart semantics.
- [ ] Fixture replay passes for every enabled method on the recorded
      `0.142.5`, `0.143.0`, and `0.144.2` schemas; malformed frames, duplicate
      fields, unsupported methods, and schema drift fail closed.
- [ ] Vault v1 uses the frozen Argon2id parameters, fresh per-item DEK,
      AES-256-GCM payload/wrap with independent nonces, canonical AAD, strict
      64-KiB JSON bound, key-check envelope, and one transaction that CAS-writes
      Vault item plus CredentialInstance revision/digest/status. Tamper, wrong
      password, hostile parameters, crash boundaries, and old-binary refusal
      fail closed on macOS, Linux, and Windows portable-backend tests.
- [ ] Migration `0005` creates empty schema only. One authenticated
      `vault.initialize` operation atomically moves `uninitialized` to locked
      with a fresh salt/key-check singleton; two locally confirmed password
      entries match before IPC; same-client/request retry returns stored locked
      success without a password digest; competing requests have one winner;
      crash before/after commit, restart, corrupt singleton, Fake compatibility,
      and three-platform tests are deterministic. Rekey remains out of scope.
- [ ] Official `codex login` enrollment is owner-bound, one-active-per-profile,
      persisted/idempotent/deadline-bounded, carries only metadata over IPC,
      disables API-key/access-token/device-auth flags, validates before atomic
      import, and preserves the prior revision on cancel/failure/restart.
- [ ] One CredentialInstance has exactly one canonical `CredentialRuntime` and
      isolated `CODEX_HOME`; compatible Sessions have separate bindings/threads,
      stopping one preserves the shared child, last release finalizes once,
      and child failure fans out once.
- [ ] The daemon, not the caller, derives binary/schema/capabilities, linked
      Account, canonical Workspace, and the allowlisted `codex.v1`
      `thread/start`/`turn/start` parameters. Unknown profile fields,
      `danger-full-access`, active-turn input, and unverified controls fail
      before a Provider write.
- [ ] Concurrent JSON-RPC calls and notifications are correlated by one reader
      loop; no blocking `ReadEvent` call can starve Usage, Approval, stop, or
      shutdown operations.
- [ ] Approval response persists a dispatch claim before writing the exact
      server-request response, becomes terminal only after a successful write,
      and becomes ambiguous/expired without automatic replay after a write or
      restart uncertainty.
- [ ] Exact `0.144.2` command/file Approval decisions map local
      `approve|deny|cancel` to `accept|decline|cancel`; `acceptForSession`,
      policy amendments, permission grants, and older unproven rows emit no
      response and fail closed.
- [ ] Approval cancel is durably claimed as requested decision `cancel`, commits
      only after the Provider write as `cancelled/written`, returns that stored
      result for an identical idempotency digest, conflicts on a different
      digest, and becomes expired/ambiguous after uncertain dispatch/restart.
- [ ] A pinned Linux server creates a real Codex Session, maps bounded structured
      events, exposes only evidence-backed Usage/Rate Limits, handles a
      structured Approval, and preserves the selected Account/Profile.
- [ ] A second local CLI can attach, replay, acquire/heartbeat the
      ControllerLease, send turn input, receive typed unsupported for Codex
      conversation resize, respond to the Approval, and exercise the frozen
      typed-unsupported Resume result after a controlled app-server restart;
      that path emits no Provider mutation, creates no local Session, and makes
      no recovery claim; stale/duplicate controls are rejected or idempotent.
- [ ] macOS compatibility smoke passes for the supported matrix; Windows
      build/protocol tests remain green and the documentation explicitly does
      not claim real Windows Codex support from this slice.
- [ ] `docs/PROVIDER_COMPATIBILITY.md`, the feature Evidence Ledger, and the
      verification report contain exact versions, platforms, commands, and
      sanitized artifacts; no unsupported method or stability claim is added.

## Risks and open questions

- The current Phase 0.5 evidence does not include a Linux app-server schema
  replay for the intended live version. This is a pre-build gate, not a reason
  to invent a version claim.
- The current Phase 1 schema is intentionally Fake-only. The compatibility
  migration and local IPC foundation must land before credential materialization
  or a real Session can be built.
- App-server schema or event ordering may drift. Mitigation: schema fingerprint,
  fixture replay, allowlist, bounded event mapping, and fail-closed downgrade.
- A Provider crash can leave an auth file changed but not durably reconciled.
  Mitigation: digest/structure validation, monotonic CAS, quarantine, and
  explicit re-login; never select by mtime.
- Approval semantics may differ across Codex versions. The adapter must keep a
  structured Approval contract and mark unsupported versions rather than parse
  terminal text.
- The Phase 1 Vault is a lock-state placeholder. P2B therefore implements only
  the portable password-derived Vault v1; Keychain/DPAPI/Secret Service wrapping
  is deferred to a Phase 5 migration and cannot reinterpret existing rows.
- The current protocol client serializes reads behind one mutex. A permanent
  event read would starve calls; P3A requires a single-reader multiplexer before
  daemon registration.
- A live test may lack a suitable Linux account or binary. The deterministic
  fixtures remain useful, but the Phase 2 exit cannot be marked complete until
  the real Linux Session scenario is reproduced.
- Windows has strong Phase 1 ConPTY/Named Pipe/sidecar evidence, but no real
  Codex provider acceptance in this slice. Windows Desktop remains Experimental
  per the implementation plan.

## Evidence

- `docs/IMPLEMENTATION_PLAN.md` §§8, 19, and 21 define the Codex boundary,
  Phase 0.5 dependency, Phase 2 deliverables, and exit condition.
- `docs/adr/0014-codex-app-server-single-writer-auth.md` freezes the
  single-writer, revisioned-CAS, interactive-login fallback.
- `docs/spikes/codex/2026-07-14-auth-refresh-spike.md`,
  `docs/spikes/codex/app-server-account-matrix.json`, and
  `docs/spikes/codex/two-device-short-run.json` contain sanitized evidence.
- `docs/PROVIDER_COMPATIBILITY.md` records exact supported/fallback rows.
- Phase 1 Device Kernel contracts are in `internal/runtime`, `internal/device`,
  `internal/vault`, `internal/app`, and `migrations/device` on `main`.
- Phase 1's current fake-only constraints and local method surface are explicit
  in `migrations/device/0001_device_identity.sql`,
  `migrations/device/0002_sessions.sql`, `internal/storage/repository.go`,
  `internal/app/authorization.go`, and `cmd/multidesk/commands.go`; this
  revision addresses that gap instead of assuming it away.

## Handoff

Next role: `feature-review`.
