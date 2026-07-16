# Feature Verify: Codex Vertical Slice P1

## Verdict

`VERIFIED` for the approved P1 Contract and Fixtures phase. Discovery,
version/schema gating, bounded persistent JSONL framing, initialize handshake,
sanitized Account/Usage/Approval mapping, exact-version replay fixtures, and
diagnostic IPC/CLI behavior pass. No live Codex support claim is made because a
credentialed Codex binary and generated live schema were not present in this
verification environment.

## Acceptance verification

| Area | Evidence | Result |
|---|---|---|
| Binary discovery | Discovery test uses an absolute executable path, bounded `--version` output, timeout, and direct `exec.Cmd` argument vectors; missing binaries fail | PASS |
| Exact compatibility gate | Registry covers `0.142.5`, `0.143.0`, and `0.144.2` with the Spike fingerprints; mismatched fingerprints and unknown versions return `provider_version_unsupported` | PASS |
| Schema probe boundary | `Probe` can use an injected fingerprint, a generated schema directory, or a bounded `app-server generate-json-schema` invocation; no shell interpolation | PASS |
| JSONL framing | Persistent `FrameReader`, max-frame bounds, duplicate-key rejection, trailing-value rejection, strict object decoding, and partial stream handling are tested | PASS |
| Initialize handshake | `initialize` request/response ID matching, method allowlist capture, and `initialized` notification are tested over `net.Pipe` | PASS |
| Sanitized mapping | Account excludes email/raw identity; Usage allows only recorded summary keys; Approval maps bounded IDs/summaries and stores only a payload digest; unknown fields fail closed | PASS |
| Fixture replay | Three synthetic, secret-free JSONL fixtures replay through Account/Usage/Approval mapping | PASS |
| Diagnostics IPC/CLI | Temporary daemon run of `provider describe` and `provider health` returns explicit `unsupported` when no binary is discovered | PASS |
| Regression/platform baseline | Full Go tests, vet, race checks, macOS/Linux/Windows CLI builds, workflow/dashboard/CI/link/license checks | PASS |

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

The temporary daemon/CLI run used a disposable `/tmp` Device root. Both
`provider describe` and `provider health` completed through authenticated local
IPC and reported `status: unsupported` with reason `codex binary was not
discovered`; no false Provider support was exposed.

## Findings and carried gates

No P1 implementation failure was found. The following evidence remains
intentionally deferred:

1. A real Codex executable was not available in this environment, so the
   generated-schema probe was not run against a live binary. The exact matrix
   rows remain based on the sanitized Phase 0.5 evidence and fixtures.
2. Credential materialization, isolated `CODEX_HOME`, canonical writer/CAS,
   and crash quarantine are P2 and remain unimplemented.
3. Real Linux Session/event/Usage/Approval dispatch and macOS/Windows live
   compatibility remain later phases.

These are phase boundaries, not P1 failures. The adapter remains fail-closed
until exact live schema evidence and the later Security Gate are available.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P1`
**Verdict**: `VERIFIED`
**Summary**: `P1 discovery, exact compatibility gating, persistent bounded JSONL framing, initialize handshake, sanitized schema mapping, three-version fixture replay, and authenticated diagnostics are verified.`
**Evidence**: `go test ./...`; `go vet ./...`; provider/core race tests; macOS/Linux/Windows builds; fixture and protocol tests; temporary diagnostics IPC/CLI; `npm run project:verify`; `npm run ci:verify`; `git diff --check` — all passed.
**Findings**: `No P1 failures. Live binary/schema probing, credential materialization, Security review, and real Provider sessions remain later gates.`
**Blockers**: `None for P1 verification; no live Codex support claim is made without a real binary and exact generated schema evidence.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice` P2.
