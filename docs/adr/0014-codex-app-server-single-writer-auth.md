# ADR 0014: Codex app-server uses a single credential writer

- Status: Accepted
- Date: 2026-07-14
- Owner: `provider`
- Impacted modules: `core`, `security`
- Security gate: accepted by `docs/reviews/spike-codex-auth-refresh/2026-07-14-security-review.md`

## Context

MultiAgentDesk needs Codex account health, usage/rate-limit data, managed login,
refresh, and multiple sessions without corrupting or silently switching an
account. Codex app-server exposes versioned schemas and account methods, while
its ChatGPT credentials live in `auth.json` under `CODEX_HOME` and may be
rotated by Codex itself.

Phase 0.5 replayed app-server initialization, `account/read`,
`account/rateLimits/read`, and `account/usage/read` with Codex CLI `0.142.5`,
`0.143.0`, and `0.144.2` on macOS. Managed proactive refresh passed with
`0.144.2`. A same-account macOS `0.144.2` and Linux `0.144.4` short run produced
four successful hourly read samples across `10812.845` seconds; its first
sample refreshed both devices and retained readable credentials and account
equality. Both observed auth files were mode `0600`.

The operator cancelled the original 48-hour requirement. Neither official
documentation nor the short run defines production-safe multi-writer refresh
token rotation. Completed isolated device-auth login was also not observed.
Those capabilities therefore remain unsupported rather than being inferred
from a short successful sample.

## Decision

Use official Codex app-server over stdio with a version-gated adapter, and give
each `CredentialInstance` exactly one canonical writable app-server/auth-home
owner.

The Codex adapter must:

- generate or load the schema for the exact CLI version and enable only methods
  whose schema and fixture replay pass; an unknown version is probed or
  downgraded, never assumed compatible;
- identify generated schema through canonical JSON hashing (sorted relative
  paths plus canonical parsed content), because raw aggregate-schema object
  ordering is not deterministic; reject duplicate/invalid JSON and symlinks;
- report successful `account/rateLimits/read` and `account/usage/read` values as
  `official/high` with source version and freshness, without using them to
  rotate or silently switch accounts;
- let `CredentialMaterializationManager` acquire one exclusive lease and own
  one writable Codex app-server plus canonical managed `CODEX_HOME` for each
  `CredentialInstance`;
- multiplex sessions that use the same credential through that managed
  app-server. Session/profile configuration and history may be separated, but
  they must not create independent writable copies of refreshable `auth.json`;
- route proactive refresh through the canonical owner, validate resulting
  credential structure, and commit changes to the local Vault with monotonic
  `credentialRevision` compare-and-swap;
- reject a second refresh writer and quarantine ambiguous crash residue rather
  than selecting a credential by mtime or last-writer-wins;
- use restrictive auth-home permissions, secret-safe diagnostics, a pinned
  Provider binary/version and pinned session account/profile;
- support official interactive login as the stable path. Device-auth initiation
  may be exposed as experimental, but completed headless login is not a
  compatibility claim until separately evidenced.
- treat Provider-initiated Approval as a JSON-RPC server request whose request
  ID is preserved through local ControllerLease and idempotency authorization;
  never invent a client Approval method alias from a notification fixture.

The Control Plane never becomes a credential writer and never receives
Provider plaintext. Credential grants remain target-bound E2EE operations to
an authorized Device; revocation blocks future materialization but cannot erase
credentials already copied by an authorized or compromised host.

## Consequences

### Positive

- Phase 2 may freeze the Codex vertical-slice contract without waiting for a
  long-duration multi-writer experiment.
- Usage and rate-limit capabilities have exact versioned evidence and an
  explicit downgrade path.
- Refresh concurrency is bounded by the existing Vault lease/revision model
  instead of undocumented Provider behavior.

### Obligations and residual limits

- A single managed app-server per credential is a stronger lifecycle boundary
  than one app-server per session; multiplexing, failure isolation, and session
  routing must be tested in Phase 2.
- Codex, same-user processes, host administrator/root, malware, backups, and
  crash collectors may access a materialized auth home. File mode `0600` does
  not protect a compromised runtime or administrator.
- Upstream schemas, credential formats, and refresh semantics may change. A
  failed probe or ambiguous refresh requires quarantine/re-login, not guessed
  recovery.
- The evidence does not support multi-writer refresh, completed headless device
  login, 48-hour stability, or versions/platforms outside the matrix.

## Phase 2 implementation evidence (2026-07-16)

The Phase 2 Codex vertical slice implements this decision for exact CLI
`0.144.2`:

- a private portable Vault and owner-bound official interactive enrollment;
- one materialized writable `CODEX_HOME` and one shared app-server per
  `CredentialInstance`, with lease refresh, digest validation, revision CAS,
  quarantine, and bounded cleanup;
- strict schema/version/profile/account/workspace binding, bounded
  credential-free proxy inheritance, and auth-only Vault import;
- per-Session thread bindings, second-CLI lease/input/observe, official Usage,
  request-ID-preserving Approval dispatch, and binding-scoped stop/kill;
- typed unsupported behavior for conversation resize, Provider continuation,
  permissions Approval, session-persistent decisions, and policy amendments.

The exact Linux `0.144.2` live exit and macOS `0.144.2` canonical-schema /
empty-home handshake smoke pass. Windows is build/protocol evidence only, the
currently bundled macOS `0.144.5` is not allowlisted, and final feature Security
Review remains a separate gate.

## Distinct-account managed Home addendum (2026-07-16)

The security-reviewed `spike-codex-distinct-account-homes` evidence extends
this decision for the exact Linux `x86_64` Codex CLI `0.144.2` arm:

- two operator-owned official interactive identities may coexist only as two
  distinct CredentialInstances, Vault items, canonical Homes, writer leases,
  app-server runtimes, Account bindings, and monotonic revision streams;
- active Sessions block logout, and revocation reservation prevents a new
  Session from racing target-scoped Home/Vault cleanup;
- logout/re-login is CredentialInstance-scoped and must never stop, revoke,
  mutate, or rotate another Account; re-login may reuse the selected
  CredentialInstance only through revision CAS;
- concurrent Usage remains advisory, source/freshness labelled, and bound to
  the selected Account. It never authorizes automatic account rotation or a
  mid-Session identity switch;
- before an alias/selector launch becomes a stable product path, the
  post-login Provider identity must be bound to the intended Account with a
  privacy-preserving stable identifier or explicit operator confirmation.
  Email and display-name PII are not identity keys, and internal metadata tuple
  equality alone is not proof of the upstream account choice.

This addendum does not accept macOS distinct-identity behavior, real Windows
Codex behavior, passive-soak stability, multi-writer refresh, or versions
outside the exact compatibility row. The fallback remains one explicitly
selected, officially logged-in target-local managed Home with no automatic
switch and fail-closed quarantine/re-login on identity, version, or revision
ambiguity.

## Evidence

- `docs/spikes/codex/2026-07-14-auth-refresh-spike.md`
- `docs/spikes/codex/app-server-account-matrix.json`
- `docs/spikes/codex/two-device-short-run.json`
- `docs/reviews/spike-codex-auth-refresh/2026-07-14-security-review.md`
- `docs/reviews/phase2-codex-vertical-slice/2026-07-16-feature-verify-p3b.md`
- `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes-spike.md`
- `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes.json`
- `docs/reviews/spike-codex-distinct-account-homes/2026-07-16-security-review.md`
