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

No migration stores real Provider credentials or terminal payloads.
