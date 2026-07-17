# P1 as built: Codex explicit selector contracts

Date: 2026-07-16

Branch: `codex/provider/codex-multi-account-selector`

Scope: approved P1 only

## Result

P1 now provides the non-bypass application contract required before a live
multi-account runtime can be connected:

- migration 7 preserves v6 enrollment data, adds the owner-bound
  `awaiting_confirmation` state with Account/Profile/Credential revision pins,
  and creates persistent random `session_start_previews` rows;
- `auth.complete` validates official staged login but does not seal it;
  `auth.confirm` requires an explicit canonical public alias, rechecks owner,
  expiry, tuple, revisions, binary fingerprint and staged auth, then atomically
  seals the Vault item and binds the Profile;
- `sessions.preview` resolves one public Codex alias, requires an unlocked
  healthy sealed Credential, selects the latest account-bound Usage snapshot,
  performs the exact compatibility preflight, and persists a ten-minute
  client-bound preview;
- `session.start` rejects every Codex raw-ID or unconfirmed request, rechecks
  preflight fingerprints, and atomically consumes the preview with insertion
  of one immutable starting Session; exact lost-response replay returns that
  Session while forged, cross-client, expired, drifted, or differently replayed
  requests fail closed;
- public CLI auth/status/logout use `--profile`; human login/run require the
  operator to type the shown canonical alias. JSON auth completion stops at
  confirmation-required and never prompts or auto-confirms.

## Phase boundary

P1 does not spawn the live exact-Linux Provider runtime. After a successful
synthetic reservation, the application records a deterministic failed Session
with `provider_capability_unavailable`; P2 replaces that receipt with the
reviewed `StartReserved` runtime handoff. This preserves truthful Session state
and prevents a reserved Session from remaining active without a process.

Only Linux amd64 Codex CLI `0.144.2` reaches the default selector preflight.
macOS and Windows remain typed unsupported/pending gates, and no support claim
was broadened.

## Security properties

- Preview and enrollment rows store opaque internal IDs, revisions,
  fingerprints, digests, and timestamps only.
- No email, organization, upstream subject, token, auth JSON, callback URL,
  code, cookie, terminal/model content, or Usage numeric value is added.
- Login confirmation and Session confirmation are separate owner-bound actions.
- Preview consumption and Session insertion share one SQLite transaction.
- Revocation reservation, Vault revision CAS, and existing materialization
  single-writer rules remain authoritative.

## Verification evidence

- Full Go tests and `go vet ./...`: pass.
- `go test -count=1 -race ./...`: pass.
- Preview race repeated ten times: exactly one Session per preview.
- Darwin arm64, Linux amd64, and Windows amd64 command builds: pass.
- Web TypeScript checks/build and Desktop Rust fmt/check: pass.
- v6-to-v7 enrollment preservation, restart/reopen, forged/cross-client,
  expiry, revision drift, exact replay, different replay, raw-ID rejection,
  and unconfirmed Vault negative tests: pass.

## Deferred to P2/P3

- P2: `Runtime.StartReserved`, real A/B selector execution, official Usage,
  target-only logout/re-login, and post-reservation binary-race evidence.
- P3: macOS/Windows typed UX/docs closure, full platform evidence, final
  independent Security Review, and Ship readiness.
