# Feature Verify: Codex Vertical Slice P2B

## Verdict

`BLOCKED` on required platform evidence, not on a reproduced implementation
failure. The refrozen P2B tree passes the complete native macOS deterministic,
race, migration, regression, build, formatting, workflow, dashboard, and CI
contract suite. The approved P2B acceptance, however, requires the portable
Vault/initialization/auth tests to execute on macOS, Linux, and Windows. Only
macOS execution is present; Linux and Windows were cross-built but were not run
on native CI runners. Cross-build success does not establish portable crypto
round-trip, filesystem permission/DACL, initialization, recovery, or enrollment
behavior on those operating systems.

## Acceptance verification

| Area | Evidence | Result |
|---|---|---|
| Migration and initialization | forward-only `0005`, literal `BEGIN IMMEDIATE`, insert-if-absent singleton, same-request replay, competing initializer, pre/post-commit and corrupt-state coverage | PASS on macOS |
| Vault v1 | frozen Argon2id/AES-256-GCM/AAD vector, fresh DEK/nonces, strict bounds and metadata, wrong-password/tamper/corruption handling, item/revision CAS | PASS on macOS |
| Official enrollment | owner binding, one-active constraint, exact interactive mode, deadline/cancel/restart recovery, post-validation import, lost-response replay, private staging, prior healthy revision preservation | PASS on macOS |
| Production credential source | Vault-backed materialization and atomic refresh commit, revision/digest validation, secret buffer zeroing, fail-closed missing sink | PASS on macOS |
| Logout and Session exclusion | durable revocation reservation precedes filesystem cleanup; Session insertion checks the reservation transactionally; final Vault/status/reservation commit is retryable and fail closed | PASS on macOS |
| Approval dispatch state | approve/deny/cancel claim, written completion, same-digest replay, conflict, failed dispatch, restart ambiguity, idle/dispatching expiry | PASS on macOS |
| Regression and governance | full Go suite, vet, focused race suite including CLI, Fake compatibility, formatting/diff checks, workflow/dashboard/Actions/CODEOWNERS/fixture/link/license contracts | PASS |
| Portable platform execution | native macOS tests executed; Linux amd64 and Windows amd64 binaries cross-built; no Linux or Windows test runner result exists for this refrozen tree | BLOCKED |

## Commands and results

```text
go test -count=1 ./...                                                PASS (macOS arm64)
go vet ./...                                                         PASS
go test -count=1 -race ./internal/vault ./internal/storage \
  ./internal/app ./internal/providers/codex ./cmd/multidesk            PASS (macOS arm64)
go build ./cmd/multidesk and ./cmd/multidesk-server                  PASS (macOS arm64)
GOOS=linux GOARCH=amd64 go build both commands                       PASS (cross-build only)
GOOS=windows GOARCH=amd64 go build both commands                     PASS (cross-build only)
gofmt -d over changed/untracked Go files                             PASS (no output)
git diff --check                                                     PASS
node scripts/workflow/verify-workflow.mjs                            PASS (agents=10, skills=3, docs=17, edges=20, statuses=15)
node scripts/dashboard/verify-static.mjs                             PASS (branch correct, dirty=54, phases=9)
node scripts/ci/verify-actions.mjs                                   PASS (checks=7, actions=15)
node scripts/ci/verify-codeowners.mjs                                PASS (owner=@jinlong17)
node scripts/ci/test-gates.mjs                                       PASS
node scripts/ci/check-local-links.mjs                                PASS (markdown_files=215)
node scripts/ci/verify-licenses.mjs                                  PASS (pnpm_groups=5, cargo_packages=418)
```

The refrozen tree remained stable across the final matrix. Earlier results
collected while the logout/session-start remediation was being edited were
discarded and are not evidence for this verdict.

## Finding

### P0 — Mandatory Linux and Windows portable-backend execution is absent

The approved test strategy requires native macOS plus CI Linux/Windows
round-trip for the password-derived backend, including the permission/DACL
abstraction. The feature brief also requires initialization, wrong-password,
tamper, hostile-parameter, and crash-boundary behavior to fail closed on all
three platforms. The current checkout has a correctly configured three-OS CI
matrix, but no remote branch or CI result for this uncommitted tree. Local
Linux/Windows cross-builds prove compilation only. P2B therefore cannot
truthfully transition to `VERIFIED` yet.

## Clearing requirement

Run the refrozen P2B Vault/initialization/auth deterministic Go tests on native
Linux and Windows CI runners (macOS is already green), retain the exact runner
results, and rerun independent `feature-verify P2B`. No implementation change
is required unless either platform run exposes a failure.

The credentialed real Linux Codex Session, real Windows Codex compatibility,
P3A runtime/Approval Provider dispatch, P3B live exit, P4 matrix work, and final
Security Review remain later gates and are not claimed here.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P2B`
**Verdict**: `BLOCKED`
**Summary**: `P2B implementation and native macOS verification are green, but the approved portable-backend acceptance lacks executed Linux and Windows test evidence.`
**Evidence**: `go test -count=1 ./...`; `go vet ./...`; focused race suite; native macOS plus Linux/Windows cross-builds; formatting/diff and workflow/dashboard/CI governance — all passed; no native Linux/Windows P2B test result exists.
**Findings**: `P0 evidence gap: cross-builds do not satisfy the mandatory native Linux and Windows portable Vault/initialization/auth test execution.`
**Blockers**: `Run the P2B deterministic Go tests on native Linux and Windows CI runners, retain results, then rerun independent feature-verify P2B.`

### Next Step

Run `original writer` for `phase2-codex-vertical-slice` to obtain native Linux and Windows P2B test evidence, then rerun independent feature-verify P2B.
