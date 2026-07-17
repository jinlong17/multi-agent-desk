# Device SQLite migrations

Files are immutable, ordered by their four-digit prefix, embedded into the Go
binary, and applied in one explicit transaction per migration. The Device store
records each filename and SHA-256 checksum in `schema_migrations` and refuses a
missing, changed, non-contiguous, or future migration without modifying data.

Phase 1 starts with:

1. `0001_device_identity.sql` — Device, local clients, Workspace, Profile, and
   fake CredentialInstance metadata.
2. `0002_sessions.sql` — Session, Attachment, ControllerLease, and structural
   runtime events.
3. `0003_operations.sql` — bounded audit/idempotency metadata and the pending
   fake materialization journal used by later approved Phase 1 phases.
4. `0004_codex_foundation.sql` — transactional Provider allowlist expansion,
   local Codex Account/Approval/Usage metadata, and the legacy-compatible
   Device table rebuilds required by Phase 2 P0.
5. `0005_codex_vault_and_approval_dispatch.sql` — schema-only portable Vault
   v1 envelopes, fail-closed credential-revocation reservations, owner-bound
   Codex enrollment state, and durable two-stage Approval dispatch metadata for
   Phase 2 P2B.
6. `0006_accounts_usage.sql` — Phase 2-compatible public Codex/Claude registry
   metadata, explicit profile aliases and revisions, generic Usage windows,
   deterministic internal Fake backfill, replay deduplication, and tombstones;
   it creates no real Provider credential or runtime home.
7. `0007_codex_selector_confirmation.sql` — owner-bound explicit Codex login
   attestation, revision-pinned confirmation metadata, and persistent random
   single-use Session start previews with atomic Session reservation/replay.

No migration stores real Provider credentials or terminal payloads.
