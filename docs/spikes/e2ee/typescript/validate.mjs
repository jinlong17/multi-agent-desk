import {
  createHash,
  createPrivateKey,
  createPublicKey,
  hkdfSync,
  sign,
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
  const sourcePublicRaw = new Uint8Array(
    await suite.kem.serializePublicKey(sourceKeys.publicKey),
  );
  const targetPublicRaw = new Uint8Array(
    await suite.kem.serializePublicKey(targetKeys.publicKey),
  );

  const attestation = {
    approverDeviceId: input.attestation.approverDeviceId,
    attestationId: input.attestation.attestationId,
    capabilities: input.attestation.capabilities,
    expiresAt: input.attestation.expiresAt,
    issuedAt: input.attestation.issuedAt,
    subjectDeviceId: input.attestation.subjectDeviceId,
    subjectExchangeKey: b64url(targetPublicRaw),
    subjectSigningKey: b64url(subject.publicRaw),
    type: "device_attestation",
    version: 1,
  };
  const attestationCanonical = canonical(attestation);
  const attestationMessage = framed(
    "multidesk-device-attestation-v1",
    encoder.encode(attestationCanonical),
  );
  const attestationSignature = new Uint8Array(
    sign(null, attestationMessage, approver.privateKey),
  );
  const attestationMutation = attestationMessage.slice();
  attestationMutation[attestationMutation.length - 2] ^= 1;

  const pinDigest = digest(
    "multidesk-device-pin-v1",
    input.attestation.subjectDeviceId,
    subject.publicRaw,
    targetPublicRaw,
  );

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
  const sessionKey1 = hex(input.seeds.sessionKeyEpoch1);
  const wrappedKey = new Uint8Array(await sender.seal(sessionKey1, wrapAAD));
  const recipient = await suite.createRecipientContext({
    recipientKey: targetKeys.privateKey,
    enc,
    info: hpkeInfo,
    senderPublicKey: sourceKeys.publicKey,
  });
  const recoveredKey = new Uint8Array(await recipient.open(wrappedKey, wrapAAD));
  if (!Buffer.from(recoveredKey).equals(Buffer.from(sessionKey1))) {
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
    sessionKey1,
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

  const replaySequence = ["100", "98", "99", "100", "36"];
  const replay = new ReplayWindow();
  const replayVerdicts = replaySequence.map((sequence) =>
    replay.accept(sequence) ? "accept" : "reject",
  );

  const sessionKey2 = hex(input.seeds.sessionKeyEpoch2);
  const traffic2 = deriveTraffic(
    sessionKey2,
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

  const pinHex = Buffer.from(pinDigest).toString("hex");
  return {
    attestation: {
      approverPublicKey: b64url(approver.publicRaw),
      canonical: attestationCanonical,
      mutationRejected: !verify(
        null,
        attestationMutation,
        approver.publicKey,
        attestationSignature,
      ),
      signature: b64url(attestationSignature),
      subjectSigningPublicKey: b64url(subject.publicRaw),
      verifies: verify(
        null,
        attestationMessage,
        approver.publicKey,
        attestationSignature,
      ),
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
    payload: {
      aad: new TextDecoder().decode(payloadAAD),
      aadMutationRejected: payloadAADMutationRejected,
      ciphertext: b64url(payloadCiphertext),
      nonce: b64url(nonce1),
      replaySequence,
      replayVerdicts,
      roundTrip: true,
      trafficContext: traffic1.contextCanonical,
      trafficKey: b64url(traffic1.key),
    },
    pin: {
      digest: b64url(pinDigest),
      fingerprint: pinHex.match(/.{8}/g).join("-"),
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
process.stdout.write(`${JSON.stringify(await run(input))}\n`);
