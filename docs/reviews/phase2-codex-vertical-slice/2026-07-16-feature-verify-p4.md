# Feature Verification: Codex Vertical Slice P4

## Verdict

`READY_TO_SHIP`

The final P4 platform, compatibility, documentation, and Security Review
handoff acceptance criteria pass. Exact macOS Codex `0.144.2` live smoke,
current Windows-target compilation, the retained native Windows CI receipt,
the complete Go/race/build matrix, repository governance, and the documented
support limits all agree. No P4 finding remains. The Security Gate is still
open, so `security-review` is required before `ship`.

## Scope and authority

- Owner: `provider`; impacts: `core`, `security`, `project-system`
- Branch: `codex/provider/phase2-codex-vertical-slice`
- Re-read the P4 test contract, compatibility matrix, threat model, ADR 0014,
  P3B verification, Security Review handoff, and the complete current diff
- Wrote no implementation, plan, compatibility, dashboard, generated, Git, or
  remote configuration state during verification

## Fresh commands and results

| Check | Result |
|---|---|
| exact official macOS arm64 Codex `0.144.2` version plus `TestConfiguredCodexBinaryCanonicalSchemaProbe` and `TestConfiguredCodexBinaryEmptyHomeHandshake` | pass; both live tests pass |
| `go test -count=1 ./...` | pass |
| `go vet ./...` | pass |
| isolated `go test -count=1 -race ./...` | pass |
| macOS arm64, Linux amd64, and Windows amd64 command builds | pass |
| Windows amd64 Provider, runtime, app, and device test-binary compilation | pass |
| direct workflow/dashboard, Actions, CODEOWNERS, gate-fixture, local-link, and license verification | pass; agents=`10`, skills=`3`, docs=`17`, phases=`9`, links=`222`, pnpm groups=`5`, Cargo packages=`418` |
| `git diff --check`; secret-like diff scan | clean/pass |
| `gh run view 29469271422` | completed/success at `31e501dc12585648e8a1d97178e7529682e893be`; Windows job `87528995056` and all other jobs successful |

The first race attempt was deliberately launched concurrently with live schema
generation, full tests, three platform builds, and four Windows test compiles.
Under that artificial host contention, the one-second Unix shell-fixture probe
timed out. The required race command was immediately rerun in isolation and the
entire repository passed. This is an orchestration note, not a product or P4
failure; no failed assertion or race report occurred.

## Acceptance and compatibility review

- Linux x86_64 Codex `0.144.2` remains the only real credentialed vertical-slice
  support claim, backed by the independent P3B live verifier.
- Exact macOS arm64 `0.144.2` canonical-schema and empty-home handshake smoke
  pass. Bundled `0.144.5` remains outside the allowlist and unsupported.
- Windows evidence is explicitly limited to current build/protocol compilation
  and the unchanged successful native Phase 1 IPC CI receipt. No real Windows
  Codex support is claimed.
- README, compatibility matrix, threat model, ADR 0014, test receipt, and the
  Security Review input package use the same exact version/platform boundaries.
- Secret-like diff scanning is clean. No token, OAuth credential, email, or raw
  Provider payload was added to repository evidence.
- The Security handoff preserves explicit residual risks: runtime-readable
  materialized credentials, unsupported multi-writer/device-auth/continuation,
  and no remote grant, packaging, release, deployment, or Ship authorization.

## Findings

None for P4.

The final implementation Security Gate remains open by design and must now be
decided by `security-review`.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P4`
**Verdict**: `READY_TO_SHIP`
**Summary**: `The final platform matrix, exact macOS 0.144.2 smoke, Windows build/protocol baseline, compatibility documentation, and Security Review handoff all pass with truthful support limits.`
**Evidence**: `fresh exact macOS live smoke; full Go/vet/isolated-race matrix; macOS/Linux/Windows builds; Windows test compilation; direct governance/CI checks; clean diff/secret scan; successful native Windows CI receipt.`
**Findings**: `none.`
**Blockers**: `none for feature verification; the mandatory Security Gate remains open.`

### Next Step

Run `security-review` for `phase2-codex-vertical-slice`.
