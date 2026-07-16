CREATE TABLE accounts (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'codex', 'claude')),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 128),
    provider_subject_digest TEXT NOT NULL DEFAULT '' CHECK (length(provider_subject_digest) <= 128),
    subscription_hint TEXT NOT NULL DEFAULT '' CHECK (length(subscription_hint) <= 64),
    internal INTEGER NOT NULL CHECK (internal IN (0, 1)),
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK ((provider = 'fake' AND internal = 1) OR (provider IN ('codex', 'claude') AND internal = 0))
);

INSERT INTO accounts(
    id, provider, display_name, provider_subject_digest, subscription_hint,
    internal, enabled, revision, created_at, updated_at
)
SELECT 'account_' || substr(id, 8), 'fake', 'Legacy Fake', '', '', 1, 1, 1,
       created_at, updated_at
FROM device_identity;

CREATE TABLE credential_instances_v4 (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'codex', 'claude')),
    auth_method TEXT NOT NULL CHECK (length(auth_method) BETWEEN 1 AND 64),
    secret_ref TEXT NOT NULL CHECK (length(secret_ref) BETWEEN 1 AND 512),
    status TEXT NOT NULL CHECK (status IN ('healthy', 'expired', 'revoked', 'unknown')),
    availability TEXT NOT NULL DEFAULT 'unknown'
        CHECK (availability IN ('available', 'limited', 'unavailable', 'unknown')),
    last_validated_at TEXT,
    credential_revision INTEGER NOT NULL CHECK (credential_revision >= 0),
    secret_digest TEXT NOT NULL CHECK (length(secret_digest) = 64),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

INSERT INTO credential_instances_v4(
    id, account_id, device_id, provider, auth_method, secret_ref, status,
    availability, last_validated_at, credential_revision, secret_digest,
    created_at, updated_at
)
SELECT id, 'account_' || substr(device_id, 8), device_id, provider, auth_method,
       secret_ref, status, 'unknown', NULL, credential_revision, secret_digest,
       created_at, updated_at
FROM credential_instances;

CREATE TABLE runtime_profiles_v4 (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    credential_instance_id TEXT REFERENCES credential_instances(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    provider TEXT NOT NULL CHECK (provider IN ('fake', 'codex', 'claude')),
    selector_alias TEXT,
    selector_key TEXT,
    settings_json TEXT NOT NULL CHECK (json_valid(settings_json)),
    internal INTEGER NOT NULL CHECK (internal IN (0, 1)),
    enabled INTEGER NOT NULL CHECK (enabled IN (0, 1)),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK ((internal = 1 AND provider = 'fake' AND selector_alias IS NULL AND selector_key IS NULL)
        OR (internal = 0 AND provider IN ('codex', 'claude')
            AND length(selector_alias) BETWEEN 1 AND 32
            AND length(selector_key) BETWEEN 1 AND 32))
);

INSERT INTO runtime_profiles_v4(
    id, account_id, credential_instance_id, device_id, name, provider,
    selector_alias, selector_key, settings_json, internal, enabled, revision,
    created_at, updated_at
)
SELECT id, 'account_' || substr(device_id, 8), NULL, device_id, name, provider,
       NULL, NULL, settings_json, 1, 1, 1, created_at, updated_at
FROM runtime_profiles;

CREATE TABLE sessions_v4 (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider = 'fake'),
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
    CHECK (status = 'failed' OR failure_code = '')
);

INSERT INTO sessions_v4(
    id, account_id, device_id, provider, credential_instance_id,
    runtime_profile_id, workspace_id, provider_session_id,
    resumed_from_session_id, status, started_at, ended_at, exit_code,
    capability_snapshot_json, failure_code
)
SELECT s.id, p.account_id, s.device_id, s.provider, s.credential_instance_id,
       s.runtime_profile_id, s.workspace_id, s.provider_session_id,
       s.resumed_from_session_id, s.status, s.started_at, s.ended_at,
       s.exit_code, s.capability_snapshot_json, s.failure_code
FROM sessions AS s
JOIN runtime_profiles_v4 AS p ON p.id = s.runtime_profile_id;

CREATE TABLE migration_v4_assertions (
    valid INTEGER NOT NULL CHECK (valid = 1)
);

INSERT INTO migration_v4_assertions(valid)
SELECT CASE WHEN
    (SELECT count(*) FROM credential_instances_v4) = (SELECT count(*) FROM credential_instances)
    AND (SELECT count(*) FROM runtime_profiles_v4) = (SELECT count(*) FROM runtime_profiles)
    AND (SELECT count(*) FROM sessions_v4) = (SELECT count(*) FROM sessions)
    AND NOT EXISTS (
        SELECT 1 FROM sessions_v4 AS s
        JOIN runtime_profiles_v4 AS p ON p.id = s.runtime_profile_id
        JOIN credential_instances_v4 AS c ON c.id = s.credential_instance_id
        WHERE s.account_id <> p.account_id OR s.account_id <> c.account_id
            OR s.device_id <> p.device_id OR s.device_id <> c.device_id
            OR s.provider <> p.provider OR s.provider <> c.provider
    )
THEN 1 ELSE 0 END;

DROP TABLE migration_v4_assertions;
DROP TABLE sessions;
DROP TABLE runtime_profiles;
DROP TABLE credential_instances;

ALTER TABLE credential_instances_v4 RENAME TO credential_instances;
ALTER TABLE runtime_profiles_v4 RENAME TO runtime_profiles;
ALTER TABLE sessions_v4 RENAME TO sessions;

CREATE UNIQUE INDEX runtime_profiles_selector_key
    ON runtime_profiles(selector_key) WHERE internal = 0;
CREATE INDEX runtime_profiles_account_created
    ON runtime_profiles(account_id, created_at, id);
CREATE INDEX accounts_provider_created
    ON accounts(provider, created_at, id);
CREATE INDEX sessions_status_started_at ON sessions(status, started_at);
CREATE INDEX sessions_resumed_from ON sessions(resumed_from_session_id);
CREATE INDEX sessions_account_status ON sessions(account_id, status);

CREATE TABLE usage_snapshots (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    credential_instance_id TEXT REFERENCES credential_instances(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider IN ('codex', 'claude')),
    provider_version TEXT NOT NULL CHECK (length(provider_version) BETWEEN 1 AND 128),
    source TEXT NOT NULL CHECK (source IN ('official', 'cli_derived', 'local_estimate', 'unavailable')),
    confidence TEXT NOT NULL CHECK (confidence IN ('high', 'medium', 'low', 'none')),
    availability TEXT NOT NULL CHECK (availability IN ('available', 'limited', 'unavailable', 'unknown')),
    observed_at TEXT NOT NULL,
    stale_at TEXT NOT NULL,
    raw_reference_hash TEXT NOT NULL DEFAULT '' CHECK (length(raw_reference_hash) <= 128)
);

CREATE INDEX usage_snapshots_account_observed
    ON usage_snapshots(account_id, observed_at DESC, id DESC);
CREATE UNIQUE INDEX usage_snapshots_replay_tuple
    ON usage_snapshots(account_id, device_id, provider_version, observed_at, raw_reference_hash);

CREATE TABLE usage_windows (
    snapshot_id TEXT NOT NULL REFERENCES usage_snapshots(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    provider_limit_id TEXT NOT NULL DEFAULT '' CHECK (length(provider_limit_id) <= 128),
    kind TEXT NOT NULL CHECK (kind IN ('rolling', 'calendar', 'spend_control', 'sdk_credit', 'unknown')),
    label TEXT NOT NULL CHECK (length(label) BETWEEN 1 AND 128),
    duration_seconds INTEGER CHECK (duration_seconds > 0),
    used_value REAL CHECK (used_value >= 0),
    limit_value REAL CHECK (limit_value >= 0),
    used_percent REAL CHECK (used_percent BETWEEN 0 AND 100),
    remaining_percent REAL CHECK (remaining_percent BETWEEN 0 AND 100),
    resets_at TEXT,
    PRIMARY KEY (snapshot_id, position)
);

CREATE TABLE metadata_tombstones (
    entity_type TEXT NOT NULL CHECK (entity_type IN ('account', 'profile')),
    entity_id TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('codex', 'claude')),
    final_revision INTEGER NOT NULL CHECK (final_revision >= 1),
    deleted_at TEXT NOT NULL,
    PRIMARY KEY (entity_type, entity_id)
);
