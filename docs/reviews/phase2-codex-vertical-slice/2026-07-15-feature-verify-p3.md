# Feature Verify: Codex Vertical Slice P3

## Verdict

`BLOCKED` for the P3 Real Linux Session phase. The deterministic
ProviderSession handshake, notification/event mapping, sanitized Usage and
Approval boundaries, stop capability gate, and typed resume fallback pass.
The release-blocking live exit cannot be performed safely because no pinned
Linux Codex environment or credentialed test Account is available, and the
available macOS Codex schema fingerprint does not match the recorded matrix
row for `0.144.2`.

## Acceptance verification

| Area | Evidence | Result |
|---|---|---|
| Pinned handshake | `ProviderSession.Start` performs initialize/initialized and rejects a changed server version | PASS (deterministic) |
| Event mapping | bounded JSON-RPC notifications map only allowlisted Account/Usage/Approval methods; unknown/request frames fail closed | PASS (deterministic) |
| Account/Usage | calls are capability-gated and decode only sanitized projections with source/version/freshness | PASS (deterministic) |
| Approval | structured response validates bounded ID/decision and remains disabled unless exact schema capability is present | PASS (deterministic) |
| Stop/kill | stop uses an explicit capability gate; unsupported app-server stop is typed, never guessed | PASS (contract) |
| Resume | Provider continuation always returns `provider_resume_unsupported` without mutation until exact evidence exists | PASS (contract) |
| Regression/platform baseline | full Go tests, vet, race checks, macOS/Linux/Windows builds, workflow/dashboard/CI checks | PASS |
| Exact macOS schema evidence | `/Applications/ChatGPT.app/Contents/Resources/codex --version` = `codex-cli 0.144.2`; generated schema SHA-256 = `3a013304c88d1e8a3b37ef541552f2eccd3df9b14eeaf4d1300fa722fb104156`, not the recorded row fingerprint | BLOCKING MISMATCH |
| Required Linux live exit | no pinned Linux Codex binary, isolated credentialed `CODEX_HOME`, or second local CLI environment is present | BLOCKED |

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
/Applications/ChatGPT.app/Contents/Resources/codex --version          0.144.2
codex app-server generate-json-schema --out <disposable dir>          PASS
generated schema fingerprint                                            3a013304...fb104156
```

The schema output was generated in a disposable temporary directory and no
credential, auth file, or raw Provider response was retained. The fingerprint
algorithm is the repository's path-plus-NUL-plus-bytes SHA-256 contract.

## Blocking findings and clearing role

1. `provider` must run a reproducible schema probe for the exact Linux binary,
   reconcile the macOS `0.144.2` mismatch through a new reviewed compatibility
   evidence row (or keep it unsupported), and capture sanitized initialize/
   Account/Usage/Approval/stop/restart fixtures.
2. A credentialed pinned Linux environment must run the real Phase 2 exit with
   one canonical writer, a second local CLI attach/control path, and the
   frozen resume contract. The clearing role is `provider` feature-build plus
   an independent feature-verify in that environment.
3. Security review cannot accept the feature while this live gate is open;
   Approval mutation and credential refresh remain unverified.

These are reproducible external/provider evidence blockers, not deterministic
implementation failures. P4 and Ship must not start until P3 is verified.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P3`
**Verdict**: `BLOCKED`
**Summary**: `Deterministic ProviderSession contracts pass, but the required Linux live exit is unavailable and the available macOS 0.144.2 schema mismatches the recorded fingerprint.`
**Evidence**: `go test ./...`; `go vet ./...`; provider/core race tests; macOS/Linux/Windows builds; workflow/CI checks; direct Codex version probe; disposable schema generation and fingerprint — deterministic checks pass, live gate blocked.
**Findings**: `Recorded schema evidence is stale or version/build-specific; no credentialed pinned Linux environment or second CLI live exit is available.`
**Blockers**: `Pinned Linux Codex + isolated credentialed CODEX_HOME + second CLI + reviewed exact schema row are required. Clearing role: provider feature-build/feature-verify.`

### Next Step

Run `feature-plan`/`provider-spike` to reconcile the exact schema evidence, then
rerun `feature-build` and `feature-verify` for P3 in a credentialed Linux
environment.
