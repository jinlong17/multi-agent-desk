-- Phase 4a P2: atomic initial Daemon bootstrap, Passkeys, recovery codes, and
-- browser sessions. Device enrollment, projections, sync, commands, and
-- realtime tables intentionally remain absent.
-- P1 wrote RFC3339Nano values with variable-width fractional seconds. P2 uses
-- fixed-width microseconds so SQLite TEXT ordering has the same order as time.
UPDATE idempotency_records
SET created_at = CASE
        WHEN instr(created_at,'.') = 0 THEN substr(created_at,1,19) || '.000000Z'
        ELSE substr(created_at,1,instr(created_at,'.')) || substr(substr(created_at,instr(created_at,'.')+1,instr(created_at,'Z')-instr(created_at,'.')-1) || '000000',1,6) || 'Z'
    END,
    expires_at = CASE
        WHEN instr(expires_at,'.') = 0 THEN substr(expires_at,1,19) || '.000000Z'
        ELSE substr(expires_at,1,instr(expires_at,'.')) || substr(substr(expires_at,instr(expires_at,'.')+1,instr(expires_at,'Z')-instr(expires_at,'.')-1) || '000000',1,6) || 'Z'
    END;

CREATE TABLE bootstrap_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    token_digest BLOB CHECK (token_digest IS NULL OR length(token_digest) = 32),
    token_expires_at TEXT,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK ((token_digest IS NULL) = (token_expires_at IS NULL))
) STRICT;

CREATE TABLE users (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    singleton INTEGER NOT NULL UNIQUE CHECK (singleton = 1),
    user_handle BLOB NOT NULL UNIQUE CHECK (length(user_handle) BETWEEN 1 AND 64),
    display_name TEXT NOT NULL CHECK (length(display_name) BETWEEN 1 AND 128),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE passkeys (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BLOB NOT NULL UNIQUE CHECK (length(credential_id) BETWEEN 1 AND 1024),
    credential_json BLOB NOT NULL CHECK (length(credential_json) BETWEEN 2 AND 1048576),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    transports_json TEXT NOT NULL CHECK (json_valid(transports_json) AND length(transports_json) <= 1024),
    sign_count INTEGER NOT NULL CHECK (sign_count >= 0),
    credential_revision INTEGER NOT NULL CHECK (credential_revision >= 1),
    active INTEGER NOT NULL CHECK (active IN (0,1)),
    created_at TEXT NOT NULL,
    last_used_at TEXT,
    updated_at TEXT NOT NULL
) STRICT;
CREATE INDEX passkeys_user_active ON passkeys(user_id,active,created_at);

CREATE TABLE webauthn_ceremonies (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    kind TEXT NOT NULL CHECK (kind IN ('bootstrap_registration','passkey_login','passkey_registration','recent_uv')),
    payload_json BLOB NOT NULL CHECK (json_valid(payload_json) AND length(payload_json) BETWEEN 2 AND 1048576),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL
) STRICT;
CREATE INDEX webauthn_ceremonies_expiry ON webauthn_ceremonies(expires_at,id);

CREATE TABLE recovery_batches (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL CHECK (status IN ('active','rotated','exhausted')),
    created_at TEXT NOT NULL,
    invalidated_at TEXT
) STRICT;
CREATE UNIQUE INDEX recovery_batches_one_active ON recovery_batches(user_id) WHERE status='active';

CREATE TABLE recovery_codes (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    batch_id TEXT NOT NULL REFERENCES recovery_batches(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ordinal INTEGER NOT NULL CHECK (ordinal BETWEEN 1 AND 10),
    salt BLOB NOT NULL CHECK (length(salt) = 16),
    code_hash BLOB NOT NULL CHECK (length(code_hash) = 32),
    status TEXT NOT NULL CHECK (status IN ('active','consumed','invalidated')),
    created_at TEXT NOT NULL,
    consumed_at TEXT,
    UNIQUE (batch_id,ordinal)
) STRICT;
CREATE INDEX recovery_codes_user_status ON recovery_codes(user_id,status,batch_id,ordinal);

CREATE TABLE anchor_devices (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    kind TEXT NOT NULL CHECK (kind = 'daemon'),
    name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 128),
    platform TEXT NOT NULL CHECK (platform IN ('darwin','linux','windows')),
    architecture TEXT NOT NULL CHECK (length(architecture) BETWEEN 1 AND 32),
    client_version TEXT NOT NULL CHECK (length(client_version) BETWEEN 1 AND 64),
    signing_public_key BLOB NOT NULL CHECK (length(signing_public_key) = 32),
    exchange_public_key BLOB NOT NULL CHECK (length(exchange_public_key) = 32),
    signing_key_digest BLOB NOT NULL CHECK (length(signing_key_digest) = 32),
    exchange_key_digest BLOB NOT NULL CHECK (length(exchange_key_digest) = 32),
    pin_digest BLOB NOT NULL CHECK (length(pin_digest) = 32),
    storage_mode TEXT NOT NULL CHECK (storage_mode = 'portable_vault_v1'),
    storage_assertion_json TEXT NOT NULL CHECK (json_valid(storage_assertion_json) AND length(storage_assertion_json) <= 4096),
    storage_assertion_digest BLOB NOT NULL CHECK (length(storage_assertion_digest) = 32),
    capabilities_json TEXT NOT NULL CHECK (json_valid(capabilities_json) AND length(capabilities_json) <= 4096),
    lifecycle TEXT NOT NULL CHECK (lifecycle = 'active'),
    key_revision INTEGER NOT NULL CHECK (key_revision = 1),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE bootstrap_receipts (
    ceremony_id TEXT PRIMARY KEY CHECK (length(ceremony_id) = 36),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    anchor_device_id TEXT NOT NULL REFERENCES anchor_devices(id) ON DELETE RESTRICT,
    receipt_json TEXT NOT NULL CHECK (json_valid(receipt_json) AND length(receipt_json) <= 4096),
    receipt_digest BLOB NOT NULL CHECK (length(receipt_digest) = 32),
    created_at TEXT NOT NULL
) STRICT;

CREATE TABLE browser_sessions (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_digest BLOB NOT NULL UNIQUE CHECK (length(token_digest) = 32),
    csrf_digest BLOB NOT NULL CHECK (length(csrf_digest) = 32),
    csrf_generation INTEGER NOT NULL CHECK (csrf_generation >= 1),
    authentication_method TEXT NOT NULL CHECK (authentication_method IN ('passkey','recovery')),
    authentication_passkey_id TEXT REFERENCES passkeys(id) ON DELETE SET NULL,
    authenticated_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    recent_uv_at TEXT,
    expires_at TEXT NOT NULL,
    idle_expires_at TEXT NOT NULL,
    revoked_at TEXT,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    activity_revision INTEGER NOT NULL CHECK (activity_revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK ((authentication_method='passkey' AND authentication_passkey_id IS NOT NULL)
        OR (authentication_method='recovery' AND authentication_passkey_id IS NULL))
) STRICT;
CREATE INDEX browser_sessions_user_live ON browser_sessions(user_id,revoked_at,expires_at,idle_expires_at);
CREATE INDEX browser_sessions_passkey_live ON browser_sessions(authentication_passkey_id,revoked_at);

CREATE TABLE auth_idempotency_operations (
    key_digest BLOB PRIMARY KEY CHECK (length(key_digest) = 32),
    request_identity_digest BLOB NOT NULL CHECK (length(request_identity_digest) = 32),
    actor_class TEXT NOT NULL CHECK (actor_class IN ('bootstrap_token','preauth_browser','browser_session')),
    actor_identity_digest BLOB NOT NULL CHECK (length(actor_identity_digest) = 32),
    method TEXT NOT NULL CHECK (method IN ('POST','DELETE')),
    canonical_path TEXT NOT NULL CHECK (canonical_path GLOB '/v1/*' AND canonical_path NOT LIKE '%?%' AND canonical_path NOT LIKE '%/' AND length(canonical_path) <= 256),
    body_digest BLOB NOT NULL CHECK (length(body_digest) = 32),
    canonical_if_match TEXT NOT NULL,
    operation_id TEXT NOT NULL UNIQUE CHECK (length(operation_id) = 36),
    operation TEXT NOT NULL CHECK (operation IN (
        'bootstrap_options',
        'bootstrap_verify',
        'passkey_login_options',
        'passkey_login_verify',
        'passkey_registration_options',
        'passkey_registration_verify',
        'passkey_delete',
        'uv_options',
        'uv_verify',
        'recovery_verify',
        'recovery_codes_rotate',
        'logout',
        'session_delete'
    )),
    state TEXT NOT NULL CHECK (state IN ('in_progress','committed')),
    server_boot_epoch TEXT NOT NULL CHECK (length(server_boot_epoch) = 36),
    public_receipt_json TEXT CHECK (public_receipt_json IS NULL OR (json_valid(public_receipt_json) AND length(public_receipt_json) <= 16384)),
    cookie_action TEXT NOT NULL CHECK (cookie_action IN ('none','clear','secret_issued')),
    ceremony_id TEXT CHECK (ceremony_id IS NULL OR length(ceremony_id) = 36),
    public_options_digest BLOB CHECK (public_options_digest IS NULL OR length(public_options_digest) = 32),
    created_at TEXT NOT NULL,
    committed_at TEXT,
    expires_at TEXT NOT NULL,
    CHECK ((operation IN ('passkey_delete','session_delete')
            AND method='DELETE'
            AND length(canonical_if_match) BETWEEN 7 AND 25
            AND substr(canonical_if_match,1,5)='"rev-'
            AND substr(canonical_if_match,-1,1)='"'
            AND substr(canonical_if_match,6,1) BETWEEN '1' AND '9'
            AND substr(canonical_if_match,6,length(canonical_if_match)-6) NOT GLOB '*[^0-9]*')
        OR (operation NOT IN ('passkey_delete','session_delete')
            AND method='POST'
            AND canonical_if_match='')),
    CHECK ((state='in_progress' AND public_receipt_json IS NULL AND committed_at IS NULL)
        OR (state='committed' AND public_receipt_json IS NOT NULL AND committed_at IS NOT NULL)),
    CHECK ((ceremony_id IS NULL) = (public_options_digest IS NULL))
) STRICT;
CREATE INDEX auth_idempotency_operations_cleanup ON auth_idempotency_operations(expires_at,key_digest);
CREATE UNIQUE INDEX auth_idempotency_operations_ceremony ON auth_idempotency_operations(ceremony_id) WHERE ceremony_id IS NOT NULL;

CREATE TABLE auth_audit_events (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    actor_class TEXT NOT NULL CHECK (actor_class IN ('pre_user','browser','recovery')),
    action TEXT NOT NULL CHECK (length(action) BETWEEN 1 AND 128),
    decision TEXT NOT NULL CHECK (decision IN ('allowed','denied','failed')),
    reason_code TEXT NOT NULL CHECK (length(reason_code) BETWEEN 1 AND 64),
    target_id TEXT CHECK (target_id IS NULL OR length(target_id) <= 128),
    created_at TEXT NOT NULL
) STRICT;
