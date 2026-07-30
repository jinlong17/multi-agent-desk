import assert from "node:assert/strict";
import test from "node:test";

import {
  authIdempotencyBodyDigestV1,
  authIdempotencyKeyDigestV1,
  authIdempotencyOperations,
  authIdempotencyRequestIdentityDigestV1,
  canonicalJSONV1,
  normalizeIdempotencyKeyV1,
} from "./auth-idempotency.js";

function hex(value: Uint8Array): string {
  return Array.from(value, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

test("JCS uses RFC 8785 UTF-16 ordering and ECMAScript number boundaries", () => {
  assert.equal(canonicalJSONV1({
    numbers: [333333333.33333329, 1e30, 4.50, 2e-3, 1e-27],
    string: "€$\u000f\nA'B\"\\\"/",
    literals: [null, true, false],
  }), `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\\u000f\\nA'B\\\"\\\\\\\"/"}`);
  assert.equal(canonicalJSONV1([-0, 1e-6, 1e-7, 1e20, 1e21]), `[0,0.000001,1e-7,100000000000000000000,1e+21]`);
  assert.throws(() => canonicalJSONV1([Number.POSITIVE_INFINITY]), /finite/);
  assert.throws(() => canonicalJSONV1("\ud800"), /surrogate/);
  assert.throws(() => canonicalJSONV1("\udfff"), /surrogate/);
  assert.throws(() => canonicalJSONV1({ "\ud800": true }), /surrogate/);
  assert.equal(canonicalJSONV1("\ud83d\ude00"), `"😀"`);
});

test("P2 key normalization and exact Go-TypeScript digest goldens", async () => {
  assert.equal(normalizeIdempotencyKeyV1(" \t0123456789abcdef\t "), "0123456789abcdef");
  for (const invalid of ["short", "0123456789abcde,", "0123456789abcde\n", "0123456789abcde£"]) {
    assert.throws(() => normalizeIdempotencyKeyV1(invalid));
  }
  assert.equal(hex(await authIdempotencyKeyDigestV1("0123456789abcdef")), "3bbe6124a204cb21542b8142aba3782c3a5484a18a5525ab2eed4673b5a6ca92");
  const body = await authIdempotencyBodyDigestV1("{}");
  assert.equal(hex(body), "4c1d668fb3a72160b6883c25379d844eac6ad23dd9794219a92c84546299a6d1");
  const request = await authIdempotencyRequestIdentityDigestV1({
    serverOrigin: "https://control.example.test",
    actorClass: "browser_session",
    actorIdentityRaw: new Uint8Array(32).fill(0x5a),
    operation: "logout",
    method: "POST",
    canonicalPath: "/v1/auth/logout",
    bodyDigest: body,
    canonicalIfMatch: "",
  });
  assert.equal(hex(request), "66302fe7c8bb4af27b2da538a00ebc716057ed380d1f351b13155f07b81209ad");
});

test("request identity rejects cross-actor/path/method/If-Match substitution", async () => {
  const body = await authIdempotencyBodyDigestV1("{}");
  const base = {
    serverOrigin: "https://control.example.test",
    actorClass: "browser_session" as const,
    actorIdentityRaw: new Uint8Array(32),
    operation: "session_delete" as const,
    method: "DELETE" as const,
    canonicalPath: "/v1/auth/sessions/018f47a0-7b1c-7cc2-8000-000000000001" as const,
    bodyDigest: body,
    canonicalIfMatch: `"rev-1"` as const,
  };
  await authIdempotencyRequestIdentityDigestV1(base);
  await assert.rejects(authIdempotencyRequestIdentityDigestV1({ ...base, actorClass: "preauth_browser" }));
  await assert.rejects(authIdempotencyRequestIdentityDigestV1({ ...base, canonicalPath: "/v1/auth/sessions/UPPER" }));
  await assert.rejects(authIdempotencyRequestIdentityDigestV1({ ...base, canonicalIfMatch: "" }));
  assert.equal(authIdempotencyOperations.length, 13);
});
