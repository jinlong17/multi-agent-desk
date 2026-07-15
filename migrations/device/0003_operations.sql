CREATE TABLE audit_events (
    id TEXT PRIMARY KEY,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (length(action) BETWEEN 1 AND 128),
    target_type TEXT NOT NULL CHECK (length(target_type) BETWEEN 1 AND 64),
    target_id TEXT NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('allowed', 'denied', 'failed')),
    error_code TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL CHECK (json_valid(metadata_json)),
    created_at TEXT NOT NULL
);

CREATE INDEX audit_events_created_at ON audit_events(created_at);

CREATE TABLE idempotency_records (
    client_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE CASCADE,
    method TEXT NOT NULL CHECK (length(method) BETWEEN 1 AND 128),
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    request_digest TEXT NOT NULL CHECK (length(request_digest) = 64),
    response_code TEXT NOT NULL CHECK (length(response_code) BETWEEN 1 AND 64),
    response_metadata_json TEXT NOT NULL CHECK (json_valid(response_metadata_json)),
    created_at TEXT NOT NULL,
    PRIMARY KEY (client_id, method, idempotency_key)
);

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
