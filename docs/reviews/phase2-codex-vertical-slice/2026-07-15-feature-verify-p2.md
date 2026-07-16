# Feature Verify: Codex Vertical Slice P2

## Verdict

`VERIFIED` for the approved P2 Auth Home and Writer phase. The typed
Vault-to-Codex materialization boundary, isolated private auth home, canonical
writer lock/lease, digest and structure validation, revisioned CAS, and crash
quarantine behavior pass the deterministic acceptance suite. No live Codex
login or Provider Session support claim is made.

## Acceptance verification

| Area | Evidence | Result |
|---|---|---|
| Typed materialization boundary | `CredentialSource` writes only into a manager-owned private staging home; no raw credential bytes are accepted by manager or IPC APIs | PASS |
| Isolated auth home | per-CredentialInstance home, restrictive directory/file modes, exact `auth.json` + manifest allowlist, path/symlink rejection | PASS |
| Vault gate | locked Vault rejects acquisition with `vault_locked` before materialization | PASS |
| Canonical writer | atomic filesystem writer lock rejects a second writer with `credential_writer_conflict`; profiles share the same home | PASS |
| Lease | owner-bound lease refresh checks lock ownership, revision, expiry, and bounded TTL | PASS |
| Digest/structure validation | bounded JSON object, duplicate/trailing JSON rejection, digest and size checks, unexpected-file quarantine | PASS |
| Revisioned CAS | changed auth digest commits only through expected-revision CAS and increments `credential_revision`; stale updates fail closed | PASS |
| Crash recovery | stale lock/staging residue and malformed homes are moved to private quarantine; a new writer requires a clean/re-login path | PASS |
| Secret safety | persisted Store fields are digest/revision metadata only; implementation diagnostics do not log or return auth contents | PASS |
| Regression/platform baseline | full Go tests, vet, race checks, macOS/Linux/Windows builds, workflow/dashboard/CI/link/license checks | PASS |

## Commands and results

```text
go test ./...                                                        PASS
go vet ./...                                                        PASS
go test -race ./internal/domain ./internal/storage ./internal/app \
  ./internal/runtime ./internal/providers/codex                        PASS
go build ./cmd/multidesk                                             PASS
GOOS=linux GOARCH=amd64 go build ./cmd/multidesk                      PASS
GOOS=windows GOARCH=amd64 go build ./cmd/multidesk                    PASS
npm run project:verify                                                PASS
npm run ci:verify                                                     PASS
git diff --check                                                      PASS
```

The provider-specific contract tests cover restrictive modes, locked Vault,
second-writer rejection, lease refresh, duplicate/invalid JSON, digest
rotation, monotonic CAS, stale revision, path traversal, unexpected residue,
and stale-lock quarantine. The tests use synthetic in-memory fixture content
only; no credentialed Codex binary or interactive login was available.

## Findings and carried gates

No P2 implementation failure was found. The following evidence remains
intentionally deferred:

1. Official interactive login and real `CODEX_HOME` materialization require a
   credentialed environment and are not simulated by these tests.
2. The exact Linux Codex app-server version/schema and real Session/event flow
   remain P3 gates; P2 does not authorize a live Provider claim.
3. Security review remains mandatory before Ship, including host compromise,
   provider refresh semantics, Approval mutation, and crash-boundary review.
4. macOS live smoke and Windows real Codex compatibility remain P4 evidence
   gates; Windows is still build/protocol baseline only.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P2`
**Verdict**: `VERIFIED`
**Summary**: `P2 credential materialization, canonical writer lease, digest/structure validation, revisioned CAS, quarantine, and secret-safety contracts pass.`
**Evidence**: `go test ./...`; `go vet ./...`; provider/core race tests; macOS/Linux/Windows builds; materialization contract tests; `npm run project:verify`; `npm run ci:verify`; `git diff --check` — all passed.
**Findings**: `No P2 failures. Live login, exact Linux schema/session, macOS smoke, Windows real Codex, and Security review remain later gates.`
**Blockers**: `None for P2 verification; no live Codex support claim is made.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice` P3.
