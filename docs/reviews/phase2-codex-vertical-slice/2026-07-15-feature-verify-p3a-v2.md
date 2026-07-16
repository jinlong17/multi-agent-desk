# Feature Re-verification: Codex Vertical Slice P3A Remediation

## Verdict

`BLOCKED`

Three prior findings are cleared, and the exact standard Approval decision
table is now exercised. One Provider-request policy-amendment path remains
actionable despite the frozen contract declaring it disabled. P3A therefore
cannot advance to P3B yet.

## Scope

- Owner: `provider`; impacts: `core`, `security`, `project-system`
- Branch: `codex/provider/phase2-codex-vertical-slice`
- Inspected the complete uncommitted P3A diff and the remediation against the
  prior `2026-07-15-feature-verify-p3a.md` findings
- Preserved later credentialed Linux P3B and final Security Review gates
- Did not modify implementation, plan, compatibility, dashboard, generated
  state, Git history, or remote state

## Fresh commands and results

| Command/check | Result |
|---|---|
| `go test -count=1 ./...` | pass |
| `go vet ./...` | pass |
| `go test -count=1 -race ./...` | pass |
| `go test -count=20 ./internal/providers/codex -run 'Test(RuntimeManagerBlockedApprovalWriteIsBoundedAndCannotReplay\|RuntimeManagerRetiresStoppedTurnsWithoutFailingSharedRuntime\|RuntimeManagerGracefulInterruptRejectionRestoresBinding\|RuntimeManagerUnknownThreadStillFailsClosed\|RuntimeManagerPersistsTruthfulUsageSuccessAndDegradation\|RuntimeManagerApprovalDecisionTable\|RuntimeManagerPermissionsApprovalFailsClosedWithoutResponse\|RuntimeManagerRejectsPersistentAndPolicyApprovalVariants)'` | pass |
| macOS arm64 `go build ./cmd/...` | pass |
| Linux amd64 CGO-disabled `go build ./cmd/...` | pass |
| Windows amd64 CGO-disabled `go build ./cmd/...` | pass |
| `gofmt -d` over changed and untracked Go files; `git diff --check` | clean/pass |
| direct workflow, dashboard-static, Actions, CODEOWNERS, gate-fixture, link, and license verifiers | pass: agents=10, skills=3, docs=17, edges=20, dashboard branch correct/dirty=19, links=218, pnpm groups=5, Cargo packages=418 |

The composite project verifier was not run because it regenerates dashboard
state, a write outside the verdict-writer allowance. All direct non-writing
components passed. Cross-platform results are compile evidence only; no native
Linux P3B or real Windows Codex claim is made.

## Clearing-condition review

### 1. Bounded single-owner writes — cleared

All frames now pass through one client-owned writer goroutine. Each enqueue and
write wait has a bounded context; timeout fails the client and closes the
owned transports, allowing the production pipe writer to unblock without a
second writer. The blocked-Approval regression proves bounded return,
`expired/ambiguous`, no response observation, no replay, and cleanup through
runtime close. Full race and repeated focused suites pass.

### 2. Binding stop/kill isolation — cleared

Stopped/killed active turns retain bounded thread/turn tombstones. Delayed
agent output and terminal turn events for that exact retired tuple are ignored,
while unrelated thread IDs still fail the shared runtime closed. Force kill
releases only its binding after an interrupt rejection; graceful stop restores
the binding. The two-binding survivor tests pass repeatedly.

### 3. Truthful Usage evidence — cleared

The exact Usage response shape now validates summary value types and daily
bucket fields/types/dates. Supported results carry `official/high`, source
version, observation time, and a raw integrity digest. Schema drift and
Provider errors persist separate low-confidence `schema_changed` and `error`
snapshots with bounded error codes and `unknown` windows. The Application
surface derives availability from the latest capability status rather than the
existence of stale rows.

### 4. Approval decision and disabled variants — not fully cleared

The runtime now proves command-execution and file-change
`approve|deny|cancel -> accept|decline|cancel`, durable written states,
same-digest replay, different-digest conflict, and permissions no-response.
It also rejects invented local decisions such as `approve_for_session` and
`update_policy`.

However, the exact Provider command-Approval request contains
`proposedExecpolicyAmendment` and `proposedNetworkPolicyAmendments`. The decoder
accepts both as raw mapped fields but never rejects a non-null amendment. The
runtime then persists the request as an ordinary actionable
`commandExecution` Approval, and a normal local `approve` emits
`{"decision":"accept"}`. The test named
`TestRuntimeManagerRejectsPersistentAndPolicyApprovalVariants` sends a normal
Provider request with neither amendment and tests only invalid local decision
strings, so it does not exercise this Provider-request path.

This conflicts directly with the frozen API statement that exec-policy and
network-policy amendments are disabled. Digest-only retention protects payload
secrecy but does not disable the semantic action.

## Blocking finding

### P1 — Non-null Provider policy amendments remain actionable

Reproduction by inspection:

1. Send exact 0.144.2 `item/commandExecution/requestApproval` with the normal
   thread/turn/item fields and a non-null `proposedExecpolicyAmendment` or
   `proposedNetworkPolicyAmendments`.
2. `DecodeApprovalServerRequest` accepts the mapped raw field and returns a
   normal `commandExecution` Approval.
3. `routeInbound` persists it pending.
4. A standard local `approve` maps to `accept` and writes a Provider response.

Clearing role: original `feature-build` writer.

Required evidence: reject any non-null exec-policy or network-policy amendment
before Approval persistence and before any Provider response, while continuing
to accept the exact no-amendment/null command Approval. Add deterministic tests
for both amendment fields, persisted-Approval absence, no Provider response,
and fail-closed runtime behavior. Retain the existing `acceptForSession`,
permissions, decision-table, replay, and conflict regressions.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P3A remediation v2`
**Verdict**: `BLOCKED`
**Summary**: `Bounded writes, binding isolation, Usage degradation, and the standard Approval table pass, but non-null Provider exec/network policy amendments remain actionable despite the frozen disabled contract.`
**Evidence**: `fresh full Go/vet/race, 20x focused clearing-condition suite, macOS/Linux/Windows builds, formatting/diff/governance checks, exact schema inspection, and complete remediation diff review.`
**Findings**: `P1: DecodeApprovalServerRequest accepts proposedExecpolicyAmendment and proposedNetworkPolicyAmendments, while the disabled-variant test covers only invalid local decision strings.`
**Blockers**: `original feature-build writer must reject non-null Provider policy amendments before persistence/response and add exact no-write regressions.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice` P3A policy-amendment remediation.
