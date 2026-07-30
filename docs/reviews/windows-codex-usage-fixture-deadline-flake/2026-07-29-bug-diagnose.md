# Bug diagnosis: Windows Codex usage-fixture deadline flake

**Verdict:** `DIAGNOSED`

## Scope

Provider-owned test-harness flake in
`internal/providers/codex/runtime_test.go`; `project-system` is a secondary CI
impact. No production code, compatibility claim, security boundary, CI run,
PR, push, or existing shipped bug unit was changed.

## Evidence-backed cause

The common runtime fixture overrides every Codex client call to a 100 ms wait
at `runtime_test.go:85`. The usage-isolation test makes three fixture-backed
`ReadUsage` calls and the truthful-usage test does the same for each case. A
fixture server goroutine must receive, decode, and answer each RPC. On Windows
runner scheduling, that workflow can exceed the test-only 100 ms override.

`Client.Call` starts a timer using `waitDuration()` and emits
`CodeDeadlineExceeded` / `codex protocol call timed out` when it fires.
`waitDuration()` otherwise defaults to five seconds. The 100 ms setting
therefore turns scheduler variation into a failed test even though neither the
production protocol default nor the test's five-second operation context has
expired.

The older `windows-codex-runtime-ci-flake` was separately fixed by moving the
five-second operation context after fixture setup. Its shipped diff did not
remove this global fixture `MaxWait`, so reopening or editing that old unit
would be inaccurate.

## Smallest repair and regression shape

Keep a scheduler-tolerant deadline on the common fixture and apply a short
deadline only to the blocked-write test that specifically asserts bounded
failure. Add a deterministic check that a normal usage fixture does not carry
the short deadline; retain the blocked-write upper-bound assertion. The writer
should then run targeted repeat coverage and native Windows CI before handing
off to `bug-verify`.

## Limit

Per operator direction, this diagnosis did not rerun CI or tests. The reported
Windows failure and direct source/history trace are sufficient to localize the
test-only cause, but not to claim a refreshed Windows pass.
