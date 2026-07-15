# Feature Brief: Phase 1 Device Kernel

- Slug: `phase1-device-kernel`
- Date: 2026-07-14
- Owner module: `core`
- Impacted modules: `security, provider, desktop, project-system`
- Requested by: Plan v0.2 Phase 1 and operator-directed sequential execution

## Motivation and outcome

Phase 0 and the Phase 0.5 decision gates are shipped, but MultiAgentDesk still
contains only an empty scaffold. Phase 1 must produce the first usable local
kernel without depending on real Codex, Claude, the Control Plane, Web, or
Desktop. The outcome is a single cross-platform `multidesk` binary whose
daemon owns a durable Device database and Fake Provider process, while a second
CLI connects over authenticated local IPC to observe and control that Session.

Success means the same behavioral contract passes on macOS, Linux, and
Windows: start, list/observe, attach/detach, acquire/release/expire control,
input, resize, graceful stop, forced kill, and resume as a new Session record.

## Scope

1. Implement the Phase 1 domain model and invariants for Device, Workspace,
   RuntimeProfile, Fake CredentialInstance, Session, SessionAttachment,
   ControllerLease, runtime events, and audit metadata.
2. Add an ordered Device SQLite migration set, WAL/foreign-key configuration,
   transactional repositories, schema/version checks, and restart recovery.
3. Implement one local IPC application protocol with bounded framed messages,
   request IDs, version negotiation, mutual client/daemon authentication,
   capabilities, lease revisions, idempotency, deadlines, and redacted errors.
4. Use `0600` Unix-domain sockets on macOS/Linux and ADR 0013 protected
   current-logon message-mode Named Pipes on Windows, with fail-closed
   single-instance ownership and no silent transport downgrade.
5. Implement daemon serve/status/start/stop plus user-service
   install/uninstall specifications for macOS, Linux, and Windows, with
   non-privileged deterministic tests and explicit unsupported-context errors.
6. Implement Vault `locked`/`unlocked` state and metadata-only behavior while
   locked; Phase 1 uses fake secret material and must not collect real Provider
   credentials.
7. Implement a Fake Provider subprocess, Process Manager, terminal stream/ring
   buffer, resize signal, Session state machine, attachment lifecycle, and
   ControllerLease arbitration.
8. Implement a fake CredentialMaterializationManager with one writer,
   monotonic revision/CAS, restrictive runtime-home permissions, atomic
   materialization, refresh, cleanup, quarantine, and crash recovery tests.
9. Replace the scaffold CLI with stable Phase 1 commands and JSON output for
   daemon/vault/session/control operations; provide a minimal local TUI/status
   view over the same application service and IPC path.
10. Add unit, contract, integration, failure-injection, and three-platform CI
    evidence that exercises the Phase 1 exit scenario with two CLI processes.
11. Update the as-built architecture/data-model/operator documentation and
    dashboard only from verified implementation facts.

## Non-goals

- Real Codex or Claude discovery, login, authentication, usage, approval, or
  Session integration.
- Unix PTY or production ConPTY provider execution; Phase 1 freezes the runtime
  stream/resize abstraction and uses a Fake Provider subprocess.
- Control Plane, Passkey, remote commands, E2EE, Device enrollment, Web/PWA, or
  Desktop UI/sidecar implementation.
- Real CredentialGrant or production Provider secret storage.
- Automatic account rotation, quota bypass, rate-limit evasion, proxying, or
  silent credential switching.
- Privileged system-wide service installation, signed packages, Windows 11
  workstation acceptance, release, deployment, tag, or version bump.
- A full-screen production TUI; only the minimal Phase 1 local status/control
  surface is required.

## User journeys

1. A user initializes a Device directory, starts the daemon, and sees a healthy
   authenticated local endpoint without exposing a public listener.
2. The user unlocks the fake Vault and starts a Fake Session for a local
   Workspace/Profile; the daemon persists the Session and owns the child.
3. A second `multidesk` process lists and observes the running Session, receives
   bounded replay, attaches without stopping the provider, and detaches safely.
4. The second process acquires the ControllerLease, sends input and resize,
   observes Fake Provider output, and releases or lets the lease expire.
5. Stop requests graceful termination; Kill forces termination; both are
   idempotent and persist the correct terminal state.
6. Resume creates a new Session linked by `resumedFromSessionId`; the old
   terminal record never returns to `running`.
7. After daemon or materialization failure, restart recovery either restores a
   safe state or quarantines ambiguous residue without overwriting a newer
   credential revision.
8. While the Vault is locked, metadata queries work but new Session or secret
   materialization returns an explicit `vault_locked` error.

## Data and trust boundaries

- The Device daemon is the only SQLite, Vault, materialization, Provider
  process, Session, attachment, and lease writer.
- CLI/TUI clients never read the database directly; every operation uses the
  application service locally or the same IPC contract.
- OS endpoint permissions narrow access but do not authenticate the client.
  The daemon and client mutually authenticate above the transport before any
  metadata or mutation is exposed.
- Every mutation binds protocol version, authenticated client identity,
  capability, request ID, and—where applicable—the current lease revision.
- Terminal content, fake credential bytes, IPC payloads, and runtime-home
  contents never enter logs, audit records, dashboard state, or error text.
- The ring buffer is bounded and explicitly reports truncated replay.
- Phase 1 uses generated fake material only. Introducing real Provider
  credentials requires a later owning feature and security review.

## Provider/external assumptions

- Fake Provider behavior is first-party and deterministic; it does not prove a
  real Provider's PTY, auth, resume, or signal behavior.
- ADR 0013 is authoritative for Windows local IPC. Windows Server CI proves the
  implementation contract, not Windows 11 multi-user/service acceptance.
- ADR 0005 requires SQLite WAL mode, ordered migrations, and explicit
  transactions. The feature plan must select a cross-platform Go driver whose
  current license and Go 1.26 support pass repository policy.
- External libraries for SQLite, identifiers, or Windows IPC must be pinned,
  license-checked, and isolated behind owned interfaces; no Go plugin or CGO
  requirement may break the three-platform binary.

## Dependencies and gates

- Depends on shipped Phase 0 repository/CI governance and shipped Phase 0.5
  decisions, especially ADR 0002, 0005, 0009, 0012, 0013, and 0015.
- Owner: `core`; Provider only reviews the runtime abstraction impact, Desktop
  only consumes the daemon/service/IPC contract later, and project-system owns
  CI/dashboard changes.
- Provider Gate: none for the deterministic Fake Provider.
- Security Gate: open. Final `READY_TO_SHIP` must undergo independent
  `security-review` for local peer authentication, endpoint ownership,
  capability/lease authorization, Vault/materialization recovery, resource
  bounds, audit redaction, and residual same-logon/admin risk.
- Phase completion, branch, push, merge, and ship are already authorized by the
  operator for this sequential execution. No release or deployment is implied.

## Acceptance criteria

- [ ] Domain tests reject illegal Session transitions, direct resume mutation,
  stale lease revisions, multiple controllers, and unauthorized mutations.
- [ ] Device SQLite starts from an empty database, applies ordered migrations
  once, uses WAL/foreign keys/busy timeout, survives restart, and rejects an
  unknown future schema without destructive fallback.
- [ ] The daemon enforces one instance, owns all local state, and exposes no
  public TCP listener.
- [ ] Unix socket endpoints are user-private; Windows Named Pipes enforce ADR
  0013 DACL/remote/first-instance rules. Both require successful protocol
  mutual authentication before metadata or mutation.
- [ ] IPC rejects wrong version, wrong identity/proof, oversized/malformed
  frames, missing capability, stale/missing lease, duplicate non-idempotent
  requests, slow clients, and endpoint ownership ambiguity.
- [ ] A second CLI can list/observe, attach, receive replay, detach, acquire
  control, send input/resize, release control, stop, and kill a Fake Session.
- [ ] Detach never stops the provider; observer mutations fail; lease timeout
  permits a new controller; Stop/Kill are idempotent.
- [ ] Resume creates a new Session with `resumedFromSessionId` and leaves the
  original terminal record unchanged.
- [ ] Vault lock permits metadata reads but denies Session start and
  materialization with `vault_locked`; unlock restores permitted fake flows.
- [ ] Fake materialization proves a single writer, monotonic CAS, atomic
  permissions, refresh, cleanup, kill/crash recovery, and quarantine on
  ambiguous residue without exposing fake bytes in diagnostics.
- [ ] User-service specifications are deterministic for macOS, Linux, and
  Windows; install/uninstall/status commands fail explicitly when the platform
  or privilege context is unsupported.
- [ ] JSON CLI contracts are deterministic and human output contains no secret
  or terminal payload leakage; the minimal TUI uses the same IPC/application
  path and does not read SQLite directly.
- [ ] macOS, Ubuntu, and Windows CI run the Phase 1 unit/contract/integration
  suite, including an actual daemon plus two client processes and platform
  local IPC.
- [ ] `npm run project:verify`, CI fixtures/links/licenses, Go formatting/tests,
  cross-platform builds, race tests on supported runners, and failure-injection
  tests pass; no new incompatible license appears.
- [ ] `docs/ARCHITECTURE.md`, `docs/DATA_MODEL.md`, CLI/operator docs, feature
  state, and dashboard describe only verified Phase 1 behavior and retain all
  later Provider/Windows/release gates.
- [ ] Independent feature verification and the required Security Gate both
  pass before protected-main ship; resulting `main` checks are green.

## Risks and open questions

- Selecting a pure-Go/portable SQLite driver may add a large dependency tree;
  license, build time, binary size, lock behavior, and Go 1.26 compatibility
  must be measured before freezing the plan.
- Windows Named Pipe security and message-mode semantics are materially more
  complex than Unix sockets; same-logon malware and administrators remain
  residual risks even after ADR 0013 controls.
- A local bootstrap secret or key is needed for mutual IPC authentication; its
  creation, storage, rotation, client provisioning, and recovery must be
  decided explicitly without treating filesystem possession as identity.
- Fake process behavior can hide real PTY/provider edge cases; the interface
  must preserve truthful capability boundaries and defer real acceptance.
- Process ownership after daemon crash differs across platforms. Recovery must
  prefer quarantine/explicit failure over adopting an unauthenticated child.
- Service installation APIs can mutate the host. Automated tests must use
  rendered specifications/temp roots or safe user contexts and never alter the
  developer's actual login/startup configuration.
- `go test -race` support and cost differ by platform; the plan must define
  where race evidence is mandatory and what Windows equivalent remains.

## Evidence

- `docs/IMPLEMENTATION_PLAN.md` Phase 1, domain model, CLI, and test sections
- ADR 0002, 0005, 0009, 0012, 0013, and 0015
- `docs/THREAT_MODEL.md` T-05 through T-10, T-14, T-18
- Phase 0.5 evidence reconciliation and ship receipt
- Existing three-platform protected CI and license/DCO/link governance

## Handoff

Next role: `feature-plan`.
