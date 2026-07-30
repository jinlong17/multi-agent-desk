CREATE TABLE idempotency_records (
    scope_digest BLOB PRIMARY KEY CHECK (length(scope_digest) = 32),
    request_digest BLOB NOT NULL CHECK (length(request_digest) = 32),
    response_status INTEGER NOT NULL CHECK (response_status BETWEEN 100 AND 599),
    response_content_type TEXT NOT NULL CHECK (response_content_type = 'application/json'),
    response_body BLOB NOT NULL CHECK (length(response_body) <= 1048576),
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
) STRICT;
CREATE INDEX idempotency_records_expiry_idx ON idempotency_records(expires_at);

CREATE TABLE pre_user_audit_events (
    id TEXT PRIMARY KEY CHECK (length(id) = 36),
    action TEXT NOT NULL CHECK (length(action) BETWEEN 1 AND 128),
    decision TEXT NOT NULL CHECK (decision IN ('allowed', 'denied', 'failed')),
    error_code TEXT CHECK (error_code IS NULL OR length(error_code) <= 64),
    request_id TEXT CHECK (request_id IS NULL OR length(request_id) = 36),
    created_at TEXT NOT NULL
) STRICT;
