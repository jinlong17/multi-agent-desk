# Bug diagnosis: Windows Codex runtime CI deadline flake

## Verdict

`DIAGNOSED` — the two Windows-only Codex failures are different visible
consequences of the same expired five-second test context. The context starts
before fixture database migration and filesystem setup; hosted-runner
scheduling can consume the full budget before the assertions use it.

## Classification

```text
Owner: provider
Confidence: high
Why: both symptoms are in Codex runtime/account-isolation tests and do not alter Device Kernel storage semantics
Impacts: project-system (Windows CI)
Branch: codex/provider/windows-codex-runtime-ci-flake
Workflow: bugfix
Gates: existing exact Provider compatibility remains resolved; Security Gate none
Docs: docs/workflow/features/windows-codex-runtime-ci-flake/dev_log.md
```

## Reproduction evidence

All three attempts used
`main@154f882e8baea8165b729dfa53d7bdf4d3e546f1`, Go `1.26.5`, and GitHub
Windows Server 2025 image `windows-2025-vs2026@20260714.173.1`.

1. [Attempt 1, job 88510801096](https://github.com/jinlong17/multi-agent-desk/actions/runs/29790417988/job/88510801096):
   `TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent`
   failed after `5.42s` at runtime test line 294 with
   `conflict: session could not be read`.
2. [Attempt 2, job 88511433690](https://github.com/jinlong17/multi-agent-desk/actions/runs/29790417988/job/88511433690):
   `TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated` failed after
   `9.14s` at line 458 with
   `conflict: database transaction could not start`.
3. [Attempt 3, job 88511790401](https://github.com/jinlong17/multi-agent-desk/actions/runs/29790417988/job/88511790401):
   the direct Go step passed the Codex package in `18.752s`; the scaffold step
   ran it again and passed in `20.115s`; the complete Windows job succeeded.

The local Darwin control passed 100 repetitions of both tests and 10 race
repetitions. It does not reproduce Windows scheduling, but it rejects a
deterministic account/runtime isolation failure on the unchanged source.

## Causal trace

```text
runtime test creates 5 s context
  -> runtimeManagerFixture uses context.Background
     -> SQLite Open + six migrations + fixture rows + temp filesystem
        can exceed 5 s under Windows package scheduling
  -> next operation uses the already-expired test context
     -> QueryRow: "session could not be read"
     or BeginTx: "database transaction could not start"
```

The exact source chain is:

- timeout before fixture: `internal/providers/codex/runtime_test.go:260-263`
  and `:451-456`;
- unbounded setup context: `runtime_test.go:207-256` and
  `materialization_test.go:63-83`;
- read wrapper: `internal/storage/repository.go:741-755`;
- transaction wrapper: `internal/storage/store.go:412-415`;
- safe public error formatting and retained internal cause:
  `internal/domain/errors.go:72-97`.

The failure messages alone do not print `context deadline exceeded` because the
domain error deliberately suppresses the internal cause in `Error()` while
preserving it in `Unwrap()`. The timing, source order, changing failure site,
and two unchanged-SHA passes in attempt 3 provide the evidence needed to call
the root cause without guessing.

## Impact

- Required Windows CI can fail spuriously after a correct Provider or docs
  change, causing unnecessary reruns and delaying protected-main integration.
- The observed failures do not show lost account isolation, corrupted Session
  state, competing SQLite writers, or a production busy-timeout defect.
- Other runtime tests in this file share the same context-before-fixture
  pattern and are exposed to the same scheduling class even though they did not
  fail in these attempts.

## Minimum repair and regression shape

Move runtime-operation context creation after fixture setup for every test that
uses `runtimeManagerFixture`. Keep bounded deadlines and the existing polling
budgets. Do not modify runtime synchronization, Store transaction policy,
SQLite `busy_timeout`, migrations, or Provider behavior.

Verification must include:

- both original test names repeated on Windows;
- the full `go test -count=1 ./...` Windows step;
- a fresh complete Windows CI job;
- a narrow assertion that an intentionally expired context unwraps to
  `context.DeadlineExceeded`, so future timeout failures are classified
  correctly;
- independent `bug-verify` before Ship.

## Explicit boundary

The independent Windows Named Pipe `Close`/`Read` hang is outside this unit. It
occurs in a different package/path and must not be mixed into this Provider
test-only repair.
