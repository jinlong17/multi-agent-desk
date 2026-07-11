# Design: Phase 0 threat model

## Decision snapshot

- Owner: `security`; impacted module: `project-system` for documentation links.
- Produce one initial threat model derived from accepted Plan v0.2 and ADRs,
  then require independent feature verification and independent
  `security-review` acceptance before ship.
- Freeze no cryptographic suite, Provider credential behavior, browser key
  storage behavior, or Windows transport/sidecar claim.

## Document structure

`docs/THREAT_MODEL.md` contains scope/non-goals, assets and security
objectives, attacker capabilities, trust boundaries, the five `CLAUDE.md`
invariants, threat/mitigation/residual-risk rows, failure and recovery rules,
Spike-gated claims, assumptions, and review/update triggers.

Each threat row has a stable identifier, affected asset/boundary, attacker,
scenario, impact, required mitigation, evidence state, and residual risk.
Mitigations described by accepted design but not implemented are labeled
`planned`; Spike-dependent mechanisms are labeled `pending evidence`. Neither
label means the product currently resists the threat.

## Required coverage

- user/session identity versus independently enrolled Device keys;
- Control Plane key substitution, metadata exposure, replay, tampering, and
  availability;
- local Vault theft, weak permissions, locked/unlocked behavior, secret
  materialization, crash residue, and refresh rollback races;
- explicit CredentialGrant authorization, target binding, revocation limits,
  replay, and auditability;
- Provider process/config injection and Provider-specific auth uncertainty;
- Web XSS/site-data loss and metadata-only fallback;
- local IPC client authorization and cross-client session control;
- compromised authorized device/server and the impossibility of guaranteed
  remote erasure;
- dependency/license/supply-chain and update/signing risks at the level already
  stated in Plan v0.2.

## Failure and recovery

The model must prefer fail-closed behavior for key mismatch, locked Vault,
ambiguous credential recovery, unauthorized control, and unsupported browser
key storage. It must distinguish Stop/Kill/revocation from erasure and preserve
audit evidence without logging secrets or terminal plaintext.

## Compatibility and rollback

No runtime or data migration. Rollback removes only documentation and workflow
state. The threat model is versioned with the repository and must be reviewed
when trust boundaries, assets, credential flows, cryptographic protocol,
Provider integration, or supported platforms change.
