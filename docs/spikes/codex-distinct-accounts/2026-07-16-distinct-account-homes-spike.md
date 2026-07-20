# Codex distinct-account managed Home spike

Date: 2026-07-16
Owner: `provider`
Workflow target: `spike-codex-distinct-account-homes`

## Verdict

The hypothesis is supported for two operator-owned identities on the exact
Linux `x86_64` Codex CLI `0.144.2` test arm, subject to Security Review. Two
official interactive logins remained isolated in two Daemon-managed Homes;
the resulting sessions and Usage reads remained account-bound; target-B logout
and re-login did not mutate target A.

This is not a cross-platform support claim. A distinct-identity macOS run, any
real Windows run, and the optional passive soak were not executed. The stable
product fallback therefore remains one explicitly selected, officially logged
in managed Home per target until those gates are separately accepted.

## Environment

| Field | Value |
|---|---|
| Target | Linux `5.4.0-148-generic`, `x86_64` |
| Codex CLI | `codex-cli 0.144.2` |
| Codex SHA-256 | `8b5b8bf86fb661c10787b5a5c70d7ba2cf3e61f3c902877509174b45d13fa6aa` |
| MultiAgentDesk binary | branch build at `87a8aee`; SHA-256 `f619d6881203c5c99461ffa0839c0b6acd5549951efa65fbb20f842a256bc160` |
| Device store | schema `5`; root mode `0700` |
| Auth | two official interactive Codex logins; operator completed provider account selection and MFA |
| Callback | SSH local forward to the enrollment callback listener; no callback URL, code, token, cookie, claim value, email, or display name retained |

The SSH host warned that its negotiated connection lacked a post-quantum key
exchange. This does not change the Provider result, but the target SSH service
should be upgraded before it is treated as a production deployment surface.

## Success and failure criteria

The exact Linux arm succeeds only when:

1. Two clean official logins produce two healthy CredentialInstances and two
   Vault items without copying Provider state between Homes.
2. Both materialized `auth.json` files are `0600`, and in-memory claim parsing
   reports two distinct identities without persisting raw identity values.
3. Two concurrent Sessions have distinct Provider session IDs, and concurrent
   Usage reads contain only the selected Account ID.
4. Logout fails closed while the target Credential is active; after stopping
   only target B, B logout removes only B's Vault/materialized state while A's
   auth bytes, running Session, and Usage remain unchanged.
5. B can complete a new official login into the same CredentialInstance with a
   monotonic revision, after which both identities can run concurrently again.

The arm fails if either Home inherits the other identity, logout is global,
account binding is ambiguous, raw Provider secrets must be copied, or the
other Account changes during B logout/re-login.

## Reproduction commands

The following is the exact command shape. Opaque IDs are intentionally replaced
with typed placeholders; obtain them from the local registry rather than
copying IDs from this report.

```bash
uname -srmo
codex --version
sha256sum codex multidesk

multidesk auth begin --root <device-root> \
  --profile-id <profile-a> --credential-id <credential-a> --json
multidesk auth begin --root <device-root> \
  --profile-id <profile-b> --credential-id <credential-b> --json

multidesk run codex --root <device-root> --workspace <workspace-id> \
  --device-id <device-id> --account-id <account-a> \
  --credential-id <credential-a> --profile-id <profile-a> --json
multidesk run codex --root <device-root> --workspace <workspace-id> \
  --device-id <device-id> --account-id <account-b> \
  --credential-id <credential-b> --profile-id <profile-b> --json

multidesk usage --root <device-root> --provider codex \
  --account <account-a> --json
multidesk usage --root <device-root> --provider codex \
  --account <account-b> --json

multidesk control acquire --root <device-root> --json <session-b>
multidesk sessions stop --root <device-root> \
  --revision <lease-revision> --json <session-b>
multidesk auth logout --root <device-root> \
  --credential-id <credential-b> --json
multidesk auth begin --root <device-root> \
  --profile-id <profile-b> --credential-id <credential-b> --json
```

During the login commands, the operator used the official Provider page and
completed MFA. The enrollment timeout was ten minutes; an earlier expired
attempt was cancelled without installing Credential material.

## Sanitized observations

| Check | Sanitized result |
|---|---|
| Healthy Credentials / Vault before logout | `2 / 2` |
| Managed Homes while both Sessions ran | `2`; both `auth.json` modes `0600` |
| Identity comparison | subjects present; email claims present; identities distinct; raw values discarded in memory |
| Concurrent Sessions | `2`; Provider session IDs distinct |
| Concurrent Usage | both `supported`, `available`, and all snapshots bound to the selected Account |
| Logout while B active | blocked with a conflict; no mutation |
| After stopping and logging out B | B revoked, Vault count `1`; A auth digest unchanged, A Session running, A Usage account-bound |
| B re-login | same CredentialInstance; revision `2 -> 3`; Vault count restored to `2` |
| Re-run after B login | two distinct identities, two Sessions, distinct Provider session IDs, concurrent account-bound Usage |
| Final cleanup | both Sessions exited; no materialized auth files; two healthy Vault-backed Credentials retained; Daemon log `0` bytes and Daemon stopped |

Machine-readable sanitized evidence is in
`2026-07-16-distinct-account-homes.json`.

## Limits and fallback

- The test establishes exact-version Linux compatibility, not an undocumented
  general Codex contract. Unknown versions and identity ambiguity must fail
  closed.
- macOS only has the earlier exact-schema/empty-Home evidence; it does not yet
  have a two-distinct-identity run. Windows has no real Codex auth run.
- No passive soak was run. The result covers two complete logout/re-login
  cycles and concurrent reads, not long-duration refresh behavior.
- OAuth browser callback binding still requires an operator-owned browser
  interaction. MultiAgentDesk must not automate MFA, CAPTCHA, account choice,
  or copy browser cookies.
- A Credential cannot be logged out while its Session is active. Stop the
  selected Account's Sessions first; never stop another Account implicitly.
- Fallback: use one explicitly selected target-local managed Home, require an
  official interactive login, disable automatic account rotation, and
  quarantine/re-login on any revision or identity ambiguity.
