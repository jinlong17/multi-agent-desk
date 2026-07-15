CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
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

CREATE INDEX sessions_status_started_at ON sessions(status, started_at);
CREATE INDEX sessions_resumed_from ON sessions(resumed_from_session_id);

CREATE TABLE session_attachments (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    client_device_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    mode TEXT NOT NULL CHECK (mode IN ('observer', 'controller')),
    connected_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    UNIQUE (session_id, client_device_id)
);

CREATE TABLE controller_leases (
    session_id TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    holder_device_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    lease_revision INTEGER NOT NULL CHECK (lease_revision >= 1),
    expires_at TEXT NOT NULL,
    last_heartbeat_at TEXT NOT NULL,
    released_at TEXT
);

CREATE TABLE session_events (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    kind TEXT NOT NULL CHECK (length(kind) BETWEEN 1 AND 64),
    metadata_json TEXT NOT NULL CHECK (json_valid(metadata_json)),
    created_at TEXT NOT NULL,
    UNIQUE (session_id, sequence)
);
