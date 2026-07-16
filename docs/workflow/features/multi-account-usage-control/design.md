# Design: 多账号用量看板与显式调用

## Decision snapshot

- Selected option: Daemon-owned isolated Provider Homes + manually managed
  Account/Profile registry + generic `UsageWindow[]` + explicit `@alias`
  selection pinned at Session creation.
- Review evidence: Feature Brief; Phase 1 shipped Device Kernel; ADR 0014 and
  ADR 0016; current Provider compatibility matrix; fresh official Codex and
  Claude documentation review on 2026-07-15.
- Frozen assumptions: no browser Cookie import; no automatic account discovery
  or rotation; Provider values are version-gated; unknown usage remains
  unknown; one refresh writer per CredentialInstance; CLI and UI use the same
  daemon application services.
- Rejected alternatives: one shared Provider Home with repeated logout/login;
  embedded Provider login WebViews; copied browser profiles/Cookie jars;
  OS-user-per-account as the product model; shell-evaluated aliases; fixed
  5h/week/month columns in storage; automatic fallback to another account.

## Context and boundaries

Remote `main` has shipped the Phase 1 Device Kernel. It contains Device
identity, authenticated local IPC, SQLite, Vault/materialization primitives,
Fake Provider Sessions, CLI control, attachments and controller leases. The
Codex and Claude packages are placeholders; the schema only accepts the Fake
Provider, and no Account CRUD, Profile CRUD, Usage collector, real Provider
process or product Web dashboard exists.

This feature owns the Provider-facing account, authentication, usage and
invocation contract. It intentionally spans physical paths owned by impacted
modules, but it does not create a second owner:

- `provider`: login lifecycle, Provider Home construction, auth/usage probes,
  version fixtures, capability downgrade, process launch.
- `core`: Account/Profile/Usage domain records, SQLite, Vault references,
  authenticated local IPC and Session pinning.
- `web`: metadata-only Accounts/Usage UI and explicit Profile selection.
- `desktop`: OS Keychain bridge and user-selected external browser launch.
- `control-plane`: non-secret Account/Profile/Usage metadata sync in a later
  phase.
- `security`: credential boundaries, login state binding, deletion/revocation,
  redaction, Provider policy and CredentialGrant review.
- `project-system`: Feature/Spike state, user guide, compatibility matrix and
  development dashboard truth.

The MultiAgentDesk Web session is unrelated to Provider browser sessions. The
Web dashboard authenticates to the Control Plane with its own Passkey/session
Cookie. Codex/Claude login happens through the official local CLI in a
Profile-specific environment. Provider browser Cookies remain inside the
browser Profile selected by the user and are neither required nor readable by
the dashboard.

## Components and ownership

### Account registry

```text
Account
  id
  provider: codex | claude | fake  # fake is internal migration state only
  displayName
  providerSubjectDigest?  # keyed digest, never raw email/org in normal state
  subscriptionHint?       # allowlisted enum/string, no billing authority
  internal                # true only for migrated Fake compatibility rows
  enabled
  revision
  createdAt / updatedAt

CredentialInstance
  id / accountId / deviceId
  authMethod
  secretRef
  authStatus: login_required | authenticating | healthy | expired | revoked | unknown
  availability: available | limited | unavailable | unknown
  lastValidatedAt / expiresAt?
  credentialRevision / secretDigest

RuntimeProfile
  id / providerId / accountId / credentialInstanceId? / deviceId
  name
  selectorAlias?         # null only for internal Fake compatibility rows
  model/config non-secret settings
  internal
  enabled
  revision
  createdAt / updatedAt
```

The selector belongs to a RuntimeProfile, not directly to Account, because one
logical account may intentionally have multiple model/tool configurations. A
manual “add account” transaction creates the Account plus one default Profile;
advanced users may create additional aliases for the same account.

P1 public creation accepts only `codex` or `claude`. `fake` is a reserved,
internal-only provider value used to preserve the shipped Device Kernel. It is
never accepted from CLI/IPC creation, never returned by default public list
calls, and never resolved through an `@alias`. Existing Fake Profiles retain
their IDs, have `internal=true` and `selectorAlias=null`, and continue to be
selected only by the legacy explicit-ID `run fake` command.

Alias input is bounded to 1–32 ASCII characters matching
`[A-Za-z][A-Za-z0-9_-]*`. Storage uses a lowercase canonical key; UI and CLI
render `@` only as a selector prefix. Provider Home paths use opaque IDs, never
aliases or display names.

### P1 creation and revision model

`accounts.create`/`accounts add` is one SQLite transaction. It creates exactly:

1. one public Account with `revision=1` and `enabled=true`; and
2. one public default RuntimeProfile on the local Device with `revision=1`,
   the requested alias, `enabled=true`, and `credentialInstanceId=null`.

P1 does **not** create a placeholder CredentialInstance, Vault item, Provider
Home, Keychain item, directory or Provider process. For a Profile with no
CredentialInstance, the response derives `authStatus=login_required`,
`availability=unknown` and `lastValidatedAt=null`; these are view values, not
duplicated Profile columns. A transaction failure therefore has no filesystem
cleanup phase.

Account and Profile have independent monotonic revisions. Every update,
enable/disable and delete supplies the expected revision and increments it once
on success. Provider, Account ownership and Device ownership are immutable.
P1 Profile update may change only display name and selector alias; it does not
accept arbitrary Provider settings or secret-bearing JSON.

### Usage model

```text
UsageSnapshot
  id / accountId / credentialInstanceId? / deviceId
  provider / providerVersion
  source: official | cli_derived | local_estimate | unavailable
  confidence: high | medium | low | none
  observedAt / staleAt
  availability
  rawReferenceHash?
  windows[]

UsageWindow
  providerLimitId?
  kind: rolling | calendar | spend_control | sdk_credit | unknown
  label
  durationSeconds?
  usedValue? / limitValue?
  usedPercent? / remainingPercent?
  resetsAt?
```

Storage and API expose an array rather than fixed columns. UI may recognize
common durations (300 minutes → “5 小时”, 10080 minutes → “7 天”), but retains
the Provider label and duration. A Provider may return no monthly window, more
than two windows, a currency/spend window or only a reset time.

`availability` is separate from authentication. A valid login can be limited,
and a missing usage snapshot does not imply an invalid login. Snapshots become
stale by adapter policy; stale values remain visible with their timestamp and
must not drive automatic selection.

### Provider Home manager

Provider Home allocation begins in P2/P3, not P1. The existing
`CredentialMaterializationManager` remains the only authority that creates
writable credential materializations; a later reviewed `provider_homes` record
must name both its owner kind and opaque owner ID so ownership is not inferred
from one ambiguous Profile field.

- Codex: one canonical managed `CODEX_HOME` and app-server owner per
  CredentialInstance (`owner_kind=credential_instance`), file credential
  storage, versioned schema/fixture gate, revisioned CAS after refresh. Multiple
  Sessions multiplex through that owner.
- Claude: one managed `CLAUDE_CONFIG_DIR` per RuntimeProfile. On macOS the
  Home uses `owner_kind=runtime_profile`; the official CLI writes to its scoped
  Keychain slot and MultiAgentDesk never reads Keychain secrets. Linux/Windows
  credentials remain in the target config dir with platform-private permissions.
- A Profile lock prevents concurrent login/logout/materialization transitions.
- Provider executable and version are pinned for the operation and included in
  sanitized evidence.

### Login coordinator

`LoginAttempt` is ephemeral, non-secret state:

```text
id / profileId / provider / processId
stateDigest / startedAt / expiresAt
status: pending | browser_required | verifying | succeeded | failed | cancelled
```

The official Provider CLI owns OAuth PKCE, browser callback and token handling.
MultiAgentDesk owns only process isolation, target Home selection, lifecycle,
timeout and post-login identity verification. It may show a sanitized URL host
or device-code instruction, but never persist authorization URLs/codes.

The user may choose an already-isolated browser Profile. If the wrong browser
identity completes the flow, post-login verification must show the mismatch
and require an explicit rebind or logout; a window title is not identity proof.

### CLI selector and Session binding

The canonical invocation is:

```text
multidesk run --profile @A --workspace <path>
multidesk run --profile @A --non-interactive -- <provider-specific args>
multidesk shell @A
```

P1 implements selector resolution and manual metadata commands but does not
launch a real Provider. P2/P3 bind selectors to real Codex/Claude adapters.
Non-interactive commands require `--profile`; interactive mode may show a list
but still requires confirmation. The daemon resolves the alias once and writes
the immutable Account/Credential/Profile tuple into the Session.

Shell completion may offer aliases. Generated shims, if later added, must exec
`multidesk` with a literal profile ID/alias array; they must never evaluate a
stored display name or arbitrary shell fragment.

### Accounts and Usage dashboard

The first product dashboard is a metadata client. It consumes paginated account
summaries and UsageSnapshots; it does not access Provider endpoints directly.
Each card contains:

- display name, Provider, Profile aliases and device;
- `authStatus`, `availability`, last validation and Provider version;
- zero or more window rows with source/confidence/observedAt/stale state;
- explicit actions: login/relogin, refresh usage, disable, logout, delete;
- disabled actions with stable explanations when a Provider or policy gate is
  not supported.

The UI must distinguish `0% used`, `100% used`, `unknown`, `unavailable` and
`stale`. It must not compute “currently usable” from usage alone.

## Data flow and state transitions

### Manual add and login

```text
manual add request
  -> validate provider + alias
  -> transaction creates Account + default RuntimeProfile
  -> no CredentialInstance, Home, Vault item or Provider process in P1
  -> derive authStatus=login_required and availability=unknown
  -> user invokes login for the exact @alias
  -> P2/P3 create the correctly owned Home and CredentialInstance
  -> profile lock + official CLI in target environment
  -> post-login auth status / actual identity verification
  -> explicit bind confirmation
  -> CredentialInstance healthy + audit event
  -> usage collection (when capability exists)
```

Metadata creation is idempotent. A failed login leaves a valid disabled or
login-required Profile rather than deleting evidence or falling back to a
different account.

### Usage collection

```text
Provider event or explicit refresh
  -> capability/version check
  -> collect via canonical Provider owner
  -> allowlist + normalize fields in memory
  -> persist UsageSnapshot + windows
  -> publish metadata update
```

Codex uses `account/rateLimits/read`, related update notifications and optional
account usage summaries only for exact compatible schemas. Claude may ingest
`rate_limits.five_hour` and `rate_limits.seven_day` from the official
status-line JSON after a real request. P3 must not launch a hidden prompt merely
to refresh the dashboard unless a separately reviewed user setting permits the
cost and clearly labels it.

### Disable, logout and delete

```text
active -> disabled                 # blocks new Sessions, preserves evidence
healthy -> login_required          # target-profile official logout
disabled + no active references
  -> delete secrets/provider home
  -> tombstone metadata
  -> deleted
```

Hard delete is refused while an active Session, materialization lease or
CredentialGrant references the Profile. Local logout/removal is not Provider-
wide revocation. The UI always links to Provider-side revocation guidance and
states that remote copies cannot be erased by MultiAgentDesk.

The executable P1 deletion contract is narrower and wholly transactional:

- a public Profile must first be disabled and match `expectedRevision`; delete
  rejects any Session or active materialization that references it;
- a public Account must first be disabled and match `expectedRevision`; one
  transaction checks every child Profile/CredentialInstance, rejects any
  Session or active materialization, deletes Usage children and all public
  Profiles, then deletes the Account;
- P1 public records cannot own a CredentialInstance or Provider Home, so P1 has
  no external cleanup and cannot partially succeed;
- each deleted Account/Profile produces a minimal tombstone containing only
  opaque ID, provider, final revision and deletion time. Display names and
  aliases are removed and the alias becomes reusable after commit;
- internal Fake Accounts/Profiles are not deletable through public methods.

CredentialGrant storage does not exist in P1 and no Grant can be created by a
P1 binary. Its reference check is explicitly deferred to P5: the migration
that first enables Grants must add the delete-side foreign-key/checker before
any Grant creation API can be enabled. P1 must not claim that future Grant
cleanup is implemented.

## Failure and recovery

- Alias conflict or malformed selector: reject before any write.
- Login process exits/cancels: retain Profile as `login_required`, remove only
  attempt-scoped temporary state, preserve sanitized error code.
- Wrong identity observed: do not silently bind; require explicit user action
  or target-profile logout.
- Unknown Provider version/schema: auth/usage capability becomes unknown or
  unavailable; keep metadata, block live start if account binding cannot be
  verified.
- Usage request fails: keep last snapshot marked stale and store a safe error
  category, not raw response.
- Ambiguous Codex refresh residue: quarantine Home per ADR 0014, block new
  Sessions and require diagnosis/re-login.
- Claude rate limits absent before first response: show unavailable-yet, not
  zero. A later real Session event may populate the snapshot.
- Vault locked: metadata remains readable; login, logout, materialization and
  new Sessions return `vault_locked`.
- Daemon crash during metadata transaction: SQLite transaction rolls back.
  Crash during Provider login leaves an expired LoginAttempt and requires an
  auth-status recheck before retry.

## Security and privacy

- No Provider Token, auth file, browser Cookie, OAuth code, email, org name or
  raw status JSON enters logs, audit metadata, Control Plane or test fixtures.
- Normal Account identity uses display text supplied by the user plus a keyed
  subject digest. Any optional email hint is separately consented, redacted by
  default and never used as a secret or authorization decision.
- All secrets are supplied through official interactive flows, stdin, a
  restricted file descriptor or the Vault; never argv or shell interpolation.
- Provider Home permissions are verified before every login/materialization.
- Login/logout/usage refresh mutations require authenticated IPC capability,
  idempotency keys and Profile-level serialization.
- Alias lookups are authorized only after local client authentication and never
  reveal secrets.
- Web/Control Plane receives non-secret status only. It cannot start a login on
  an unapproved Device or become a credential refresh writer.
- The Claude Provider policy gate is fail-closed. Until applicability is
  accepted, subscription login/rate-limit surfacing stays developer-only and is
  not advertised as a stable product capability.
- There is no automatic rotation, transparent retry on another Profile or
  quota-bypass ranking.

## Compatibility and migration

P1 adds forward-only Device DB migrations for Account, Profile alias and Usage
tables/fields. Existing Fake Provider fixtures are backfilled into a synthetic
local Account without changing their Session IDs. The migration rebuilds
provider-constrained tables only after copying and validating all rows.

The Fake backfill contract is exact:

1. Preflight verifies all existing Profile, CredentialInstance and Session
   providers are `fake`, every reference resolves, entity IDs have the shipped
   grammar, and no reserved P1 table exists outside the migration ledger.
2. For each Device, create exactly one internal Fake Account whose ID is
   `account_` plus the unchanged 32-hex suffix of that Device ID. A conflicting
   pre-existing ID with different data is `schema_incompatible`; the migration
   does not choose another ID.
3. Preserve every existing RuntimeProfile and CredentialInstance ID. Attach all
   of them on that Device to the synthetic Account. Profiles receive
   `internal=true`, `selector_alias=NULL`, `revision=1`, and no default
   CredentialInstance; multiple Profiles/credentials therefore remain
   unambiguous without inventing a binding.
4. Rebuild `runtime_profiles`, `credential_instances`, and `sessions` in one
   exclusive Store migration. Create/copy in parent order: `accounts`,
   `credential_instances_v4`, `runtime_profiles_v4`, then `sessions_v4`. Add
   `sessions.account_id NOT NULL` and backfill it from the referenced Profile
   while preserving the Session ID and its exact Profile/Credential tuple.
   After count and relationship validation, drop in child order (`sessions`,
   `runtime_profiles`, `credential_instances`), rename replacements in parent
   order (`credential_instances`, `runtime_profiles`, `sessions`), and recreate
   the shipped indexes. Existing attachment/lease/event/materialization tables
   retain their IDs and resolve the same final table names.
5. Because those tables are referenced, the migration runner disables SQLite
   foreign-key enforcement on its single connection **before** the transaction,
   copies into new tables, validates counts/provider/account equality, swaps
   tables in the preceding fixed order, runs
   `PRAGMA foreign_key_check` before commit, then re-enables and re-verifies
   `foreign_keys=ON`. Any preflight/copy/check failure rolls back with old tables
   intact and writes no migration-ledger row.
6. The ordered ledger makes a completed migration a no-op on restart. A crash
   before commit leaves the Phase 1 schema; a crash after commit leaves exactly
   one ledger row and one synthetic Account per Device.

No Fake alias is generated, so there is no migrated alias collision policy to
guess. The internal Account ID is deterministic; existing Profile and Session
IDs are unchanged. The old explicit-ID `run fake` command remains the only Fake
selection surface, and `accounts/profiles list` exclude internal rows unless a
test-only repository query is used.

The existing `run fake` path remains compatible and explicit. New account and
profile commands are additive. Real Provider names may exist as manual metadata
before their adapters are enabled; attempts to run them return
`provider_capability_unavailable` rather than falling through to Fake.

The API schema begins at version 1 and uses cursor pagination. Provider
fixtures are pinned to exact versions. New Usage window kinds and fields are
additive; unknown kinds round-trip without reinterpretation.

The current local `user-operations-guide` branch is not part of remote `main`.
This feature must not cherry-pick it implicitly. User-guide integration is a
separate explicit branch/integration action after the product state is updated.

## Rollback

- Before P1 migration, create the normal Device DB backup/checkpoint. v0.1 does
  not promise downgrade migrations; rollback means disable the new commands and
  restore the pre-migration backup, or roll forward with a corrected binary.
- P1 contains no real Provider secret or browser integration and can be disabled
  without affecting existing Fake Sessions.
- P2/P3 adapters are feature-capability gated. On regression, disable the exact
  Provider version, retain Account/Profile metadata and require official direct
  CLI fallback.
- P4 dashboard is read-only metadata first; rollback removes the UI route while
  preserving local records.
- No rollback may select another account, copy an auth home or convert unknown
  usage to pass.
