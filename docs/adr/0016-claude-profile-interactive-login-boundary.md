# ADR 0016: Claude profiles use target-local interactive login

- Status: Accepted
- Date: 2026-07-14
- Owner: `provider`
- Impacted modules: `core`, `desktop`, `security`
- Security gate: accepted by `docs/reviews/spike-claude-config-keychain/2026-07-14-security-review.md`

## Context

Claude Code exposes an interactive CLI/PTY integration and stores authenticated
state differently by platform. MultiAgentDesk needs isolated RuntimeProfiles,
machine-readable health, Linux execution, and truthful credential-grant and
revocation behavior without copying the macOS Keychain or depending on an
unverified long-lived setup token.

Phase 0.5 observed with Claude Code `2.1.207` on macOS that two empty
`CLAUDE_CONFIG_DIR` profiles did not inherit the default Keychain login, Profile
A could complete official interactive login while Profile B stayed logged out,
and logout scoped to A did not log out the default profile. No credentials file
appeared in the macOS profile. macOS `2.1.207` and Linux `2.1.132` returned the
same seven-key `auth status --json` schema and represented the same account in
memory; identity values were not persisted. Linux empty profiles also did not
inherit the default credentials.

The setup-token PTY initiated and survived resize, but hCaptcha was not bypassed
and no token was issued, injected, run long-term, or revoked. Official evidence
does not define targetable per-setup-token revocation. Real requests were
account quota/session limited rather than authentication failures, so long
session continuity was not proven. The operator requires one account for v0.1;
distinct-account isolation is not claimed.

## Decision

Use official interactive login on each target device and target
`CLAUDE_CONFIG_DIR` as the stable Claude v0.1 authentication path.

The Claude adapter must:

- create one Daemon-owned restrictive config directory per RuntimeProfile and
  pass it only through `CLAUDE_CONFIG_DIR`; never scan, copy, export, or modify
  the macOS Keychain database or unrelated profiles;
- invoke official interactive login for that exact target profile and bind the
  validated profile/account choice to the Session. It never auto-rotates,
  inherits the default profile silently, or switches profiles after an error;
- version-gate `claude auth status --json`. Normal state may retain only
  allowlisted `loggedIn`, auth/provider class, CLI version, and validation time;
  email, organization fields, raw JSON, browser URL/code, credentials, and
  terminal capture remain redacted;
- classify quota/session-limit separately from authentication health and never
  treat quota as authorization to select another account;
- keep setup-token and `CLAUDE_CODE_OAUTH_TOKEN` CredentialGrant disabled in
  stable v0.1. Enabling it requires separate evidence for issuance, injection,
  secret-safe process handling, long-session behavior, expiry, and revocation,
  followed by another security review;
- treat local profile logout as local state removal, not Provider-wide
  revocation or remote erasure. Broader revocation guidance points to official
  account/admin controls;
- fail closed with `interactive_login_required` when the target profile is not
  logged in or auth health is unknown.

Phase 3 must prove real Claude PTY input, resize, reconnect/replay, stop, and
long-session behavior on the target Linux profile. Those are implementation
acceptance items, not claims from this Spike.

## Consequences

### Positive

- macOS profile isolation and JSON health have exact versioned evidence.
- Stable v0.1 avoids copying Keychain material and avoids an unverified token
  issuance/revocation contract.
- One-account operation can still use multiple profiles/configurations without
  claiming distinct simultaneous identities.

### Obligations and residual limits

- Every new target device/profile requires an explicit official login; Claude
  credentials are not remotely provisioned by stable v0.1 CredentialGrant.
- Account quota prevented a successful long-session Spike; Phase 3 remains
  responsible for real-session continuity acceptance.
- Upstream Keychain service naming, config layout, JSON fields, login UI, and
  credential behavior may change and require a compatibility re-probe.
- Same-user malware, host administrator/root, browser compromise during login,
  Keychain/credential-file access, Claude itself, backups, and crash tooling can
  use or copy authenticated state. Local logout cannot erase remote copies.

## Evidence

- `docs/spikes/claude/2026-07-14-config-keychain-spike.md`
- `docs/spikes/claude/auth-profile-matrix.json`
- `docs/spikes/claude/profile-session-control.json`
- `docs/reviews/spike-claude-config-keychain/2026-07-14-security-review.md`
