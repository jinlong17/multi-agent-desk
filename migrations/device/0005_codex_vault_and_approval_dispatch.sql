-- Phase 2 P2B: encrypted local Vault, interactive enrollment metadata, and
-- durable Approval dispatch state. Vault config is intentionally empty after
-- migration; vault.initialize is the only password-bound first-use writer.
PRAGMA defer_foreign_keys = ON;

CREATE TABLE vault_config (
    singleton_id INTEGER PRIMARY KEY CHECK (singleton_id = 1),
    format_version INTEGER NOT NULL CHECK (format_version = 1),
    kdf_name TEXT NOT NULL CHECK (kdf_name = 'argon2id-v19'),
    kdf_salt BLOB NOT NULL CHECK (length(kdf_salt) = 16),
    argon_time INTEGER NOT NULL CHECK (argon_time = 3),
    argon_memory_kib INTEGER NOT NULL CHECK (argon_memory_kib = 65536),
    argon_parallelism INTEGER NOT NULL CHECK (argon_parallelism BETWEEN 1 AND 4),
    key_check_nonce BLOB NOT NULL CHECK (length(key_check_nonce) = 12),
    key_check_ciphertext BLOB NOT NULL CHECK (length(key_check_ciphertext) = 49),
    initialized_at TEXT NOT NULL,
    initialized_by_device_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    init_request_digest TEXT NOT NULL CHECK (length(init_request_digest) = 64),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE vault_items (
    credential_instance_id TEXT PRIMARY KEY REFERENCES credential_instances(id) ON DELETE RESTRICT,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    device_id TEXT NOT NULL REFERENCES device_identity(id) ON DELETE RESTRICT,
    provider TEXT NOT NULL CHECK (provider = 'codex'),
    envelope_version INTEGER NOT NULL CHECK (envelope_version = 1),
    credential_revision INTEGER NOT NULL CHECK (credential_revision >= 1),
    cipher_name TEXT NOT NULL CHECK (cipher_name = 'aes-256-gcm'),
    payload_nonce BLOB NOT NULL CHECK (length(payload_nonce) = 12),
    payload_ciphertext BLOB NOT NULL CHECK (length(payload_ciphertext) BETWEEN 18 AND 65552),
    wrap_name TEXT NOT NULL CHECK (wrap_name = 'aes-256-gcm'),
    wrap_nonce BLOB NOT NULL CHECK (length(wrap_nonce) = 12),
    wrapped_dek BLOB NOT NULL CHECK (length(wrapped_dek) = 48),
    aad_digest TEXT NOT NULL CHECK (length(aad_digest) = 64),
    secret_digest TEXT NOT NULL CHECK (length(secret_digest) = 64),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (credential_instance_id, credential_revision)
);

-- Logout reserves a credential before touching its canonical filesystem home.
-- CreateSession checks this row in the same transaction as its insert, closing
-- the check/delete/revoke race while keeping Vault bytes until cleanup succeeds.
CREATE TABLE credential_revocations (
    credential_instance_id TEXT PRIMARY KEY REFERENCES credential_instances(id) ON DELETE CASCADE,
    requested_at TEXT NOT NULL
);

CREATE TABLE auth_enrollments (
    id TEXT PRIMARY KEY,
    client_device_id TEXT NOT NULL REFERENCES client_identities(id) ON DELETE RESTRICT,
    runtime_profile_id TEXT NOT NULL REFERENCES runtime_profiles(id) ON DELETE RESTRICT,
    credential_instance_id TEXT REFERENCES credential_instances(id) ON DELETE RESTRICT,
    binary_fingerprint TEXT NOT NULL CHECK (length(binary_fingerprint) = 64),
    staging_path TEXT NOT NULL CHECK (length(staging_path) BETWEEN 1 AND 4096),
    state TEXT NOT NULL CHECK (state IN ('begun', 'validating', 'succeeded', 'cancelled', 'expired', 'failed')),
    idempotency_digest TEXT NOT NULL CHECK (length(idempotency_digest) = 64),
    completion_idempotency_digest TEXT CHECK (completion_idempotency_digest IS NULL OR length(completion_idempotency_digest) = 64),
    expires_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX auth_enrollments_one_active_profile
ON auth_enrollments(runtime_profile_id)
WHERE state IN ('begun', 'validating');

CREATE UNIQUE INDEX auth_enrollments_client_begin_digest
ON auth_enrollments(client_device_id, idempotency_digest);

ALTER TABLE approvals RENAME TO approvals_v4;
DROP INDEX approvals_status_requested_at;
DROP INDEX approvals_session_requested_at;

CREATE TABLE approvals (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    provider_approval_id TEXT NOT NULL CHECK (length(provider_approval_id) BETWEEN 1 AND 256),
    kind TEXT NOT NULL CHECK (length(kind) BETWEEN 1 AND 64),
    payload_digest TEXT NOT NULL CHECK (length(payload_digest) = 64),
    summary TEXT NOT NULL CHECK (length(summary) <= 2048),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'cancelled')),
    response_state TEXT NOT NULL CHECK (response_state IN ('idle', 'dispatching', 'written', 'ambiguous')),
    requested_decision TEXT CHECK (requested_decision IS NULL OR requested_decision IN ('approve', 'deny', 'cancel')),
    responded_by_device_id TEXT REFERENCES client_identities(id) ON DELETE RESTRICT,
    idempotency_key TEXT NOT NULL CHECK (length(idempotency_key) BETWEEN 1 AND 128),
    dispatch_digest TEXT CHECK (dispatch_digest IS NULL OR length(dispatch_digest) = 64),
    requested_at TEXT NOT NULL,
    dispatch_started_at TEXT,
    responded_at TEXT,
    dispatch_error_code TEXT NOT NULL DEFAULT '' CHECK (length(dispatch_error_code) <= 64),
    UNIQUE (session_id, provider_approval_id),
    CHECK (
        (response_state = 'idle' AND status = 'pending' AND requested_decision IS NULL
            AND dispatch_digest IS NULL AND dispatch_started_at IS NULL AND responded_at IS NULL
            AND responded_by_device_id IS NULL AND dispatch_error_code = '')
        OR (response_state = 'dispatching' AND status = 'pending' AND requested_decision IS NOT NULL
            AND dispatch_digest IS NOT NULL AND dispatch_started_at IS NOT NULL AND responded_at IS NULL
            AND responded_by_device_id IS NOT NULL AND dispatch_error_code = '')
        OR (response_state = 'written' AND responded_at IS NOT NULL AND responded_by_device_id IS NOT NULL
            AND dispatch_digest IS NOT NULL AND dispatch_started_at IS NOT NULL AND dispatch_error_code = ''
            AND ((requested_decision = 'approve' AND status = 'approved')
              OR (requested_decision = 'deny' AND status = 'denied')
              OR (requested_decision = 'cancel' AND status = 'cancelled')))
        OR (response_state = 'ambiguous' AND status = 'expired' AND requested_decision IS NOT NULL
            AND dispatch_digest IS NOT NULL AND dispatch_started_at IS NOT NULL
            AND responded_at IS NOT NULL
            AND (responded_by_device_id IS NOT NULL OR dispatch_error_code IN ('legacy_dispatch_unknown','daemon_restart_before_dispatch'))
            AND dispatch_error_code <> '')
    )
);

-- P0 could record local terminal decisions without a Provider write. Preserve
-- their intent but downgrade them to expired/ambiguous rather than falsely
-- claiming dispatch. Pending requests remain idle and actionable.
INSERT INTO approvals(
    id, session_id, provider_approval_id, kind, payload_digest, summary, status,
    response_state, requested_decision, responded_by_device_id, idempotency_key,
    dispatch_digest, requested_at, dispatch_started_at, responded_at, dispatch_error_code
)
SELECT id, session_id, provider_approval_id, kind, payload_digest, summary,
       CASE WHEN status = 'pending' THEN 'pending' ELSE 'expired' END,
       CASE WHEN status = 'pending' THEN 'idle' ELSE 'ambiguous' END,
       CASE status WHEN 'approved' THEN 'approve' WHEN 'denied' THEN 'deny'
                   WHEN 'cancelled' THEN 'cancel' WHEN 'expired' THEN 'deny' ELSE NULL END,
       CASE WHEN status = 'pending' THEN NULL ELSE responded_by_device_id END,
       idempotency_key,
       CASE WHEN status = 'pending' THEN NULL ELSE lower(hex(randomblob(32))) END,
       requested_at,
       CASE WHEN status = 'pending' THEN NULL ELSE coalesce(responded_at, requested_at) END,
       CASE WHEN status = 'pending' THEN NULL ELSE coalesce(responded_at, requested_at) END,
       CASE WHEN status = 'pending' THEN '' ELSE 'legacy_dispatch_unknown' END
FROM approvals_v4;

DROP TABLE approvals_v4;
CREATE INDEX approvals_status_requested_at ON approvals(status, requested_at);
CREATE INDEX approvals_session_requested_at ON approvals(session_id, requested_at);
CREATE INDEX approvals_response_state_requested_at ON approvals(response_state, requested_at);
