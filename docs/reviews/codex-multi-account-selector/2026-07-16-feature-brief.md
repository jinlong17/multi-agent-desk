# Feature Brief: Codex explicit multi-account selector

- Slug: `codex-multi-account-selector`
- Date: `2026-07-16`
- Owner module: `provider`
- Impacted modules: `core`, `security`, `desktop`, `project-system`
- Requested by: operator continuation of `multi-account-usage-control` P2

## Module classification

Owner: `provider`  
Confidence: high  
Why: the change binds Account/Profile selection to the existing Codex
app-server, official login, Usage, and managed `CODEX_HOME` runtime.  
Impacts: `core` for CLI/application contracts and persistence, `security` for
identity confirmation and credential binding, `desktop` for later capability
display, and `project-system` for workflow/dashboard truth.  
Branch: `codex/provider/codex-multi-account-selector`  
Workflow: `feature`  
Gates: exact Linux `0.144.2` Provider matrix; open feature Security Gate;
macOS distinct-identity and real Windows Codex remain capability gates.  
Docs: this Feature Brief and
`docs/workflow/features/codex-multi-account-selector/`.

## Motivation and measurable outcome

The repository already has a verified P1 Account/Profile/Usage registry and a
shipped Phase 2 Codex Vault/runtime using raw IDs. The missing product slice is
the safe connection between them. An operator should be able to preview an
explicit `@alias`, confirm exactly one Account/Profile/Credential/Workspace and
the latest Usage source, then start the exact supported Codex runtime without
typing opaque IDs or permitting silent account substitution.

The measurable outcome is one Linux `0.144.2` CLI/TUI path that performs
preview -> explicit confirmation -> pinned Session start, preserves the
selected tuple for the Session lifetime, and scopes status/logout/re-login to
that CredentialInstance. A wrong, stale, disabled, ambiguous, or unsupported
selection must create no Session and materialize no credential.

## Scope

- Resolve one explicit `@alias` to the P1 Account/Profile/Credential/Device
  tuple using existing revisioned registry rules.
- Produce a redacted preview containing Provider, alias/display labels,
  RuntimeProfile, Device, Workspace, exact CLI compatibility, auth health, and
  latest Usage source/freshness. Never expose raw OAuth or Provider identity.
- Require an operator confirmation bound to the previewed tuple and revision;
  reject stale confirmations and any tuple drift.
- On official login/re-login, show an ephemeral redacted Provider identity
  projection and require explicit operator confirmation that it is the
  intended internal Account. Persist no email/display name/raw claim. The
  supported product contract is explicit human confirmation, not automated
  upstream identity inference.
- Start the existing shared-per-Credential Codex runtime only for exact Linux
  CLI `0.144.2`; preserve ADR 0014 writer lease, Vault CAS, Account/Profile/
  Workspace binding, ControllerLease, Usage, Approval, and cleanup behavior.
- Add alias-aware auth status/logout/re-login commands that operate only on the
  selected CredentialInstance and retain the active-Session logout denial.
- Keep macOS visible as schema/empty-home compatible but not distinct-account
  supported; keep real Windows Codex unsupported. Both return typed capability
  states rather than silently using the Linux claim.
- Update CLI/TUI help, user guide, compatibility documentation, tests, and
  dashboard state for the exact supported slice.

## Non-goals

- Automatic account rotation, quota evasion, recommendation-triggered launch,
  or mid-Session identity switching.
- Using email/display name as a durable identity key, parsing private Provider
  endpoints, copying browser cookies, or persisting raw JWT/OAuth claims.
- A second Codex refresh writer, independent writable auth copies, device-auth
  stable support, CredentialGrant, Control Plane sync, Web dashboard, or remote
  browser login.
- Stable macOS distinct-account or real Windows Codex support without separate
  evidence and review.
- Provider continuation, dynamic policy amendments, or unsupported Approval
  types excluded by the shipped Phase 2 slice.

## User journeys

1. The operator runs a preview command with `@work`, sees the resolved Account,
   Profile, target Device/Workspace, exact compatibility, auth health, and
   Usage freshness, then confirms and starts a pinned Codex Session.
2. If the Profile is logged out, the operator starts official interactive
   login for that exact alias, confirms the ephemeral redacted upstream
   identity projection, and retries the preview with a new revision.
3. The operator stops only the selected alias's Sessions, logs out that alias,
   and observes that another Account's Session, Vault item, Home, Usage, and
   auth health remain unchanged.
4. On macOS with only schema evidence or on Windows, the same command explains
   the unsupported capability and deterministic fallback without creating a
   Session or materializing auth.

## Data and trust boundaries

- Device DB stores Account/Profile/Credential/Usage metadata and revisioned
  confirmation facts; the Vault stores Provider credential plaintext only in
  encrypted form at rest.
- Managed auth plaintext exists only in the CredentialInstance-scoped private
  Home while the official Provider runtime owns it. Same-user malware,
  root/admin, Provider compromise, backups, and crash tooling remain residual
  risks.
- The confirmation binds internal IDs/revisions and an ephemeral redacted
  Provider identity view. No PII or raw auth payload enters logs, dashboard,
  review artifacts, Control Plane, or Git.
- Local IPC authentication/capabilities and ControllerLease remain mandatory;
  alias convenience never bypasses authorization.
- Control Plane and Web are out of this slice and receive no Provider
  credential or Session plaintext.

## Provider assumptions

- Exact Linux Codex CLI `0.144.2` app-server, login, Usage, Approval, and shared
  runtime behavior remains the only accepted live Provider row.
- The `spike-codex-distinct-account-homes` decision proves two official
  identities can coexist on that exact Linux arm and that scoped logout/
  re-login does not mutate the other CredentialInstance.
- `account/read` is suitable for a transient redacted confirmation view but is
  not assumed to expose an official durable non-PII identity key. The feature
  therefore relies on explicit operator confirmation and fails closed if the
  required projection is unavailable or ambiguous.
- Unknown versions/schemas and changed Provider semantics are probed and
  disabled, never inferred compatible.

## Dependencies and gates

- `multi-account-usage-control` P1 reconciliation: `VERIFIED`.
- `phase2-codex-vertical-slice`: shipped on remote `main` at the recorded final
  baseline and reconciled locally with P1.
- `spike-codex-distinct-account-homes`: `GATE_RESOLVED`.
- ADR 0014 distinct-account addendum and compatibility row are authoritative.
- Feature Security Gate is open for identity confirmation, Vault/revision,
  scoped logout, audit/redaction, and unsupported-platform behavior.
- The plan must be independently reviewed before build; each build phase stops
  for independent verification.

## Acceptance criteria

- [ ] `@alias` resolves exactly one enabled Codex Account/Profile/Credential and
      preview binds Account, Profile, Credential, Device, Workspace, Usage
      snapshot/freshness, auth revision, and exact Provider compatibility.
- [ ] Confirmation replay, tuple/revision drift, ambiguous alias, disabled
      Account/Profile, logged-out Credential, wrong Provider, and unsupported
      platform/version all fail before Session insert/materialization.
- [ ] Official login/re-login requires an ephemeral redacted identity
      confirmation; no email, org, display name, raw claim, auth JSON, URL/code,
      token, or cookie is persisted.
- [ ] Two distinct Linux `0.144.2` aliases can run concurrently with different
      Provider session IDs and account-bound Usage; no automatic selection or
      mid-Session switch occurs.
- [ ] Active target logout is denied; after explicit target stop, scoped
      logout/re-login changes only that CredentialInstance and advances its
      revision while the other Account remains byte/session/Usage stable.
- [ ] One CredentialInstance still has at most one writable managed Home/
      app-server writer, including concurrent preview/start/logout races and
      restart recovery.
- [ ] CLI/TUI and docs clearly label Linux exact support, macOS pending
      distinct-account acceptance, and real Windows Codex unsupported.
- [ ] Focused/full Go tests, race tests, migration/restart tests, Linux live
      acceptance, Darwin/Linux/Windows builds, Web/Rust scaffold checks, and
      workflow/dashboard/governance checks pass.

## Risks and unresolved questions

- Upstream `account/read` identity projection may be absent or change. The plan
  must define the minimum redacted confirmation UX and typed failure without
  relying on email as a durable identifier.
- The existing raw-ID `run codex` path must not become a bypass around preview
  and confirmation for product use; compatibility/debug access needs an
  explicit boundary.
- Login confirmation and Session confirmation may need separate revisions to
  prevent time-of-check/time-of-use races.
- Linux live acceptance requires operator participation for official login but
  must reuse normal work rather than hidden quota probes.
- macOS and Windows product messaging must remain capability-based so a future
  evidence row can be added without changing the security contract.

## Evidence

- `docs/reviews/spike-codex-distinct-account-homes/2026-07-16-security-review.md`
- `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes-spike.md`
- `docs/adr/0014-codex-app-server-single-writer-auth.md`
- `docs/workflow/features/multi-account-usage-control/p1-as-built.md`
- `docs/workflow/features/phase2-codex-vertical-slice/dev_log.md`
- `docs/PROVIDER_COMPATIBILITY.md`

## Handoff

Next role: `feature-plan`.
