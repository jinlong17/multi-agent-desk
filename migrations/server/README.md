# Control Plane server migrations

This directory contains the embedded, forward-only SQLite migration ledger for
the Control Plane server. P1 owns only the server and request foundations:

- `0001_server_foundation.sql`
- `0002_request_foundation.sql`

Later phases add their reviewed schemas without renumbering or rewriting these
files. Rollback restores a verified pre-migration backup with the prior binary;
there are no destructive down migrations.
