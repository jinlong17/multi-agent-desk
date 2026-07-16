# Test strategy: Codex explicit multi-account selector

## Automated acceptance matrix

| Area | Scenario | Expected result |
|---|---|---|
| Selector | canonical/case/Unicode/path/flag/shell corpus; ambiguous and internal aliases | exactly one public Profile or typed failure; no command evaluation |
| Preview | enabled valid tuple with 0/1/many Usage rows | latest account-bound snapshot selected; bounded labels/freshness; no PII |
| TOCTOU | mutate alias, Account/Profile revision, Credential revision/status, Workspace, Usage binding or compatibility after preview | pre-reservation failure creates zero Session/materialization/process; post-reservation binary race records failed Session and no credential commit |
| Preview authority | forged/random ID, cross-client use, restart/expiry, two requests racing one preview, same-request lost-response replay | only daemon-issued owner-bound row works; one Session; exact replay returns it |
| Confirmation | missing/false/replayed/expired/wrong-client/wrong-selector confirmation | fail closed; no Credential seal or Session start |
| Enrollment | official-login success then confirm/cancel/timeout/restart | only confirmed exact tuple seals; staging removed in every terminal path |
| Logout race | active/starting/stopping Session; start after revocation reservation | logout blocked or start denied; no cross-Credential mutation |
| Writer | concurrent A/B, same-Credential double start, stale lock/revision | A/B independent; one writer per Credential; ambiguity quarantined |
| Compatibility | Linux exact, macOS pending, Windows unsupported, unknown CLI/schema | only exact Linux path launches; others return typed capability state |
| Redaction | DB/audit/errors/dashboard/fixtures/debug output scan | no Provider identity, auth JSON, token, URL/code, Cookie, transcript/content |
| Existing behavior | Phase 2 harness seeded with public alias and preview; direct CLI/RPC raw-ID attempt; P1 registry/Fake path | harness remains deterministic; raw-ID start fails before Session; Fake unchanged |

## Unit and property tests

- Canonical selector parsing, bounded labels, preview record validation/expiry,
  random ID validation, and client/tuple/revision binding.
- Confirmation struct validation and error taxonomy.
- Enrollment state machine adds `awaiting_confirmation` without allowing direct
  `begun -> succeeded`; terminal and idempotent transitions remain valid.
- Alias confirmation digest uses only internal selector/enrollment identity and
  reveals no label or Provider value.
- Account/Profile/Credential/Device/Workspace/Provider/status/revision equality.
- Compatibility mapping for exact Linux `0.144.2`, macOS pending, Windows and
  unknown versions.
- Safe audit projection and exhaustive secret/PII field blacklist.

## Storage and migration tests

1. Migrate schema v6 fixtures containing Phase 2 Vault/Credential/Session/
   Approval/Usage rows and P1 aliases to v7; preserve every row and foreign key.
2. Reopen/restart twice; migration checksum/version and enrollment states remain
   deterministic.
3. Inject invalid/partial confirmation fields and prove migration/transaction
   rollback.
4. Race alias/profile update, credential refresh, confirmation, Session insert,
   and revocation reservation under the single SQLite writer.
5. Expire/cancel/recover every enrollment state and prove no private staging
   directory or unconfirmed Vault item survives.
6. Persist preview rows across restart; reject forged/cross-client/expired IDs;
   atomically resolve two-start races; return the same Session for exact
   idempotent replay; clean expired/consumed rows after bounded retention.

## Integration tests

Use authenticated native IPC and synthetic Provider fixtures:

1. Create two public Codex Accounts/Profiles/Credentials and one internal Fake
   Profile; preview by aliases and reject internal/wrong-provider tuples.
2. Preview A, change every bound field one at a time, then start; assert no
   Session/process/materialization on every mismatch.
3. Complete synthetic official login, receive confirmation-required, confirm
   wrong alias/client/expired attempt, then exact alias; verify one Vault item
   and revision advance.
4. Run A/B concurrently with separate synthetic app-server IDs and Usage;
   verify immutable Session/account bindings and no auto-rotation after quota/
   auth/provider failures.
5. Stop/logout/re-login B while A stays running; compare safe digests and rows,
   then restart Daemon and repeat.
6. Attempt the legacy raw-ID-only CLI and direct authenticated RPC form with a
   client that has `session.start`; prove `identity_confirmation_required` and
   zero Session/process/materialization.
7. Replace/change the accepted Provider binary before preview consumption and
   after Session reservation. Verify respectively zero Session and one failed
   Session, with no Vault revision or retained materialization in either case.

## Live Linux acceptance

On the operator-approved Linux target with exact CLI `0.144.2`:

1. Reuse the two operator-owned identities from the accepted sanitized Spike;
   do not persist identity values or hidden requests.
2. Run official login only if a retained Credential is invalid. Type the exact
   internal alias at the new confirmation gate.
3. Preview and explicitly start A/B through selectors, verify distinct Provider
   session IDs and concurrent account-bound official Usage.
4. Prove preview drift and active target logout fail closed.
5. Stop/logout/re-login only B, prove A auth/session/Usage unchanged, then
   cleanly stop both Sessions/Daemon and remove materialized auth files.

## Cross-platform verification

- `go test -count=1 ./...`
- `go vet ./...`
- `go test -count=1 -race ./...`
- build commands for Darwin arm64, Linux amd64, and Windows amd64
- storage migration tests compiled/run on available three-platform CI
- macOS exact-schema/empty-home selector negative: pending identity acceptance,
  no Provider start
- Windows selector negative: unsupported, no Provider start
- Web TypeScript and Desktop/Rust scaffold checks remain green
- workflow, dashboard, Actions, CODEOWNERS, fixtures, links, licenses, layout,
  format, and `git diff --check`

## Rollback tests

- Disable the exact compatibility capability and prove new previews report
  unavailable while existing Sessions remain pinned and controllable.
- Restart with unconfirmed enrollment or stale preview; both expire/fail closed
  without deleting existing Vault items.
- Roll back the binary without reversing migration 7; older code must refuse a
  future schema rather than open it unsafely.
