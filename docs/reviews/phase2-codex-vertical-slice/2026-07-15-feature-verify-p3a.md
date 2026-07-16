# Feature Verification: Codex Vertical Slice P3A

## Verdict

`BLOCKED`

P3A has substantial passing implementation and regression evidence, but the
approved Approval dispatch transaction is not bounded once the durable claim
has been written. A blocked app-server stdin can therefore leave an Approval
in `dispatching` indefinitely instead of returning and recording
`expired/ambiguous`. Binding termination and Usage degradation also do not yet
meet the frozen P3A contract. The original `feature-build` writer must correct
these conditions and provide deterministic regressions before P3A can be
verified.

## Scope and classification

- Owner: `provider` (high confidence)
- Secondary impacts: `core`, `security`, `project-system`
- Branch: `codex/provider/phase2-codex-vertical-slice`
- Workflow: `feature`, phase `P3A`
- Gates preserved: exact Provider compatibility, later credentialed Linux P3B,
  and final Security Review
- Diff inspected: all 18 P3A paths, including the new shared runtime and its
  tests; no implementation, plan, compatibility, or dashboard file was changed
  by this verdict writer

## Fresh verification evidence

| Command/check | Result |
|---|---|
| `go test -count=1 ./...` | pass |
| `go vet ./...` | pass |
| `go test -count=1 -race ./internal/providers/codex ./internal/runtime ./internal/app ./internal/storage` | pass |
| `go test -count=20 ./internal/providers/codex -run 'Test(ClientSingleReaderMultiplexesConcurrentCallsAndInbound\|RuntimeManagerSharesCredentialRuntimeAndKeepsBindingsIndependent\|RuntimeManagerChildCrashFailsAllBindingsOnce\|RuntimeManagerApproval)'` | pass |
| `GOOS=darwin GOARCH=arm64 go build ./cmd/...` | pass |
| `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/...` | pass |
| `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/...` | pass |
| `gofmt -d` over every changed/untracked Go file | clean |
| `git diff --check` | pass |
| direct workflow, dashboard-static, Actions, CODEOWNERS, gate-fixture, local-link, and license verifiers through the bundled Node runtime | pass: agents=10, skills=3, docs=17, edges=20, dashboard branch correct/dirty=18, links=217, pnpm groups=5, Cargo packages=418 |
| secret-term scan of the implementation diff | no credential, URL, email, Cookie, password, or bearer-token material found |

The composite `project:verify` wrapper was not run because it invokes the
dashboard generator, which writes generated dashboard state and is outside the
verdict-writer write allowance. Its direct non-writing verification components
were run and passed. Cross-platform results above are compile evidence; P3B
still owns native credentialed Linux Provider execution and no real Windows
Codex support claim is made.

## Findings

### P0 — Approval Provider writes are not bounded after the durable claim

`RuntimeManager.RespondApproval` first calls `ClaimApprovalDispatch`, then
calls `Client.RespondServerRequest`, and reaches `FailApprovalDispatch` only if
that call returns an error. `RespondServerRequest` checks `ctx.Done()` only
before entering `write`; `write` holds `writeMu` and calls `WriteFrame`
directly, with no deadline, cancellation path, or transport close tied to the
request context. The same unbounded write occurs before the response timer is
started for ordinary client calls.

Consequently, a child that remains alive but stops reading stdin can block the
goroutine forever after the Approval is durably `dispatching`. The required
`expired/ambiguous` transition is unreachable until an unrelated restart, the
local IPC request remains stuck, and the write is not demonstrably bounded.
The current test covers only an immediate writer error, not a blocked write.
This violates the approved claim-before-write, bounded-write, ambiguity, and
no-replay contract.

Clearing evidence: make Provider writes bounded/cancellable without creating an
unowned second writer; add a deterministic blocked-stdin test proving that the
claimed Approval becomes `expired/ambiguous`, no automatic replay occurs, and
all waiters/runtime ownership terminate consistently.

### P1 — Binding stop/kill can damage another Session or fail to force-cancel

For an active turn, both graceful stop and `killed=true` wait only for the
`turn/interrupt` RPC response and then immediately delete the SessionBinding.
Any later `turn/completed` or output notification for that thread is classified
as an unknown thread, causing `eventPump` to fail the entire shared
CredentialRuntime and every remaining binding. The deterministic fixture does
not emit this valid delayed ordering. In addition, an interrupt error restores
the binding to `running` and returns for both stop and kill, so kill does not
implement the frozen requirement to force-cancel only that binding.

Clearing evidence: define and implement terminal-turn drain/tombstone routing
that preserves other bindings, keep unknown cross-binding IDs fail-closed, and
add tests for delayed completion/output plus interrupt failure during both stop
and kill.

### P1 — Usage schema/error states are not represented truthfully

The domain supports `supported | unavailable | schema_changed | error`, but
the runtime persists only the success case. A changed Usage response returns a
Provider error that `usage.read` propagates instead of an explicit
`schema_changed`/unknown projection with reason; call failure is collapsed to
top-level `unavailable` without a persisted error-bearing snapshot. Moreover,
`dailyUsageBuckets` elements are retained as raw JSON and only counted, so a
changed bucket shape can still produce `official/high/supported`.

Clearing evidence: strictly validate the exact mapped Usage shape, persist or
return explicit unavailable/schema-changed/error evidence with source version,
freshness, confidence, and bounded reason, and add changed-bucket and timeout
tests.

### P1 — Exact Approval decision-table evidence is incomplete

The new runtime test proves command `approve -> accept -> approved/written` and
file-change cancel on an immediate write failure. It does not execute successful
command and file-change `approve`, `deny`, and `cancel` rows, nor prove that a
permissions request emits no response. Existing Store-only cancel tests and the
older ProviderSession approve test do not cover the daemon runtime transaction.

Clearing evidence: table-driven runtime/Application tests for both enabled
methods and all three local decisions, idempotent stored replay, different
digest conflict, permissions no-write, and disabled session-persistent/policy
variants.

## Passing boundaries retained

- one shared `CredentialRuntime` and separate per-Session thread bindings are
  present; the happy-path two-Session/one-child test passes repeatedly;
- daemon-owned `codex.v1` settings reject unknown fields and
  `danger-full-access`, canonicalize Workspace paths, and emit allowlisted
  `thread/start`/`turn/start` parameters;
- Provider thread IDs are persisted before the local Session becomes running;
- one reader correlates concurrent responses and queues inbound frames; unknown
  response IDs and queue overflow fail closed;
- active-turn input is rejected while steer is disabled, and conversation
  resize returns `provider_control_unsupported` without a Provider write;
- child crash fans out failure and finalization once in the tested ordering;
- Fake Provider tests and macOS/Linux/Windows compile baselines remain green;
- permissions Approval is rejected without a response in the implementation,
  although the required end-to-end negative test is missing;
- no P3B, live Linux, real Windows Codex, Security Review, Ship, merge, release,
  deployment, or support claim was upgraded.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P3A`
**Verdict**: `BLOCKED`
**Summary**: `P3A regression and platform checks pass, but Approval writes are unbounded after durable claim, binding termination can fail the shared runtime, and Usage degradation plus exact decision-table evidence are incomplete.`
**Evidence**: `fresh full Go/vet/race/repeated concurrency suites, macOS/Linux/Windows builds, formatting/diff/governance checks, and complete P3A diff inspection; exact commands are recorded above.`
**Findings**: `P0 unbounded Approval Provider write; P1 binding stop/kill ordering and force-cancel gap; P1 untruthful Usage schema/error degradation; P1 incomplete exact Approval decision-table tests.`
**Blockers**: `original feature-build writer must correct the four findings and provide deterministic blocked-write, delayed-event, Usage-degradation, and Approval-table regressions.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice` P3A remediation.
