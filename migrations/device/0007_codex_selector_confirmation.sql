-- Codex selector P1: explicit enrollment attestation and owner-bound,
-- single-use Session start previews. This migration stores only internal
-- opaque identifiers, revisions, fingerprints, digests, and timestamps.
PRAGMA defer_foreign_keys = ON;

ALTER TABLE auth_enrollments RENAME TO auth_enrollments_v6;
DROP INDEX auth_enrollments_one_active_profile;
DROP INDEX auth_enrollments_client_begin_digest;

CREATE TABLE auth_enrollments (
    id TEXT PRIMARY KEY,
    client_device_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    runtime_profile_id TEXT NOT NULL REFERENCES runtime_profiles(id) ON DELETE RESTRICT,
    credential_instance_id TEXT REFERENCES credential_instances(id) ON DELETE RESTRICT,
    binary_fingerprint TEXT NOT NULL CHECK (length(binary_fingerprint) = 64),
    staging_path TEXT NOT NULL CHECK (length(staging_path) BETWEEN 1 AND 4096),
    state TEXT NOT NULL CHECK (state IN ('begun', 'validating', 'awaiting_confirmation', 'succeeded', 'cancelled', 'expired', 'failed')),
    idempotency_digest TEXT NOT NULL CHECK (length(idempotency_digest) = 64),
    completion_idempotency_digest TEXT CHECK (completion_idempotency_digest IS NULL OR length(completion_idempotency_digest) = 64),
    confirmation_account_id TEXT REFERENCES accounts(id) ON DELETE RESTRICT,
    confirmation_account_revision INTEGER CHECK (confirmation_account_revision IS NULL OR confirmation_account_revision >= 1),
    confirmation_profile_id TEXT REFERENCES runtime_profiles(id) ON DELETE RESTRICT,
    confirmation_profile_revision INTEGER CHECK (confirmation_profile_revision IS NULL OR confirmation_profile_revision >= 1),
    confirmation_credential_id TEXT REFERENCES credential_instances(id) ON DELETE RESTRICT,
    confirmation_credential_revision INTEGER CHECK (confirmation_credential_revision IS NULL OR confirmation_credential_revision >= 1),
    confirmation_alias_digest TEXT CHECK (confirmation_alias_digest IS NULL OR length(confirmation_alias_digest) = 64),
    confirmed_by_client_id TEXT REFERENCES client_identities(id) ON DELETE RESTRICT,
    confirmed_at TEXT,
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (
        (confirmation_account_id IS NULL AND confirmation_account_revision IS NULL
            AND confirmation_profile_id IS NULL AND confirmation_profile_revision IS NULL
            AND confirmation_credential_id IS NULL AND confirmation_credential_revision IS NULL
            AND confirmation_alias_digest IS NULL)
        OR (confirmation_account_id IS NOT NULL AND confirmation_account_revision IS NOT NULL
            AND confirmation_profile_id IS NOT NULL AND confirmation_profile_revision IS NOT NULL
            AND confirmation_credential_id IS NOT NULL AND confirmation_credential_revision IS NOT NULL
            AND confirmation_alias_digest IS NOT NULL)
    ),
    CHECK ((confirmed_by_client_id IS NULL AND confirmed_at IS NULL)
        OR (confirmed_by_client_id IS NOT NULL AND confirmed_at IS NOT NULL))
);

INSERT INTO auth_enrollments(
    id, client_device_id, runtime_profile_id, credential_instance_id,
    binary_fingerprint, staging_path, state, idempotency_digest,
    completion_idempotency_digest, expires_at, created_at, updated_at
)
SELECT id, client_device_id, runtime_profile_id, credential_instance_id,
       binary_fingerprint, staging_path, state, idempotency_digest,
       completion_idempotency_digest, expires_at, created_at, updated_at
FROM auth_enrollments_v6;

DROP TABLE auth_enrollments_v6;

CREATE UNIQUE INDEX auth_enrollments_one_active_profile
ON auth_enrollments(runtime_profile_id)
WHERE state IN ('begun', 'validating', 'awaiting_confirmation');

CREATE UNIQUE INDEX auth_enrollments_client_begin_digest
ON auth_enrollments(client_device_id, idempotency_digest);

CREATE TABLE session_start_previews (
    id TEXT PRIMARY KEY,
    client_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider = 'codex'),
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    account_revision INTEGER NOT NULL CHECK (account_revision >= 1),
    runtime_profile_id TEXT NOT NULL REFERENCES runtime_profiles(id) ON DELETE RESTRICT,
    profile_revision INTEGER NOT NULL CHECK (profile_revision >= 1),
    credential_instance_id TEXT NOT NULL REFERENCES credential_instances(id) ON DELETE RESTRICT,
    credential_revision INTEGER NOT NULL CHECK (credential_revision >= 1),
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
    workspace_updated_at TEXT NOT NULL,
    usage_snapshot_id TEXT REFERENCES usage_snapshots(id) ON DELETE RESTRICT,
    provider_version TEXT NOT NULL CHECK (length(provider_version) BETWEEN 1 AND 128),
    binary_fingerprint TEXT NOT NULL CHECK (length(binary_fingerprint) = 64),
    schema_fingerprint TEXT NOT NULL CHECK (length(schema_fingerprint) = 64),
    capability_digest TEXT NOT NULL CHECK (length(capability_digest) = 64),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    consumed_at TEXT,
    consumed_request_digest TEXT CHECK (consumed_request_digest IS NULL OR length(consumed_request_digest) = 64),
    session_id TEXT REFERENCES sessions(id) ON DELETE RESTRICT,
    CHECK ((consumed_at IS NULL AND consumed_request_digest IS NULL AND session_id IS NULL)
        OR (consumed_at IS NOT NULL AND consumed_request_digest IS NOT NULL AND session_id IS NOT NULL))
);

CREATE INDEX session_start_previews_expiry
    ON session_start_previews(expires_at, consumed_at);
CREATE INDEX session_start_previews_client_created
    ON session_start_previews(client_id, created_at DESC);
