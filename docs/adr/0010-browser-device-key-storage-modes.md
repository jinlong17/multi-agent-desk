# ADR 0010: Browser device key storage modes

- Status: Accepted
- Date: 2026-07-14
- Owner: `web`
- Security gate: resolved by `docs/reviews/spike-browser-key-storage/2026-07-14-security-review.md`

## Context

A Web Device needs persistent signing and key-agreement material before it may
receive E2EE terminal content, approve actions, or participate in Credential
Grants. Browser algorithm names and key-generation success are insufficient:
the private key must remain usable and non-exportable after storage and a full
process restart.

Phase 0.5 evidence shows:

- Chrome 150 on macOS, Edge 149 on Windows, and Firefox 152 on Linux persist
  non-exportable Ed25519 and X25519 `CryptoKey` objects in IndexedDB and reuse
  them after a browser process restart.
- Safari 26.4 and WebKit on macOS 26.4/26.5.2 persist non-exportable Ed25519 and
  AES-GCM keys, but persisted X25519 use returns `TypeError`.
- Safari WebDriver sessions are isolated from one another, so a minimal
  WKWebView app was also executed in separate processes to verify the same
  storage behavior without relying on cross-session automation state.

## Decision

Browser enrollment uses a versioned runtime capability probe and assigns
exactly one auditable storage mode:

1. `native`: non-exportable Ed25519 and X25519 WebCrypto private keys pass
   generation, IndexedDB persistence, process-restart use, and export-rejection
   tests.
2. `software_wrapped`: when native X25519 persistence/use fails, a library
   X25519 private key is encrypted with a non-exportable AES-256-GCM WebCrypto
   wrapping key stored in IndexedDB. Raw private bytes exist only for the
   shortest required in-memory operation and are wiped immediately afterward.
3. `metadata_only`: when neither accepted mode passes, the browser cannot
   receive E2EE content, approve actions, or initiate Credential Grants.

The following controls are part of this decision, not optional hardening:

- `software_wrapped` is an explicit user-visible downgrade and must never be
  labeled native, hardware-backed, or XSS-resistant.
- The Control Plane never receives unwrapped device private material and does
  not act as a recovery escrow.
- A pure Web Device cannot bootstrap the E2EE trust graph. Phase 4a uses a
  Daemon whose remote Ed25519/X25519 identity is sealed by the already-shipped
  portable password-derived Vault v1 as the initial anchor; that path is not
  OS-backed. OS Keychain/DPAPI/Secret Service wrapping and the Desktop product
  key store remain Phase 5. An already pinned key-bearing Device approves Web.
- Enrollment and local audit state bind browser family/version, probe version,
  key-suite version, key revision, and storage mode.
- A mode, key-handle, or origin change creates a new device-key revision and
  requires the normal pinning or attestation ceremony.
- The enrolled web application uses a stable dedicated origin with restrictive
  CSP, no unreviewed third-party scripts, Trusted Types where supported, locked
  dependencies, and release-integrity controls.
- Origin-storage loss means device-identity loss and re-enrollment; it never
  triggers silent recreation of the same identity.

## Consequences

Chrome, Edge/Windows, and Firefox/Linux can use the native path at the tested
versions. Safari/WebKit is supported with the `software_wrapped` fallback at
the tested versions. Unknown or regressed versions are not grandfathered; they
must pass the runtime probe or become `metadata_only`.

The fallback is weaker than a native non-exportable X25519 key. Same-origin
malicious script can decrypt and permanently exfiltrate raw fallback key bytes.
Native keys can also be abused while malicious script is active, so no browser
mode is an origin-compromise boundary. Device revocation stops future access
after protocol enforcement and rotation but cannot erase already copied
material.

This ADR does not freeze the E2EE envelope, AAD, replay, attestation, recovery,
or Credential Grant protocol. Those remain gated by
`spike-e2ee-protocol-vectors` and its independent cryptographic review.

Phase 4a pulls forward only the browser storage probe, Ed25519/X25519 key
lifecycle, proof of possession, metadata enrollment, and revocation. Pairwise
Roots, HPKE, WSS, terminal/Approval payloads, and Credential Grants are not a
Phase 4a behavior claim.

## Evidence

- `docs/spikes/browser/2026-07-14-browser-key-storage-spike.md`
- `docs/spikes/browser/chrome-macos.json`
- `docs/spikes/browser/edge-windows.json`
- `docs/spikes/browser/firefox-linux.json`
- `docs/spikes/browser/safari-macos.json`
- `docs/spikes/browser/webkit-macos-ci.json`
- `docs/spikes/browser/webkit-macos.json`
- `docs/reviews/spike-browser-key-storage/2026-07-14-security-review.md`
- PR #6 browser run `29373730100`
