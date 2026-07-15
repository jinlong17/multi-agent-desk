# Verification record: Phase 1 Device Kernel P3 — BLOCKED

## Verdict

`BLOCKED` at exact head `0e91e2b6e24e7286917bfe538266cf0629239751`.

The P3 implementation is locally green and macOS/Ubuntu runner-green, but the
required Windows runtime check failed. The failure is a real portability bug,
not an unavailable runner: the test builds an executable at an extensionless
temporary path, Windows produces `<path>.exe`, and `StartProcess` invokes the
extensionless path. `TestManagerRunsRealFakeProviderSubprocess` therefore fails
before the Fake Provider emits `ready`.

## Evidence

- CI run `29391693620`, Windows job `87276396962`: `go test ./...` fails only in
  `internal/runtime`, at `manager_test.go:65`, with `provider_failed: fake
  provider could not be started`.
- The same exact head passed macOS job `87276396994`, Ubuntu job `87276396993`,
  project-verify `87276396987`, DCO, license-gate, and link-check.
- Local `go test ./...`, scoped race tests, `go vet`, and three-target builds
  passed; those results do not override the actual Windows failure.

## Required correction

Resolve the Windows executable suffix at the process boundary (or build the
test artifact with an explicit `.exe` path), add a regression check, rerun all
protected checks, and then independently verify the native two-client IPC
acceptance before changing P3 to `VERIFIED`.

No Vault, real Provider, PTY/ConPTY, Windows 11, release, or deployment claim
is made by this blocked result.
