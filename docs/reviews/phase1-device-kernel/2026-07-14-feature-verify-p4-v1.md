# Verification record: Phase 1 Device Kernel P4 — BLOCKED

## Verdict

`BLOCKED` at implementation/evidence head `da67374`.

The P4 implementation passes the local Go, race, vet, and cross-build checks,
but the protected Windows runner fails the P4 materialization test. The failure
is concrete and reproducible: the test asserts Unix permission bits (`0600`),
while Windows Go reports ordinary file mode bits as `0666`. The existing
Windows private-directory boundary uses the current-logon DACL and explicitly
does not treat mode bits as an access-control primitive.

## Acceptance evidence

- Draft PR #13 CI run `29393403418` passed the project, macOS, and Ubuntu jobs;
  project job `87281550016`, macOS job `87281549967`, and Ubuntu job
  `87281549973` were green.
- Governance run `29393403406` passed DCO `87281506703`, license-gate
  `87281506640`, and link-check `87281506613`.
- Windows job `87281550000` failed only in `internal/vault`:
  `TestMaterializerAtomicCommitAndQuarantine` reported
  `vault_test.go:76: credential mode=666`. The remaining Windows Go packages,
  including device, runtime, storage, app, and domain, passed.

## Required correction

Make the permission assertion platform-aware: retain the `0600` mode assertion
on Unix, while Windows must be validated through an explicit ACL boundary or a
documented test seam. Do not interpret a Windows `0666` mode-bit report as
proof that the DACL is permissive. Update the P4 evidence and limits so that
the phase claims only the boundary actually tested; Windows 11 multi-user and
production credential protection remain outside P4.

## Next action

`feature-build P4 Windows permissions correction`, then a fresh independent
three-platform verification on the corrected exact commit. The final Security
Gate remains open.
