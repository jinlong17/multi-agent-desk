# Security review: phase0-threat-model

## Verdict

`ACCEPTED`

The initial threat model is suitable as the Phase 0 security baseline. It
correctly describes required controls as accepted design, planned, pending, or
deferred rather than implemented/verified, and it preserves every mandatory
trust invariant. Acceptance applies to the document and its evidence honesty;
it does not accept any pending runtime, cryptographic, Provider, browser, or
Windows claim.

## Findings

No P0, P1, or P2 finding requiring revision.

## Security review matrix

| Area | Conclusion |
|---|---|
| Trust anchors and pinning | Passkey/device-key separation is explicit; Control Plane directory is an index, not an anchor; key substitution requires local-pin/attestation/fingerprint defenses |
| Attestation and enrollment | directly pinned approver, subject-key pinning, key-change-as-new-device, and user-error/compromised-approver residual risks are represented |
| Replay, AAD, and ordering | source, target, purpose, audience, revision, expiry, message ID, replay cache, and ordering requirements are named; protocol remains pending the E2EE Spike |
| Vault and materialization | locked fail-closed behavior, single writer, monotonic revision/CAS, quarantine, filesystem/process exposure, and root/live-process residual risk are explicit |
| CredentialGrant | explicit confirmation, eligible target capability, direct pin/attestation, target-bound encryption, revision/expiry/replay, receipt, revocation limits, and non-erasure are covered |
| Local IPC and session control | peer authorization, endpoint permissions, capability/lease checks, detach/process separation, and same-user/root residual risk are covered; Windows transport remains pending |
| Provider boundary | process/config injection, isolated runtime, materialized plaintext, pinned session identity, and exact Provider Spike dependencies are covered without compatibility claims |
| Web/XSS | active-origin compromise, non-exportable-key limitation, metadata-only fallback, CSP/dependency controls, and pending browser evidence are explicit |
| Audit and privacy | protected plaintext exclusion, allowlisted telemetry, redaction tests, and novel-field/crash-tool residual risk are covered |
| Availability and recovery | relay suppression, offline local operation, fail-closed remote actions, Vault recovery, ambiguous lease quarantine, and data-loss risk are represented |
| Supply chain | dependencies, external adapters, research/license contamination, provenance/signing/SBOM requirements, and deferred release controls are represented |
| Windows boundary | exact deferred wording, open-gate/not-pass semantics, CI-vs-interactive distinction, and all three DRAFT Spikes are preserved |

## Evidence

- `docs/THREAT_MODEL.md`
- `CLAUDE.md` security invariants
- ADR 0002, 0003, 0007, and 0008
- Plan v0.2 pairing, Vault/materialization, failure, and risk sections
- all seven linked Spike dev_logs; three Windows logs confirmed `DRAFT`
- `npm run project:verify && git diff --check` — pass during independent review
- feature verification report and its structure/link/invariant evidence

## Residual risk

An authorized or root-compromised device can copy Provider-readable plaintext;
revocation cannot erase it. A compromised Control Plane retains metadata and
availability leverage. Active XSS can use an approved Web Device's live keys
and decrypted content. Users can approve the wrong fingerprint or grant.
Upstreams/toolchains/reviewers can fail. Windows interactive behavior is
unverified and deferred. These risks are disclosed, not accepted as product
release risk by this documentation verdict; later features must test and gate
their mitigations.
