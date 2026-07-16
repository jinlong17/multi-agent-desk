# P1 as-built: manual multi-account registry and usage contracts

## Verdict

P1 implements the approved metadata-only foundation. An authenticated local
client can manually create, inspect, update, disable, enable, and delete any
number of Codex or Claude Accounts and RuntimeProfiles, select a Profile with
an explicit alias such as `@A`, and read normalized stored Usage snapshots.

P1 does **not** log in to either Provider, allocate a Provider Home, launch a
Provider process, refresh quota, render the product Web dashboard, or transfer
credentials to another Device. Those capabilities remain gated in P2-P5.

## Stored model

- `Account` is public metadata for one Codex or Claude identity. It has no
  secret fields and carries provider, display/subscription hints, enabled state,
  timestamps, and an optimistic revision.
- `RuntimeProfile` belongs to exactly one Account and Device. Public Profiles
  use a globally case-insensitive alias with a 1-32 character safe ASCII
  grammar. One optional leading `@` is accepted only at selector boundaries.
- `CredentialInstance` remains absent after P1 Account/Profile creation.
  Therefore auth is reported as `login_required`, availability as `unknown`,
  and last validation as null.
- `UsageSnapshot` stores provider/version/source/confidence/availability,
  observed and stale times, and an ordered, arbitrary `UsageWindow[]`. Missing
  values stay null and unknown Provider windows stay unknown. Replays are
  deduplicated transactionally and by a unique
  `(account,device,providerVersion,observedAt,rawReferenceHash)` index.
- `Session` now persists `account_id`. Every Session write requires its
  Account, Profile, Credential, Device, Provider and internal/public boundary
  to agree. A real-provider confirmation must also match the previewed tuple;
  otherwise it fails before a row or process can start.

## Creation, update, pagination, and deletion

Manual Account creation is one SQLite transaction containing exactly one
Account and one default RuntimeProfile. It creates no credential, Vault item,
directory, Keychain item, cookie, auth file, browser session, or subprocess.

Account and Profile mutations require the expected revision. Lists use stable
ascending `(created_at,id)` keyset pagination with a default of 50 and a maximum
of 200; cursors are versioned and bound to their original filter. No fixed
account-count limit exists.

Deletion is local metadata deletion, not Provider-wide revocation. An entity
must first be disabled. Active Session or unexpected credential references fail
closed, and Account deletion writes minimal Account/Profile tombstones in the
same transaction. Internal Fake rows cannot be reached through public Account
or alias interfaces.

## Migration and Fake compatibility

Device migration `0006_accounts_usage.sql` reconciles the P1 metadata model on
top of the shipped Phase 2 schema version 5. It extends Accounts and public
Profile metadata, adds structured Usage windows and tombstones, and preserves
the Phase 2 Codex Account, RuntimeProfile, CredentialInstance, Vault linkage,
Usage rows, and Session account binding. Existing Fake data migrates to one
deterministic internal Fake Account per Device. Existing Profile,
CredentialInstance, and Session IDs and tuples are preserved; internal Fake
Profiles have null aliases and the shipped explicit-ID `run fake` path
continues to work.

The migration runner performs the connection-scoped foreign-key suspension
needed by the version-6 table rebuild, runs preflight and post-copy checks,
verifies `foreign_key_check`, restores foreign-key enforcement, and remains
restart idempotent. The migration creates no Provider credential, Provider
Home, Vault item, or real Provider process.

## Local IPC and CLI surface

Authenticated IPC now exposes:

- `accounts.create/list/show/update/disable/enable/delete`
- `profiles.create/list/show/resolveAlias/update/disable/enable/delete`
- `usage.list`
- reserved fail-closed methods for validate/login/logout/shell/refresh

Metadata reads reuse the shipped `metadata.read` capability; mutations require
`client.admin`. The CLI exposes matching `accounts` and `profiles` commands,
plus `login`, `logout`, `shell`, `usage`, and `run --profile @A` entry points.
Until P2/P3, real Provider operations deliberately return
`provider_capability_unavailable`; accepting a selector never falls through to
Fake or another account.

## Verification evidence before independent verdict

- `go test -count=1 ./...` and `go vet ./...` pass.
- Race tests pass for domain, storage, app, device, and CLI packages.
- Storage tests compile for Darwin arm64, Linux amd64, and Windows amd64 with
  Go 1.26.5.
- Tests cover eight Accounts, twelve Profiles, keyset pagination, alias
  conflicts, revision conflicts, atomic deletion/tombstones, unknown Usage,
  authenticated IPC, idempotency, migration/restart/foreign keys, unchanged
  filesystem state, Usage replay deduplication, mixed Fake/Codex tuple
  rejection, and fail-closed real-provider launch.
- A temporary real daemon accepted six mixed Codex/Claude Accounts (`@A`-`@F`),
  paginated at three, rejected login/run as unavailable, and created no Session.
- Go, Web TypeScript/build, Rust format/check, workflow, dashboard, CI static,
  fixture, link, and license checks pass with the repository-pinned runtimes.

## Remaining gates

1. P0 Codex and Claude Spikes need evidence from two distinct real identities.
2. P2 must allocate one managed `CODEX_HOME` per credential, preserve the
   single refresh-writer/CAS contract, implement official login/status/logout,
   collect official app-server Usage, and launch explicit Sessions.
3. P3 must allocate one `CLAUDE_CONFIG_DIR` per Profile, prove any Keychain
   scope, implement login/status/logout and PTY launch, collect supported 5h/7d
   status-line Usage, and keep monthly Usage explicitly unavailable unless a
   supported contract is proven. The Claude policy gate remains open.
4. P4 must add metadata sync and the responsive Accounts/Usage Web/Desktop
   product surface. Provider cookies and auth Homes must never enter browser
   storage.
5. P5 must add target-local login guidance and, only after Security approval,
   scoped E2EE CredentialGrant. Copying browser cookies or entire auth Homes is
   not a supported transfer design.
