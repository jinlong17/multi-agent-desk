# Security review: Codex auth, usage, and refresh boundary

- Date: 2026-07-14
- Role: `security-review`
- Target: `spike-codex-auth-refresh`
- Evidence commit: `25d8f1b`
- Verdict: **ACCEPTED**

## Scope

Reviewed the app-server schema/account evidence, file credential observations,
isolated device-auth initiation, sanitized macOS/Linux short-run artifact, the
operator cancellation of the 48-hour criterion, and the selected canonical
single-refresh-writer fallback. This verdict accepts a constrained integration
boundary; it does not accept multi-writer refresh, completed headless login, or
48-hour stability.

Evidence reviewed:

- `docs/spikes/codex/2026-07-14-auth-refresh-spike.md`
- `docs/spikes/codex/app-server-account-matrix.json`
- `docs/spikes/codex/two-device-short-run.json`
- `docs/spikes/codex/run_two_device_soak.py`
- `docs/THREAT_MODEL.md` T-03 through T-06, T-09, T-12, and T-17
- `docs/IMPLEMENTATION_PLAN.md` credential materialization, CAS, Provider, and
  audit boundaries
- `CLAUDE.md` credential single-writer invariant

## Verdict rationale

**ACCEPTED.** The evidence is secret-safe and sufficiently honest to select the
conservative production boundary. It proves the listed app-server reads and
managed refresh behavior on the exact tested versions, observes restrictive
`0600` auth-file modes on macOS and Linux, and records only sanitized booleans.
It explicitly rejects the unsupported production multi-writer claim.

The accepted design requires one canonical refresh writer per
`CredentialInstance`, one exclusive lease, monotonic `credentialRevision`, and
revisioned compare-and-swap back to the local Vault. A second writer must be
rejected. Provider-side mutation must be imported transactionally or
quarantined on ambiguity. Device-auth initiation is experimental; interactive
official login is the required fallback until completed isolated login has its
own evidence.

## Findings

- P0: none.
- P1: none, provided the single-writer boundary is treated as mandatory rather
  than a performance preference.
- P2: the short run demonstrates compatibility only. It cannot be used in UI,
  documentation, tests, or release notes to imply multi-device refresh safety
  or long-duration stability.

## Required implementation obligations

1. Keep Provider plaintext and refreshable credential state on an authorized
   device. The Control Plane may carry only encrypted grants and non-secret
   metadata; it is never the canonical credential writer.
2. Acquire one exclusive materialization lease before creating or refreshing a
   runtime home. Reject or quarantine a competing writer.
3. Detect Provider auth-file mutation by content digest, validate the expected
   structure, and commit it with monotonic revision/CAS. Treat `mtime` only as
   a change hint and never overwrite a newer Vault revision.
4. Use restrictive permissions and an isolated `CODEX_HOME`; avoid shell
   interpolation, logs, crash attachments, backups, and telemetry containing
   auth-file content, account identity, usage values, authorization URLs, or
   device codes.
5. Pin the Provider binary path/version and the selected account/profile to the
   session. Never auto-rotate accounts or silently switch credentials after a
   rate-limit or refresh error.
6. On ambiguous crash recovery, stop new Provider starts for that credential,
   quarantine the materialized home, and require reconciliation or official
   re-login. Never guess which refresh token is newest.
7. Treat official interactive login as the supported fallback. A device-auth
   prompt may be exposed only as an explicit experimental operator action until
   successful isolated completion is separately verified.
8. Revocation must block future materialization and instruct the user to revoke
   the Provider-side session when required; it cannot promise erasure from a
   machine that already received plaintext.

## Residual risk

The Codex process, same-user processes, host administrator/root, malware,
backup tools, and crash collectors may read or copy a materialized `auth.json`
while it exists. Codex may mutate its credential format or refresh semantics in
a future version. A compromised authorized device can retain credentials and
plaintext after MultiAgentDesk revocation. File mode `0600`, a Vault, and CAS
reduce accidental exposure and stale overwrite; they do not protect against a
compromised runtime or administrator. Interactive re-login may still be needed
after an ambiguous refresh or upstream behavior change.

These risks are accepted only for the Spike decision boundary. Production
implementation must satisfy the obligations above and pass its own security
and release verification.
