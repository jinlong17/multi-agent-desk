# Contracts: Phase 1 Device Kernel

## Public interfaces

The stable Phase 1 command surface is:

```text
multidesk init [--root DIR] [--json]
multidesk client create|list|rotate|revoke [--root DIR] [--json]
multidesk daemon install|uninstall|start|stop|status|serve [--root DIR] [--json]
multidesk vault status|unlock|lock [--root DIR] [--json]
multidesk run fake --workspace PATH [--profile ID] [--root DIR] [--json]
multidesk sessions list|show|observe|attach|detach|stop|kill|resume [ID] [--root DIR] [--json]
multidesk control acquire|heartbeat|release SESSION_ID [--revision N] [--root DIR] [--json]
multidesk terminal input|resize SESSION_ID [--revision N] [--root DIR] [--json]
multidesk tui [--root DIR]
```

`daemon serve` and the hidden Fake Provider child mode are process entrypoints,
not network APIs. Service install/uninstall defaults to rendering and validating
user-level platform specifications. Host mutation requires an explicit flag and
is excluded from automated tests and Phase 1 acceptance on the developer host.

Every `--json` response has `schema_version`, `request_id`, `ok`, and exactly
one of `result` or `error`. Human output may summarize metadata but never prints
identity private keys, fake credential bytes, terminal payloads, or raw IPC.

## Requests, events, and responses

The local protocol uses a 4-byte big-endian length followed by one JSON object;
the encoded object may not exceed 256 KiB.

```text
ClientHello  {protocol_major, protocol_minor, client_id, client_nonce,
              requested_capabilities[]}
ServerProof  {daemon_id, daemon_nonce, endpoint_instance, negotiated_minor,
              daemon_signature}
ClientProof  {client_signature}
AuthOK       {connection_id, granted_capabilities[], expires_at}
Request      {protocol_major, request_id, method, idempotency_key?,
              lease_revision?, body}
Response     {protocol_major, request_id, ok, result?, error?}
Event        {protocol_major, stream_id, sequence, kind, truncated, body}
```

Canonical signatures cover a domain-separation label, both IDs, both public-key
digests, both nonces, endpoint instance, negotiated protocol version, and the
ordered requested capability set. Nonces are 32 random bytes and single-use.
The server does not send `AuthOK` or protected metadata before both proofs pass.

Phase 1 methods and required authorization are:

| Method | Capability | Additional precondition |
|---|---|---|
| `daemon.status`, `vault.status`, `sessions.list/show/observe` | `metadata.read` | authenticated connection |
| `sessions.attach/detach` | `session.observe` | live or terminal Session as applicable |
| `vault.unlock/lock` | `vault.control` | idempotency key |
| `sessions.start` | `session.start` | Vault unlocked; idempotency key |
| `control.acquire` | `session.control.acquire` | no current lease or expired lease; idempotency key |
| `control.heartbeat/release` | `session.control` | current holder and lease revision |
| `terminal.input/resize` | `terminal.control` | current holder, lease revision, idempotency key/sequence |
| `sessions.stop/kill` | `session.control` | current holder, lease revision, idempotency key |
| `sessions.resume` | `session.resume` | terminal source Session, Vault unlocked, idempotency key |
| `client.*` | `client.admin` | local-owner identity; rotation is monotonic |

The Fake Provider emits deterministic `ready`, `output`, `resized`, and `exit`
events. Terminal bytes are transported as base64 inside JSON but are never
persisted as audit payload. Replay returns the requested monotonic sequence
range or `replay_unavailable` plus the retained buffer and `truncated=true`.

## Error semantics

Stable error codes are lowercase snake case:

`invalid_argument`, `not_found`, `already_exists`, `conflict`, `unauthenticated`,
`permission_denied`, `unsupported_version`, `method_not_found`, `frame_too_large`,
`deadline_exceeded`, `resource_exhausted`, `daemon_unavailable`,
`schema_incompatible`, `vault_locked`, `invalid_transition`, `lease_required`,
`lease_held`, `stale_lease`, `replay_unavailable`, `provider_failed`,
`materialization_conflict`, `quarantined`, and `unsupported_platform`.

Errors contain a stable code, safe message, and optional bounded metadata such
as current state or revision. They never echo proof bytes, terminal content,
fake secret data, full raw paths, SQL, Named Pipe DACL internals, or frames.
Authentication failures use the same generic external response.

## Authentication and authorization

Daemon and client identities are Ed25519 keys created by `init` or explicit
client administration. The Daemon key is pinned by each client; client public
keys, status, revision, and capabilities are allowlisted in Device SQLite.
Private keys are atomic restrictive files and never cross IPC.

OS peer context may be retained as bounded audit metadata but never grants a
capability. Every method is mapped server-side to one required capability.
Lease-protected operations also bind Session ID, holder client ID, lease
revision, and request identity. Revocation or key rotation prevents new
connections immediately; existing connections are closed when their identity
revision no longer matches.

## Idempotency, ordering, and replay

- Request IDs correlate one connection and are unique for its lifetime.
- Mutation idempotency keys are scoped to `(client_id, method, key)` and store a
  request digest plus bounded response metadata in the same transaction as the
  mutation. Reuse with another digest returns `conflict`.
- Terminal input uses a monotonic client sequence. A duplicate returns the
  prior ACK; a gap returns the expected sequence without executing input.
- Lease revisions increase on every acquire, transfer-by-expiry, and release.
  Heartbeat extends expiry without lowering or reusing a revision.
- Runtime output sequence is monotonic per Session stream. A bounded ring
  buffer supports replay and marks truncation explicitly.
- Stop, Kill, Lock, Unlock, Attach, Detach, and Release are idempotent for an
  identical key and safe terminal/current state.

## Versioning and compatibility

Phase 1 protocol major is `1`, initial minor is `0`. Unsupported major versions
fail before authentication completes without negotiation fallback. Peers use
the lower supported minor after the signed transcript binds it. New minor fields
must be optional; existing meanings and error codes are not repurposed.

CLI JSON `schema_version` starts at `1`. Database schema version starts at `1`.
The service specification and Fake Provider child protocol are versioned but
internal. No real Provider, PTY, Windows 11, or Desktop compatibility claim is
created by these contracts.

## Data retention and deletion

Session metadata and bounded structural events persist in Device SQLite.
Terminal replay remains memory-only in Phase 1 and disappears on Daemon exit.
Idempotency results and audit metadata are bounded and prunable by age/count.
Fake runtime homes are removed after the last reference when safe; ambiguous
residue is moved to a private quarantine directory for explicit diagnosis.
Client revocation keeps the public-key digest and decision metadata but not a
private key. Automated uninstall never deletes Device data.
