# Multi-account P1 Phase 2 reconciliation build receipt

## Scope

This writer run reconciles the previously verified metadata-only P1 foundation
onto final remote `main` at `96ba36d`. It does not start Codex multi-account P2,
Claude P3, product dashboard P4, remote grant P5, release, or deployment work.

## History and conflict policy

- The exact original P1 worktree was verified and preserved as signed commit
  `1ac1f41`; after rebase its equivalent commit is `eeeb558`.
- Final Phase 2 `main` remains authoritative for the shipped Codex Vault,
  enrollment, runtime, approval, usage, and session contracts.
- P1 remains authoritative only for manual public Account/Profile metadata,
  aliases, optimistic revisions, structured stored Usage windows, replay
  deduplication, tombstones, and fail-closed explicit-selector preview.
- Shared Go types are one union rather than duplicate declarations. Public
  APIs hide deterministic internal Fake Accounts.

## Migration resolution

The original P1 migration number collided with the shipped Phase 2 migrations.
The reconciled migration is `migrations/device/0006_accounts_usage.sql`, applied
only after Phase 2 schema version 5. It rebuilds the overlapping Account,
RuntimeProfile, and Usage tables while preserving existing Codex rows and all
dependent CredentialInstance, Vault, Session, and authorization data.

The Store performs a preflight, temporarily suspends connection-scoped foreign
keys for the rebuild, runs `PRAGMA foreign_key_check` before commit, restores
foreign keys, and verifies the restored setting. A dedicated test builds the
exact version-5 schema, seeds a Codex Account/Profile/Credential/Vault/Usage
tuple, applies version 6, and proves the tuple and Vault linkage survive with
zero foreign-key violations.

## Verification performed by the writer

- `go test -count=1 ./...`
- `go vet ./...`
- `go test -count=1 -race ./...`
- Darwin arm64, Linux amd64, and Windows amd64 `go build ./cmd/...`
- repeated migration preservation, Account visibility, alias, replay, and
  tuple-binding tests
- `git diff --check`

All commands passed. This is build evidence, not an independent verdict.

## Non-claims and remaining gates

- No public metadata action allocates a credential, Vault item, Provider Home,
  browser state, or subprocess.
- An explicit `@alias` can be resolved and its preview tuple checked, but real
  multi-account launch still returns a Provider-unavailable error.
- Codex/Claude account-isolation and Claude usage/policy child decisions remain
  open. P2-P5 require their own plan/review/build/verify/security lifecycle.

## Handoff

- Workflow: `FEATURE_DEV`
- Target: `multi-account-usage-control`
- Current Phase: `BUILD P1 RECONCILIATION`
- Status: `READY_FOR_VERIFY`
- Next Role: independent `feature-verify`
- Next Action: verify the reconciliation against original P1 acceptance and shipped Phase 2 contracts; do not start P2 or ship from this receipt
