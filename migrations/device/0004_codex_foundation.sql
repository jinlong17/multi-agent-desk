-- Phase 2 P0: expand the Device store without weakening the Phase 1 data
-- contract. The table rebuilds are intentionally contained in the migration
-- transaction owned by storage.Store.
PRAGMA defer_foreign_keys = ON;

CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider = 'codex'),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 128),
    provider_subject_digest TEXT CHECK (provider_subject_digest IS NULL OR length(provider_subject_digest) = 64),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Rename children first so SQLite rewrites their foreign keys to the
-- temporary session table while the parent tables are rebuilt.
ALTER TABLE session_attachments RENAME TO session_attachments_v3;
ALTER TABLE controller_leases RENAME TO controller_leases_v3;
ALTER TABLE session_events RENAME TO session_events_v3;
ALTER TABLE sessions RENAME TO sessions_v3;
ALTER TABLE credential_materializations RENAME TO credential_materializations_v3;
ALTER TABLE credential_instances RENAME TO credential_instances_v3;
ALTER TABLE runtime_profiles RENAME TO runtime_profiles_v3;

DROP INDEX sessions_status_started_at;
DROP INDEX sessions_resumed_from;

CREATE TABLE runtime_profiles (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    account_id TEXT REFERENCES accounts(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'codex')),
    settings_json TEXT NOT NULL CHECK (json_valid(settings_json)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (provider <> 'codex' OR account_id IS NOT NULL),
    UNIQUE (device_id, name)
);

INSERT INTO runtime_profiles(
    id, device_id, account_id, name, provider, settings_json, created_at, updated_at
)
SELECT id, device_id, NULL, name, provider, settings_json, created_at, updated_at
FROM runtime_profiles_v3;

CREATE TABLE credential_instances (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    account_id TEXT REFERENCES accounts(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'codex')),
    auth_method TEXT NOT NULL CHECK (auth_method IN ('fake', 'interactive', 'device_code')),
    secret_ref TEXT NOT NULL CHECK (length(secret_ref) BETWEEN 1 AND 512),
    status TEXT NOT NULL CHECK (status IN ('healthy', 'expired', 'revoked', 'unknown')),
    credential_revision INTEGER NOT NULL CHECK (credential_revision >= 0),
    secret_digest TEXT NOT NULL CHECK (length(secret_digest) = 64),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (
        (provider = 'fake' AND auth_method = 'fake' AND account_id IS NULL
            AND secret_ref LIKE 'fake:%' AND credential_revision >= 0)
        OR (provider = 'codex' AND auth_method IN ('interactive', 'device_code')
            AND account_id IS NOT NULL AND secret_ref LIKE 'vault:%' AND credential_revision >= 1)
    )
);

INSERT INTO credential_instances(
    id, device_id, account_id, provider, auth_method, secret_ref, status,
    credential_revision, secret_digest, created_at, updated_at
)
SELECT id, device_id, NULL, provider, auth_method, secret_ref, status,
       credential_revision, secret_digest, created_at, updated_at
FROM credential_instances_v3;

CREATE TABLE credential_materializations (
    lease_id TEXT PRIMARY KEY,
    credential_instance_id TEXT NOT NULL REFERENCES credential_instances(id) ON DELETE RESTRICT,
    credential_revision INTEGER NOT NULL CHECK (credential_revision >= 1),
    manifest_version INTEGER NOT NULL CHECK (manifest_version = 1),
    manifest_digest TEXT NOT NULL CHECK (length(manifest_digest) = 64),
    state TEXT NOT NULL CHECK (state IN ('pending', 'active', 'quarantined', 'released')),
    ref_count INTEGER NOT NULL CHECK (ref_count >= 0),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (credential_instance_id, credential_revision)
);

INSERT INTO credential_materializations(
    lease_id, credential_instance_id, credential_revision, manifest_version,
    manifest_digest, state, ref_count, created_at, updated_at
)
SELECT lease_id, credential_instance_id, credential_revision, manifest_version,
       manifest_digest, state, ref_count, created_at, updated_at
FROM credential_materializations_v3;

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    account_id TEXT REFERENCES accounts(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'codex')),
    credential_instance_id TEXT NOT NULL REFERENCES credential_instances(id) ON DELETE RESTRICT,
    runtime_profile_id TEXT NOT NULL REFERENCES runtime_profiles(id) ON DELETE RESTRICT,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
    provider_session_id TEXT,
    resumed_from_session_id TEXT REFERENCES sessions(id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('starting', 'running', 'stopping', 'exited', 'failed', 'killed')),
    started_at TEXT NOT NULL,
    ended_at TEXT,
    exit_code INTEGER,
    capability_snapshot_json TEXT NOT NULL CHECK (json_valid(capability_snapshot_json)),
    failure_code TEXT NOT NULL DEFAULT '',
    CHECK ((status IN ('exited', 'failed', 'killed') AND ended_at IS NOT NULL)
        OR (status IN ('starting', 'running', 'stopping') AND ended_at IS NULL)),
    CHECK (status = 'failed' OR failure_code = ''),
    CHECK (provider <> 'codex' OR account_id IS NOT NULL)
);

INSERT INTO sessions(
    id, device_id, account_id, provider, credential_instance_id,
    runtime_profile_id, workspace_id, provider_session_id,
    resumed_from_session_id, status, started_at, ended_at, exit_code,
    capability_snapshot_json, failure_code
)
SELECT id, device_id, NULL, provider, credential_instance_id,
       runtime_profile_id, workspace_id, provider_session_id,
       resumed_from_session_id, status, started_at, ended_at, exit_code,
       capability_snapshot_json, failure_code
FROM sessions_v3;

CREATE TABLE session_attachments (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    client_device_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    mode TEXT NOT NULL CHECK (mode IN ('observer', 'controller')),
    connected_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    UNIQUE (session_id, client_device_id)
);

INSERT INTO session_attachments(id, session_id, client_device_id, mode, connected_at, last_seen_at)
SELECT id, session_id, client_device_id, mode, connected_at, last_seen_at
FROM session_attachments_v3;

CREATE TABLE controller_leases (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    holder_device_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    lease_revision INTEGER NOT NULL CHECK (lease_revision >= 1),
    expires_at TEXT NOT NULL,
    last_heartbeat_at TEXT NOT NULL,
    released_at TEXT
);

INSERT INTO controller_leases(
    session_id, holder_device_id, lease_revision, expires_at, last_heartbeat_at, released_at
)
SELECT session_id, holder_device_id, lease_revision, expires_at, last_heartbeat_at, released_at
FROM controller_leases_v3;

CREATE TABLE session_events (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    kind TEXT NOT NULL CHECK (length(kind) BETWEEN 1 AND 64),
    metadata_json TEXT NOT NULL CHECK (json_valid(metadata_json)),
    created_at TEXT NOT NULL,
    UNIQUE (session_id, sequence)
);

INSERT INTO session_events(id, session_id, sequence, kind, metadata_json, created_at)
SELECT id, session_id, sequence, kind, metadata_json, created_at
FROM session_events_v3;

DROP TABLE session_attachments_v3;
DROP TABLE controller_leases_v3;
DROP TABLE session_events_v3;
DROP TABLE credential_materializations_v3;
DROP TABLE sessions_v3;
DROP TABLE credential_instances_v3;
DROP TABLE runtime_profiles_v3;

CREATE INDEX sessions_status_started_at ON sessions(status, started_at);
CREATE INDEX sessions_resumed_from ON sessions(resumed_from_session_id);

CREATE TABLE approvals (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    provider_approval_id TEXT NOT NULL CHECK (length(provider_approval_id) BETWEEN 1 AND 256),
    kind TEXT NOT NULL CHECK (length(kind) BETWEEN 1 AND 64),
    payload_digest TEXT NOT NULL CHECK (length(payload_digest) = 64),
    summary TEXT NOT NULL CHECK (length(summary) <= 2048),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'cancelled')),
    responded_by_device_id TEXT REFERENCES client_identities(id) ON DELETE RESTRICT,
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    requested_at TEXT NOT NULL,
    responded_at TEXT,
    UNIQUE (session_id, provider_approval_id)
);

CREATE INDEX approvals_status_requested_at ON approvals(status, requested_at);
CREATE INDEX approvals_session_requested_at ON approvals(session_id, requested_at);

CREATE TABLE usage_snapshots (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider = 'codex'),
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    source TEXT NOT NULL CHECK (source IN ('official', 'cli_derived', 'local_estimate', 'unofficial')),
    confidence TEXT NOT NULL CHECK (confidence IN ('high', 'medium', 'low')),
    window_kind TEXT NOT NULL CHECK (length(window_kind) BETWEEN 1 AND 64),
    used_value REAL,
    limit_value REAL,
    used_percent REAL CHECK (used_percent IS NULL OR (used_percent >= 0 AND used_percent <= 100)),
    resets_at TEXT,
    observed_at TEXT NOT NULL,
    raw_reference_hash TEXT CHECK (raw_reference_hash IS NULL OR length(raw_reference_hash) = 64),
    source_version TEXT NOT NULL CHECK (length(source_version) BETWEEN 1 AND 128),
    capability_status TEXT NOT NULL CHECK (capability_status IN ('supported', 'unavailable', 'schema_changed', 'error')),
    error_code TEXT NOT NULL DEFAULT '' CHECK (length(error_code) <= 64)
);

CREATE INDEX usage_snapshots_account_observed_at ON usage_snapshots(account_id, observed_at);
