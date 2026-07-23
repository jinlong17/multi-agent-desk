import { X509Certificate, createHash } from "node:crypto";
import {
  closeSync, constants as fsConstants, createReadStream, existsSync, fstatSync, lstatSync, openSync, readFileSync,
  readdirSync, readSync, realpathSync, statSync, writeFileSync,
} from "node:fs";
import { get as httpsGet } from "node:https";
import { isIP } from "node:net";
import { basename, dirname, isAbsolute, relative, resolve, sep } from "node:path";
import { pathToFileURL } from "node:url";
import { TextDecoder } from "node:util";
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
const manifestKeys = ["schemaVersion", "implementationSha", "cliBinary", "rows"];
const rowKeys = [
  "browser", "origin", "rpId", "serverBinary", "serverConfig", "databasePath", "serverProcessLockPath", "cursorKeyPath",
  "tlsLeafCertificate", "tlsLeafPrivateKey", "temporaryCA", "deviceRoot", "browserBundle", "browserExecutable",
  "runtimeRoot", "deviceDatabasePath", "transferRoot", "logRoot", "evidenceRoot", "serverPid", "daemonPid",
  "serverStdoutFifo", "serverStderrFifo", "daemonStdoutFifo", "daemonStderrFifo",
  "serverStdoutReaderPid", "serverStderrReaderPid", "daemonStdoutReaderPid", "daemonStderrReaderPid",
];
const journeyKeys = [
  "browser", "startedAt", "finishedAt", "bootstrap", "registration", "login", "recovery",
  "replacementPasskey", "passkeyDelete", "logout", "browserReportedSecureConnection",
  "tlsWarningBypassed", "viteProxyUsed", "corsWorkaroundUsed", "secretArtifactCaptured",
  "automationSubstituteUsed", "webdriverSubstituteUsed", "terminalRecordingUsed",
  "terminalScrollbackCleared", "manualOperatorConfirmed",
];
const safariJourneyKeys = [...journeyKeys, "authenticatorAttachment", "userVerificationObserved", "touchIdOperatorConfirmed"];
const isolatedPathKeys = [
  "serverConfig", "databasePath", "serverProcessLockPath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey", "deviceRoot",
  "runtimeRoot", "deviceDatabasePath", "transferRoot", "logRoot", "evidenceRoot", "serverStdoutFifo",
  "serverStderrFifo", "daemonStdoutFifo", "daemonStderrFifo",
];
const contextKeys = [
  "schemaVersion", "status", "frozenAt", "implementationSha", "manifestSha256", "receiptToolSha256",
  "cleanWorktree", "os", "cli", "rows",
];
const contextOSKeys = ["productVersion", "buildVersion", "architecture"];
const contextCLIKeys = ["path", "sha256", "vcsRevision", "vcsModified"];
const contextRowKeys = [
  "browser", "origin", "rpId", "deviceId", "privateRuntimeStorageVerified", "isolationPathDigests",
  "server", "browserExecutable", "tls", "processes", "consoleReaders",
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
const contextProcessKeys = ["server", "daemon"];
const consoleReaderKeys = ["serverStdout", "serverStderr", "daemonStdout", "daemonStderr"];
const processKeys = ["pid", "uid", "state", "startTime", "executable", "executableSha256", "argvDigest", "image", "database", "inputs", "stdout", "stderr"];
const serverProcessKeys = [...processKeys, "processLock"];
const readerProcessKeys = ["pid", "uid", "state", "startTime", "executable", "argvDigest", "stdin", "stdoutKind", "stderrKind", "terminalPathDigest"];
const processFDKeys = ["pathDigest", "kind", "device", "inode", "uid", "mode", "regularFile"];
const processImageKeys = ["pathDigest", "device", "inode", "sha256"];
const processDatabaseKeys = ["pathDigest", "device", "inode"];
const processInputKeys = ["role", "pathDigest", "device", "inode", "ctimeNs"];
const processLockKeys = ["pathDigest", "device", "inode", "uid", "mode", "linkCount", "size", "fd", "access", "lockStatus", "regularFile", "holderCount"];
const scanKeys = [
  "schemaVersion", "status", "policy", "startedAt", "finishedAt", "implementationSha", "manifestSha256",
  "frozenContextSha256", "receiptToolSha256", "rows",
];
const scanRowKeys = [
  "browser", "targetClasses", "secretClasses", "databaseAssertions", "logEvidence", "transferEvidence",
  "processLockEvidence", "postSnapshotSha256",
];
const processLockEvidenceKeys = ["boundToDeclaredServer", "exclusiveWholeFileLock", "ownerOnly", "singleLink", "empty", "holderCount", "inventorySha256"];
const targetEvidenceKeys = [
  "class", "pathCount", "regularFileCount", "bytesScanned", "readErrorCount", "unstableFileCount",
  "matchCount", "unexpectedPathCount", "inventorySha256",
];
const secretEvidenceKeys = ["class", "detectorCount", "syntheticCanaryCount", "matchCount", "ruleSetSha256"];
const databaseAssertionKeys = [
  "sqliteIntegrityCheckPassed", "serverLogicalQueryCount", "deviceLogicalQueryCount", "serverViolationCount",
  "deviceViolationCount", "activeCeremonyCount", "serverSidecarCount", "serverBackupCount", "deviceSidecarCount",
  "deviceBackupCount", "rawByteMatchCount",
];
const logEvidenceKeys = [
  "mode", "processCount", "fdBindingCount", "fifoCount", "regularSinkCount", "declaredRootRegularSinkCount",
  "unexpectedPathCount", "inventorySha256",
];
const transferEvidenceKeys = [
  "publicDescriptorCount", "publicReceiptCount", "transientChallengeCount", "transientProofCount",
  "recoveryArtifactCount", "unexpectedFileCount", "inventorySha256",
];
const secretScanBindingKeys = ["policy", "receiptSha256", "receipt"];
const receiptKeys = ["schemaVersion", "status", "finalizedAt", "evidenceBoundary", "frozenContext", "journeys", "secretScan", "exclusions"];
const exclusionKeys = [
  "windowsBrowserAcceptance", "ubuntuBrowserAcceptance", "claudeAcceptance", "apiKey", "printMode", "dollarBudget",
  "usageCredits", "webdriverOrEmulation",
];
const forbiddenSecretMarkers = ["MAD-RC1-", "__Host-mad_session=", '"csrfToken"', '"bootstrapToken"'];
const sha256Pattern = /^[0-9a-f]{64}$/u;
const x509FingerprintPattern = /^(?:[0-9a-f]{2}:){31}[0-9a-f]{2}$/iu;
const targetClassNames = Object.freeze(["server_database", "device_database", "runtime_residue", "logs", "transfers", "evidence"]);
const secretClassNames = Object.freeze(["bootstrap_token", "session_cookie", "csrf", "recovery_code", "webauthn_ceremony", "bootstrap_proof"]);
const bootstrapCapabilitySet = new Set([
  "mad.v1.device.enroll_approve", "mad.v1.device.enroll_request", "mad.v1.device.revoke", "mad.v1.metadata.read",
  "mad.v1.metadata.write", "mad.v1.presence.write", "mad.v1.session.command_ack", "mad.v1.session.command_claim",
  "mad.v1.session.command_result", "mad.v1.sync.pull", "mad.v1.sync.push",
]);
const fifoKeys = Object.freeze(["serverStdoutFifo", "serverStderrFifo", "daemonStdoutFifo", "daemonStderrFifo"]);
const readerPIDKeys = Object.freeze(["serverStdoutReaderPid", "serverStderrReaderPid", "daemonStdoutReaderPid", "daemonStderrReaderPid"]);
const receiptPolicy = "p2-secret-scan-v1";
const receiptEvidenceBoundary = "declared server, Device, runtime, log, transfer, and evidence roots are machine-scanned; declared server/daemon/cat processes are machine-checked for FIFO-to-live-TTY routing with no regular-file sink, and declared log roots are machine-checked for no regular-file sink; browser and OS Passkey stores, browser profiles, the operator recovery store, long-term private-key contents, PTY-master recording, Terminal scrollback/application behavior, OS screen capture, and other operator-side capture are outside that machine proof; terminal recording absence and scrollback clearing are explicit operator attestations; declared roots still receive auth-marker and metadata scans; browser journeys and Safari Touch ID remain explicit operator attestations; WebDriver, automation, and emulation are forbidden";

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
  if (manifest.schemaVersion !== 2) throw new Error("manifest.schemaVersion must be 2");
  if (typeof manifest.implementationSha !== "string" || !/^[0-9a-f]{40}$/u.test(manifest.implementationSha)) {
    throw new Error("manifest.implementationSha must be one full lowercase Git SHA");
  }
  canonicalAbsolutePath(manifest.cliBinary, "manifest.cliBinary");
  if (/\s/u.test(manifest.cliBinary)) throw new Error("manifest.cliBinary must not contain whitespace because argv is bound exactly");
  if (!Array.isArray(manifest.rows) || manifest.rows.length !== 2) throw new Error("manifest.rows must contain exact Chrome and Safari rows");
  const seen = new Set();
  const rows = manifest.rows.map((candidate, index) => {
    const row = exactObject(candidate, rowKeys, `manifest.rows[${index}]`);
    if (row.browser !== "chrome" && row.browser !== "safari" || seen.has(row.browser)) throw new Error("manifest must contain one Chrome and one Safari row");
    seen.add(row.browser);
    const authority = rowAuthority[row.browser];
    if (row.origin !== authority.origin || row.rpId !== authority.rpId) throw new Error(`${row.browser} origin/RP ID differs from frozen P2 authority`);
    for (const key of [
      "serverBinary", "serverConfig", "databasePath", "serverProcessLockPath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey",
      "temporaryCA", "deviceRoot", "browserBundle", "browserExecutable", "runtimeRoot", "deviceDatabasePath",
      "transferRoot", "logRoot", "evidenceRoot",
    ]) canonicalAbsolutePath(row[key], `${row.browser}.${key}`);
    if (row.serverProcessLockPath !== `${row.databasePath}.process.lock`) {
      throw new Error(`${row.browser}.serverProcessLockPath must equal databasePath + '.process.lock' exactly`);
    }
    for (const key of ["serverBinary", "serverConfig", "deviceRoot"]) {
      if (/\s/u.test(row[key])) throw new Error(`${row.browser}.${key} must not contain whitespace because argv is bound exactly`);
    }
    if (!Number.isSafeInteger(row.serverPid) || row.serverPid < 2 || !Number.isSafeInteger(row.daemonPid) || row.daemonPid < 2 || row.serverPid === row.daemonPid) {
      throw new Error(`${row.browser} serverPid and daemonPid must be distinct positive process IDs`);
    }
    for (const key of readerPIDKeys) {
      if (!Number.isSafeInteger(row[key]) || row[key] < 2) throw new Error(`${row.browser}.${key} must be a positive process ID`);
    }
    const processIDs = [row.serverPid, row.daemonPid, ...readerPIDKeys.map((key) => row[key])];
    if (new Set(processIDs).size !== processIDs.length) throw new Error(`${row.browser} writer and FIFO reader PIDs must all be distinct`);
    for (const key of fifoKeys) canonicalAbsolutePath(row[key], `${row.browser}.${key}`);
    if (row.deviceDatabasePath !== resolve(row.deviceRoot, "device.db")) throw new Error(`${row.browser}.deviceDatabasePath must be the Device root database`);
    for (const key of ["deviceRoot", "transferRoot", "logRoot", "evidenceRoot"]) {
      if (!pathContains(row.runtimeRoot, row[key])) throw new Error(`${row.browser}.${key} must be contained by its runtimeRoot`);
    }
    const subroots = [row.deviceRoot, row.transferRoot, row.logRoot, row.evidenceRoot];
    for (let left = 0; left < subroots.length; left += 1) {
      for (let right = left + 1; right < subroots.length; right += 1) {
        if (pathContains(subroots[left], subroots[right]) || pathContains(subroots[right], subroots[left])) {
          throw new Error(`${row.browser} Device, transfer, log, and evidence roots must not overlap or nest`);
        }
      }
    }
    if (["databasePath", "serverProcessLockPath", "serverConfig", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey"].some((key) => !pathContains(row.runtimeRoot, row[key]))) {
      throw new Error(`${row.browser} server state and keys must be contained by its runtimeRoot`);
    }
    for (const key of fifoKeys) {
      if (!pathContains(row.logRoot, row[key])) throw new Error(`${row.browser}.${key} must be contained by its logRoot`);
    }
    return row;
  });
  const isolated = ["serverConfig", "databasePath", "serverProcessLockPath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey", "deviceRoot"];
  for (const key of isolated) {
    if (rows[0][key] === rows[1][key]) throw new Error(`Chrome and Safari must use independent ${key}`);
  }
  return { schemaVersion: 2, implementationSha: manifest.implementationSha, cliBinary: manifest.cliBinary, rows: rows.sort((left, right) => left.browser.localeCompare(right.browser)) };
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
    "browserReportedSecureConnection", "terminalScrollbackCleared", "manualOperatorConfirmed",
  ]) {
    if (journey[key] !== true) throw new Error(`${browser}.${key} must be explicitly confirmed true`);
  }
  for (const key of [
    "tlsWarningBypassed", "viteProxyUsed", "corsWorkaroundUsed", "secretArtifactCaptured",
    "automationSubstituteUsed", "webdriverSubstituteUsed", "terminalRecordingUsed",
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

export function parseStrictJSON(input, label = "JSON", maxBytes = 1 << 20) {
  const bytes = Buffer.isBuffer(input) || ArrayBuffer.isView(input) ? Buffer.from(input.buffer, input.byteOffset, input.byteLength) : undefined;
  if (!bytes || bytes.length > maxBytes || bytes.includes(0)) throw new Error(`${label} exceeds its strict UTF-8/size boundary`);
  let text;
  try {
    text = new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes);
  } catch {
    throw new Error(`${label} contains invalid UTF-8`);
  }
  let index = 0;
  const whitespace = () => { while (text[index] === " " || text[index] === "\t" || text[index] === "\r" || text[index] === "\n") index += 1; };
  const stringValue = () => {
    const start = index;
    index += 1;
    let escaped = false;
    while (index < text.length) {
      const value = text[index];
      if (!escaped && value === '"') {
        index += 1;
        return JSON.parse(text.slice(start, index));
      }
      if (!escaped && value.charCodeAt(0) < 0x20) throw new Error(`${label} contains an invalid JSON string`);
      escaped = !escaped && value === "\\";
      if (value !== "\\") escaped = false;
      index += 1;
    }
    throw new Error(`${label} contains an unterminated JSON string`);
  };
  const value = (depth) => {
    if (depth > 32) throw new Error(`${label} exceeds maximum JSON depth`);
    whitespace();
    if (text[index] === '"') return stringValue();
    if (text[index] === "{") {
      index += 1;
      const object = Object.create(null);
      const keys = new Set();
      whitespace();
      if (text[index] === "}") { index += 1; return object; }
      while (true) {
        whitespace();
        if (text[index] !== '"') throw new Error(`${label} object key is invalid`);
        const key = stringValue();
        if (["__proto__", "constructor", "prototype"].includes(key)) throw new Error(`${label} contains a forbidden object key`);
        if (keys.has(key)) throw new Error(`${label} contains duplicate object key`);
        keys.add(key);
        whitespace();
        if (text[index] !== ":") throw new Error(`${label} object separator is invalid`);
        index += 1;
        object[key] = value(depth + 1);
        whitespace();
        if (text[index] === "}") { index += 1; return object; }
        if (text[index] !== ",") throw new Error(`${label} object delimiter is invalid`);
        index += 1;
      }
    }
    if (text[index] === "[") {
      index += 1;
      const array = [];
      whitespace();
      if (text[index] === "]") { index += 1; return array; }
      while (true) {
        array.push(value(depth + 1));
        whitespace();
        if (text[index] === "]") { index += 1; return array; }
        if (text[index] !== ",") throw new Error(`${label} array delimiter is invalid`);
        index += 1;
      }
    }
    for (const [token, result] of [["true", true], ["false", false], ["null", null]]) {
      if (text.startsWith(token, index)) { index += token.length; return result; }
    }
    const number = text.slice(index).match(/^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?/u)?.[0];
    if (!number) throw new Error(`${label} contains an invalid JSON value`);
    index += number.length;
    const parsed = Number(number);
    if (!Number.isFinite(parsed)) throw new Error(`${label} contains a non-finite number`);
    return parsed;
  };
  const parsed = value(0);
  whitespace();
  if (index !== text.length) throw new Error(`${label} contains trailing JSON data`);
  return parsed;
}

export function readJSON(path, label) {
  try {
    return parseStrictJSON(readFileSync(path), label);
  } catch (error) {
    throw new Error(`${label} is not valid strict JSON: ${error.message}`);
  }
}

function assertPath(path, kind, label) {
  const info = lstatSync(path);
  if (info.isSymbolicLink() || kind === "file" && !info.isFile() || kind === "directory" && !info.isDirectory() || kind === "fifo" && !info.isFIFO()) {
    throw new Error(`${label} is not a non-symlink ${kind}`);
  }
}

export function assertPrivatePath(path, kind, label) {
  assertPath(path, kind, label);
  if (process.platform !== "darwin") return;
  const info = lstatSync(path);
  const expectedMode = kind === "directory" ? 0o700 : 0o600;
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

export function canonicalJSONStringify(value) {
  rejectUnsafeJSON(value, "canonical JSON");
  if (value === null || typeof value !== "object") return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map((entry) => canonicalJSONStringify(entry)).join(",")}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonicalJSONStringify(value[key])}`).join(",")}}`;
}

function canonicalJSONDigest(value) {
  return sha256Text(canonicalJSONStringify(value));
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
      ["serverBinary", "file"], ["serverConfig", "file"], ["databasePath", "file"], ["serverProcessLockPath", "file"], ["cursorKeyPath", "file"],
      ["tlsLeafCertificate", "file"], ["tlsLeafPrivateKey", "file"], ["temporaryCA", "file"],
      ["deviceRoot", "directory"], ["deviceDatabasePath", "file"], ["runtimeRoot", "directory"],
      ["transferRoot", "directory"], ["logRoot", "directory"], ["evidenceRoot", "directory"],
      ["serverStdoutFifo", "fifo"], ["serverStderrFifo", "fifo"], ["daemonStdoutFifo", "fifo"], ["daemonStderrFifo", "fifo"],
      ["browserBundle", "directory", true], ["browserExecutable", "file", true],
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
  if (pathContains(chromePaths.runtimeRoot, safariPaths.runtimeRoot) || pathContains(safariPaths.runtimeRoot, chromePaths.runtimeRoot)) {
    throw new Error("Chrome and Safari runtime roots must not overlap or nest");
  }
  for (const key of ["serverConfig", "databasePath", "serverProcessLockPath", "cursorKeyPath", "tlsLeafCertificate", "tlsLeafPrivateKey"]) {
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
  assertPrivatePath(row.serverProcessLockPath, "file", `${row.browser} server process lock`);
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

function parseLsofRecords(output, pid, label) {
  const lines = output.split("\n").filter(Boolean);
  if (lines.shift() !== `p${pid}`) throw new Error(`${label} lsof output is not bound to the declared PID`);
  const result = [];
  let current;
  for (const line of lines) {
    const field = line[0];
    const value = line.slice(1);
    if (field === "f") {
      if (current) result.push(current);
      current = { fd: value };
    } else if (current && field === "a") current.access = value;
    else if (current && field === "l") current.lockStatus = value;
    else if (current && field === "t") current.kind = value;
    else if (current && field === "D") current.device = BigInt(value).toString();
    else if (current && field === "i") current.inode = BigInt(value).toString();
    else if (current && field === "n") current.path = value;
  }
  if (current) result.push(current);
  return result;
}

function uniqueNumericFDs(records, label) {
  const result = new Map();
  for (const record of records.filter((item) => /^\d+$/u.test(item.fd))) {
    if (result.has(record.fd)) throw new Error(`${label} lsof returned a duplicate numeric FD`);
    result.set(record.fd, record);
  }
  return result;
}

function liveProcessSnapshot(pid, label) {
  const output = command(
    "/bin/ps",
    ["-ww", "-p", String(pid), "-o", "pid=", "-o", "uid=", "-o", "state=", "-o", "lstart=", "-o", "comm="],
    `inspect ${label} process identity`,
  );
  const match = output.match(/^\s*(\d+)\s+(\d+)\s+([^\s]+)\s+([A-Z][a-z]{2}\s+[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+\d{4})\s+(.+)$/u);
  if (!match || Number(match[1]) !== pid || !Number.isSafeInteger(Number(match[2])) || Number.isNaN(Date.parse(match[4]))) {
    throw new Error(`${label} process identity is unavailable`);
  }
  return { pid, uid: Number(match[2]), state: match[3], startTime: new Date(match[4]).toISOString(), executable: match[5] };
}

function liveProcessInspector(pid, label) {
  if (process.platform !== "darwin") throw new Error("live P2 process/FIFO inspection is available only on native macOS");
  const before = liveProcessSnapshot(pid, label);
  const commandLine = command("/bin/ps", ["-ww", "-p", String(pid), "-o", "command="], `inspect ${label} argv`);
  if (!commandLine || /[\r\n'"\\]/u.test(commandLine)) throw new Error(`${label} process argv is unavailable or ambiguously quoted`);
  const argv = commandLine.split(/[ \t]+/u);
  const openFiles = parseLsofRecords(
    command("/usr/sbin/lsof", ["-n", "-a", "-p", String(pid), "-FnaDiftl"], `inspect ${label} open files`),
    pid,
    label,
  );
  const fds = uniqueNumericFDs(openFiles, label);
  const after = liveProcessSnapshot(pid, label);
  if (canonicalJSONStringify(before) !== canonicalJSONStringify(after)) throw new Error(`${label} process identity changed during inspection`);
  return { ...before, argv, openFiles, fds };
}

function parseGlobalLsofRecords(output, label) {
  const records = [];
  let pid;
  let processName;
  let current;
  const finish = () => {
    if (current) records.push({ pid, processName, ...current });
    current = undefined;
  };
  for (const line of output.split("\n").filter(Boolean)) {
    const field = line[0];
    const value = line.slice(1);
    if (field === "p") {
      finish();
      pid = Number(value);
      processName = undefined;
      if (!Number.isSafeInteger(pid) || pid < 2) throw new Error(`${label} global lsof PID is invalid`);
    } else if (field === "c") processName = value;
    else if (field === "f") {
      finish();
      if (!pid) throw new Error(`${label} global lsof record has no process`);
      current = { fd: value };
    } else if (current && field === "a") current.access = value;
    else if (current && field === "l") current.lockStatus = value;
    else if (current && field === "t") current.kind = value;
    else if (current && field === "D") current.device = BigInt(value).toString();
    else if (current && field === "i") current.inode = BigInt(value).toString();
    else if (current && field === "n") current.path = value;
  }
  finish();
  return records;
}

export function liveFIFOHolderInspector(path, label = "FIFO") {
  const info = lstatSync(path, { bigint: true });
  if (!info.isFIFO()) throw new Error(`${label} holder target is not a FIFO`);
  const targetPath = realpathSync.native(path);
  const output = command("/usr/sbin/lsof", ["-n", "-FpcfnaDit"], `inspect ${label} FIFO holders globally`);
  const pids = parseGlobalLsofRecords(output, label)
    .filter((record) => {
      if (record.kind !== "FIFO" || record.inode !== info.ino.toString() || typeof record.path !== "string") return false;
      try {
        return realpathSync.native(record.path) === targetPath;
      } catch {
        return false;
      }
    })
    .map((record) => record.pid);
  return [...new Set(pids)].sort((left, right) => left - right);
}

export function liveProcessLockHolderInspector(path, label = "server process lock") {
  const info = lstatSync(path, { bigint: true });
  if (!info.isFile() || info.isSymbolicLink()) throw new Error(`${label} holder target is not a regular file`);
  const targetPath = realpathSync.native(path);
  const output = command("/usr/sbin/lsof", ["-n", "-FpcfnaDitl"], `inspect ${label} holders globally`);
  return parseGlobalLsofRecords(output, label)
    .filter((record) => {
      if (!/^\d+$/u.test(record.fd) || record.kind !== "REG" || record.device !== info.dev.toString() ||
          record.inode !== info.ino.toString() || typeof record.path !== "string") return false;
      try {
        return realpathSync.native(record.path) === targetPath;
      } catch {
        return false;
      }
    })
    .map(({ pid, fd, access, lockStatus }) => ({ pid, fd, access, lockStatus }))
    .sort((left, right) => left.pid - right.pid || Number(left.fd) - Number(right.fd));
}

export function liveExclusiveProcessLockProbe(path, label = "server process lock") {
  const script = [
    "import fcntl, os, sys",
    "fd = os.open(sys.argv[1], os.O_RDWR)",
    "try:",
    "    fcntl.flock(fd, fcntl.LOCK_SH | fcntl.LOCK_NB)",
    "except BlockingIOError:",
    "    sys.exit(73)",
    "except Exception:",
    "    sys.exit(74)",
    "else:",
    "    fcntl.flock(fd, fcntl.LOCK_UN)",
    "    sys.exit(0)",
  ].join("\n");
  const result = spawnSync("/usr/bin/python3", ["-c", script, path], { cwd: repoRoot, encoding: "utf8", shell: false, timeout: 5_000 });
  if (result.error || ![0, 73].includes(result.status)) {
    throw new Error(`${label} exclusive-lock contention probe failed closed`);
  }
  return result.status === 73;
}

async function processLockBinding(path, inspected, pid, holderInspector, lockProbeInspector, label) {
  const info = lstatSync(path, { bigint: true });
  if (!info.isFile() || info.isSymbolicLink() || info.nlink !== 1n || info.size !== 0n ||
      (info.mode & 0o777n) !== 0o600n || typeof process.getuid !== "function" || info.uid !== BigInt(process.getuid())) {
    throw new Error(`${label} must be an owner-only 0600 single-link empty regular file`);
  }
  const targetPath = realpathSync.native(path);
  const matching = (inspected.openFiles ?? []).filter((record) => /^\d+$/u.test(record.fd) && record.kind === "REG" &&
    record.device === info.dev.toString() && record.inode === info.ino.toString() && typeof record.path === "string" &&
    !/\s\(deleted\)$/u.test(record.path) && realpathSync.native(record.path) === targetPath);
  if (matching.length !== 1 || matching[0].access !== "u" || !["W", " "].includes(matching[0].lockStatus)) {
    throw new Error(`${label} must be the declared server's unique numeric O_RDWR FD with a whole-file write lock`);
  }
  if (matching[0].lockStatus !== "W" && await lockProbeInspector(path, label) !== true) {
    throw new Error(`${label} has no provable exclusive whole-file flock`);
  }
  const holders = await holderInspector(path, label);
  if (!Array.isArray(holders) || holders.length !== 1 || holders[0].pid !== pid || holders[0].fd !== matching[0].fd ||
      holders[0].access !== "u" || holders[0].lockStatus !== matching[0].lockStatus) {
    throw new Error(`${label} must have exactly one global holder bound to the declared server PID and FD`);
  }
  return {
    pathDigest: sha256Text(targetPath),
    device: info.dev.toString(),
    inode: info.ino.toString(),
    uid: Number(info.uid),
    mode: "0600",
    linkCount: 1,
    size: 0,
    fd: matching[0].fd,
    access: "read_write",
    lockStatus: "whole_file_write",
    regularFile: true,
    holderCount: 1,
  };
}

function fifoBinding(path, fd, label, expectedAccess) {
  const info = lstatSync(path, { bigint: true });
  if (!info.isFIFO() || info.nlink !== 1n || (info.mode & 0o777n) !== 0o600n || typeof process.getuid !== "function" || info.uid !== BigInt(process.getuid())) {
    throw new Error(`${label} must be an owner-only 0600 FIFO with one link`);
  }
  if (!fd || fd.kind !== "FIFO" || fd.access !== expectedAccess || fd.device !== info.dev.toString() || fd.inode !== info.ino.toString() || realpathSync.native(fd.path) !== realpathSync.native(path)) {
    throw new Error(`${label} is not the declared process FD FIFO`);
  }
  return {
    pathDigest: sha256Text(realpathSync.native(path)),
    kind: "fifo",
    device: info.dev.toString(),
    inode: info.ino.toString(),
    uid: Number(info.uid),
    mode: "0600",
    regularFile: false,
  };
}

function immutableProcessInput(path, role, startedAt, label) {
  const info = lstatSync(path, { bigint: true });
  if (!info.isFile() || info.isSymbolicLink() || info.nlink !== 1n || info.ctimeNs >= BigInt(Date.parse(startedAt)) * 1_000_000n) {
    throw new Error(`${label} must be an immutable single-link snapshot created before process start`);
  }
  return { role, pathDigest: sha256Text(realpathSync.native(path)), device: info.dev.toString(), inode: info.ino.toString(), ctimeNs: info.ctimeNs.toString() };
}

export async function validateProcessBindings(row, paths, cliPath, options = {}) {
  const inspector = options.processInspector ?? liveProcessInspector;
  const holderInspector = options.fifoHolderInspector ?? liveFIFOHolderInspector;
  const lockHolderInspector = options.processLockHolderInspector ?? liveProcessLockHolderInspector;
  const lockProbeInspector = options.processLockProbeInspector ?? liveExclusiveProcessLockProbe;
  const declarations = [
    ["server", row.serverPid, paths.serverBinary, paths.databasePath, [paths.serverBinary, "--config", paths.serverConfig], [
      ["server_config", paths.serverConfig], ["cursor_key", paths.cursorKeyPath], ["tls_leaf_certificate", paths.tlsLeafCertificate],
      ["tls_leaf_private_key", paths.tlsLeafPrivateKey], ["temporary_ca", paths.temporaryCA],
    ], "serverStdoutFifo", "serverStderrFifo", true],
    ["daemon", row.daemonPid, cliPath, paths.deviceDatabasePath, [cliPath, "daemon", "serve", "--root", paths.deviceRoot], [
      ["daemon_identity", resolve(paths.deviceRoot, "daemon.identity.json")],
    ], "daemonStdoutFifo", "daemonStderrFifo", false],
  ];
  const result = {};
  const seenFIFOIdentities = new Set();
  for (const [name, pid, expectedExecutable, expectedDatabase, expectedArgv, declaredInputs, stdoutKey, stderrKey, expectsProcessLock] of declarations) {
    const inspected = await inspector(pid, `${row.browser} ${name}`);
    if (inspected.pid !== undefined && inspected.pid !== pid) throw new Error(`${row.browser} ${name} PID changed during inspection`);
    if (!Number.isSafeInteger(inspected.uid) || typeof process.getuid !== "function" || inspected.uid !== process.getuid() || typeof inspected.state !== "string" || !/^[A-Z]+[+<ENs]*$/u.test(inspected.state) || inspected.state.startsWith("Z")) {
      throw new Error(`${row.browser} ${name} process owner or state is invalid`);
    }
    const executable = canonicalRealPath(inspected.executable, "file", `${row.browser} ${name} executable`);
    if (executable !== expectedExecutable) throw new Error(`${row.browser} ${name} PID executable differs from the manifest binary`);
    if (canonicalJSONStringify(inspected.argv) !== canonicalJSONStringify(expectedArgv)) {
      throw new Error(`${row.browser} ${name} PID argv differs from the manifest invocation`);
    }
    const binaryInfo = lstatSync(expectedExecutable, { bigint: true });
    const images = inspected.openFiles ?? (inspected.image ? [inspected.image] : []);
    const matchingImages = images.filter((item) => item.fd === "txt" && item.kind === "REG" && item.path === expectedExecutable &&
      item.device === binaryInfo.dev.toString() && item.inode === binaryInfo.ino.toString());
    if (!binaryInfo.isFile() || binaryInfo.isSymbolicLink() || matchingImages.length !== 1 ||
        canonicalRealPath(matchingImages[0].path, "file", `${row.browser} ${name} running image`) !== expectedExecutable) {
      throw new Error(`${row.browser} ${name} running image vnode differs from the manifest binary`);
    }
    const databaseInfo = lstatSync(expectedDatabase, { bigint: true });
    const matchingDatabases = (inspected.openFiles ?? []).filter((item) => /^\d+$/u.test(item.fd) && item.kind === "REG" && item.path === expectedDatabase &&
      item.device === databaseInfo.dev.toString() && item.inode === databaseInfo.ino.toString());
    if (!databaseInfo.isFile() || databaseInfo.isSymbolicLink() || databaseInfo.nlink !== 1n || matchingDatabases.length !== 1 ||
        /\s\(deleted\)$/u.test(matchingDatabases[0].path) || canonicalRealPath(matchingDatabases[0].path, "file", `${row.browser} ${name} open database`) !== expectedDatabase) {
      throw new Error(`${row.browser} ${name} open database vnode differs from the manifest database`);
    }
    const executableSha256 = await sha256File(expectedExecutable);
    const startTime = canonicalTimestamp(inspected.startTime, `${row.browser} ${name} start time`);
    const inputs = declaredInputs.map(([role, path]) => immutableProcessInput(path, role, startTime, `${row.browser} ${name} ${role}`));
    const writableRegularFiles = (inspected.openFiles ?? []).filter((item) => /^\d+$/u.test(item.fd) && item.kind === "REG" && ["w", "u"].includes(item.access));
    const writableAllowlist = new Set([expectedDatabase, `${expectedDatabase}-wal`, `${expectedDatabase}-shm`, `${expectedDatabase}-journal`]);
    if (expectsProcessLock) writableAllowlist.add(paths.serverProcessLockPath);
    if (writableRegularFiles.some((item) => /\s\(deleted\)$/u.test(item.path) || !writableAllowlist.has(item.path))) {
      throw new Error(`${row.browser} ${name} has an undeclared writable regular-file FD`);
    }
    const lockInfo = lstatSync(paths.serverProcessLockPath, { bigint: true });
    const matchingLockFiles = (inspected.openFiles ?? []).filter((item) => /^\d+$/u.test(item.fd) && item.kind === "REG" &&
      item.device === lockInfo.dev.toString() && item.inode === lockInfo.ino.toString());
    if (!expectsProcessLock && matchingLockFiles.length !== 0) throw new Error(`${row.browser} daemon must not hold the server process lock`);
    const processLock = expectsProcessLock ? await processLockBinding(
      paths.serverProcessLockPath, inspected, pid, lockHolderInspector, lockProbeInspector, `${row.browser} server process lock`,
    ) : undefined;
    const stdout = fifoBinding(paths[stdoutKey], inspected.fds.get("1"), `${row.browser} ${name} stdout`, "w");
    const stderr = fifoBinding(paths[stderrKey], inspected.fds.get("2"), `${row.browser} ${name} stderr`, "w");
    for (const binding of [stdout, stderr]) {
      const identity = `${binding.device}:${binding.inode}`;
      if (seenFIFOIdentities.has(identity)) throw new Error(`${row.browser} process streams must use four distinct FIFOs`);
      seenFIFOIdentities.add(identity);
    }
    result[name] = {
      pid,
      uid: inspected.uid,
      state: inspected.state,
      startTime,
      executable,
      executableSha256,
      argvDigest: canonicalJSONDigest(expectedArgv),
      image: {
        pathDigest: sha256Text(expectedExecutable),
        device: binaryInfo.dev.toString(),
        inode: binaryInfo.ino.toString(),
        sha256: executableSha256,
      },
      database: {
        pathDigest: sha256Text(expectedDatabase),
        device: databaseInfo.dev.toString(),
        inode: databaseInfo.ino.toString(),
      },
      inputs,
      stdout,
      stderr,
      ...(processLock ? { processLock } : {}),
    };
  }
  const readerDeclarations = [
    ["serverStdout", row.serverStdoutReaderPid, "serverStdoutFifo", row.serverPid],
    ["serverStderr", row.serverStderrReaderPid, "serverStderrFifo", row.serverPid],
    ["daemonStdout", row.daemonStdoutReaderPid, "daemonStdoutFifo", row.daemonPid],
    ["daemonStderr", row.daemonStderrReaderPid, "daemonStderrFifo", row.daemonPid],
  ];
  const consoleReaders = {};
  for (const [name, pid, fifoKey, writerPid] of readerDeclarations) {
    const inspected = await inspector(pid, `${row.browser} ${name} reader`);
    if (inspected.pid !== undefined && inspected.pid !== pid) throw new Error(`${row.browser} ${name} reader PID changed during inspection`);
    if (!Number.isSafeInteger(inspected.uid) || typeof process.getuid !== "function" || inspected.uid !== process.getuid() || typeof inspected.state !== "string" || !/^[A-Z]+[+<ENs]*$/u.test(inspected.state) || inspected.state.startsWith("Z")) {
      throw new Error(`${row.browser} ${name} reader process owner or state is invalid`);
    }
    const executable = canonicalRealPath(inspected.executable, "file", `${row.browser} ${name} reader executable`);
    if (executable !== "/bin/cat") throw new Error(`${row.browser} ${name} FIFO reader must be /bin/cat`);
    if (canonicalJSONStringify(inspected.argv) !== canonicalJSONStringify(["/bin/cat"])) throw new Error(`${row.browser} ${name} FIFO reader argv must be exact /bin/cat with stdin redirection`);
    const stdin = fifoBinding(paths[fifoKey], inspected.fds.get("0"), `${row.browser} ${name} reader stdin`, "r");
    const stdout = inspected.fds.get("1");
    const stderr = inspected.fds.get("2");
    if (!stdout || !stderr || stdout.kind !== "CHR" || stderr.kind !== "CHR" || !["w", "u"].includes(stdout.access) || !["w", "u"].includes(stderr.access) ||
        stdout.device !== stderr.device || stdout.inode !== stderr.inode || !/^\/dev\/ttys[0-9]+$/u.test(stdout.path) || stdout.path !== stderr.path) {
      throw new Error(`${row.browser} ${name} reader output must go directly to one Terminal TTY`);
    }
    const holders = await holderInspector(paths[fifoKey], `${row.browser} ${name}`);
    const expectedHolders = [writerPid, pid].sort((left, right) => left - right);
    if (JSON.stringify(holders) !== JSON.stringify(expectedHolders)) throw new Error(`${row.browser} ${name} FIFO has an undeclared holder or tee`);
    consoleReaders[name] = {
      pid,
      uid: inspected.uid,
      state: inspected.state,
      startTime: canonicalTimestamp(inspected.startTime, `${row.browser} ${name} reader start time`),
      executable,
      argvDigest: canonicalJSONDigest(["/bin/cat", "<", paths[fifoKey]]),
      stdin,
      stdoutKind: "tty",
      stderrKind: "tty",
      terminalPathDigest: sha256Text(stdout.path),
    };
  }
  return { processes: result, consoleReaders };
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
  const manifestSha256 = canonicalJSONDigest(manifest);
  const receiptToolSha256 = await sha256File(resolve(import.meta.dirname, "p2-browser-receipt.mjs"));
  const resolvedRows = await validateRowIsolation(manifest.rows);
  const deviceIDs = validateDeviceIdentities(manifest.rows);
  const rows = [];
  for (const row of manifest.rows) {
    const paths = resolvedRows.get(row.browser);
    for (const [key, kind] of [
      ["serverConfig", "file"], ["databasePath", "file"], ["serverProcessLockPath", "file"], ["cursorKeyPath", "file"], ["tlsLeafPrivateKey", "file"],
      ["deviceRoot", "directory"], ["deviceDatabasePath", "file"], ["runtimeRoot", "directory"],
      ["transferRoot", "directory"], ["logRoot", "directory"], ["evidenceRoot", "directory"],
      ["serverStdoutFifo", "fifo"], ["serverStderrFifo", "fifo"], ["daemonStdoutFifo", "fifo"], ["daemonStderrFifo", "fifo"],
    ]) {
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
    const { processes, consoleReaders } = await validateProcessBindings(row, paths, cli.path);
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
      processes,
      consoleReaders,
    });
  }
  return validateFrozenContext({
    schemaVersion: 2,
    status: "FROZEN_PENDING_MANUAL_JOURNEY",
    frozenAt,
    implementationSha: manifest.implementationSha,
    manifestSha256,
    receiptToolSha256,
    cleanWorktree: true,
    os,
    cli,
    rows,
  });
}

export function validateFrozenContext(input) {
  rejectUnsafeJSON(input, "frozen context");
  const context = exactObject(input, contextKeys, "frozen context");
  if (context.schemaVersion !== 2 || context.status !== "FROZEN_PENDING_MANUAL_JOURNEY" || context.cleanWorktree !== true) {
    throw new Error("frozen context status/schema/worktree fields are invalid");
  }
  canonicalTimestamp(context.frozenAt, "frozen context frozenAt");
  if (typeof context.implementationSha !== "string" || !/^[0-9a-f]{40}$/u.test(context.implementationSha)) {
    throw new Error("frozen context implementationSha is invalid");
  }
  exactSHA256(context.manifestSha256, "frozen context manifestSha256");
  exactSHA256(context.receiptToolSha256, "frozen context receiptToolSha256");

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

    const processes = exactObject(row.processes, contextProcessKeys, `${browser}.processes`);
    for (const name of contextProcessKeys) {
      const processRecord = exactObject(processes[name], name === "server" ? serverProcessKeys : processKeys, `${browser}.processes.${name}`);
      if (!Number.isSafeInteger(processRecord.pid) || processRecord.pid < 2) throw new Error(`${browser}.${name}.pid is invalid`);
      if (!Number.isSafeInteger(processRecord.uid) || processRecord.uid < 0 || typeof processRecord.state !== "string" || !/^[A-Z]+[+<ENs]*$/u.test(processRecord.state) || processRecord.state.startsWith("Z")) throw new Error(`${browser}.${name} process owner or state is invalid`);
      canonicalTimestamp(processRecord.startTime, `${browser}.${name}.startTime`);
      canonicalAbsolutePath(processRecord.executable, `${browser}.${name}.executable`);
      exactSHA256(processRecord.executableSha256, `${browser}.${name}.executableSha256`);
      exactSHA256(processRecord.argvDigest, `${browser}.${name}.argvDigest`);
      const image = exactObject(processRecord.image, processImageKeys, `${browser}.${name}.image`);
      exactSHA256(image.pathDigest, `${browser}.${name}.image.pathDigest`);
      exactSHA256(image.sha256, `${browser}.${name}.image.sha256`);
      if (!/^[0-9]+$/u.test(image.device) || !/^[0-9]+$/u.test(image.inode) || image.sha256 !== processRecord.executableSha256) {
        throw new Error(`${browser}.${name}.image is not bound to the frozen executable`);
      }
      const database = exactObject(processRecord.database, processDatabaseKeys, `${browser}.${name}.database`);
      exactSHA256(database.pathDigest, `${browser}.${name}.database.pathDigest`);
      if (!/^[0-9]+$/u.test(database.device) || !/^[0-9]+$/u.test(database.inode)) throw new Error(`${browser}.${name}.database is not bound to a frozen vnode`);
      const expectedInputRoles = name === "server" ? ["server_config", "cursor_key", "tls_leaf_certificate", "tls_leaf_private_key", "temporary_ca"] : ["daemon_identity"];
      if (!Array.isArray(processRecord.inputs) || canonicalJSONStringify(processRecord.inputs.map((item) => item.role)) !== canonicalJSONStringify(expectedInputRoles)) throw new Error(`${browser}.${name}.inputs are incomplete or out of order`);
      for (const [index, value] of processRecord.inputs.entries()) {
        const input = exactObject(value, processInputKeys, `${browser}.${name}.inputs[${index}]`);
        exactSHA256(input.pathDigest, `${browser}.${name}.inputs[${index}].pathDigest`);
        if (input.role !== expectedInputRoles[index] || !/^[0-9]+$/u.test(input.device) || !/^[0-9]+$/u.test(input.inode) || !/^[0-9]+$/u.test(input.ctimeNs)) throw new Error(`${browser}.${name}.inputs[${index}] is invalid`);
      }
      for (const stream of ["stdout", "stderr"]) {
        const binding = exactObject(processRecord[stream], processFDKeys, `${browser}.${name}.${stream}`);
        exactSHA256(binding.pathDigest, `${browser}.${name}.${stream}.pathDigest`);
        if (binding.kind !== "fifo" || !/^[0-9]+$/u.test(binding.device) || !/^[0-9]+$/u.test(binding.inode) ||
            !Number.isSafeInteger(binding.uid) || binding.uid < 0 || binding.mode !== "0600" || binding.regularFile !== false) {
          throw new Error(`${browser}.${name}.${stream} is not an owner-only FIFO binding`);
        }
      }
      if (name === "server") {
        const lock = exactObject(processRecord.processLock, processLockKeys, `${browser}.server.processLock`);
        exactSHA256(lock.pathDigest, `${browser}.server.processLock.pathDigest`);
        if (!/^[0-9]+$/u.test(lock.device) || !/^[0-9]+$/u.test(lock.inode) || !Number.isSafeInteger(lock.uid) || lock.uid < 0 ||
            lock.mode !== "0600" || lock.linkCount !== 1 || lock.size !== 0 || !/^\d+$/u.test(lock.fd) || lock.access !== "read_write" ||
            lock.lockStatus !== "whole_file_write" || lock.regularFile !== true || lock.holderCount !== 1) {
          throw new Error(`${browser}.server.processLock is not an exclusive owner-only empty lock binding`);
        }
      }
    }
    if (processes.server.pid === processes.daemon.pid) throw new Error(`${browser} server and daemon PID must differ`);
    const readers = exactObject(row.consoleReaders, consoleReaderKeys, `${browser}.consoleReaders`);
    for (const name of consoleReaderKeys) {
      const reader = exactObject(readers[name], readerProcessKeys, `${browser}.consoleReaders.${name}`);
      if (!Number.isSafeInteger(reader.pid) || reader.pid < 2) throw new Error(`${browser}.${name} reader PID is invalid`);
      if (!Number.isSafeInteger(reader.uid) || reader.uid < 0 || typeof reader.state !== "string" || !/^[A-Z]+[+<ENs]*$/u.test(reader.state) || reader.state.startsWith("Z")) throw new Error(`${browser}.${name} reader process owner or state is invalid`);
      canonicalTimestamp(reader.startTime, `${browser}.${name}.reader.startTime`);
      if (reader.executable !== "/bin/cat") throw new Error(`${browser}.${name} reader executable must be /bin/cat`);
      exactSHA256(reader.argvDigest, `${browser}.${name}.reader.argvDigest`);
      const stdin = exactObject(reader.stdin, processFDKeys, `${browser}.${name}.reader.stdin`);
      exactSHA256(stdin.pathDigest, `${browser}.${name}.reader.stdin.pathDigest`);
      if (stdin.kind !== "fifo" || stdin.mode !== "0600" || stdin.regularFile !== false ||
          !/^[0-9]+$/u.test(stdin.device) || !/^[0-9]+$/u.test(stdin.inode)) {
        throw new Error(`${browser}.${name} reader stdin is not a FIFO binding`);
      }
      if (reader.stdoutKind !== "tty" || reader.stderrKind !== "tty") throw new Error(`${browser}.${name} reader must write directly to a TTY`);
      exactSHA256(reader.terminalPathDigest, `${browser}.${name}.reader.terminalPathDigest`);
    }
    const pidSet = new Set([processes.server.pid, processes.daemon.pid, ...consoleReaderKeys.map((name) => readers[name].pid)]);
    if (pidSet.size !== 6) throw new Error(`${browser} writer and FIFO reader PIDs must all differ`);
    for (const [readerName, writerName, streamName] of [
      ["serverStdout", "server", "stdout"], ["serverStderr", "server", "stderr"],
      ["daemonStdout", "daemon", "stdout"], ["daemonStderr", "daemon", "stderr"],
    ]) {
      if (canonicalJSONStringify(readers[readerName].stdin) !== canonicalJSONStringify(processes[writerName][streamName])) {
        throw new Error(`${browser}.${readerName} reader is not bound to its declared writer FIFO`);
      }
    }
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

export function validateReceiptOutputPath(manifest, output, label = "--out") {
  const absolute = canonicalAbsolutePath(output, label);
  const parent = dirname(absolute);
  const realParent = canonicalRealPath(parent, "directory", `${label} parent`);
  if (realParent !== parent || resolve(realParent, basename(absolute)) !== absolute) throw new Error(`${label} must not traverse a symbolic-link parent`);
  const scanRoots = manifest.rows.flatMap((row) => [row.runtimeRoot, row.deviceRoot, row.logRoot, row.transferRoot, row.evidenceRoot]);
  if (scanRoots.some((root) => pathContains(root, absolute))) throw new Error(`${label} must be outside every declared scan root`);
  return absolute;
}

const detectorRules = Object.freeze({
  bootstrap_token: [
    /Bootstrap token \(shown once; expires in 10 minutes\): [A-Za-z0-9_-]{43}/gu,
    /Bootstrap[ \t]+[A-Za-z0-9_-]{43}/gu,
    /["']bootstrapToken["'][ \t\r\n]*:[ \t\r\n]*["'][A-Za-z0-9_-]{43}["']/gu,
    /P2-BOOTSTRAP-CANARY-[A-Z0-9_-]+/gu,
  ],
  session_cookie: [
    /__Host-mad_session=[A-Za-z0-9_-]{43}/gu,
    /P2-SESSION-COOKIE-CANARY-[A-Z0-9_-]+/gu,
  ],
  csrf: [
    /["']csrfToken["'][ \t\r\n]*:[ \t\r\n]*["'][A-Za-z0-9_-]{43}["']/gu,
    /X-CSRF-Token[ \t]*:[ \t]*[A-Za-z0-9_-]{43}/gu,
    /P2-CSRF-CANARY-[A-Z0-9_-]+/gu,
  ],
  recovery_code: [
    /MAD-RC1-(?:[A-HJ-NP-Z2-9]{4}-){7}[A-HJ-NP-Z2-9]{4}/gu,
    /P2-RECOVERY-CANARY-[A-Z0-9_-]+/gu,
  ],
  webauthn_ceremony: [
    /["'](?:challenge|clientDataJSON|attestationObject|authenticatorData)["'][ \t\r\n]*:/gu,
    /P2-WEBAUTHN-CANARY-[A-Z0-9_-]+/gu,
  ],
  bootstrap_proof: [
    /["'](?:signingProof|exchangeProof)["'][ \t\r\n]*:/gu,
    /P2-BOOTSTRAP-PROOF-CANARY-[A-Z0-9_-]+/gu,
  ],
});
const detectorCanaries = Object.freeze({
  bootstrap_token: "Bootstrap token (shown once; expires in 10 minutes): AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  session_cookie: "P2-SESSION-COOKIE-CANARY-SELFTEST",
  csrf: "P2-CSRF-CANARY-SELFTEST",
  recovery_code: "P2-RECOVERY-CANARY-SELFTEST",
  webauthn_ceremony: "P2-WEBAUTHN-CANARY-SELFTEST",
  bootstrap_proof: "P2-BOOTSTRAP-PROOF-CANARY-SELFTEST",
});

export function detectorRuleDigest(name) {
  return canonicalJSONDigest(detectorRules[name].map((rule) => rule.source));
}

export function countDetectorMatches(contents) {
  const text = contents.toString("utf8");
  const counts = Object.fromEntries(secretClassNames.map((name) => [name, 0]));
  for (const name of secretClassNames) {
    for (const rule of detectorRules[name]) counts[name] += [...text.matchAll(rule)].length;
  }
  return counts;
}

function validateDetectorCanaries() {
  for (const name of secretClassNames) {
    const counts = countDetectorMatches(Buffer.from(detectorCanaries[name], "utf8"));
    if (counts[name] !== 1 || secretClassNames.some((other) => other !== name && counts[other] !== 0)) {
      throw new Error(`${name} detector synthetic canary self-test failed`);
    }
  }
}

function privateMode(info, kind, label) {
  if (process.platform !== "darwin") return;
  const expected = kind === "directory" ? 0o700n : 0o600n;
  if (typeof process.getuid !== "function" || info.uid !== BigInt(process.getuid()) || (info.mode & 0o777n) !== expected) {
    throw new Error(`${label} is not owner-only private storage`);
  }
}

function stableReadFile(path, label, hooks = {}) {
  const before = lstatSync(path, { bigint: true });
  if (!before.isFile() || before.isSymbolicLink() || before.nlink !== 1n) throw new Error(`${label} is not a single-link regular file`);
  privateMode(before, "file", label);
  if (before.size > 64n * 1024n * 1024n) throw new Error(`${label} exceeds the 64 MiB scan bound`);
  let descriptor;
  try {
    descriptor = openSync(path, fsConstants.O_RDONLY | (fsConstants.O_NOFOLLOW ?? 0));
    const opened = fstatSync(descriptor, { bigint: true });
    if (opened.dev !== before.dev || opened.ino !== before.ino || opened.nlink !== 1n) throw new Error(`${label} was replaced before scanning`);
    const contents = Buffer.alloc(Number(opened.size));
    let offset = 0;
    while (offset < contents.length) {
      const count = readSync(descriptor, contents, offset, contents.length - offset, offset);
      if (count === 0) throw new Error(`${label} changed length while scanning`);
      offset += count;
    }
    hooks.afterRead?.(path);
    const afterFD = fstatSync(descriptor, { bigint: true });
    const afterPath = lstatSync(path, { bigint: true });
    for (const key of ["dev", "ino", "size", "mtimeNs", "ctimeNs"]) {
      if (afterFD[key] !== before[key] || afterPath[key] !== before[key]) throw new Error(`${label} was unstable while scanning`);
    }
    return {
      path,
      pathDigest: sha256Text(realpathSync.native(path)),
      kind: "file",
      size: Number(before.size),
      device: before.dev.toString(),
      inode: before.ino.toString(),
      mode: (before.mode & 0o777n).toString(8).padStart(4, "0"),
      mtimeNs: before.mtimeNs.toString(),
      ctimeNs: before.ctimeNs.toString(),
      sha256: createHash("sha256").update(contents).digest("hex"),
      contents,
    };
  } finally {
    if (descriptor !== undefined) closeSync(descriptor);
  }
}

function walkPrivateTree(root, label, options = {}) {
  const rootReal = canonicalRealPath(root, "directory", label);
  const excludedRoots = (options.excludeRoots ?? []).map((path) => canonicalRealPath(path, "directory", `${label} exclusion`));
  const excludedFiles = new Set((options.excludeFiles ?? []).filter((path) => existsSync(path)).map((path) => canonicalRealPath(path, "file", `${label} excluded file`)));
  const entries = [];
  const visit = (directory, includeInInventory = false) => {
    const before = lstatSync(directory, { bigint: true });
    if (!before.isDirectory() || before.isSymbolicLink()) throw new Error(`${label} contains a non-directory traversal boundary`);
    privateMode(before, "directory", `${label} directory`);
    let descriptor;
    try {
      descriptor = openSync(directory, fsConstants.O_RDONLY | (fsConstants.O_NOFOLLOW ?? 0) | (fsConstants.O_DIRECTORY ?? 0));
      const opened = fstatSync(descriptor, { bigint: true });
      if (!opened.isDirectory() || opened.dev !== before.dev || opened.ino !== before.ino) throw new Error(`${label} directory was replaced before scanning`);
      for (const name of readdirSync(directory).sort()) {
        const path = resolve(directory, name);
        const info = lstatSync(path, { bigint: true });
        if (info.isSymbolicLink()) throw new Error(`${label} contains a symbolic link`);
        if (info.isDirectory()) {
          const real = realpathSync.native(path);
          if (!excludedRoots.includes(real)) visit(real, true);
        } else if (info.isFile()) {
          const real = realpathSync.native(path);
          if (!excludedFiles.has(real)) entries.push(stableReadFile(real, `${label} file`, options.fileHooks));
        } else if (info.isFIFO() && options.allowFifos === true) {
          privateMode(info, "file", `${label} FIFO`);
          if (info.nlink !== 1n) throw new Error(`${label} contains a hard-linked FIFO`);
          entries.push({
            path, pathDigest: sha256Text(realpathSync.native(path)), kind: "fifo", size: 0,
            device: info.dev.toString(), inode: info.ino.toString(), mode: "0600", mtimeNs: info.mtimeNs.toString(), ctimeNs: info.ctimeNs.toString(),
            sha256: sha256Text(""), contents: Buffer.alloc(0),
          });
        } else {
          throw new Error(`${label} contains an unsupported filesystem object`);
        }
      }
      options.directoryHooks?.afterRead?.(directory);
      const afterFD = fstatSync(descriptor, { bigint: true });
      const afterPath = lstatSync(directory, { bigint: true });
      for (const key of ["dev", "ino", "uid", "gid", "mode", "nlink", "size", "mtimeNs", "ctimeNs"]) {
        if (afterFD[key] !== before[key] || afterPath[key] !== before[key]) throw new Error(`${label} directory was unstable while scanning`);
      }
      if (includeInInventory) {
        entries.push({
          path: directory,
          pathDigest: sha256Text(realpathSync.native(directory)),
          kind: "directory",
          size: 0,
          device: before.dev.toString(),
          inode: before.ino.toString(),
          mode: (before.mode & 0o777n).toString(8).padStart(4, "0"),
          mtimeNs: before.mtimeNs.toString(),
          ctimeNs: before.ctimeNs.toString(),
          sha256: sha256Text(""),
          contents: Buffer.alloc(0),
        });
      }
    } finally {
      if (descriptor !== undefined) closeSync(descriptor);
    }
  };
  visit(rootReal);
  return entries.sort((left, right) => left.pathDigest.localeCompare(right.pathDigest));
}

function existingServerDatabaseFiles(row) {
  return [row.databasePath, ...["-wal", "-shm", "-journal"]
    .map((suffix) => `${row.databasePath}${suffix}`)
    .filter(existsSync)].sort();
}

function scanEntries(className, entries, unexpectedPathCount = 0) {
  const secretCounts = Object.fromEntries(secretClassNames.map((name) => [name, 0]));
  for (const entry of entries) {
    const counts = countDetectorMatches(entry.contents);
    for (const name of secretClassNames) secretCounts[name] += counts[name];
  }
  const matchCount = Object.values(secretCounts).reduce((sum, value) => sum + value, 0);
  const inventory = entries.map(({ pathDigest, kind, size, device, inode, mode, mtimeNs, ctimeNs }) => ({ pathDigest, kind, size, device, inode, mode, mtimeNs, ctimeNs }));
  return {
    evidence: {
      class: className,
      pathCount: entries.length,
      regularFileCount: entries.filter((entry) => entry.kind === "file").length,
      bytesScanned: entries.reduce((sum, entry) => sum + entry.size, 0),
      readErrorCount: 0,
      unstableFileCount: 0,
      matchCount,
      unexpectedPathCount,
      inventorySha256: canonicalJSONDigest(inventory),
    },
    secretCounts,
    inventory,
  };
}

export function decodeBase64URL32(value, label) {
  if (typeof value !== "string" || !/^[A-Za-z0-9_-]{43}$/u.test(value)) throw new Error(`${label} is not canonical Base64url-encoded 32 bytes`);
  const decoded = Buffer.from(value, "base64url");
  if (decoded.length !== 32 || decoded.toString("base64url") !== value) throw new Error(`${label} is not canonical Base64url-encoded 32 bytes`);
  return value;
}

function uuidV7(value, label) {
  if (typeof value !== "string" || !/^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u.test(value)) throw new Error(`${label} is not a canonical UUIDv7`);
  return value;
}

function rfc3339GoTimestamp(value, label) {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/u.test(value) || Number.isNaN(Date.parse(value))) throw new Error(`${label} is not a Go RFC3339 UTC timestamp`);
  return value;
}

function framedDigest(fields) {
  const framed = [];
  for (const field of fields) {
    const bytes = Buffer.isBuffer(field) ? field : Buffer.from(field);
    const length = Buffer.alloc(4);
    length.writeUInt32BE(bytes.length);
    framed.push(length, bytes);
  }
  return createHash("sha256").update(Buffer.concat(framed)).digest("base64url");
}

function validateDescriptorCryptography(anchor, label) {
  const signing = Buffer.from(anchor.signingPublicKey, "base64url");
  const exchange = Buffer.from(anchor.exchangePublicKey, "base64url");
  if (createHash("sha256").update(signing).digest("base64url") !== anchor.signingKeyDigest ||
      createHash("sha256").update(exchange).digest("base64url") !== anchor.exchangeKeyDigest) {
    throw new Error(`${label} public key digest binding failed`);
  }
  const pin = framedDigest(["multidesk-device-pin-v1", anchor.deviceId, signing, exchange]);
  if (pin !== anchor.pinDigest) throw new Error(`${label} device pin binding failed`);
  if (!Array.isArray(anchor.capabilities) || anchor.capabilities.length < 1 || anchor.capabilities.length > 12 ||
      anchor.capabilities.some((capability, index) => typeof capability !== "string" || !bootstrapCapabilitySet.has(capability) || index > 0 && capability <= anchor.capabilities[index - 1])) {
    throw new Error(`${label} capabilities are not a bounded sorted unique recognized set`);
  }
  const canonicalAssertion = canonicalJSONStringify(anchor.keyEnvelopeAssertion);
  return { canonicalAssertion, storageAssertionDigest: createHash("sha256").update(canonicalAssertion).digest("base64url") };
}

function classifyTransfer(entries, row) {
  let descriptor;
  let receipt;
  let transientChallengeCount = 0;
  let transientProofCount = 0;
  let recoveryArtifactCount = 0;
  let unexpectedFileCount = 0;
  for (const entry of entries) {
    let value;
    try { value = parseStrictJSON(entry.contents, `${row.browser} transfer`, 64 << 10); } catch { unexpectedFileCount += 1; continue; }
    const keys = Object.keys(value).sort();
    if (keys.join(",") === ["anchor", "serverOrigin", "version"].sort().join(",")) {
      if (descriptor) { unexpectedFileCount += 1; continue; }
      const top = exactObject(value, ["version", "serverOrigin", "anchor"], `${row.browser} descriptor`);
      const anchor = exactObject(top.anchor, [
        "deviceId", "kind", "name", "platform", "architecture", "clientVersion", "signingPublicKey", "exchangePublicKey",
        "signingKeyDigest", "exchangeKeyDigest", "pinDigest", "storageMode", "keyEnvelopeAssertion", "capabilities",
      ], `${row.browser} descriptor anchor`);
      const assertion = exactObject(anchor.keyEnvelopeAssertion, ["formatVersion", "keyRevision", "recordRevision", "status", "sealedAt"], `${row.browser} descriptor assertion`);
      if (top.version !== 1 || top.serverOrigin !== row.origin || anchor.kind !== "daemon" || anchor.storageMode !== "portable_vault_v1" ||
          !["darwin", "linux", "windows"].includes(anchor.platform) || assertion.formatVersion !== 1 || assertion.keyRevision !== 1 ||
          !Number.isSafeInteger(assertion.recordRevision) || assertion.recordRevision < 1 || assertion.status !== "pending") throw new Error(`${row.browser} public descriptor invariant failed`);
      uuidV7(anchor.deviceId, `${row.browser} descriptor deviceId`);
      rfc3339GoTimestamp(assertion.sealedAt, `${row.browser} descriptor sealedAt`);
      for (const key of ["signingPublicKey", "exchangePublicKey", "signingKeyDigest", "exchangeKeyDigest", "pinDigest"]) decodeBase64URL32(anchor[key], `${row.browser} descriptor ${key}`);
      validateDescriptorCryptography(anchor, `${row.browser} descriptor`);
      descriptor = { value: top, bytes: entry.contents };
    } else if (keys.join(",") === [
      "activatedAt", "anchorDeviceId", "ceremonyId", "exchangeKeyDigest", "exchangeProofDigest", "serverOrigin",
      "signingKeyDigest", "signingProofDigest", "storageAssertionDigest", "storageMode", "type", "userId", "version",
    ].sort().join(",")) {
      if (receipt) { unexpectedFileCount += 1; continue; }
      const publicReceipt = exactObject(value, [
        "activatedAt", "anchorDeviceId", "ceremonyId", "exchangeKeyDigest", "exchangeProofDigest", "serverOrigin",
        "signingKeyDigest", "signingProofDigest", "storageAssertionDigest", "storageMode", "type", "userId", "version",
      ], `${row.browser} activation receipt`);
      if (publicReceipt.version !== 1 || publicReceipt.type !== "bootstrap_commit_receipt" || publicReceipt.serverOrigin !== row.origin || publicReceipt.storageMode !== "portable_vault_v1") throw new Error(`${row.browser} public activation receipt invariant failed`);
      for (const key of ["anchorDeviceId", "ceremonyId", "userId"]) uuidV7(publicReceipt[key], `${row.browser} receipt ${key}`);
      for (const key of ["signingKeyDigest", "exchangeKeyDigest", "storageAssertionDigest", "signingProofDigest", "exchangeProofDigest"]) decodeBase64URL32(publicReceipt[key], `${row.browser} receipt ${key}`);
      rfc3339GoTimestamp(publicReceipt.activatedAt, `${row.browser} receipt activatedAt`);
      receipt = { value: publicReceipt, bytes: entry.contents };
    } else if (keys.includes("challenge") || keys.includes("passkeyCreationOptions")) transientChallengeCount += 1;
    else if (keys.includes("signingProof") || keys.includes("exchangeProof")) transientProofCount += 1;
    else if (keys.includes("codes") || entry.contents.includes("MAD-RC1-")) recoveryArtifactCount += 1;
    else unexpectedFileCount += 1;
  }
  if (!descriptor || !receipt) throw new Error(`${row.browser} transfers must retain exactly one public descriptor and one activation receipt`);
  if (descriptor.value.anchor.deviceId !== receipt.value.anchorDeviceId || descriptor.value.anchor.signingKeyDigest !== receipt.value.signingKeyDigest || descriptor.value.anchor.exchangeKeyDigest !== receipt.value.exchangeKeyDigest) {
    throw new Error(`${row.browser} public descriptor and activation receipt do not bind the same anchor`);
  }
  if (validateDescriptorCryptography(descriptor.value.anchor, `${row.browser} descriptor`).storageAssertionDigest !== receipt.value.storageAssertionDigest) {
    throw new Error(`${row.browser} descriptor assertion and receipt digest do not bind`);
  }
  return {
    publicDescriptorCount: 1,
    publicReceiptCount: 1,
    transientChallengeCount,
    transientProofCount,
    recoveryArtifactCount,
    unexpectedFileCount,
    inventorySha256: canonicalJSONDigest(entries.map(({ pathDigest, kind, size, device, inode, mode, mtimeNs, ctimeNs }) => ({ pathDigest, kind, size, device, inode, mode, mtimeNs, ctimeNs }))),
    descriptor,
    receipt,
  };
}

function sqliteFileIdentity(path, label) {
  if (canonicalRealPath(path, "file", label) !== path) throw new Error(`${label} must use its canonical database path`);
  assertPrivatePath(path, "file", label);
  const info = lstatSync(path, { bigint: true });
  if (!info.isFile() || info.isSymbolicLink() || info.nlink !== 1n) throw new Error(`${label} must be one regular file with no aliases`);
  return [info.dev, info.ino, info.size, info.mtimeNs, info.ctimeNs].map(String).join(":");
}

function sqliteOutput(path, sql, label, options = {}) {
  const before = sqliteFileIdentity(path, label);
  const runner = options.sqliteRunner ?? spawnSync;
  const result = runner("/usr/bin/sqlite3", ["-nofollow", "-readonly", "-batch", "-noheader", path, `PRAGMA secure_delete=ON; PRAGMA query_only=ON; ${sql}`], {
    cwd: repoRoot, encoding: "utf8", shell: false, maxBuffer: 4 << 20,
  });
  if (result.status !== 0 || result.error) throw new Error(`${label} logical SQLite assertion failed`);
  const after = sqliteFileIdentity(path, label);
  if (before !== after) throw new Error(`${label} identity changed during logical SQLite assertion`);
  const lines = result.stdout.trim().split("\n");
  if (lines.shift() !== "1") throw new Error(`${label} secure_delete could not be enabled and verified`);
  return lines.join("\n").trim();
}

function sqliteCount(path, sql, label, options = {}) {
  const value = sqliteOutput(path, sql, label, options);
  if (!/^[0-9]+$/u.test(value)) throw new Error(`${label} did not return one count`);
  return Number(value);
}

function expectedMigrationLedger(kind) {
  const directory = resolve(repoRoot, "migrations", kind);
  return readdirSync(directory).filter((name) => /^\d{4}_.+\.sql$/u.test(name)).sort().map((name, index) => {
    const contents = readFileSync(resolve(directory, name));
    return `${index + 1}|${name}|${createHash("sha256").update(contents).digest("hex")}`;
  }).join("\n");
}

const expectedServerTables = "anchor_devices,auth_audit_events,auth_idempotency_operations,bootstrap_receipts,bootstrap_state,browser_sessions,idempotency_records,passkeys,pre_user_audit_events,recovery_batches,recovery_codes,schema_migrations,server_metadata,users,webauthn_ceremonies";
const expectedDeviceTables = "accounts,approvals,audit_events,auth_enrollments,client_identities,controller_leases,controlplane_id_mappings,credential_instances,credential_materializations,credential_revocations,device_identity,idempotency_records,metadata_tombstones,remote_device_identities,runtime_profiles,schema_migrations,session_attachments,session_events,session_start_previews,sessions,usage_snapshots,usage_windows,vault_config,vault_items,workspaces";
const expectedAnchorColumns = "id,kind,name,platform,architecture,client_version,signing_public_key,exchange_public_key,signing_key_digest,exchange_key_digest,pin_digest,storage_mode,storage_assertion_json,storage_assertion_digest,capabilities_json,lifecycle,key_revision,revision,created_at,updated_at";
const expectedRemoteIdentityColumns = "id,server_origin,server_device_id,signing_public_key,exchange_public_key,signing_key_digest,exchange_key_digest,key_revision,record_revision,lifecycle,payload_algorithm,payload_nonce,payload_ciphertext,wrap_algorithm,wrap_nonce,wrapped_dek,aad_digest,plaintext_digest,bootstrap_receipt_json,bootstrap_receipt_digest,quarantine_reason,created_at,updated_at";

function schemaViolations(path, kind, options) {
  let violations = 0;
  const expectedLedger = expectedMigrationLedger(kind);
  const ledger = sqliteOutput(path, "SELECT version||'|'||name||'|'||lower(checksum) FROM schema_migrations ORDER BY version", `${kind} migration ledger`, options);
  if (ledger !== expectedLedger) violations += 1;
  violations += sqliteCount(path, "SELECT count(*) FROM schema_migrations WHERE length(applied_at)=0", `${kind} migration timestamps`, options);
  const tables = sqliteOutput(path, "SELECT group_concat(name,',') FROM (SELECT name FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name)", `${kind} table schema`, options);
  if (tables !== (kind === "server" ? expectedServerTables : expectedDeviceTables)) violations += 1;
  if (kind === "server") {
    if (sqliteOutput(path, "SELECT group_concat(name,',') FROM pragma_table_info('anchor_devices')", "server anchor schema", options) !== expectedAnchorColumns) violations += 1;
  } else {
    if (sqliteOutput(path, "SELECT group_concat(name,',') FROM pragma_table_info('remote_device_identities')", "device remote identity schema", options) !== expectedRemoteIdentityColumns) violations += 1;
    if (sqliteOutput(path, "SELECT group_concat(name,',') FROM pragma_table_info('controlplane_id_mappings')", "device mapping schema", options) !== "entity_type,local_id,server_id,created_at,updated_at") violations += 1;
  }
  return violations;
}

function validatePublicReceiptBytes(bytes, receipt, label) {
  const parsed = parseStrictJSON(bytes, label, 4096);
  if (canonicalJSONStringify(parsed) !== canonicalJSONStringify(receipt.value)) throw new Error(`${label} differs from the strict transfer receipt`);
  return parsed;
}

function parseSQLiteObject(value, label) {
  return parseStrictJSON(Buffer.from(value, "utf8"), label, 64 << 10);
}

function sameInstant(left, right) {
  return typeof left === "string" && typeof right === "string" && Date.parse(left) === Date.parse(right);
}

function base64URLHex(value) {
  return Buffer.from(value, "base64url").toString("hex");
}

export function databaseAssertions(row, transfer, options = {}) {
  const serverPath = row.databasePath;
  const devicePath = row.deviceDatabasePath;
  const output = (path, sql, label) => sqliteOutput(path, sql, label, options);
  const count = (path, sql, label) => sqliteCount(path, sql, label, options);
  if (output(serverPath, "PRAGMA quick_check", `${row.browser} server DB`) !== "ok" || output(devicePath, "PRAGMA quick_check", `${row.browser} Device DB`) !== "ok") {
    throw new Error(`${row.browser} SQLite quick_check failed`);
  }
  let serverViolations = 0;
  let deviceViolations = 0;
  serverViolations += count(serverPath, "SELECT count(*) FROM pragma_foreign_key_check", `${row.browser} server FK`);
  deviceViolations += count(devicePath, "SELECT count(*) FROM pragma_foreign_key_check", `${row.browser} Device FK`);
  serverViolations += count(serverPath, "SELECT ((SELECT user_version FROM pragma_user_version)<>3) + ((SELECT count(*) FROM schema_migrations)<>3) + ((SELECT ifnull(max(version),0) FROM schema_migrations)<>3) + ((SELECT secure_delete FROM pragma_secure_delete)<>1)", `${row.browser} server schema`);
  deviceViolations += count(devicePath, "SELECT ((SELECT user_version FROM pragma_user_version)<>8) + ((SELECT count(*) FROM schema_migrations)<>8) + ((SELECT ifnull(max(version),0) FROM schema_migrations)<>8) + ((SELECT secure_delete FROM pragma_secure_delete)<>1)", `${row.browser} Device schema`);
  serverViolations += schemaViolations(serverPath, "server", options);
  deviceViolations += schemaViolations(devicePath, "device", options);
  serverViolations += count(serverPath, "SELECT ((SELECT count(*) FROM bootstrap_state)<>1) + (SELECT count(*) FROM bootstrap_state WHERE token_digest IS NOT NULL OR token_expires_at IS NOT NULL)", `${row.browser} bootstrap state`);
  const activeCeremonyCount = count(serverPath, "SELECT count(*) FROM webauthn_ceremonies", `${row.browser} active ceremonies`);
  serverViolations += activeCeremonyCount;
  serverViolations += count(serverPath, "SELECT ((SELECT count(*) FROM recovery_batches WHERE status='active')<>1) + ((SELECT count(*) FROM recovery_codes)<>10) + (SELECT count(*) FROM recovery_codes WHERE length(salt)<>16 OR length(code_hash)<>32) + ((SELECT count(*) FROM recovery_codes WHERE status='consumed')<>1)", `${row.browser} recovery rows`);
  serverViolations += count(serverPath, "SELECT count(*) FROM browser_sessions WHERE length(token_digest)<>32 OR length(csrf_digest)<>32", `${row.browser} session digests`);
  serverViolations += count(serverPath, "SELECT count(*) FROM browser_sessions WHERE revoked_at IS NULL", `${row.browser} live sessions`);
  serverViolations += count(serverPath, "SELECT count(*) FROM auth_idempotency_operations WHERE state<>'committed' OR length(key_digest)<>32 OR length(request_identity_digest)<>32 OR length(actor_identity_digest)<>32 OR length(body_digest)<>32 OR (public_options_digest IS NOT NULL AND length(public_options_digest)<>32)", `${row.browser} idempotency rows`);
  serverViolations += count(serverPath, "SELECT count(*) FROM auth_idempotency_operations,json_tree(auth_idempotency_operations.public_receipt_json) WHERE lower(json_tree.key) IN ('csrftoken','bootstraptoken','recoverycodes','codes','challenge','signingproof','exchangeproof')", `${row.browser} public receipt secret keys`);
  serverViolations += count(serverPath, "SELECT ((SELECT count(*) FROM anchor_devices)<>1) + ((SELECT count(*) FROM bootstrap_receipts)<>1)", `${row.browser} bootstrap public rows`);
  deviceViolations += count(devicePath, `SELECT (count(*)<>1) + sum(server_origin<>'${row.origin.replaceAll("'", "''")}' OR lifecycle<>'active' OR quarantine_reason IS NOT NULL OR key_revision<>1 OR length(signing_public_key)<>32 OR length(exchange_public_key)<>32 OR length(signing_key_digest)<>32 OR length(exchange_key_digest)<>32 OR length(payload_nonce)<>12 OR length(wrap_nonce)<>12 OR length(wrapped_dek)<>48 OR length(aad_digest)<>32 OR length(plaintext_digest)<>32 OR bootstrap_receipt_json IS NULL OR length(bootstrap_receipt_digest)<>32) FROM remote_device_identities`, `${row.browser} remote identity`);
  deviceViolations += count(devicePath, "SELECT ((SELECT count(*) FROM remote_device_identities)<>(SELECT count(*) FROM controlplane_id_mappings WHERE entity_type='device'))", `${row.browser} remote identity mapping`);
  deviceViolations += count(devicePath, "SELECT count(*) FROM remote_device_identities,json_tree(remote_device_identities.bootstrap_receipt_json) WHERE lower(json_tree.key) IN ('csrftoken','bootstraptoken','recoverycodes','codes','challenge','signingproof','exchangeproof')", `${row.browser} Device public receipt secret keys`);

  const descriptor = transfer.descriptor.value;
  const anchor = descriptor.anchor;
  const assertion = anchor.keyEnvelopeAssertion;
  const receipt = transfer.receipt.value;
  const descriptorCrypto = validateDescriptorCryptography(anchor, `${row.browser} descriptor`);
  if (descriptorCrypto.storageAssertionDigest !== receipt.storageAssertionDigest) serverViolations += 1;
  const serverAnchor = parseSQLiteObject(output(serverPath, `SELECT json_object(
    'id',id,'kind',kind,'name',name,'platform',platform,'architecture',architecture,'clientVersion',client_version,
    'signingPublicKey',lower(hex(signing_public_key)),'exchangePublicKey',lower(hex(exchange_public_key)),
    'signingKeyDigest',lower(hex(signing_key_digest)),'exchangeKeyDigest',lower(hex(exchange_key_digest)),
    'pinDigest',lower(hex(pin_digest)),'storageMode',storage_mode,'storageAssertionJSON',storage_assertion_json,
    'storageAssertionDigest',lower(hex(storage_assertion_digest)),'capabilitiesJSON',capabilities_json,
    'lifecycle',lifecycle,'keyRevision',key_revision,'revision',revision,'createdAt',created_at,'updatedAt',updated_at)
    FROM anchor_devices`, `${row.browser} server anchor binding`), `${row.browser} server anchor binding`);
  let storedAssertion;
  let storedCapabilities;
  try {
    storedAssertion = parseStrictJSON(Buffer.from(serverAnchor.storageAssertionJSON, "utf8"), `${row.browser} stored storage assertion`, 4096);
    storedCapabilities = parseStrictJSON(Buffer.from(serverAnchor.capabilitiesJSON, "utf8"), `${row.browser} stored capabilities`, 4096);
  } catch {
    serverViolations += 1;
    storedAssertion = null;
    storedCapabilities = null;
  }
  const assertionDigestHex = typeof serverAnchor.storageAssertionJSON === "string" ? createHash("sha256").update(serverAnchor.storageAssertionJSON).digest("hex") : "";
  if (descriptor.serverOrigin !== row.origin || serverAnchor.id !== anchor.deviceId || serverAnchor.kind !== anchor.kind || serverAnchor.name !== anchor.name ||
      serverAnchor.platform !== anchor.platform || serverAnchor.architecture !== anchor.architecture || serverAnchor.clientVersion !== anchor.clientVersion ||
      serverAnchor.signingPublicKey !== base64URLHex(anchor.signingPublicKey) || serverAnchor.exchangePublicKey !== base64URLHex(anchor.exchangePublicKey) ||
      serverAnchor.signingKeyDigest !== base64URLHex(anchor.signingKeyDigest) || serverAnchor.exchangeKeyDigest !== base64URLHex(anchor.exchangeKeyDigest) ||
      serverAnchor.pinDigest !== base64URLHex(anchor.pinDigest) || serverAnchor.storageMode !== anchor.storageMode ||
      serverAnchor.storageAssertionJSON !== descriptorCrypto.canonicalAssertion || canonicalJSONStringify(storedAssertion) !== canonicalJSONStringify(assertion) || canonicalJSONStringify(storedCapabilities) !== canonicalJSONStringify(anchor.capabilities) ||
      serverAnchor.storageAssertionDigest !== assertionDigestHex || serverAnchor.storageAssertionDigest !== base64URLHex(receipt.storageAssertionDigest) ||
      serverAnchor.lifecycle !== "active" || serverAnchor.keyRevision !== assertion.keyRevision || serverAnchor.revision !== 1 ||
      !sameInstant(serverAnchor.createdAt, receipt.activatedAt) || !sameInstant(serverAnchor.updatedAt, receipt.activatedAt)) serverViolations += 1;

  const serverReceiptBinding = parseSQLiteObject(output(serverPath, `SELECT json_object('ceremonyId',ceremony_id,'userId',user_id,'anchorDeviceId',anchor_device_id,'createdAt',created_at) FROM bootstrap_receipts`, `${row.browser} server receipt row binding`), `${row.browser} server receipt row binding`);
  if (serverReceiptBinding.ceremonyId !== receipt.ceremonyId || serverReceiptBinding.userId !== receipt.userId ||
      serverReceiptBinding.anchorDeviceId !== receipt.anchorDeviceId || !sameInstant(serverReceiptBinding.createdAt, receipt.activatedAt)) serverViolations += 1;

  const deviceIdentity = parseSQLiteObject(output(devicePath, `SELECT json_object(
    'id',id,'serverOrigin',server_origin,'serverDeviceId',server_device_id,
    'signingPublicKey',lower(hex(signing_public_key)),'exchangePublicKey',lower(hex(exchange_public_key)),
    'signingKeyDigest',lower(hex(signing_key_digest)),'exchangeKeyDigest',lower(hex(exchange_key_digest)),
    'keyRevision',key_revision,'recordRevision',record_revision,'lifecycle',lifecycle,
    'receiptDigest',lower(hex(bootstrap_receipt_digest)),'quarantineReason',quarantine_reason,
    'createdAt',created_at,'updatedAt',updated_at) FROM remote_device_identities`, `${row.browser} Device identity binding`), `${row.browser} Device identity binding`);
  const mapping = parseSQLiteObject(output(devicePath, `SELECT json_object('entityType',entity_type,'localId',local_id,'serverId',server_id,'createdAt',created_at,'updatedAt',updated_at) FROM controlplane_id_mappings WHERE entity_type='device'`, `${row.browser} Device mapping binding`), `${row.browser} Device mapping binding`);
  if (deviceIdentity.serverOrigin !== descriptor.serverOrigin || deviceIdentity.serverDeviceId !== anchor.deviceId ||
      deviceIdentity.signingPublicKey !== base64URLHex(anchor.signingPublicKey) || deviceIdentity.exchangePublicKey !== base64URLHex(anchor.exchangePublicKey) ||
      deviceIdentity.signingKeyDigest !== base64URLHex(anchor.signingKeyDigest) || deviceIdentity.exchangeKeyDigest !== base64URLHex(anchor.exchangeKeyDigest) ||
      deviceIdentity.keyRevision !== assertion.keyRevision || deviceIdentity.recordRevision !== assertion.recordRevision + 1 || deviceIdentity.lifecycle !== "active" ||
      deviceIdentity.quarantineReason !== null || !sameInstant(deviceIdentity.createdAt, assertion.sealedAt) || !sameInstant(deviceIdentity.updatedAt, receipt.activatedAt) ||
      mapping.entityType !== "device" || mapping.localId !== deviceIdentity.id || mapping.serverId !== deviceIdentity.serverDeviceId ||
      !sameInstant(mapping.createdAt, deviceIdentity.createdAt) || !sameInstant(mapping.updatedAt, deviceIdentity.updatedAt)) deviceViolations += 1;

  const serverReceiptHex = output(serverPath, "SELECT hex(receipt_json) FROM bootstrap_receipts", `${row.browser} server receipt`);
  const serverDigestHex = output(serverPath, "SELECT lower(hex(receipt_digest)) FROM bootstrap_receipts", `${row.browser} server receipt digest`);
  const deviceReceiptHex = output(devicePath, "SELECT hex(bootstrap_receipt_json) FROM remote_device_identities", `${row.browser} Device receipt`);
  const deviceDigestHex = output(devicePath, "SELECT lower(hex(bootstrap_receipt_digest)) FROM remote_device_identities", `${row.browser} Device receipt digest`);
  const serverReceipt = Buffer.from(serverReceiptHex, "hex");
  const deviceReceipt = Buffer.from(deviceReceiptHex, "hex");
  validatePublicReceiptBytes(serverReceipt, transfer.receipt, `${row.browser} server DB receipt`);
  validatePublicReceiptBytes(deviceReceipt, transfer.receipt, `${row.browser} Device DB receipt`);
  const receiptDigest = createHash("sha256").update(serverReceipt).digest("hex");
  if (!serverReceipt.equals(deviceReceipt) || serverDigestHex !== receiptDigest || deviceDigestHex !== receiptDigest) serverViolations += 1;
  if (deviceIdentity.receiptDigest !== receiptDigest) deviceViolations += 1;
  return {
    sqliteIntegrityCheckPassed: true,
    serverLogicalQueryCount: 12,
    deviceLogicalQueryCount: 7,
    serverViolationCount: serverViolations,
    deviceViolationCount: deviceViolations,
    activeCeremonyCount,
    serverSidecarCount: ["-wal", "-shm", "-journal"].filter((suffix) => existsSync(`${serverPath}${suffix}`)).length,
    serverBackupCount: existingServerDatabaseFiles(row).filter((path) => ![serverPath, `${serverPath}-wal`, `${serverPath}-shm`, `${serverPath}-journal`].includes(path)).length,
    deviceSidecarCount: ["-wal", "-shm", "-journal"].filter((suffix) => existsSync(`${devicePath}${suffix}`)).length,
    deviceBackupCount: walkPrivateTree(row.deviceRoot, `${row.browser} Device inventory`).filter((entry) => entry.path.includes(`${sep}backups${sep}`)).length,
    rawByteMatchCount: 0,
  };
}

function evidenceUnexpectedCount(entries, browser, manifest, context, observedAt) {
  let manifestCount = 0;
  let contextCount = 0;
  let journeyCount = 0;
  let summaryCount = 0;
  let unexpected = 0;
  for (const entry of entries) {
    if (entry.kind !== "file") { unexpected += 1; continue; }
    let value;
    try { value = parseStrictJSON(entry.contents, `${browser} evidence`, 1 << 20); } catch { unexpected += 1; continue; }
    try {
      if (value.schemaVersion === 2 && value.implementationSha && value.cliBinary) {
        const evidenceManifest = validateManifest(value);
        if (canonicalJSONStringify(evidenceManifest) !== canonicalJSONStringify(manifest)) throw new Error("evidence manifest is cross-run");
        manifestCount += 1;
        continue;
      }
      if (value.schemaVersion === 2 && value.status === "FROZEN_PENDING_MANUAL_JOURNEY") {
        const evidenceContext = validateFrozenContext(value);
        if (canonicalJSONStringify(evidenceContext) !== canonicalJSONStringify(context)) throw new Error("evidence context is cross-run");
        contextCount += 1;
        continue;
      }
      if (value.browser === browser && Object.keys(value).length === (browser === "safari" ? safariJourneyKeys.length : journeyKeys.length)) {
        validateJourney(value, browser, context.frozenAt, observedAt);
        journeyCount += 1;
        continue;
      }
      if (value.schemaVersion === 1 && value.status === "PASS") {
        exactObject(value, ["schemaVersion", "status", "tests", "failures", "retainedRawLogs"], `${browser} sanitized summary`);
        if (!Number.isSafeInteger(value.tests) || value.tests < 1 || value.failures !== 0 || value.retainedRawLogs !== false) throw new Error("sanitized summary is invalid");
        summaryCount += 1;
        continue;
      }
    } catch { unexpected += 1; continue; }
    unexpected += 1;
  }
  if (manifestCount !== 1 || contextCount !== 1 || journeyCount !== 1 || summaryCount !== 1) {
    unexpected += Math.abs(1 - manifestCount) + Math.abs(1 - contextCount) + Math.abs(1 - journeyCount) + Math.abs(1 - summaryCount);
  }
  return unexpected;
}

export function validateMachineScanReceipt(input, context) {
  validateDetectorCanaries();
  rejectUnsafeJSON(input, "machine scan receipt");
  const scan = exactObject(input, scanKeys, "machine scan receipt");
  if (scan.schemaVersion !== 1 || scan.status !== "PASS" || scan.policy !== receiptPolicy) throw new Error("machine scan receipt status/schema/policy is invalid");
  canonicalTimestamp(scan.startedAt, "machine scan startedAt");
  canonicalTimestamp(scan.finishedAt, "machine scan finishedAt");
  if (Date.parse(scan.finishedAt) < Date.parse(scan.startedAt) || scan.implementationSha !== context.implementationSha || scan.manifestSha256 !== context.manifestSha256 || scan.receiptToolSha256 !== context.receiptToolSha256 || scan.frozenContextSha256 !== canonicalJSONDigest(context)) {
    throw new Error("machine scan receipt is stale or cross-environment");
  }
  if (!Array.isArray(scan.rows) || scan.rows.length !== 2) throw new Error("machine scan receipt must contain exact Chrome and Safari rows");
  scan.rows.forEach((candidate, index) => {
    const browser = index === 0 ? "chrome" : "safari";
    const row = exactObject(candidate, scanRowKeys, `${browser} machine scan row`);
    if (row.browser !== browser) throw new Error("machine scan row order/browser mismatch");
    if (!Array.isArray(row.targetClasses) || row.targetClasses.length !== targetClassNames.length || !Array.isArray(row.secretClasses) || row.secretClasses.length !== secretClassNames.length) throw new Error(`${browser} machine scan class inventory is incomplete`);
    row.targetClasses.forEach((value, classIndex) => {
      const evidence = exactObject(value, targetEvidenceKeys, `${browser} target evidence`);
      if (evidence.class !== targetClassNames[classIndex] || !Number.isSafeInteger(evidence.pathCount) || evidence.pathCount < 1 ||
          !Number.isSafeInteger(evidence.regularFileCount) || evidence.regularFileCount < 0 || !Number.isSafeInteger(evidence.bytesScanned) || evidence.bytesScanned < 0 ||
          evidence.readErrorCount !== 0 || evidence.unstableFileCount !== 0 || evidence.matchCount !== 0 || evidence.unexpectedPathCount !== 0) throw new Error(`${browser} target evidence is not a complete zero-finding scan`);
      exactSHA256(evidence.inventorySha256, `${browser} target inventory digest`);
    });
    row.secretClasses.forEach((value, classIndex) => {
      const evidence = exactObject(value, secretEvidenceKeys, `${browser} secret evidence`);
      if (evidence.class !== secretClassNames[classIndex] || evidence.detectorCount !== detectorRules[evidence.class].length || evidence.syntheticCanaryCount !== 1 || evidence.matchCount !== 0 || evidence.ruleSetSha256 !== detectorRuleDigest(evidence.class)) throw new Error(`${browser} secret detector evidence is incomplete`);
    });
    const database = exactObject(row.databaseAssertions, databaseAssertionKeys, `${browser} database assertions`);
    for (const key of databaseAssertionKeys.filter((key) => key !== "sqliteIntegrityCheckPassed")) {
      if (!Number.isSafeInteger(database[key]) || database[key] < 0) throw new Error(`${browser} database assertion ${key} is not a nonnegative integer`);
    }
    if (database.sqliteIntegrityCheckPassed !== true || database.serverLogicalQueryCount < 1 || database.deviceLogicalQueryCount < 1 || database.serverViolationCount !== 0 || database.deviceViolationCount !== 0 || database.activeCeremonyCount !== 0 || database.serverBackupCount !== 0 || database.rawByteMatchCount !== 0) throw new Error(`${browser} logical database assertions did not pass`);
    const logs = exactObject(row.logEvidence, logEvidenceKeys, `${browser} log evidence`);
    if (logs.mode !== "fifo_to_live_tty" || logs.processCount !== 6 || logs.fdBindingCount !== 8 || logs.fifoCount !== 4 || logs.regularSinkCount !== 0 || logs.declaredRootRegularSinkCount !== 0 || logs.unexpectedPathCount !== 0) throw new Error(`${browser} declared-process/root log-sink proof is incomplete`);
    exactSHA256(logs.inventorySha256, `${browser} log inventory digest`);
    const transfers = exactObject(row.transferEvidence, transferEvidenceKeys, `${browser} transfer evidence`);
    if (transfers.publicDescriptorCount !== 1 || transfers.publicReceiptCount !== 1 || transfers.transientChallengeCount !== 0 || transfers.transientProofCount !== 0 || transfers.recoveryArtifactCount !== 0 || transfers.unexpectedFileCount !== 0) throw new Error(`${browser} transfer cleanup is incomplete`);
    exactSHA256(transfers.inventorySha256, `${browser} transfer inventory digest`);
    const processLock = exactObject(row.processLockEvidence, processLockEvidenceKeys, `${browser} process lock evidence`);
    if (processLock.boundToDeclaredServer !== true || processLock.exclusiveWholeFileLock !== true || processLock.ownerOnly !== true ||
        processLock.singleLink !== true || processLock.empty !== true || processLock.holderCount !== 1) {
      throw new Error(`${browser} server process lock evidence is incomplete`);
    }
    exactSHA256(processLock.inventorySha256, `${browser} process lock inventory digest`);
    exactSHA256(row.postSnapshotSha256, `${browser} post snapshot digest`);
  });
  rejectSecretMarkers(scan, "machine scan receipt");
  return scan;
}

export async function scanEnvironment(manifest, context, options = {}) {
  validateDetectorCanaries();
  const validatedContext = validateFrozenContext(context);
  if (canonicalJSONDigest(manifest) !== validatedContext.manifestSha256 || await sha256File(resolve(import.meta.dirname, "p2-browser-receipt.mjs")) !== validatedContext.receiptToolSha256) throw new Error("scan implementation, manifest, or receipt tool differs from frozen context");
  const startedAt = canonicalTimestamp(options.startedAt ?? new Date().toISOString(), "machine scan startedAt");
  const resolvedRows = await validateRowIsolation(manifest.rows);
  const rows = [];
  for (const row of manifest.rows) {
    const paths = resolvedRows.get(row.browser);
    const processEvidence = await validateProcessBindings(row, paths, validatedContext.cli.path, options);
    if (canonicalJSONStringify(processEvidence.processes) !== canonicalJSONStringify(validatedContext.rows.find((candidate) => candidate.browser === row.browser).processes) ||
        canonicalJSONStringify(processEvidence.consoleReaders) !== canonicalJSONStringify(validatedContext.rows.find((candidate) => candidate.browser === row.browser).consoleReaders)) throw new Error(`${row.browser} process/FIFO binding changed after freeze`);

    const serverFiles = existingServerDatabaseFiles(row);
    const serverEntries = serverFiles.map((path) => stableReadFile(path, `${row.browser} server database artifact`, options.scanFileHooks));
    const deviceEntries = walkPrivateTree(row.deviceRoot, `${row.browser} Device database/runtime`, { fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks });
    const logEntries = walkPrivateTree(row.logRoot, `${row.browser} logs`, { allowFifos: true, fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks });
    const transferEntries = walkPrivateTree(row.transferRoot, `${row.browser} transfers`, { fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks });
    const evidenceEntries = walkPrivateTree(row.evidenceRoot, `${row.browser} evidence`, { fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks });
    const runtimeEntries = walkPrivateTree(row.runtimeRoot, `${row.browser} runtime residue`, {
      excludeRoots: [row.deviceRoot, row.logRoot, row.transferRoot, row.evidenceRoot],
      excludeFiles: serverFiles,
      fileHooks: options.scanFileHooks,
      directoryHooks: options.scanDirectoryHooks,
    });
    const transfers = classifyTransfer(transferEntries, row);
    const allowedRuntimeFiles = new Set([
      paths.serverConfig, paths.serverProcessLockPath, paths.cursorKeyPath, paths.tlsLeafCertificate, paths.tlsLeafPrivateKey,
    ]);
    const unexpectedRuntime = runtimeEntries.filter((entry) => !allowedRuntimeFiles.has(entry.path)).length;
    const unexpectedLogs = logEntries.filter((entry) => entry.kind !== "fifo").length;
    const unexpectedEvidence = evidenceUnexpectedCount(evidenceEntries, row.browser, manifest, validatedContext, startedAt);
    const scans = [
      scanEntries("server_database", serverEntries),
      scanEntries("device_database", deviceEntries),
      scanEntries("runtime_residue", runtimeEntries, unexpectedRuntime),
      scanEntries("logs", logEntries, unexpectedLogs),
      scanEntries("transfers", transferEntries, transfers.transientChallengeCount + transfers.transientProofCount + transfers.recoveryArtifactCount + transfers.unexpectedFileCount),
      scanEntries("evidence", evidenceEntries, unexpectedEvidence),
    ];
    const secretCounts = Object.fromEntries(secretClassNames.map((name) => [name, scans.reduce((sum, scan) => sum + scan.secretCounts[name], 0)]));
    if (options.scanEvidenceObserver) {
      const observation = Object.freeze({
        browser: row.browser,
        targetClasses: Object.freeze(scans.map((scan) => Object.freeze({ ...scan.evidence }))),
        secretCounts: Object.freeze({ ...secretCounts }),
      });
      options.scanEvidenceObserver(observation);
    }
    const assertions = (options.databaseAssertionRunner ?? databaseAssertions)(row, transfers);
    assertions.rawByteMatchCount = Object.values(secretCounts).reduce((sum, value) => sum + value, 0);
    const logEvidence = {
      mode: "fifo_to_live_tty", processCount: 6, fdBindingCount: 8, fifoCount: logEntries.filter((entry) => entry.kind === "fifo").length,
      regularSinkCount: 0, declaredRootRegularSinkCount: unexpectedLogs, unexpectedPathCount: unexpectedLogs,
      inventorySha256: scans[3].evidence.inventorySha256,
    };
    const transferEvidence = {
      publicDescriptorCount: transfers.publicDescriptorCount, publicReceiptCount: transfers.publicReceiptCount,
      transientChallengeCount: transfers.transientChallengeCount, transientProofCount: transfers.transientProofCount,
      recoveryArtifactCount: transfers.recoveryArtifactCount, unexpectedFileCount: transfers.unexpectedFileCount,
      inventorySha256: transfers.inventorySha256,
    };
    const lockEntry = runtimeEntries.find((entry) => entry.path === paths.serverProcessLockPath);
    const frozenServerLock = processEvidence.processes.server.processLock;
    if (!lockEntry || lockEntry.kind !== "file" || lockEntry.pathDigest !== frozenServerLock.pathDigest ||
        lockEntry.device !== frozenServerLock.device || lockEntry.inode !== frozenServerLock.inode || lockEntry.mode !== frozenServerLock.mode ||
        lockEntry.size !== frozenServerLock.size) {
      throw new Error(`${row.browser} runtime scan is not bound to the declared server process lock vnode`);
    }
    const processLockEvidence = {
      boundToDeclaredServer: true,
      exclusiveWholeFileLock: frozenServerLock.lockStatus === "whole_file_write",
      ownerOnly: frozenServerLock.mode === "0600",
      singleLink: frozenServerLock.linkCount === 1,
      empty: lockEntry.size === 0,
      holderCount: frozenServerLock.holderCount,
      inventorySha256: canonicalJSONDigest({
        pathDigest: lockEntry.pathDigest, kind: lockEntry.kind, size: lockEntry.size, device: lockEntry.device,
        inode: lockEntry.inode, mode: lockEntry.mode, mtimeNs: lockEntry.mtimeNs, ctimeNs: lockEntry.ctimeNs,
      }),
    };
    const postSnapshotSha256 = canonicalJSONDigest(scans.map((scan) => scan.inventory));
    if (options.afterInitialSnapshot) await options.afterInitialSnapshot(row.browser);
    const secondSnapshot = [
      scanEntries("server_database", existingServerDatabaseFiles(row).map((path) => stableReadFile(path, `${row.browser} server database rescan`, options.scanFileHooks))).inventory,
      scanEntries("device_database", walkPrivateTree(row.deviceRoot, `${row.browser} Device rescan`, { fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks })).inventory,
      scanEntries("runtime_residue", walkPrivateTree(row.runtimeRoot, `${row.browser} runtime rescan`, { excludeRoots: [row.deviceRoot, row.logRoot, row.transferRoot, row.evidenceRoot], excludeFiles: existingServerDatabaseFiles(row), fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks })).inventory,
      scanEntries("logs", walkPrivateTree(row.logRoot, `${row.browser} log rescan`, { allowFifos: true, fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks })).inventory,
      scanEntries("transfers", walkPrivateTree(row.transferRoot, `${row.browser} transfer rescan`, { fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks })).inventory,
      scanEntries("evidence", walkPrivateTree(row.evidenceRoot, `${row.browser} evidence rescan`, { fileHooks: options.scanFileHooks, directoryHooks: options.scanDirectoryHooks })).inventory,
    ];
    if (canonicalJSONDigest(secondSnapshot) !== postSnapshotSha256) throw new Error(`${row.browser} scan target changed during the machine scan`);
    rows.push({
      browser: row.browser,
      targetClasses: scans.map((scan) => scan.evidence),
      secretClasses: secretClassNames.map((name) => ({ class: name, detectorCount: detectorRules[name].length, syntheticCanaryCount: 1, matchCount: secretCounts[name], ruleSetSha256: detectorRuleDigest(name) })),
      databaseAssertions: assertions,
      logEvidence,
      transferEvidence,
      processLockEvidence,
      postSnapshotSha256,
    });
  }
  const finishedAt = canonicalTimestamp(options.finishedAt ?? new Date().toISOString(), "machine scan finishedAt");
  return validateMachineScanReceipt({
    schemaVersion: 1, status: "PASS", policy: receiptPolicy, startedAt, finishedAt,
    implementationSha: validatedContext.implementationSha, manifestSha256: validatedContext.manifestSha256,
    frozenContextSha256: canonicalJSONDigest(validatedContext), receiptToolSha256: validatedContext.receiptToolSha256, rows,
  }, validatedContext);
}

export function finalizeReceipt(originalInput, freshInput, chromeInput, safariInput, scanInput, freshScanInput, observedAt = new Date().toISOString()) {
  const observation = canonicalTimestamp(observedAt, "finalize observation");
  const original = validateFrozenContext(originalInput);
  const fresh = validateFrozenContext(freshInput);
  if (Date.parse(original.frozenAt) > Date.parse(observation)) throw new Error("frozen context timestamp is after finalization observation");
  if (JSON.stringify(stableContext(original)) !== JSON.stringify(stableContext(fresh))) {
    throw new Error("frozen implementation/environment changed before receipt finalization");
  }
  const chrome = validateJourney(chromeInput, "chrome", original.frozenAt, observation);
  const safari = validateJourney(safariInput, "safari", original.frozenAt, observation);
  const scan = validateMachineScanReceipt(scanInput, original);
  const freshScan = validateMachineScanReceipt(freshScanInput, fresh);
  if (Date.parse(scan.startedAt) < Math.max(Date.parse(chrome.finishedAt), Date.parse(safari.finishedAt))) throw new Error("machine scan must start after both manual journeys finish");
  if (Date.parse(scan.finishedAt) > Date.parse(observation)) throw new Error("machine scan finished after finalization observation");
  if (canonicalJSONStringify(scan.rows) !== canonicalJSONStringify(freshScan.rows)) throw new Error("machine scan targets changed after the signed scan");
  return validateReceipt({
    schemaVersion: 2,
    status: "PASS",
    finalizedAt: observation,
    evidenceBoundary: receiptEvidenceBoundary,
    frozenContext: original,
    journeys: [chrome, safari],
    secretScan: { policy: receiptPolicy, receiptSha256: canonicalJSONDigest(scan), receipt: scan },
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
  if (receipt.schemaVersion !== 2 || receipt.status !== "PASS" || receipt.evidenceBoundary !== receiptEvidenceBoundary) {
    throw new Error("final receipt status/schema/evidence boundary is invalid");
  }
  const finalizedAt = canonicalTimestamp(receipt.finalizedAt, "final receipt finalizedAt");
  const context = validateFrozenContext(receipt.frozenContext);
  if (!Array.isArray(receipt.journeys) || receipt.journeys.length !== 2) throw new Error("final receipt must contain exact Chrome and Safari journeys");
  validateJourney(receipt.journeys[0], "chrome", context.frozenAt, finalizedAt);
  validateJourney(receipt.journeys[1], "safari", context.frozenAt, finalizedAt);
  const secretScan = exactObject(receipt.secretScan, secretScanBindingKeys, "final receipt secretScan");
  if (secretScan.policy !== receiptPolicy) throw new Error("final receipt secret scan policy is invalid");
  const machineScan = validateMachineScanReceipt(secretScan.receipt, context);
  if (secretScan.receiptSha256 !== canonicalJSONDigest(machineScan)) throw new Error("final receipt machine scan digest mismatch");
  if (Date.parse(machineScan.startedAt) < Math.max(...receipt.journeys.map((journey) => Date.parse(journey.finishedAt))) || Date.parse(machineScan.finishedAt) > Date.parse(finalizedAt)) {
    throw new Error("final receipt machine scan is stale relative to the journeys/finalization");
  }
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
    const output = validateReceiptOutputPath(manifest, args.get("--out"));
    if (relative(repoRoot, output) !== "" && !relative(repoRoot, output).startsWith("..")) throw new Error("frozen context must be written outside the Git worktree");
    writeExclusiveJSON(output, await collectContext(manifest));
    console.log(`frozen P2 browser context written to ${output}`);
    return;
  }
  const contextPath = canonicalAbsolutePath(args.get("--context"), "--context");
  assertPrivatePath(contextPath, "file", "--context");
  const original = validateFrozenContext(readJSON(contextPath, "frozen context"));
  if (mode === "scan") {
    if (args.size !== 3 || !args.has("--out")) throw new Error("scan requires --manifest, --context, and --out");
    const output = validateReceiptOutputPath(manifest, args.get("--out"));
    writeExclusiveJSON(output, await scanEnvironment(manifest, original));
    console.log(`P2 machine secret-scan receipt written to ${output}`);
    return;
  }
  if (mode !== "finalize" || args.size !== 6 || !args.has("--scan") || !args.has("--chrome-journey") || !args.has("--safari-journey") || !args.has("--out")) {
    throw new Error("finalize requires --manifest, --context, --scan, --chrome-journey, --safari-journey, and --out");
  }
  const fresh = await collectContext(manifest, original.frozenAt);
  const scanPath = canonicalAbsolutePath(args.get("--scan"), "--scan");
  assertPrivatePath(scanPath, "file", "--scan");
  const scan = validateMachineScanReceipt(readJSON(scanPath, "machine scan receipt"), original);
  const freshScan = await scanEnvironment(manifest, fresh);
  const chromeJourneyPath = canonicalAbsolutePath(args.get("--chrome-journey"), "--chrome-journey");
  const safariJourneyPath = canonicalAbsolutePath(args.get("--safari-journey"), "--safari-journey");
  assertPrivatePath(chromeJourneyPath, "file", "--chrome-journey");
  assertPrivatePath(safariJourneyPath, "file", "--safari-journey");
  const chrome = readJSON(chromeJourneyPath, "Chrome journey");
  const safari = readJSON(safariJourneyPath, "Safari journey");
  const receipt = finalizeReceipt(original, fresh, chrome, safari, scan, freshScan, new Date().toISOString());
  writeExclusiveJSON(validateReceiptOutputPath(manifest, args.get("--out")), receipt);
  console.log(`final P2 browser receipt written to ${args.get("--out")}`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.message : String(error));
    process.exitCode = 1;
  });
}
