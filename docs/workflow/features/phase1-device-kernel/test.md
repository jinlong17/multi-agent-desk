# Test strategy: Phase 1 Device Kernel

## Acceptance matrix

| Requirement | Level | Command/scenario | Expected evidence |
|---|---|---|---|
| Domain invariants | unit/property | `go test ./internal/domain/...` | illegal Session transitions, resume mutation, multiple controllers, stale revisions rejected |
| SQLite and migrations | unit/integration | `go test ./internal/storage/...` | empty/current/restart/interrupted/future schema, WAL/FK/busy settings, rollback atomicity |
| Peer authentication | contract/adversarial | `go test ./internal/device/...` | both proofs, transcript binding, rotation/revocation, wrong key/version/replay/timeout rejection |
| Native IPC | platform integration | platform-tagged device tests | private Unix socket and ADR 0013 Named Pipe ownership/readback with no TCP listener |
| Application authorization | unit/integration | `go test ./internal/app/...` | capability and lease checks deny before mutation; idempotency is transactional |
| Runtime and replay | unit/integration | `go test ./internal/runtime/...` | Fake Provider subprocess, bounded ring, attach/detach, input/resize, stop/kill, truncated replay |
| Vault boundary | unit/integration | lock/restart/unlock scenarios | metadata while locked; start/materialization denied; restart locked |
| Materialization recovery | failure injection | staged/CAS/crash/quarantine scenarios | single writer, permissions, monotonic revision, atomic commit, no stale overwrite |
| CLI contract | golden/black-box | `go test ./cmd/multidesk/...` | deterministic JSON v1 and redacted human/errors |
| Phase 1 exit | cross-process E2E | actual Daemon plus two clients | start, observe, attach/detach, lease, input, resize, stop/kill, new-record resume |
| Governance | repository | project/CI/license/link checks | all protected checks and required documents pass |

## Unit and property tests

- Table-test every Session transition and terminal-state invariant.
- Generate lease operation sequences against an injectable clock and assert at
  most one unexpired holder and monotonic revisions.
- Exercise ring-buffer wraparound at zero, exact capacity, oversized chunk,
  multiple wraps, and requested sequences before/inside/after retention.
- Test identifiers, capability parsing, stable error redaction, request digest,
  and canonical handshake transcript determinism.
- Test Vault lock state, restart reset, and fake-secret access rejection.
- Test service-spec rendering from deterministic paths without mutating the
  actual host.

Race tests are mandatory on Ubuntu and macOS for `internal/domain`,
`internal/app`, `internal/runtime`, and `internal/device` where supported.
Windows runs concurrency stress and the race-free cross-compile/build gate;
absence of the Go Windows race detector is not reported as equivalent race
evidence.

## Contract and fixture tests

- Golden encode/decode fixtures for handshake and request/response/event
  envelopes at protocol 1.0.
- Negative fixtures for unknown major/minor requirements, unknown fields where
  forbidden, invalid base64, duplicate JSON keys, missing IDs, malformed
  signatures, uncanonical capability order, and 256 KiB boundaries.
- JSON CLI golden files require stable error codes and additive schemas.
- CLI mutation tests derive distinct request-bound idempotency keys for
  different bodies, replay exact retries, and reject argv Vault secrets while
  accepting bounded stdin unlock input without echoing it.
- Fake Provider fixtures cover normal output, resize acknowledgement, delayed
  graceful exit, crash, malformed child frame, blocked writer, and forced kill.
- Migration checksums are immutable and applied exactly once.

## Integration and E2E

The canonical E2E builds one binary, creates a temporary Device root, provisions
two distinct client identities, starts `daemon serve`, unlocks the fake Vault,
and starts a Fake Session. Client B lists/observes, attaches, receives replay,
detaches without terminating the child, acquires control, sends sequenced input
and resize, heartbeats/releases, and demonstrates observer denial. Expiry under
an injectable short test lease permits Client A to acquire a higher revision.

The scenario separately proves graceful Stop and forced Kill are idempotent.
Resume starts a new Session ID with `resumedFromSessionId`; the original record
stays terminal. The Daemon is restarted to prove database durability, locked
Vault restart, terminal Session recovery, identity pinning, and endpoint
reacquisition.

Tests use actual subprocesses and native IPC, not an in-memory transport. All
paths and service roots are temporary. No test installs a real login item,
systemd unit, Scheduled Task, or service on the developer or CI host.

## Security/adversarial tests

- Endpoint squatting, stale Unix socket, permissive mode, Windows DACL mismatch,
  remote pipe path, and second Daemon all fail closed.
- Unknown/revoked/rotated client, wrong Daemon pin, replayed hello/proof,
  transcript downgrade, signature substitution, and nonce reuse are rejected
  before metadata.
- Missing capability, observer mutation, wrong Session, wrong holder, stale or
  expired lease, duplicated input, request-ID collision, and idempotency digest
  mismatch produce no duplicate side effect.
- Oversized length, truncated frame, invalid JSON, slowloris reads, blocked
  writes, connection storms, request concurrency overflow, and child output
  floods remain bounded and do not starve a healthy client.
- Audit/log capture scans for private-key encodings, fake secret markers,
  terminal fixture markers, raw frame content, and unbounded filesystem paths.
- Symlink/path traversal, permission drift, partial staging, stale revision,
  unknown manifest version, digest mismatch, missing pending database row,
  abrupt Daemon exit, and concurrent refresh prove quarantine/no-overwrite
  behavior without claiming authenticity against a same-user attacker.

## Cross-platform matrix

| Evidence | macOS arm64 | Ubuntu x64 | Windows x64 |
|---|---:|---:|---:|
| Go unit/contract/integration | required | required | required |
| Native local IPC E2E | Unix socket required | Unix socket required | Named Pipe required |
| Actual Daemon + two clients | required | required | required |
| SQLite WAL/restart | required | required | required |
| Fake Provider subprocess | required | required | required |
| Service specification render | LaunchAgent | systemd user | Scheduled Task |
| Race detector | required | required | concurrency stress; no equivalence claim |
| Cross-build auxiliary targets | linux/windows | darwin/windows | darwin/linux |

Windows Server CI is contract evidence only. Windows 11 multi-user, Fast User
Switching, startup-task/service context, sleep/resume, signed packaging, real
Provider ConPTY, IME, mouse, and accessibility remain later acceptance gates.

## Failure injection and recovery

- Abort each migration boundary and reopen; only a complete migration records
  its version.
- Crash after staging, manifest fsync, rename, database CAS, child start, and
  child exit; restart chooses recover, clean, or quarantine deterministically.
- Hold SQLite writer locks beyond deadline and verify a stable error without
  database replacement.
- Kill the Daemon while Fake Provider runs; restart must not authenticate/adopt
  the child and must persist a safe terminal recovery result.
- Corrupt the runtime-home manifest version/digest, remove its pending row, and
  create a newer database revision; restart quarantines residue and never
  overwrites the newer row. A complete exact match to a pre-registered pending
  row is the only promotable recovery case.
- Fill replay, connection, request, event, and materialization quotas; only the
  scoped operation/client is rejected or truncated.

## Manual acceptance

After automated evidence passes, run the human CLI journey on the local macOS
host and inspect human/JSON output, permissions, endpoint ownership, service
specification text, attach/detach behavior, lease expiry, and restart recovery.
The security-review role then independently inspects the code and evidence.
No Phase 1 manual step installs a service, records a real Provider credential,
publishes a release, or claims Windows 11 acceptance.
