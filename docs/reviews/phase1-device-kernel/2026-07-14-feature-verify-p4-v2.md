# Verification record: Phase 1 Device Kernel P4 — VERIFIED

## Verdict

`VERIFIED` at implementation/evidence head
`b57fb4014c5dc567dcb9287b6ceca003bda6143c`.

The P4 Windows blocker was reproduced and corrected. Materialization now uses
the platform-private filesystem boundary: Unix checks owner-only mode bits and
Windows applies the current-logon DACL. The Windows test no longer treats the
platform's ordinary `0666` mode-bit report as an ACL failure.

## Acceptance evidence

- Draft PR #13 CI run `29393903799` passed project-verify `87283019631`,
  macOS `87283019664`, Ubuntu `87283019825`, and Windows `87283019648`.
- The Windows runner executed `go test ./...`; its log reports
  `internal/vault` green, including `TestMaterializerAtomicCommitAndQuarantine`.
- Governance run `29393903755` passed DCO `87283019479`, license-gate
  `87283019530`, and link-check `87283019542`.
- Local `go test ./...`, `go vet ./...`, scoped race coverage, and Windows
  cross-compilation also pass at the exact head.

## Scope and limits

P4 proves the locked/unlocked fake Vault boundary, credential revision/CAS,
atomic materialization, manifest/content digest checks, release, restart
recovery, and quarantine. It does not claim production encryption,
Argon2id/OS-keychain integration, real Provider credentials, Windows 11
multi-user/service acceptance, signed packaging, release, or deployment. The
final Phase 1 Security Gate remains open.

## Next action

Unlock `feature-build P5 CLI/TUI and platform exit`; preserve the same protected
three-platform and governance gates for the Phase 1 exit scenario.
