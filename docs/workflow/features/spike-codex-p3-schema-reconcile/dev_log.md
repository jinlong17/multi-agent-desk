# Spike log: Codex P3 schema reconciliation

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-codex-p3-schema-reconcile` |
| Title | `Codex P3 schema reconciliation` |
| Owner Module | `provider` |
| Impacted Modules | `core, security, project-system` |
| Hypothesis | `Codex CLI schema bundles can be keyed by a deterministic canonical JSON fingerprint, and the exact 0.144.2 initialize/account/Approval method shapes can be reproduced without reading credentials` |
| Time-box | `2 hours` |
| Current Phase | `DECISION` |
| Status | `GATE_RESOLVED` |
| Executor | `Codex (GPT-5) as feature-plan` |
| Updated | `2026-07-15 17:24 PDT` |
| Suggested Next | `feature-build P3 protocol correction` |
| Security Gate | `resolved — accepted only for canonical fingerprinting and empty-home protocol evidence; credentialed P3 remains open` |
| Evidence Path | `docs/spikes/codex/p3-schema-reconcile.json` |
| Decision Record | `docs/PROVIDER_COMPATIBILITY.md`; `docs/adr/0014-codex-app-server-single-writer-auth.md` |

## Success and failure criteria

- Supported when: two schema generations from the same binary and an exact npm
  version produce one stable canonical fingerprint; initialize and unauthenticated
  Account calls reproduce with bounded sanitized shapes; actual Approval method
  names are extracted from the exact schema.
- Falsified when: canonicalized JSON still changes between generations, a live
  response requires reading credential material, or the exact schema cannot be
  reduced to an allowlisted method set.

## Environment

| Field | Value |
|---|---|
| Tool + version | ChatGPT bundled `codex-cli 0.144.2`; npm `@openai/codex@0.142.5`, `0.143.0`, `0.144.2` |
| OS | macOS 26.5.2 arm64 |
| Auth mode | empty disposable `CODEX_HOME`; no login completion and no credential file read |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-15 17:08 PDT | generated two schema bundles from the bundled `0.144.2` binary and compared all 267 files | raw bundle hashes differed because one aggregate JSON file emitted map definitions in nondeterministic order; semantic content was equal | `docs/spikes/codex/p3-schema-reconcile.json` |
| 2026-07-15 17:09 PDT | canonicalized every JSON file with recursively sorted keys, then hashed `relative-path + NUL + canonical JSON` | both bundled runs and exact npm `0.144.2` produced `a1a35476587fe9bbfbe9e291b5200b8bc541df8c00241fe578d285ff26996e1c` | `docs/spikes/codex/p3-schema-reconcile.json` |
| 2026-07-15 17:10 PDT | generated exact npm schemas for `0.142.5`, `0.143.0`, `0.144.2` | stable canonical fingerprints recorded for all three 267-file bundles | `docs/spikes/codex/p3-schema-reconcile.json` |
| 2026-07-15 17:11 PDT | launched bundled app-server with an empty isolated `CODEX_HOME`; sent exact initialize, initialized, Account read, Rate Limits, and Usage requests | initialize succeeded with current result shape; Account read returned `account:null` and `requiresOpenaiAuth:true`; authenticated reads returned bounded auth-required errors; no secret was present | `docs/spikes/codex/p3-schema-reconcile.json` |
| 2026-07-15 17:12 PDT | extracted ClientRequest and ServerRequest method enums from the exact schema | current initialize uses `clientInfo`; Approvals are server requests such as `item/commandExecution/requestApproval`, not synthetic `approval/request`/`approval/respond` methods | `docs/spikes/codex/p3-schema-reconcile.json` |

## Result, limitations, and fallback

The hypothesis is supported for deterministic schema identification and the
empty-home protocol surface. Raw-byte hashing of generated JSON is invalid
because aggregate definition order changes across runs. Canonical JSON hashing
is stable across bundled and exact npm `0.144.2` builds and produces distinct
stable rows for the three recorded versions.

The current adapter's synthetic initialize/result and Approval method contracts
do not match the exact `0.144.2` schema. Production capability must remain
disabled until feature-build applies this evidence and deterministic tests pass.
Credentialed Account/Usage/Approval/turn execution and the Linux second-CLI
exit remain outside this macOS empty-home Spike.

Fallback: keep Codex Session support disabled; retain Provider diagnostics and
Fake Provider functionality; never accept raw schema fingerprints or guessed
Approval method aliases.

## Risks and Blockers

- Credentialed Linux live Session, second CLI attach/control, real Approval,
  Usage, stop, and resume evidence remain required for P3 verification.
- Canonical JSON must be implemented without accepting non-JSON or symlinked
  schema files, oversized inputs, duplicate keys, or unknown versions.
- Security review must confirm that the empty-home evidence and revised method
  allowlist do not weaken credential or Approval boundaries.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-15 17:03 PDT | Codex (GPT-5) as feature-plan | Classified the P3 blocker as `provider`, froze a two-hour falsifiable schema/protocol hypothesis, required an empty isolated auth home, and opened the Security Gate | this file | `SPIKE_READY`; no production code or compatibility claim changed | `provider-spike` |
| 2026-07-15 17:12 PDT | Codex (GPT-5) as provider-spike | Reproduced nondeterministic raw schema hashes, proved stable canonical fingerprints across bundled/npm 0.144.2 and all three exact versions, replayed empty-home initialize/Account behavior, and extracted actual Approval method names | `docs/spikes/codex/p3-schema-reconcile.json`; this file | `EVIDENCE_READY`; synthetic initialize/Approval contracts are falsified; live Linux credentialed exit remains open | `security-review` |
| 2026-07-15 17:18 PDT | Codex (GPT-5) as security-review | Reviewed canonicalization fail-closed requirements, empty-home evidence handling, server-request Approval semantics, single-writer/CAS boundaries, audit safety, and residual host/Provider risks | `docs/reviews/spike-codex-p3-schema-reconcile/2026-07-15-security-review.md`; this file | `ACCEPTED` for the narrow schema/protocol decision; credentialed Linux Session/Approval/Usage evidence remains open | `feature-plan decision` |
| 2026-07-15 17:24 PDT | Codex (GPT-5) as feature-plan | Recorded canonical JSON fingerprint rows, deprecated raw generated-byte hashes as compatibility keys, and froze exact initialize plus server-request Approval semantics while retaining the credentialed Linux P3 gate | `docs/PROVIDER_COMPATIBILITY.md`; `docs/adr/0014-codex-app-server-single-writer-auth.md`; this file | `GATE_RESOLVED`; production adapter correction is authorized, live P3 support remains pending | `feature-build P3 protocol correction` |

## Handoff

**Target**: `spike-codex-p3-schema-reconcile`
**Completed**: `feature-plan`
**Status**: `GATE_RESOLVED`
**Summary**: `Canonical JSON fingerprints and exact initialize/server-request Approval semantics are now the compatibility decision; raw generated-byte hashes are historical only.`
**Files Written**: `docs/PROVIDER_COMPATIBILITY.md`; `docs/adr/0014-codex-app-server-single-writer-auth.md`; this file.
**Evidence**: `docs/spikes/codex/p3-schema-reconcile.json`; accepted security review; three exact-version canonical rows.
**Blockers**: `Production adapter correction and credentialed Linux P3 live exit remain required.`

### Next Step

Run `feature-build` P3 protocol correction for `phase2-codex-vertical-slice`.
