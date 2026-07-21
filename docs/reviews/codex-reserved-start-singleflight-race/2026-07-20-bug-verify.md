# Bug verification: Codex reserved start singleflight race

## Verdict

`READY_TO_SHIP` — the original stale pre-gate Session-read race is independently
reproduced on the base revision and is closed by the scoped patch. The
deterministic late-caller schedule, single-P stress, race detector, adjacent
Codex package, full Go suite, and vet all pass. No Provider process, credential,
CI rerun, implementation repair, commit, push, or PR action was used by this
verification.

## Classification and scope

```text
Owner: provider
Confidence: high
Why: the changed contract is Codex RuntimeManager exact-once reserved start
Impacts: core (durable Session lifecycle), project-system (verification records)
Branch: codex/provider/codex-reserved-start-singleflight-race
Workflow: bugfix
Gates: Provider compatibility resolved; Security Gate none
Docs: docs/workflow/features/codex-reserved-start-singleflight-race/dev_log.md
```

The exact implementation diff is limited to
`internal/providers/codex/runtime.go` and
`internal/providers/codex/runtime_test.go`. It adds a nil-by-default scheduling
seam, moves per-Session gate acquisition before the durable Session read, makes
waiters re-enter gate competition after wake, and strengthens the existing
focused regression. There are no Store CAS, SQLite, compatibility matrix,
platform-support, fixture-budget, Named Pipe, Device, or Provider-call changes.

## Original failure reproduced

- Read-only GitHub evidence for run `29805832734`, macOS job `88556037636`
  confirms head `aa9b52639dddaa3c1125335a298febf1b2beeb26`, conclusion `failure`, and
  `runtime_test.go:399: ... err=conflict: provider session identity changed`.
- An independent archive of exact base
  `2d3b4162b72bff26d203c55bb63782b725464f87` was tested outside the worktree:

  ```text
  GOMAXPROCS=1 go test -count=100 \
    -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' \
    ./internal/providers/codex
  ```

  Result: `FAIL` with repeated instances of the same line-399
  `provider session identity changed` conflict. This independently confirms the
  diagnosis ledger's exact `59/100` base run as a real schedule-sensitive
  production race rather than a CI-only timeout.

## Diff and boundary review

- **Gate before read:** the reserved request is validated, then the gate keyed
  by `reserved.SessionID` is acquired before `Store.Session` is called.
- **Waiter loop and revalidation:** a waiter observes gate closure, loops, wins
  or waits on the next gate, and only an elected owner performs the fresh
  durable read and full Device/Account/Credential/Profile/Workspace/Provider
  tuple validation.
- **Cancellation:** a waiting caller selects on `ctx.Done()` and returns the
  stable `deadline_exceeded` error without becoming a second Provider leader or
  invoking cleanup for another caller's binding.
- **Running and terminal states:** after the fresh tuple-checked read, every
  status beyond `starting` returns the durable Session before discovery,
  materialization, spawn, `thread/start`, binding mutation, or `failStart`.
- **Failure boundary:** `failStart` is created only after an elected owner has
  fresh-read a still-`starting` Session. Existing compatibility, shared-runtime,
  spawn, handshake, thread-start, and storage-CAS failures retain their
  fail-closed behavior; a late caller can no longer fail or finalize the
  already-running leader result.
- **Exact-once postconditions:** the focused regression asserts identical
  successful results, `discover=1`, `spawn=1`, `thread/start=1`, `kills=0`, a
  durable `running` Session, no residual start gate, and the original
  binding/runtime still present and non-finalizing.

## Commands and results

```text
go test -count=1 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex
PASS (0.199s)

GOMAXPROCS=1 go test -count=100 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex
PASS

GOMAXPROCS=2 go test -race -count=20 -run '^TestRuntimeManagerStartsReservedSessionOnceAndFailsPostReservationDrift$' ./internal/providers/codex
PASS (8.870s)

go test -race -count=1 ./internal/providers/codex
PASS (24.415s)

go test -count=1 ./...
PASS (all Go packages)

go vet ./...
PASS

pnpm run workflow:verify
PASS: agents=10, skills=3, docs=17, edges=20, statuses=15

pnpm run ci:links
PASS: markdown_files=290

pnpm run ci:licenses
PASS after including the existing Cargo toolchain in PATH:
pnpm_groups=5, cargo_packages=418

pnpm --dir /private/tmp/codex-reserved-project-verify.N4O6Sr run project:verify
ENVIRONMENT LIMITATION: the isolated copy stopped at nested `npm` lookup with
`sh: npm: command not found`; no project assertion ran or failed. This is not a
product blocker because the directly relevant workflow/link/license checks and
all Go gates passed, the verdict writer did not change dashboard authority, and
protected CI remains required before merge.
```

## Findings

None. The repair is scoped to the diagnosed ordering defect and its focused
regression. Protected native CI remains a ship/merge gate and must not be
replaced by rerunning only the previously failed job.
