# Security review: browser non-exportable key storage

Date: 2026-07-14  
Role: `security-review`  
Verdict: `ACCEPTED`

## Scope of acceptance

This verdict accepts the Phase 0.5 compatibility conclusion and its bounded
fallback. It does not approve production E2EE code, the envelope protocol,
attestation, replay handling, recovery, or Credential Grant behavior. Those
remain gated by `spike-e2ee-protocol-vectors` and the Phase 4b security review.

## Evidence reviewed

- `docs/spikes/browser/2026-07-14-browser-key-storage-spike.md`
- Sanitized Chrome, Edge/Windows, Firefox/Linux, Safari 26.4, and WebKit macOS
  26.4/26.5.2 JSON artifacts under `docs/spikes/browser/`
- Reproducible WebCrypto/IndexedDB, Selenium, Playwright, and WKWebView probes
- PR #6 browser run `29373730100`: Edge/Windows, Firefox/Linux, and
  Safari/macOS jobs passed
- [W3C Web Cryptography Level 2](https://www.w3.org/TR/webcrypto-2/), including
  its origin-sharing, script-injection, and storage-loss security notes
- [W3C Web Cryptography API Recommendation](https://www.w3.org/TR/2017/REC-WebCryptoAPI-20170126/#cryptokey-interface)
  structured-clone requirements
- [Apple Safari WebDriver documentation](https://developer.apple.com/documentation/safari-developer-tools/webdriver/)
  describing isolated automation windows
- `docs/THREAT_MODEL.md`, especially the malicious-origin, trust-anchor,
  revocation, and key-substitution boundaries

## Compatibility conclusion

| Runtime | Accepted mode | Security interpretation |
|---|---|---|
| Chrome 150 / macOS | native non-exportable Ed25519 + X25519 | API-level non-exportability; not hardware attestation or an XSS boundary |
| Edge 149 / Windows | native non-exportable Ed25519 + X25519 | same boundary as Chrome |
| Firefox 152 / Linux | native non-exportable Ed25519 + X25519 | same boundary as Chrome |
| Safari 26.4 / WebKit macOS 26.4 and 26.5.2 | Ed25519 plus non-exportable AES-GCM wrapping-key fallback | accepted compatibility downgrade; raw X25519 material exists briefly in JavaScript memory |

Safari/WebKit consistently returned `TypeError` for the persisted X25519 native
path, while the non-exportable AES-GCM wrapping key and encrypted private-key
envelope survived a separate WebKit application process. The fallback is
therefore evidence-backed, not inferred from a compatibility table.

## Findings and required downstream controls

### P1 — fallback permits durable key theft under same-origin script compromise

The fallback must decrypt the library private key into JavaScript memory. An
XSS or compromised same-origin dependency can exfiltrate those bytes and retain
the device identity after the injection is removed. A native non-exportable
`CryptoKey` can also be abused while malicious script is active, but it does
not expose the raw private bytes through the WebCrypto API.

Acceptance boundary:

- enable fallback only after a versioned runtime capability probe fails the
  native X25519 persistence/use check;
- label enrollment and audit state as `software_wrapped`, never `native` or
  `hardware_backed`;
- show an explicit compatibility downgrade before enabling terminal content,
  approvals, or Credential Grants;
- zero transient raw-key buffers immediately after import/use and never log,
  serialize, sync, back up, or send them to the Control Plane;
- support immediate device revocation and rotation, while stating that
  revocation cannot erase already exfiltrated key material.

### P1 — non-exportability is not an origin-compromise defense

All accepted browser modes remain callable by same-origin JavaScript. Phase 4b
must use a dedicated stable origin, a restrictive CSP, no unreviewed third-party
scripts, Trusted Types where available, locked dependencies, and explicit
script-integrity/release controls. The Web Device must not become the initial
E2EE trust anchor; an OS-Vault-backed Daemon/Desktop must approve enrollment as
already required by the implementation plan.

### P1 — storage loss must not create escrow or silent recovery

Users and browsers can clear or evict origin storage. Loss or corruption of the
local key means loss of that device identity. Recovery must use re-enrollment
from another pinned device or an approved recovery ceremony. The Control Plane
must not receive an unwrapped device private key or silently recreate the same
identity.

### P2 — compatibility mode must be protocol-visible and auditable

Enrollment records and local audit events must bind the runtime probe version,
browser family/version, key-suite version, and `key_storage_mode` (`native`,
`software_wrapped`, or `metadata_only`). A change in key-storage mode or key
handle must create a new device key revision and require the normal pinning or
attestation path; it cannot be treated as transparent rotation.

### P2 — this Spike does not resolve protocol attacks

AAD binding, replay caches, sequence handling, session-key rotation,
attestation, key substitution, and Credential Grant audience/purpose/expiry
remain outside this Spike. No browser result may be used to bypass the pending
E2EE protocol vectors or independent cryptographic review.

## Verdict rationale

No P0 condition was found. The native paths provide the expected WebCrypto API
guarantee on Chrome, Edge, and Firefox. Safari/WebKit has a reproducible,
explicitly weaker fallback and a deterministic metadata-only downgrade if that
fallback fails. The residual risks are already compatible with the project's
malicious-origin threat model when the controls above are treated as mandatory
Phase 4b requirements.

`ACCEPTED` resolves only this Spike's Security Gate. Omitting any P1 acceptance
boundary from the compatibility decision or E2EE ADR must reopen the gate.

## Handoff

**Target**: `spike-browser-key-storage`
**Completed**: `security-review`
**Verdict**: `ACCEPTED`
**Summary**: Native non-exportable storage is supported on Chrome, Edge, and Firefox; Safari/WebKit is accepted only through an explicit software-wrapped fallback.
**Findings**: No P0; P1 durable fallback-key exfiltration under origin compromise, origin hardening, and no-escrow recovery boundaries; P2 auditable mode binding and protocol-scope limits.
**Evidence**: `docs/spikes/browser/`, PR #6 run `29373730100`, W3C WebCrypto, Apple Safari WebDriver, and `docs/THREAT_MODEL.md`.
**Residual Risk**: Same-origin script can abuse every browser key and can permanently exfiltrate the software-wrapped raw private key; revocation cannot erase copied material.

### Next Step

Run `feature-plan` to record the compatibility decision and bind these controls
into the E2EE ADR inputs.
