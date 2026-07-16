# Test strategy: 多账号用量看板与显式调用

## Acceptance matrix

| Requirement | Level | Command/scenario | Expected evidence |
|---|---|---|---|
| Unbounded manual registry | unit/integration | create 8 mixed Codex/Claude Accounts and 12 Profiles, paginate at 3 | all round-trip; no fixed-count constant or truncation |
| Alias contract | unit/property | case, prefix, length, Unicode/path/shell injection corpus | canonical uniqueness; unsafe selectors reject before write |
| Migration | integration | open Phase 1 DB with multiple Fake Profiles/credentials/Sessions, apply migration, restart twice | IDs/tuples preserved; deterministic internal Account per Device; null Fake aliases; no duplicate migration |
| Explicit binding | integration | preview `@A`, mutate/disable mapping, then start | `profile_binding_changed`/disabled; no Provider process starts |
| Cross-account isolation | provider manual | two distinct identities per Provider in separate Homes; status/request/logout canaries | expected identity per Home; A never changes B; sanitized artifact |
| Generic Usage | unit/fixture | 0, 1, 2, 4 windows; missing values; spend/month/unknown | lossless normalized arrays; unknown never becomes zero |
| Codex quota | contract/live manual | exact schema + fixture + rate-limit read/update | duration, used, reset and buckets match accepted schema |
| Claude 5h/7d | contract/live manual | real Session emits official status-line JSON | correct account-bound snapshot after first response; no hidden probe |
| Claude monthly | spike | inspect supported CLI/SDK/account output without private API scraping | supported fixture or explicit `unavailable`; never inferred |
| Login lifecycle | integration/manual | cancel, timeout, wrong browser identity, daemon crash, retry | target Profile remains login-required/mismatch; no cross-bind |
| Delete safety | integration/security | active Session/materialization then disable/delete; unexpected credential | hard delete blocked or `provider_cleanup_required`; atomic metadata delete/tombstone; Grant case remains P5-gated |
| Web truthfulness | component/E2E | zero/100/unknown/unavailable/stale cards at mobile/desktop sizes | distinct accessible labels, source, freshness and reset shown |
| No rotation | static/integration | quota exhausted/auth failure on A with B healthy | no B process/start; explicit error and user selection required |
| Remote fallback | E2E/manual | target lacks supported Grant | explicit target-local login; no Cookie/auth-home copy |

## Unit and property tests

- Account, CredentialInstance, RuntimeProfile and UsageSnapshot validation.
- Alias parser/canonicalizer fuzzing with bounded input; leading `@` accepted
  only by selector interfaces, not stored.
- Case-insensitive uniqueness and revision conflicts.
- Account/Profile lifecycle and referential delete rules.
- Exact P1 create transaction: Account + default Profile only; no Credential,
  Vault item, Provider Home, directory, Keychain mutation or process.
- Arbitrary Usage window ordering, optional fields, percent bounds, reset time,
  stale calculation and unknown-kind round-trip.
- Auth status and availability remain orthogonal.
- Session preview/confirmation tuple equality and immutable snapshot.
- P1 filesystem snapshot is unchanged by create/update/disable/delete. Provider
  Home derivation, path traversal and symlink checks begin in P2/P3.
- Redaction allowlist for auth/status/usage events and errors.
- No hard-coded account count; pagination boundaries at 0, 1, 50, 200 and 201.

## Contract and fixture tests

- Codex app-server schema generation and fixture replay for each accepted
  version: initialize, account read, rate-limit read/update, usage read, login
  state, logout and refresh-owner behavior.
- Codex `RateLimitWindow` mappings including absent primary/secondary,
  `rateLimitsByLimitId`, spend control and unknown limit ID.
- Claude `auth status --json` fixtures with allowlisted fields only.
- Claude official status-line fixtures with absent-before-first-response,
  five-hour/seven-day values, reset epoch, null/extra fields and unknown version.
- Provider raw fixture secret scanner; no email/org/token/URL/code/cookie/auth
  content may enter Git.
- Local IPC and future OpenAPI schema compatibility, cursor pagination and
  stable error codes.

## Integration and E2E

P1 automated:

1. Bootstrap Device, create 8 Accounts and 12 Profiles through authenticated
   IPC, update names/aliases with revisions, paginate both lists at limit 3,
   reject filter/cursor reuse, show/disable/enable/delete, restart Daemon, and
   verify durable aliases plus no fixed-count ceiling.
2. Seed synthetic UsageSnapshots for multiple window shapes and verify CLI JSON
   plus human output retain source/freshness/unknown distinctions.
3. Resolve `@A`; Codex/Claude validate/login/run/refresh return
   `provider_capability_unavailable` with no subprocess and never select another
   Profile. Migrated Fake rows are not public aliases; `run fake` continues only
   through its shipped explicit IDs.
4. Migrate a Phase 1 fixture containing multiple Fake Profiles, credentials and
   Sessions. Verify one deterministic internal Account per Device, preserved
   Profile/Credential/Session IDs and exact Session tuples, null internal aliases,
   foreign-key check, rollback on injected collision/check failure, restart
   idempotence, and the existing native Fake Session scenario.
5. Delete a Profile and an Account-with-multiple-Profiles under revision and
   reference races. Prove atomic rollback, minimal tombstones, alias release,
   internal Fake protection, and `provider_cleanup_required` for an unexpected
   CredentialInstance. CredentialGrant cases are explicitly not executable until P5.

P2/P3 manual release validation:

1. Use two operator-owned Codex identities and two Claude identities. No CI
   secret or identity value is persisted.
2. Complete official login in four isolated Homes, compare expected identity
   only in process memory and store booleans/digests.
3. Run one minimal real request per Profile with explicit operator approval,
   verify Provider-side identity, collect quota fields and check scoped logout.
4. Run A/B concurrently, trigger read/refresh/Session lifecycle and verify no
   auth file/Keychain/Profile mutation crosses boundaries.
5. On Linux, repeat target-local login and alias invocation. Headless Codex
   device auth remains experimental until a completed flow passes.

P4 Web E2E:

- Add/remove/reorder 1, 6, 20 and 201 mocked accounts with pagination.
- Validate keyboard navigation, screen-reader labels, color-independent quota
  states, narrow viewport, stale timestamps and timezone rendering.
- Prove browser storage contains only MultiAgentDesk Device/session data, not
  Provider Cookie/auth values.

## Security/adversarial tests

- Malicious alias/display name/provider arguments cannot inject shell, flags,
  paths, logs or HTML.
- Concurrent add/update/disable/delete uses idempotency plus expected revisions;
  P1 creates no credential writer. P2/P3 login/logout serialization is gated.
- OAuth/login callback state mismatch, wrong Profile completion and replay fail
  closed.
- Symlink Provider Home, permissive file mode/DACL, hard link, crash residue and
  stale credential revision are rejected/quarantined.
- Secret scanning covers process argv, env diagnostics, audit events, DB,
  generated dashboard state, fixtures and debug bundle.
- Unapproved Web Device cannot invoke login/logout/usage refresh or see
  Provider identity hints beyond authorized metadata.
- Quota exhaustion, 401, 429, malformed schema and Provider crash never trigger
  another account automatically.
- Delete with active Session/materialization is denied; unexpected P1 external
  cleanup refuses atomically. Grant checks start before P5 enables Grant create;
  local deletion is never labeled Provider-wide revocation.
- Claude policy gate is testable in UI/CLI: stable capability remains disabled
  until an accepted decision artifact is present.

## Cross-platform matrix

| Scenario | macOS arm64 | macOS x64 | Linux x64 | Linux arm64 | Windows x64 |
|---|---:|---:|---:|---:|---:|
| P1 registry/alias/usage storage | required | cross-build | required | cross-build | required |
| P1 no-Home filesystem invariant | required | cross-build | required | cross-build | required |
| Codex two-home auth/usage | live required | build | live required | build | acceptance |
| Claude two-profile auth/usage | live required | build | live required | build | acceptance |
| CLI selector and no-rotation | required | build | required | build | required |
| Web dashboard responsive/accessibility | browser required | n/a | browser CI | n/a | Edge required |

Real Provider credentials run only in operator-approved isolated validation;
protected CI uses sanitized fixtures and Fake Provider canaries.

## Failure injection and recovery

- Kill Daemon during the P1 Account/Profile transaction, usage write and
  deletion; Home staging/login failures begin in P2/P3.
- Kill canonical Codex app-server before/after refresh and ensure CAS/quarantine
  behavior from ADR 0014.
- Make usage source unavailable while retaining an older snapshot; verify stale
  display and no selection change.
- Lock Vault before login/run/logout; verify metadata-only reads and stable
  `vault_locked` errors.
- Corrupt/upgrade Provider JSON schema; exact version downgrades without
  reinterpreting raw data.
- Change alias between preview/start, revoke Account during login, and expire
  LoginAttempt while browser remains open.
- Exhaust disk/global Usage quota and preserve newest state plus truncation
  marker.

## Manual acceptance

- Operator identifies at least two distinct test accounts per Provider and
  confirms which browser Profile completes each login. No account identifier is
  copied into repository evidence.
- Operator visually verifies one Accounts/Usage page containing 6+ Profiles,
  mixed available/limited/unknown/stale states and correct local timezone reset
  labels.
- Operator runs interactive Codex/Claude and one explicitly approved
  non-interactive call through `@A`/`@B`, then confirms Provider identity and
  account-specific quota source.
- Operator performs local disable/logout/delete and reviews Provider-side
  revocation wording.
- Operator approves any CredentialGrant test separately; otherwise remote
  server acceptance uses target-local official login.
- Anthropic policy applicability is recorded before Claude subscription
  login/rate-limit capability is marked stable.
