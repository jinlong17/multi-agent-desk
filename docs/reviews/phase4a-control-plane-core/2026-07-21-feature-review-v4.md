# Feature Review v4: Phase 4a Control Plane Core

- Date: 2026-07-21
- Role: `feature-review`
- Owner module: `control-plane`
- Reviewed plan version: `v0.4`
- Verdict: `APPROVED`

## Conclusion

Plan v0.4 is decision-complete and approved. It closes the sole v3 finding by
making the already-enrolled, signed, active target Device an out-of-band
snapshot prerequisite, keeping `CanonicalSyncRevisionV1` as the six metadata
resource types, and changing the exact topology to Account-first. The manifest,
page, and final inputs now have strict RFC 8785 schemas, domain-separated
digests, deterministic page slicing and chaining, target/epoch/token/cursor
binding, expiry behavior, byte-identical replay, and conflict-safe commit
semantics with cross-language and hostile-path gates.

No material contract, phase-ordering, security-boundary, migration, rollback,
or verification gap remains. The workflow-valid state is `APPROVED`; this is
ready to build starting with `feature-build` P0, not a new
`READY_TO_BUILD` status.

## V3 finding closure audit

### Authoritative snapshot resource set — closed

- The signed caller must resolve to the same active, enrolled
  `targetDeviceId`; the Daemon must already have a consistent envelope, Device
  mapping, pin, and activation receipt. Device is explicitly excluded from the
  snapshot union rather than being implied by an unencodable first row.
- Snapshot resources are exactly Account, Credential status, Profile,
  Workspace, Session, and Usage, ordered by that type rank and then canonical
  lowercase UUIDv7. The latest authorized revision appears exactly once.
- `SnapshotManifestV1`, `SnapshotPageDigestInputV1`, and
  `SnapshotFinalDigestInputV1` freeze the IDs, epoch, target, counts, expiry,
  complete ordered manifest, resources, continuation, prior digest, and
  incremental base cursor needed by their three domain-separated digests.
- Page count and slices, one empty final page, first/later prior digests,
  next-versus-final continuation, persisted token binding, one active snapshot,
  ten-minute expiry, exact replay, atomic local apply, and server commit are
  deterministic.
- Tests cover independent Go/TypeScript goldens and mixed target/epoch/
  snapshot, reorder, omission, duplication, truncation, manifest/resource/
  digest mismatch, premature final, token/cursor substitution, expiry,
  conflicting commit, mapping/parent failure, and replay retention.

## P0-P6 executable-phase review

- **P0:** documentation/vector-only reconciliation is bounded and executable;
  the full pin digest, six-group display, restricted JCS attestation, both
  storage assertion mutations, dependency/toolchain pins, and prior E2EE
  negatives have explicit gates.
- **P1:** server lifecycle, migration behavior, OpenAPI 3.0.3 authority,
  generated Go server/client/models and TypeScript types, first-party exhaustive
  runtime client, middleware bounds, deterministic drift, and license checks
  are frozen without activating a user or Device.
- **P2:** migration 0008, mapping, exact portable-Vault-v1 Device envelope,
  actor-complete bootstrap, public no-secret receipt, Passkey/recovery/session,
  cookie/CSRF, token rotation, and official Daemon evidence precede bootstrap
  availability.
- **P3:** additional Device enrollment reuses the P2 foundation; Web storage
  modes, Desktop fixture-only scope, pin/attestation/activation, capability
  evolution, signed REST, presence, revocation, and snapshot-required state
  have positive, negative, restart, and concurrency gates.
- **P4:** typed projection, create/update/delete digests, deterministic patches,
  conflicts, mappings, Account-first authoritative snapshots, cursors,
  tombstones, lifetime watermarks, backup, replay, and quarantine are now
  wire-complete and independently testable.
- **P5:** asynchronous commands retain truthful `202` and at-least-once
  semantics, reserved-only attempt rebind, executing/later-state reconciliation,
  durable local identity, ambiguity refusal, and no automatic uncertain
  re-execution.
- **P6:** the metadata-only Web/PWA and Desktop render-smoke boundary has exact
  journeys, responsive/accessibility/browser gates, end-to-end scenarios,
  prohibited-capability scans, and final independent Security Review handoff.

## Boundary, rollback, and gates

- The 4a/4b/5 boundary remains truthful: Phase 4a includes no WSS, HPKE,
  Pairwise Root, terminal transport, Approval response, Credential Grant,
  Desktop key store, Provider credential path, or release claim.
- The Control Plane remains metadata-only; Provider secrets, Vault material,
  terminal/model content, Passkey private material, and recovery plaintext are
  excluded from storage, logs, audit, debug, and wire schemas.
- Migrations are forward-only and phase ordered. Rollback uses a verified
  pre-migration backup plus the prior binary, preserves local rows, envelopes,
  mappings, outboxes, and receipts, and never performs destructive reinterpretation.
- Provider Gate is correctly `none`. The Security Gate remains open and can be
  resolved only by the later independent `security-review`; this approval does
  not accept residual security risk or authorize ship.

## Findings

None.

## Evidence

- Re-read `AGENTS.md`, `CLAUDE.md`, the complete implementation plan, workflow
  policy, module registry, classification skill, `feature-review` role, the
  complete v0.4 feature brief, `design.md`, `api.md`, `test.md`, `dev_log.md`,
  and the v3 review report.
- Classified the feature as owner `control-plane` with impacts to `security`,
  `core`, `web`, `desktop`, and `project-system`; Provider Gate `none` and
  Security Gate open remain correct.
- Cross-checked the v3 snapshot finding against exact DTO membership,
  topology, digest inputs, page continuity, expiry/replay/commit behavior,
  cursor installation, restore/mapping behavior, and negative tests.
- Regressed P0-P6 phase dependencies, migration ownership, API/type authority,
  auth and crypto failure modes, Phase 4a exclusions, rollback, and build/
  verification/security handoffs.
- Coordinator-recorded v0.4 `git diff --check`, `project:verify`, and
  `ci:verify` success is acknowledged as structural baseline evidence and was
  not represented as rerun by this reviewer.

## Verdict

`APPROVED`. The next authorized lifecycle action is `feature-build` for P0.
That writer must implement only P0, record evidence, set `READY_FOR_VERIFY`,
and stop for independent `feature-verify`.
