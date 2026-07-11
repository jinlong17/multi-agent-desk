# Contract: Phase 0 threat model

This feature defines a security-documentation contract, not a runtime API.

## Required sections

`THREAT_MODEL.md` must include scope/non-goals, assets/objectives, attackers,
trust boundaries, invariant mapping, threat matrix, failure/recovery,
Spike-gated evidence, residual risk, assumptions, and update triggers.

## Evidence-state vocabulary

- `accepted design`: a reviewed architecture requirement, not implementation
  evidence;
- `planned`: required mitigation has not yet been proven in running code;
- `pending evidence`: an exact Spike must resolve the mechanism or support
  claim;
- `verified`: allowed only with a reproducible artifact linked from the row;
- `deferred`: explicitly outside the current goal/platform acceptance and kept
  as an open gate.

Unknown, planned, pending, partial, or deferred evidence is never equivalent to
verified or pass.

## Invariant contract

The document must state all five `CLAUDE.md` invariants without weakening them:
Passkey is not a decryption key; the Control Plane is not the key trust anchor;
grants are explicit, target-scoped, encrypted, revocable, and not remotely
erasable after compromise; credential refresh is single-writer with
revision/CAS; automatic rotation/quota/rate-limit evasion is prohibited.

## Spike linkage

Every pending E2EE, browser key, Codex auth/refresh, Claude config/keychain, or
Windows transport/sidecar claim names its exact current Spike slug. The three
Windows Spikes remain DRAFT and Windows interactive acceptance is recorded as
deferred with no local Windows machine.
