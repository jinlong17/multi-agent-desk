# P3 as-built: Fake runtime and Session control

P3 connects the P2 authenticated request boundary to a deterministic, bounded
Fake Provider subprocess. The Daemon remains the only SQLite writer and owns
the child process, Session transitions, attachments, lease CAS, input ordering,
and in-memory terminal replay.

## Implemented boundary

- `internal/runtime` starts the same `multidesk internal fake-provider` binary,
  communicates over a capped line-framed JSON protocol, observes exit, and
  supports bounded input, resize, graceful stop, and forced kill.
- `RingBuffer` defaults to 4 MiB and 4 KiB chunks. Output is copied at the
  boundary, sequence numbered monotonically, and replay reports retained data
  with `truncated=true` plus `replay_unavailable` when the requested sequence
  predates the retained window.
- `Manager` creates `starting -> running` Sessions only after the child emits
  `ready`; unexpected exits become `failed/provider_failed`; graceful stop
  becomes `stopping -> exited`; kill becomes terminal `killed`; resume creates
  a new record linked by `resumed_from_session_id`.
- Attach/detach persist per-client attachments and never stop the child.
  Controller leases use the existing domain invariants and SQLite CAS. Input
  sequences are monotonic per `(session, client)`; duplicates return a bounded
  duplicate ACK and gaps do not execute provider input.
- `SessionService` maps P3 protocol methods to the runtime manager, enforces
  strict request bodies, and persists idempotency digests/results in the
  existing `idempotency_records` table. Reusing a key with a different digest
  returns `conflict`.
- `daemon serve` now uses this service and the hidden child command is wired in
  the main binary. No CLI surface claims Vault unlock, real Provider access,
  PTY/ConPTY, or deployment support before P4/P5.

## Evidence and limits

The runtime package test builds a real `multidesk` binary and drives a child
subprocess through start, replay, duplicate input, resize, stop, resume, and
kill. `internal/device/native_session_e2e_test.go` additionally builds that
binary, starts the authenticated native endpoint, and drives two clients
through start/idempotent replay/list/attach/observe/observer denial,
lease/input/resize/stop/resume/kill, and shutdown. Full Go tests, scoped race
tests, `go vet`, and darwin/arm64, linux/amd64, and windows/amd64 cross-builds
pass locally.

Independent verification must still run the application service through two
authenticated native IPC clients and repeat the P3 acceptance on macOS,
Ubuntu, and Windows. Windows Server Named Pipe evidence proves transport and
process compatibility only; it does not claim Windows 11 multi-user,
ConPTY, signed packaging, Vault encryption, or release readiness.
