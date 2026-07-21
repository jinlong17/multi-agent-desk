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
An operator can complete a one-time bootstrap ceremony with a Passkey and a
Daemon trust anchor whose new remote Device keys are encrypted in the already
shipped portable password-derived Vault v1, use one-time recovery codes,
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
   one portable-Vault-v1-backed Daemon as the initial trust anchor, issues
   one-time recovery codes, and removes the bootstrap-token hash only after the
   ceremony commits in full. Before enabling bootstrap, add Device migration
   `0008`, the generic UUID mapping, exact Vault-v1 `DeviceKeyEnvelopeV1`
   create/open/CAS and pending-to-active lifecycle, plus the executable Daemon
   prepare/prove/activate and Web import/verify actor flow. Add a local-only,
   pre-user bootstrap-token rotate command; the storage mode is a signed client
   assertion backed by official Daemon integration evidence, not a server-
   verifiable at-rest claim.
3. Implement Passkey authentication, secure cookie sessions, CSRF protection,
   the exact pre-auth/authenticated/Device endpoint security matrix, ten-code
   recovery batches/rotation, Passkey list/delete/recent-UV lifecycle,
   recovery-to-normal session transition, session expiry/revocation, and fixed
   HTTPS RP ID/origin constraints.
4. Implement Daemon/Web enrollment plus the server-side `kind=desktop` contract,
   challenge proofs, ten-minute
   one-time enrollment expiry, six-group fingerprint confirmation, signed
   DeviceAttestation validation, local pinned-key checks, presence, key-change
   rejection, public signed activation receipts, versioned capability
   attestation/elevation, and revocation with immediate invalidation.
5. Define OpenAPI as the REST type authority and generate/verify Go and
   TypeScript contract artifacts: generated Go server/client/models, generated
   TypeScript OpenAPI types, and a first-party exhaustively typed runtime client
   for bootstrap/auth, devices, accounts, profiles, workspaces, sessions, usage,
   session commands, audit, health, and version.
6. Implement metadata APIs with UUIDv7 identifiers, bounded cursor pagination,
   allowlisted filtering/sorting, `apiVersion`, stable typed errors,
   idempotency keys, and revision-based `If-Match` conflict responses.
7. Implement asynchronous `start|stop|kill|resume|acquire_control|release_control`
   Session Command creation/querying and daemon delivery/claim/ack/result with
   durable receipt reconciliation, reserved-only expired-attempt CAS rebind,
   later-state reconcile, and ambiguous-state refusal. The
   REST request returns `202 + commandId` and never claims a remote process has
   already started.
8. Implement revisioned configuration synchronization through device outbox,
   server inbox, per-device cursors, idempotent replay, field-level conflicts,
   exact domain-separated canonical full-base/full-next/patch wire including
   create sentinel and missing-history behavior, deletion tombstones plus
   lifetime watermarks, authoritative topological snapshots with an already-
   authenticated target Device prerequisite and canonical manifest/page/final
   digest chain, bidirectional UUID mappings, and retention only after all known
   eligible devices acknowledge the revision.
9. Implement a responsive metadata-only Web/PWA shell and the Overview,
   Devices, Accounts, Profiles, Sessions, and Usage pages against generated
   TypeScript types through the first-party runtime client, including loading/
   empty/error/offline/revoked states and accessible keyboard/screen-reader
   behavior.
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
   recovery codes, and mutually confirms the fingerprint of a Daemon whose new
   remote keys are already encrypted in portable Vault v1. Only the atomic
   completion activates the account.
3. The operator signs in with the Passkey. Secure session cookies and CSRF
   checks protect mutations; one recovery code can be consumed once to regain
   access and register a replacement Passkey without changing the RP ID
   silently.
4. A new Daemon or Web identity requests enrollment. The same server contract
   accepts a fixture-backed `kind=desktop`, while the Desktop product key-store
   client remains Phase 5. An already pinned device verifies the displayed
   fingerprint and signs an attestation;
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
- Web and Desktop are distinct Device kinds even when Desktop later connects to
  a daemon on the same host. Phase 4a implements Web key generation/storage and
  enrollment; it accepts/tests the Desktop server contract only. The Desktop
  product key-store client remains Phase 5. Only `kind=daemon` owns credentials
  or Provider processes.
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

## Plan v0.7 contract amendment

The P0 contract remains independently verified. Plan v0.7 retains every v0.6
contract and every v0.5 architectural boundary, including Feature Review v5's
five closed wire-definition findings, and closes Feature Review v6's sole P5
cross-document outcome-union contradiction. P1 implementation is paused at
a safe checkpoint while the complete Phase 4a OpenAPI and generated clients are
reconciled to this amendment. P1 may expose runtime behavior only for
`healthz`, `readyz`, and `version`; every P2+ operation must exist as a complete
OpenAPI/generator contract but remain unmounted or fail closed until its own
verified build phase.

The v0.7 amendment retains these additional exit requirements without expanding
the Phase 4a product boundary:

1. P2 binds every remote identity to one immutable canonical HTTPS server
   origin in local storage, Vault AAD, bootstrap descriptors, and receipts;
   completes the exact WebAuthn binary DTO/counter-CAS/session-revocation and
   passkey-deletion semantics; uses the `__Host-mad_session` cookie; and requires
   a verified v7 backup before Device migration 0008 plus real-browser receipts.
2. P3 defines the signed Device-auth challenge/exchange protocol, persistent
   nonce/restart behavior, all typed JCS/digest/signature domains, candidate
   pre-auth and the complete enrollment state machine. Candidate pin/receipt
   verification precedes the final activation acknowledgement. Migration 0009
   owns remote pins/receipts, remote identity IDs remain distinct from the local
   IPC Device ID, browser and Device capabilities are separate, lifecycle and
   presence are separate, and `software_wrapped` Web Devices perform real
   X25519 proof of possession. P3 only sets `snapshot_required`.
3. P4 caps snapshot pages so every worst-case response fits the one-MiB wire
   limit, excludes Fake Sessions from the network projection, creates browser
   Profiles disabled pending local-only policy completion, and keeps server sync
   revisions separate from Device-local entity revisions. P4 migration moves to
   0010.
4. P5 freezes canonical command/request digests, preallocated result Session
   IDs, an authoritative signed Device query, append-only delivery revisions,
   exact half-open clocks/priorities, bounded attempts/retention, state-specific
   receipts and restart proofs, a dedicated `RemoteCommandService`, stable
   per-kind outcomes, deterministic per-call idempotency, and bounded worker/
   per-Session concurrency. `acquire_control|release_control` are always typed
   unsupported until Phase 4b. P5 migration moves to 0011.
5. P6 adds enrollment listing, a bounded Overview aggregate, exact Usage units/
   scales and Profile conflict DTO, browser-safe crypto/JCS/pin/PoP plus
   IndexedDB pin/key CAS, strict PWA/API cache boundaries, executable browser
   and Desktop render receipts, and exact UI behavior for online derivation,
   capability elevation, command polling, recovery-output privacy, and logout
   key retention. P6 may be reviewed as P6A/P6B without scope expansion.

For complete P1 generation, v0.7 keeps these shapes closed and
non-overlapping: P5 list offers do not claim, claim alone allocates attempt/
lease, and ack alone validates the locally durable reserved receipt and commits
the contiguous server delivery cursor; every claim/ack/result/reconcile/query
DTO carries its exact delivery/receipt revision and closed outcome/proof oneOf.
P3 signs only the strict `ActivationReceiptV1` payload while raw approver keys,
attestation, and detached signatures live in
`EnrollmentActivationPackageV1`; the subject signs the final exact ack. P2
WebAuthn creation/request/credential shapes are fully enumerated with empty v1
extension results. Profile conflict mutations/fields/digests are closed and
distinguish omitted, null, and value. P4's migration is uniformly 0010.

## Acceptance criteria

- [ ] Empty-server startup applies ordered server migrations once, enables WAL
      and foreign keys, preserves data across restart, bounds busy/cleanup work,
      and refuses an unknown future schema without destructive fallback.
- [ ] Bootstrap token plaintext is emitted once, only its SHA-256 digest persists,
      the ceremony expires after ten minutes, and user + Passkey + recovery-code
      hashes + portable-Vault-v1 Daemon trust anchor activate atomically or not
      at all; P2 first verifies migration 0008, the separate <=4-KiB
      `DeviceKeyEnvelopeV1`/mapping transaction, exact prepare/prove/activate
      actor, and public bootstrap commit receipt with no activation secret;
      local-only pre-user token rotation cannot reset an initialized or running
      server.
- [ ] A pure Web client cannot become the initial trust anchor; incomplete,
      expired, replayed, origin/RP-ID-mismatched, or concurrent bootstrap
      attempts leave no partial active identity and produce redacted audit facts.
- [ ] Passkey registration/authentication verifies challenge, origin, RP ID,
      user presence/verification policy, exact `0->0`, `0->N`, and increasing
      counter success plus equal/regressed nonzero counter clone handling under
      CAS, session revocation, and replay;
      production config rejects insecure or mutable RP-ID/origin settings.
- [ ] Auth sessions use `Secure`, `HttpOnly`, and appropriate `SameSite` cookies,
      with exact name `__Host-mad_session`, `Path=/`, and no `Domain`,
      issue/rotate a memory-only 32-byte CSRF value whose digest is stored, and
      enforce the frozen pre-auth/authenticated/Device Origin/Fetch-Metadata/
      JSON/cookie/CSRF matrix.
- [ ] Recovery uses exactly ten `MAD-RC1-` codes of 20 random bytes with strict
      Base32 parsing, 16-byte salts/frozen Argon2id, atomic one-time consume and
      recent-UV rotation; Passkey list/delete preserves the last key, and
      replacement exits recovery by rotating/revoking browser sessions.
- [ ] Enrollment validates Device kind/capabilities, key formats, exact
      Ed25519/X25519 proof-of-possession transcript, challenge
      signature, expiry, one-time use, fingerprint derivation, pinned approver,
      attestation signature, and exact key digests before activation.
- [ ] Duplicate/replayed enrollment, changed keys, invalid/unpinned approver,
      digest mismatch, unsupported device capability, and revoked identity all
      fail closed with stable error codes and redacted audit events.
- [ ] Candidate Daemon/Web and anchor actor sequences persist pins before
      signing/activation, verify the public approver-signed activation receipt,
      return no connection secret, support cancel/resume/idempotency and strict
      TTY/noninteractive fingerprint confirmation; versioned unknown
      capabilities are preserved-but-ineffective and same-key elevation needs a
      monotonic directly pinned capability attestation.
- [ ] Presence transitions survive reconnect and restart without falsely
      treating an unauthenticated socket as online; lifecycle and presence are
      separate and online requires active lifecycle, the current server boot
      epoch, and `lastSeenAt` no older than 60 seconds; revocation immediately
      closes active authenticated connections and blocks later reads/writes.
- [ ] OpenAPI is the REST type authority; generated Go server/client and
      TypeScript types plus the first-party exhaustive runtime client are
      reproducible/type-safe, and exact `api:generate`/temp-byte `api:verify`
      plus tool-graph license scan reject drift.
- [ ] Every mutation honors `Idempotency-Key`; updates/deletes require
      `If-Match`; stale revisions return `409 sync_conflict` with bounded
      field-level differences and never silently overwrite server state.
- [ ] Session Command creation returns `202 + commandId`; retries are idempotent;
      only an authenticated eligible target can claim/ack it; offline, expired,
      unsupported, duplicate, and daemon-restart paths have durable typed states
      including reserved-only attempt rebind, later-state reconciliation, and
      receipt ambiguity, and never claim synchronous/exactly-once Provider
      success or repeat an uncertain local execution.
- [ ] `start|resume` commands bind a server-preallocated UUIDv7
      `resultSessionId`; command delivery/query, append-only delivery revision,
      bounded attempt/retention, receipt oneOf, per-kind restart proof, stable
      outcome allowlist, and deterministic derived local-call idempotency are
      executable and never send the browser's creation Idempotency-Key to a
      Daemon. `acquire_control|release_control` remain typed unsupported.
- [ ] Sync outbox/inbox processing is ordered, bounded, transactional, and
      idempotent; cursors advance only after commit; replay creates no duplicate
      resource; secret fields and credential grants are rejected from this path.
- [ ] Device-origin rows commit mapping before push; server-created target
      Profiles allocate a correct local prefixed ID + mapping only on the target
      Daemon; missing parents, wrong ownership/type/binding, backup replay, or
      mapping collision quarantine and block ack rather than overwrite.
- [ ] Tombstones carry resource type/id/revision/digest/deletion time, propagate
      to eligible devices, and leave lifetime deletion watermarks after payload
      GC; domain-separated RFC 8785 typed revision/create digests plus exact
      `fullBase/fullNext/patch` wire make create, history-missing, nested-map,
      and atomic-array diffs reproducible, and initial/re-enrolled/restored
      Devices finish a targeted snapshot before incrementals; the target Device
      is an out-of-band enrolled/authenticated prerequisite, resources start at
      Account, and exact RFC 8785 manifest/page/final digests reject mixed epoch,
      reorder, omission, duplication, truncation, replay substitution, expiry,
      or conflicting commit.
- [ ] Overview, Devices, Accounts, Profiles, Sessions, and Usage pages render
      only allowed metadata and correct loading/empty/error/offline/revoked/
      conflict states; Usage always shows source, confidence, and observation
      time with explicit unit/scale and never fabricates a zero, dollar value,
      or official Claude quota; Overview is a bounded server aggregate rather
      than client-side unbounded pagination.
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
- Atomic bootstrap spans authentication material and a portable-Vault-backed
  Daemon trust
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
  proves otherwise. Visual polish cannot substitute for generated types plus
  the first-party typed runtime client,
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

Next role: `feature-review v7` for plan v0.7.
