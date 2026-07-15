# P2 as-built: identity, local IPC, and Daemon lifecycle

## Implemented boundary

P2 adds the first production-shaped local control boundary around the P1
Device Store. The Daemon remains the only persistent writer; clients receive
no database handle and communicate through one authenticated local endpoint.

- Device bootstrap atomically creates a private root, database, Ed25519 Daemon
  identity, pinned owner client identity, and capability allowlist.
- Client administration creates, rotates, and revokes public keys with
  monotonic revisions. Private keys are generated and returned to the local
  operator boundary; they are never inserted into SQLite or sent over IPC.
- The four-byte big-endian framed protocol has a 256 KiB body limit, typed
  version-1 envelopes, strict decoding, duplicate-key rejection, bounded
  request IDs/methods, and generic authentication failures.
- The handshake binds both key digests, both 32-byte nonces, endpoint-instance
  identity, protocol version, and canonical requested capabilities to Ed25519
  signatures. Client nonces are single-use within a bounded server cache.
- Every request is checked against the current client identity revision and a
  server-owned method-to-capability map. Unknown methods, revoked identities,
  stale revisions, and missing capabilities fail before the handler runs.
- Unix uses a private mode-0600 socket. When a macOS temporary root exceeds the
  platform sockaddr path limit, a private hashed endpoint directory is used;
  protocol authentication remains mandatory and no socket is silently
  replaced.
- Windows uses message-mode Named Pipes with a protected current-logon SID
  DACL, Network SID denial, remote-client rejection, first-instance ownership,
  live DACL readback, same-session verification, and `CancelIoEx`-backed
  deadlines. No loopback or TCP fallback exists.
- Service specifications render without mutating the host: LaunchAgent,
  systemd user unit, and least-privilege interactive Scheduled Task.
- `multidesk init`, `multidesk daemon serve`, and `multidesk daemon status`
  exercise the same application service boundary and versioned JSON response
  shape. Full session, Vault, runtime, TUI, and install/uninstall commands stay
  in later phases.

## Verification scope

The P2 writer suite covers duplicate JSON keys, frame bounds, authenticated
handshake and capability intersection, key rotation/revocation CAS behavior,
Bootstrap identity/store consistency, Unix native daemon round-trip, service
specification rendering, authorization denial, ordinary tests, race tests,
vet, three-target compile, exact Go license scanning, project/CI/scaffold
checks, and the Windows-native Named Pipe test compiled for the hosted runner.

## Explicit limits

The current Windows evidence is still Windows Server CI. It does not claim
Windows 11 multi-user, Fast User Switching, service Session 0, signed
packaging, ConPTY, or production Vault encryption. Identity-file rotation
returns a new private-key record to the operator boundary; a later CLI phase
will persist that record using the same atomic private-file contract.
