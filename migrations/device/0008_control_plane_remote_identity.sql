-- Phase 4a P2: server-origin-bound remote Device identity envelopes and the
-- generic local/server identifier mapping. Trust, receipt history, sync, and
-- command receipt tables intentionally belong to later migrations.
PRAGMA defer_foreign_keys = ON;

CREATE TABLE remote_device_identities (
    id TEXT PRIMARY KEY
        CHECK (length(id) = 48 AND id GLOB 'remote_identity_[0-9a-f][0-9a-f]*'),
    server_origin TEXT NOT NULL CHECK (
        length(server_origin) BETWEEN 9 AND 2048
        AND substr(server_origin, 1, 8) = 'https://'
    ),
    server_device_id TEXT NOT NULL CHECK (length(server_device_id) = 36),
    signing_public_key BLOB NOT NULL CHECK (length(signing_public_key) = 32),
    exchange_public_key BLOB NOT NULL CHECK (length(exchange_public_key) = 32),
    signing_key_digest BLOB NOT NULL CHECK (length(signing_key_digest) = 32),
    exchange_key_digest BLOB NOT NULL CHECK (length(exchange_key_digest) = 32),
    key_revision INTEGER NOT NULL CHECK (key_revision = 1),
    record_revision INTEGER NOT NULL CHECK (record_revision >= 1),
    lifecycle TEXT NOT NULL CHECK (lifecycle IN ('pending', 'active', 'retired')),
    payload_algorithm TEXT NOT NULL CHECK (payload_algorithm = 'aes-256-gcm'),
    payload_nonce BLOB NOT NULL CHECK (length(payload_nonce) = 12),
    payload_ciphertext BLOB NOT NULL CHECK (length(payload_ciphertext) BETWEEN 17 AND 4112),
    wrap_algorithm TEXT NOT NULL CHECK (wrap_algorithm = 'aes-256-gcm'),
    wrap_nonce BLOB NOT NULL CHECK (length(wrap_nonce) = 12),
    wrapped_dek BLOB NOT NULL CHECK (length(wrapped_dek) = 48),
    aad_digest BLOB NOT NULL CHECK (length(aad_digest) = 32),
    plaintext_digest BLOB NOT NULL CHECK (length(plaintext_digest) = 32),
    bootstrap_receipt_json TEXT CHECK (
        bootstrap_receipt_json IS NULL OR (
            json_valid(bootstrap_receipt_json)
            AND length(bootstrap_receipt_json) BETWEEN 2 AND 4096
        )
    ),
    bootstrap_receipt_digest BLOB CHECK (
        bootstrap_receipt_digest IS NULL OR length(bootstrap_receipt_digest) = 32
    ),
    quarantine_reason TEXT CHECK (
        quarantine_reason IS NULL OR length(quarantine_reason) BETWEEN 1 AND 64
    ),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (server_origin, server_device_id),
    CHECK ((bootstrap_receipt_json IS NULL) = (bootstrap_receipt_digest IS NULL)),
    CHECK (lifecycle <> 'active' OR bootstrap_receipt_digest IS NOT NULL)
);

CREATE INDEX remote_device_identities_origin_lifecycle
    ON remote_device_identities(server_origin, lifecycle, created_at);

CREATE TABLE controlplane_id_mappings (
    entity_type TEXT NOT NULL CHECK (
        length(entity_type) BETWEEN 1 AND 64
        AND entity_type GLOB '[a-z][a-z0-9_]*'
    ),
    local_id TEXT NOT NULL CHECK (length(local_id) BETWEEN 1 AND 128),
    server_id TEXT NOT NULL CHECK (length(server_id) = 36),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (entity_type, local_id),
    UNIQUE (entity_type, server_id)
);

CREATE TRIGGER remote_device_identity_mapping_insert
BEFORE INSERT ON remote_device_identities
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM controlplane_id_mappings
        WHERE entity_type = 'device'
          AND local_id = NEW.id
          AND server_id = NEW.server_device_id
    ) THEN RAISE(ABORT, 'remote identity mapping is missing') END;
END;

CREATE TRIGGER remote_device_identity_mapping_update
BEFORE UPDATE OF id, server_device_id ON remote_device_identities
BEGIN
    SELECT CASE WHEN NEW.id <> OLD.id OR NEW.server_device_id <> OLD.server_device_id
        THEN RAISE(ABORT, 'remote identity mapping is immutable') END;
END;
