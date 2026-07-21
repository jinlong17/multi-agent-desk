# Feature Brief: Phase 4a Control Plane Core

- Slug: `phase4a-control-plane-core`
- Date: 2026-07-21
- Owner module: `control-plane`
- Impacted modules: `security`, `core`, `web`, `desktop`, `project-system`
- Requested by: operator-directed sequential completion of the remaining v0.1 roadmap

## Motivation and outcome

Phase 1 and the Codex local vertical slice provide a device-owned daemon,
authenticated local IPC, durable metadata, session control, and a production
local Vault boundary. The next unblocked product milestone is a Control Plane
that can authenticate one user, enroll and revoke devices, synchronize only
non-secret configuration, index device/session/usage metadata, and route
asynchronous session commands without pretending that a remote process starts
synchronously.

The outcome is a self-hosted `multidesk-server` and metadata-only Web surface.
An operator can complete a one-time bootstrap ceremony with a Passkey and an
OS-Vault-backed Daemon/Desktop trust anchor, use one-time recovery codes,
approve additional Daemon/Web/Desktop identities through pinned keys and signed
attestations, observe presence, revoke a device, synchronize allowed metadata
with explicit revisions and tombstones, and submit/query asynchronous session
commands. Overview, Devices, Accounts, Profiles, Sessions, and Usage pages show
only metadata and evidence-backed freshness/source fields.

This phase does not deliver remote terminal plaintext, E2EE realtime payloads,
credential transfer, or a stable Claude provider path. Claude subscription-only
compatibility remains an independent macOS experiment and is not a dependency
for Phase 4a.

## Scope

1. Implement the `multidesk-server` process, configuration validation, health
   and version endpoints, graceful lifecycle, and a versioned Control Plane
   SQLite store with WAL, foreign keys, ordered migrations, expiry cleanup, and
   unknown-future-schema refusal.
2. Implement an atomic ten-minute bootstrap ceremony that creates the first
   single user, verifies a Passkey for a fixed production RP ID/origin, registers
   one OS-Vault-backed Daemon/Desktop as the initial trust anchor, issues
   one-time recovery codes, and removes the bootstrap-token hash only after the
   ceremony commits in full.
3. Implement Passkey authentication, secure cookie sessions, CSRF protection,
   one-time hashed recovery-code consumption, session expiry/revocation, and
   deployment validation for HTTPS and stable RP ID/origin constraints.
4. Implement Daemon/Web/Desktop enrollment, challenge signatures, ten-minute
   one-time enrollment expiry, six-group fingerprint confirmation, signed
   DeviceAttestation validation, local pinned-key checks, presence, key-change
   rejection, and revocation with immediate connection invalidation.
5. Define OpenAPI as the REST type authority and generate/verify Go and
   TypeScript clients for bootstrap/auth, devices, accounts, profiles,
   workspaces, sessions, usage, session commands, audit, health, and version.
6. Implement metadata APIs with UUIDv7 identifiers, bounded cursor pagination,
   allowlisted filtering/sorting, `apiVersion`, stable typed errors,
   idempotency keys, and revision-based `If-Match` conflict responses.
7. Implement asynchronous `start|stop|kill|resume|acquire_control|release_control`
   Session Command creation/querying and daemon delivery/acknowledgement. The
   REST request returns `202 + commandId` and never claims a remote process has
   already started.
8. Implement revisioned configuration synchronization through device outbox,
   server inbox, per-device cursors, idempotent replay, field-level conflicts,
   deletion tombstones, and retention only after all known eligible devices
   have acknowledged the relevant revision.
9. Implement a responsive metadata-only Web/PWA shell and the Overview,
   Devices, Accounts, Profiles, Sessions, and Usage pages against generated
   protocol clients, including loading/empty/error/offline/revoked states and
   accessible keyboard/screen-reader behavior.
10. Add bounded audit events, redacted structured logs, metrics for API/device
    connection health, deployment examples needed to preserve the RP ID/TLS
    contract, and unit/contract/integration/security evidence for the Phase 4a
    exit scenarios.

## Non-goals

- E2EE Pairwise Roots, HPKE wrapping, realtime terminal/event plaintext, WSS
  replay/flow control, xterm.js control, remote Approval response, or Web Device
  decryption capability; those belong to Phase 4b.
- CredentialGrant, Provider credential upload, Vault secret transport, macOS
  Keychain/Windows DPAPI/Linux Secret Service migration, or Desktop packaging;
  those belong to Phase 5.
- Docker release artifacts, signing, notarization, SBOM, `v0.1.0` tagging, or
  production release claims; those belong to Phase 6.
- Claude API keys, usage-credit spending, subscription-token export, setup-token
  injection, or treating the gated Claude Phase 3 path as complete.
- Password login, email recovery, multi-user/team tenancy, RBAC, hosted SaaS,
  Provider request proxying, automatic account rotation, or terminal-content
  persistence in the Control Plane.
- Treating Passkey authentication as an E2EE device identity or allowing a pure
  browser to bootstrap the first cryptographic trust anchor.

## User journeys

1. The operator starts a new Control Plane with a fixed HTTPS origin/RP ID and
   receives a bootstrap token exactly once. An expired, replayed, partial, or
   origin-mismatched ceremony fails without leaving an activated user or trust
   root.
2. During bootstrap the operator registers a Passkey, records a one-time set of
   recovery codes, and mutually confirms the fingerprint of an OS-Vault-backed
   Daemon/Desktop. Only the atomic completion activates the account.
3. The operator signs in with the Passkey. Secure session cookies and CSRF
   checks protect mutations; one recovery code can be consumed once to regain
   access and register a replacement Passkey without changing the RP ID
   silently.
4. A new Daemon, Web, or Desktop identity requests enrollment. An already
   pinned device verifies the displayed fingerprint and signs an attestation;
   expired challenges, unpinned approvers, digest mismatch, and changed keys are
   rejected and audited.
5. The operator views online/offline presence and revokes a device. Its active
   authenticated connection closes, new requests fail, and metadata reflects
   revocation without claiming deletion of data or secrets already held by the
   endpoint.
6. A daemon uploads non-secret Account/Profile/Workspace/Session/Usage metadata
   from an outbox and applies server inbox entries. Retry is idempotent; stale
   revisions return a field-level conflict; deletion propagates through a
   tombstone before retention cleanup.
7. The operator submits a Session Command from Web. The API returns a durable
   command identifier, the target daemon later acknowledges a result, and the
   Web page distinguishes queued, delivered, acknowledged, failed, expired,
   unsupported, and offline outcomes.
8. An authenticated but not E2EE-enrolled Web client can view allowed metadata
   only. No terminal, Approval, Pairwise Root, Provider plaintext, or credential
   action is present or implied in Phase 4a.

## Data and trust boundaries

- The Control Plane owns user authentication, device directory/connection
  status, non-secret synchronized metadata, asynchronous command metadata,
  audit events, and short-lived enrollment/session state.
- Each Daemon remains the authority for Provider credentials, Vault contents,
  Runtime Profile materialization, Provider processes, complete session events,
  and terminal/model plaintext. Phase 4a adds no server-side credential column
  or Provider auth import path.
- Passkeys authenticate the single user to the server. Device identities use
  separate Ed25519 signing and X25519 exchange keys; Passkeys neither derive nor
  replace those keys.
- The server public-key directory is an index, not a trust anchor. Sensitive
  enrollment decisions require locally pinned approver keys and an attestation
  whose device/key digests match. A server-supplied key change fails closed.
- Web and Desktop are distinct Device identities even when Desktop connects to
  a daemon on the same host. Only `kind=daemon` owns credentials or Provider
  processes.
- Control Plane SQLite and logs may contain metadata, hashes, revisions,
  ciphertext-free command status, and bounded audit context. They must not
  contain Provider tokens/auth files, Vault keys, terminal/model plaintext,
  Passkey private material, recovery-code plaintext, or raw sensitive request
  bodies.

## Provider/external assumptions

- Phase 4a is Provider-neutral. Fake and verified Codex metadata can exercise
  contracts; no live Provider token is required in CI or stored by the server.
- Passkey behavior must follow the selected WebAuthn library's current stable
  API and W3C/WebAuthn semantics. The implementation plan must pin the library,
  validate its license and supported Go/toolchain versions, and record any
  browser ceremony constraints before build.
- Production Passkeys require HTTPS plus a fixed RP ID/origin. `localhost` is a
  development-only exception and cannot be promoted to a production default.
- UUIDv7, SQLite, OpenAPI generation, cookie/CSRF middleware, and any Web
  dependencies must pass the repository license and three-platform build gates.
- Phase 1 local IPC and Device identity foundations are reusable, but server
  authentication and network protocol cannot inherit trust solely from local
  filesystem/socket ownership.

## Dependencies and gates

- Phase 1 Device Kernel is shipped on remote `main` and is the only required
  product-phase dependency. Phase 3 Claude is explicitly not a dependency.
- ADR 0002, 0003, 0005, 0010, 0011, and the implementation plan define the
  durable device-owned-secret, metadata-only server, migration, identity, and
  pinning boundaries. Phase 4b protocol work may not be pulled into this phase.
- Owner: `control-plane`; `security` reviews bootstrap/auth/enrollment/revocation,
  `core` owns daemon integration boundaries, `web` owns metadata presentation,
  `desktop` is a protocol consumer only, and `project-system` owns CI/dashboard
  reconciliation.
- Feature Review must approve the decision-complete design/API/test plan before
  implementation. One writer implements one approved phase, followed by an
  independent Feature Verify verdict before the next phase.
- Security Review is required before Ship because authentication, recovery,
  device identity, revocation, cookies/CSRF, public-key pinning, and network
  ingress are in scope.
- Provider Gate: none for Phase 4a. Live Claude/Codex execution is not required
  for the metadata and Fake asynchronous-command exit tests.
- Protected-main integration, seven required checks, exact-main regeneration,
  Ship receipt, and dashboard reconciliation are part of the operator-authorized
  completion sequence. Release/tag/deployment remains a Phase 6 gate.

## Acceptance criteria

- [ ] Empty-server startup applies ordered server migrations once, enables WAL
      and foreign keys, preserves data across restart, bounds busy/cleanup work,
      and refuses an unknown future schema without destructive fallback.
- [ ] Bootstrap token plaintext is emitted once, only its bounded hash persists,
      the ceremony expires after ten minutes, and user + Passkey + recovery-code
      hashes + OS-Vault trust anchor activate atomically or not at all.
- [ ] A pure Web client cannot become the initial trust anchor; incomplete,
      expired, replayed, origin/RP-ID-mismatched, or concurrent bootstrap
      attempts leave no partial active identity and produce redacted audit facts.
- [ ] Passkey registration/authentication verifies challenge, origin, RP ID,
      user presence/verification policy, signature counter behavior, and replay;
      production config rejects insecure or mutable RP-ID/origin settings.
- [ ] Auth sessions use `Secure`, `HttpOnly`, and appropriate `SameSite` cookies,
      expire/revoke deterministically, and every state-changing browser request
      passes origin and CSRF validation.
- [ ] Recovery codes are generated from adequate randomness, displayed once,
      stored only as individually salted/slow hashes, consumed atomically once,
      rate limited, and never logged or returned by later reads.
- [ ] Enrollment validates Device kind/capabilities, key formats, challenge
      signature, expiry, one-time use, fingerprint derivation, pinned approver,
      attestation signature, and exact key digests before activation.
- [ ] Duplicate/replayed enrollment, changed keys, invalid/unpinned approver,
      digest mismatch, unsupported device capability, and revoked identity all
      fail closed with stable error codes and redacted audit events.
- [ ] Presence transitions survive reconnect and restart without falsely
      treating an unauthenticated socket as online; revocation immediately
      closes active authenticated connections and blocks later reads/writes.
- [ ] OpenAPI is the REST type authority; generated Go/TypeScript clients are
      reproducible and CI rejects schema/client drift, undocumented fields,
      invalid pagination/filter/sort values, and responses without `apiVersion`.
- [ ] Every mutation honors `Idempotency-Key`; updates/deletes require
      `If-Match`; stale revisions return `409 sync_conflict` with bounded
      field-level differences and never silently overwrite server state.
- [ ] Session Command creation returns `202 + commandId`; retries are idempotent;
      only an authenticated eligible target can claim/ack it; offline, expired,
      unsupported, duplicate, and daemon-restart paths have durable typed states
      and never claim synchronous Provider success.
- [ ] Sync outbox/inbox processing is ordered, bounded, transactional, and
      idempotent; cursors advance only after commit; replay creates no duplicate
      resource; secret fields and credential grants are rejected from this path.
- [ ] Tombstones carry resource type/id/revision/deletion time, propagate to all
      known eligible devices, resist stale resurrection, and are retained until
      acknowledgement plus the configured retention condition is satisfied.
- [ ] Overview, Devices, Accounts, Profiles, Sessions, and Usage pages render
      only allowed metadata and correct loading/empty/error/offline/revoked/
      conflict states; Usage always shows source, confidence, and observation
      time and never fabricates official Claude quota.
- [ ] An authenticated but unapproved Web Device remains metadata-only and the
      Phase 4a Web bundle exposes no terminal input, Approval response,
      CredentialGrant, E2EE key delivery, or Provider plaintext path.
- [ ] Server database/log/debug-bundle scans find no Provider secret, terminal
      content, Passkey private material, recovery-code plaintext, cookie value,
      CSRF token, or raw challenge; request/body/frame bounds and rate limits are
      covered by hostile-input tests.
- [ ] Pairing, key-change rejection, revocation, sync conflict, tombstone,
      asynchronous command, restart, and offline behavior pass deterministic
      integration tests; local Phase 1/Codex behavior remains green.
- [ ] Go tests/vet/format, Web typecheck/build/tests, OpenAPI generation check,
      security tests, license/link/DCO/project verification, and protected
      Ubuntu/macOS/Windows builds all pass before merge.
- [ ] Architecture, data model, threat model, OpenAPI/operator/deployment docs,
      feature evidence, dashboard, and compatibility claims describe only the
      verified metadata-only Phase 4a boundary and retain Phase 4b/5/6 gates.

## Risks and open questions

- WebAuthn libraries differ in counter policy, resident-key behavior, metadata
  validation, and origin configuration. The plan must choose and pin one library
  only after a bounded compatibility/license spike or documented evaluation.
- Atomic bootstrap spans authentication material and a signed device trust
  anchor. A partial transaction or external ceremony retry must not produce an
  active user that lacks a recoverable trust root.
- The server cannot prove a remote human compared fingerprints. It can verify
  challenges and attestations, while the UI/CLI must make the manual comparison
  explicit and preserve this residual social-engineering risk.
- Presence is advisory and connection-derived. It must not be used as proof that
  a Provider process, credential, or Session is healthy.
- Async Session Commands cross an offline boundary and can race expiry,
  cancellation, reconnect, and daemon restart. Durable claims/idempotency must be
  designed before handlers or UI are built.
- Metadata schemas can accidentally grow secret-like fields. An allowlist at the
  domain, OpenAPI, sync, database, logging, and UI boundaries is safer than
  attempting to redact arbitrary objects after receipt.
- Tombstone garbage collection depends on a precise definition of known and
  permanently revoked devices. The feature plan must freeze acknowledgement and
  retention semantics to avoid either stale resurrection or unbounded storage.
- The existing Web/Desktop packages are scaffolds until implementation audit
  proves otherwise. Visual polish cannot substitute for generated-client,
  security, accessibility, and offline-state correctness.
- Phase 4a is broad. The feature plan must split it into independently
  verifiable phases and keep E2EE realtime, credential transfer, packaging, and
  Claude subscription experiments out of every build phase.

## Evidence

- `docs/IMPLEMENTATION_PLAN.md` sections 4, 5.3-5.5, 6.5, 6.8, 6.13,
  12.1-12.3, 13.1, 14.2, 15, 16, 19 Phase 4a, and 20
- `docs/workflow/project/dashboard-state.json` Phase 4a dependency/gate
- `docs/workflow/project/workflow.md` lifecycle and verification transitions
- `docs/workflow/project/module-registry.json` module ownership boundaries
- ADR 0002, 0003, 0005, 0010, and 0011
- Phase 1 and Phase 2 shipped feature evidence on protected remote `main`

## Handoff

Next role: `feature-plan`.
