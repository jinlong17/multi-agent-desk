# P4 as-built: Vault and materialization recovery

P4 adds the locked/unlocked runtime boundary and fake credential
materialization recovery without claiming production cryptographic Vault
encryption. The Daemon remains the single writer for the SQLite journal and the
only runtime-home materialization writer.

## Implemented boundary

- `internal/vault.Manager` starts locked, accepts a bounded explicit unlock
  input, retains only state/epoch, and resets to locked when a new Manager is
  created (the Daemon restart boundary). `RequireUnlocked` returns the stable
  `vault_locked` code. `daemon serve` wires this gate into `sessions.start`;
  `vault.status`, `vault.unlock`, and `vault.lock` are authenticated service
  methods with idempotency keys.
- `credential_materializations` now has repository create/read/list/CAS
  transitions. A materializer writes only fake content, first records a
  `pending` row, writes a version-1 canonical manifest and `credential.fake`
  into a private staging directory, fsyncs the files, and atomically renames
  into `leases/<lease_id>` before promoting the row to `active`.
- Manifest and content SHA-256 digests, credential revision, lease ID, and
  state are checked during recovery. Missing rows, unknown paths, malformed or
  stale manifests, content-digest mismatches, and interrupted staging are
  moved to a private `quarantine` directory; no ambiguous residue overwrites a
  newer database revision. Release marks the row released and removes its
  runtime home.
- Recovery and materialization are serialized inside one materializer mutex;
  SQLite remains one connection/WAL with transactional repository writes. No
  secret bytes are written to audit/idempotency metadata.

## Evidence and limits

`internal/vault` tests prove locked/unlocked/restart behavior, atomic active
commit, restrictive fake-content permissions, stale revision rejection,
idempotent same-manifest replay, and quarantine after manifest corruption.
Full Go tests, scoped race tests, `go vet`, and darwin/arm64, linux/amd64, and
windows/amd64 builds pass locally.

This is a fake Vault state/materialization boundary only. It does not claim
Argon2id/OS-keychain integration, envelope encryption, real Provider
credentials, Windows 11 multi-user behavior, release, or deployment.
