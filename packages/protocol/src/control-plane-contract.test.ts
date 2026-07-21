import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import type { components } from "./generated/control-plane-v1.js";

const usageWindow: components["schemas"]["UsageWindowV1"] = {
  kind: "rolling",
  label: "requests",
  scale: 0,
  unit: "request",
  usedScaled: "10",
};

const legacyUsageWindow: components["schemas"]["UsageWindowV1"] = {
  kind: "rolling",
  label: "requests",
  scale: 0,
  unit: "request",
  // @ts-expect-error v0.7 forbids legacy floating usage values.
  usedValue: 10,
};
void legacyUsageWindow;

const supportedProvider: components["schemas"]["SessionProjectionV1"]["provider"] = "codex";
// @ts-expect-error Fake is local-only and has no network projection.
const rejectedFakeProvider: components["schemas"]["SessionProjectionV1"]["provider"] = "fake";
void supportedProvider;
void rejectedFakeProvider;

test("generated TypeScript optional and enum semantics match the Go golden", () => {
  const fixture = readFileSync("../../api/openapi/fixtures/usage-window-v1.json", "utf8").trim();
  assert.deepEqual(JSON.parse(fixture), usageWindow);
  assert.equal(JSON.stringify(usageWindow), fixture);
});
