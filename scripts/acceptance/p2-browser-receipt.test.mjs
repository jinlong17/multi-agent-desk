import assert from "node:assert/strict";
import { X509Certificate, createHash } from "node:crypto";
import {
  chmodSync, linkSync, mkdtempSync, mkdirSync, realpathSync, rmSync, statSync, symlinkSync, writeFileSync,
} from "node:fs";
import { Agent as HTTPSAgent, createServer as createHTTPSServer, get as httpsGet } from "node:https";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import {
  assertPrivatePath,
  finalizeReceipt,
  readJSON,
  stableContext,
  validateBrowserBundleMetadata,
  validateDeviceIdentities,
  validateFrozenContext,
  validateJourney,
  validateManifest,
  validateReceipt,
  validateRowIsolation,
  validateVersionTLSSocket,
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
  cursorKeyPath: `/private/tmp/mad/${prefix}/cursor.key`,
  tlsLeafCertificate: `/private/tmp/mad/${prefix}/leaf.pem`,
  tlsLeafPrivateKey: `/private/tmp/mad/${prefix}/leaf.key`,
  temporaryCA: "/private/tmp/mad/ca.pem",
  deviceRoot: `/private/tmp/mad/${prefix}/device`,
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
  const manifest = validateManifest({ implementationSha: sha, cliBinary, rows: [row("safari", "safari"), row("chrome", "chrome")] });
  assert.equal(manifest.cliBinary, cliBinary);
  assert.deepEqual(manifest.rows.map((item) => item.browser), ["chrome", "safari"]);
  assert.throws(() => validateManifest({ implementationSha: sha, cliBinary, rows: [row("chrome", "shared"), row("safari", "shared")] }), /independent/u);
  assert.throws(() => validateManifest({ implementationSha: sha, cliBinary, rows: [{ ...row("chrome", "chrome"), origin: "https://localhost:8443" }, row("safari", "safari")] }), /frozen P2 authority/u);
  assert.throws(() => validateManifest({ implementationSha: sha, cliBinary, rows: [{ ...row("chrome", "chrome"), extra: true }, row("safari", "safari")] }), /missing or unknown/u);
  assert.throws(() => validateManifest({ implementationSha: sha, rows: [row("chrome", "chrome"), row("safari", "safari")] }), /missing or unknown/u);
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
  databaseSecretScanPassed: true,
  logSecretScanPassed: true,
  artifactSecretScanPassed: true,
  manualOperatorConfirmed: true,
  ...(browser === "safari" ? { authenticatorAttachment: "platform", userVerificationObserved: true, touchIdOperatorConfirmed: true } : {}),
});

test("browser journey finalization rejects bypass, automation, future completion, and missing Touch ID", () => {
  assert.equal(validateJourney(journey("chrome"), "chrome", frozenAt, observedAt).logout, true);
  assert.equal(validateJourney(journey("safari"), "safari", frozenAt, observedAt).authenticatorAttachment, "platform");
  assert.throws(() => validateJourney({ ...journey("chrome"), tlsWarningBypassed: true }, "chrome", frozenAt, observedAt), /explicitly confirmed false/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), automationSubstituteUsed: true }, "chrome", frozenAt, observedAt), /explicitly confirmed false/u);
  assert.throws(() => validateJourney({ ...journey("safari"), webdriverSubstituteUsed: true }, "safari", frozenAt, observedAt), /explicitly confirmed false/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), registration: false }, "chrome", frozenAt, observedAt), /explicitly confirmed true/u);
  assert.throws(() => validateJourney({ ...journey("safari"), touchIdOperatorConfirmed: false }, "safari", frozenAt, observedAt), /Touch ID/u);
  assert.throws(() => validateJourney({ ...journey("safari"), startedAt: "2029-12-31T23:59:59.000Z" }, "safari", frozenAt, observedAt), /observation window/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), finishedAt: "2030-01-01T00:20:00.001Z" }, "chrome", frozenAt, observedAt), /observation window/u);
  assert.throws(() => validateJourney({ ...journey("chrome"), csrfToken: "forbidden" }, "chrome", frozenAt, observedAt), /missing or unknown/u);
});

function contextRow(browser, marker) {
  return {
    browser,
    origin: browser === "chrome" ? "https://chrome.localhost:8443" : "https://safari.localhost:9443",
    rpId: browser === "chrome" ? "chrome.localhost" : "safari.localhost",
    deviceId: `device_${marker.repeat(32)}`,
    privateRuntimeStorageVerified: true,
    isolationPathDigests: {
      serverConfig: digest(marker),
      databasePath: digest(marker === "b" ? "c" : "d"),
      cursorKeyPath: digest(marker === "b" ? "e" : "f"),
      tlsLeafCertificate: digest(marker === "b" ? "1" : "2"),
      tlsLeafPrivateKey: digest(marker === "b" ? "3" : "4"),
      deviceRoot: digest(marker === "b" ? "5" : "6"),
    },
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
  };
}

function frozenContext() {
  return {
    schemaVersion: 1,
    status: "FROZEN_PENDING_MANUAL_JOURNEY",
    frozenAt,
    implementationSha: sha,
    cleanWorktree: true,
    os: { productVersion: "27.0", buildVersion: "26A123", architecture: "arm64" },
    cli: { path: cliBinary, sha256: digest("0"), vcsRevision: sha, vcsModified: false },
    rows: [contextRow("chrome", "b"), contextRow("safari", "c")],
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
    const bundle = join(base, browser === "chrome" ? "Google Chrome.app" : "Safari.app");
    const macOS = join(bundle, "Contents", "MacOS");
    const deviceRoot = join(base, "device");
    mkdirSync(macOS, { recursive: true, mode: 0o700 });
    mkdirSync(deviceRoot, { mode: 0o700 });
    writeFileSync(join(deviceRoot, "daemon.identity.json"), JSON.stringify({
      schema_version: 1,
      device_id: `device_${(browser === "chrome" ? "a" : "b").repeat(32)}`,
      private_key: Buffer.alloc(64, browser === "chrome" ? 1 : 2).toString("base64"),
    }), { mode: 0o600 });
    const values = {
      serverConfig: join(base, "server.json"),
      databasePath: join(base, "server.sqlite"),
      cursorKeyPath: join(base, "cursor.key"),
      tlsLeafCertificate: join(base, "leaf.pem"),
      tlsLeafPrivateKey: join(base, "leaf.key"),
      browserExecutable: join(macOS, browser === "chrome" ? "Google Chrome" : "Safari"),
    };
    for (const [key, path] of Object.entries(values)) writeFileSync(path, `${browser}-${key}`, { mode: 0o600 });
    return {
      ...row(browser, browser),
      serverBinary,
      temporaryCA,
      ...values,
      deviceRoot,
      browserBundle: bundle,
    };
  });
  return { root, rows, cleanup: () => rmSync(root, { recursive: true, force: true }) };
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
    const receipt = finalizeReceipt(read, structuredClone(read), journey("chrome"), journey("safari"), observedAt);
    writeExclusiveJSON(receiptPath, receipt);
    assert.equal(statSync(receiptPath).mode & 0o777, 0o600);
    const final = validateReceipt(readJSON(receiptPath, "roundtrip receipt"));
    assert.equal(final.status, "PASS");
    assert.equal(final.exclusions.webdriverOrEmulation, false);
    assert.throws(() => writeExclusiveJSON(receiptPath, final), /EEXIST/u);

    const drifted = structuredClone(read);
    drifted.frozenAt = "2030-01-01T00:00:00.001Z";
    assert.throws(() => finalizeReceipt(read, drifted, journey("chrome"), journey("safari"), observedAt), /changed before receipt finalization/u);
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
