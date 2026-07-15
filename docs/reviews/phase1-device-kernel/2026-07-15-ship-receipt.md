# Ship receipt: Phase 1 Device Kernel

## Status

`SHIPPED`

## Authorized action

The operator explicitly authorized immediate ship, direct main integration,
single-account operation, and no 48-hour observation period. Draft PR #13 was
marked ready and squash-merged after the Security Gate was `ACCEPTED` and all
required protected checks were green.

## Remote result

- PR: `https://github.com/jinlong17/multi-agent-desk/pull/13`
- PR head: `95749ab72cbcc00987fb08b55e88ffff2c45e3fb`
- Product merge commit on `main`: `36efca882e97e36cb5ded59ffa45b88cf416ba09`
- Final `main` after receipt/DCO-anchor documentation: `2e73929a80ed27b0c608a393b92a2a36e1324f0c`
- Merge mode: squash; no release tag, package publication, or deployment was
  performed.
- Main protection readback: seven required checks, zero required approvals,
  code-owner review disabled, strict status checks enabled.

## Evidence

- Final PR CI `29396943598`: project-verify, build-ubuntu, build-macos, and
  build-windows all passed.
- Final PR Governance `29396943643`: DCO, license-gate, and link-check passed.
- Final main CI `29398511139` and Governance `29398511092` are green at
  `2e73929`; the Governance link-check was rerun after one transient network
  reset and then passed.
- Security Gate: `ACCEPTED` in
  `2026-07-15-security-review-v2.md` at exact corrected head.
- Local evidence: full Go tests, vet, race, three-target compilation, exact
  licenses, project/CI/scaffold/workflow/dashboard verification passed.

## Rollback

Revert product commit `36efca882e97e36cb5ded59ffa45b88cf416ba09` through a
new reviewed PR; governance-only follow-up commits may be reverted separately.
No database migration, release artifact, or deployment state was created by
this ship action.

## Handoff

**Target**: `phase1-device-kernel`
**Completed**: `ship`
**Status**: `SHIPPED`
**Summary**: `Phase 1 Device Kernel was squash-merged to protected main after Security Gate acceptance and seven green protected checks.`
**Commit/Release**: `product main 36efca8; final main 2e73929; PR #13 plus governance receipts #15/#16/#17; no tag/release/deployment`
**Tests**: `local full/race/vet/cross-target/license/project/CI/scaffold/workflow/dashboard; PR CI/Governance 29396943598/29396943643; final main CI/Governance 29398511139/29398511092`
**Blockers**: `none for Phase 1 ship; later production Vault, real Provider/PTY, Windows 11 multi-user/service, packaging, deployment, and Phase 2+ gates remain open`

### Next Step

`Phase 2 feature-plan; no release follow-up was authorized or performed.`
