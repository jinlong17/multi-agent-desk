CREATE TABLE device_identity (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind = 'daemon'),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 128),
    signing_public_key BLOB NOT NULL CHECK (length(signing_public_key) = 32),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE client_identities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    public_key BLOB NOT NULL CHECK (length(public_key) = 32),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    status TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
    capabilities_json TEXT NOT NULL CHECK (json_valid(capabilities_json)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX client_identities_public_key
    ON client_identities(public_key);

CREATE TABLE workspaces (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    path TEXT NOT NULL CHECK (length(path) BETWEEN 1 AND 4096),
    label TEXT NOT NULL CHECK (length(label) <= 256),
    tags_json TEXT NOT NULL CHECK (json_valid(tags_json)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (device_id, path)
);

CREATE TABLE runtime_profiles (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    provider TEXT NOT NULL CHECK (provider = 'fake'),
    settings_json TEXT NOT NULL CHECK (json_valid(settings_json)),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (device_id, name)
);

CREATE TABLE credential_instances (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider = 'fake'),
    auth_method TEXT NOT NULL CHECK (auth_method = 'fake'),
    secret_ref TEXT NOT NULL CHECK (length(secret_ref) BETWEEN 1 AND 512),
    status TEXT NOT NULL CHECK (status IN ('healthy', 'expired', 'revoked', 'unknown')),
    credential_revision INTEGER NOT NULL CHECK (credential_revision >= 0),
    secret_digest TEXT NOT NULL CHECK (length(secret_digest) = 64),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
