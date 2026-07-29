# Bug verification: Windows Codex usage-fixture deadline flake

**Verdict:** `READY_TO_SHIP`

**Verified repair:** `c6dcf2ec1f34cc65e53868aad43876956dfe5e49`, carried unchanged through the native-Windows receipt `38f3e5bbd9942e479ba1827f8052cc021c860e64` and current docs-only PR head `92c4db05ee09bfcca40a3b15d93f77bf8ef06364`.

## Scope and race conclusion

`c6dcf2e` changes only `internal/providers/codex/runtime_test.go` and this bug
log. It parameterizes fixture construction: normal runtime-manager fixtures
create a five-second `MaxWait` before `RuntimeManager.Start` can schedule its
event pump, while the blocked-write test creates its intentional 100 ms
fixture before that same point. The prior post-start assignment is gone.
`Client.waitDuration` retains its five-second production fallback; no production
timeout, provider behavior, compatibility row, or Windows support claim changed.

The repair is an ancestor of `92c4db0`; `38f3e5b..92c4db0` changes only the bug
log. Thus the native Windows test result applies to the unchanged repaired code,
not merely to an earlier, different implementation.

## Independent evidence

| Command or inspection | Result |
|---|---|
| `git diff --check c6dcf2e^ c6dcf2e`; source review of fixture construction, `RuntimeManager.Start`, and `Client.waitDuration` | PASS — no whitespace errors, construction-time-only timeout configuration, no live `MaxWait` mutation, and unchanged five-second production fallback. |
| `git merge-base --is-ancestor 38f3e5b 92c4db0`; `git diff --name-status 38f3e5b 92c4db0` | PASS — the native-receipt SHA is an ancestor of current head; later changes are only `dev_log.md`. |
| `go test -count=1 ./internal/providers/codex`; `go vet ./internal/providers/codex`; `git diff --check` | PASS. |
| `pnpm run workflow:verify` with the bundled Node runtime | PASS — agents=10, skills=3, docs=17, edges=20, statuses=15. |
| PR #35 CI run `30438687149`, native job `90532479920` at exact `38f3e5b` | PASS — `build-windows` completed successfully and its `Run Go test suite` step (`go test -count=1 ./...`) succeeded. |

## Current PR-head state (not used to substitute for the repair receipt)

At verification, PR #35 is open/draft at docs-only `92c4db0`. Its new
`project-verify`, `license-gate`, `dco`, and `link-check` checks are successful;
`build-ubuntu`, `build-macos`, and `build-windows` remain in progress. Those
pending documentation-head checks must be allowed to finish before a merge, but
they do not invalidate the existing native-Windows receipt for the unchanged
repair and therefore do not block this bug-verification verdict.

No push, merge, implementation edit, or support-boundary change was made by
this verifier.
