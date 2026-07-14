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
      crossPeerForge: "rejected",
      crossPeerOpen: "rejected",
      envelopeAADMutation: "rejected",
      nonceSequenceMismatch: "rejected",
      oldPairwiseRootAfterRotation: "rejected",
      replayDuplicateAndTooOld: "rejected",
      sessionWrapAADMutation: "rejected",
      wrongPinnedSender: "rejected",
    },
  })}\n`,
);
