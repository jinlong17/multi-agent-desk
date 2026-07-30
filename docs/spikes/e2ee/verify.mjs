import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));

function execute(command, args, cwd) {
  const result = spawnSync(command, args, {
    cwd,
    encoding: "utf8",
    maxBuffer: 4 * 1024 * 1024,
  });
  if (result.status !== 0) {
    throw new Error(
      `${command} failed with ${result.status}: ${result.stderr || result.stdout}`,
    );
  }
  return JSON.parse(result.stdout);
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

const goResult = execute("go", ["run", ".", "../vectors.json"], join(root, "go"));
const typescriptResult = execute(
  process.execPath,
  ["validate.mjs", "../vectors.json"],
  join(root, "typescript"),
);

assert.deepEqual(typescriptResult, goResult, "Go and TypeScript vectors diverged");
assert.equal(goResult.attestation.verifies, true);
assert.equal(goResult.attestation.mutationRejected, true);
for (const field of [
  "arbitraryMapRejected",
  "capabilityMutationRejected",
  "duplicateMemberRejected",
  "escapingCanonical",
  "exchangeDigestMutationRejected",
  "exchangeKeyDigestMismatchRejected",
  "expiryMutationRejected",
  "floatRejected",
  "idMutationRejected",
  "invalidCalendarDateRejected",
  "invalidCapabilityRejected",
  "invalidHour24Rejected",
  "invalidIDRejected",
  "invalidLifetimeRejected",
  "leapDayBoundaryAccepted",
  "negativeIntegerRejected",
  "orderIndependent",
  "signingDigestMutationRejected",
  "signingKeyDigestMismatchRejected",
  "subjectKeyDigestsMatch",
  "unicodeSurrogateRejected",
  "unknownMemberRejected",
  "unsafeIntegerRejected",
]) {
  assert.equal(goResult.attestation[field], true, `attestation ${field}`);
}
assert.match(
  goResult.pin.fingerprint,
  /^[A-Z2-7]{4}(?:-[A-Z2-7]{4}){5}$/,
  "pin fingerprint must be six Base32 groups",
);
assert.equal(Buffer.from(goResult.pin.digest, "base64url").length, 32);
for (const field of [
  "alteredGroupRejected",
  "invalidBase32Rejected",
  "length23Rejected",
  "length25Rejected",
  "lowercaseAccepted",
  "oldFullHexDisplayRejected",
  "truncatedAsFullDigestRejected",
  "unhyphenatedAccepted",
]) {
  assert.equal(goResult.pin[field], true, `pin ${field}`);
}
for (const field of [
  "allZeroSharedSecretRejected",
  "ceremonyMutationRejected",
  "challengeMutationRejected",
  "deviceMutationRejected",
  "exchangeKeyMutationRejected",
  "exchangeProofContentMutationRejected",
  "exchangeProofLongRejected",
  "exchangeProofShortRejected",
  "expiryMutationRejected",
  "firstConsumeAccepted",
  "purposeMutationRejected",
  "replayRejected",
  "restartInvalidated",
  "serverEphemeralMutationRejected",
  "signingKeyMutationRejected",
  "storageAssertionMutationRejected",
  "storageModeMutationRejected",
  "verifies",
]) {
  assert.equal(goResult.keyPop[field], true, `keyPop ${field}`);
}
assert.equal(Buffer.from(goResult.keyPop.sharedSecret, "base64url").length, 32);
assert.equal(Buffer.from(goResult.keyPop.popKey, "base64url").length, 32);
assert.equal(Buffer.from(goResult.keyPop.exchangeProof, "base64url").length, 32);
assert.equal(Buffer.from(goResult.keyPop.signingProof, "base64url").length, 64);
assert.equal(goResult.keyWrap.unwrapMatches, true);
assert.equal(goResult.keyWrap.aadMutationRejected, true);
assert.equal(goResult.keyWrap.wrongPinnedSenderRejected, true);
assert.equal(goResult.payload.roundTrip, true);
assert.equal(goResult.payload.aadMutationRejected, true);
assert.equal(goResult.payload.nonceSequenceMismatchRejected, true);
assert.equal(goResult.crossPeer.peerAForPeerBOpenRejected, true);
assert.equal(goResult.crossPeer.peerAForPeerBForgeRejected, true);
assert.deepEqual(goResult.payload.replayVerdicts, [
  "accept",
  "accept",
  "accept",
  "reject",
  "reject",
]);
assert.equal(goResult.rotation.oldKeyRejected, true);
assert.equal(goResult.rotation.newKeyRecovered, true);

const canonicalResult = canonical(goResult);
const resultSha256 = createHash("sha256")
  .update(canonicalResult)
  .digest("hex");

process.stdout.write(
  `${JSON.stringify({
    schemaVersion: 1,
    result: "pass",
    implementations: ["go", "typescript"],
    resultSha256,
    negativeCases: {
      attestationMutation: "rejected",
      attestationSchemaAndJCS: "rejected",
      crossPeerForge: "rejected",
      crossPeerOpen: "rejected",
      envelopeAADMutation: "rejected",
      nonceSequenceMismatch: "rejected",
      oldPairwiseRootAfterRotation: "rejected",
      pinPresentationAndTruncation: "rejected",
      popAllZeroSharedSecret: "rejected",
      popFieldMutations: "rejected",
      popReplayAndRestart: "rejected",
      replayDuplicateAndTooOld: "rejected",
      sessionWrapAADMutation: "rejected",
      wrongPinnedSender: "rejected",
    },
  })}\n`,
);
