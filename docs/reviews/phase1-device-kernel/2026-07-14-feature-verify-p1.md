# Feature verification: Phase 1 Device Kernel P1

- Date: 2026-07-14
- Role: `feature-verify`
- Phase: `P1 domain and Device store`
- Verified commit: `de11a4973b9b30bf17d9ae17382500e5b98b159a`
- Verdict: `BLOCKED`

## Conclusion

The migration/store implementation, dependency governance, local race tests,
three-target compilation, project checks, and documented state are healthy.
Verification nevertheless found three domain counterexamples that existing
tests do not cover. Two violate explicit P1 Session/ControllerLease invariants;
the third leaves an unbounded identifier input despite the contract's bounded
resource requirement.

P2 must not begin. The P1 writer can clear all findings locally without a plan
revision or external dependency.

## Findings

### P1 — persisted Resume can manufacture a capability snapshot

Files: `internal/storage/repository.go:548-584`,
`internal/domain/session.go`.

`validateResumeSource` reads and compares the source's Device, Provider,
CredentialInstance, RuntimeProfile, Workspace, Provider Session, terminal
status, and end time, but it does not read the source
`capability_snapshot_json`. `NewSession` validates only the proposed new
snapshot. A caller can therefore take a terminal source that never had
`session.resume`, construct a new record with the same other frozen fields and
an invented snapshot containing `session.resume`, and persist it successfully.
The same path can add or remove any other frozen Capability.

Required correction: load and validate the source snapshot in the same
transaction, require `session.resume` on the source, require canonical exact
snapshot equality, and add repository tests for missing source capability and
snapshot expansion/removal. The rejected transaction must leave no new row.

### P1 — released ControllerLease accepts a time before release

File: `internal/domain/lease.go:26-53`.

`AcquireControllerLease` checks `current.Active(at)`. A released lease is never
active because `ReleasedAt != nil`, even when `at` is earlier than
`LastHeartbeat`/release time. The function then creates a higher revision whose
timestamps move backward. Heartbeat/release correctly reject non-monotonic
time, but acquisition does not.

Required correction: when a current lease exists, reject an acquisition time
before its last heartbeat/release boundary before testing expiry, and add a
counterexample test. The existing active-holder and revision rules must remain.

### P2 — ValidateID accepts arbitrary and unbounded prefixes

File: `internal/domain/types.go:15-45`.

`NewID` limits the prefix to 24 lowercase/digit/underscore characters, but
`ValidateID` splits on `_` and validates only the final 16-byte hex suffix.
Values such as an arbitrarily long or invalid-character prefix followed by a
valid suffix pass and can be sent to repositories. Parameterized SQL prevents
injection, but the behavior violates the intended stable ID grammar and leaves
an unnecessary resource-amplification input before P2 framing.

Required correction: validate the complete prefix with the same grammar and
bound as `NewID`, retain underscore support inside the prefix, and add boundary
tests for empty, overlong, uppercase/punctuation, malformed suffix, and valid
multi-part prefixes.

## Passing evidence

- `go test ./...` — pass.
- `go vet ./...` — pass.
- `go test -race ./internal/domain/... ./internal/storage/... ./migrations/device/...`
  — pass on macOS arm64.
- `go test -c ./internal/storage` — production/test compilation passed for
  darwin/arm64, linux/amd64, and windows/amd64.
- `go-licenses v2.0.1 check ./... --include_tests` with the repository allowlist
  — pass; only the known x/sys assembly inspection warning.
- `npm run project:verify`, `npm run ci:verify`, `npm run scaffold:verify`,
  `npm run ci:links`, and `git diff --check` — pass after frozen workspace
  dependency installation.
- Migration review — ordered contiguous embedded versions, checksum ledger,
  transactional DDL rollback, future/checksum refusal, WAL/FK/busy readback,
  restart persistence, and no destructive fallback are covered and passing.
- Boundary review — no IPC, Daemon, runtime, Vault, CLI, real Provider, release,
  or deployment work entered P1.

## Gates and clearing condition

- Provider Gate: none.
- Security Gate: remains open for final Phase 1.
- External blocker: none.
- Clearing role: `feature-build` P1 adds the three minimal corrections and
  regression tests, reruns the complete P1 evidence set, restores
  `READY_FOR_VERIFY`, and returns to independent `feature-verify`.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-verify / P1 domain and Device store`
**Verdict**: `BLOCKED`
**Summary**: Storage and governance checks pass, but persisted Resume can invent capabilities, released Lease acquisition can move time backward, and ValidateID accepts unbounded invalid prefixes.
**Evidence**: Exact commit audit, full Go/vet/race/cross-build suite, exact Go license gate, project/CI/scaffold/link checks, migration review, and code counterexamples.
**Findings**: P1 compare the source Resume capability snapshot exactly; P1 reject non-monotonic acquisition after release; P2 validate the complete bounded ID grammar.
**Blockers**: The original P1 feature-build writer must correct all three findings and add regression tests before reverification.

### Next Step

Run `feature-build` P1 correction for `phase1-device-kernel`.
