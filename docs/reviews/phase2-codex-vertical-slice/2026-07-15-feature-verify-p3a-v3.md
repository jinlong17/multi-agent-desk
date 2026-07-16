# Feature Re-verification: Codex Vertical Slice P3A Remediation v3

## Verdict

`VERIFIED`

The sole v2 Provider policy-amendment blocker is closed, every earlier P3A
clearing condition remains green, and no new P3A acceptance, architecture,
security, compatibility, or regression finding was identified. P3B remains the
next approved phase; this verdict does not claim the credentialed Linux exit,
real Windows Codex support, Security acceptance, Ship, release, or deployment.

## Scope

- Owner: `provider`; secondary impacts: `core`, `security`, `project-system`
- Branch: `codex/provider/phase2-codex-vertical-slice`
- Re-read the current state authority, frozen Approval API mapping, and v2
  verification report
- Inspected the complete P3A diff and the focused policy-amendment remediation
- Wrote no implementation, plan, compatibility, dashboard, generated, Git, or
  remote state

## Fresh commands and results

| Command/check | Result |
|---|---|
| `go test -count=1 ./...` | pass |
| `go vet ./...` | pass |
| `go test -count=1 -race ./...` | pass |
| `go test -count=30 ./internal/providers/codex -run 'Test(CommandApprovalRejectsPolicyAmendmentsButAllowsNull\|RuntimeManagerRejectsProviderPolicyAmendmentsWithoutResponse\|RuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay\|RuntimeManagerRetiresStoppedTurnsWithoutFailingSharedRuntime\|RuntimeManagerPersistsTruthfulUsageSuccessAndDegradation\|RuntimeManagerApprovalDecisionTable\|RuntimeManagerPermissionsApprovalFailsClosedWithoutResponse\|RuntimeManagerRejectsPersistentAndPolicyApprovalVariants)'` | pass |
| macOS arm64 `go build ./cmd/...` | pass |
| Linux amd64 CGO-disabled `go build ./cmd/...` | pass |
| Windows amd64 CGO-disabled `go build ./cmd/...` | pass |
| `gofmt -d` over changed/untracked Go files; `git diff --check` | clean/pass |
| direct workflow, dashboard-static, Actions, CODEOWNERS, gate-fixture, link, and license verifiers | pass: agents=10, skills=3, docs=17, edges=20, dashboard branch correct/dirty=20, links=219, pnpm groups=5, Cargo packages=418 |

The composite project verifier was not invoked because it regenerates
dashboard state, which is outside the verdict-writer write allowance. Its
direct non-writing verification components passed. Cross-platform results are
compile evidence; P3B still owns native credentialed Linux execution.

## v2 blocker closure

### Non-null Provider amendments fail before persistence or response

`DecodeApprovalServerRequest` now checks both exact command-request fields:

- `proposedExecpolicyAmendment`
- `proposedNetworkPolicyAmendments`

An absent field or explicit JSON `null` is accepted as the normal exact command
Approval shape. Any non-null JSON value returns
`provider_version_unsupported` before the mapper produces an Approval event.
The event pump consequently fails the affected runtime closed without creating
an Approval record and without emitting a JSON-RPC response.

Fresh mapper tests cover both non-null fields and the explicit-null positive
case. Fresh runtime tests independently cover both amendment types and prove
Session failure, zero persisted Approvals, and zero Provider responses. The
tests pass for 30 uncached repetitions alongside the full race suite.

## Prior clearing-condition regression review

- Bounded writes: one owned writer, close-on-timeout, durable ambiguity, no
  replay, and cleanup regression remain green.
- Binding lifecycle: delayed retired-turn output/completion is isolated;
  force-kill survives interrupt rejection; unrelated thread IDs fail closed.
- Usage: exact supported shape and persisted `schema_changed`/`error` evidence
  remain truthful and green.
- Approval table: command/file approve-deny-cancel mapping, written states,
  replay/conflict, permissions no-response, `acceptForSession`/invented local
  variants, and Provider policy amendments are all covered by negative or
  positive tests as appropriate.
- Shared runtime, daemon-owned profile mapping, Provider thread persistence,
  input/resize behavior, lease refresh, crash fan-out, Fake regression, and
  three-platform compilation remain green through the full suite.

## Findings

None for P3A.

P3B still requires reproducible sanitized credentialed Linux Session,
second-CLI attach/replay/lease/turn input, structured Approval/Usage,
binding-scoped controls, typed resize unsupported, and frozen Resume evidence.
Final Security Review remains open after later phases.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P3A remediation v3`
**Verdict**: `VERIFIED`
**Summary**: `The sole Provider policy-amendment blocker is closed and every P3A acceptance, regression, platform-build, and governance check passes.`
**Evidence**: `fresh full Go/vet/race, 30x focused remediation/regression suite, macOS/Linux/Windows builds, formatting/diff/governance checks, frozen API comparison, and complete P3A diff inspection.`
**Findings**: `none for P3A.`
**Blockers**: `none for P3A; credentialed Linux P3B evidence and final Security Review remain later gates.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice` P3B.
