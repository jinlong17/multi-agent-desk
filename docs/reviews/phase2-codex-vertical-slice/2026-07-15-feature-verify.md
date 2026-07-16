# Feature Verify: Codex Vertical Slice P0

## Verdict

`VERIFIED` for the approved P0 phase. The Device schema/Store foundation,
bounded Codex metadata records, authenticated local IPC/CLI surfaces, Approval
lease/idempotency behavior, restart expiration, and Fake compatibility meet the
P0 acceptance boundary. This verdict does not authorize Ship and does not claim
that a real Codex Provider is runnable.

## Scope and repository state

- Target: `phase2-codex-vertical-slice`
- Phase: `P0`
- Branch: `codex/provider/phase2-codex-vertical-slice`
- Base HEAD: `cb93c02` (implementation changes remain uncommitted)
- Verification worktree: `/Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/phase2-codex-vertical-slice`

## Acceptance verification

| Area | Evidence | Result |
|---|---|---|
| Migration and Fake preservation | `go test ./...`; storage migration upgrade/future-schema tests; schema version 4 and four migration ledger entries | PASS |
| Codex Store records | `internal/storage/codex_foundation_test.go` round-trips Account, Profile, CredentialInstance, Session, UsageSnapshot, and Approval records; invalid Account linkage leaves no partial row | PASS |
| Authenticated IPC/CLI | Temporary Device daemon plus CLI created/listed a Codex Account, created/listed a Codex Profile, and returned `usage.read` as unavailable without fabricating quota data | PASS |
| Approval control | `go test -race ./internal/domain ./internal/storage ./internal/app ./internal/runtime`; lease-bound Approval response and idempotent replay test passed | PASS |
| Security boundary | Static inspection confirms bounded/redacted Approval and Usage persistence, Vault references/digests only, no raw Provider payloads, and typed `provider_unsupported`/`provider_resume_unsupported` fallbacks | PASS |
| Platform baseline | Native macOS build plus `GOOS=linux GOARCH=amd64` and `GOOS=windows GOARCH=amd64` CLI builds passed | PASS |
| Workflow and CI contracts | `npm run project:verify`, `npm run ci:verify`, and `git diff --check` passed | PASS |

## Commands and results

```text
/opt/homebrew/bin/go test ./...                                      PASS
/opt/homebrew/bin/go vet ./...                                      PASS
/opt/homebrew/bin/go test -race ./internal/domain ./internal/storage \
  ./internal/app ./internal/runtime                                    PASS
/opt/homebrew/bin/go build -o /tmp/mad-multidesk-macos ./cmd/multidesk   PASS
GOOS=linux GOARCH=amd64 go build -o /tmp/mad-multidesk-linux ./cmd/multidesk PASS
GOOS=windows GOARCH=amd64 go build -o /tmp/mad-multidesk-windows.exe \
  ./cmd/multidesk                                                  PASS
npm run project:verify                                                PASS
npm run ci:verify                                                     PASS
git diff --check                                                      PASS
```

The temporary daemon/CLI run used a disposable `/tmp` Device root and passed
`accounts create`, `accounts list`, `profiles create`, `profiles list`, and
`usage --provider codex --json`. The Usage result was explicitly
`capability_status: unavailable` with no fabricated values.

## Findings and carried gates

No P0 implementation failure was found. The following remain intentionally open
and prevent a release verdict:

1. P1 exact Linux app-server schema/version probe, framing, handshake, and
   sanitized fixtures are not implemented.
2. P2 credential materialization, isolated `CODEX_HOME`, single-writer/CAS,
   crash recovery, and the implementation Security Gate remain open.
3. P3 real Linux Codex Session, Provider event mapping, live Usage/Approval
   dispatch, second-client live exit, and verified Provider continuation remain
   unstarted.
4. P4 macOS compatibility smoke and Windows real Codex compatibility remain
   unverified. Windows is covered only by the existing Phase 1/CI baseline and
   cross-compilation; no real Windows Codex support claim is made.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P0`
**Verdict**: `VERIFIED`
**Summary**: `P0 acceptance is verified: migration and Fake preservation, Codex metadata Store records, authenticated Account/Profile/Usage IPC/CLI behavior, Approval lease/idempotency handling, security boundaries, and macOS/Linux/Windows build baselines all pass.`
**Evidence**: `go test ./...`; `go vet ./...`; core `go test -race`; native and GOOS linux/windows builds; temporary daemon/CLI Account/Profile/Usage run; `npm run project:verify`; `npm run ci:verify`; `git diff --check` — all passed.
**Findings**: `No P0 failures. P1 exact schema/adapter, P2 materialization/security, P3 Linux live exit, and P4 macOS/Windows compatibility gates remain open.`
**Blockers**: `None for P0 verification; later-phase Provider and Security gates remain required before Ship.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice` P1.
