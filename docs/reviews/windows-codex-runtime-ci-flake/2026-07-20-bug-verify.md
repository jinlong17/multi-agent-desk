# Bug verification: Windows Codex runtime CI deadline flake

## Verdict

`READY_TO_SHIP` — the test-only repair starts the unchanged five-second
runtime-operation budget after the SQLite/filesystem fixture has completed,
the deterministic regression proves that ordering, and fresh native Windows
CI executed the complete Codex package twice without either original failure.

## Classification and locked revision

```text
Owner: provider
Confidence: high
Why: the only implementation change is the Codex RuntimeManager test fixture contract
Impacts: project-system (Windows required CI)
Branch: codex/provider/windows-codex-runtime-ci-flake
Workflow: bugfix
Gates: Provider compatibility remains resolved; Security Gate none
Verified head: 6c53c6869cccf8081f3c1b3a7130d3a724123a02
Base: 154f882e8baea8165b729dfa53d7bdf4d3e546f1
PR: https://github.com/jinlong17/multi-agent-desk/pull/24
```

## Scope review

The product diff against the locked base changes only
`internal/providers/codex/runtime_test.go` (`44` additions, `46` deletions).
It splits setup from operation-context creation, creates the same bounded
five-second context only after setup, moves all 15 same-shaped RuntimeManager
tests onto that fixture contract, and adds
`TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget`.

No RuntimeManager production code, Store/SQLite policy, migrations, Provider
capabilities, platform support claim, Named Pipe code, dashboard state, or
security boundary changed. The other two PR paths are the pre-existing bug
diagnosis and state log required by the bugfix workflow. `git diff --check`
passed.

## Independent local verification

All commands ran from the locked worktree on Darwin arm64:

| Check | Result |
|---|---|
| `go test -count=100 -run '^(TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget|TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` | PASS; 300 targeted executions, package `20.342s` |
| `go test -race -count=10 -run '^(TestRuntimeManagerFixtureSetupDoesNotConsumeOperationBudget|TestRuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent|TestRuntimeManagerKeepsConcurrentAccountsAndUsageIsolated)$' ./internal/providers/codex` | PASS; 30 targeted race executions, package `21.344s` |
| `go test -count=1 ./...` | PASS; all Go packages, Codex `5.087s` |
| `go vet ./...` | PASS; no findings |
| `go test -count=100 -run '^TestNativeTwoClientFakeSessionControl$' ./internal/device` | PASS; 100 adjacent executions, package `78.909s` |

The new ordering assertion directly checks that the operation deadline is not
earlier than `setupFinished + operationTimeout`; it does not sleep, remove the
deadline, or widen the five-second operation budget used by RuntimeManager
tests. The existing safe error-wrapping implementation and diagnostic causal
trace remain unchanged.

## Native Windows and protected checks

PR #24 checked out merge ref `3472cdb`, which merges exact head `6c53c68` into
base `154f882`, on Microsoft Windows Server 2025 `10.0.26100`, image
`windows-2025-vs2026@20260714.173.1`, with Go `1.26.5 windows/amd64`.

Windows job `88525423470` passed both complete, unfiltered Go-suite executions:

- direct `go test -count=1 ./...`: `internal/providers/codex` passed in
  `32.059s`;
- `npm run scaffold:verify` repeated `go test ./...` and the Codex package
  passed in `34.714s`.

Because neither command used a test-name filter, both original failing tests
and the new regression ran in each Codex package execution. The job completed
successfully, so the original `session could not be read` and
`database transaction could not start` symptoms did not recur.

The first Ubuntu job `88523410234` is preserved as a non-target signal: its
direct Go suite passed Codex, but the later scaffold repetition failed only
`internal/device/TestNativeTwoClientFakeSessionControl` at line 173 with
`conflict: session status changed`. The implementation diff does not touch
that package. The single failed-job rerun `88525411694` passed both Device and
Codex packages twice. Local 100-count stress of that Device test also passed.
This evidence does not erase the first failure; it shows that the unrelated
signal does not invalidate this Provider test-harness repair.

At exact head `6c53c68`, all seven required checks are final green:
`project-verify`, `build-ubuntu`, `build-macos`, `build-windows`,
`license-gate`, `dco`, and `link-check`.

Local `workflow:verify` passed (`agents=10`, `skills=3`, `docs=17`,
`edges=20`, `statuses=15`), local link verification passed for 283 Markdown
files, and `git diff --check` passed. A direct local `dashboard:verify` without
the preceding writer-owned `dashboard` refresh reported `generated commit is
stale`; the verifier did not mutate the ignored generated snapshot because
verdict writers may write only this report and the target log. The canonical
`project:verify` chain generates that snapshot before checking it, and that
exact chain passed on PR #24 (`project-verify` job `88525424090`).

## Findings

No blocking or scope-creep finding. The independent Windows Named Pipe
close/read hang and the observed Ubuntu Device E2E flake remain separate bug
units; neither is repaired or reclassified by this verdict.
