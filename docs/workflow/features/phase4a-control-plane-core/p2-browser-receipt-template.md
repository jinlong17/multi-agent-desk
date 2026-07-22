# P2 macOS browser acceptance receipt template

This is the executable receipt procedure for Phase 4a P2. It is not itself an
acceptance result. A `PASS` receipt is valid only after P2 has one final clean
implementation commit and both real-browser rows complete against binaries
built from that exact commit.

## Frozen rows

| Row | Origin | RP ID | Required browser evidence |
|---|---|---|---|
| Chrome | `https://chrome.localhost:8443` | `chrome.localhost` | current signed Chrome executable; real bootstrap, registration, login, recovery, replacement Passkey, Passkey delete, logout |
| Safari | `https://safari.localhost:9443` | `safari.localhost` | current signed Safari executable; the same real journey plus operator-confirmed Touch ID, `authenticatorAttachment=platform`, and user verification |

The rows use independent server config, database, cursor key, TLS leaf/key, and
Device root. They may share one temporary local CA. Do not share or copy a DB,
cursor key, TLS leaf, Device root, cookie, Passkey, recovery batch, bootstrap
token, challenge, proof, or receipt between rows.

## Preconditions

1. Run on native macOS arm64 after the coordinator creates the final P2
   implementation commit. The feature worktree must be clean.
2. Build `multidesk-server` with the exact full commit injected into
   `controlplane.BuildCommit`; build `multidesk` from the same commit. Example:

   ```sh
   go build -trimpath \
     -ldflags "-X github.com/jinlong17/multi-agent-desk/internal/controlplane.BuildVersion=0.1.0-p2-acceptance -X github.com/jinlong17/multi-agent-desk/internal/controlplane.BuildCommit=$(git rev-parse HEAD)" \
     -o /absolute/private/acceptance/bin/multidesk-server ./cmd/multidesk-server
   go build -trimpath -o /absolute/private/acceptance/bin/multidesk ./cmd/multidesk
   ```

   Do not use `-buildvcs=false`. The collector requires the CLI Go build info
   to contain `vcs.revision=<the exact implementation SHA>` and
   `vcs.modified=false`, in addition to hashing that binary. The server's
   injected `BuildCommit` remains an independent runtime check.

3. Create one temporary local CA and two distinct localhost leaf certificates.
   Chrome's leaf SAN is only `DNS:chrome.localhost`; Safari's is only
   `DNS:safari.localhost`. Trust that CA for the duration of the manual browser
   run, verify both browsers show a normal secure connection, then remove the
   trust entry after finalization. Never use `-k`, ignore a certificate warning,
   disable verification, use a Vite proxy, or add a CORS workaround.
4. Create two owner-only server configs and cursor keys. Bind loopback only
   (`127.0.0.1` or `[::1]`); the frozen rows in this procedure use
   `127.0.0.1:8443` and `127.0.0.1:9443`. Set the exact origins/RP IDs above,
   set `developmentAllowLocalhost:true`, and use distinct databases.
   Each database parent must be `0700`; each database and every existing
   `-wal`, `-shm`, or `-journal` sidecar must be `0600`.
5. Initialize two distinct Device roots, start a Daemon for each, then start
   the matching Control Plane. Bootstrap token plaintext may be read only from
   its protected live console and entered into that row's browser. Do not save
   server stdout, browser devtools, screenshots, traces, HAR files, or clipboard
   history containing authentication material.

## Freeze the environment

Create an owner-only JSON manifest outside the repository with exactly:

```json
{
  "implementationSha": "FULL_40_CHARACTER_LOWERCASE_SHA",
  "cliBinary": "/absolute/private/acceptance/bin/multidesk",
  "rows": [
    {
      "browser": "chrome",
      "origin": "https://chrome.localhost:8443",
      "rpId": "chrome.localhost",
      "serverBinary": "/absolute/private/acceptance/bin/multidesk-server",
      "serverConfig": "/absolute/private/acceptance/chrome/server.json",
      "databasePath": "/absolute/private/acceptance/chrome/server.sqlite",
      "cursorKeyPath": "/absolute/private/acceptance/chrome/cursor.key",
      "tlsLeafCertificate": "/absolute/private/acceptance/chrome/leaf.pem",
      "tlsLeafPrivateKey": "/absolute/private/acceptance/chrome/leaf.key",
      "temporaryCA": "/absolute/private/acceptance/ca/ca.pem",
      "deviceRoot": "/absolute/private/acceptance/chrome/device",
      "browserBundle": "/Applications/Google Chrome.app",
      "browserExecutable": "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    },
    {
      "browser": "safari",
      "origin": "https://safari.localhost:9443",
      "rpId": "safari.localhost",
      "serverBinary": "/absolute/private/acceptance/bin/multidesk-server",
      "serverConfig": "/absolute/private/acceptance/safari/server.json",
      "databasePath": "/absolute/private/acceptance/safari/server.sqlite",
      "cursorKeyPath": "/absolute/private/acceptance/safari/cursor.key",
      "tlsLeafCertificate": "/absolute/private/acceptance/safari/leaf.pem",
      "tlsLeafPrivateKey": "/absolute/private/acceptance/safari/leaf.key",
      "temporaryCA": "/absolute/private/acceptance/ca/ca.pem",
      "deviceRoot": "/absolute/private/acceptance/safari/device",
      "browserBundle": "/Applications/Safari.app",
      "browserExecutable": "/Applications/Safari.app/Contents/MacOS/Safari"
    }
  ]
}
```

With both exact servers running:

```sh
node scripts/acceptance/p2-browser-receipt.mjs collect \
  --manifest /absolute/private/acceptance/manifest.json \
  --out /absolute/private/acceptance/frozen-context.json
```

The collector fails unless the worktree is clean, native OS/architecture and
browser signatures/versions are readable, each bundle's
`CFBundleIdentifier`/`CFBundleExecutable` matches its frozen Chrome or Safari
row, `codesign --verify --strict` passes for both bundle and executable, the
CLI has clean exact-SHA Go VCS build info, server binaries report the final
SHA, certificates chain to only the supplied CA for the exact host, and a
direct loopback TLS request to each `/v1/version` returns that SHA. It also
requires `/usr/bin/security verify-cert` to confirm the leaf is trusted by the
active macOS system trust evaluation without a CA override. Private manifest,
config, database, cursor key, leaf key, Device root, context, and journey files
must be owned by the current user with exact `0600`/`0700` modes. It records
hashes and path digests, never private-key/cursor contents or auth material.
Private server/Device state and binary paths must already be canonical real
paths. The browser bundle/executable fields may use Apple's `/Applications`
system alias; the collector resolves both first and then binds the executable
inside the resolved signed bundle. Cross-row private-state path aliases, hard links, nested/overlapping Device roots, isolated files under
the other row's Device root, repeated public `device_id` values, copied daemon
identity keys, copied cursor keys, and copied TLS private keys are rejected.
Only the public `device_id` is retained from each protected daemon identity.

## Execute each real journey

For each isolated row, in its named browser:

1. Prepare/import the Daemon descriptor and complete initial bootstrap with a
   real Passkey.
2. Save the one-time recovery codes in a protected operator-chosen location;
   do not include them in evidence.
3. Log out, sign in with the Passkey, log out, then consume one recovery code.
4. Register a replacement Passkey and confirm the restricted recovery session
   becomes a normal rotated session and older browser sessions are revoked.
5. Complete recent user verification, delete a non-last Passkey, and log out.
6. For Safari, explicitly observe the system Touch ID prompt, a platform
   authenticator attachment, and user verification. Emulation is invalid.
7. Run the P2 DB/runtime secret tests and scan retained DB/log/artifact outputs.
   Delete any accidental sensitive artifact and restart the entire row; never
   mark a scan passed after redacting an otherwise invalid run.

Create one exact journey JSON per browser outside the repository. The shared
fields are the browser name; canonical UTC `startedAt`/`finishedAt`; true values
for `bootstrap`, `registration`, `login`, `recovery`, `replacementPasskey`,
`passkeyDelete`, `logout`, `browserReportedSecureConnection`, the three
`*SecretScanPassed` fields, and `manualOperatorConfirmed`; and false values for
`tlsWarningBypassed`, `viteProxyUsed`, `corsWorkaroundUsed`, and
`secretArtifactCaptured`, `automationSubstituteUsed`, and
`webdriverSubstituteUsed`. Safari additionally requires:

```json
{
  "authenticatorAttachment": "platform",
  "userVerificationObserved": true,
  "touchIdOperatorConfirmed": true
}
```

The schema rejects unknown fields, so it cannot be used to store raw token,
cookie, CSRF, recovery-code, challenge, proof, or credential material.
WebDriver, browser automation, emulation, and simulated Touch ID are invalid
substitutes for either real-browser row.

## Finalize

```sh
node scripts/acceptance/p2-browser-receipt.mjs finalize \
  --manifest /absolute/private/acceptance/manifest.json \
  --context /absolute/private/acceptance/frozen-context.json \
  --chrome-journey /absolute/private/acceptance/chrome-journey.json \
  --safari-journey /absolute/private/acceptance/safari-journey.json \
  --out /absolute/repository/docs/reviews/phase4a-control-plane-core/YYYY-MM-DD-p2-browser-receipt.json
```

Finalization repeats every machine check and fails if the implementation,
binary, browser build, TLS identity, origin, RP ID, or isolated path changed.
The frozen context and final receipt have exact schemas and canonical
timestamps; each journey must finish no later than the finalization observation.
Its `PASS` means machine-verified environment plus explicit human browser/Touch
ID attestation; it does not turn the manual journey into automated evidence.
Remove temporary CA trust and delete protected ephemeral secrets only after the
receipt is durable and independently reviewed.
