# Feature verification v2: Codex explicit multi-account selector P3

- Date: 2026-07-20
- Executor: `feature-verify / P3 correction`
- Build commits: `25328b1`, correction `ec73d1c`
- Prior verdict: `BLOCKED` at `4ba0090`
- Verdict: `READY_TO_SHIP`

## Prior finding closure

The prior P1 finding is closed. One production platform-gate helper now covers
all five selector boundary points:

1. `auth.begin` before enrollment/Credential/staging creation;
2. `auth.complete` before validator/read/seal, with terminal cleanup on drift;
3. `auth.confirm` before validator/read/Vault seal, with terminal cleanup;
4. `sessions.preview` before schema/preview/Session/materialization;
5. `Runtime.StartReserved` before fingerprint/materialization/spawn.

The injected service hook is not an IPC field and is used only by application
tests to model accepted Linux and pending-platform transitions. Production
defaults to `codex.RequireSelectorPlatform`.

Direct regressions prove pending `auth.begin` creates no Credential or staging
root; complete/confirm drift produces a failed enrollment with its unknown
Credential reference removed and staging deleted; the accepted Linux test path
continues through seal, logout, and re-login. Code order independently confirms
the gate precedes the validator and Vault seal.

## Independent verification evidence

- `git show --check --stat ec73d1c` and correction diff review — pass.
- `go test -count=1 ./...`, `go vet ./...`, and
  `go test -race -count=1 ./...` — pass.
- Auth enrollment and selector platform/reserved-runtime suites with
  `-race -count=10` — pass.
- Darwin arm64, Linux amd64, Windows amd64 builds — pass with SHA-256
  `2540b1...b0d70`, `6929ff...74804`, `0868ae...9becc`; compile evidence only
  for macOS/Windows.
- pnpm `10.23.0` Web TypeScript/build and Desktop Rust fmt/check — pass.
- Actions, CODEOWNERS, CI fixtures, links, licenses, workflow, dashboard,
  project structural verification, and diff integrity — pass. One first CI
  invocation lacked the local `npm` wrapper; the identical matrix passed after
  restoring the pinned pnpm-10 wrapper and is not a product failure.
- P2's stopped Linux A/B acceptance, distinct Usage, scoped logout/re-login,
  zero materialized auth files, and daemon cleanup evidence remains unchanged.

## Platform verdict

- Linux amd64 plus exact Codex `0.144.2`: live selector acceptance verified.
- macOS: schema/empty-home evidence only;
  `schema_compatible_identity_acceptance_pending` before selector enrollment
  artifacts or runtime work.
- Windows: compile/protocol evidence only; `provider_platform_unsupported`
  before selector enrollment artifacts or runtime work.
- No default Account/Profile, automatic rotation, raw-ID public bypass, or
  cross-platform fallback exists.

## Verdict

The final approved feature phase is `READY_TO_SHIP`. No functional verification
finding remains. The feature is not yet shippable because its Security Gate is
open; only `security-review` may decide that gate. This verdict does not
authorize push, merge, tag, package, release, or deployment.

## Findings

None. The prior enrollment-platform P1 is closed.

## Handoff

**Target**: `codex-multi-account-selector`
**Completed**: `feature-verify / P3 correction`
**Verdict**: `READY_TO_SHIP`
**Summary**: `All enrollment, preview, and reserved-runtime platform gates now form one fail-closed boundary; the complete feature passes independent functional, race, platform-build, UI, documentation, and governance verification.`
**Evidence**: `25328b1 + ec73d1c; full Go/vet/race; targeted race x10; three-OS builds; Web/Desktop; CI/project governance; retained exact-Linux A/B live evidence.`
**Findings**: `none; prior P1 closed`
**Blockers**: `open Security Gate only`

### Next Step

Run `security-review` for `codex-multi-account-selector`.
