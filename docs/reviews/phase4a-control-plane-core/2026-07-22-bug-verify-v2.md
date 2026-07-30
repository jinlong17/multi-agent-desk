# Bug verification: Phase 4a P2 Windows residue-test private root

## Verdict

`READY_TO_SHIP` for the scoped
`phase4a-p2-windows-residue-test-private-root` bugfix.

The feature-level Phase 4a P2 status remains `BLOCKED`. This local verdict does
not claim that the uncommitted repair has executed on Windows; a fresh
exact-head native Windows Actions run remains mandatory after the coordinator
commits and pushes the scoped repair. The clean-SHA Chrome/Safari journeys,
physical Safari Touch ID/platform-Passkey ceremony, and final machine scan also
remain separate feature-level blockers.

## Scope and ownership

- Owner: `control-plane` (high confidence).
- Secondary impacts: `core`/Device storage, `security`, and `project-system` CI
  evidence.
- Audited implementation files:
  `internal/controlplane/p2_secret_residue_test.go` and
  `internal/storage/p2_secret_residue_test.go`.
- Base commit:
  `95ab131348dedde8a87d644bf0f2e6306b839b2c`; branch:
  `codex/control-plane/phase4a-core`.
- No implementation, plan, dashboard judgment, commit, push, or PR write was
  performed by this verifier.

## Original failure and root-cause reproduction

The public Actions job API reconfirmed Windows job `89080953653` in run
`29967128042` at exact head `95ab131348dedde8a87d644bf0f2e6306b839b2c`:
`build-windows` concluded `failure`; both `Run Go test suite` and
`Windows P2 private-storage acceptance` failed, while the following migration
stress step passed.

The retained failure evidence names only the new residue fixtures:

- Control Plane:
  `TestP2SecretCanariesAbsentFromClosedDatabaseAndSidecars` failed with
  `Windows path owner is not the current logon SID`.
- Device storage:
  `TestP2SecretCanariesAbsentFromClosedDeviceDatabaseAndSidecars` failed with
  `permission_denied: private Windows owner is not the current logon SID`.

`git show HEAD:<fixture>` reproduced the bad setup in both files: each passed
an already-existing `t.TempDir()` parent after only `os.Chmod(0700)`. On
Windows, chmod does not establish the repository's exact owner plus protected
TokenUser/LocalSystem DACL contract. This is a test-fixture boundary error, not
a reason to relax the production verifier.

## Regression and boundary evidence

The implementation diff is limited to the two fixture roots:

- Control Plane now uses the existing `privateTestDirectory(t)` helper, which
  applies and verifies the exact platform-private directory boundary.
- Device storage now uses a nonexistent child of `t.TempDir()`, causing the
  unchanged production `Open -> ensurePrivateDirectory ->
  protectDevicePrivateDirectory` path to create, protect, and verify the root.

`git diff` reports no change in
`internal/controlplane/privatefs_windows.go`,
`internal/storage/schema_v7_backup_windows.go`, or
`internal/device/privatefs_windows.go`. Inspection confirms the production
Windows boundary still fails closed unless the owner is the current process
TokenUser SID, the DACL is protected, and it contains exactly two zero-flag
full-access allow ACEs for TokenUser and LocalSystem.

The following independent checks passed:

1. `go test -count=10 -run 'TestP2SecretCanariesAbsentFromClosed(Database|DeviceDatabase)AndSidecars' ./internal/controlplane ./internal/storage`
   passed in both packages.
2. `go test -count=1 ./...` passed all packages.
3. `go vet ./...` passed.
4. `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c` passed for both
   `./internal/controlplane` and `./internal/storage`; `file` identified both
   outputs as PE32+ x86-64 Windows console executables.
5. `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build ./cmd/...` passed.
6. `npm run project:verify` passed: workflow `10/3/17/20/15` and dashboard
   generation/verification with exactly the three expected dirty paths.
7. `npm run ci:verify` passed: 7 Actions checks, 15 pinned actions,
   CODEOWNERS, positive/negative CI fixtures, receipt tests 50/50, 324 links,
   6 pnpm license groups, and 418 Cargo packages.
8. `gofmt -d` for both fixtures and `git diff --check` produced no output.

## Evidence boundary

This macOS host cannot execute the generated Windows PE test binaries.
Cross-compilation proves Windows build-tag and type compatibility only. It does
not prove owner/DACL runtime behavior. The coordinator must therefore commit
and push the verified scoped fix and require a fresh exact-head native Windows
runner to pass the full Go and Windows P2 private-storage steps before P2 can
advance.

## Findings

None.

## Blockers

None for shipping this scoped bugfix to obtain the exact-head native Windows
receipt. Feature-level P2 remains blocked on that receipt and the already
recorded real-browser, physical Touch ID, and final-scan evidence.
