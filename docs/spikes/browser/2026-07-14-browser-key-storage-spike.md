# Browser non-exportable key storage spike

Date: 2026-07-14  
Owner: `web`  
Secondary impacts: `security`, `control-plane`  
Branch: `codex/web/spike-browser-key-storage`

## Question

Can each supported browser retain an E2EE device key across a full browser
process restart without exposing private-key bytes to JavaScript, and is there a
bounded fallback when a browser lacks native Ed25519 or X25519?

## Acceptance contract

A browser passes the native path only when all of the following are observed in
that browser engine:

1. `Ed25519` and `X25519` private `CryptoKey` objects are generated with
   `extractable=false`.
2. The keys are persisted in IndexedDB, the browser process is stopped, and a
   fresh browser process can use them.
3. Ed25519 sign/verify and X25519 derive operations succeed after restart.
4. Private-key export is rejected with `InvalidAccessError`.

The bounded fallback generates a non-exportable AES-GCM wrapping key in
WebCrypto, encrypts a simulated raw library private key before IndexedDB
storage, wipes the source byte array, and repeats the restart/decrypt check. A
browser that passes neither path must be restricted to metadata-only access.

## Reproducible probes

- `poc/index.html` is the engine-neutral test page.
- `run_chromium_probe.cjs` exercises an installed Chromium-family browser with
  Playwright and a persistent, temporary profile.
- `run_selenium_probe.py` exercises Chrome, Edge, Firefox, or Safari with two
  independent WebDriver sessions.
- `run_webkit_process_probe.py` compiles a minimal, background-only WKWebView
  app and runs write/read phases in separate application processes without
  touching the operator's open Safari windows.
- `.github/workflows/browser-key-spike.yml` runs the same Selenium probe on
  Edge/Windows, Firefox/Linux, and Safari/macOS. Safari automation is enabled
  only inside the disposable GitHub runner.

Local Chrome command:

```sh
NODE_PATH=/path/to/node_modules node \
  docs/spikes/browser/run_chromium_probe.cjs \
  --browser-name 'Google Chrome' \
  --binary '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome'
```

## Evidence matrix

| Browser / OS | Exact version | Native Ed25519 + X25519 | AES-GCM fallback | Process restart | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| Google Chrome / macOS 26.5.2 arm64 | 150.0.7871.116 | pass | pass | pass | `PASS` | `chrome-macos.json` |
| Microsoft Edge / Windows | 149.0.4022.98 | pass | pass | pass | `PASS` | GitHub Actions artifact |
| Mozilla Firefox / Linux | 152.0.4 | pass | pass | pass | `PASS` | GitHub Actions artifact |
| Apple Safari / WebKit / macOS | Safari runner 26.4; local macOS 26.5.2 WebKit | Ed25519 pass; X25519 `TypeError` after process restart | pass | pass in separate WKWebView app processes | `PASS_WITH_FALLBACK` | `webkit-macos.json`; Safari CI rerun pending |

The Chrome result was reproduced independently through Playwright and Selenium.
Edge and Firefox passed on hosted Windows and Linux runners. The local WebKit
harness retained Ed25519 and the non-exportable AES-GCM wrapping key across
separate application processes; X25519 failed on the native path, so WebKit is
eligible only through the encrypted fallback. Stored artifacts contain no key
bytes, profile paths, hostnames, or session identifiers.

## Standards and vendor baseline

- [Web Cryptography Level 2](https://www.w3.org/TR/webcrypto-2/) specifies
  Ed25519, X25519, non-extractable key generation, and explicitly treats
  IndexedDB as an available storage mechanism for `CryptoKey` objects.
- [Web Cryptography API Recommendation](https://www.w3.org/TR/2017/REC-WebCryptoAPI-20170126/#cryptokey-interface) defines the structured clone behavior for a
  `CryptoKey`, including preservation of its `extractable` state and key
  handle.
- [Indexed Database API 3.0](https://www.w3.org/TR/IndexedDB/) defines the
  structured serialization used for values stored by IndexedDB.
- [WebKit Safari 18.4 release notes](https://webkit.org/blog/16574/webkit-features-in-safari-18-4/) record Safari's X25519 WebCrypto support. Runtime evidence, not this
  compatibility statement, remains the acceptance authority.
- [Apple WebDriver documentation](https://developer.apple.com/documentation/safari-developer-tools/webdriver/) states that Safari automation windows are isolated from normal
  browsing data and from other test runs. A second WebDriver session therefore
  cannot be used as Safari process-restart persistence evidence.

## Security interpretation

`extractable=false` is a browser API guarantee, not evidence of hardware-backed
storage. It blocks `exportKey` and `wrapKey`; it does not stop same-origin
malicious JavaScript from asking the browser to sign, derive, or decrypt. The
production design therefore still requires a strict CSP, no third-party script
execution on the enrolled origin, Trusted Types where supported, dependency
integrity controls, device revocation, and key rotation.

The AES-GCM fallback improves raw-key-at-rest handling but is not an XSS
boundary because same-origin script can invoke the stored wrapping key. It may
be accepted only as a documented compatibility fallback, never as stronger
protection than native WebCrypto.

Origin storage can be cleared or evicted. Device enrollment must treat loss of
the local key as device loss and require re-enrollment or recovery from another
trusted device; the Control Plane must not escrow an unwrapped device private
key. The production web origin must also remain stable because WebCrypto and
IndexedDB are origin-scoped.

## Provisional decision

Chrome, Edge/Windows, and Firefox/Linux satisfy both paths. WebKit/macOS
satisfies the bounded encrypted fallback across separate app processes but not
the complete native path. The project-wide decision remains open until the
Safari session rerun completes and a security verdict accepts or rejects that
fallback. Any unsupported browser is downgraded to metadata-only access; there
is no silent plaintext or exportable-key mode.
