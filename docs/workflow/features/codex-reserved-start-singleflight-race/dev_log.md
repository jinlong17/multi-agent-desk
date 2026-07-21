# Bug log: Codex reserved start singleflight race

## Status Panel

| Field | Value |
|---|---|
| Workflow | `BUGFIX` |
| Target | `codex-reserved-start-singleflight-race` |
| Title | `Codex reserved start singleflight race` |
| Owner Module | `provider` |
| Impacted Modules | `core, project-system` |
| Current Phase | `SHIP` |
| Status | `SHIPPED` |
| Executor | `Codex (GPT-5) as ship` |
| Updated | `2026-07-20 23:55 PDT` |
| Suggested Next | `none` |
| Branch / Worktree | `PR #29 merged to main@281c151; receipt reconciliation @ /Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/codex-reserved-start-singleflight-race-receipt` |
| Provider Gate | `resolved — no Provider compatibility or platform claim changes` |
| Security Gate | `none` |

## Reproduction

| Field | Value |
|---|---|
| Environment / versions | Base `origin/main@2d3b4162b72bff26d203c55bb63782b725464f87`; PR #28 head `aa9b52639dddaa3c1125335a298febf1b2beeb26`; GitHub macOS arm64 job `88556037636`, Go `1.26.5`; local Darwin arm64, Go `1.26.5` |
| Minimal reproduction | From exact PR head `aa9b526`, run `GOMAXPROCS=1 go test -count=100 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex`. |
| Expected behavior | Two concurrent `StartReserved` calls for one already-inserted `starting` Session perform exactly one Provider start; both callers receive the same durable running Session. |
| Actual behavior | `59/100` local single-P reproductions failed at `runtime_test.go:399` with `conflict: provider session identity changed`, matching PR #28 macOS job `88556037636`. A late caller can issue a second Provider `thread/start` from a stale `starting` Session snapshot. |

## Root cause (bug-diagnose)

`internal/providers/codex/runtime.go` performs the durable Session read before
acquiring the in-memory `m.starting[sessionID]` singleflight gate:

1. Two callers may both read the Session as `starting` at lines 245-255.
2. Caller A acquires the gate, starts the Provider thread, persists
   `provider_session_id`, transitions the Session to `running`, then removes
   and closes the gate in its deferred cleanup at lines 270-275.
3. Caller B can be descheduled after its old Session read but before the gate
   lock at line 257. If it resumes after A removed the gate, it sees an empty
   map and installs itself as a new leader while retaining the stale
   `starting` snapshot.
4. B reaches a second `thread/start`, then
   `Store.SetSessionProviderSessionID` at line 415. The storage CAS requires
   `status='starting'` and an empty Provider identity
   (`internal/storage/repository.go:828-837`), so it changes zero rows and
   returns the exact observed `conflict: provider session identity changed`.

This is a production idempotency race rather than a test-only timeout. The
error path calls `releaseBinding`, which can remove the successful binding,
finalize the shared runtime, and transition the already-running Session to
`failed`; therefore a successful CI rerun does not make the defect harmless.

PR #28 changes only Claude compatibility/spike documentation and has no diff
under `internal/providers/codex` or `internal/storage`. The race was already
present in `origin/main@2d3b416` after commits `a4c3f21` and `621a4a2`; those
commits repaired and verified the independent Windows fixture-deadline bug but
did not change this pre-gate Session read.

## Fix scope (smallest repair)

- Modify `internal/providers/codex/runtime.go` so a valid reserved request
  acquires or waits on the per-Session gate, keyed by `reserved.SessionID`,
  **before** reading durable Session state.
- The elected caller must read a fresh Session under that ownership, validate
  the complete tuple, and return immediately when its status is already beyond
  `starting`. A waiter returns the durable result after the active gate closes;
  a late caller that becomes a new gate owner reads the fresh `running` state
  and must not contact the Provider.
- Add focused regressions in
  `internal/providers/codex/runtime_test.go`: preserve the existing exact-once
  concurrent contract; stress the previously failing single-P schedule; assert
  one spawn/one Provider thread, two identical running results, and no Session
  failure or runtime finalization.
- Do not change Store CAS semantics, SQLite policy, Provider compatibility,
  platform gates, fixture operation budgets, or unrelated Named Pipe/Device
  flakes.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-20 23:03 PDT | REMOTE REPRO | PR #28 CI run `29805832734`, macOS job `88556037636`, `go test -count=1 ./...` | FAIL: `TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift (0.05s)`, `runtime_test.go:399`, `conflict: provider session identity changed`; Codex package failed in `2.064s` | [macOS job](https://github.com/jinlong17/multi-agent-desk/actions/runs/29805832734/job/88556037636) |
| 2026-07-20 23:07 PDT | SCOPE | compared PR #28 base `2d3b416` and head `aa9b526`; inspected PR file rollup | PR is documentation/spike-only; no `internal/providers/codex`, runtime, or storage change introduced the failure | [PR #28](https://github.com/jinlong17/multi-agent-desk/pull/28) |
| 2026-07-20 23:09 PDT | LOCAL CONTROL | targeted Darwin test repeated under default scheduling and race scheduling; parent orchestration also recorded count `300`, race count `100`, full Go and vet passes | PASS under ordinary scheduling; establishes intermittent schedule dependence but does not clear the remote failure | local command output |
| 2026-07-20 23:12 PDT | LOCAL REPRO | `GOMAXPROCS=1 go test -count=100 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex` at exact `aa9b526`, Darwin arm64 Go `1.26.5` | FAIL; identical symptom reproduced `59/100` times; package `7.366s` | `/tmp/reserved-gomaxprocs1-count100.log` (local transient evidence; not committed) |
| 2026-07-20 23:14 PDT | STATIC TRACE | inspected `RuntimeManager.start`, `SetSessionProviderSessionID`, `releaseBinding`, caller path in `internal/runtime/manager.go`, Git blame/history, and prior fixture stabilization commits | Evidence-backed TOCTOU located between the durable Session read and `m.starting` gate acquisition; duplicate start and destructive cleanup impact confirmed | `internal/providers/codex/runtime.go:239-278,304-421,618-650`; `internal/storage/repository.go:818-839` |
| 2026-07-20 23:23 PDT | PRE-FIX REGRESSION CONTROL | installed the focused channel-controlled late-caller schedule, then ran `GOMAXPROCS=1 go test -count=1 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex` before moving the gate | FAIL deterministically: leader returned the durable `running` Session while the paused late caller returned `conflict: provider session identity changed` at `runtime_test.go:409` | local command output; no Provider process or credential used |
| 2026-07-20 23:25 PDT | FIX | gate keyed by validated `reserved.SessionID` now precedes every durable Session read; waiters re-enter gate competition after wake; only the elected owner fresh-reads and validates the full tuple/status | PASS: a late caller observes the completed `running` Session without `Discover`, materialization, spawn, `thread/start`, or cleanup; Store CAS and fail-closed Provider paths are unchanged | `internal/providers/codex/runtime.go`; `internal/providers/codex/runtime_test.go` |
| 2026-07-20 23:27 PDT | TARGET STRESS | `GOMAXPROCS=1 go test -count=100 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex` | PASS, `11.644s`; each iteration proves two identical successful results, one discover/spawn/thread, zero kills, durable `running`, and intact binding/runtime | local command output |
| 2026-07-20 23:29 PDT | TARGET RACE | `GOMAXPROCS=2 go test -race -count=20 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex` | PASS, `15.992s`; bounded hook channels and late-caller completion are race-clean | local command output |
| 2026-07-20 23:31 PDT | PACKAGE / FULL | `go test -race -count=1 ./internal/providers/codex`; `go test -count=1 ./...`; `go vet ./...` on Darwin arm64 Go `1.26.5` | PASS: Codex race package `30.809s`; full Go including adjacent Provider/Core/Storage packages passed; vet passed | local command output |
| 2026-07-20 23:36 PDT | INDEPENDENT REPRO | read-only GitHub job `88556037636`; exact-base archive `2d3b416` with `GOMAXPROCS=1 go test -count=100 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex` outside the worktree | CONFIRMED: remote and independent base both fail at the same assertion with `conflict: provider session identity changed`; diagnosis ledger's `59/100` is consistent with the reproduced scheduling race | [bug-verify report](../../../reviews/codex-reserved-start-singleflight-race/2026-07-20-bug-verify.md) |
| 2026-07-20 23:39 PDT | INDEPENDENT TARGET / RACE | deterministic target; `GOMAXPROCS=1` count `100`; `GOMAXPROCS=2 go test -race` count `20`; `go test -race -count=1 ./internal/providers/codex` | PASS: exact-once assertions hold (`discover/spawn/thread=1`, `kills=0`), durable Session remains `running`, and binding/runtime remain intact; Codex package race passed in `24.415s` | [bug-verify report](../../../reviews/codex-reserved-start-singleflight-race/2026-07-20-bug-verify.md) |
| 2026-07-20 23:41 PDT | INDEPENDENT ADJACENT / GOVERNANCE | `go test -count=1 ./...`; `go vet ./...`; `pnpm run workflow:verify`; `pnpm run ci:links`; `pnpm run ci:licenses`; isolated `project:verify` attempt | PASS: all Go, vet, workflow, links, and licenses; isolated `project:verify` stopped before assertions because nested `npm` was absent from that diagnostic PATH, recorded as an environment limitation rather than a product failure | [bug-verify report](../../../reviews/codex-reserved-start-singleflight-race/2026-07-20-bug-verify.md) |
| 2026-07-20 23:50 PDT | PROTECTED PR | PR #29 locked head `776498b1470f734d515a9ded90fc517736326be4`; CI run `29808094137`; Governance run `29808094113` | PASS: project verification, Ubuntu/macOS/Windows builds, license, DCO, and link checks all succeeded; protected rebase merge landed as `281c151d120f53365ecbc1f9150c084ca28d1205` | [PR #29](https://github.com/jinlong17/multi-agent-desk/pull/29) |
| 2026-07-20 23:54 PDT | EXACT MAIN RECONCILIATION | `main@281c151`; CI run `29808322809`; Governance run `29808322805`; direct PR-head/main tree comparison | PASS: all seven checks succeeded on exact final main and the rebase-landed tree is content-identical to the locked PR head | [ship receipt](../../../reviews/codex-reserved-start-singleflight-race/2026-07-20-ship-receipt.md) |

## Risks and Blockers

- No blocker remains. The production race is fixed and reconciled on protected
  `main@281c151`; all seven required checks passed on both the locked PR head
  and exact final main.
- PR #28 must be refreshed onto this shipped fix and pass a new complete check
  matrix; the old failed macOS job must not be treated as a rerun-only flake.
- This unit is separate from the shipped Windows runtime fixture-deadline fix,
  the Windows Named Pipe close fix, and the unrelated Device E2E flake.
- No Provider process or credentialed Provider call was used during diagnosis.
- The fix changes only reserved-start singleflight ordering and its focused
  regression. Independent `bug-verify` is complete; protected native CI remains
  a ship/merge gate.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-20 23:15 PDT | Codex (GPT-5) as bug-diagnose | Classified the recurrence as Provider-owned with Core and CI impacts; inspected exact main/PR history and macOS logs; separated it from the prior deadline fixture bug; reproduced the same CAS conflict `59/100` times under single-P Darwin scheduling; traced the stale pre-gate Session read through duplicate Provider start and destructive cleanup | this file; `docs/reviews/codex-reserved-start-singleflight-race/2026-07-20-bug-diagnose.md` | `DIAGNOSED`; production singleflight ordering plus focused concurrency regression required; rerun-only is insufficient | `bug-fix` |
| 2026-07-20 23:31 PDT | Codex (GPT-5) as bug-fix | Added a deterministic late-caller regression that fails on the diagnosed ordering; moved reserved-start gate ownership before the fresh durable Session read; made waiters re-compete and revalidate rather than return an unchecked snapshot; verified identical success, exact-once Provider contact, zero finalization, durable running state, and intact runtime/binding | `internal/providers/codex/runtime.go`; `internal/providers/codex/runtime_test.go`; this file | `READY_FOR_VERIFY`; target stress, target/package race, full Go, and vet pass; no Store, compatibility, platform, or Provider-call changes | `bug-verify` |
| 2026-07-20 23:42 PDT | Codex (GPT-5) as bug-verify | Independently reproduced the base and remote CAS conflict; reviewed the exact two-file implementation diff and gate/read, waiter, cancellation, terminal, tuple, and failStart boundaries; ran deterministic, single-P stress, target/package race, full Go/vet, workflow, link, and license checks; checked exact-once durable/runtime postconditions and scope | this file; `docs/reviews/codex-reserved-start-singleflight-race/2026-07-20-bug-verify.md` | `READY_TO_SHIP`; no findings or product blockers; isolated project verifier lacked nested `npm`, while its directly relevant constituent checks passed and protected CI remains required | `ship` |
| 2026-07-20 23:55 PDT | Codex (GPT-5) as ship | Audited the exact signed fix commit and scoped diff; confirmed all seven protected checks on locked PR #29 head; rebase-merged the content-identical tree; verified the landed DCO signoff and all seven checks on exact final `main@281c151`; recorded rollback and receipt without claiming a tag, release, publish, or deploy | this file; `docs/reviews/codex-reserved-start-singleflight-race/2026-07-20-ship-receipt.md`; PR #29; main `281c151d120f53365ecbc1f9150c084ca28d1205` | `SHIPPED`; production singleflight race is repaired and remotely reconciled with no blocker | `none` |
