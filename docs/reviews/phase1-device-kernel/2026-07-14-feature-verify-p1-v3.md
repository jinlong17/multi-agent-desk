# Feature verification v3: Phase 1 Device Kernel P1

- Date: 2026-07-14
- Role: `feature-verify`
- Phase: `P1 domain and Device store`
- Verified head: `0867dacdb203087df9f8c0846c8b33af74ee2689`
- Pull request: `#13` (Draft)
- Verdict: `VERIFIED`

## Conclusion

P1 meets its domain, persistence, migration, restart, concurrency, license, and
three-platform acceptance. The prior Windows database-open failure is closed by
the explicit Unix, drive-rooted Windows, and UNC SQLite file-URI mapping. The
complete Device Store suite now passes on the actual macOS, Ubuntu, and Windows
runners, so P2 is unlocked. This verdict does not approve P2-P5, close the
Security Gate, or authorize merging Draft PR #13.

## Finding closure

1. The prior Resume snapshot, non-monotonic Lease acquisition, and invalid ID
   prefix findings remain closed by direct negative tests and transactional
   rejection evidence.
2. SQLite DSN construction now emits rooted `file:///C:/...` URIs for Windows
   drive paths, authority-bearing `file://server/share/...` URIs for UNC paths,
   and rooted file URIs for Unix paths, with special characters percent-encoded.
3. Pure mapping fixtures cover accepted and rejected Unix, Windows drive, and
   UNC inputs without weakening the Store's private-directory or regular-file
   checks.
4. GitHub Actions job `87267956531` executed the complete suite on Windows
   Server 2025: `internal/domain`, `internal/storage`, and `migrations/device`
   pass, as do the Web checks and Tauri desktop check/build.

## Evidence

- Verified exact PR head `0867dacdb203087df9f8c0846c8b33af74ee2689`;
  PR #13 remains Draft, mergeable, and clean against `main`.
- PR #13 CI run `29388956859`: `project-verify` passed; `build-macos`
  passed in 1m43s; `build-ubuntu` passed in 1m57s; `build-windows` passed in
  3m56s.
- Windows job `87267956531`: the previously failing `internal/storage` suite
  passed in 0.912s, `internal/domain` passed in 0.049s, and
  `migrations/device` passed in 0.021s; the complete scaffold and Windows
  desktop release build also passed.
- PR #13 Governance run `29388956809`: `license-gate`, `dco`, and `link-check`
  passed.
- The corrected head retained green local full, race, vet, three-target
  compile, exact license, project, CI-contract, scaffold, dashboard, and diff
  evidence from the build receipt.

## Gates and scope

- Provider Gate: none for the deterministic first-party Fake Provider.
- Security Gate: remains open for final Phase 1 independent review.
- P1 blockers: none.
- Remaining work: P2-P5 plus the final Security Gate; Windows Server CI does
  not by itself prove Windows 11 multi-user/service or signed-release behavior.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-verify / P1 domain and Device store`
**Verdict**: `VERIFIED`
**Summary**: P1 domain, SQLite persistence, migrations, restart, and corrected path handling pass on actual macOS, Ubuntu, and Windows runners.
**Evidence**: Exact head 0867dac; PR #13 CI run 29388956859; Governance run 29388956809; Windows job 87267956531; retained local full/race/license/project evidence.
**Findings**: None blocking P1; Windows 11 multi-user/service and signed-release evidence remain later exit criteria.
**Blockers**: None for starting P2; the final Phase 1 Security Gate remains open.

### Next Step

Run `feature-build` P2 for `phase1-device-kernel`.
