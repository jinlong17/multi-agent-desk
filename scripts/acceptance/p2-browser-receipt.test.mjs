import assert from "node:assert/strict";
import { X509Certificate, createHash } from "node:crypto";
import {
  chmodSync, existsSync, linkSync, mkdtempSync, mkdirSync, readFileSync, readdirSync, realpathSync, renameSync, rmSync, statSync, symlinkSync, unlinkSync, writeFileSync,
} from "node:fs";
import { Agent as HTTPSAgent, createServer as createHTTPSServer, get as httpsGet } from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import test from "node:test";

import {
  assertPrivatePath,
  canonicalJSONStringify,
  detectorRuleDigest,
  decodeBase64URL32,
  finalizeReceipt,
  parseStrictJSON,
  readJSON,
  stableContext,
  validateBrowserBundleMetadata,
  validateDeviceIdentities,
  validateFrozenContext,
  validateJourney,
  validateMachineScanReceipt,
  validateManifest,
  validateProcessBindings,
  liveFIFOHolderInspector,
  liveExclusiveProcessLockProbe,
  liveProcessLockHolderInspector,
  validateReceiptOutputPath,
  validateReceipt,
  validateRowIsolation,
  validateVersionTLSSocket,
  countDetectorMatches,
  databaseAssertions,
  scanEnvironment,
  writeExclusiveJSON,
} from "./p2-browser-receipt.mjs";

const sha = "a".repeat(40);
const digest = (character) => character.repeat(64);
const fingerprint = (octet) => Array.from({ length: 32 }, () => octet).join(":");
const cliBinary = "/private/tmp/mad/bin/multidesk";
// Publicly committed, throwaway localhost-only fixture. It is not a product
// credential and is never written to a receipt or printed by the test.
const testTLSPrivateKey = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC9Xrp8wr0HEQZD
uVaUMjTA6L+gzgp+14DcrBgRKR6LIZ8uqMMO+0EJMgxeLZJ6qFY4/hhC0aGCmP1a
cbm0cVUmt7pKlA53YqUUQ/5Y2KNcsEQxqnhtEa1N05R4z/nvtCiK36dTr58cZBsA
gDhPTjvMUPKqpTw3VC68WdMAlYH52M8ceq2vPSdORxt/7r4VX97umAsGWfMu55u2
DnX6ijpDABq+eq4z1+O1vl9ZmNISTzaTx0GIOK30gXA/FyIxvjzfCKBM/IRKGUuf
OFa+VD7aY1nR98iZjx/OwNaKuOHmwi9HB9Q99sbqoSwzLx1Dc9JX+i420PYGZFBC
yY/Ho7+LAgMBAAECggEAP6T4tDmW4iscmeJOcNw20qbm0Jqu+FZhXskQBaR2OXiB
UWMyu3RCNV72vSg/1K2C3QC5Eqv1xji43Y7fRP/aCHszRyFfg0xKAvefIikdLmen
Y7HRa4bHYiK8AaaUb7Vy8smcKQobRaV3VcHCKxU2D8Mc67FA/a9zTaY6vjWBS4C3
gba00pmTXAbg6TUNmz84ev1zd8dNVubDuybnOORXnk/EVmoRW6/N1JP/q10aDfOK
MGzXoV0sufJv6jix03YntdA7iAqEYkPF6aGkJlJ6fLxVzjEhbvhDsLii9uK7XDqa
8gTzDvezumc/jKjMOQdiEYTaVrSJ4F5aTeudLP/6UQKBgQD30fTzY/OWEsXEKfLd
KZq6ULgB+94p0Se2axK/NK78gU2RUvw9qYYVWtYGJXr8edoZ8hESndExpTFvqJNg
yq0hGW0QC1mi4V5T0vXTSe9W0ofA+/hG/X3f557PIgX97L7lU1N9NAPyIS96VKZu
DOrQdh14mcQ5RrnEW8v6rMXSzQKBgQDDnuB+g+5yRj3lUSBFbhLJDRq/XMk6Do1C
ma8vrIA66cGRpYhYQHqnUmbel57aL5RgBSSTB9LI3dy1GlrL+jWi65Fdz54iLz41
Z5zwqUsGZev6h5S40OJVQPjXjXP8a2FfPMP4teh6ngisXHkBwkpNWWDoJ19ATmmj
Tp5tR1hLtwKBgDl4KRPgU/azd8Vb7QQ4x7b5TRK4s/aCmHEHN5u7vfC0k6Zl1jT+
gSemnwdh3bl7EIb/ydHFY2Pd6S75quPBXJDWcqJL34eUN+m8fGF5PdWmkPDB/fuI
gY5RClUCkN0n78UCo9PfIiMeawI1azsOJ84b9g2nqweVTTMqDo2dT2rpAoGBAMBL
onTbbf8pa1jLycRWcuLuHdf09t46RcQtXNepY5gGB0EMDp5qK+flCbhQJVhnoxxM
kepyq1LHPVlNoemXeThBBvHH0LPb6vQGeXDdiiGs+S6aLqkKtSKHLtZ9d4GvcNV0
31PSRcibJv2AHXeMLQwiCy/K3EhTjGZ7NyNHGdW7AoGAcJy6jFqdm2Z1caUotijY
WPZ2RpuTlyYnrvCNVu97t1XGNYNpG4X0MKg0zX8fSJ55EQuR5OssbYwbdf+F4e3O
JZalCK0gXGwnHc0BRkqGudvLhYbTm3n++boiqVwgoLZhrnJH1EXGhLdqMji/+UfV
/YL3t/tTzS7Mu4+US/NqgvA=
-----END PRIVATE KEY-----`;
const testTLSCertificate = `-----BEGIN CERTIFICATE-----
MIIC8DCCAdgCCQC9RH23xlD85zANBgkqhkiG9w0BAQsFADA6MRIwEAYDVQQDDAls
b2NhbGhvc3QxJDAiBgNVBAoMG011bHRpQWdlbnREZXNrIHRlc3QgZml4dHVyZTAe
Fw0yNjA3MjIyMTQzMjJaFw0zNjA3MTkyMTQzMjJaMDoxEjAQBgNVBAMMCWxvY2Fs
aG9zdDEkMCIGA1UECgwbTXVsdGlBZ2VudERlc2sgdGVzdCBmaXh0dXJlMIIBIjAN
BgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvV66fMK9BxEGQ7lWlDI0wOi/oM4K
fteA3KwYESkeiyGfLqjDDvtBCTIMXi2SeqhWOP4YQtGhgpj9WnG5tHFVJre6SpQO
d2KlFEP+WNijXLBEMap4bRGtTdOUeM/577Qoit+nU6+fHGQbAIA4T047zFDyqqU8
N1QuvFnTAJWB+djPHHqtrz0nTkcbf+6+FV/e7pgLBlnzLuebtg51+oo6QwAavnqu
M9fjtb5fWZjSEk82k8dBiDit9IFwPxciMb483wigTPyEShlLnzhWvlQ+2mNZ0ffI
mY8fzsDWirjh5sIvRwfUPfbG6qEsMy8dQ3PSV/ouNtD2BmRQQsmPx6O/iwIDAQAB
MA0GCSqGSIb3DQEBCwUAA4IBAQCgNORK1kzN2tCyxGwLDhFGwOOQX4JjPXGPlOuy
DdVN22rhJ8/4/MmYeorW1bA38yU1HWQULQ/U9V8wWPrQ8b0SV8mLRu/9q6bpdF7T
q8dFQ0w9jlgZxWqbAgGMAGWd7InXVZAt5qfpHuha1YX9yFpsW6RlMa/CPbWrm/vF
3TpCmat2XWqDjmFjed+ZfBI/IIAdF7VGxSf/KveBdgV77vlPjpFUcH5Fz5uX6MU1
o6M0jSdqDgI2ktarURqSN8ViOQ+qoF3UayeM5qgFDRQLp1G+XKkS+wlCZd1rvaZ3
kEm9UYQia80mw2anQDv00/byzJ335AoVHJ3RQ7VDGj1ES+j9
-----END CERTIFICATE-----`;
const testTLSLeafSHA256 = new X509Certificate(testTLSCertificate).fingerprint256.replaceAll(":", "").toLowerCase();
const row = (browser, prefix) => ({
  browser,
  origin: browser === "chrome" ? "https://chrome.localhost:8443" : "https://safari.localhost:9443",
  rpId: browser === "chrome" ? "chrome.localhost" : "safari.localhost",
  serverBinary: "/private/tmp/mad/bin/multidesk-server",
  serverConfig: `/private/tmp/mad/${prefix}/server.json`,
  databasePath: `/private/tmp/mad/${prefix}/server.sqlite`,
  serverProcessLockPath: `/private/tmp/mad/${prefix}/server.sqlite.process.lock`,
  cursorKeyPath: `/private/tmp/mad/${prefix}/cursor.key`,
  tlsLeafCertificate: `/private/tmp/mad/${prefix}/leaf.pem`,
  tlsLeafPrivateKey: `/private/tmp/mad/${prefix}/leaf.key`,
  temporaryCA: "/private/tmp/mad/ca.pem",
  deviceRoot: `/private/tmp/mad/${prefix}/device`,
  runtimeRoot: `/private/tmp/mad/${prefix}`,
  deviceDatabasePath: `/private/tmp/mad/${prefix}/device/device.db`,
  transferRoot: `/private/tmp/mad/${prefix}/transfers`,
  logRoot: `/private/tmp/mad/${prefix}/logs`,
  evidenceRoot: `/private/tmp/mad/${prefix}/evidence`,
  serverPid: browser === "chrome" ? 101 : 201,
  daemonPid: browser === "chrome" ? 102 : 202,
  serverStdoutFifo: `/private/tmp/mad/${prefix}/logs/server.stdout.fifo`,
  serverStderrFifo: `/private/tmp/mad/${prefix}/logs/server.stderr.fifo`,
  daemonStdoutFifo: `/private/tmp/mad/${prefix}/logs/daemon.stdout.fifo`,
  daemonStderrFifo: `/private/tmp/mad/${prefix}/logs/daemon.stderr.fifo`,
  serverStdoutReaderPid: browser === "chrome" ? 103 : 203,
  serverStderrReaderPid: browser === "chrome" ? 104 : 204,
  daemonStdoutReaderPid: browser === "chrome" ? 105 : 205,
  daemonStderrReaderPid: browser === "chrome" ? 106 : 206,
  browserBundle: browser === "chrome" ? "/Applications/Google Chrome.app" : "/Applications/Safari.app",
  browserExecutable: browser === "chrome" ? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" : "/Applications/Safari.app/Contents/MacOS/Safari",
});

test("version TLS socket validation fails closed for every missing or mismatched boundary", () => {
  const raw = Buffer.from("throwaway peer certificate bytes");
  const expected = createHash("sha256").update(raw).digest("hex");
  const valid = { authorized: true, remoteAddress: "127.0.0.1", getPeerCertificate: () => ({ raw }) };
  assert.equal(validateVersionTLSSocket(valid, expected), valid);
  assert.throws(() => validateVersionTLSSocket(undefined, expected), /missing its TLS socket/u);
  assert.throws(() => validateVersionTLSSocket({ ...valid, authorized: false }, expected), /not authorized/u);
  assert.throws(() => validateVersionTLSSocket({ ...valid, remoteAddress: "192.0.2.1" }, expected), /not a direct loopback/u);
  assert.throws(() => validateVersionTLSSocket({ ...valid, getPeerCertificate: () => ({}) }, expected), /raw bytes are missing/u);
  assert.throws(() => validateVersionTLSSocket(valid, "0".repeat(64)), /differs from the manifest certificate/u);
});

test("Node 24 delayed keep-alive response retains the early validated TLS socket after response.socket is cleared", async (t) => {
  const server = createHTTPSServer({ key: testTLSPrivateKey, cert: testTLSCertificate }, (_request, response) => {
    response.writeHead(200, { "Content-Type": "application/json", Connection: "keep-alive" });
    response.write('{"apiVersion":"v1","data":');
    setTimeout(() => response.end('{"version":"test","commit":"fixture"}}'), 40);
  });
  server.keepAliveTimeout = 1_000;
  await new Promise((accept, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", accept);
  });
  const address = server.address();
  assert.equal(typeof address, "object");
  const agent = new HTTPSAgent({ keepAlive: true, maxSockets: 1 });
  try {
    await t.test("authorized loopback leaf remains provable from the socket captured in the response callback", async () => {
      let earlySocket;
      const body = await new Promise((accept, reject) => {
        const request = httpsGet({
          hostname: "127.0.0.1",
          port: address.port,
          path: "/v1/version",
          servername: "localhost",
          ca: testTLSCertificate,
          rejectUnauthorized: true,
          agent,
        }, (response) => {
          earlySocket = validateVersionTLSSocket(response.socket, testTLSLeafSHA256);
          const chunks = [];
          response.on("data", (chunk) => chunks.push(chunk));
          response.on("end", () => {
            try {
              assert.equal(response.socket, null);
              assert.equal(validateVersionTLSSocket(earlySocket, testTLSLeafSHA256), earlySocket);
              accept(Buffer.concat(chunks).toString("utf8"));
            } catch (error) {
              reject(error);
            }
          });
        });
        request.on("error", reject);
      });
      assert.equal(JSON.parse(body).data.commit, "fixture");
    });

    await t.test("unknown CA rejects the handshake before the response callback", async () => {
      let responseCallbackObserved = false;
      await assert.rejects(new Promise((accept, reject) => {
        const request = httpsGet({
          hostname: "127.0.0.1",
          port: address.port,
          path: "/v1/version",
          servername: "localhost",
          rejectUnauthorized: true,
        }, (response) => {
          responseCallbackObserved = true;
          response.resume();
          accept();
        });
        request.on("error", reject);
      }), /self-signed certificate|unable to verify the first certificate/u);
      assert.equal(responseCallbackObserved, false);
    });
  } finally {
    agent.destroy();
    server.closeAllConnections();
    await new Promise((accept) => server.close(accept));
  }
});

test("browser receipt manifest freezes exact rows, CLI, and independent state", () => {
  const manifest = validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary, rows: [row("safari", "safari"), row("chrome", "chrome")] });
  assert.equal(manifest.cliBinary, cliBinary);
  assert.deepEqual(manifest.rows.map((item) => item.browser), ["chrome", "safari"]);
  assert.throws(() => validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary, rows: [row("chrome", "shared"), row("safari", "shared")] }), /independent/u);
  assert.throws(() => validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary, rows: [{ ...row("chrome", "chrome"), origin: "https://localhost:8443" }, row("safari", "safari")] }), /frozen P2 authority/u);
  assert.throws(() => validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary, rows: [{ ...row("chrome", "chrome"), extra: true }, row("safari", "safari")] }), /missing or unknown/u);
  assert.throws(() => validateManifest({ schemaVersion: 2, implementationSha: sha, rows: [row("chrome", "chrome"), row("safari", "safari")] }), /missing or unknown/u);
  assert.throws(() => validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary, rows: [{ ...row("chrome", "chrome"), serverProcessLockPath: "/private/tmp/mad/chrome/*.lock" }, row("safari", "safari")] }), /must equal databasePath/u);
  const missingLock = row("chrome", "chrome");
  delete missingLock.serverProcessLockPath;
  assert.throws(() => validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary, rows: [missingLock, row("safari", "safari")] }), /missing or unknown/u);
});

const frozenAt = "2030-01-01T00:00:00.000Z";
const observedAt = "2030-01-01T00:20:00.000Z";
const journey = (browser) => ({
  browser,
  startedAt: "2030-01-01T00:01:00.000Z",
  finishedAt: "2030-01-01T00:10:00.000Z",
  bootstrap: true,
  registration: true,
  login: true,
  recovery: true,
  replacementPasskey: true,
  passkeyDelete: true,
  logout: true,
  browserReportedSecureConnection: true,
  tlsWarningBypassed: false,
  viteProxyUsed: false,
  corsWorkaroundUsed: false,
  secretArtifactCaptured: false,
  automationSubstituteUsed: false,
  webdriverSubstituteUsed: false,
  terminalRecordingUsed: false,
  terminalScrollbackCleared: true,
  manualOperatorConfirmed: true,
  ...(browser === "safari" ? { authenticatorAttachment: "platform", userVerificationObserved: true, touchIdOperatorConfirmed: true } : {}),
});

test("browser journey finalization rejects bypass, automation, future completion, and missing Touch ID", () => {
  assert.equal(validateJourney(journey("chrome"), "chrome", frozenAt, observedAt).logout, true);
  assert.equal(validateJourney(journey("safari"), "safari", frozenAt, observedAt).authenticatorAttachment, "platform");
  assert.throws(() => validateJourney({ ...journey("chrome"), tlsWarningBypassed: true }, "chrome", frozenAt, observedAt), /explicitly confirmed false/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), automationSubstituteUsed: true }, "chrome", frozenAt, observedAt), /explicitly confirmed false/u);
  assert.throws(() => validateJourney({ ...journey("safari"), webdriverSubstituteUsed: true }, "safari", frozenAt, observedAt), /explicitly confirmed false/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), terminalRecordingUsed: true }, "chrome", frozenAt, observedAt), /explicitly confirmed false/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), terminalScrollbackCleared: false }, "chrome", frozenAt, observedAt), /explicitly confirmed true/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), registration: false }, "chrome", frozenAt, observedAt), /explicitly confirmed true/u);
  assert.throws(() => validateJourney({ ...journey("safari"), touchIdOperatorConfirmed: false }, "safari", frozenAt, observedAt), /Touch ID/u);
  assert.throws(() => validateJourney({ ...journey("safari"), startedAt: "2029-12-31T23:59:59.000Z" }, "safari", frozenAt, observedAt), /observation window/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), finishedAt: "2030-01-01T00:20:00.001Z" }, "chrome", frozenAt, observedAt), /observation window/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), csrfToken: "forbidden" }, "chrome", frozenAt, observedAt), /missing or unknown/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), databaseSecretScanPassed: true }, "chrome", frozenAt, observedAt), /missing or unknown/u);
});

function contextRow(browser, marker) {
  const pathDigests = {};
  for (const [index, key] of [
    "serverConfig", "databasePath", "serverProcessLockPath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey", "deviceRoot",
    "runtimeRoot", "deviceDatabasePath", "transferRoot", "logRoot", "evidenceRoot", "serverStdoutFifo",
    "serverStderrFifo", "daemonStdoutFifo", "daemonStderrFifo",
  ].entries()) pathDigests[key] = createHash("sha256").update(`${marker}-${index}`).digest("hex");
  const fifo = (name) => ({
    pathDigest: createHash("sha256").update(`${marker}-${name}`).digest("hex"),
    kind: "fifo", device: "1", inode: String(100 + name.length), uid: 501, mode: "0600", regularFile: false,
  });
  const writer = (pid, executable, name) => ({
    pid,
    uid: 501,
    state: "S+",
    startTime: frozenAt,
    executable,
    executableSha256: digest(name === "server" ? "7" : "6"),
    argvDigest: digest(name === "server" ? "5" : "4"),
    image: { pathDigest: digest("3"), device: "1", inode: String(pid + 1000), sha256: digest(name === "server" ? "7" : "6") },
    database: { pathDigest: digest("2"), device: "1", inode: String(pid + 2000) },
    inputs: (name === "server" ? ["server_config", "cursor_key", "tls_leaf_certificate", "tls_leaf_private_key", "temporary_ca"] : ["daemon_identity"]).map((role, index) => ({
      role, pathDigest: digest(String((index + 1) % 10)), device: "1", inode: String(pid + 3000 + index), ctimeNs: String(1_000_000 + index),
    })),
    stdout: fifo(`${name}-stdout`),
    stderr: fifo(`${name}-stderr`),
    ...(name === "server" ? {
      processLock: {
        pathDigest: digest("8"), device: "1", inode: String(pid + 4000), uid: 501, mode: "0600",
        linkCount: 1, size: 0, fd: "9", access: "read_write", lockStatus: "whole_file_write",
        regularFile: true, holderCount: 1,
      },
    } : {}),
  });
  const reader = (pid, name) => ({
    pid,
    uid: 501,
    state: "S+",
    startTime: frozenAt,
    executable: "/bin/cat",
    argvDigest: digest("1"),
    stdin: fifo(name),
    stdoutKind: "tty",
    stderrKind: "tty",
    terminalPathDigest: createHash("sha256").update(`${marker}-terminal`).digest("hex"),
  });
  const basePid = browser === "chrome" ? 100 : 200;
  return {
    browser,
    origin: browser === "chrome" ? "https://chrome.localhost:8443" : "https://safari.localhost:9443",
    rpId: browser === "chrome" ? "chrome.localhost" : "safari.localhost",
    deviceId: `device_${marker.repeat(32)}`,
    privateRuntimeStorageVerified: true,
    isolationPathDigests: pathDigests,
    server: {
      executable: "/private/tmp/mad/bin/multidesk-server",
      sha256: digest("7"),
      versionOutput: `multidesk-server 0.1.0 (${sha})`,
      versionEndpoint: { version: "0.1.0", commit: sha, directLoopbackTLS: true },
    },
    browserExecutable: {
      path: browser === "chrome" ? "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" : "/Applications/Safari.app/Contents/MacOS/Safari",
      sha256: digest(browser === "chrome" ? "8" : "9"),
      bundle: browser === "chrome" ? "Google Chrome.app" : "Safari.app",
      version: { product: "1.2.3", build: "123" },
      codeIdentity: {
        identifier: browser === "chrome" ? "com.google.Chrome" : "com.apple.Safari",
        teamIdentifier: browser === "chrome" ? "EQHXZ8M8AV" : "APPLECOMPUTER",
        cdHash: marker.repeat(40),
      },
    },
    tls: {
      temporaryCAFingerprintSHA256: fingerprint("AA"),
      temporaryCASHA256: digest("a"),
      leafFingerprintSHA256: fingerprint(browser === "chrome" ? "BB" : "CC"),
      leafSubject: `CN=${browser}.localhost`,
      leafIssuer: "CN=temporary-ca",
      leafValidFrom: "Jan  1 00:00:00 2029 GMT",
      leafValidTo: "Jan  1 00:00:00 2031 GMT",
      directValidatedRequest: true,
      macOSSystemTrustVerified: true,
      insecureVerificationBypass: false,
    },
    processes: {
      server: writer(basePid + 1, "/private/tmp/mad/bin/multidesk-server", "server"),
      daemon: writer(basePid + 2, cliBinary, "daemon"),
    },
    consoleReaders: {
      serverStdout: reader(basePid + 3, "server-stdout"),
      serverStderr: reader(basePid + 4, "server-stderr"),
      daemonStdout: reader(basePid + 5, "daemon-stdout"),
      daemonStderr: reader(basePid + 6, "daemon-stderr"),
    },
  };
}

function frozenContext() {
  return {
    schemaVersion: 2,
    status: "FROZEN_PENDING_MANUAL_JOURNEY",
    frozenAt,
    implementationSha: sha,
    manifestSha256: digest("d"),
    receiptToolSha256: digest("e"),
    cleanWorktree: true,
    os: { productVersion: "27.0", buildVersion: "26A123", architecture: "arm64" },
    cli: { path: cliBinary, sha256: digest("0"), vcsRevision: sha, vcsModified: false },
    rows: [contextRow("chrome", "b"), contextRow("safari", "c")],
  };
}

const targetClasses = ["server_database", "device_database", "runtime_residue", "logs", "transfers", "evidence"];
const secretClasses = ["bootstrap_token", "session_cookie", "csrf", "recovery_code", "webauthn_ceremony", "bootstrap_proof"];
const detectorCounts = { bootstrap_token: 4, session_cookie: 2, csrf: 3, recovery_code: 2, webauthn_ceremony: 2, bootstrap_proof: 2 };
const canonicalDigest = (value) => createHash("sha256").update(canonicalJSONStringify(value)).digest("hex");

function machineScan(context = frozenContext()) {
  const scanRow = (browser) => ({
    browser,
    targetClasses: targetClasses.map((name) => ({
      class: name, pathCount: 1, regularFileCount: name === "logs" ? 0 : 1, bytesScanned: name === "logs" ? 0 : 1,
      readErrorCount: 0, unstableFileCount: 0, matchCount: 0, unexpectedPathCount: 0,
      inventorySha256: createHash("sha256").update(`${browser}-${name}`).digest("hex"),
    })),
    secretClasses: secretClasses.map((name) => ({
      class: name, detectorCount: detectorCounts[name], syntheticCanaryCount: 1, matchCount: 0,
      ruleSetSha256: detectorRuleDigest(name),
    })),
    databaseAssertions: {
      sqliteIntegrityCheckPassed: true, serverLogicalQueryCount: 12, deviceLogicalQueryCount: 7,
      serverViolationCount: 0, deviceViolationCount: 0, activeCeremonyCount: 0,
      serverSidecarCount: 0, serverBackupCount: 0, deviceSidecarCount: 0, deviceBackupCount: 0,
      rawByteMatchCount: 0,
    },
    logEvidence: {
      mode: "fifo_to_live_tty", processCount: 6, fdBindingCount: 8, fifoCount: 4, regularSinkCount: 0,
      declaredRootRegularSinkCount: 0, unexpectedPathCount: 0, inventorySha256: digest(browser === "chrome" ? "1" : "2"),
    },
    transferEvidence: {
      publicDescriptorCount: 1, publicReceiptCount: 1, transientChallengeCount: 0, transientProofCount: 0,
      recoveryArtifactCount: 0, unexpectedFileCount: 0, inventorySha256: digest(browser === "chrome" ? "3" : "4"),
    },
    processLockEvidence: {
      boundToDeclaredServer: true, exclusiveWholeFileLock: true, ownerOnly: true, singleLink: true,
      empty: true, holderCount: 1, inventorySha256: digest(browser === "chrome" ? "7" : "8"),
    },
    postSnapshotSha256: digest(browser === "chrome" ? "5" : "6"),
  });
  return {
    schemaVersion: 1,
    status: "PASS",
    policy: "p2-secret-scan-v1",
    startedAt: "2030-01-01T00:11:00.000Z",
    finishedAt: "2030-01-01T00:12:00.000Z",
    implementationSha: context.implementationSha,
    manifestSha256: context.manifestSha256,
    frozenContextSha256: canonicalDigest(context),
    receiptToolSha256: context.receiptToolSha256,
    rows: [scanRow("chrome"), scanRow("safari")],
  };
}

test("frozen context exact schema preserves every field and fails closed on drift or non-JSON values", () => {
  const left = frozenContext();
  const right = structuredClone(left);
  assert.deepEqual(stableContext(left), stableContext(right));
  right.frozenAt = "2030-01-01T00:00:01.000Z";
  assert.notDeepEqual(stableContext(left), stableContext(right));

  const legacyRequestID = frozenContext();
  legacyRequestID.rows[0].server.versionEndpoint.requestId = "legacy-random";
  assert.throws(() => stableContext(legacyRequestID), /missing or unknown/u);

  const nan = frozenContext();
  nan.schemaVersion = Number.NaN;
  assert.throws(() => stableContext(nan), /non-finite/u);

  const secret = frozenContext();
  secret.rows[0].server.versionOutput = "MAD-RC1-secret-material";
  assert.throws(() => stableContext(secret), /forbidden secret marker/u);

  const uppercaseDigest = frozenContext();
  uppercaseDigest.cli.sha256 = "A".repeat(64);
  assert.throws(() => stableContext(uppercaseDigest), /canonical lowercase/u);

  const duplicateDevice = frozenContext();
  duplicateDevice.rows[1].deviceId = duplicateDevice.rows[0].deviceId;
  assert.throws(() => validateFrozenContext(duplicateDevice), /distinct valid public device IDs/u);

  const missingProcessLock = frozenContext();
  delete missingProcessLock.rows[0].processes.server.processLock;
  assert.throws(() => validateFrozenContext(missingProcessLock), /missing or unknown/u);
  const weakenedProcessLock = frozenContext();
  weakenedProcessLock.rows[0].processes.server.processLock.lockStatus = "partial_write";
  assert.throws(() => validateFrozenContext(weakenedProcessLock), /exclusive owner-only empty lock binding/u);
});

test("strict JSON and all six secret detectors fail closed without storing secret values in evidence", () => {
  assert.throws(() => parseStrictJSON(Buffer.from('{"value":1,"value":2}', "utf8"), "duplicate fixture"), /duplicate.*key/u);
  assert.throws(() => parseStrictJSON(Buffer.from([0x7b, 0x22, 0x78, 0x22, 0x3a, 0xc3, 0x28, 0x7d]), "invalid UTF-8 fixture"), /invalid UTF-8/u);
  assert.throws(() => parseStrictJSON(Buffer.from('{"value":\u00a01}', "utf8"), "non-JSON whitespace fixture"), /invalid JSON value/u);
  for (const key of ["__proto__", "constructor", "prototype"]) {
    assert.throws(() => parseStrictJSON(Buffer.from(`{"${key}":true}`, "utf8"), `${key} fixture`), /forbidden object key/u);
  }
  const nullPrototype = parseStrictJSON(Buffer.from('{"safe":true}', "utf8"), "safe object fixture");
  assert.equal(Object.getPrototypeOf(nullPrototype), null);
  const canaries = Buffer.from([
    "P2-BOOTSTRAP-CANARY-TEST", "P2-SESSION-COOKIE-CANARY-TEST", "P2-CSRF-CANARY-TEST",
    "P2-RECOVERY-CANARY-TEST", "P2-WEBAUTHN-CANARY-TEST", "P2-BOOTSTRAP-PROOF-CANARY-TEST",
  ].join("\n"));
  assert.deepEqual(countDetectorMatches(canaries), {
    bootstrap_token: 1,
    session_cookie: 1,
    csrf: 1,
    recovery_code: 1,
    webauthn_ceremony: 1,
    bootstrap_proof: 1,
  });
  assert.equal(countDetectorMatches(Buffer.from("Bootstrap token (shown once; expires in 10 minutes): " + "A".repeat(43))).bootstrap_token, 1);
  const context = frozenContext();
  const scan = machineScan(context);
  assert.equal(JSON.stringify(validateMachineScanReceipt(scan, context)).includes("CANARY"), false);
  assert.equal(decodeBase64URL32(Buffer.alloc(32).toString("base64url"), "32-byte fixture").length, 43);
  assert.throws(() => decodeBase64URL32("A".repeat(42), "truncated fixture"), /32 bytes/u);
});

test("machine scan binding rejects findings, stale context, cross-row evidence, and post-scan mutation", () => {
  const context = frozenContext();
  const scan = machineScan(context);
  assert.equal(validateMachineScanReceipt(scan, context).status, "PASS");

  const finding = structuredClone(scan);
  finding.rows[0].targetClasses[0].matchCount = 1;
  assert.throws(() => validateMachineScanReceipt(finding, context), /zero-finding/u);

  const stale = structuredClone(scan);
  stale.frozenContextSha256 = digest("0");
  assert.throws(() => validateMachineScanReceipt(stale, context), /stale or cross-environment/u);

  const crossRow = structuredClone(scan);
  crossRow.rows.reverse();
  assert.throws(() => validateMachineScanReceipt(crossRow, context), /order\/browser mismatch/u);

  const missingProcessLockEvidence = structuredClone(scan);
  delete missingProcessLockEvidence.rows[0].processLockEvidence;
  assert.throws(() => validateMachineScanReceipt(missingProcessLockEvidence, context), /missing or unknown/u);
  const nonExclusiveLock = structuredClone(scan);
  nonExclusiveLock.rows[0].processLockEvidence.exclusiveWholeFileLock = false;
  assert.throws(() => validateMachineScanReceipt(nonExclusiveLock, context), /process lock evidence is incomplete/u);
  const serverBackup = structuredClone(scan);
  serverBackup.rows[0].databaseAssertions.serverBackupCount = 1;
  assert.throws(() => validateMachineScanReceipt(serverBackup, context), /logical database assertions did not pass/u);

  const mutated = structuredClone(scan);
  mutated.rows[1].postSnapshotSha256 = digest("f");
  assert.throws(
    () => finalizeReceipt(context, structuredClone(context), journey("chrome"), journey("safari"), scan, mutated, observedAt),
    /changed after the signed scan/u,
  );

  const preJourney = structuredClone(scan);
  preJourney.startedAt = "2030-01-01T00:09:59.000Z";
  assert.throws(
    () => finalizeReceipt(context, structuredClone(context), journey("chrome"), journey("safari"), preJourney, structuredClone(preJourney), observedAt),
    /must start after both manual journeys/u,
  );
});

function createMigratedSQLite(path, kind) {
  const directory = join(import.meta.dirname, "../..", "migrations", kind);
  const migrations = readdirSync(directory).filter((name) => /^\d{4}_.+\.sql$/u.test(name)).sort();
  const ledger = `CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY,name TEXT NOT NULL UNIQUE,checksum TEXT NOT NULL,applied_at TEXT NOT NULL);`;
  let result = spawnSync("/usr/bin/sqlite3", [path, ledger], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  migrations.forEach((name, index) => {
    const sql = readFileSync(join(directory, name), "utf8");
    const checksum = createHash("sha256").update(sql).digest("hex");
    result = spawnSync("/usr/bin/sqlite3", ["--", path, `${sql}\nINSERT INTO schema_migrations VALUES(${index + 1},'${name}','${checksum}','2030-01-01T00:00:00Z'); PRAGMA user_version=${index + 1};`], { encoding: "utf8" });
    assert.equal(result.status, 0, `${name}: ${result.stderr}`);
  });
}

test("machine database assertions execute against closed server and Device SQLite state", { skip: process.platform !== "darwin" }, () => {
  const temporary = mkdtempSync(join(tmpdir(), "mad-p2-logical-db-"));
  const root = realpathSync.native(temporary);
  try {
    chmodSync(root, 0o700);
    const deviceRoot = join(root, "device");
    mkdirSync(deviceRoot, { mode: 0o700 });
    const serverPath = join(root, "server.sqlite");
    const devicePath = join(deviceRoot, "device.db");
    const assertion = { formatVersion: 1, keyRevision: 1, recordRevision: 1, status: "pending", sealedAt: "2030-01-01T00:00:00Z" };
    const assertionJSON = canonicalJSONStringify(assertion);
    const storageAssertionDigest = createHash("sha256").update(assertionJSON).digest("base64url");
    const capabilities = ["mad.v1.metadata.read"];
    const deviceId = "018f47a0-7b1c-7cc2-8000-000000000003";
    const signingKey = Buffer.alloc(32, 1);
    const exchangeKey = Buffer.alloc(32, 2);
    const frame = (fields) => Buffer.concat(fields.flatMap((field) => {
      const value = Buffer.isBuffer(field) ? field : Buffer.from(field);
      const length = Buffer.alloc(4); length.writeUInt32BE(value.length); return [length, value];
    }));
    const pinDigest = createHash("sha256").update(frame(["multidesk-device-pin-v1", deviceId, signingKey, exchangeKey])).digest("base64url");
    const descriptor = {
      version: 1,
      serverOrigin: "https://chrome.localhost:8443",
      anchor: {
        deviceId, kind: "daemon", name: "fixture daemon", platform: "darwin",
        architecture: "arm64", clientVersion: "0.1.0", signingPublicKey: signingKey.toString("base64url"), exchangePublicKey: exchangeKey.toString("base64url"),
        signingKeyDigest: createHash("sha256").update(signingKey).digest("base64url"), exchangeKeyDigest: createHash("sha256").update(exchangeKey).digest("base64url"), pinDigest,
        storageMode: "portable_vault_v1", keyEnvelopeAssertion: assertion, capabilities,
      },
    };
    const publicReceipt = {
      version: 1,
      type: "bootstrap_commit_receipt",
      ceremonyId: "018f47a0-7b1c-7cc2-8000-000000000001",
      serverOrigin: "https://chrome.localhost:8443",
      userId: "018f47a0-7b1c-7cc2-8000-000000000002",
      anchorDeviceId: deviceId,
      signingKeyDigest: descriptor.anchor.signingKeyDigest,
      exchangeKeyDigest: descriptor.anchor.exchangeKeyDigest,
      storageMode: "portable_vault_v1",
      storageAssertionDigest,
      signingProofDigest: "A".repeat(43),
      exchangeProofDigest: "A".repeat(43),
      activatedAt: "2030-01-01T00:10:00Z",
    };
    const receiptBytes = Buffer.from(JSON.stringify(publicReceipt));
    const receiptHex = receiptBytes.toString("hex");
    const receiptDigestHex = createHash("sha256").update(receiptBytes).digest("hex");
    const zeroHex = Buffer.alloc(32).toString("hex");
    const signingKeyHex = signingKey.toString("hex");
    const exchangeKeyHex = exchangeKey.toString("hex");
    const signingDigestHex = Buffer.from(descriptor.anchor.signingKeyDigest, "base64url").toString("hex");
    const exchangeDigestHex = Buffer.from(descriptor.anchor.exchangeKeyDigest, "base64url").toString("hex");
    const pinDigestHex = Buffer.from(pinDigest, "base64url").toString("hex");
    const assertionHex = Buffer.from(assertionJSON).toString("hex");
    const capabilitiesHex = Buffer.from(JSON.stringify(capabilities)).toString("hex");
    const storageDigestHex = Buffer.from(storageAssertionDigest, "base64url").toString("hex");
    createMigratedSQLite(serverPath, "server");
    const serverSQL = `
      INSERT INTO server_metadata VALUES(1,'018f47a0-7b1c-7cc2-8000-000000000099','2030-01-01T00:00:00.000000Z','2030-01-01T00:00:00.000000Z');
      INSERT INTO bootstrap_state VALUES(1,NULL,NULL,1,'2030-01-01T00:00:00.000000Z','2030-01-01T00:10:00.000000Z');
      INSERT INTO users VALUES('018f47a0-7b1c-7cc2-8000-000000000002',1,X'01','fixture',1,'2030-01-01T00:10:00.000000Z','2030-01-01T00:10:00.000000Z');
      INSERT INTO anchor_devices VALUES('${deviceId}','daemon','fixture daemon','darwin','arm64','0.1.0',X'${signingKeyHex}',X'${exchangeKeyHex}',X'${signingDigestHex}',X'${exchangeDigestHex}',X'${pinDigestHex}','portable_vault_v1',CAST(X'${assertionHex}' AS TEXT),X'${storageDigestHex}',CAST(X'${capabilitiesHex}' AS TEXT),'active',1,1,'2030-01-01T00:10:00.000000Z','2030-01-01T00:10:00.000000Z');
      INSERT INTO recovery_batches VALUES('018f47a0-7b1c-7cc2-8000-000000000010','018f47a0-7b1c-7cc2-8000-000000000002','active','2030-01-01T00:10:00.000000Z',NULL);
      ${Array.from({ length: 10 }, (_, index) => `INSERT INTO recovery_codes VALUES('018f47a0-7b1c-7cc2-8${String(index).padStart(3, "0")}-00000000001${index}','018f47a0-7b1c-7cc2-8000-000000000010','018f47a0-7b1c-7cc2-8000-000000000002',${index + 1},zeroblob(16),zeroblob(32),'${index === 0 ? "consumed" : "active"}','2030-01-01T00:10:00.000000Z',${index === 0 ? "'2030-01-01T00:11:00.000000Z'" : "NULL"});`).join("\n")}
      INSERT INTO bootstrap_receipts VALUES('${publicReceipt.ceremonyId}','${publicReceipt.userId}','${publicReceipt.anchorDeviceId}',CAST(X'${receiptHex}' AS TEXT),X'${receiptDigestHex}','2030-01-01T00:10:00.000000Z');
    `;
    const serverCreated = spawnSync("/usr/bin/sqlite3", [serverPath, serverSQL], { encoding: "utf8" });
    assert.equal(serverCreated.status, 0, serverCreated.stderr);

    createMigratedSQLite(devicePath, "device");
    const deviceSQL = `
      INSERT INTO controlplane_id_mappings VALUES('device','remote_identity_00000000000000000000000000000000','${publicReceipt.anchorDeviceId}','2030-01-01T00:00:00Z','2030-01-01T00:10:00Z');
      INSERT INTO remote_device_identities VALUES('remote_identity_00000000000000000000000000000000','${publicReceipt.serverOrigin}','${publicReceipt.anchorDeviceId}',X'${signingKeyHex}',X'${exchangeKeyHex}',X'${signingDigestHex}',X'${exchangeDigestHex}',1,2,'active','aes-256-gcm',zeroblob(12),zeroblob(17),'aes-256-gcm',zeroblob(12),zeroblob(48),zeroblob(32),zeroblob(32),CAST(X'${receiptHex}' AS TEXT),X'${receiptDigestHex}',NULL,'2030-01-01T00:00:00Z','2030-01-01T00:10:00Z');
    `;
    const deviceCreated = spawnSync("/usr/bin/sqlite3", [devicePath, deviceSQL], { encoding: "utf8" });
    assert.equal(deviceCreated.status, 0, deviceCreated.stderr);
    chmodSync(serverPath, 0o600);
    chmodSync(devicePath, 0o600);

    const dbRow = {
      browser: "chrome", origin: "https://chrome.localhost:8443", databasePath: serverPath,
      deviceDatabasePath: devicePath, deviceRoot,
    };
    const transfer = { descriptor: { value: descriptor, bytes: Buffer.from(JSON.stringify(descriptor)) }, receipt: { value: publicReceipt, bytes: receiptBytes } };
    const assertions = databaseAssertions(dbRow, transfer);
    assert.equal(assertions.sqliteIntegrityCheckPassed, true);
    assert.equal(assertions.serverViolationCount, 0);
    assert.equal(assertions.deviceViolationCount, 0);
    assert.equal(assertions.activeCeremonyCount, 0);

    const tamperedDescriptor = structuredClone(transfer);
    tamperedDescriptor.descriptor.value.anchor.name = "tampered";
    assert.ok(databaseAssertions(dbRow, tamperedDescriptor).serverViolationCount > 0);
    for (const [field, value, pattern] of [
      ["signingKeyDigest", Buffer.alloc(32, 9).toString("base64url"), /public key digest binding/u],
      ["pinDigest", Buffer.alloc(32, 8).toString("base64url"), /device pin binding/u],
    ]) {
      const tampered = structuredClone(transfer);
      tampered.descriptor.value.anchor[field] = value;
      assert.throws(() => databaseAssertions(dbRow, tampered), pattern);
    }
    const assertionTamper = structuredClone(transfer);
    assertionTamper.descriptor.value.anchor.keyEnvelopeAssertion.recordRevision = 2;
    assert.ok(databaseAssertions(dbRow, assertionTamper).serverViolationCount > 0);
    const capabilityTamper = structuredClone(transfer);
    capabilityTamper.descriptor.value.anchor.capabilities = ["unknown.capability"];
    assert.throws(() => databaseAssertions(dbRow, capabilityTamper), /capabilities are not/u);

    const serverMigration = join(import.meta.dirname, "../..", "migrations", "server", "0001_server_foundation.sql");
    let mutation = spawnSync("/usr/bin/sqlite3", [serverPath, "UPDATE schema_migrations SET checksum=lower(hex(randomblob(32))) WHERE version=1"], { encoding: "utf8" });
    assert.equal(mutation.status, 0, mutation.stderr);
    assert.ok(databaseAssertions(dbRow, transfer).serverViolationCount > 0);
    const originalChecksum = createHash("sha256").update(readFileSync(serverMigration)).digest("hex");
    mutation = spawnSync("/usr/bin/sqlite3", [serverPath, `UPDATE schema_migrations SET checksum='${originalChecksum}' WHERE version=1`], { encoding: "utf8" });
    assert.equal(mutation.status, 0, mutation.stderr);

    mutation = spawnSync("/usr/bin/sqlite3", [devicePath, "UPDATE controlplane_id_mappings SET server_id='018f47a0-7b1c-7cc2-8000-000000000004' WHERE entity_type='device'"], { encoding: "utf8" });
    assert.equal(mutation.status, 0, mutation.stderr);
    assert.ok(databaseAssertions(dbRow, transfer).deviceViolationCount > 0);
    mutation = spawnSync("/usr/bin/sqlite3", [devicePath, `UPDATE controlplane_id_mappings SET server_id='${publicReceipt.anchorDeviceId}' WHERE entity_type='device'`], { encoding: "utf8" });
    assert.equal(mutation.status, 0, mutation.stderr);

    const insecureRunner = (binary, args, options) => {
      const modified = [...args];
      modified[modified.length - 1] = modified.at(-1).replace("PRAGMA secure_delete=ON", "PRAGMA secure_delete=OFF");
      return spawnSync(binary, modified, options);
    };
    assert.throws(() => databaseAssertions(dbRow, transfer, { sqliteRunner: insecureRunner }), /secure_delete could not be enabled/u);

    let replaced = false;
    const originalServerDatabase = readFileSync(serverPath);
    const replacingRunner = (binary, args, options) => {
      const result = spawnSync(binary, args, options);
      if (!replaced) {
        replaced = true;
        rmSync(serverPath);
        writeFileSync(serverPath, "clean replacement", { mode: 0o600 });
      }
      return result;
    };
    assert.throws(() => databaseAssertions(dbRow, transfer, { sqliteRunner: replacingRunner }), /identity changed/u);
    rmSync(serverPath);
    writeFileSync(serverPath, originalServerDatabase, { mode: 0o600 });
    assert.equal(databaseAssertions(dbRow, transfer).serverViolationCount, 0);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("browser bundle metadata rejects swapped rows and executable paths", () => {
  const chrome = row("chrome", "chrome");
  const chromeMetadata = { identifier: "com.google.Chrome", executable: "Google Chrome", product: "1", build: "2" };
  assert.equal(validateBrowserBundleMetadata(chrome, chromeMetadata).identifier, "com.google.Chrome");
  assert.throws(() => validateBrowserBundleMetadata(chrome, { ...chromeMetadata, identifier: "com.apple.Safari", executable: "Safari" }), /frozen browser authority/u);
  assert.throws(() => validateBrowserBundleMetadata(chrome, chromeMetadata, chrome.browserBundle, "/Applications/Safari.app/Contents/MacOS/Safari"), /does not match/u);
});

function makeIsolationFixture() {
  const temporary = mkdtempSync(join(tmpdir(), "mad-p2-isolation-"));
  const root = realpathSync.native(temporary);
  const shared = join(root, "shared");
  mkdirSync(shared, { recursive: true, mode: 0o700 });
  const serverBinary = join(shared, "multidesk-server");
  const temporaryCA = join(shared, "ca.pem");
  writeFileSync(serverBinary, "server", { mode: 0o600 });
  writeFileSync(temporaryCA, "ca", { mode: 0o600 });
  const rows = ["chrome", "safari"].map((browser) => {
    const base = join(root, browser);
    const bundle = join(shared, browser === "chrome" ? "Google Chrome.app" : "Safari.app");
    const macOS = join(bundle, "Contents", "MacOS");
    const deviceRoot = join(base, "device");
    const transferRoot = join(base, "transfers");
    const logRoot = join(base, "logs");
    const evidenceRoot = join(base, "evidence");
    mkdirSync(macOS, { recursive: true, mode: 0o700 });
    mkdirSync(base, { recursive: true, mode: 0o700 });
    mkdirSync(deviceRoot, { mode: 0o700 });
    mkdirSync(transferRoot, { mode: 0o700 });
    mkdirSync(logRoot, { mode: 0o700 });
    mkdirSync(evidenceRoot, { mode: 0o700 });
    writeFileSync(join(deviceRoot, "daemon.identity.json"), JSON.stringify({
      schema_version: 1,
      device_id: `device_${(browser === "chrome" ? "a" : "b").repeat(32)}`,
      private_key: Buffer.alloc(64, browser === "chrome" ? 1 : 2).toString("base64"),
    }), { mode: 0o600 });
    const values = {
      serverConfig: join(base, "server.json"),
      databasePath: join(base, "server.sqlite"),
      serverProcessLockPath: join(base, "server.sqlite.process.lock"),
      cursorKeyPath: join(base, "cursor.key"),
      tlsLeafCertificate: join(base, "leaf.pem"),
      tlsLeafPrivateKey: join(base, "leaf.key"),
      deviceDatabasePath: join(deviceRoot, "device.db"),
      browserExecutable: join(macOS, browser === "chrome" ? "Google Chrome" : "Safari"),
    };
    for (const [key, path] of Object.entries(values)) writeFileSync(path, key === "serverProcessLockPath" ? "" : `${browser}-${key}`, { mode: 0o600 });
    const fifoValues = {
      serverStdoutFifo: join(logRoot, "server.stdout.fifo"),
      serverStderrFifo: join(logRoot, "server.stderr.fifo"),
      daemonStdoutFifo: join(logRoot, "daemon.stdout.fifo"),
      daemonStderrFifo: join(logRoot, "daemon.stderr.fifo"),
    };
    for (const path of Object.values(fifoValues)) {
      const created = spawnSync("/usr/bin/mkfifo", [path], { encoding: "utf8" });
      assert.equal(created.status, 0, created.stderr);
      chmodSync(path, 0o600);
    }
    return {
      ...row(browser, browser),
      serverBinary,
      temporaryCA,
      ...values,
      ...fifoValues,
      deviceRoot,
      runtimeRoot: base,
      transferRoot,
      logRoot,
      evidenceRoot,
      browserBundle: bundle,
    };
  });
  return { root, rows, cleanup: () => rmSync(root, { recursive: true, force: true }) };
}

function populateScanRow(row, seed) {
  const assertion = { formatVersion: 1, keyRevision: 1, recordRevision: 1, status: "pending", sealedAt: "2030-01-01T00:00:00Z" };
  const assertionJSON = canonicalJSONStringify(assertion);
  const storageAssertionDigest = createHash("sha256").update(assertionJSON).digest("base64url");
  const capabilities = ["mad.v1.metadata.read"];
  const anchorDeviceId = `018f47a0-7b1c-7cc2-8000-00000000000${seed + 3}`;
  const signingKey = Buffer.alloc(32, seed + 1);
  const exchangeKey = Buffer.alloc(32, seed + 11);
  const frame = (fields) => Buffer.concat(fields.flatMap((field) => {
    const value = Buffer.isBuffer(field) ? field : Buffer.from(field);
    const length = Buffer.alloc(4); length.writeUInt32BE(value.length); return [length, value];
  }));
  const pinDigest = createHash("sha256").update(frame(["multidesk-device-pin-v1", anchorDeviceId, signingKey, exchangeKey])).digest("base64url");
  const descriptor = {
    version: 1,
    serverOrigin: row.origin,
    anchor: {
      deviceId: anchorDeviceId, kind: "daemon", name: `fixture daemon ${seed}`, platform: "darwin", architecture: "arm64", clientVersion: "0.1.0",
      signingPublicKey: signingKey.toString("base64url"), exchangePublicKey: exchangeKey.toString("base64url"),
      signingKeyDigest: createHash("sha256").update(signingKey).digest("base64url"), exchangeKeyDigest: createHash("sha256").update(exchangeKey).digest("base64url"), pinDigest,
      storageMode: "portable_vault_v1", keyEnvelopeAssertion: assertion, capabilities,
    },
  };
  const receipt = {
    version: 1, type: "bootstrap_commit_receipt",
    ceremonyId: `018f47a0-7b1c-7cc2-8000-00000000001${seed}`,
    serverOrigin: row.origin,
    userId: `018f47a0-7b1c-7cc2-8000-00000000002${seed}`,
    anchorDeviceId,
    signingKeyDigest: descriptor.anchor.signingKeyDigest,
    exchangeKeyDigest: descriptor.anchor.exchangeKeyDigest,
    storageMode: "portable_vault_v1", storageAssertionDigest,
    signingProofDigest: "A".repeat(43), exchangeProofDigest: "A".repeat(43),
    activatedAt: "2030-01-01T00:10:00Z",
  };
  const receiptBytes = Buffer.from(JSON.stringify(receipt));
  const receiptHex = receiptBytes.toString("hex");
  const receiptDigestHex = createHash("sha256").update(receiptBytes).digest("hex");
  const signingKeyHex = signingKey.toString("hex");
  const exchangeKeyHex = exchangeKey.toString("hex");
  const signingDigestHex = Buffer.from(descriptor.anchor.signingKeyDigest, "base64url").toString("hex");
  const exchangeDigestHex = Buffer.from(descriptor.anchor.exchangeKeyDigest, "base64url").toString("hex");
  const pinDigestHex = Buffer.from(pinDigest, "base64url").toString("hex");
  const assertionHex = Buffer.from(assertionJSON).toString("hex");
  const capabilitiesHex = Buffer.from(JSON.stringify(capabilities)).toString("hex");
  const storageDigestHex = Buffer.from(storageAssertionDigest, "base64url").toString("hex");
  rmSync(row.databasePath);
  rmSync(row.deviceDatabasePath);
  createMigratedSQLite(row.databasePath, "server");
  const serverSQL = `
    INSERT INTO server_metadata VALUES(1,'018f47a0-7b1c-7cc2-8000-000000000099','2030-01-01T00:00:00.000000Z','2030-01-01T00:00:00.000000Z');
    INSERT INTO bootstrap_state VALUES(1,NULL,NULL,1,'2030-01-01T00:00:00.000000Z','2030-01-01T00:10:00.000000Z');
    INSERT INTO users VALUES('${receipt.userId}',1,X'01','fixture',1,'2030-01-01T00:10:00.000000Z','2030-01-01T00:10:00.000000Z');
    INSERT INTO anchor_devices VALUES('${anchorDeviceId}','daemon','fixture daemon ${seed}','darwin','arm64','0.1.0',X'${signingKeyHex}',X'${exchangeKeyHex}',X'${signingDigestHex}',X'${exchangeDigestHex}',X'${pinDigestHex}','portable_vault_v1',CAST(X'${assertionHex}' AS TEXT),X'${storageDigestHex}',CAST(X'${capabilitiesHex}' AS TEXT),'active',1,1,'2030-01-01T00:10:00.000000Z','2030-01-01T00:10:00.000000Z');
    INSERT INTO recovery_batches VALUES('018f47a0-7b1c-7cc2-8000-000000000030','${receipt.userId}','active','2030-01-01T00:10:00.000000Z',NULL);
    ${Array.from({ length: 10 }, (_, index) => `INSERT INTO recovery_codes VALUES('018f47a0-7b1c-7cc2-8${String(index).padStart(3, "0")}-00000000003${index}','018f47a0-7b1c-7cc2-8000-000000000030','${receipt.userId}',${index + 1},zeroblob(16),zeroblob(32),'${index === 0 ? "consumed" : "active"}','2030-01-01T00:10:00.000000Z',${index === 0 ? "'2030-01-01T00:11:00.000000Z'" : "NULL"});`).join("\n")}
    INSERT INTO bootstrap_receipts VALUES('${receipt.ceremonyId}','${receipt.userId}','${anchorDeviceId}',CAST(X'${receiptHex}' AS TEXT),X'${receiptDigestHex}','2030-01-01T00:10:00.000000Z');
  `;
  let result = spawnSync("/usr/bin/sqlite3", [row.databasePath, serverSQL], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  createMigratedSQLite(row.deviceDatabasePath, "device");
  const deviceSQL = `
    INSERT INTO controlplane_id_mappings VALUES('device','remote_identity_${String(seed).padStart(32, "0")}','${anchorDeviceId}','2030-01-01T00:00:00Z','2030-01-01T00:10:00Z');
    INSERT INTO remote_device_identities VALUES('remote_identity_${String(seed).padStart(32, "0")}','${row.origin}','${anchorDeviceId}',X'${signingKeyHex}',X'${exchangeKeyHex}',X'${signingDigestHex}',X'${exchangeDigestHex}',1,2,'active','aes-256-gcm',zeroblob(12),zeroblob(17),'aes-256-gcm',zeroblob(12),zeroblob(48),zeroblob(32),zeroblob(32),CAST(X'${receiptHex}' AS TEXT),X'${receiptDigestHex}',NULL,'2030-01-01T00:00:00Z','2030-01-01T00:10:00Z');
  `;
  result = spawnSync("/usr/bin/sqlite3", [row.deviceDatabasePath, deviceSQL], { encoding: "utf8" });
  assert.equal(result.status, 0, result.stderr);
  chmodSync(row.databasePath, 0o600);
  chmodSync(row.deviceDatabasePath, 0o600);
  writeFileSync(join(row.transferRoot, "descriptor.json"), JSON.stringify(descriptor), { mode: 0o600 });
  writeFileSync(join(row.transferRoot, "receipt.json"), receiptBytes, { mode: 0o600 });
}

function cleanDatabaseAssertions() {
  return {
    sqliteIntegrityCheckPassed: true, serverLogicalQueryCount: 12, deviceLogicalQueryCount: 7,
    serverViolationCount: 0, deviceViolationCount: 0, activeCeremonyCount: 0,
    serverSidecarCount: 0, serverBackupCount: 0, deviceSidecarCount: 0, deviceBackupCount: 0, rawByteMatchCount: 0,
  };
}

async function makeScanEnvironmentFixture() {
  const fixture = makeIsolationFixture();
  fixture.rows.forEach(populateScanRow);
  const manifest = validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary: fixture.rows[0].serverBinary, rows: fixture.rows });
  const resolved = await validateRowIsolation(manifest.rows);
  const records = new Map();
  const holderMap = new Map();
  const lockHolderMap = new Map();
  const fifoRecord = (path, access) => {
    const info = statSync(path, { bigint: true });
    return { kind: "FIFO", access, device: info.dev.toString(), inode: info.ino.toString(), path };
  };
  const regularRecord = (path, fd, access = "r", lockStatus) => {
    const info = statSync(path, { bigint: true });
    return { fd, access, kind: "REG", device: info.dev.toString(), inode: info.ino.toString(), path, ...(lockStatus ? { lockStatus } : {}) };
  };
  const tty = { kind: "CHR", access: "u", device: "1", inode: "1", path: "/dev/ttys001" };
  for (const row of manifest.rows) {
    const paths = resolved.get(row.browser);
    records.set(row.serverPid, { executable: paths.serverBinary, argv: [paths.serverBinary, "--config", paths.serverConfig], openFiles: [regularRecord(paths.serverBinary, "txt"), regularRecord(paths.databasePath, "7", "u"), regularRecord(paths.serverProcessLockPath, "9", "u", "W")], fds: new Map([["1", fifoRecord(paths.serverStdoutFifo, "w")], ["2", fifoRecord(paths.serverStderrFifo, "w")]]) });
    records.set(row.daemonPid, { executable: paths.serverBinary, argv: [paths.serverBinary, "daemon", "serve", "--root", paths.deviceRoot], openFiles: [regularRecord(paths.serverBinary, "txt"), regularRecord(paths.deviceDatabasePath, "8", "u")], fds: new Map([["1", fifoRecord(paths.daemonStdoutFifo, "w")], ["2", fifoRecord(paths.daemonStderrFifo, "w")]]) });
    for (const [pid, fifo] of [[row.serverStdoutReaderPid, paths.serverStdoutFifo], [row.serverStderrReaderPid, paths.serverStderrFifo], [row.daemonStdoutReaderPid, paths.daemonStdoutFifo], [row.daemonStderrReaderPid, paths.daemonStderrFifo]]) {
      records.set(pid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(fifo, "r")], ["1", tty], ["2", tty]]) });
    }
    holderMap.set(paths.serverStdoutFifo, [row.serverPid, row.serverStdoutReaderPid]);
    holderMap.set(paths.serverStderrFifo, [row.serverPid, row.serverStderrReaderPid]);
    holderMap.set(paths.daemonStdoutFifo, [row.daemonPid, row.daemonStdoutReaderPid]);
    holderMap.set(paths.daemonStderrFifo, [row.daemonPid, row.daemonStderrReaderPid]);
    lockHolderMap.set(paths.serverProcessLockPath, [{ pid: row.serverPid, fd: "9", access: "u", lockStatus: "W" }]);
  }
  const processInspector = async (pid) => ({ pid, uid: process.getuid(), state: "S+", startTime: frozenAt, ...records.get(pid) });
  const fifoHolderInspector = async (path) => [...holderMap.get(path)].sort((left, right) => left - right);
  const processLockHolderInspector = async (path) => structuredClone(lockHolderMap.get(path));
  const context = frozenContext();
  context.manifestSha256 = canonicalDigest(manifest);
  context.receiptToolSha256 = createHash("sha256").update(readFileSync(join(import.meta.dirname, "p2-browser-receipt.mjs"))).digest("hex");
  context.cli = { path: manifest.cliBinary, sha256: createHash("sha256").update(readFileSync(manifest.cliBinary)).digest("hex"), vcsRevision: sha, vcsModified: false };
  for (const [index, row] of manifest.rows.entries()) {
    const paths = resolved.get(row.browser);
    const proof = await validateProcessBindings(row, paths, manifest.cliBinary, { processInspector, fifoHolderInspector, processLockHolderInspector });
    const target = context.rows[index];
    target.deviceId = `device_${(row.browser === "chrome" ? "a" : "b").repeat(32)}`;
    target.isolationPathDigests = Object.fromEntries(Object.keys(target.isolationPathDigests).map((key) => [key, createHash("sha256").update(paths[key]).digest("hex")]));
    target.server.executable = paths.serverBinary;
    target.server.sha256 = context.cli.sha256;
    target.browserExecutable.path = paths.browserExecutable;
    target.browserExecutable.sha256 = createHash("sha256").update(readFileSync(paths.browserExecutable)).digest("hex");
    target.processes = proof.processes;
    target.consoleReaders = proof.consoleReaders;
  }
  validateFrozenContext(context);
  for (const row of manifest.rows) {
    for (const [name, value] of [
      ["manifest.json", manifest], ["context.json", context], ["journey.json", journey(row.browser)],
      ["summary.json", { schemaVersion: 1, status: "PASS", tests: 1, failures: 0, retainedRawLogs: false }],
    ]) writeFileSync(join(row.evidenceRoot, name), JSON.stringify(value), { mode: 0o600 });
  }
  return {
    ...fixture, manifest, context, processInspector, fifoHolderInspector, processLockHolderInspector,
    scanOptions: { processInspector, fifoHolderInspector, processLockHolderInspector, startedAt: "2030-01-01T00:11:00.000Z", finishedAt: "2030-01-01T00:12:00.000Z" },
  };
}

test("row isolation accepts distinct real paths and key material", async () => {
  const fixture = makeIsolationFixture();
  try {
    const resolved = await validateRowIsolation(fixture.rows);
    assert.equal(resolved.size, 2);
    assert.deepEqual([...validateDeviceIdentities(fixture.rows).keys()], ["chrome", "safari"]);
  } finally {
    fixture.cleanup();
  }
});

test("receipt outputs reject every scan root and symbolic-link parents", () => {
  const fixture = makeIsolationFixture();
  try {
    const manifest = { rows: fixture.rows };
    for (const key of ["runtimeRoot", "deviceRoot", "logRoot", "transferRoot", "evidenceRoot"]) {
      assert.throws(() => validateReceiptOutputPath(manifest, join(fixture.rows[0][key], "receipt.json")), /outside every declared scan root/u);
    }
    const safe = join(fixture.root, "safe");
    mkdirSync(safe, { mode: 0o700 });
    assert.equal(validateReceiptOutputPath(manifest, join(safe, "receipt.json")), join(safe, "receipt.json"));
    const alias = join(fixture.root, "safe-alias");
    symlinkSync(safe, alias);
    assert.throws(() => validateReceiptOutputPath(manifest, join(alias, "receipt.json")), /canonical real path|symbolic-link parent/u);
  } finally {
    fixture.cleanup();
  }
});

test("scanEnvironment end-to-end rejects a writer hidden regular-file sink before scanning", async () => {
  const fixture = makeIsolationFixture();
  try {
    const manifest = validateManifest({ schemaVersion: 2, implementationSha: sha, cliBinary: fixture.rows[0].serverBinary, rows: fixture.rows });
    const context = frozenContext();
    context.manifestSha256 = canonicalDigest(manifest);
    context.receiptToolSha256 = createHash("sha256").update(readFileSync(join(import.meta.dirname, "p2-browser-receipt.mjs"))).digest("hex");
    context.cli.path = fixture.rows[0].serverBinary;
    const row = fixture.rows[0];
    const fifoRecord = (path, access) => {
      const info = statSync(path, { bigint: true });
      return { kind: "FIFO", access, device: info.dev.toString(), inode: info.ino.toString(), path };
    };
    const regularRecord = (path, fd, access) => {
      const info = statSync(path, { bigint: true });
      return { fd, access, kind: "REG", device: info.dev.toString(), inode: info.ino.toString(), path };
    };
    await assert.rejects(scanEnvironment(manifest, context, {
      processInspector: async (pid) => {
        assert.equal(pid, row.serverPid);
        return {
          pid, uid: process.getuid(), state: "S+", startTime: frozenAt, executable: row.serverBinary,
          argv: [row.serverBinary, "--config", row.serverConfig],
          openFiles: [regularRecord(row.serverBinary, "txt", "r"), regularRecord(row.databasePath, "7", "u"), { fd: "9", access: "w", kind: "REG", device: "1", inode: "999", path: join(fixture.root, "hidden.log") }],
          fds: new Map([["1", fifoRecord(row.serverStdoutFifo, "w")], ["2", fifoRecord(row.serverStderrFifo, "w")]]),
        };
      },
      fifoHolderInspector: async () => [],
    }), /undeclared writable regular-file FD/u);
  } finally {
    fixture.cleanup();
  }
});

test("scanEnvironment reaches all six roots and fails closed on residue and filesystem races", { skip: process.platform !== "darwin" }, async (t) => {
  const fixture = await makeScanEnvironmentFixture();
  const chrome = fixture.manifest.rows[0];
  const fastOptions = { ...fixture.scanOptions, databaseAssertionRunner: cleanDatabaseAssertions };
  const temporaryFile = (root, name, contents = "clean fixture") => {
    const path = join(root, name);
    writeFileSync(path, contents, { mode: 0o600 });
    return path;
  };
  try {
    await t.test("success scans every target with real logical SQLite assertions", async () => {
      const scan = await scanEnvironment(fixture.manifest, fixture.context, fixture.scanOptions);
      assert.equal(scan.status, "PASS");
      for (const row of scan.rows) {
        assert.deepEqual(row.targetClasses.map((entry) => entry.class), targetClasses);
        assert.ok(row.targetClasses.every((entry) => entry.pathCount > 0 && entry.matchCount === 0));
        assert.equal(row.databaseAssertions.serverBackupCount, 0);
        assert.deepEqual(row.processLockEvidence, {
          boundToDeclaredServer: true, exclusiveWholeFileLock: true, ownerOnly: true, singleLink: true,
          empty: true, holderCount: 1, inventorySha256: row.processLockEvidence.inventorySha256,
        });
      }
    });

    for (const name of ["server.sqlite.unknown", "server.sqlite.bak", "server.sqlite.backup"]) {
      await t.test(`unknown server database artifact ${name} fails closed as runtime residue`, async () => {
        const path = temporaryFile(chrome.runtimeRoot, name);
        try {
          await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /zero-finding scan/u);
        } finally {
          rmSync(path);
        }
      });
    }

    for (const name of ["server.sqlite.unknown", "server.sqlite.bak", "server.sqlite.backup", "alternate.process.lock", "backups"]) {
      await t.test(`unknown empty runtime directory ${name} fails closed`, async () => {
        const path = join(chrome.runtimeRoot, name);
        mkdirSync(path, { mode: 0o700 });
        try {
          await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /zero-finding scan/u);
        } finally {
          rmSync(path, { recursive: true });
        }
      });
    }

    const canaries = [
      ["server_database", "bootstrap_token", chrome.databasePath, "Bootstrap token (shown once; expires in 10 minutes): " + "A".repeat(43), true],
      ["device_database", "session_cookie", chrome.deviceDatabasePath, "P2-SESSION-COOKIE-CANARY-E2E", true],
      ["runtime_residue", "csrf", chrome.runtimeRoot, "P2-CSRF-CANARY-E2E", false],
      ["logs", "recovery_code", chrome.logRoot, "P2-RECOVERY-CANARY-E2E", false],
      ["transfers", "webauthn_ceremony", chrome.transferRoot, "P2-WEBAUTHN-CANARY-E2E", false],
      ["evidence", "bootstrap_proof", chrome.evidenceRoot, "P2-BOOTSTRAP-PROOF-CANARY-E2E", false],
    ];
    for (const [className, secretClass, target, canary, append] of canaries) {
      await t.test(`${className} canary reaches the scanner`, async () => {
        let observation;
        const options = {
          ...fastOptions,
          scanEvidenceObserver: (value) => { if (value.browser === "chrome") observation = value; },
        };
        if (append) {
          const original = readFileSync(target);
          try {
            writeFileSync(target, Buffer.concat([original, Buffer.from(canary)]), { mode: 0o600 });
            await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, options), /zero-finding scan/u);
          } finally {
            writeFileSync(target, original, { mode: 0o600 });
          }
        } else {
          const path = temporaryFile(target, `${className}.canary`, canary);
          try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, options), /zero-finding scan/u); } finally { rmSync(path); }
        }
        assert.ok(observation, `${className} scan observation is missing`);
        assert.ok(observation.targetClasses.find((entry) => entry.class === className).matchCount > 0, `${className} detector target count is zero`);
        assert.ok(observation.secretCounts[secretClass] > 0, `${secretClass} detector count is zero`);
        assert.equal(Object.isFrozen(observation), true);
        assert.equal(Object.isFrozen(observation.targetClasses), true);
        assert.equal(Object.isFrozen(observation.secretCounts), true);
        assert.equal(JSON.stringify(observation).includes(canary), false);
      });
    }

    await t.test("unreadable or non-private file rejects inside a scanned root", async () => {
      const path = temporaryFile(chrome.evidenceRoot, "unreadable.json");
      chmodSync(path, 0o000);
      try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /owner-only private storage|EACCES/u); } finally { chmodSync(path, 0o600); rmSync(path); }
    });

    await t.test("file mutation during stable read is detected", async () => {
      const path = temporaryFile(chrome.evidenceRoot, "unstable.json");
      let changed = false;
      try {
        await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, {
          ...fastOptions,
          scanFileHooks: { afterRead: (readPath) => { if (!changed && readPath === path) { changed = true; writeFileSync(path, "changed", { mode: 0o600 }); } } },
        }), /unstable while scanning/u);
      } finally { rmSync(path); }
    });

    await t.test("file replacement during stable read is detected", async () => {
      const path = temporaryFile(chrome.evidenceRoot, "replaced.json");
      let replaced = false;
      try {
        await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, {
          ...fastOptions,
          scanFileHooks: { afterRead: (readPath) => { if (!replaced && readPath === path) { replaced = true; rmSync(path); writeFileSync(path, "replacement", { mode: 0o600 }); } } },
        }), /unstable while scanning/u);
      } finally { rmSync(path); }
    });

    await t.test("directory mutation during stable traversal is detected", async () => {
      const directory = join(chrome.deviceRoot, "stable-directory");
      mkdirSync(directory, { mode: 0o700 });
      let changed = false;
      try {
        await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, {
          ...fastOptions,
          scanDirectoryHooks: { afterRead: (readPath) => {
            if (!changed && readPath === directory) {
              changed = true;
              mkdirSync(join(directory, "late-child"), { mode: 0o700 });
            }
          } },
        }), /directory was unstable while scanning/u);
      } finally {
        rmSync(directory, { recursive: true });
      }
    });

    await t.test("symbolic aliases and hard links reject inside scanned roots", async () => {
      const source = temporaryFile(chrome.evidenceRoot, "source.json");
      const alias = join(chrome.evidenceRoot, "alias.json");
      symlinkSync(source, alias);
      try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /symbolic link/u); } finally { rmSync(alias); }
      const hardlink = join(chrome.evidenceRoot, "hardlink.json");
      linkSync(source, hardlink);
      try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /single-link regular file/u); } finally { rmSync(hardlink); rmSync(source); }
      const directoryAlias = join(chrome.runtimeRoot, "directory-alias");
      symlinkSync(chrome.deviceRoot, directoryAlias, "dir");
      try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /symbolic link/u); } finally { unlinkSync(directoryAlias); }
      const nonPrivateDirectory = join(chrome.runtimeRoot, "non-private-directory");
      mkdirSync(nonPrivateDirectory, { mode: 0o755 });
      try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /owner-only private storage/u); } finally { rmSync(nonPrivateDirectory, { recursive: true }); }
    });

    await t.test("unexpected regular log file rejects after complete scanning", async () => {
      const path = temporaryFile(chrome.logRoot, "unexpected.log");
      try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /zero-finding scan|log-sink proof/u); } finally { rmSync(path); }
    });

    await t.test("empty evidence root rejects its required exact evidence set", async () => {
      const saved = readdirSync(chrome.evidenceRoot).map((name) => [name, readFileSync(join(chrome.evidenceRoot, name))]);
      for (const [name] of saved) rmSync(join(chrome.evidenceRoot, name));
      try { await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, fastOptions), /zero-finding scan/u); } finally {
        for (const [name, contents] of saved) writeFileSync(join(chrome.evidenceRoot, name), contents, { mode: 0o600 });
      }
    });

    await t.test("post-snapshot mutation rejects before receipt creation", async () => {
      const path = join(chrome.evidenceRoot, "summary.json");
      const original = readFileSync(path);
      let changed = false;
      try {
        await assert.rejects(scanEnvironment(fixture.manifest, fixture.context, {
          ...fastOptions,
          afterInitialSnapshot: async (browser) => { if (!changed && browser === "chrome") { changed = true; writeFileSync(path, `${original.toString("utf8")} `, { mode: 0o600 }); } },
        }), /scan target changed/u);
      } finally { writeFileSync(path, original, { mode: 0o600 }); }
    });

    await t.test("empty directory add, remove, and rename after a signed scan change the final inventory", async () => {
      const freshOptions = { ...fastOptions, startedAt: "2030-01-01T00:13:00.000Z", finishedAt: "2030-01-01T00:14:00.000Z" };
      const assertFinalRejects = (signed, fresh) => assert.throws(
        () => finalizeReceipt(fixture.context, structuredClone(fixture.context), journey("chrome"), journey("safari"), signed, fresh, observedAt),
        /changed after the signed scan/u,
      );

      const signedBeforeAdd = await scanEnvironment(fixture.manifest, fixture.context, fastOptions);
      const added = join(chrome.deviceRoot, "added-empty-directory");
      mkdirSync(added, { mode: 0o700 });
      const freshAfterAdd = await scanEnvironment(fixture.manifest, fixture.context, freshOptions);
      assertFinalRejects(signedBeforeAdd, freshAfterAdd);
      rmSync(added, { recursive: true });

      const removed = join(chrome.deviceRoot, "removed-empty-directory");
      mkdirSync(removed, { mode: 0o700 });
      const signedBeforeRemove = await scanEnvironment(fixture.manifest, fixture.context, fastOptions);
      rmSync(removed, { recursive: true });
      const freshAfterRemove = await scanEnvironment(fixture.manifest, fixture.context, freshOptions);
      assertFinalRejects(signedBeforeRemove, freshAfterRemove);

      const original = join(chrome.deviceRoot, "renamed-empty-directory");
      const renamed = join(chrome.deviceRoot, "renamed-empty-directory-v2");
      mkdirSync(original, { mode: 0o700 });
      const signedBeforeRename = await scanEnvironment(fixture.manifest, fixture.context, fastOptions);
      renameSync(original, renamed);
      const freshAfterRename = await scanEnvironment(fixture.manifest, fixture.context, freshOptions);
      assertFinalRejects(signedBeforeRename, freshAfterRename);
      rmSync(renamed, { recursive: true });
    });
  } finally {
    fixture.cleanup();
  }
});

test("macOS global lsof holder inventory finds real FIFO writer and cat without pathname lookup", { skip: process.platform !== "darwin" }, async () => {
  const temporary = mkdtempSync(join(tmpdir(), "mad-p2-real-fifo-"));
  const fifo = join(realpathSync.native(temporary), "stream.fifo");
  chmodSync(temporary, 0o700);
  const created = spawnSync("/usr/bin/mkfifo", [fifo], { encoding: "utf8" });
  assert.equal(created.status, 0, created.stderr);
  chmodSync(fifo, 0o600);
  const quoted = `'${fifo.replaceAll("'", `'\\''`)}'`;
  const reader = spawn("/bin/sh", ["-c", `exec /bin/cat < ${quoted} > /dev/null`], { stdio: "ignore" });
  const writer = spawn("/bin/sh", ["-c", `exec /bin/sleep 30 > ${quoted}`], { stdio: "ignore" });
  const stop = async (child) => {
    if (child.exitCode === null) child.kill("SIGTERM");
    if (child.exitCode === null) await Promise.race([new Promise((accept) => child.once("exit", accept)), new Promise((accept) => setTimeout(accept, 2000))]);
  };
  try {
    const expected = [reader.pid, writer.pid].sort((left, right) => left - right);
    let holders = [];
    for (let attempt = 0; attempt < 20; attempt += 1) {
      holders = liveFIFOHolderInspector(fifo, "real FIFO regression");
      if (canonicalJSONStringify(holders) === canonicalJSONStringify(expected)) break;
      await new Promise((accept) => setTimeout(accept, 100));
    }
    assert.deepEqual(holders, expected);
  } finally {
    await Promise.all([stop(reader), stop(writer)]);
    rmSync(temporary, { recursive: true, force: true });
  }
});

test("macOS lsof reports the real server-style process lock as one O_RDWR whole-file writer", {
  skip: process.platform !== "darwin" || !existsSync("/opt/homebrew/bin/go"),
}, async () => {
  const temporary = mkdtempSync(join(tmpdir(), "mad-p2-real-process-lock-"));
  chmodSync(temporary, 0o700);
  const root = realpathSync.native(temporary);
  const source = join(root, "holder.go");
  const binary = join(root, "holder");
  const lock = join(root, "server.sqlite.process.lock");
  writeFileSync(source, `package main
import ("fmt"; "os"; "syscall")
func main() {
  file, err := os.OpenFile(os.Args[1], os.O_CREATE|os.O_RDWR, 0600); if err != nil { panic(err) }
  if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil { panic(err) }
  fmt.Println("ready")
  buffer := make([]byte, 1); _, _ = os.Stdin.Read(buffer)
}
`, { mode: 0o600 });
  const built = spawnSync("/opt/homebrew/bin/go", ["build", "-o", binary, source], { encoding: "utf8", timeout: 30_000 });
  assert.equal(built.status, 0, built.stderr);
  const child = spawn(binary, [lock], { stdio: ["pipe", "pipe", "pipe"] });
  try {
    await Promise.race([
      new Promise((accept, reject) => {
        child.stdout.once("data", (value) => value.toString("utf8").includes("ready") ? accept() : reject(new Error("lock helper did not become ready")));
        child.once("error", reject);
        child.once("exit", (code) => reject(new Error(`lock helper exited early with ${code}`)));
      }),
      new Promise((_, reject) => setTimeout(() => reject(new Error("lock helper readiness timed out")), 5_000)),
    ]);
    const holders = liveProcessLockHolderInspector(lock, "real process-lock regression");
    assert.equal(holders.length, 1);
    assert.equal(holders[0].pid, child.pid);
    assert.match(holders[0].fd, /^\d+$/u);
    assert.equal(holders[0].access, "u");
    assert.ok(["W", " "].includes(holders[0].lockStatus));
    assert.equal(liveExclusiveProcessLockProbe(lock, "real process-lock regression"), true);
  } finally {
    child.stdin.end();
    if (child.exitCode === null) child.kill("SIGTERM");
    if (child.exitCode === null) await Promise.race([new Promise((accept) => child.once("exit", accept)), new Promise((accept) => setTimeout(accept, 2_000))]);
    rmSync(temporary, { recursive: true, force: true });
  }
});

test("declared-process log proof binds each writer to one cat-to-TTY reader and rejects regular sinks or tee holders", async (t) => {
  const fixture = makeIsolationFixture();
  try {
    const row = fixture.rows[0];
    const paths = (await validateRowIsolation(fixture.rows)).get("chrome");
    const fifoRecord = (path, access) => {
      const info = statSync(path, { bigint: true });
      return { kind: "FIFO", access, device: info.dev.toString(), inode: info.ino.toString(), path };
    };
    const tty = { kind: "CHR", access: "u", device: "1", inode: "1", path: "/dev/ttys001" };
    const imageRecord = (path) => {
      const info = statSync(path, { bigint: true });
      return { fd: "txt", access: "r", kind: "REG", device: info.dev.toString(), inode: info.ino.toString(), path };
    };
    const databaseRecord = (path, fd, lockStatus) => {
      const info = statSync(path, { bigint: true });
      return { fd, access: "u", kind: "REG", device: info.dev.toString(), inode: info.ino.toString(), path, ...(lockStatus ? { lockStatus } : {}) };
    };
    const dyldImage = { fd: "txt", kind: "REG", device: "999", inode: "999", path: "/usr/lib/dyld" };
    const processRecords = new Map([
      [row.serverPid, { executable: paths.serverBinary, argv: [paths.serverBinary, "--config", paths.serverConfig], openFiles: [dyldImage, imageRecord(paths.serverBinary), databaseRecord(paths.databasePath, "7"), databaseRecord(paths.serverProcessLockPath, "9", "W")], fds: new Map([["1", fifoRecord(paths.serverStdoutFifo, "w")], ["2", fifoRecord(paths.serverStderrFifo, "w")]]) }],
      [row.daemonPid, { executable: paths.serverBinary, argv: [paths.serverBinary, "daemon", "serve", "--root", paths.deviceRoot], openFiles: [dyldImage, imageRecord(paths.serverBinary), databaseRecord(paths.deviceDatabasePath, "8")], fds: new Map([["1", fifoRecord(paths.daemonStdoutFifo, "w")], ["2", fifoRecord(paths.daemonStderrFifo, "w")]]) }],
      [row.serverStdoutReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.serverStdoutFifo, "r")], ["1", tty], ["2", tty]]) }],
      [row.serverStderrReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.serverStderrFifo, "r")], ["1", tty], ["2", tty]]) }],
      [row.daemonStdoutReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.daemonStdoutFifo, "r")], ["1", tty], ["2", tty]]) }],
      [row.daemonStderrReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.daemonStderrFifo, "r")], ["1", tty], ["2", tty]]) }],
    ]);
    const inspector = async (pid) => ({ pid, uid: process.getuid(), state: "S+", startTime: frozenAt, ...processRecords.get(pid) });
    const writerReader = new Map([
      [paths.serverStdoutFifo, [row.serverPid, row.serverStdoutReaderPid]],
      [paths.serverStderrFifo, [row.serverPid, row.serverStderrReaderPid]],
      [paths.daemonStdoutFifo, [row.daemonPid, row.daemonStdoutReaderPid]],
      [paths.daemonStderrFifo, [row.daemonPid, row.daemonStderrReaderPid]],
    ]);
    const holders = async (path) => [...writerReader.get(path)].sort((left, right) => left - right);
    const lockHolders = async () => [{ pid: row.serverPid, fd: "9", access: "u", lockStatus: "W" }];
    const bindingOptions = (extra = {}) => ({ processInspector: inspector, fifoHolderInspector: holders, processLockHolderInspector: lockHolders, ...extra });

    const proof = await validateProcessBindings(row, paths, paths.serverBinary, bindingOptions());
    assert.equal(Object.keys(proof.consoleReaders).length, 4);
    assert.equal(proof.processes.server.stdout.regularFile, false);
    assert.equal(proof.processes.server.argvDigest, canonicalDigest([paths.serverBinary, "--config", paths.serverConfig]));

    await t.test("daemon root argv mismatch", async () => {
      const original = processRecords.get(row.daemonPid).argv;
      processRecords.get(row.daemonPid).argv = [paths.serverBinary, "daemon", "serve", "--root", fixture.rows[1].deviceRoot];
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()),
        /argv differs/u,
      );
      processRecords.get(row.daemonPid).argv = original;
    });

    await t.test("ambiguous argv and PID reuse evidence", async () => {
      const original = processRecords.get(row.serverPid).argv;
      processRecords.get(row.serverPid).argv = [paths.serverBinary, "--config", `${paths.serverConfig} copied`];
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()),
        /argv differs/u,
      );
      processRecords.get(row.serverPid).argv = original;
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions({
          processInspector: async (pid) => ({ ...(await inspector(pid)), pid: pid + 1 }),
        })),
        /PID changed/u,
      );
    });

    await t.test("multiple txt records select one exact image and reject duplicate exact images", async () => {
      const files = processRecords.get(row.serverPid).openFiles;
      const exact = files.find((item) => item.fd === "txt" && item.path === paths.serverBinary);
      files.push({ ...exact });
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()),
        /running image vnode differs/u,
      );
      files.pop();
    });

    await t.test("server process lock rejects missing FD, wrong access/status, duplicate or foreign holders, and daemon access", async () => {
      const serverFiles = processRecords.get(row.serverPid).openFiles;
      const lockRecord = serverFiles.find((item) => item.path === paths.serverProcessLockPath);
      serverFiles.splice(serverFiles.indexOf(lockRecord), 1);
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /unique numeric O_RDWR FD/u);
      serverFiles.push(lockRecord);

      for (const [key, value] of [["access", "r"], ["lockStatus", "w"]]) {
        const original = lockRecord[key];
        lockRecord[key] = value;
        await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /unique numeric O_RDWR FD/u);
        lockRecord[key] = original;
      }

      serverFiles.push({ ...lockRecord, fd: "10" });
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /unique numeric O_RDWR FD/u);
      serverFiles.pop();
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions({
        processLockHolderInspector: async () => [
          { pid: row.serverPid, fd: "9", access: "u", lockStatus: "W" },
          { pid: 999, fd: "4", access: "u", lockStatus: "W" },
        ],
      })), /exactly one global holder/u);

      const daemonFiles = processRecords.get(row.daemonPid).openFiles;
      daemonFiles.push({ ...lockRecord, fd: "10" });
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /undeclared writable regular-file FD|must not hold/u);
      daemonFiles.pop();
    });

    await t.test("server process lock must remain private, single-link, and empty", async () => {
      writeFileSync(paths.serverProcessLockPath, "not empty", { mode: 0o600 });
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /single-link empty/u);
      writeFileSync(paths.serverProcessLockPath, "", { mode: 0o600 });
      chmodSync(paths.serverProcessLockPath, 0o640);
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /owner-only/u);
      chmodSync(paths.serverProcessLockPath, 0o600);
      const alias = join(fixture.root, "process-lock-hardlink");
      linkSync(paths.serverProcessLockPath, alias);
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /single-link/u);
      rmSync(alias);
    });

    await t.test("post-start input replacement, reader argv drift, and hidden writable log FD fail closed", async () => {
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions({
          processInspector: async (pid) => ({ ...(await inspector(pid)), ...(pid === row.serverPid ? { startTime: "2020-01-01T00:00:00.000Z" } : {}) }),
        })),
        /immutable single-link snapshot created before process start/u,
      );
      const reader = processRecords.get(row.serverStdoutReaderPid);
      reader.argv = ["/bin/cat", paths.serverStdoutFifo];
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /reader argv must be exact/u);
      reader.argv = ["/bin/cat"];
      const originalAccess = reader.fds.get("1").access;
      reader.fds.get("1").access = "r";
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /directly to one Terminal TTY/u);
      reader.fds.get("1").access = originalAccess;
      const files = processRecords.get(row.serverPid).openFiles;
      files.push({ fd: "9", access: "w", kind: "REG", device: "1", inode: "9999", path: join(fixture.root, "hidden.log") });
      await assert.rejects(validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()), /undeclared writable regular-file FD/u);
      files.pop();
    });

    await t.test("open secret database inode cannot be hidden by clean path replacement", async () => {
      const openRecord = processRecords.get(row.daemonPid).openFiles.find((item) => item.path === paths.deviceDatabasePath);
      rmSync(paths.deviceDatabasePath);
      writeFileSync(paths.deviceDatabasePath, "clean replacement", { mode: 0o600 });
      assert.notEqual(openRecord.inode, statSync(paths.deviceDatabasePath, { bigint: true }).ino.toString());
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()),
        /open database vnode differs/u,
      );
    });

    await t.test("regular output sink", async () => {
      rmSync(paths.serverStdoutFifo);
      writeFileSync(paths.serverStdoutFifo, "retained log", { mode: 0o600 });
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()),
        /owner-only 0600 FIFO/u,
      );
    });

    await t.test("running binary replaced after process start", async () => {
      rmSync(paths.serverBinary);
      writeFileSync(paths.serverBinary, "replacement", { mode: 0o600 });
      await assert.rejects(
        validateProcessBindings(row, paths, paths.serverBinary, bindingOptions()),
        /running image vnode differs/u,
      );
    });
  } finally {
    fixture.cleanup();
  }

  const teeFixture = makeIsolationFixture();
  try {
    const row = teeFixture.rows[0];
    const paths = (await validateRowIsolation(teeFixture.rows)).get("chrome");
    const fifoRecord = (path, access) => {
      const info = statSync(path, { bigint: true });
      return { kind: "FIFO", access, device: info.dev.toString(), inode: info.ino.toString(), path };
    };
    const tty = { kind: "CHR", access: "w", device: "1", inode: "1", path: "/dev/ttys001" };
    const imageRecord = (path) => {
      const info = statSync(path, { bigint: true });
      return { fd: "txt", access: "r", kind: "REG", device: info.dev.toString(), inode: info.ino.toString(), path };
    };
    const databaseRecord = (path, fd, lockStatus) => {
      const info = statSync(path, { bigint: true });
      return { fd, access: "u", kind: "REG", device: info.dev.toString(), inode: info.ino.toString(), path, ...(lockStatus ? { lockStatus } : {}) };
    };
    const records = new Map([
      [row.serverPid, { executable: paths.serverBinary, argv: [paths.serverBinary, "--config", paths.serverConfig], openFiles: [imageRecord(paths.serverBinary), databaseRecord(paths.databasePath, "7"), databaseRecord(paths.serverProcessLockPath, "9", "W")], fds: new Map([["1", fifoRecord(paths.serverStdoutFifo, "w")], ["2", fifoRecord(paths.serverStderrFifo, "w")]]) }],
      [row.daemonPid, { executable: paths.serverBinary, argv: [paths.serverBinary, "daemon", "serve", "--root", paths.deviceRoot], openFiles: [imageRecord(paths.serverBinary), databaseRecord(paths.deviceDatabasePath, "8")], fds: new Map([["1", fifoRecord(paths.daemonStdoutFifo, "w")], ["2", fifoRecord(paths.daemonStderrFifo, "w")]]) }],
      [row.serverStdoutReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.serverStdoutFifo, "r")], ["1", tty], ["2", tty]]) }],
      [row.serverStderrReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.serverStderrFifo, "r")], ["1", tty], ["2", tty]]) }],
      [row.daemonStdoutReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.daemonStdoutFifo, "r")], ["1", tty], ["2", tty]]) }],
      [row.daemonStderrReaderPid, { executable: "/bin/cat", argv: ["/bin/cat"], fds: new Map([["0", fifoRecord(paths.daemonStderrFifo, "r")], ["1", tty], ["2", tty]]) }],
    ]);
    await assert.rejects(
      validateProcessBindings(row, paths, paths.serverBinary, {
        processInspector: async (pid) => ({ pid, uid: process.getuid(), state: "S+", startTime: frozenAt, ...records.get(pid) }),
        fifoHolderInspector: async () => [row.serverPid, row.serverStdoutReaderPid, 999].sort((left, right) => left - right),
        processLockHolderInspector: async () => [{ pid: row.serverPid, fd: "9", access: "u", lockStatus: "W" }],
      }),
      /undeclared holder or tee/u,
    );
  } finally {
    teeFixture.cleanup();
  }
});

test("row isolation resolves the macOS Safari system bundle alias but not private-state aliases", async () => {
  const fixture = makeIsolationFixture();
  try {
    const safari = fixture.rows[1];
    const realBundle = safari.browserBundle;
    const aliasBundle = join(fixture.root, "Safari-system-alias.app");
    symlinkSync(realBundle, aliasBundle);
    safari.browserBundle = aliasBundle;
    safari.browserExecutable = join(aliasBundle, "Contents", "MacOS", "Safari");
    const resolved = await validateRowIsolation(fixture.rows);
    assert.equal(resolved.get("safari").browserBundle, realBundle);
    assert.equal(resolved.get("safari").browserExecutable, join(realBundle, "Contents", "MacOS", "Safari"));
  } finally {
    fixture.cleanup();
  }
});

test("row isolation rejects path aliases, DB hard links, and nested Device roots", async (t) => {
  await t.test("path alias", async () => {
    const fixture = makeIsolationFixture();
    try {
      const alias = join(fixture.root, "chrome-alias");
      symlinkSync(join(fixture.root, "chrome"), alias);
      fixture.rows[0].serverConfig = join(alias, "server.json");
      await assert.rejects(validateRowIsolation(fixture.rows), /real path, not an alias/u);
    } finally {
      fixture.cleanup();
    }
  });
  await t.test("database hard link", async () => {
    const fixture = makeIsolationFixture();
    try {
      rmSync(fixture.rows[1].databasePath);
      linkSync(fixture.rows[0].databasePath, fixture.rows[1].databasePath);
      await assert.rejects(validateRowIsolation(fixture.rows), /hard-links isolated path/u);
    } finally {
      fixture.cleanup();
    }
  });
  await t.test("nested Device roots", async () => {
    const fixture = makeIsolationFixture();
    try {
      rmSync(fixture.rows[1].deviceRoot, { recursive: true });
      fixture.rows[1].deviceRoot = join(fixture.rows[0].deviceRoot, "nested-safari");
      mkdirSync(fixture.rows[1].deviceRoot, { mode: 0o700 });
      fixture.rows[1].deviceDatabasePath = join(fixture.rows[1].deviceRoot, "device.db");
      writeFileSync(fixture.rows[1].deviceDatabasePath, "safari-deviceDatabasePath", { mode: 0o600 });
      await assert.rejects(validateRowIsolation(fixture.rows), /must not overlap or nest/u);
    } finally {
      fixture.cleanup();
    }
  });
  await t.test("isolated file under the other Device root", async () => {
    const fixture = makeIsolationFixture();
    try {
      fixture.rows[1].serverConfig = join(fixture.rows[0].deviceRoot, "safari-server.json");
      writeFileSync(fixture.rows[1].serverConfig, "safari-config", { mode: 0o600 });
      await assert.rejects(validateRowIsolation(fixture.rows), /other browser row's Device root/u);
    } finally {
      fixture.cleanup();
    }
  });
});

test("row isolation rejects copied cursor or TLS private-key content without persisting it", async (t) => {
  for (const key of ["cursorKeyPath", "tlsLeafPrivateKey"]) {
    await t.test(key, async () => {
      const fixture = makeIsolationFixture();
      try {
        writeFileSync(fixture.rows[1][key], `${fixture.rows[0].browser}-${key}`, { mode: 0o600 });
        await assert.rejects(validateRowIsolation(fixture.rows), /duplicates secret key material/u);
      } finally {
        fixture.cleanup();
      }
    });
  }
});

test("Device identity isolation rejects copied daemon private keys even with different public IDs", () => {
  const fixture = makeIsolationFixture();
  try {
    const copiedPrivateKey = Buffer.alloc(64, 1).toString("base64");
    writeFileSync(join(fixture.rows[1].deviceRoot, "daemon.identity.json"), JSON.stringify({
      schema_version: 1,
      device_id: `device_${"b".repeat(32)}`,
      private_key: copiedPrivateKey,
    }), { mode: 0o600 });
    assert.throws(() => validateDeviceIdentities(fixture.rows), /private keys must be distinct/u);
  } finally {
    fixture.cleanup();
  }
});

test("hermetic frozen-context to final-receipt JSON roundtrip enforces exact schema and owner-only mode", { skip: process.platform === "win32" }, () => {
  const root = mkdtempSync(join(tmpdir(), "mad-p2-receipt-roundtrip-"));
  try {
    chmodSync(root, 0o700);
    const contextPath = join(root, "context.json");
    const receiptPath = join(root, "receipt.json");
    const collected = frozenContext();
    writeExclusiveJSON(contextPath, collected);
    assert.equal(statSync(contextPath).mode & 0o777, 0o600);
    const read = validateFrozenContext(readJSON(contextPath, "roundtrip context"));
    const scan = machineScan(read);
    const receipt = finalizeReceipt(read, structuredClone(read), journey("chrome"), journey("safari"), scan, structuredClone(scan), observedAt);
    writeExclusiveJSON(receiptPath, receipt);
    assert.equal(statSync(receiptPath).mode & 0o777, 0o600);
    const final = validateReceipt(readJSON(receiptPath, "roundtrip receipt"));
    assert.equal(final.status, "PASS");
    assert.equal(final.exclusions.webdriverOrEmulation, false);
    assert.throws(() => writeExclusiveJSON(receiptPath, final), /EEXIST/u);

    const drifted = structuredClone(read);
    drifted.frozenAt = "2030-01-01T00:00:00.001Z";
    assert.throws(() => finalizeReceipt(read, drifted, journey("chrome"), journey("safari"), scan, structuredClone(scan), observedAt), /changed before receipt finalization/u);
    assert.throws(() => validateReceipt({ ...final, extra: true }), /missing or unknown/u);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("macOS receipt private paths reject group-readable files", { skip: process.platform !== "darwin" }, () => {
  const root = mkdtempSync(join(tmpdir(), "mad-p2-receipt-permissions-"));
  try {
    chmodSync(root, 0o700);
    const directory = join(root, "device");
    mkdirSync(directory, { mode: 0o700 });
    const file = join(root, "manifest.json");
    writeFileSync(file, "{}", { mode: 0o600 });
    assert.doesNotThrow(() => assertPrivatePath(file, "file", "fixture"));
    assert.doesNotThrow(() => assertPrivatePath(directory, "directory", "fixture"));
    chmodSync(file, 0o640);
    assert.throws(() => assertPrivatePath(file, "file", "fixture"), /mode 600/u);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
