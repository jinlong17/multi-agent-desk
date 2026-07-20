# P2 as-built: selector-bound exact Linux runtime

## Implemented boundary

P2 connects the verified P1 selector/confirmation boundary to the shipped
Codex runtime without adding a second Session or credential writer.

- A confirmed `session.start` consumes its daemon-issued preview and inserts
  one `starting` Session transactionally. `Runtime.StartReserved` launches the
  Provider against that exact persisted tuple; it never accepts a replacement
  Account, Profile, Credential, Device, or Workspace.
- The post-reservation start re-discovers the official Codex binary and
  rechecks the version, binary fingerprint, schema fingerprint, and capability
  digest immediately before materialization/spawn. Drift records the reserved
  Session as failed instead of silently switching binaries or Accounts.
- Concurrent replay of the same reserved start is coalesced by Session ID. A
  shared Credential runtime must match the preview fingerprints before it can
  accept another binding.
- Runtime ownership remains keyed by CredentialInstance. Distinct A/B
  Credentials therefore receive distinct managed Homes, app-server processes,
  Provider thread IDs, immutable Session tuples, and Usage snapshots.
- `usage --profile @alias --refresh` resolves the public selector, requires a
  healthy confirmed Credential and an active supported runtime, calls the
  official `account/usage` method, and persists the exact Account,
  CredentialInstance, Device, Provider version, availability, observation,
  staleness, and redacted response digest.
- Human CLI and minimal TUI starts use the same selector preview and typed
  alias confirmation contract. Public raw Profile IDs remain rejected.
- Administrative Profile operations use a separate explicit target resolver
  so legacy raw Profile IDs can be repaired without weakening public selector,
  auth, Session-start, or Usage boundaries.
- Alias-scoped logout retains the active-Session denial and revocation
  reservation. Re-login creates and binds a new Credential only for the
  selected Profile; another Account's Profile, Credential, Session, and Usage
  remain unchanged.

## Live exact-Linux acceptance

The operator-approved Linux amd64 target ran the exact Codex CLI `0.144.2`
with two distinct operator-owned identities through the public `@A` and `@B`
selector path.

- A and B ran concurrently with distinct local Session IDs, Provider thread
  IDs, CredentialInstance IDs, and Account IDs.
- Official Usage refresh produced distinct `usage_*` rows and distinct 64-character
  redacted response digests. The final projection returned non-null matching
  CredentialInstance IDs, Provider version `0.144.2`, availability
  `available`, and persisted observation/staleness timestamps.
- Logout of active B failed closed. After B stopped, B logout and official
  re-login produced a new B CredentialInstance while A remained running and
  unchanged; B then started successfully with the new Credential.
- Both final Sessions stopped normally. Vault was re-locked, the acceptance
  daemon was stopped, and the managed Home tree contained zero materialized
  `auth.json` files.

No Provider identity, email, organization, token, OAuth code, auth payload,
quota value, prompt, or transcript is recorded in this artifact.

## Explicit limits

- The only live support claim remains Linux amd64 plus Codex CLI `0.144.2` and
  its accepted schema. macOS distinct-identity acceptance and real Windows
  Codex remain P3 capability gates; successful cross-compilation is not live
  platform acceptance.
- The accepted official `account/usage` response in this exact version exposes
  metadata/digest evidence but no reviewed quota-window projection. P2 does
  not fabricate quota values. A fresh snapshot uses `stale_at == observed_at`
  as the conservative existing contract until a later reviewed freshness
  policy exists.
- Operator typed alias confirmation is an internal target attestation, not an
  upstream identity claim.
- The Linux SSH server warns that its current connection does not negotiate a
  post-quantum KEX. This is an environment hardening item and does not broaden
  the product's support or security claims.

## Rollback

Disable the exact compatibility row to prevent new selector starts, stop
existing Sessions through their controller leases, and leave migration 7,
Vault items, Session history, and Usage evidence intact. No automatic Account
fallback or credential rotation is permitted.
