import {
  createHash,
  createHmac,
  createPrivateKey,
  createPublicKey,
  diffieHellman,
  hkdfSync,
  sign,
  timingSafeEqual,
  verify,
} from "node:crypto";
import { readFile } from "node:fs/promises";

import { CipherSuite, HkdfSha256 } from "@hpke/core";
import { DhkemX25519HkdfSha256 } from "@hpke/dhkem-x25519";
import { Chacha20Poly1305 } from "@hpke/chacha20poly1305";
import { xchacha20poly1305 } from "@noble/ciphers/chacha.js";

const hpkeSuiteName =
  "DHKEM(X25519,HKDF-SHA256)/HKDF-SHA256/ChaCha20Poly1305/Auth";
const encoder = new TextEncoder();

function hex(value) {
  return new Uint8Array(Buffer.from(value, "hex"));
}

function b64url(value) {
  return Buffer.from(value).toString("base64url");
}

function canonical(value) {
  if (Array.isArray(value)) {
    return `[${value.map((item) => canonical(item)).join(",")}]`;
  }
  if (value !== null && typeof value === "object") {
    return `{${Object.keys(value)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${canonical(value[key])}`)
      .join(",")}}`;
  }
  return JSON.stringify(value);
}

function framed(...parts) {
  const normalized = parts.map((part) =>
    typeof part === "string" ? encoder.encode(part) : new Uint8Array(part),
  );
  const size = normalized.reduce((total, part) => total + 4 + part.length, 0);
  const result = new Uint8Array(size);
  const view = new DataView(result.buffer);
  let offset = 0;
  for (const part of normalized) {
    view.setUint32(offset, part.length, false);
    offset += 4;
    result.set(part, offset);
    offset += part.length;
  }
  return result;
}

function digest(...parts) {
  return new Uint8Array(createHash("sha256").update(framed(...parts)).digest());
}

function plainDigest(value) {
  return new Uint8Array(createHash("sha256").update(value).digest());
}

function base32Fingerprint(pinDigest) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = 0;
  let value = 0;
  let encoded = "";
  for (const byte of pinDigest.slice(0, 15)) {
    value = (value << 8) | byte;
    bits += 8;
    while (bits >= 5) {
      encoded += alphabet[(value >>> (bits - 5)) & 31];
      bits -= 5;
    }
  }
  if (bits !== 0) encoded += alphabet[(value << (5 - bits)) & 31];
  return encoded.match(/.{4}/g).join("-");
}

function decodeBase32Fingerprint(display) {
  if (display.length === 29) {
    for (const offset of [4, 9, 14, 19, 24]) {
      if (display[offset] !== "-") throw new Error("invalid fingerprint grouping");
    }
    display = display.replaceAll("-", "");
  } else if (display.length !== 24) {
    throw new Error("invalid fingerprint length");
  }
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = 0;
  let value = 0;
  const decoded = [];
  for (const character of display.toUpperCase()) {
    const digit = alphabet.indexOf(character);
    if (digit < 0) throw new Error("invalid fingerprint encoding");
    value = (value << 5) | digit;
    bits += 5;
    if (bits >= 8) {
      decoded.push((value >>> (bits - 8)) & 0xff);
      bits -= 8;
    }
  }
  if (bits !== 0 || decoded.length !== 15) {
    throw new Error("invalid fingerprint encoding");
  }
  return new Uint8Array(decoded);
}

function fingerprintMatches(display, pinDigest) {
  try {
    return timingSafeEqual(
      Buffer.from(decodeBase32Fingerprint(display)),
      Buffer.from(pinDigest.slice(0, 15)),
    );
  } catch {
    return false;
  }
}

function ed25519KeyPair(seed) {
  const prefix = Buffer.from("302e020100300506032b657004220420", "hex");
  const privateKey = createPrivateKey({
    key: Buffer.concat([prefix, Buffer.from(seed)]),
    format: "der",
    type: "pkcs8",
  });
  const publicKey = createPublicKey(privateKey);
  const publicDer = publicKey.export({ format: "der", type: "spki" });
  return {
    privateKey,
    publicKey,
    publicRaw: new Uint8Array(publicDer.subarray(publicDer.length - 32)),
  };
}

function x25519KeyPair(rawPrivate) {
  const privatePrefix = Buffer.from("302e020100300506032b656e04220420", "hex");
  const privateKey = createPrivateKey({
    key: Buffer.concat([privatePrefix, Buffer.from(rawPrivate)]),
    format: "der",
    type: "pkcs8",
  });
  const publicKey = createPublicKey(privateKey);
  const publicDer = publicKey.export({ format: "der", type: "spki" });
  return {
    privateKey,
    publicKey,
    publicRaw: new Uint8Array(publicDer.subarray(publicDer.length - 32)),
  };
}

function x25519PublicKey(rawPublic) {
  const publicPrefix = Buffer.from("302a300506032b656e032100", "hex");
  return createPublicKey({
    key: Buffer.concat([publicPrefix, Buffer.from(rawPublic)]),
    format: "der",
    type: "spki",
  });
}

const attestationMembers = [
  "approverDeviceId",
  "attestationId",
  "capabilities",
  "expiresAt",
  "issuedAt",
  "subjectDeviceId",
  "subjectExchangeKeyDigest",
  "subjectSigningKeyDigest",
  "type",
  "version",
];

const uuidV7Pattern = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
const capabilityPattern = /^mad\.v[1-9][0-9]*\.[a-z][a-z0-9]*(?:\.[a-z][a-z0-9_]*)+$/;
const utcTimePattern = /^([0-9]{4})-([0-9]{2})-([0-9]{2})T([0-9]{2}):([0-9]{2}):([0-9]{2})(?:\.([0-9]{1,6}))?Z$/;

function parseStrictUTCRFC3339(value) {
  const match = utcTimePattern.exec(value);
  if (match === null) return null;
  const [, yearText, monthText, dayText, hourText, minuteText, secondText, fraction = ""] = match;
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);
  const hour = Number(hourText);
  const minute = Number(minuteText);
  const second = Number(secondText);
  if (
    month < 1 || month > 12 ||
    day < 1 || day > 31 ||
    hour > 23 || minute > 59 || second > 59
  ) return null;

  const instant = new Date(0);
  instant.setUTCFullYear(year, month - 1, day);
  instant.setUTCHours(hour, minute, second, 0);
  if (
    instant.getUTCFullYear() !== year ||
    instant.getUTCMonth() !== month - 1 ||
    instant.getUTCDate() !== day ||
    instant.getUTCHours() !== hour ||
    instant.getUTCMinutes() !== minute ||
    instant.getUTCSeconds() !== second
  ) return null;

  return BigInt(instant.getTime()) * 1000n + BigInt(fraction.padEnd(6, "0"));
}

function constantTimeEqual(expected, candidate) {
  const expectedBytes = Buffer.from(expected);
  const candidateBytes = Buffer.from(candidate);
  return (
    expectedBytes.length === candidateBytes.length &&
    timingSafeEqual(expectedBytes, candidateBytes)
  );
}

function hasUnpairedSurrogateEscape(raw) {
  for (let index = 0; index + 5 < raw.length; index += 1) {
    if (raw[index] !== "\\" || raw[index + 1] !== "u") continue;
    let precedingSlashes = 0;
    for (let prior = index - 1; prior >= 0 && raw[prior] === "\\"; prior -= 1) {
      precedingSlashes += 1;
    }
    if (precedingSlashes % 2 === 1) continue;
    const code = Number.parseInt(raw.slice(index + 2, index + 6), 16);
    if (!Number.isInteger(code)) continue;
    if (code >= 0xdc00 && code <= 0xdfff) return true;
    if (code < 0xd800 || code > 0xdbff) continue;
    if (raw.slice(index + 6, index + 8) !== "\\u") return true;
    const low = Number.parseInt(raw.slice(index + 8, index + 12), 16);
    if (!Number.isInteger(low) || low < 0xdc00 || low > 0xdfff) return true;
    index += 6;
  }
  return false;
}

function parseAttestationJSON(raw) {
  if (hasUnpairedSurrogateEscape(raw)) throw new Error("non-I-JSON surrogate");
  const memberMatches = [...raw.matchAll(/"(?:\\.|[^"\\])*"\s*:/g)].map(
    ([match]) => JSON.parse(match.slice(0, match.lastIndexOf(":"))),
  );
  const seen = new Set();
  for (const member of memberMatches) {
    if (!attestationMembers.includes(member) || seen.has(member)) {
      throw new Error("unknown or duplicate attestation member");
    }
    seen.add(member);
  }
  if (seen.size !== attestationMembers.length) throw new Error("incomplete attestation");
  return JSON.parse(raw);
}

function canonicalAttestationFromRaw(raw) {
  const value = parseAttestationJSON(raw);
  if (Object.keys(value).some((key) => !attestationMembers.includes(key))) {
    throw new Error("unknown attestation member");
  }
  if (value.version !== 1 || !Number.isSafeInteger(value.version)) {
    throw new Error("invalid attestation version");
  }
  if (value.type !== "device_attestation") throw new Error("invalid attestation type");
  for (const name of ["approverDeviceId", "attestationId", "subjectDeviceId"]) {
    if (!uuidV7Pattern.test(value[name])) throw new Error("invalid attestation UUIDv7");
  }
  const issuedAt = parseStrictUTCRFC3339(value.issuedAt);
  const expiresAt = parseStrictUTCRFC3339(value.expiresAt);
  if (
    issuedAt === null ||
    expiresAt === null ||
    expiresAt <= issuedAt ||
    expiresAt - issuedAt > 10n * 60n * 1_000_000n
  ) {
    throw new Error("invalid attestation lifetime");
  }
  if (!Array.isArray(value.capabilities) || value.capabilities.length === 0) {
    throw new Error("invalid capabilities");
  }
  if (value.capabilities.some((item) => typeof item !== "string")) {
    throw new Error("invalid capability type");
  }
  const sortedCapabilities = [...value.capabilities].sort();
  if (
    sortedCapabilities.some((item, index) => item !== value.capabilities[index]) ||
    new Set(value.capabilities).size !== value.capabilities.length
  ) {
    throw new Error("capabilities not canonical");
  }
  if (value.capabilities.some((item) => !capabilityPattern.test(item))) {
    throw new Error("invalid capability");
  }
  for (const name of attestationMembers) {
    if (!(name in value)) throw new Error("missing attestation member");
    if (
      name !== "capabilities" &&
      name !== "version" &&
      typeof value[name] !== "string"
    ) {
      throw new Error("invalid attestation member type");
    }
  }
  for (const name of ["subjectExchangeKeyDigest", "subjectSigningKeyDigest"]) {
    const decoded = Buffer.from(value[name], "base64url");
    if (decoded.length !== 32 || decoded.toString("base64url") !== value[name]) {
      throw new Error("invalid key digest");
    }
  }
  return canonical(value);
}

function attestationKeyDigestsMatch(attestation, signingPublicKey, exchangePublicKey) {
  try {
    return (
      timingSafeEqual(
        Buffer.from(attestation.subjectSigningKeyDigest, "base64url"),
        Buffer.from(plainDigest(signingPublicKey)),
      ) &&
      timingSafeEqual(
        Buffer.from(attestation.subjectExchangeKeyDigest, "base64url"),
        Buffer.from(plainDigest(exchangePublicKey)),
      )
    );
  } catch {
    return false;
  }
}

function popContext(fields) {
  return framed(
    "multidesk-x25519-pop-context-v1",
    fields.apiVersion,
    fields.purpose,
    fields.ceremonyId,
    fields.subjectDeviceId,
    fields.subjectSigningPublicKey,
    fields.subjectExchangePublicKey,
    fields.storageMode,
    fields.storageAssertionDigest,
    fields.serverEphemeralPublicKey,
    fields.challenge,
    fields.expiresAt,
  );
}

function popProofs(subjectExchangePrivate, subjectSigningPrivate, fields) {
  const context = popContext(fields);
  const sharedSecret = new Uint8Array(
    diffieHellman({
      privateKey: subjectExchangePrivate,
      publicKey: x25519PublicKey(fields.serverEphemeralPublicKey),
    }),
  );
  if (sharedSecret.every((byte) => byte === 0)) throw new Error("all-zero shared secret");
  const salt = digest(
    "multidesk-x25519-pop-salt-v1",
    fields.ceremonyId,
    fields.challenge,
  );
  const popKey = new Uint8Array(hkdfSync("sha256", sharedSecret, salt, context, 32));
  const exchangeProof = new Uint8Array(
    createHmac("sha256", popKey)
      .update(framed("multidesk-x25519-pop-proof-v1", context))
      .digest(),
  );
  const signingProof = new Uint8Array(
    sign(
      null,
      framed("multidesk-ed25519-pop-proof-v1", context),
      subjectSigningPrivate,
    ),
  );
  return { context, sharedSecret, popKey, exchangeProof, signingProof };
}

function verifyPop(serverPrivate, subjectSigningPublic, fields, exchangeProof, signingProof) {
  try {
    const context = popContext(fields);
    const sharedSecret = new Uint8Array(
      diffieHellman({
        privateKey: serverPrivate,
        publicKey: x25519PublicKey(fields.subjectExchangePublicKey),
      }),
    );
    if (sharedSecret.every((byte) => byte === 0)) return false;
    const salt = digest(
      "multidesk-x25519-pop-salt-v1",
      fields.ceremonyId,
      fields.challenge,
    );
    const popKey = new Uint8Array(hkdfSync("sha256", sharedSecret, salt, context, 32));
    const expectedExchangeProof = new Uint8Array(
      createHmac("sha256", popKey)
        .update(framed("multidesk-x25519-pop-proof-v1", context))
        .digest(),
    );
    return (
      constantTimeEqual(expectedExchangeProof, exchangeProof) &&
      verify(
        null,
        framed("multidesk-ed25519-pop-proof-v1", context),
        subjectSigningPublic,
        signingProof,
      )
    );
  } catch {
    return false;
  }
}

function deriveTraffic(
  root,
  sessionId,
  keyEpoch,
  sourceDeviceId,
  targetDeviceId,
  direction,
  streamId,
) {
  const context = {
    direction,
    keyEpoch,
    purpose: "session_traffic",
    sessionId,
    sourceDeviceId,
    streamId,
    targetDeviceId,
    version: 1,
  };
  const contextCanonical = canonical(context);
  const salt = digest(
    "multidesk-session-traffic-salt-v1",
    sessionId,
    keyEpoch,
  );
  const info = framed(
    "multidesk-session-traffic-info-v1",
    encoder.encode(contextCanonical),
  );
  const material = new Uint8Array(
    hkdfSync("sha256", root, salt, info, 48),
  );
  return {
    key: material.slice(0, 32),
    noncePrefix: material.slice(32),
    contextCanonical,
  };
}

function makeNonce(prefix, sequence) {
  const result = new Uint8Array(24);
  result.set(prefix);
  new DataView(result.buffer).setBigUint64(16, BigInt(sequence), false);
  return result;
}

class ReplayWindow {
  initialized = false;
  high = 0n;
  seen = 0n;

  accept(sequenceValue) {
    const sequence = BigInt(sequenceValue);
    if (!this.initialized) {
      this.initialized = true;
      this.high = sequence;
      this.seen = 1n;
      return true;
    }
    if (sequence > this.high) {
      const delta = sequence - this.high;
      this.seen = delta >= 64n ? 1n : (this.seen << delta) | 1n;
      this.seen &= (1n << 64n) - 1n;
      this.high = sequence;
      return true;
    }
    const delta = this.high - sequence;
    if (delta >= 64n) return false;
    const mask = 1n << delta;
    if ((this.seen & mask) !== 0n) return false;
    this.seen |= mask;
    return true;
  }
}

async function run(input) {
  if (input.schemaVersion !== 1) throw new Error("unsupported vector schema");

  const approver = ed25519KeyPair(hex(input.seeds.approverEd25519));
  const subject = ed25519KeyPair(hex(input.seeds.subjectEd25519));

  const suite = new CipherSuite({
    kem: new DhkemX25519HkdfSha256(),
    kdf: new HkdfSha256(),
    aead: new Chacha20Poly1305(),
  });
  const sourceKeys = await suite.kem.deriveKeyPair(
    hex(input.seeds.sourceX25519Ikm),
  );
  const targetKeys = await suite.kem.deriveKeyPair(
    hex(input.seeds.targetX25519Ikm),
  );
  const peerBKeys = await suite.kem.deriveKeyPair(
    hex(input.seeds.peerBX25519Ikm),
  );
  const sourcePublicRaw = new Uint8Array(
    await suite.kem.serializePublicKey(sourceKeys.publicKey),
  );
  const targetPublicRaw = new Uint8Array(
    await suite.kem.serializePublicKey(targetKeys.publicKey),
  );
  const peerBPublicRaw = new Uint8Array(
    await suite.kem.serializePublicKey(peerBKeys.publicKey),
  );

  const targetPrivateRaw = new Uint8Array(
    await suite.kem.serializePrivateKey(targetKeys.privateKey),
  );
  const subjectExchange = x25519KeyPair(targetPrivateRaw);
  if (!Buffer.from(subjectExchange.publicRaw).equals(Buffer.from(targetPublicRaw))) {
    throw new Error("subject X25519 public key mismatch");
  }
  const serverPop = x25519KeyPair(hex(input.seeds.serverPopX25519Private));
  const restartPop = x25519KeyPair(hex(input.seeds.restartPopX25519Private));

  const subjectSigningDigest = plainDigest(subject.publicRaw);
  const subjectExchangeDigest = plainDigest(targetPublicRaw);

  const attestation = {
    approverDeviceId: input.attestation.approverDeviceId,
    attestationId: input.attestation.attestationId,
    capabilities: input.attestation.capabilities,
    expiresAt: input.attestation.expiresAt,
    issuedAt: input.attestation.issuedAt,
    subjectDeviceId: input.attestation.subjectDeviceId,
    subjectExchangeKeyDigest: b64url(subjectExchangeDigest),
    subjectSigningKeyDigest: b64url(subjectSigningDigest),
    type: "device_attestation",
    version: 1,
  };
  const attestationRaw = JSON.stringify(attestation);
  const attestationCanonical = canonicalAttestationFromRaw(attestationRaw);
  const attestationMessage = framed(
    "multidesk-device-attestation-v1",
    encoder.encode(attestationCanonical),
  );
  const attestationSignature = new Uint8Array(
    sign(null, attestationMessage, approver.privateKey),
  );
  const attestationMutation = attestationMessage.slice();
  attestationMutation[attestationMutation.length - 2] ^= 1;

  const attestationChangeRejected = (change) => {
    const candidate = structuredClone(attestation);
    change(candidate);
    try {
      const candidateCanonical = canonicalAttestationFromRaw(JSON.stringify(candidate));
      return !verify(
        null,
        framed("multidesk-device-attestation-v1", encoder.encode(candidateCanonical)),
        approver.publicKey,
        attestationSignature,
      );
    } catch {
      return true;
    }
  };
  const exchangeDigestMutationRejected = attestationChangeRejected((value) => {
    value.subjectExchangeKeyDigest = b64url(new Uint8Array(32));
  });
  const signingDigestMutationRejected = attestationChangeRejected((value) => {
    value.subjectSigningKeyDigest = b64url(new Uint8Array(32));
  });
  const capabilityMutationRejected = attestationChangeRejected((value) => {
    value.capabilities[0] = "mad.v1.device.revoke";
    value.capabilities.sort();
  });
  const attestationExpiryMutationRejected = attestationChangeRejected((value) => {
    value.expiresAt = "2026-07-15T00:00:01Z";
  });
  const attestationIdMutationRejected = attestationChangeRejected((value) => {
    value.attestationId = input.keyPop.ceremonyId;
  });
  const rejectsAttestationRaw = (raw) => {
    try {
      canonicalAttestationFromRaw(raw);
      return false;
    } catch {
      return true;
    }
  };
  const duplicateMemberRejected = rejectsAttestationRaw(
    `${attestationRaw.slice(0, -1)},"version":1}`,
  );
  const unknownMemberRejected = rejectsAttestationRaw(
    `${attestationRaw.slice(0, -1)},"unknown":true}`,
  );
  const floatRejected = rejectsAttestationRaw(
    attestationRaw.replace('"version":1', '"version":1.5'),
  );
  const arbitraryMapRejected = rejectsAttestationRaw(
    attestationRaw.replace(
      '"capabilities":[',
      '"capabilities":{"value":[',
    ).replace('],"expiresAt"', ']},"expiresAt"'),
  );
  const escapedCanonical = canonicalAttestationFromRaw(
    attestationRaw.replace("device_attestation", "device\\u005fattestation"),
  );
  const unsafeIntegerRejected = rejectsAttestationRaw(
    attestationRaw.replace('"version":1', '"version":9007199254740992'),
  );
  const negativeIntegerRejected = rejectsAttestationRaw(
    attestationRaw.replace('"version":1', '"version":-1'),
  );
  const invalidCapabilityRejected = rejectsAttestationRaw(
    attestationRaw.replace("mad.v1.metadata.read", "MAD.invalid"),
  );
  const invalidIdRejected = rejectsAttestationRaw(
    attestationRaw.replace(
      input.attestation.attestationId,
      "018f47d2-7c11-6d3f-a9b8-1f6d83de1003",
    ),
  );
  const invalidLifetimeRejected = rejectsAttestationRaw(
    attestationRaw.replace(
      input.attestation.expiresAt,
      "2026-07-14T16:10:00.0000001Z",
    ),
  );
  const invalidCalendarDateRejected = rejectsAttestationRaw(
    attestationRaw
      .replace(input.attestation.issuedAt, "2026-02-30T16:00:00Z")
      .replace(input.attestation.expiresAt, "2026-02-30T16:10:00Z"),
  );
  const invalidHour24Rejected = rejectsAttestationRaw(
    attestationRaw
      .replace(input.attestation.issuedAt, "2026-07-14T24:00:00Z")
      .replace(input.attestation.expiresAt, "2026-07-14T24:10:00Z"),
  );
  const leapDayBoundaryAccepted = !rejectsAttestationRaw(
    attestationRaw
      .replace(input.attestation.issuedAt, "2024-02-29T23:59:59.999999Z")
      .replace(input.attestation.expiresAt, "2024-03-01T00:00:00Z"),
  );
  const unicodeSurrogateRejected = rejectsAttestationRaw(
    attestationRaw.replace("device_attestation", "\\ud800"),
  );
  const reorderedCanonical = canonicalAttestationFromRaw(
    JSON.stringify(Object.fromEntries(Object.entries(attestation).reverse())),
  );
  const subjectKeyDigestsMatch = attestationKeyDigestsMatch(
    attestation,
    subject.publicRaw,
    targetPublicRaw,
  );
  const mutatedSigningPublic = subject.publicRaw.slice();
  mutatedSigningPublic[0] ^= 1;
  const mutatedExchangePublic = targetPublicRaw.slice();
  mutatedExchangePublic[0] ^= 1;
  const signingKeyDigestMismatchRejected = !attestationKeyDigestsMatch(
    attestation,
    mutatedSigningPublic,
    targetPublicRaw,
  );
  const exchangeKeyDigestMismatchRejected = !attestationKeyDigestsMatch(
    attestation,
    subject.publicRaw,
    mutatedExchangePublic,
  );

  const pinDigest = digest(
    "multidesk-device-pin-v1",
    input.attestation.subjectDeviceId,
    subject.publicRaw,
    targetPublicRaw,
  );
  const fingerprint = base32Fingerprint(pinDigest);
  const lowercaseAccepted = fingerprintMatches(fingerprint.toLowerCase(), pinDigest);
  const unhyphenatedAccepted = fingerprintMatches(
    fingerprint.replaceAll("-", ""),
    pinDigest,
  );
  const alteredFingerprint = `${fingerprint[0] === "A" ? "B" : "A"}${fingerprint.slice(1)}`;
  const alteredGroupRejected = !fingerprintMatches(alteredFingerprint, pinDigest);
  const compactFingerprint = fingerprint.replaceAll("-", "");
  const length23Rejected = !fingerprintMatches(compactFingerprint.slice(0, 23), pinDigest);
  const length25Rejected = !fingerprintMatches(`${compactFingerprint}A`, pinDigest);
  const invalidBase32Rejected = !fingerprintMatches(
    "0000-0000-0000-0000-0000-0000",
    pinDigest,
  );
  const pinHex = Buffer.from(pinDigest).toString("hex");
  const oldFullHexDisplayRejected = !fingerprintMatches(
    pinHex.match(/.{8}/g).join("-"),
    pinDigest,
  );
  const truncatedAsFullDigestRejected = pinDigest.slice(0, 15).length !== 32;

  const keyEnvelopeAssertionCanonical = canonical(input.keyPop.keyEnvelopeAssertion);
  const storageAssertionDigest = plainDigest(
    encoder.encode(keyEnvelopeAssertionCanonical),
  );
  const popFieldsValue = {
    apiVersion: input.keyPop.apiVersion,
    purpose: input.keyPop.purpose,
    ceremonyId: input.keyPop.ceremonyId,
    subjectDeviceId: input.keyPop.subjectDeviceId,
    subjectSigningPublicKey: subject.publicRaw,
    subjectExchangePublicKey: targetPublicRaw,
    storageMode: input.keyPop.storageMode,
    storageAssertionDigest,
    serverEphemeralPublicKey: serverPop.publicRaw,
    challenge: hex(input.keyPop.challengeHex),
    expiresAt: input.keyPop.expiresAt,
  };
  const keyPop = popProofs(
    subjectExchange.privateKey,
    subject.privateKey,
    popFieldsValue,
  );
  const popVerifies = verifyPop(
    serverPop.privateKey,
    subject.publicKey,
    popFieldsValue,
    keyPop.exchangeProof,
    keyPop.signingProof,
  );
  const mutatedExchangeProof = keyPop.exchangeProof.slice();
  mutatedExchangeProof[0] ^= 1;
  const exchangeProofContentMutationRejected = !verifyPop(
    serverPop.privateKey,
    subject.publicKey,
    popFieldsValue,
    mutatedExchangeProof,
    keyPop.signingProof,
  );
  const exchangeProofShortRejected = !verifyPop(
    serverPop.privateKey,
    subject.publicKey,
    popFieldsValue,
    keyPop.exchangeProof.slice(0, -1),
    keyPop.signingProof,
  );
  const exchangeProofLongRejected = !verifyPop(
    serverPop.privateKey,
    subject.publicKey,
    popFieldsValue,
    new Uint8Array([...keyPop.exchangeProof, 0]),
    keyPop.signingProof,
  );
  const mutatePop = (change) => {
    const candidate = structuredClone(popFieldsValue);
    change(candidate);
    return !verifyPop(
      serverPop.privateKey,
      subject.publicKey,
      candidate,
      keyPop.exchangeProof,
      keyPop.signingProof,
    );
  };
  const storageModeMutationRejected = mutatePop((value) => {
    value.storageMode = "native";
  });
  const storageAssertionMutationRejected = mutatePop((value) => {
    value.storageAssertionDigest[0] ^= 1;
  });
  const purposeMutationRejected = mutatePop((value) => {
    value.purpose = "bootstrap";
  });
  const ceremonyMutationRejected = mutatePop((value) => {
    value.ceremonyId = input.attestation.attestationId;
  });
  const deviceMutationRejected = mutatePop((value) => {
    value.subjectDeviceId = input.attestation.approverDeviceId;
  });
  const signingKeyMutationRejected = mutatePop((value) => {
    value.subjectSigningPublicKey[0] ^= 1;
  });
  const exchangeKeyMutationRejected = mutatePop((value) => {
    value.subjectExchangePublicKey[0] ^= 1;
  });
  const challengeMutationRejected = mutatePop((value) => {
    value.challenge[0] ^= 1;
  });
  const popExpiryMutationRejected = mutatePop((value) => {
    value.expiresAt = "2026-07-14T16:09:59Z";
  });
  const serverEphemeralMutationRejected = mutatePop((value) => {
    value.serverEphemeralPublicKey = restartPop.publicRaw;
  });
  let allZeroSharedSecretRejected = false;
  try {
    diffieHellman({
      privateKey: serverPop.privateKey,
      publicKey: x25519PublicKey(new Uint8Array(32)),
    });
  } catch {
    allZeroSharedSecretRejected = true;
  }
  const restartFields = structuredClone(popFieldsValue);
  restartFields.serverEphemeralPublicKey = restartPop.publicRaw;
  const restartInvalidated = !verifyPop(
    restartPop.privateKey,
    subject.publicKey,
    restartFields,
    keyPop.exchangeProof,
    keyPop.signingProof,
  );
  let consumed = false;
  const verifyOnce = () => {
    if (
      consumed ||
      !verifyPop(
        serverPop.privateKey,
        subject.publicKey,
        popFieldsValue,
        keyPop.exchangeProof,
        keyPop.signingProof,
      )
    ) return false;
    consumed = true;
    return true;
  };
  const firstConsumeAccepted = verifyOnce();
  const replayRejected = !verifyOnce();

  const sourceExchangeDigest = plainDigest(sourcePublicRaw);
  const targetExchangeDigest = plainDigest(targetPublicRaw);
  const wrapBase = {
    expiresAt: input.keyWrap.expiresAt,
    keyEpoch: input.keyWrap.keyEpoch,
    purpose: input.keyWrap.purpose,
    sessionId: input.keyWrap.sessionId,
    sourceDeviceId: input.attestation.approverDeviceId,
    sourceExchangeKeyDigest: b64url(sourceExchangeDigest),
    targetDeviceId: input.attestation.subjectDeviceId,
    targetExchangeKeyDigest: b64url(targetExchangeDigest),
    type: "session_key_wrap",
    version: 1,
    wrapId: input.keyWrap.wrapId,
  };
  const wrapBaseCanonical = canonical(wrapBase);
  const hpkeInfo = digest(
    "multidesk-hpke-session-wrap-info-v1",
    encoder.encode(wrapBaseCanonical),
  );
  const sender = await suite.createSenderContext({
    recipientPublicKey: targetKeys.publicKey,
    senderKey: sourceKeys.privateKey,
    info: hpkeInfo,
    ekm: hex(input.seeds.ephemeralX25519Ikm),
  });
  const enc = new Uint8Array(sender.enc);
  const wrapHeader = {
    ...wrapBase,
    enc: b64url(enc),
    hpkeSuite: hpkeSuiteName,
  };
  const wrapAAD = encoder.encode(canonical(wrapHeader));
  const pairwiseRootPeerA1 = hex(input.seeds.pairwiseRootPeerAEpoch1);
  const wrappedKey = new Uint8Array(
    await sender.seal(pairwiseRootPeerA1, wrapAAD),
  );
  const recipient = await suite.createRecipientContext({
    recipientKey: targetKeys.privateKey,
    enc,
    info: hpkeInfo,
    senderPublicKey: sourceKeys.publicKey,
  });
  const recoveredKey = new Uint8Array(await recipient.open(wrappedKey, wrapAAD));
  if (!Buffer.from(recoveredKey).equals(Buffer.from(pairwiseRootPeerA1))) {
    throw new Error("HPKE recovered key mismatch");
  }
  const mutatedWrapAAD = encoder.encode(
    canonical({
      ...wrapHeader,
      targetDeviceId: input.attestation.approverDeviceId,
    }),
  );
  const mutatedRecipient = await suite.createRecipientContext({
    recipientKey: targetKeys.privateKey,
    enc,
    info: hpkeInfo,
    senderPublicKey: sourceKeys.publicKey,
  });
  let wrapAADMutationRejected = false;
  try {
    await mutatedRecipient.open(wrappedKey, mutatedWrapAAD);
  } catch {
    wrapAADMutationRejected = true;
  }
  let wrongPinnedSenderRejected = false;
  try {
    const wrongSenderRecipient = await suite.createRecipientContext({
      recipientKey: targetKeys.privateKey,
      enc,
      info: hpkeInfo,
      senderPublicKey: targetKeys.publicKey,
    });
    await wrongSenderRecipient.open(wrappedKey, wrapAAD);
  } catch {
    wrongPinnedSenderRejected = true;
  }

  const traffic1 = deriveTraffic(
    pairwiseRootPeerA1,
    input.keyWrap.sessionId,
    input.payload.keyEpoch,
    input.attestation.approverDeviceId,
    input.attestation.subjectDeviceId,
    input.payload.direction,
    input.payload.streamId,
  );
  const nonce1 = makeNonce(traffic1.noncePrefix, input.payload.sequence);
  const payloadHeader = {
    direction: input.payload.direction,
    keyEpoch: input.payload.keyEpoch,
    kind: input.payload.kind,
    messageId: input.payload.messageId,
    nonce: b64url(nonce1),
    sentAt: input.payload.sentAt,
    sequence: input.payload.sequence,
    sessionId: input.keyWrap.sessionId,
    sourceDeviceId: input.attestation.approverDeviceId,
    streamId: input.payload.streamId,
    targetDeviceId: input.attestation.subjectDeviceId,
    type: "session_envelope",
    version: 1,
  };
  const payloadAAD = encoder.encode(canonical(payloadHeader));
  const payloadPlaintext = hex(input.payload.plaintextHex);
  const payloadCiphertext = xchacha20poly1305(
    traffic1.key,
    nonce1,
    payloadAAD,
  ).encrypt(payloadPlaintext);
  const payloadRecovered = xchacha20poly1305(
    traffic1.key,
    nonce1,
    payloadAAD,
  ).decrypt(payloadCiphertext);
  if (!Buffer.from(payloadRecovered).equals(Buffer.from(payloadPlaintext))) {
    throw new Error("payload round trip failed");
  }
  let payloadAADMutationRejected = false;
  try {
    xchacha20poly1305(
      traffic1.key,
      nonce1,
      encoder.encode(canonical({ ...payloadHeader, kind: "approval_request" })),
    ).decrypt(payloadCiphertext);
  } catch {
    payloadAADMutationRejected = true;
  }
  const badNonce = makeNonce(traffic1.noncePrefix, "101");
  const badNonceAAD = encoder.encode(
    canonical({ ...payloadHeader, nonce: b64url(badNonce) }),
  );
  const badNonceCiphertext = xchacha20poly1305(
    traffic1.key,
    badNonce,
    badNonceAAD,
  ).encrypt(payloadPlaintext);
  let nonceSequenceMismatchRejected = !Buffer.from(badNonce).equals(
    Buffer.from(nonce1),
  );
  try {
    xchacha20poly1305(traffic1.key, nonce1, badNonceAAD).decrypt(
      badNonceCiphertext,
    );
    nonceSequenceMismatchRejected = false;
  } catch {
    // The receiver's recomputed nonce does not authenticate the bad frame.
  }

  const pairwiseRootPeerB1 = hex(input.seeds.pairwiseRootPeerBEpoch1);
  const peerBTraffic = deriveTraffic(
    pairwiseRootPeerB1,
    input.keyWrap.sessionId,
    input.payload.keyEpoch,
    input.attestation.approverDeviceId,
    input.peerB.deviceId,
    input.payload.direction,
    input.payload.streamId,
  );
  const peerBNonce = makeNonce(peerBTraffic.noncePrefix, input.peerB.sequence);
  const peerBHeader = {
    direction: input.payload.direction,
    keyEpoch: input.payload.keyEpoch,
    kind: input.payload.kind,
    messageId: input.peerB.messageId,
    nonce: b64url(peerBNonce),
    sentAt: input.peerB.sentAt,
    sequence: input.peerB.sequence,
    sessionId: input.keyWrap.sessionId,
    sourceDeviceId: input.attestation.approverDeviceId,
    streamId: input.payload.streamId,
    targetDeviceId: input.peerB.deviceId,
    type: "session_envelope",
    version: 1,
  };
  const peerBAAD = encoder.encode(canonical(peerBHeader));
  const peerBPlaintext = hex(input.peerB.plaintextHex);
  const peerBCiphertext = xchacha20poly1305(
    peerBTraffic.key,
    peerBNonce,
    peerBAAD,
  ).encrypt(peerBPlaintext);
  let peerAOpenPeerBRejected = false;
  try {
    xchacha20poly1305(traffic1.key, peerBNonce, peerBAAD).decrypt(
      peerBCiphertext,
    );
  } catch {
    peerAOpenPeerBRejected = true;
  }

  const forgeDirection = "client_to_device";
  const forgeStream = "control";
  const forgeSequence = "1";
  const attackerTraffic = deriveTraffic(
    pairwiseRootPeerA1,
    input.keyWrap.sessionId,
    input.payload.keyEpoch,
    input.peerB.deviceId,
    input.attestation.approverDeviceId,
    forgeDirection,
    forgeStream,
  );
  const expectedPeerBTraffic = deriveTraffic(
    pairwiseRootPeerB1,
    input.keyWrap.sessionId,
    input.payload.keyEpoch,
    input.peerB.deviceId,
    input.attestation.approverDeviceId,
    forgeDirection,
    forgeStream,
  );
  const attackerNonce = makeNonce(attackerTraffic.noncePrefix, forgeSequence);
  const expectedPeerBNonce = makeNonce(
    expectedPeerBTraffic.noncePrefix,
    forgeSequence,
  );
  const forgeHeader = {
    direction: forgeDirection,
    keyEpoch: input.payload.keyEpoch,
    kind: "control_input",
    messageId: "018f47d2-7c11-7d3f-a9b8-1f6d83de3004",
    nonce: b64url(attackerNonce),
    sentAt: input.peerB.sentAt,
    sequence: forgeSequence,
    sessionId: input.keyWrap.sessionId,
    sourceDeviceId: input.peerB.deviceId,
    streamId: forgeStream,
    targetDeviceId: input.attestation.approverDeviceId,
    type: "session_envelope",
    version: 1,
  };
  const forgeAAD = encoder.encode(canonical(forgeHeader));
  const forgeCiphertext = xchacha20poly1305(
    attackerTraffic.key,
    attackerNonce,
    forgeAAD,
  ).encrypt(encoder.encode("forged control"));
  let peerAForgeryRejected = !Buffer.from(attackerNonce).equals(
    Buffer.from(expectedPeerBNonce),
  );
  try {
    xchacha20poly1305(
      expectedPeerBTraffic.key,
      expectedPeerBNonce,
      forgeAAD,
    ).decrypt(forgeCiphertext);
    peerAForgeryRejected = false;
  } catch {
    // Pairwise root B cannot authenticate a ciphertext forged with root A.
  }

  const replaySequence = ["100", "98", "99", "100", "36"];
  const replay = new ReplayWindow();
  const replayVerdicts = replaySequence.map((sequence) =>
    replay.accept(sequence) ? "accept" : "reject",
  );

  const pairwiseRootPeerA2 = hex(input.seeds.pairwiseRootPeerAEpoch2);
  const traffic2 = deriveTraffic(
    pairwiseRootPeerA2,
    input.keyWrap.sessionId,
    input.rotation.keyEpoch,
    input.attestation.approverDeviceId,
    input.attestation.subjectDeviceId,
    input.payload.direction,
    input.payload.streamId,
  );
  const nonce2 = makeNonce(traffic2.noncePrefix, input.rotation.sequence);
  const rotationHeader = {
    direction: input.payload.direction,
    keyEpoch: input.rotation.keyEpoch,
    kind: input.payload.kind,
    messageId: input.rotation.messageId,
    nonce: b64url(nonce2),
    sentAt: input.rotation.sentAt,
    sequence: input.rotation.sequence,
    sessionId: input.keyWrap.sessionId,
    sourceDeviceId: input.attestation.approverDeviceId,
    streamId: input.payload.streamId,
    targetDeviceId: input.attestation.subjectDeviceId,
    type: "session_envelope",
    version: 1,
  };
  const rotationAAD = encoder.encode(canonical(rotationHeader));
  const rotationPlaintext = hex(input.rotation.plaintextHex);
  const rotationCiphertext = xchacha20poly1305(
    traffic2.key,
    nonce2,
    rotationAAD,
  ).encrypt(rotationPlaintext);
  let oldKeyRejected = false;
  try {
    xchacha20poly1305(traffic1.key, nonce2, rotationAAD).decrypt(
      rotationCiphertext,
    );
  } catch {
    oldKeyRejected = true;
  }
  const rotationRecovered = xchacha20poly1305(
    traffic2.key,
    nonce2,
    rotationAAD,
  ).decrypt(rotationCiphertext);
  const newKeyRecovered = Buffer.from(rotationRecovered).equals(
    Buffer.from(rotationPlaintext),
  );

  return {
    attestation: {
      approverPublicKey: b64url(approver.publicRaw),
      arbitraryMapRejected,
      canonical: attestationCanonical,
      capabilityMutationRejected,
      exchangeDigestMutationRejected,
      exchangeKeyDigestMismatchRejected,
      duplicateMemberRejected,
      escapingCanonical: escapedCanonical === attestationCanonical,
      expiryMutationRejected: attestationExpiryMutationRejected,
      floatRejected,
      idMutationRejected: attestationIdMutationRejected,
      invalidCapabilityRejected,
      invalidCalendarDateRejected,
      invalidHour24Rejected,
      invalidIDRejected: invalidIdRejected,
      invalidLifetimeRejected,
      leapDayBoundaryAccepted,
      mutationRejected: !verify(
        null,
        attestationMutation,
        approver.publicKey,
        attestationSignature,
      ),
      negativeIntegerRejected,
      orderIndependent: reorderedCanonical === attestationCanonical,
      signature: b64url(attestationSignature),
      signingDigestMutationRejected,
      signingKeyDigestMismatchRejected,
      subjectExchangeKeyDigest: b64url(subjectExchangeDigest),
      subjectExchangePublicKey: b64url(targetPublicRaw),
      subjectSigningKeyDigest: b64url(subjectSigningDigest),
      subjectSigningPublicKey: b64url(subject.publicRaw),
      subjectKeyDigestsMatch,
      unicodeSurrogateRejected,
      unknownMemberRejected,
      unsafeIntegerRejected,
      verifies: verify(
        null,
        attestationMessage,
        approver.publicKey,
        attestationSignature,
      ),
    },
    keyPop: {
      allZeroSharedSecretRejected,
      ceremonyMutationRejected,
      challengeMutationRejected,
      context: b64url(keyPop.context),
      deviceMutationRejected,
      exchangeKeyMutationRejected,
      exchangeProof: b64url(keyPop.exchangeProof),
      exchangeProofContentMutationRejected,
      exchangeProofLongRejected,
      exchangeProofShortRejected,
      expiryMutationRejected: popExpiryMutationRejected,
      firstConsumeAccepted,
      popKey: b64url(keyPop.popKey),
      purposeMutationRejected,
      replayRejected,
      restartInvalidated,
      serverEphemeralMutationRejected,
      serverEphemeralPublicKey: b64url(serverPop.publicRaw),
      signingKeyMutationRejected,
      signingProof: b64url(keyPop.signingProof),
      sharedSecret: b64url(keyPop.sharedSecret),
      storageAssertionDigest: b64url(storageAssertionDigest),
      storageAssertionMutationRejected,
      storageModeMutationRejected,
      verifies: popVerifies,
    },
    keyWrap: {
      aad: new TextDecoder().decode(wrapAAD),
      aadMutationRejected: wrapAADMutationRejected,
      ciphertext: b64url(wrappedKey),
      enc: b64url(enc),
      info: b64url(hpkeInfo),
      sourceExchangePublicKey: b64url(sourcePublicRaw),
      targetExchangePublicKey: b64url(targetPublicRaw),
      unwrapMatches: true,
      wrongPinnedSenderRejected,
    },
    crossPeer: {
      forgeAAD: new TextDecoder().decode(forgeAAD),
      forgeCiphertext: b64url(forgeCiphertext),
      peerAForPeerBForgeRejected: peerAForgeryRejected,
      peerAForPeerBOpenRejected: peerAOpenPeerBRejected,
      peerBAAD: new TextDecoder().decode(peerBAAD),
      peerBCiphertext: b64url(peerBCiphertext),
      peerBExchangePublicKey: b64url(peerBPublicRaw),
      peerBTrafficContext: peerBTraffic.contextCanonical,
      peerBTrafficKey: b64url(peerBTraffic.key),
    },
    payload: {
      aad: new TextDecoder().decode(payloadAAD),
      aadMutationRejected: payloadAADMutationRejected,
      ciphertext: b64url(payloadCiphertext),
      nonce: b64url(nonce1),
      nonceSequenceMismatchRejected,
      replaySequence,
      replayVerdicts,
      roundTrip: true,
      trafficContext: traffic1.contextCanonical,
      trafficKey: b64url(traffic1.key),
    },
    pin: {
      alteredGroupRejected,
      digest: b64url(pinDigest),
      fingerprint,
      invalidBase32Rejected,
      length23Rejected,
      length25Rejected,
      lowercaseAccepted,
      oldFullHexDisplayRejected,
      truncatedAsFullDigestRejected,
      unhyphenatedAccepted,
    },
    rotation: {
      aad: new TextDecoder().decode(rotationAAD),
      ciphertext: b64url(rotationCiphertext),
      newKeyRecovered,
      nonce: b64url(nonce2),
      oldKeyRejected,
      trafficContext: traffic2.contextCanonical,
      trafficKey: b64url(traffic2.key),
    },
  };
}

const path = process.argv[2] ?? "../vectors.json";
const input = JSON.parse(await readFile(path, "utf8"));
process.stdout.write(`${canonical(await run(input))}\n`);
