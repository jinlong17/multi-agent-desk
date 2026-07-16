# Feature verification: multi-account P1 Phase 2 reconciliation

## Verdict

`VERIFIED`

The reconciliation preserves both the previously verified P1 metadata-only
foundation and the shipped Phase 2 Codex/Vault/runtime contracts. No remaining
finding blocks this P1 phase. Later product phases still exist, so the correct
verdict is `VERIFIED`, not `READY_TO_SHIP`.

## Scope inspected

- final Phase 2 base `96ba36d`
- preserved/rebased P1 commit `eeeb558`
- reconciliation commit `a7ca66f`
- build receipt `2026-07-16-phase2-reconciliation-build.md`
- shared domain, Store, migration, authenticated IPC, CLI, Session binding,
  Codex Credential/Vault, structured Usage, and internal Fake boundaries

## Independent evidence

- `go test -count=10 ./internal/storage -run
  'TestManualRegistryPaginationRevisionAndAtomicDeletion|TestGenericUsageWindowsRoundTripUnknownAndMissingValues|TestAccountCreateDoesNotAllocateProviderHome|TestMigrationV6PreservesPopulatedFakeTuplesAndIsIdempotent|TestAccountsUsageMigrationPreservesPhase2CodexRowsAndVaultLinks|TestMigrationFailureRollsBackDDLAndLedger'`
  — pass. All six migration, registry, replay, no-Home, rollback, and exact
  Phase 2 data-preservation tests passed ten consecutive runs.
- `go test -count=10 ./internal/device -run
  'TestAuthenticatedAccountRegistryAndRealProviderFailClosed|TestNativeTwoClientFakeSessionControl'`
  — pass. Authenticated public metadata and mixed Fake/Codex tuple boundaries
  passed ten consecutive runs.
- `go test -count=1 ./...` and `go vet ./...` — pass.
- `go test -count=1 -race ./...` — pass.
- Darwin arm64, Linux amd64, and Windows amd64
  `CGO_ENABLED=0 go build ./cmd/...` — pass.
- pnpm Web TypeScript check, Rust format, and locked Cargo check — pass.
- workflow, generated dashboard, Actions, CODEOWNERS, CI fixtures, local links,
  licenses, Go formatting, and scaffold layout verifiers — pass. The first
  dashboard attempt correctly reported the pre-commit machine snapshot as
  stale; the already-authorized dashboard writer regenerated it for
  `a7ca66f`, after which verification passed with zero unrelated dirty files.
- `git diff --check 96ba36d..HEAD` — pass before the atomic verdict write.

## Acceptance and compatibility conclusions

- Migration 6 upgrades the exact Phase 2 schema version 5 and retains seeded
  Codex Account, RuntimeProfile, CredentialInstance, Vault item, and Usage data
  with zero foreign-key violations.
- Existing Fake Profile, Credential, and Session IDs remain intact. The
  deterministic internal Fake Account cannot be addressed through public
  Account or alias APIs.
- Manual Account/Profile creation remains metadata-only and creates no
  CredentialInstance, Vault item, Provider Home, browser state, directory, or
  subprocess.
- Usage replay deduplication and arbitrary structured windows remain intact.
- Explicit selector preview checks the frozen Account/Profile/Credential/Usage
  tuple. Real multi-account launch still fails closed and cannot rotate to a
  different Account or fall through to Fake.
- The shipped Phase 2 explicit-ID Codex enrollment, Vault, usage, approval, and
  runtime tests continue to pass under the unified schema and domain model.

## Findings

None for P1 reconciliation.

## Remaining gates

Codex distinct-account and Claude distinct-account/usage Spike decisions,
Claude policy evidence, and the feature Security Gate remain open. P2-P5 must
pass their own plan, review, build, verify, and security transitions. This
verdict does not authorize real multi-account Provider claims, push, merge,
ship, release, or deployment.

## Handoff

**Target**: `multi-account-usage-control`
**Completed**: `feature-verify / P1 Phase 2 reconciliation`
**Verdict**: `VERIFIED`
**Summary**: `P1 metadata contracts and shipped Phase 2 Codex/Vault/runtime contracts remain compatible after migration 6 reconciliation.`
**Evidence**: `Repeated targeted migration/IPC probes, full Go/vet/race, three-OS builds, Web/Rust, and all project governance checks passed.`
**Findings**: `none`
**Blockers**: `none for P1; P2-P5 Provider and Security gates remain open`

### Next Step

Run `feature-plan` for `multi-account-usage-control` child Spike assumption updates before any later Provider build.
