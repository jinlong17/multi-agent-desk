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
5. Initialize two distinct Device roots and finish writing each daemon identity,
   server config, cursor key, TLS leaf/certificate key, and temporary CA before
   starting either process. Cross a full wall-clock second after the last of
   those files is frozen, then start a Daemon for each and the matching Control
   Plane; a same-second file ctime/process start is rejected because macOS `ps`
   exposes start time only to whole-second precision. Bootstrap token plaintext may be read only from
   its protected live console and entered into that row's browser. Do not save
   server stdout, browser devtools, screenshots, traces, HAR files, or clipboard
   history containing authentication material. For each row, connect server and
   Daemon stdout/stderr to four distinct owner-only `0600` FIFOs. Give every
   FIFO exactly one `/bin/cat` reader whose stdout/stderr is the same live
   Terminal TTY. Terminal session recording and operator-side screen capture
   must be disabled. Record all six writer/reader PIDs in the manifest. A
   regular-file sink, `tee`, diagnostic collector, or undeclared FIFO holder
   invalidates the row. The writers must keep only their declared SQLite main
   database/sidecars open for writable regular-file FDs; any other writable
   regular FD invalidates the row.

   The machine check is deliberately bounded: it proves the declared
   server/Daemon/`cat` FD graph routes each stream through one FIFO to a live
   TTY and that the declared log roots contain no regular-file sink. It does
   not prove the absence of a PTY-master recorder, Terminal scrollback,
   Terminal application recording, OS screen capture, or other operator-side
   capture. Those surfaces are outside the machine claim and require the exact
   operator confirmations in each journey receipt.

## Freeze the environment

Create an owner-only JSON manifest outside the repository with exactly:

```json
{
  "schemaVersion": 2,
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
      "runtimeRoot": "/absolute/private/acceptance/chrome",
      "deviceRoot": "/absolute/private/acceptance/chrome/device",
      "deviceDatabasePath": "/absolute/private/acceptance/chrome/device/device.db",
      "transferRoot": "/absolute/private/acceptance/chrome/transfers",
      "logRoot": "/absolute/private/acceptance/chrome/logs",
      "evidenceRoot": "/absolute/private/acceptance/chrome/evidence",
      "serverPid": 10001,
      "daemonPid": 10002,
      "serverStdoutFifo": "/absolute/private/acceptance/chrome/logs/server.stdout.fifo",
      "serverStderrFifo": "/absolute/private/acceptance/chrome/logs/server.stderr.fifo",
      "daemonStdoutFifo": "/absolute/private/acceptance/chrome/logs/daemon.stdout.fifo",
      "daemonStderrFifo": "/absolute/private/acceptance/chrome/logs/daemon.stderr.fifo",
      "serverStdoutReaderPid": 10003,
      "serverStderrReaderPid": 10004,
      "daemonStdoutReaderPid": 10005,
      "daemonStderrReaderPid": 10006,
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
      "runtimeRoot": "/absolute/private/acceptance/safari",
      "deviceRoot": "/absolute/private/acceptance/safari/device",
      "deviceDatabasePath": "/absolute/private/acceptance/safari/device/device.db",
      "transferRoot": "/absolute/private/acceptance/safari/transfers",
      "logRoot": "/absolute/private/acceptance/safari/logs",
      "evidenceRoot": "/absolute/private/acceptance/safari/evidence",
      "serverPid": 20001,
      "daemonPid": 20002,
      "serverStdoutFifo": "/absolute/private/acceptance/safari/logs/server.stdout.fifo",
      "serverStderrFifo": "/absolute/private/acceptance/safari/logs/server.stderr.fifo",
      "daemonStdoutFifo": "/absolute/private/acceptance/safari/logs/daemon.stdout.fifo",
      "daemonStderrFifo": "/absolute/private/acceptance/safari/logs/daemon.stderr.fifo",
      "serverStdoutReaderPid": 20003,
      "serverStderrReaderPid": 20004,
      "daemonStdoutReaderPid": 20005,
      "daemonStderrReaderPid": 20006,
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
Every manifest, context, journey, transfer, scan, and final-receipt JSON input
is decoded from raw bytes with fatal UTF-8 handling. Outside JSON strings only
space, tab, CR, and LF are accepted as whitespace; duplicate keys and
`__proto__`, `constructor`, or `prototype` object keys are rejected.
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
7. Confirm Terminal session recording and OS/operator screen capture remained
   disabled for the whole row, then clear the Terminal scrollback that displayed
   the protected live console. Do not stop or replace the declared processes.
8. Run the P2 DB/runtime secret tests and scan the declared DB/log/artifact roots.
   Delete any accidental sensitive artifact and restart the entire row; never
   mark a scan passed after redacting an otherwise invalid run.

Create one exact journey JSON per browser outside the repository. The shared
fields are the browser name; canonical UTC `startedAt`/`finishedAt`; true values
for `bootstrap`, `registration`, `login`, `recovery`, `replacementPasskey`,
`passkeyDelete`, `logout`, `browserReportedSecureConnection`, and
`manualOperatorConfirmed`; and false values for
`tlsWarningBypassed`, `viteProxyUsed`, `corsWorkaroundUsed`, and
`secretArtifactCaptured`, `automationSubstituteUsed`, and
`webdriverSubstituteUsed`, and `terminalRecordingUsed`; each journey also
requires `terminalScrollbackCleared:true`. These last two fields are explicit
operator attestations, not machine-derived PTY-master or Terminal-application
proof. Safari additionally requires:

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

## Machine scan

After both journeys finish, preserve only the exact public Daemon descriptor,
the exact public activation receipt, the manifest, frozen context, each row's
journey JSON, and a sanitized test summary. Delete transient bootstrap
challenge/proof and recovery artifacts. Do not stop or replace the declared
writer/reader processes. Run:

```sh
node scripts/acceptance/p2-browser-receipt.mjs scan \
  --manifest /absolute/private/acceptance/manifest.json \
  --context /absolute/private/acceptance/frozen-context.json \
  --out /absolute/private/acceptance/machine-secret-scan.json
```

The scan is fail-closed over six complete target classes: server database and
sidecars/backups, Device database/runtime, remaining runtime residue, declared
log FIFOs, transfers, and evidence. It applies detectors for bootstrap tokens,
session cookies, CSRF values, recovery codes, WebAuthn ceremony material, and
bootstrap proofs; verifies both SQLite databases logically and by raw bytes;
and records only counts, non-content filesystem metadata inventories, and rule
digests. It never records a secret, a per-file content digest, or a secret
digest. Any unreadable, unstable, replaced, aliased,
hard-linked, overlapping, unsupported, unexpected, or nonzero-finding target
invalidates the complete row. Fix the storage/logging defect and rerun that row
from bootstrap; deleting/redacting a finding does not rescue the run.
The protected acceptance suite executes one complete success path with real
logical SQLite assertions, injects a distinct detector canary into each of the
six target classes, and independently observes only sanitized per-target and
per-secret counts to prove each detector fired even when an allowlist would
also reject the file. The observation contains no matched bytes or raw path.
The suite also rejects unreadable/private-mode failures, unstable or replaced
files, symbolic aliases, hard links, unexpected and empty roots, and
post-snapshot mutation.

This bounded claim excludes the OS Passkey store, browser profiles, the
operator's recovery-code store, generalized inspection of long-term private-key
contents, PTY-master recording, Terminal scrollback/application behavior, OS
screen capture, and other operator-side capture. The latter surfaces are
covered only by the exact journey attestations. The exclusion does not remove
declared roots from marker and metadata scans, or declared writers/readers from
the FIFO-to-live-TTY FD check.

## Finalize

```sh
node scripts/acceptance/p2-browser-receipt.mjs finalize \
  --manifest /absolute/private/acceptance/manifest.json \
  --context /absolute/private/acceptance/frozen-context.json \
  --scan /absolute/private/acceptance/machine-secret-scan.json \
  --chrome-journey /absolute/private/acceptance/chrome-journey.json \
  --safari-journey /absolute/private/acceptance/safari-journey.json \
  --out /absolute/repository/docs/reviews/phase4a-control-plane-core/YYYY-MM-DD-p2-browser-receipt.json
```

Finalization repeats every machine check and rescans every declared target. It
fails if the implementation,
binary, browser build, TLS identity, origin, RP ID, or isolated path changed.
The frozen context and final receipt have exact schemas and canonical
timestamps; each journey must finish before the signed scan starts, and the
scan must finish no later than the finalization observation. A stale,
cross-row, non-`PASS`, mutated, or differently inventoried scan is rejected.
Its `PASS` means machine-verified environment plus explicit human browser/Touch
ID and Terminal-handling attestations; it does not turn the manual journey or
excluded PTY/Terminal/operator surfaces into automated evidence.
Remove temporary CA trust and delete protected ephemeral secrets only after the
receipt is durable and independently reviewed.
