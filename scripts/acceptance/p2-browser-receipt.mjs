import { X509Certificate, createHash } from "node:crypto";
import { createReadStream, existsSync, lstatSync, readFileSync, realpathSync, statSync, writeFileSync } from "node:fs";
import { get as httpsGet } from "node:https";
import { isIP } from "node:net";
import { basename, dirname, isAbsolute, relative, resolve, sep } from "node:path";
import { pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";

const repoRoot = resolve(import.meta.dirname, "../..");
const rowAuthority = Object.freeze({
  chrome: Object.freeze({
    origin: "https://chrome.localhost:8443",
    rpId: "chrome.localhost",
    bundleIdentifier: "com.google.Chrome",
    bundleExecutable: "Google Chrome",
  }),
  safari: Object.freeze({
    origin: "https://safari.localhost:9443",
    rpId: "safari.localhost",
    bundleIdentifier: "com.apple.Safari",
    bundleExecutable: "Safari",
  }),
});
const manifestKeys = ["implementationSha", "cliBinary", "rows"];
const rowKeys = [
  "browser", "origin", "rpId", "serverBinary", "serverConfig", "databasePath", "cursorKeyPath",
  "tlsLeafCertificate", "tlsLeafPrivateKey", "temporaryCA", "deviceRoot", "browserBundle", "browserExecutable",
];
const journeyKeys = [
  "browser", "startedAt", "finishedAt", "bootstrap", "registration", "login", "recovery",
  "replacementPasskey", "passkeyDelete", "logout", "browserReportedSecureConnection",
  "tlsWarningBypassed", "viteProxyUsed", "corsWorkaroundUsed", "secretArtifactCaptured",
  "automationSubstituteUsed", "webdriverSubstituteUsed", "databaseSecretScanPassed", "logSecretScanPassed",
  "artifactSecretScanPassed", "manualOperatorConfirmed",
];
const safariJourneyKeys = [...journeyKeys, "authenticatorAttachment", "userVerificationObserved", "touchIdOperatorConfirmed"];
const isolatedPathKeys = ["serverConfig", "databasePath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey", "deviceRoot"];
const contextKeys = ["schemaVersion", "status", "frozenAt", "implementationSha", "cleanWorktree", "os", "cli", "rows"];
const contextOSKeys = ["productVersion", "buildVersion", "architecture"];
const contextCLIKeys = ["path", "sha256", "vcsRevision", "vcsModified"];
const contextRowKeys = [
  "browser", "origin", "rpId", "deviceId", "privateRuntimeStorageVerified", "isolationPathDigests",
  "server", "browserExecutable", "tls",
];
const contextServerKeys = ["executable", "sha256", "versionOutput", "versionEndpoint"];
const contextVersionEndpointKeys = ["version", "commit", "directLoopbackTLS"];
const contextBrowserExecutableKeys = ["path", "sha256", "bundle", "version", "codeIdentity"];
const contextBrowserVersionKeys = ["product", "build"];
const contextCodeIdentityKeys = ["identifier", "teamIdentifier", "cdHash"];
const contextTLSKeys = [
  "temporaryCAFingerprintSHA256", "temporaryCASHA256", "leafFingerprintSHA256", "leafSubject", "leafIssuer",
  "leafValidFrom", "leafValidTo", "directValidatedRequest", "macOSSystemTrustVerified", "insecureVerificationBypass",
];
const receiptKeys = ["schemaVersion", "status", "finalizedAt", "evidenceBoundary", "frozenContext", "journeys", "exclusions"];
const exclusionKeys = [
  "windowsBrowserAcceptance", "ubuntuBrowserAcceptance", "claudeAcceptance", "apiKey", "printMode", "dollarBudget",
  "usageCredits", "webdriverOrEmulation",
];
const forbiddenSecretMarkers = ["MAD-RC1-", "__Host-mad_session=", '"csrfToken"', '"bootstrapToken"'];
const sha256Pattern = /^[0-9a-f]{64}$/u;
const x509FingerprintPattern = /^(?:[0-9a-f]{2}:){31}[0-9a-f]{2}$/iu;
const receiptEvidenceBoundary = "environment is machine-verified; browser journeys and Safari Touch ID are explicit operator attestations; WebDriver, automation, and emulation substitutes are forbidden";

function exactObject(value, keys, label) {
  if (typeof value !== "object" || value === null || Array.isArray(value)) throw new Error(`${label} must be an object`);
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) {
    throw new Error(`${label} has missing or unknown fields`);
  }
  return value;
}

function canonicalAbsolutePath(value, label) {
  if (typeof value !== "string" || !isAbsolute(value) || resolve(value) !== value) throw new Error(`${label} must be an absolute clean path`);
  return value;
}

function canonicalTimestamp(value, label) {
  if (typeof value !== "string" || !value.endsWith("Z") || Number.isNaN(Date.parse(value)) || new Date(value).toISOString() !== value) {
    throw new Error(`${label} must be a canonical UTC timestamp`);
  }
  return value;
}

function nonEmptyString(value, label) {
  if (typeof value !== "string" || value.length === 0) throw new Error(`${label} must be a non-empty string`);
  return value;
}

function exactBoolean(value, expected, label) {
  if (value !== expected) throw new Error(`${label} must be ${expected}`);
  return value;
}

function exactSHA256(value, label) {
  if (typeof value !== "string" || !sha256Pattern.test(value)) throw new Error(`${label} must be a canonical lowercase SHA-256 digest`);
  return value;
}

function exactX509Fingerprint(value, label) {
  if (typeof value !== "string" || !x509FingerprintPattern.test(value)) throw new Error(`${label} must be a colon-delimited SHA-256 fingerprint`);
  return value;
}

function rejectUnsafeJSON(value, label) {
  if (typeof value === "number" && !Number.isFinite(value)) throw new Error(`${label} contains a non-finite number`);
  if (value === undefined || typeof value === "bigint" || typeof value === "function" || typeof value === "symbol") {
    throw new Error(`${label} contains a non-JSON value`);
  }
  if (Array.isArray(value)) {
    value.forEach((entry, index) => rejectUnsafeJSON(entry, `${label}[${index}]`));
  } else if (typeof value === "object" && value !== null) {
    for (const [key, entry] of Object.entries(value)) rejectUnsafeJSON(entry, `${label}.${key}`);
  }
}

function rejectSecretMarkers(value, label) {
  const contents = JSON.stringify(value);
  for (const forbidden of forbiddenSecretMarkers) {
    if (contents.includes(forbidden)) throw new Error(`${label} contains forbidden secret marker ${forbidden}`);
  }
}

export function validateManifest(input) {
  const manifest = exactObject(input, manifestKeys, "manifest");
  if (typeof manifest.implementationSha !== "string" || !/^[0-9a-f]{40}$/u.test(manifest.implementationSha)) {
    throw new Error("manifest.implementationSha must be one full lowercase Git SHA");
  }
  canonicalAbsolutePath(manifest.cliBinary, "manifest.cliBinary");
  if (!Array.isArray(manifest.rows) || manifest.rows.length !== 2) throw new Error("manifest.rows must contain exact Chrome and Safari rows");
  const seen = new Set();
  const rows = manifest.rows.map((candidate, index) => {
    const row = exactObject(candidate, rowKeys, `manifest.rows[${index}]`);
    if (row.browser !== "chrome" && row.browser !== "safari" || seen.has(row.browser)) throw new Error("manifest must contain one Chrome and one Safari row");
    seen.add(row.browser);
    const authority = rowAuthority[row.browser];
    if (row.origin !== authority.origin || row.rpId !== authority.rpId) throw new Error(`${row.browser} origin/RP ID differs from frozen P2 authority`);
    for (const key of rowKeys.slice(3)) canonicalAbsolutePath(row[key], `${row.browser}.${key}`);
    return row;
  });
  const isolated = ["serverConfig", "databasePath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey", "deviceRoot"];
  for (const key of isolated) {
    if (rows[0][key] === rows[1][key]) throw new Error(`Chrome and Safari must use independent ${key}`);
  }
  return { implementationSha: manifest.implementationSha, cliBinary: manifest.cliBinary, rows: rows.sort((left, right) => left.browser.localeCompare(right.browser)) };
}

export function validateJourney(input, browser, frozenAt, observedAt) {
  const keys = browser === "safari" ? safariJourneyKeys : journeyKeys;
  const journey = exactObject(input, keys, `${browser} journey`);
  if (journey.browser !== browser) throw new Error(`${browser} journey browser mismatch`);
  const frozenTime = canonicalTimestamp(frozenAt, "frozenAt");
  const observedTime = canonicalTimestamp(observedAt, "finalize observation");
  const startedAt = canonicalTimestamp(journey.startedAt, `${browser}.startedAt`);
  const finishedAt = canonicalTimestamp(journey.finishedAt, `${browser}.finishedAt`);
  if (Date.parse(startedAt) < Date.parse(frozenTime) || Date.parse(finishedAt) < Date.parse(startedAt) ||
      Date.parse(finishedAt) > Date.parse(observedTime)) {
    throw new Error(`${browser} journey timestamps are outside the frozen/finalize observation window`);
  }
  for (const key of [
    "bootstrap", "registration", "login", "recovery", "replacementPasskey", "passkeyDelete", "logout",
    "browserReportedSecureConnection", "databaseSecretScanPassed", "logSecretScanPassed", "artifactSecretScanPassed", "manualOperatorConfirmed",
  ]) {
    if (journey[key] !== true) throw new Error(`${browser}.${key} must be explicitly confirmed true`);
  }
  for (const key of [
    "tlsWarningBypassed", "viteProxyUsed", "corsWorkaroundUsed", "secretArtifactCaptured",
    "automationSubstituteUsed", "webdriverSubstituteUsed",
  ]) {
    if (journey[key] !== false) throw new Error(`${browser}.${key} must be explicitly confirmed false`);
  }
  if (browser === "safari" && (journey.authenticatorAttachment !== "platform" || journey.userVerificationObserved !== true || journey.touchIdOperatorConfirmed !== true)) {
    throw new Error("Safari requires operator-confirmed Touch ID, platform attachment, and user verification");
  }
  return journey;
}

function command(path, args, label) {
  const result = spawnSync(path, args, { cwd: repoRoot, encoding: "utf8", shell: false, maxBuffer: 4 << 20 });
  if (result.status !== 0 || result.error) throw new Error(`${label} failed: ${(result.stderr || result.error?.message || "unknown error").trim()}`);
  return `${result.stdout ?? ""}${result.stderr ?? ""}`.trim();
}

export function readJSON(path, label) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    throw new Error(`${label} is not valid JSON: ${error.message}`);
  }
}

function assertPath(path, kind, label) {
  const info = lstatSync(path);
  if (info.isSymbolicLink() || kind === "file" && !info.isFile() || kind === "directory" && !info.isDirectory()) {
    throw new Error(`${label} is not a non-symlink ${kind}`);
  }
}

export function assertPrivatePath(path, kind, label) {
  assertPath(path, kind, label);
  if (process.platform !== "darwin") return;
  const info = lstatSync(path);
  const expectedMode = kind === "file" ? 0o600 : 0o700;
  if ((info.mode & 0o777) !== expectedMode || typeof process.getuid !== "function" || info.uid !== process.getuid()) {
    throw new Error(`${label} must be owned by the current macOS user with mode ${expectedMode.toString(8)}`);
  }
}

function canonicalRealPath(path, kind, label, allowAlias = false) {
  canonicalAbsolutePath(path, label);
  const real = realpathSync.native(path);
  if (!allowAlias && real !== path) throw new Error(`${label} must use its canonical real path, not an alias`);
  assertPath(real, kind, label);
  return real;
}

function pathContains(parent, child) {
  const candidate = relative(parent, child);
  return candidate === "" || candidate !== ".." && !candidate.startsWith(`..${sep}`) && !isAbsolute(candidate);
}

function fileIdentity(path) {
  const info = statSync(path, { bigint: true });
  return `${info.dev}:${info.ino}`;
}

async function sha256File(path) {
  const digest = createHash("sha256");
  await new Promise((accept, reject) => {
    const input = createReadStream(path);
    input.on("data", (chunk) => digest.update(chunk));
    input.on("error", reject);
    input.on("end", accept);
  });
  return digest.digest("hex");
}

function sha256Text(value) {
  return createHash("sha256").update(value).digest("hex");
}

// validateRowIsolation resolves every security-relevant path before comparing
// it. It rejects aliases, hard links, nested Device roots, and copied key
// material without persisting either cursor-key or TLS-private-key digests.
export async function validateRowIsolation(rows) {
  if (!Array.isArray(rows) || rows.length !== 2) throw new Error("row isolation requires exact Chrome and Safari rows");
  const resolved = new Map();
  const seenIdentities = new Map();
  for (const row of rows) {
    const paths = {};
    for (const [key, kind, allowAlias = false] of [
      ["serverBinary", "file"], ["serverConfig", "file"], ["databasePath", "file"], ["cursorKeyPath", "file"],
      ["tlsLeafCertificate", "file"], ["tlsLeafPrivateKey", "file"], ["temporaryCA", "file"],
      ["deviceRoot", "directory"], ["browserBundle", "directory", true], ["browserExecutable", "file", true],
    ]) {
      paths[key] = canonicalRealPath(row[key], kind, `${row.browser}.${key}`, allowAlias);
    }
    resolved.set(row.browser, paths);
    for (const key of isolatedPathKeys) {
      const identity = fileIdentity(paths[key]);
      const prior = seenIdentities.get(identity);
      if (prior) throw new Error(`${row.browser}.${key} aliases or hard-links isolated path ${prior}`);
      seenIdentities.set(identity, `${row.browser}.${key}`);
    }
  }

  const chromePaths = resolved.get("chrome");
  const safariPaths = resolved.get("safari");
  if (!chromePaths || !safariPaths) throw new Error("row isolation requires one Chrome and one Safari row");
  if (pathContains(chromePaths.deviceRoot, safariPaths.deviceRoot) || pathContains(safariPaths.deviceRoot, chromePaths.deviceRoot)) {
    throw new Error("Chrome and Safari Device roots must not overlap or nest");
  }
  for (const key of ["serverConfig", "databasePath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey"]) {
    if (pathContains(safariPaths.deviceRoot, chromePaths[key]) || pathContains(chromePaths.deviceRoot, safariPaths[key])) {
      throw new Error(`${key} must not be placed under the other browser row's Device root`);
    }
  }

  const secretDigests = new Map();
  for (const [browser, paths] of resolved) {
    for (const key of ["cursorKeyPath", "tlsLeafPrivateKey"]) {
      const digest = await sha256File(paths[key]);
      const prior = secretDigests.get(digest);
      if (prior) throw new Error(`${browser}.${key} duplicates secret key material from ${prior}`);
      secretDigests.set(digest, `${browser}.${key}`);
    }
  }
  return resolved;
}

function bundleMetadata(bundle) {
  const info = resolve(bundle, "Contents/Info.plist");
  assertPath(info, "file", "browser Info.plist");
  return {
    identifier: command("/usr/libexec/PlistBuddy", ["-c", "Print :CFBundleIdentifier", info], "read browser bundle identifier"),
    executable: command("/usr/libexec/PlistBuddy", ["-c", "Print :CFBundleExecutable", info], "read browser bundle executable"),
    product: command("/usr/libexec/PlistBuddy", ["-c", "Print :CFBundleShortVersionString", info], "read browser product version"),
    build: command("/usr/libexec/PlistBuddy", ["-c", "Print :CFBundleVersion", info], "read browser build version"),
  };
}

export function validateBrowserBundleMetadata(row, metadata, bundlePath = row.browserBundle, executablePath = row.browserExecutable) {
  const values = exactObject(metadata, ["identifier", "executable", "product", "build"], `${row.browser} browser bundle metadata`);
  const authority = rowAuthority[row.browser];
  if (!authority || values.identifier !== authority.bundleIdentifier || values.executable !== authority.bundleExecutable) {
    throw new Error(`${row.browser} bundle identity or executable differs from frozen browser authority`);
  }
  const expectedExecutable = resolve(bundlePath, "Contents/MacOS", values.executable);
  if (executablePath !== expectedExecutable) throw new Error(`${row.browser} executable does not match its verified bundle CFBundleExecutable`);
  nonEmptyString(values.product, `${row.browser} browser product version`);
  nonEmptyString(values.build, `${row.browser} browser build version`);
  return values;
}

function codeIdentity(row, bundlePath, executablePath) {
  command("/usr/bin/codesign", ["--verify", "--strict", "--verbose=2", bundlePath], `verify ${row.browser} browser bundle signature`);
  command("/usr/bin/codesign", ["--verify", "--strict", "--verbose=2", executablePath], `verify ${row.browser} browser executable signature`);
  const output = command("/usr/bin/codesign", ["-dv", "--verbose=4", executablePath], "inspect browser signature");
  const field = (name) => output.match(new RegExp(`(?:^|\\n)${name}=([^\\n]+)`, "u"))?.[1]?.trim();
  const identity = { identifier: field("Identifier"), teamIdentifier: field("TeamIdentifier"), cdHash: field("CDHash") };
  if (!identity.identifier || !identity.teamIdentifier || !identity.cdHash) throw new Error("browser code-signing identity is incomplete");
  if (identity.identifier !== rowAuthority[row.browser].bundleIdentifier) throw new Error(`${row.browser} executable signing identifier differs from its bundle authority`);
  return identity;
}

function loopback(address) {
  if (typeof address !== "string") return false;
  const normalized = address.startsWith("::ffff:") ? address.slice(7) : address;
  return normalized === "::1" || isIP(normalized) === 4 && normalized.startsWith("127.");
}

export function validateVersionTLSSocket(socket, expectedLeafSHA256) {
  exactSHA256(expectedLeafSHA256, "expected TLS leaf SHA-256");
  if (!socket) throw new Error("version endpoint response is missing its TLS socket");
  if (socket.authorized !== true) throw new Error("version endpoint TLS socket is not authorized");
  if (!loopback(socket.remoteAddress)) throw new Error("version endpoint TLS socket is not a direct loopback connection");
  if (typeof socket.getPeerCertificate !== "function") throw new Error("version endpoint TLS peer certificate raw bytes are missing");
  const peer = socket.getPeerCertificate();
  if (!peer || !Buffer.isBuffer(peer.raw) || peer.raw.length === 0) {
    throw new Error("version endpoint TLS peer certificate raw bytes are missing");
  }
  const peerFingerprint = createHash("sha256").update(peer.raw).digest("hex");
  if (peerFingerprint !== expectedLeafSHA256) throw new Error("served TLS leaf differs from the manifest certificate");
  return socket;
}

async function probeVersion(row, expectedCommit, leaf) {
  const url = new URL("/v1/version", row.origin);
  const ca = readFileSync(row.temporaryCA);
  const expectedLeafSHA256 = leaf.fingerprint256.replaceAll(":", "").toLowerCase();
  return new Promise((accept, reject) => {
    const request = httpsGet({
      hostname: url.hostname,
      port: Number(url.port),
      path: url.pathname,
      method: "GET",
      headers: { Accept: "application/json" },
      ca,
      rejectUnauthorized: true,
      servername: url.hostname,
      minVersion: "TLSv1.2",
      timeout: 5_000,
    }, (response) => {
      const socket = response.socket;
      try {
        validateVersionTLSSocket(socket, expectedLeafSHA256);
      } catch (error) {
        response.destroy();
        reject(error);
        return;
      }
      const chunks = [];
      response.on("data", (chunk) => chunks.push(chunk));
      response.on("end", () => {
        try {
          if (response.statusCode !== 200 || response.headers["content-type"] !== "application/json") throw new Error("version endpoint did not return exact JSON 200");
          const payload = JSON.parse(Buffer.concat(chunks).toString("utf8"));
          if (typeof payload !== "object" || payload === null || Array.isArray(payload) ||
              typeof payload.data !== "object" || payload.data === null || Array.isArray(payload.data)) {
            throw new Error("/v1/version body is not the expected JSON envelope");
          }
          nonEmptyString(payload.data.version, "/v1/version data.version");
          if (payload.data.commit !== expectedCommit) throw new Error("/v1/version commit differs from frozen implementation SHA");
          accept({ version: payload.data.version, commit: payload.data.commit, directLoopbackTLS: true });
        } catch (error) {
          reject(error);
        }
      });
    });
    request.on("timeout", () => request.destroy(new Error("version endpoint timed out")));
    request.on("error", reject);
  });
}

function validateServerConfig(row) {
  const config = readJSON(row.serverConfig, `${row.browser} server config`);
  const url = new URL(row.origin);
  const expected = {
    publicOrigin: row.origin,
    rpId: row.rpId,
    databasePath: row.databasePath,
    cursorHmacKeyFile: row.cursorKeyPath,
    tlsCertificateFile: row.tlsLeafCertificate,
    tlsPrivateKeyFile: row.tlsLeafPrivateKey,
  };
  for (const [key, value] of Object.entries(expected)) {
    if (config[key] !== value) throw new Error(`${row.browser} config ${key} differs from the manifest`);
  }
  if (config.listen !== `127.0.0.1:${url.port}` && config.listen !== `[::1]:${url.port}`) throw new Error(`${row.browser} config must listen only on loopback port ${url.port}`);
  if (config.developmentAllowLocalhost !== true) throw new Error(`${row.browser} config must explicitly allow the frozen localhost development origin`);
}

async function cliBuildIdentity(path, expectedCommit) {
  const cliPath = canonicalRealPath(path, "file", "manifest.cliBinary");
  const output = command("/opt/homebrew/bin/go", ["version", "-m", cliPath], "inspect multidesk CLI Go build info");
  const settings = new Map();
  for (const line of output.split("\n")) {
    const match = line.match(/^\s*build\s+([^=\s]+)=(.*)$/u);
    if (match) {
      if (settings.has(match[1])) throw new Error(`multidesk CLI build info repeats ${match[1]}`);
      settings.set(match[1], match[2]);
    }
  }
  if (settings.get("vcs.revision") !== expectedCommit || settings.get("vcs.modified") !== "false") {
    throw new Error("multidesk CLI must be an unmodified Go build from the frozen implementation SHA");
  }
  return {
    path: cliPath,
    sha256: await sha256File(cliPath),
    vcsRevision: settings.get("vcs.revision"),
    vcsModified: false,
  };
}

function verifyPrivateRuntimeStorage(row) {
  assertPrivatePath(dirname(row.databasePath), "directory", `${row.browser} database parent`);
  assertPrivatePath(row.databasePath, "file", `${row.browser} database`);
  for (const suffix of ["-wal", "-shm", "-journal"]) {
    const sidecar = `${row.databasePath}${suffix}`;
    if (existsSync(sidecar)) assertPrivatePath(sidecar, "file", `${row.browser} database sidecar ${suffix}`);
  }
}

function publicDeviceIdentity(root, browser) {
  const identityPath = resolve(root, "daemon.identity.json");
  canonicalRealPath(identityPath, "file", `${browser} daemon identity`);
  assertPrivatePath(identityPath, "file", `${browser} daemon identity`);
  const identity = exactObject(readJSON(identityPath, `${browser} daemon identity`), ["schema_version", "device_id", "private_key"], `${browser} daemon identity`);
  const privateKey = typeof identity.private_key === "string" ? Buffer.from(identity.private_key, "base64") : Buffer.alloc(0);
  if (identity.schema_version !== 1 || typeof identity.device_id !== "string" || !/^device_[0-9a-f]{32}$/u.test(identity.device_id) ||
      privateKey.length !== 64 || privateKey.toString("base64") !== identity.private_key) {
    throw new Error(`${browser} daemon identity is invalid`);
  }
  return { deviceId: identity.device_id, privateKeyDigest: createHash("sha256").update(privateKey).digest("hex") };
}

export function validateDeviceIdentities(rows) {
  const publicIDs = new Set();
  const privateKeyDigests = new Set();
  const result = new Map();
  for (const row of rows) {
    const identity = publicDeviceIdentity(row.deviceRoot, row.browser);
    if (publicIDs.has(identity.deviceId)) throw new Error("Chrome and Safari Device roots must have different public device_id values");
    if (privateKeyDigests.has(identity.privateKeyDigest)) throw new Error("Chrome and Safari daemon identity private keys must be distinct");
    publicIDs.add(identity.deviceId);
    privateKeyDigests.add(identity.privateKeyDigest);
    result.set(row.browser, identity.deviceId);
  }
  return result;
}

async function collectContext(manifest, frozenAt = new Date().toISOString()) {
  if (process.platform !== "darwin" || process.arch !== "arm64") throw new Error("P2 browser receipts require native macOS arm64");
  canonicalTimestamp(frozenAt, "frozenAt");
  const head = command("/usr/bin/git", ["rev-parse", "HEAD"], "read Git HEAD");
  if (head !== manifest.implementationSha) throw new Error("working HEAD differs from manifest implementation SHA");
  if (command("/usr/bin/git", ["status", "--porcelain=v1", "--untracked-files=all"], "inspect Git worktree") !== "") throw new Error("P2 browser receipt requires a clean final implementation worktree");
  const architecture = command("/usr/bin/uname", ["-m"], "read architecture");
  if (architecture !== "arm64") throw new Error("P2 browser receipt requires arm64 hardware");
  const os = {
    productVersion: command("/usr/bin/sw_vers", ["-productVersion"], "read macOS product version"),
    buildVersion: command("/usr/bin/sw_vers", ["-buildVersion"], "read macOS build version"),
    architecture,
  };

  const cli = await cliBuildIdentity(manifest.cliBinary, manifest.implementationSha);
  const resolvedRows = await validateRowIsolation(manifest.rows);
  const deviceIDs = validateDeviceIdentities(manifest.rows);
  const rows = [];
  for (const row of manifest.rows) {
    const paths = resolvedRows.get(row.browser);
    for (const [key, kind] of [["serverConfig", "file"], ["databasePath", "file"], ["cursorKeyPath", "file"], ["tlsLeafPrivateKey", "file"], ["deviceRoot", "directory"]]) {
      assertPrivatePath(paths[key], kind, `${row.browser}.${key}`);
    }
    verifyPrivateRuntimeStorage(row);
    const deviceId = deviceIDs.get(row.browser);
    validateServerConfig(row);
    const ca = new X509Certificate(readFileSync(paths.temporaryCA));
    const leaf = new X509Certificate(readFileSync(paths.tlsLeafCertificate));
    const hostname = new URL(row.origin).hostname;
    const now = Date.now();
    if (!ca.ca || Date.parse(ca.validFrom) > now || Date.parse(ca.validTo) <= now ||
        !leaf.verify(ca.publicKey) || leaf.checkIssued(ca) !== true || leaf.checkHost(hostname) !== hostname ||
        leaf.subjectAltName !== `DNS:${hostname}` || Date.parse(leaf.validFrom) > now || Date.parse(leaf.validTo) <= now) {
      throw new Error(`${row.browser} TLS leaf is not a current CA-issued certificate for ${hostname}`);
    }
    command("/usr/bin/security", ["verify-cert", "-c", paths.tlsLeafCertificate, "-p", "ssl", "-s", hostname], `${row.browser} macOS system TLS trust`);
    const binaryVersion = command(paths.serverBinary, ["--version"], `${row.browser} server binary version`);
    if (!binaryVersion.includes(`(${manifest.implementationSha})`)) throw new Error(`${row.browser} server binary lacks the frozen BuildCommit`);
    const versionEndpoint = await probeVersion(row, manifest.implementationSha, leaf);
    const metadata = validateBrowserBundleMetadata(row, bundleMetadata(paths.browserBundle), paths.browserBundle, paths.browserExecutable);
    rows.push({
      browser: row.browser,
      origin: row.origin,
      rpId: row.rpId,
      deviceId,
      privateRuntimeStorageVerified: true,
      isolationPathDigests: Object.fromEntries(isolatedPathKeys.map((key) => [key, sha256Text(paths[key])])),
      server: { executable: paths.serverBinary, sha256: await sha256File(paths.serverBinary), versionOutput: binaryVersion, versionEndpoint },
      browserExecutable: {
        path: paths.browserExecutable,
        sha256: await sha256File(paths.browserExecutable),
        bundle: basename(paths.browserBundle),
        version: { product: metadata.product, build: metadata.build },
        codeIdentity: codeIdentity(row, paths.browserBundle, paths.browserExecutable),
      },
      tls: {
        temporaryCAFingerprintSHA256: ca.fingerprint256,
        temporaryCASHA256: await sha256File(paths.temporaryCA),
        leafFingerprintSHA256: leaf.fingerprint256,
        leafSubject: leaf.subject,
        leafIssuer: leaf.issuer,
        leafValidFrom: leaf.validFrom,
        leafValidTo: leaf.validTo,
        directValidatedRequest: true,
        macOSSystemTrustVerified: true,
        insecureVerificationBypass: false,
      },
    });
  }
  return validateFrozenContext({
    schemaVersion: 1,
    status: "FROZEN_PENDING_MANUAL_JOURNEY",
    frozenAt,
    implementationSha: manifest.implementationSha,
    cleanWorktree: true,
    os,
    cli,
    rows,
  });
}

export function validateFrozenContext(input) {
  rejectUnsafeJSON(input, "frozen context");
  const context = exactObject(input, contextKeys, "frozen context");
  if (context.schemaVersion !== 1 || context.status !== "FROZEN_PENDING_MANUAL_JOURNEY" || context.cleanWorktree !== true) {
    throw new Error("frozen context status/schema/worktree fields are invalid");
  }
  canonicalTimestamp(context.frozenAt, "frozen context frozenAt");
  if (typeof context.implementationSha !== "string" || !/^[0-9a-f]{40}$/u.test(context.implementationSha)) {
    throw new Error("frozen context implementationSha is invalid");
  }

  const os = exactObject(context.os, contextOSKeys, "frozen context os");
  nonEmptyString(os.productVersion, "frozen context macOS product version");
  nonEmptyString(os.buildVersion, "frozen context macOS build version");
  if (os.architecture !== "arm64") throw new Error("frozen context architecture must be arm64");

  const cli = exactObject(context.cli, contextCLIKeys, "frozen context cli");
  canonicalAbsolutePath(cli.path, "frozen context cli.path");
  exactSHA256(cli.sha256, "frozen context cli.sha256");
  if (cli.vcsRevision !== context.implementationSha || cli.vcsModified !== false) {
    throw new Error("frozen context CLI provenance differs from the implementation SHA");
  }

  if (!Array.isArray(context.rows) || context.rows.length !== 2) throw new Error("frozen context must have exact Chrome and Safari rows");
  const deviceIDs = new Set();
  context.rows.forEach((candidate, index) => {
    const browser = index === 0 ? "chrome" : "safari";
    const row = exactObject(candidate, contextRowKeys, `frozen context ${browser} row`);
    const authority = rowAuthority[browser];
    if (row.browser !== browser || row.origin !== authority.origin || row.rpId !== authority.rpId) {
      throw new Error(`frozen context row ${index} differs from frozen ${browser} authority/order`);
    }
    if (typeof row.deviceId !== "string" || !/^device_[0-9a-f]{32}$/u.test(row.deviceId) || deviceIDs.has(row.deviceId)) {
      throw new Error("frozen context rows must have distinct valid public device IDs");
    }
    deviceIDs.add(row.deviceId);
    exactBoolean(row.privateRuntimeStorageVerified, true, `${browser}.privateRuntimeStorageVerified`);

    const pathDigests = exactObject(row.isolationPathDigests, isolatedPathKeys, `${browser}.isolationPathDigests`);
    for (const key of isolatedPathKeys) exactSHA256(pathDigests[key], `${browser}.isolationPathDigests.${key}`);

    const server = exactObject(row.server, contextServerKeys, `${browser}.server`);
    canonicalAbsolutePath(server.executable, `${browser}.server.executable`);
    exactSHA256(server.sha256, `${browser}.server.sha256`);
    nonEmptyString(server.versionOutput, `${browser}.server.versionOutput`);
    const endpoint = exactObject(server.versionEndpoint, contextVersionEndpointKeys, `${browser}.server.versionEndpoint`);
    nonEmptyString(endpoint.version, `${browser}.server.versionEndpoint.version`);
    if (endpoint.commit !== context.implementationSha || endpoint.directLoopbackTLS !== true) {
      throw new Error(`${browser} version endpoint does not prove the frozen implementation over direct TLS`);
    }

    const executable = exactObject(row.browserExecutable, contextBrowserExecutableKeys, `${browser}.browserExecutable`);
    canonicalAbsolutePath(executable.path, `${browser}.browserExecutable.path`);
    exactSHA256(executable.sha256, `${browser}.browserExecutable.sha256`);
    if (executable.bundle !== (browser === "chrome" ? "Google Chrome.app" : "Safari.app")) {
      throw new Error(`${browser} frozen bundle name is invalid`);
    }
    const version = exactObject(executable.version, contextBrowserVersionKeys, `${browser}.browserExecutable.version`);
    nonEmptyString(version.product, `${browser} browser product version`);
    nonEmptyString(version.build, `${browser} browser build version`);
    const identity = exactObject(executable.codeIdentity, contextCodeIdentityKeys, `${browser}.browserExecutable.codeIdentity`);
    if (identity.identifier !== authority.bundleIdentifier) throw new Error(`${browser} frozen signing identifier is invalid`);
    nonEmptyString(identity.teamIdentifier, `${browser} browser team identifier`);
    nonEmptyString(identity.cdHash, `${browser} browser CDHash`);

    const tls = exactObject(row.tls, contextTLSKeys, `${browser}.tls`);
    exactX509Fingerprint(tls.temporaryCAFingerprintSHA256, `${browser}.tls.temporaryCAFingerprintSHA256`);
    exactSHA256(tls.temporaryCASHA256, `${browser}.tls.temporaryCASHA256`);
    exactX509Fingerprint(tls.leafFingerprintSHA256, `${browser}.tls.leafFingerprintSHA256`);
    for (const key of ["leafSubject", "leafIssuer", "leafValidFrom", "leafValidTo"]) nonEmptyString(tls[key], `${browser}.tls.${key}`);
    if (Number.isNaN(Date.parse(tls.leafValidFrom)) || Number.isNaN(Date.parse(tls.leafValidTo))) {
      throw new Error(`${browser} frozen TLS validity is invalid`);
    }
    exactBoolean(tls.directValidatedRequest, true, `${browser}.tls.directValidatedRequest`);
    exactBoolean(tls.macOSSystemTrustVerified, true, `${browser}.tls.macOSSystemTrustVerified`);
    exactBoolean(tls.insecureVerificationBypass, false, `${browser}.tls.insecureVerificationBypass`);
  });
  rejectSecretMarkers(context, "frozen context");
  return context;
}

export function stableContext(value) {
  return structuredClone(validateFrozenContext(value));
}

export function writeExclusiveJSON(path, value) {
  canonicalAbsolutePath(path, "output");
  const contents = `${JSON.stringify(value, null, 2)}\n`;
  for (const forbidden of forbiddenSecretMarkers) {
    if (contents.includes(forbidden)) throw new Error(`receipt contains forbidden secret marker ${forbidden}`);
  }
  writeFileSync(path, contents, { encoding: "utf8", flag: "wx", mode: 0o600 });
}

export function finalizeReceipt(originalInput, freshInput, chromeInput, safariInput, observedAt = new Date().toISOString()) {
  const observation = canonicalTimestamp(observedAt, "finalize observation");
  const original = validateFrozenContext(originalInput);
  const fresh = validateFrozenContext(freshInput);
  if (Date.parse(original.frozenAt) > Date.parse(observation)) throw new Error("frozen context timestamp is after finalization observation");
  if (JSON.stringify(stableContext(original)) !== JSON.stringify(stableContext(fresh))) {
    throw new Error("frozen implementation/environment changed before receipt finalization");
  }
  const chrome = validateJourney(chromeInput, "chrome", original.frozenAt, observation);
  const safari = validateJourney(safariInput, "safari", original.frozenAt, observation);
  return validateReceipt({
    schemaVersion: 1,
    status: "PASS",
    finalizedAt: observation,
    evidenceBoundary: receiptEvidenceBoundary,
    frozenContext: original,
    journeys: [chrome, safari],
    exclusions: {
      windowsBrowserAcceptance: false,
      ubuntuBrowserAcceptance: false,
      claudeAcceptance: false,
      apiKey: false,
      printMode: false,
      dollarBudget: false,
      usageCredits: false,
      webdriverOrEmulation: false,
    },
  });
}

export function validateReceipt(input) {
  rejectUnsafeJSON(input, "final receipt");
  const receipt = exactObject(input, receiptKeys, "final receipt");
  if (receipt.schemaVersion !== 1 || receipt.status !== "PASS" || receipt.evidenceBoundary !== receiptEvidenceBoundary) {
    throw new Error("final receipt status/schema/evidence boundary is invalid");
  }
  const finalizedAt = canonicalTimestamp(receipt.finalizedAt, "final receipt finalizedAt");
  const context = validateFrozenContext(receipt.frozenContext);
  if (!Array.isArray(receipt.journeys) || receipt.journeys.length !== 2) throw new Error("final receipt must contain exact Chrome and Safari journeys");
  validateJourney(receipt.journeys[0], "chrome", context.frozenAt, finalizedAt);
  validateJourney(receipt.journeys[1], "safari", context.frozenAt, finalizedAt);
  const exclusions = exactObject(receipt.exclusions, exclusionKeys, "final receipt exclusions");
  for (const key of exclusionKeys) exactBoolean(exclusions[key], false, `final receipt exclusions.${key}`);
  rejectSecretMarkers(receipt, "final receipt");
  return receipt;
}

function argumentsFor(argv) {
  const result = new Map();
  for (let index = 0; index < argv.length; index += 2) {
    const name = argv[index];
    const value = argv[index + 1];
    if (!name?.startsWith("--") || value === undefined || result.has(name)) throw new Error("arguments must be unique --name value pairs");
    result.set(name, value);
  }
  return result;
}

async function main() {
  const [mode, ...argv] = process.argv.slice(2);
  const args = argumentsFor(argv);
  const manifestPath = canonicalAbsolutePath(args.get("--manifest"), "--manifest");
  assertPrivatePath(manifestPath, "file", "--manifest");
  const manifest = validateManifest(readJSON(manifestPath, "manifest"));
  if (mode === "collect") {
    if (args.size !== 2 || !args.has("--out")) throw new Error("collect requires --manifest and --out");
    const output = canonicalAbsolutePath(args.get("--out"), "--out");
    if (relative(repoRoot, output) !== "" && !relative(repoRoot, output).startsWith("..")) throw new Error("frozen context must be written outside the Git worktree");
    writeExclusiveJSON(output, await collectContext(manifest));
    console.log(`frozen P2 browser context written to ${output}`);
    return;
  }
  if (mode !== "finalize" || args.size !== 5 || !args.has("--context") || !args.has("--chrome-journey") || !args.has("--safari-journey") || !args.has("--out")) {
    throw new Error("finalize requires --manifest, --context, --chrome-journey, --safari-journey, and --out");
  }
  const contextPath = canonicalAbsolutePath(args.get("--context"), "--context");
  assertPrivatePath(contextPath, "file", "--context");
  const original = validateFrozenContext(readJSON(contextPath, "frozen context"));
  const fresh = await collectContext(manifest, original.frozenAt);
  const chromeJourneyPath = canonicalAbsolutePath(args.get("--chrome-journey"), "--chrome-journey");
  const safariJourneyPath = canonicalAbsolutePath(args.get("--safari-journey"), "--safari-journey");
  assertPrivatePath(chromeJourneyPath, "file", "--chrome-journey");
  assertPrivatePath(safariJourneyPath, "file", "--safari-journey");
  const chrome = readJSON(chromeJourneyPath, "Chrome journey");
  const safari = readJSON(safariJourneyPath, "Safari journey");
  const receipt = finalizeReceipt(original, fresh, chrome, safari, new Date().toISOString());
  writeExclusiveJSON(canonicalAbsolutePath(args.get("--out"), "--out"), receipt);
  console.log(`final P2 browser receipt written to ${args.get("--out")}`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}
