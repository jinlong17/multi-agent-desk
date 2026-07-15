# Verification record: Phase 1 Device Kernel P3 — VERIFIED

## Verdict

`VERIFIED` at implementation/evidence head `b001842b6fc017260591f944f57fbce538a88f7a`.

P3 now has a deterministic Fake Provider subprocess, bounded replay, durable
Session/attachment/lease behavior, strict application service handling, and an
actual two-client native IPC acceptance path. The prior Windows executable
path blocker and the single-connection `ListSessions` deadlock were both
reproduced, persisted as blockers, corrected, and rerun on the protected
runner.

## Acceptance evidence

- Local `go test ./...`, scoped `go test -race ./internal/runtime ./internal/app
  ./internal/device`, `go vet ./...`, and the native Unix two-client test pass.
- The native E2E builds a real `multidesk` binary, starts the authenticated
  endpoint, uses two distinct client identities, and covers start,
  idempotent replay, list, attach, observe, observer mutation denial, lease,
  sequenced input, resize, graceful stop, new-record resume, forced kill, and
  daemon shutdown.
- Draft PR #13 CI run `29392655976` passed macOS job `87279295874`, Ubuntu job
  `87279295931`, Windows job `87279295885`, and project-verify job
  `87279295866`. Windows logs show green `internal/device`, `internal/runtime`,
  and `internal/storage` packages after the `.exe` correction.
- Governance run `29392655993` passed DCO `87279295892`, license-gate
  `87279295933`, and link-check `87279295938`.

## Scope and limits

P3 does not claim Vault encryption/materialization recovery, real Provider
compatibility, PTY/ConPTY, Windows 11 multi-user/service behavior, signed
packaging, release, or deployment. P4 remains required for Vault and
materialization/restart recovery; the final Phase 1 Security Gate remains open.
