# Development log: Phase 1 Device Kernel

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `phase1-device-kernel` |
| Title | `Phase 1 Device Kernel` |
| Owner Module | `core` |
| Impacted Modules | `security, provider, desktop, project-system` |
| Current Phase | `REVIEW` |
| Status | `APPROVED` |
| Executor | `Codex (GPT-5) as independent feature-review` |
| Updated | `2026-07-14 20:54 -0700` |
| Suggested Next | `feature-build P1 domain and Device store` |
| Branch / Worktree | `codex/core/phase1-device-kernel` / `/Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/phase1-device-kernel` |
| Plan Version | `v0.2` |
| Provider Gate | `none — deterministic first-party Fake Provider only` |
| Security Gate | `open — local peer auth, authorization, Vault/materialization, recovery, bounds, redaction` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 domain and Device store | domain entities/invariants; SQLite driver, schema, migrations, repositories, transactions, future-schema/restart handling | approved plan | domain and storage suites pass on empty/current/restart/future state; WAL/FK/busy settings proven | `PLANNED` |
| P2 identity, IPC, and Daemon lifecycle | Ed25519 bootstrap/rotation/revocation; framed protocol; Unix socket/Windows Named Pipe; application authorization shell; Daemon/service specs | P1 verified | mutual authentication and fail-closed endpoint tests; native IPC on three platforms; no TCP listener | `PLANNED` |
| P3 Fake runtime and Session control | Fake Provider subprocess; process manager; Session state machine; ring buffer; attachments; ControllerLease; input/resize/stop/kill/resume | P2 verified | two-client native-IPC scenario passes; observer/lease/idempotency/replay and bounded process behavior proven | `PLANNED` |
| P4 Vault and materialization recovery | locked/unlocked runtime; fake credential revision/CAS; atomic runtime home; cleanup/quarantine; failure injection | P3 verified | lock/restart boundary, single writer, permission, crash and stale-overwrite tests pass | `PLANNED` |
| P5 CLI/TUI and platform exit | stable JSON/human commands; minimal TUI; service-spec commands; docs; complete three-platform E2E/CI/license/race evidence | P4 verified | Phase 1 exit scenario passes on macOS/Linux/Windows; full project checks and security review ready | `PLANNED` |

Each phase is implemented by one writer, sets `READY_FOR_VERIFY`, receives an
independent phase verdict, and only then unlocks the next phase. After P5
verification, the required independent Security Gate must pass before ship.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-14 20:43 -0700 | INTAKE | `mad-module-classify` against the Feature Brief, implementation plan, module registry, ADRs, and current scaffold | high-confidence owner `core`; secondary `security, provider, desktop, project-system`; Provider Gate none; Security Gate open | Feature Brief and task record |
| 2026-07-14 20:45 -0700 | PLAN | `go list -m -json` and `go mod download -json` for `modernc.org/sqlite@v1.53.0`, `ncruces/go-sqlite3@v0.35.2`, `x/sys@v0.47.0`, `go-winio@v0.6.2`; upstream module/readme/license inspection | selected `ncruces/go-sqlite3 v0.35.2`: CGO-free, Go 1.25, smaller direct boundary; native x/sys Windows adapter retained for ADR 0013 controls | this design; Go module cache metadata |
| 2026-07-14 20:45 -0700 | PLAN | Go 1.26.5 `go test -c` for `ncruces/go-sqlite3/driver` targeting darwin/arm64, linux/amd64, windows/amd64; direct license inspection | pass: 15–17 MiB test binaries produced on all targets; MIT, MIT-0, and BSD-family dependency licenses observed | temporary `/tmp/mad-sqlite-compile.*` evaluation |
| 2026-07-14 20:45 -0700 | PLAN | upstream driver test attempted from read-only module cache | expected evaluation-environment failure: example tried to create `recordings.db` in read-only module cache; not treated as driver failure or pass | retained command output; project-owned temp-directory tests required in P1 |
| 2026-07-14 20:50 -0700 | REVIEW | exact `go-licenses v2.0.1 csv/check` comparison | `ncruces/go-sqlite3-wasm/v3` reported `Unknown`; `modernc.org/sqlite v1.53.0` production set fully identified as MIT/BSD-3-Clause and allowlist check passed | `docs/reviews/phase1-device-kernel/2026-07-14-feature-review.md` |
| 2026-07-14 20:53 -0700 | PLAN | Go 1.26.5 minimal production smoke and cross-build for `modernc.org/sqlite v1.53.0` | pass: WAL, foreign keys, and 5000 ms busy timeout read back on macOS; 9.1–9.5 MiB programs built for darwin/arm64, linux/amd64, windows/amd64 | temporary `/tmp/mad-modernc-smoke` evaluation |

## Risks and Blockers

- No planning blocker. The plan requires independent review before production
  implementation begins.
- Full SQLite migration, lock-contention, restart, and recovery behavior must be
  proven with project-owned temporary-directory tests; the planning smoke only
  proves driver configuration and three-platform production compilation.
- Native Windows Named Pipe code is security-sensitive and must satisfy every
  ADR 0013 control plus the final Security Gate.
- Identity recovery is intentionally offline and fail closed; implementation
  must not add a convenience reset over an unauthenticated channel.
- Windows Server CI does not close Windows 11 multi-user/service or signed
  release acceptance.
- Real Provider, PTY/ConPTY, real Vault crypto, release, and deployment remain
  out of scope and must not be inferred from Fake evidence.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-14 20:45 -0700 | Codex (GPT-5) as feature-plan | Converted the approved Feature Brief into a five-phase implementation plan; froze package ownership, SQLite choice, Ed25519 peer authentication, protocol/authorization semantics, native platform IPC, runtime/Vault/materialization recovery, cross-platform evidence, rollback, and retained gates without changing production code | Feature Brief; `design.md`, `api.md`, `test.md`, this file | `NEEDS_REVIEW`; no implementation performed | feature-review |
| 2026-07-14 20:50 -0700 | Codex (GPT-5) as independent feature-review | Reviewed scope, ownership, contracts, failure/recovery, authentication and authorization, migrations, platform matrix, testability, rollback, dependency license evidence, and phase ordering; found one unidentified transitive SQLite license and one undefined materialization signature contract; modified only verdict surfaces | `docs/reviews/phase1-device-kernel/2026-07-14-feature-review.md`, this file | `REVISE`; no external blocker | feature-plan |
| 2026-07-14 20:53 -0700 | Codex (GPT-5) as feature-plan | Resolved both review findings: selected `modernc.org/sqlite v1.53.0` after exact license, runtime pragma, and three-platform build evidence; replaced the undefined signature with a versioned canonical manifest, database pre-registration, SHA-256 integrity-only semantics, exact recovery rule, and explicit same-user limitation | `design.md`, `test.md`, this file; temporary evaluation evidence | `NEEDS_REVIEW`; no implementation performed | feature-review |
| 2026-07-14 20:54 -0700 | Codex (GPT-5) as independent feature-review | Re-reviewed the complete plan and prior finding closure across scope, contracts, dependency compliance, manifest integrity, failure/recovery, security/privacy, migrations, compatibility, tests, rollback, and phase ordering; modified only verdict surfaces | `docs/reviews/phase1-device-kernel/2026-07-14-feature-review-v2.md`, this file | `APPROVED`; no blocking finding | feature-build P1 domain and Device store |
