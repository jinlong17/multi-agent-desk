# Security review: As-built product documentation authority

## Verdict

**ACCEPTED** for the documentation trust, Provider-boundary, residual-risk, and release wording at `d0459e0c16867aee4cef026ff1c037432e21462f`.

This accepts the bounded documentation feature only. It does not accept an implementation security posture, remote product flow, Provider capability, platform support, deployment, package, release, or ship decision. The Provider Gate remains open.

## Scope and evidence

- Re-read the target feature state, canonical product snapshot, append-only claim ledger, P4 verification trace, Provider compatibility matrix, and threat model.
- Verified both positive Provider receipt objects and report paths are reachable from the ledger baseline `397194bfaea6983c4c0a54289d8bb1844711cfd6`: `phase2-codex-vertical-slice feature-verify@250bf57f2ec21beb5f02e03b58f030b9d67e5ff4` and `codex-multi-account-selector feature-verify@b041240646c337aa67a2f3c078a498356218b44d`.
- Ran `pnpm run ci:links` (315 Markdown files) and `pnpm run project:verify` with the repository Node 24 runtime; both passed. These are structural results only.
- Searched the reviewed product/documentation surfaces for credential, private key, and JWT-like values; none were found.

## Security conclusion

The documentation retains the required trust boundaries: a Passkey is not an E2EE Device Key; the Control Plane directory is not a key trust anchor; key changes require re-pairing; unpaired browsers are metadata-only; and remote Web/Desktop, enrollment, grant, and revocation product flows remain planned.

Credential wording remains accurate: grants are explicit, encrypted, target-device scoped, and revocable only for future use. It expressly avoids claiming remote erasure of plaintext already copied to an authorized or compromised target. The materialization and host/Provider-process exposure risks remain visible.

Provider wording is also bounded: only the exact Codex CLI `0.144.2` Linux `x86_64`/`amd64` source-built scope is positive, while macOS identity acceptance, real Windows Codex, broad account/auth/usage conclusions, and managed Claude surfaces retain their stated pending, unknown, or unsupported boundaries. The documentation does not treat compatibility evidence, CI, source presence, or a structural check as release or security assurance.

## Findings

No P0, P1, or P2 security findings in the reviewed documentation scope.

## Residual risk

This acceptance does not reduce the threat-model residual risks: host root or admin, a compromised Provider process, backup/crash tooling, an already authorized compromised target, active same-origin Web compromise, metadata and availability exposure at the Control Plane, human approval error, supply-chain compromise, and deferred Windows/remote production enforcement remain. Any future change to credential, key, E2EE, enrollment, revocation, logging, or residual-risk wording requires a new independent security review.

## Gate disposition

- Security Gate: resolved for this documentation wording only.
- Provider Gate: remains open; no documentation verdict establishes Provider compatibility or accepts Provider risk.
- Release/ship: not accepted and still requires explicit human authorization.
