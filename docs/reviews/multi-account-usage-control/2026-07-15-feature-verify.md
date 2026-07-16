# Feature verification: multi-account-usage-control P1

## Verdict

`VERIFIED`

The second independent verification clears both reproducible blockers from the
first `BLOCKED` verdict. P1 now satisfies the approved metadata-only Account,
Profile, alias, stored Usage, forward migration, authenticated IPC/CLI, Session
binding, and fail-closed Provider boundaries.

P2-P5, real Codex/Claude login and quota collection, and the product Web
dashboard remain gated and were not judged implemented. Because later phases
remain, this verdict is `VERIFIED`, not `READY_TO_SHIP`.

## Prior findings and closure

### [CLOSED P0] Stored Usage replay deduplication

The first verification proved that two different Usage IDs with the same
frozen `(accountId, deviceId, providerVersion, observedAt, rawReferenceHash)`
tuple produced two rows. The clearing writer added both a migration-level
unique index and a transactional Store replay lookup.

The exact Store repro was rerun with two different snapshot IDs and the same
tuple. Both calls returned safely, `ListUsageSnapshots` returned exactly one
observation, and its two original Usage windows remained intact. Independent
inspection of a newly migrated database returned
`usage_snapshots_replay_tuple` as a unique index, and the populated v3-to-v4
migration/restart tests passed.

### [CLOSED P1] Public Codex Profile on the legacy Fake start surface

The first verification proved that an authenticated `run fake` could combine a
valid internal Fake Credential with a public Codex Profile and create a running
mixed Session. The clearing writer added transactional checks for Account,
Profile, Credential, Device, Provider, Profile credential binding, and the
internal/public boundary before Session persistence and process creation.

The exact daemon/CLI repro was rerun on a clean temporary Device. The mixed
Fake-Credential/public-Codex-Profile request returned `conflict`, and
`sessions list` remained empty. The same Fake Credential with the valid
internal Fake Profile then returned `ok:true` and reached `running`, proving
that compatibility was preserved without weakening the boundary. The new
authenticated native IPC regression independently covers the same negative
case before its valid Fake lifecycle.

## Independent evidence

- Targeted blocker reruns:
  - `go test -count=1 -run
    '^TestGenericUsageWindowsRoundTripUnknownAndMissingValues$' -v
    ./internal/storage` — pass; two IDs, one stored observation.
  - `go test -count=1 -run '^TestNativeTwoClientFakeSessionControl$' -v
    ./internal/device` — pass; mixed tuple rejected with no Session, valid
    internal Fake path runs.
  - `go test -count=1 -run
    'TestMigrationV4PreservesPopulatedFakeTuplesAndIsIdempotent|TestOpenConfiguresAndRestartsDeviceDatabase'
    -v ./internal/storage` — pass.
  - Fresh daemon/CLI replay plus `PRAGMA index_list('usage_snapshots')` — mixed
    tuple rejected, Session count zero before the valid Fake start, unique
    replay index present.
- `go test -count=1 ./...` and `go vet ./...` — pass.
- `go test -race -count=1 ./internal/domain ./internal/storage ./internal/app
  ./internal/device ./cmd/multidesk` — pass.
- Darwin arm64, Linux amd64, and Windows amd64 storage test cross-compiles with
  Go 1.26.5 — pass.
- Workflow, dashboard static, Actions, CODEOWNERS, CI fixture, local-link,
  layout, Go-format, and license verifiers — pass before the atomic verdict
  transition; the link scan covered 209 Markdown files.
- Exact pnpm 10.23 Web TypeScript check, Rust format check, and locked Rust
  check with an isolated target directory — pass.
- `git diff --check origin/main` — pass.
- Source inspection still confirms public metadata creation allocates no
  CredentialInstance, Vault item, Provider Home, Keychain item, browser state,
  directory, or subprocess. The `@A` real-Provider path remains explicitly
  unavailable and cannot fall through to Fake.

## Findings

No remaining P1 finding. The two earlier failures remain recorded above and in
the append-only feature Work Log; this verdict records their independent
closure rather than erasing them.

## Remaining gates

Both distinct-account Provider Spikes and the feature Security Gate remain
open. P2 Codex, P3 Claude, P4 dashboard, and P5 remote authorization require
their named evidence, review, and verification transitions. This verdict does
not authorize real Provider claims, credential transfer, push, merge, ship, or
release.
