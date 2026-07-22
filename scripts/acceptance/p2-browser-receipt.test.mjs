import assert from "node:assert/strict";
import {
  chmodSync, linkSync, mkdtempSync, mkdirSync, realpathSync, rmSync, statSync, symlinkSync, writeFileSync,
} from "node:fs";
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
  writeExclusiveJSON,
} from "./p2-browser-receipt.mjs";

const sha = "a".repeat(40);
const digest = (character) => character.repeat(64);
const fingerprint = (octet) => Array.from({ length: 32 }, () => octet).join(":");
const cliBinary = "/private/tmp/mad/bin/multidesk";
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
