CREATE TABLE server_metadata (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    schema_epoch TEXT NOT NULL CHECK (length(schema_epoch) = 36),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;
