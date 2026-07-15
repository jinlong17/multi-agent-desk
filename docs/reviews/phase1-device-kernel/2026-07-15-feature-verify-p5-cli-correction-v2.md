# Feature verification: P5 CLI correction hardening — READY_TO_SHIP

## Verdict

`READY_TO_SHIP` at exact implementation/evidence head
`99fe917aef7906ac46304e93b02897d63e6e091a`.

The parser-boundary hardening closes the remaining argv ingress gap: Vault
unlock rejects the removed `--secret` flag and any positional argument before
reading stdin. The request-bound idempotency correction remains intact. The
final P5 implementation is ready for the independent Security Gate; this
verdict does not authorize merge or release.

## Acceptance evidence

- Local `go test ./...`, `go vet ./...`, scoped `go test -race`, three-target
  `go test -c` for `darwin/arm64`, `linux/amd64`, and `windows/amd64`, exact
  `go-licenses check --include_tests ./...`, `npm run project:verify`,
  `npm run ci:verify`, `npm run scaffold:verify`, `git diff --check`, and
  workflow/dashboard verification pass.
- Draft PR #13 CI run `29396390634` passed project-verify `87290750927`,
  macOS `87290750960`, Ubuntu `87290752643`, and Windows `87290750942`.
- Governance run `29396390660` passed DCO, license-gate, and link-check.
- `TestCLIRequestIdentityBindsBodyAndRevision` proves distinct operation
  bodies receive distinct request IDs/keys and exact retries retain identity.
- `TestVaultSecretReaderIsBoundedAndDoesNotEcho`,
  `TestVaultUnlockRejectsArgvSecret`, and
  `TestVaultUnlockRejectsPositionalSecret` prove bounded stdin input, no CLI
  output of the value, and rejection of flag or positional argv secrets.

## Scope and limits

P5 remains an authenticated thin CLI/TUI over the local Device Kernel;
service rendering is non-mutating and client provisioning remains offline-only.
`--secret-stdin` is intended for a pipe or other no-echo input source;
production Vault cryptography, real Provider/PTY behavior, Windows 11
multi-user/service acceptance, signed packaging, deployment, and release are
not claimed. The independent Security Gate remains mandatory.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-verify / P5 CLI correction hardening`
**Verdict**: `READY_TO_SHIP`
**Summary**: `The final P5 correction passes local, governance, and protected three-platform evidence; request identity and Vault argv ingress controls are closed.`
**Evidence**: `99fe917; CI 29396390634; Governance 29396390660; local full/race/vet/cross-target/license/project/CI/scaffold/workflow/dashboard checks`
**Findings**: `none`
**Blockers**: `none; independent Security Gate remains required before ship`

### Next Step

Run `security-review` for `phase1-device-kernel` at the corrected exact head.
