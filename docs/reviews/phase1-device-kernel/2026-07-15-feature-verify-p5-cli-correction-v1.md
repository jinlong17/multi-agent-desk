# Feature verification: P5 CLI correction — READY_TO_SHIP

## Verdict

`READY_TO_SHIP` at exact implementation/evidence head
`a2898a603effa7be3ee04a21c1cfa55a701c0839`.

The two P1 findings from the Security Gate correction are closed at the CLI
boundary: request identity is bound to method/body/lease data, and Vault
unlock no longer accepts argv secret material. The corrected P5 surface is
ready for the required independent Security Gate review; this verdict does
not itself authorize merge or release.

## Acceptance evidence

- Local `go test ./...`, `go vet ./...`, scoped `go test -race`, three-target
  `go test -c` for `darwin/arm64`, `linux/amd64`, and `windows/amd64`, exact
  `go-licenses check --include_tests ./...`, `npm run project:verify`,
  `npm run ci:verify`, `npm run scaffold:verify`, and `git diff --check` pass.
- Draft PR #13 CI run `29395736524` passed project-verify `87288702701`,
  macOS `87288702686`, Ubuntu `87288702693`, and Windows `87288702656`.
- Governance run `29395736454` passed DCO `87288702135`, license-gate
  `87288702080`, and link-check `87288702081`.
- `TestCLIRequestIdentityBindsBodyAndRevision` proves distinct operation
  bodies receive distinct request IDs/keys and exact retries retain identity.
- `TestVaultSecretReaderIsBoundedAndDoesNotEcho` and
  `TestVaultUnlockRejectsArgvSecret` prove bounded stdin input, no CLI output
  of the value, and rejection of the former `--secret` flag.

## Scope and limits

P5 remains an authenticated thin CLI/TUI over the same local Device Kernel;
service rendering remains non-mutating and client provisioning remains
offline-only. `--secret-stdin` is intended for a pipe or other no-echo input
source; production Vault cryptography, real Provider/PTY behavior, Windows 11
multi-user/service acceptance, signed packaging, deployment, and release are
not claimed. The Security Gate must independently accept the corrected head.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `feature-verify / P5 CLI correction`
**Verdict**: `READY_TO_SHIP`
**Summary**: `The corrected P5 CLI passes local, governance, and protected three-platform evidence; both scoped P1 findings are closed.`
**Evidence**: `a2898a6; CI 29395736524; Governance 29395736454; local full/race/vet/cross-target/license/project/CI/scaffold checks`
**Findings**: `none`
**Blockers**: `none; independent Security Gate remains required before ship`

### Next Step

Run `security-review` for `phase1-device-kernel` at the corrected exact head.
