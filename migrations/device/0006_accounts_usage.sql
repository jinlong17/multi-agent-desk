-- Multi-account P1 reconciliation on top of the shipped Phase 2 schema.
-- Store.applyMigration performs the version-6 preflight, suspends foreign-key
-- enforcement for this transaction, and requires PRAGMA foreign_key_check to
-- pass before commit. No Provider credential or runtime home is created here.
PRAGMA defer_foreign_keys = ON;

CREATE TABLE accounts_v6 (
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
    CHECK ((provider = 'fake' AND internal = 1)
        OR (provider IN ('codex', 'claude') AND internal = 0))
);

INSERT INTO accounts_v6(
    id, provider, display_name, provider_subject_digest, subscription_hint,
    internal, enabled, revision, created_at, updated_at
)
SELECT id, provider, display_name, coalesce(provider_subject_digest, ''), '',
       0, enabled, 1, created_at, updated_at
FROM accounts;

INSERT INTO accounts_v6(
    id, provider, display_name, provider_subject_digest, subscription_hint,
    internal, enabled, revision, created_at, updated_at
)
SELECT 'account_' || substr(id, 8), 'fake', 'Legacy Fake', '', '',
       1, 1, 1, created_at, updated_at
FROM device_identity;

DROP TABLE accounts;
ALTER TABLE accounts_v6 RENAME TO accounts;

CREATE TABLE runtime_profiles_v6 (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    credential_instance_id TEXT REFERENCES credential_instances(id) ON DELETE RESTRICT,
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
    UNIQUE (device_id, name),
    CHECK ((selector_alias IS NULL AND selector_key IS NULL)
        OR (length(selector_alias) BETWEEN 1 AND 32
            AND length(selector_key) BETWEEN 1 AND 32)),
    CHECK ((provider = 'fake' AND internal = 1)
        OR (provider IN ('codex', 'claude') AND internal = 0))
);

INSERT INTO runtime_profiles_v6(
    id, device_id, account_id, credential_instance_id, name, provider,
    selector_alias, selector_key, settings_json, internal, enabled, revision,
    created_at, updated_at
)
SELECT id, device_id,
       coalesce(account_id, 'account_' || substr(device_id, 8)),
       NULL, name, provider, NULL, NULL, settings_json,
       CASE WHEN provider = 'fake' THEN 1 ELSE 0 END,
       1, 1, created_at, updated_at
FROM runtime_profiles;

DROP TABLE runtime_profiles;
ALTER TABLE runtime_profiles_v6 RENAME TO runtime_profiles;

CREATE UNIQUE INDEX runtime_profiles_selector_key
    ON runtime_profiles(selector_key) WHERE internal = 0 AND selector_key IS NOT NULL;
CREATE INDEX runtime_profiles_account_created
    ON runtime_profiles(account_id, created_at, id);

UPDATE sessions
SET account_id = 'account_' || substr(device_id, 8)
WHERE provider = 'fake' AND account_id IS NULL;
CREATE INDEX IF NOT EXISTS sessions_account_status ON sessions(account_id, status);

CREATE TABLE usage_snapshots_v6 (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider IN ('codex', 'claude')),
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    credential_instance_id TEXT REFERENCES credential_instances(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    source TEXT NOT NULL CHECK (source IN ('official', 'cli_derived', 'local_estimate', 'unofficial', 'unavailable')),
    confidence TEXT NOT NULL CHECK (confidence IN ('high', 'medium', 'low', 'none')),
    window_kind TEXT NOT NULL DEFAULT 'structured' CHECK (length(window_kind) BETWEEN 1 AND 64),
    used_value REAL CHECK (used_value IS NULL OR used_value >= 0),
    limit_value REAL CHECK (limit_value IS NULL OR limit_value >= 0),
    used_percent REAL CHECK (used_percent IS NULL OR used_percent BETWEEN 0 AND 100),
    resets_at TEXT,
    observed_at TEXT NOT NULL,
    raw_reference_hash TEXT NOT NULL DEFAULT '' CHECK (length(raw_reference_hash) <= 128),
    source_version TEXT NOT NULL DEFAULT '' CHECK (length(source_version) <= 128),
    capability_status TEXT NOT NULL DEFAULT 'unavailable'
        CHECK (capability_status IN ('supported', 'unavailable', 'schema_changed', 'error')),
    error_code TEXT NOT NULL DEFAULT '' CHECK (length(error_code) <= 64),
    provider_version TEXT NOT NULL CHECK (length(provider_version) BETWEEN 1 AND 128),
    availability TEXT NOT NULL CHECK (availability IN ('available', 'limited', 'unavailable', 'unknown')),
    stale_at TEXT NOT NULL
);

INSERT INTO usage_snapshots_v6(
    id, provider, account_id, credential_instance_id, device_id, source,
    confidence, window_kind, used_value, limit_value, used_percent, resets_at,
    observed_at, raw_reference_hash, source_version, capability_status,
    error_code, provider_version, availability, stale_at
)
SELECT id, provider, account_id, NULL, device_id, source, confidence,
       window_kind, used_value, limit_value, used_percent, resets_at,
       observed_at, coalesce(raw_reference_hash, ''), source_version,
       capability_status, error_code, source_version,
       CASE WHEN capability_status = 'supported' THEN 'available' ELSE 'unknown' END,
       observed_at
FROM usage_snapshots;

DROP TABLE usage_snapshots;
ALTER TABLE usage_snapshots_v6 RENAME TO usage_snapshots;

CREATE INDEX usage_snapshots_account_observed_at
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
