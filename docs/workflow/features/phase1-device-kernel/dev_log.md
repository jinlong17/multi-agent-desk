# Development log: Phase 1 Device Kernel

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `phase1-device-kernel` |
| Title | `Phase 1 Device Kernel` |
| Owner Module | `core` |
| Impacted Modules | `security, provider, desktop, project-system` |
| Current Phase | `P1 domain and Device store` |
| Status | `VERIFIED` |
| Executor | `Codex (GPT-5) as independent feature-verify P1 v3` |
| Updated | `2026-07-14 21:30 -0700` |
| Suggested Next | `feature-build P2 identity, IPC, and Daemon lifecycle` |
| Branch / Worktree | `codex/core/phase1-device-kernel` / `/Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/phase1-device-kernel` |
| Plan Version | `v0.2` |
| Provider Gate | `none — deterministic first-party Fake Provider only` |
| Security Gate | `open — local peer auth, authorization, Vault/materialization, recovery, bounds, redaction` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P1 domain and Device store | domain entities/invariants; SQLite driver, schema, migrations, repositories, transactions, future-schema/restart handling | approved plan | domain and storage suites pass on empty/current/restart/future state; WAL/FK/busy settings proven | `VERIFIED` |
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
| 2026-07-14 21:02 -0700 | P1 build | initial scoped domain/storage/migration test run | domain passed; storage correctly rejected the test framework's broad temporary directory, so tests were moved under a Store-created private root and rerun | `internal/storage/store_test.go` |
| 2026-07-14 21:07 -0700 | P1 build | Go 1.26.5 `go test ./...`; `go vet ./...`; scoped `go test -race`; `go test -c` storage suite for darwin/arm64, linux/amd64, windows/amd64 | pass: all Go packages; race-clean P1 packages; three 11 MiB cross-platform test binaries compiled | `/tmp/mad-storage-*.test*` |
| 2026-07-14 21:08 -0700 | P1 build | `npm run project:verify`; `npm run ci:verify`; first `npm run scaffold:verify` | project/workflow/dashboard, CI contracts/fixtures/links/licenses, and Go checks passed; scaffold stopped at Web `tsc` because the new worktree had no `node_modules` | retained command output; no source failure inferred |
| 2026-07-14 21:09 -0700 | P1 build | frozen `pnpm install`; `npm run scaffold:verify` | pass: Go/Web/Desktop checks and builds; no dependency version changed | workspace lockfile and command output |
| 2026-07-14 21:11 -0700 | P1 build | final Go tests/vet/race; resume/lease/storage recovery regression; exact `go-licenses v2.0.1 check --include_tests`; project/link/diff checks | pass; one transient misplaced Resume validation compile error was corrected before final run; only the known x/sys assembly inspection warning remains | P1 source/tests and this log |
| 2026-07-14 21:13 -0700 | P1 verify | independent exact-commit code/invariant audit plus full Go/vet/race/cross-build, license, project/CI/scaffold/link, migration, and boundary evidence review | automated checks pass, but three uncovered counterexamples block P1: Resume snapshot invention, non-monotonic reacquire after release, and unbounded invalid ID prefix | `docs/reviews/phase1-device-kernel/2026-07-14-feature-verify-p1.md` |
| 2026-07-14 21:17 -0700 | P1 correction | added exact source/new Resume snapshot equality and source capability enforcement in the creation transaction; monotonic reacquire check; complete bounded ID grammar; direct negative tests; full Go/vet/race/three-target compile, exact license, project/CI/scaffold/diff suite | pass: all three verifier counterexamples now reject without side effects; existing P1 evidence remains green | correction diff and command output |
| 2026-07-14 21:22 -0700 | P1 verify v2 | re-audited the three corrections; reran uncached local/race/governance evidence; pushed corrected head to Draft PR #13 and required actual macOS/Ubuntu/Windows execution | 6/7 protected checks pass; Windows job `87267159036` fails every Store test at database Ping while domain/migrations pass; macOS and Ubuntu Store suites pass | `docs/reviews/phase1-device-kernel/2026-07-14-feature-verify-p1-v2.md`; CI `29388679737`; Governance `29388679739` |
| 2026-07-14 21:25 -0700 | P1 Windows correction | replaced drive-ambiguous URI construction with explicit Unix, drive-rooted Windows, and UNC file-URI mapping; percent-encoded special path characters; added pure mapping positive/negative fixtures; reran local/race/vet/three-target compile, exact licenses, project/CI/scaffold/diff | pass locally; corrected head still requires a new actual Windows run | `internal/storage/store.go`; `internal/storage/store_test.go`; command output |
| 2026-07-14 21:30 -0700 | P1 verify v3 | verified corrected exact head and actual macOS, Ubuntu, and Windows execution; inspected Windows Server 2025 log for domain, Store, migration, scaffold, and desktop results; confirmed all seven protected checks and Draft PR state | `VERIFIED`: all P1 acceptance passes; P2 unlocked; Security Gate remains open | `docs/reviews/phase1-device-kernel/2026-07-14-feature-verify-p1-v3.md`; CI `29388956859`; Governance `29388956809`; Windows job `87267956531` |

## Risks and Blockers

- No P1 blocker. P1 is independently verified and P2 is unlocked.
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
| 2026-07-14 21:11 -0700 | Codex (GPT-5) as feature-build P1 | Implemented only the approved P1 slice: stable domain errors/IDs/capabilities, Session and ControllerLease invariants, three ordered checksum migrations, private single-connection WAL Store, transactional repositories/CAS, future/changed/interrupted schema refusal, restart persistence, as-built data model, and unit/integration/race/cross-build evidence; did not start IPC, Daemon, runtime, Vault, or CLI | `internal/domain`; `internal/storage`; `migrations/device`; `go.mod`; `go.sum`; `docs/DATA_MODEL.md`; this file | `READY_FOR_VERIFY`; all final scoped and repository checks pass | feature-verify P1 domain and Device store |
| 2026-07-14 21:11 -0700 | operator-directed project-system writer via `mad-dashboard-sync` | Bound manual dashboard judgment to the persisted P1 `READY_FOR_VERIFY` verdict without advancing Phase 1 or P2; regenerated workflow mirrors and machine facts | `docs/workflow/project/dashboard-state.json`; generated dashboard state; this file | workflow generation and dashboard verification pass; branch `codex/core/phase1-device-kernel`; Phase 1 remains `in_progress` | feature-verify P1 domain and Device store |
| 2026-07-14 21:13 -0700 | Codex (GPT-5) as independent feature-verify P1 | Recomputed P1 evidence and audited the exact commit for domain, migration, persistence, compatibility, security boundaries, and regression scope; found three reproducible counterexamples not covered by the green suite; modified only verdict surfaces | `docs/reviews/phase1-device-kernel/2026-07-14-feature-verify-p1.md`, this file | `BLOCKED`; no external dependency | feature-build P1 correction |
| 2026-07-14 21:13 -0700 | operator-directed project-system writer via `mad-dashboard-sync` | Rebound manual dashboard judgment to the persisted P1 `BLOCKED` verifier verdict without changing the verdict or starting P2 | `docs/workflow/project/dashboard-state.json`; generated dashboard state; this file | focus expects `BLOCKED`; Phase 1 remains `in_progress` | feature-build P1 correction |
| 2026-07-14 21:17 -0700 | Codex (GPT-5) as feature-build P1 correction | Cleared all three verifier findings without expanding P1: repository Resume now validates the terminal source's canonical Capability snapshot and exact equality; Lease acquisition enforces monotonic time; ID validation enforces the same bounded grammar as creation; added missing-capability, snapshot expansion/removal, rollback, time reversal, and ID boundary tests; reran the complete evidence set | `internal/domain/types.go`; `internal/domain/lease.go`; domain tests; `internal/storage/repository.go`; storage tests; this file | `READY_FOR_VERIFY`; no remaining P1 build blocker | feature-verify P1 domain and Device store |
| 2026-07-14 21:17 -0700 | operator-directed project-system writer via `mad-dashboard-sync` | Rebound dashboard focus from the persisted verifier block to the persisted corrected `READY_FOR_VERIFY` state; regenerated mirrors and machine facts without advancing P1/P2 | `docs/workflow/project/dashboard-state.json`; generated dashboard state; this file | dashboard verification passes; Phase 1 remains `in_progress` | feature-verify P1 domain and Device store |
| 2026-07-14 21:22 -0700 | Codex (GPT-5) as independent feature-verify P1 v2 | Confirmed closure of all three domain findings and reran local evidence; created Draft PR #13 to obtain real three-platform execution; retained the Windows failure instead of treating cross-build as runtime proof; modified only verdict surfaces | `docs/reviews/phase1-device-kernel/2026-07-14-feature-verify-p1-v2.md`, this file; GitHub runs `29388679737`, `29388679739` | `BLOCKED`; Windows Device Store cannot open its test database | feature-build P1 Windows SQLite path correction |
| 2026-07-14 21:22 -0700 | operator-directed project-system writer via `mad-dashboard-sync` | Rebound dashboard focus to the persisted Windows-runtime `BLOCKED` verdict without changing P1 scope or starting P2 | `docs/workflow/project/dashboard-state.json`; generated dashboard state; this file | focus expects `BLOCKED`; Phase 1 remains `in_progress` | feature-build P1 Windows SQLite path correction |
| 2026-07-14 21:25 -0700 | Codex (GPT-5) as feature-build P1 Windows correction | Corrected SQLite DSN construction for absolute Unix, Windows drive-rooted, and UNC paths using explicit valid file URI shapes and percent encoding; retained private-root and regular-file checks; added mapping fixtures and reran the complete local P1/repository suite without modifying other phases | Store code/tests; this file | `READY_FOR_VERIFY`; new Windows runner required | feature-verify P1 on Draft PR #13 |
| 2026-07-14 21:25 -0700 | operator-directed project-system writer via `mad-dashboard-sync` | Rebound dashboard focus from the persisted Windows block to the persisted corrected `READY_FOR_VERIFY` state; regenerated mirrors and facts without inferring a Windows pass | dashboard manual/generated state; this file | dashboard verification passes; actual runner remains required | feature-verify P1 on Draft PR #13 |
| 2026-07-14 21:30 -0700 | Codex (GPT-5) as independent feature-verify P1 v3 | Verified the exact corrected head, retained prior finding closure, inspected actual Windows Server 2025 domain/Store/migration/scaffold/desktop results, and confirmed all seven protected PR checks; modified only verdict surfaces | `docs/reviews/phase1-device-kernel/2026-07-14-feature-verify-p1-v3.md`, this file; GitHub runs `29388956859`, `29388956809` | `VERIFIED`; P2 unlocked; Draft PR #13 remains unmerged | feature-build P2 identity, IPC, and Daemon lifecycle |
| 2026-07-14 21:30 -0700 | operator-directed project-system writer via `mad-dashboard-sync` | Rebound manual dashboard judgment to the persisted P1 `VERIFIED` verdict and exposed P2 as the next executable phase without advancing it or closing the Security Gate | dashboard manual/generated state; this file | focus expects `VERIFIED`; Phase 1 remains `in_progress`; P2 remains unstarted | feature-build P2 identity, IPC, and Daemon lifecycle |
