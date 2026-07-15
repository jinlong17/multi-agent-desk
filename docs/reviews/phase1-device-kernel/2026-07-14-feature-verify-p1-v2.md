# Feature verification v2: Phase 1 Device Kernel P1

- Date: 2026-07-14
- Role: `feature-verify`
- Phase: `P1 domain and Device store`
- Verified head: `fb71bf0868bce13c6b4837983f787c903cdf2167`
- Pull request: `#13` (Draft)
- Verdict: `BLOCKED`

## Conclusion

All three prior domain findings are correctly fixed and covered by direct
negative tests. Local macOS evidence and six of seven protected GitHub checks
pass. The actual Windows runner consistently fails every Device Store test at
database `Ping`, so P1 still does not meet its mandatory Windows runtime
acceptance and cannot advance to P2.

## Prior finding closure

1. Resume creation now loads the terminal source Capability snapshot inside the
   transaction, rejects non-canonical storage, requires source
   `session.resume`, and requires exact source/new snapshot equality. Missing,
   expanded, and reduced snapshot tests prove no rejected row is persisted.
2. ControllerLease acquisition now rejects a current revision below one and an
   acquisition time before the current last heartbeat/release boundary.
3. `ValidateID` now applies the same 24-character lowercase/digit/underscore
   prefix grammar as `NewID` and exact 16-byte hex suffix bounds.

## Blocking finding

### P1 — Windows absolute Device path cannot be opened by the SQLite DSN

Files: `internal/storage/store.go` (`Open` DSN construction),
`internal/storage/store_test.go`.

GitHub Actions run `29388679737`, job `87267159036`, reaches `go test ./...` on
the Windows runner. `internal/domain` and `migrations/device` pass, but all eight
`internal/storage` tests fail immediately with `conflict: database open failed`
from the Store's `PingContext` path. macOS and Ubuntu run the same suite and
pass.

The Store constructs a file URL from `filepath.ToSlash(absolute)`. On Windows a
drive-rooted path is shaped `C:/...`; using it as `url.URL.Path` without a URI
root produces a form that is not the portable absolute SQLite filename used on
macOS/Linux. Local Windows cross-compilation cannot detect that runtime path
semantic.

Required correction: use a modernc-supported DSN construction that preserves
drive-rooted and UNC absolute paths without weakening path inspection, add a
platform-aware unit test for the DSN/path mapping, keep public errors redacted,
and require a new Windows runner to execute the complete Store suite. macOS,
Ubuntu, license, governance, and DCO checks must remain green.

## Evidence

- Independent local `go test -count=1` and scoped `go test -race -count=1` —
  pass after the three domain corrections.
- Local `go vet ./...`, full tests, exact Go license gate, project/CI/scaffold,
  links, dashboard, and diff integrity — pass.
- PR #13 CI run `29388679737`: `project-verify`, `build-macos`, and
  `build-ubuntu` pass; `build-windows` fails at Store open.
- PR #13 Governance run `29388679739`: `license-gate`, `dco`, and `link-check`
  pass.
- Failed Windows log retained under job `87267159036`; no failure was converted
  into cross-compile success.

## Gates and clearing condition

- Provider Gate: none.
- Security Gate: remains open for final Phase 1.
- External blocker: none; a code correction and rerun are available.
- Clearing role: `feature-build` P1 fixes portable SQLite DSN construction,
  adds regression evidence, restores `READY_FOR_VERIFY`, pushes the corrected
  head to Draft PR #13, and returns to independent verification.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-verify / P1 domain and Device store`
**Verdict**: `BLOCKED`
**Summary**: The three domain findings are closed and six protected checks pass, but every Windows Store test fails at database open on the actual runner.
**Evidence**: Local full/race/governance evidence; PR #13 CI run 29388679737 and Governance run 29388679739; failed Windows job 87267159036.
**Findings**: P1 correct drive-rooted and UNC-safe SQLite DSN construction and prove the complete Store suite on a new Windows runner.
**Blockers**: Actual Windows runtime Store open is failing; cross-compilation is not acceptable replacement evidence.

### Next Step

Run `feature-build` P1 correction for `phase1-device-kernel`.
