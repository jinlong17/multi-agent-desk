import assert from "node:assert/strict";
import test from "node:test";

import {
  ControlPlaneError,
  createControlPlaneClient,
  operationDefinitions,
  type ControlPlaneCallShape,
  type EnrollmentRequestHeaders,
} from "./control-plane-client.js";

const requestId = "018f47a0-7b1c-7cc2-8000-000000000001";

function enrollmentHeaders(browserCandidate: boolean): EnrollmentRequestHeaders {
  return {
    enrollmentId: requestId,
    timestamp: "2030-03-01T00:00:00Z",
    nonce: "N".repeat(22),
    contentSHA256: "D".repeat(43),
    signature: "S".repeat(86),
    browserCandidate,
  };
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

test("operation map is exhaustive and has no arbitrary path escape hatch", () => {
  assert.equal(Object.keys(operationDefinitions).length, 65);
  const client = createControlPlaneClient("https://control.test");
  assert.deepEqual(Object.keys(client.methods).sort(), Object.keys(operationDefinitions).sort());
  assert.equal("request" in client, false);
});

test("same-origin JSON request includes credentials, CSRF, idempotency and body", async () => {
  Object.defineProperty(globalThis, "location", { value: new URL("https://control.test/app"), configurable: true });
  let captured: { url?: URL; init?: RequestInit } = {};
  const client = createControlPlaneClient("https://control.test", async (input, init) => {
    captured = { url: new URL(String(input)), init };
    return jsonResponse({ apiVersion: "v1", data: { id: requestId }, meta: { requestId, nextCursor: null } });
  });
  client.setCsrfToken("csrf-token");
  await client.methods.createProfile({
    idempotencyKey: "0123456789abcdef",
    body: {
      targetDeviceId: requestId,
      provider: "codex",
      name: "build",
      environmentNonSecret: {},
      mcpRefKeys: [],
      skillRefKeys: [],
      hookRefKeys: [],
    },
  });
  assert.equal(captured.url?.href, "https://control.test/v1/profiles");
  assert.equal(captured.init?.credentials, "include");
  const headers = new Headers(captured.init?.headers);
  assert.equal(headers.get("content-type"), "application/json");
  assert.equal(headers.get("x-csrf-token"), "csrf-token");
  assert.equal(headers.get("idempotency-key"), "0123456789abcdef");
  assert.match(String(captured.init?.body), /"provider":"codex"/);
});

test("Enrollment pre-auth sends exact GET and mutation headers with browser CSRF composition", async () => {
  Object.defineProperty(globalThis, "location", { value: new URL("https://control.test/app"), configurable: true });
  const captured: Array<{ url: URL; init?: RequestInit }> = [];
  const client = createControlPlaneClient("https://control.test", async (input, init) => {
    captured.push({ url: new URL(String(input)), init });
    return jsonResponse({ apiVersion: "v1", data: {}, meta: { requestId, nextCursor: null } });
  });
  client.setCsrfToken("C".repeat(42));
  await client.methods.getDeviceEnrollment({
    path: { enrollmentId: requestId },
    enrollment: enrollmentHeaders(true),
  });
  await client.methods.cancelDeviceEnrollment({
    path: { enrollmentId: requestId },
    body: { expectedEnrollmentRevision: 1, reason: "operator_cancelled" },
    idempotencyKey: "0123456789abcdef",
    enrollment: enrollmentHeaders(true),
  });

  assert.equal(captured.length, 2);
  const getHeaders = new Headers(captured[0]?.init?.headers);
  assert.equal(captured[0]?.url.href, `https://control.test/v1/device-enrollments/${requestId}`);
  assert.equal(captured[0]?.init?.method, "GET");
  assert.equal(captured[0]?.init?.credentials, "include");
  assert.equal(getHeaders.get("authorization"), `Enrollment ${requestId}`);
  assert.equal(getHeaders.get("x-mad-timestamp"), "2030-03-01T00:00:00Z");
  assert.equal(getHeaders.get("x-mad-nonce"), "N".repeat(22));
  assert.equal(getHeaders.get("x-mad-content-sha256"), "D".repeat(43));
  assert.equal(getHeaders.get("x-mad-enrollment-signature"), "S".repeat(86));
  assert.equal(getHeaders.get("x-mad-signature"), null);
  assert.equal(getHeaders.get("x-csrf-token"), null);

  const mutationHeaders = new Headers(captured[1]?.init?.headers);
  assert.equal(captured[1]?.init?.method, "POST");
  assert.equal(captured[1]?.init?.credentials, "include");
  assert.equal(mutationHeaders.get("authorization"), `Enrollment ${requestId}`);
  assert.equal(mutationHeaders.get("x-mad-enrollment-signature"), "S".repeat(86));
  assert.equal(mutationHeaders.get("x-csrf-token"), "C".repeat(42));
  assert.equal(mutationHeaders.get("idempotency-key"), "0123456789abcdef");
});

test("Enrollment authorization classes, CSRF mode, identity and operation coverage fail closed", async () => {
  Object.defineProperty(globalThis, "location", { value: new URL("https://control.test/app"), configurable: true });
  let calls = 0;
  const client = createControlPlaneClient("https://control.test", async () => {
    calls++;
    return jsonResponse({ apiVersion: "v1", data: {}, meta: { requestId, nextCursor: null } });
  });
  const unsafeEnrollmentCall = client.methods.getDeviceEnrollment as unknown as (input: Record<string, unknown>) => Promise<unknown>;
  const input = { path: { enrollmentId: requestId }, enrollment: enrollmentHeaders(true) };
  await assert.rejects(unsafeEnrollmentCall({ ...input, bootstrapToken: "collision" }), /mutually exclusive/);
  await assert.rejects(unsafeEnrollmentCall({ ...input, device: {
    authorization: `Device token`, deviceId: requestId, timestamp: "2030-03-01T00:00:00Z",
    nonce: "N".repeat(22), contentSHA256: "D".repeat(43), signature: "S".repeat(86),
  } }), /mutually exclusive/);
  await assert.rejects(unsafeEnrollmentCall({ path: { enrollmentId: requestId } }), /headers are required/);
  await assert.rejects(unsafeEnrollmentCall({
    path: { enrollmentId: "018f47a0-7b1c-7cc2-8000-000000000002" },
    enrollment: enrollmentHeaders(true),
  }), /does not match/);
  const unsafeHealth = client.methods.getHealth as unknown as (input: Record<string, unknown>) => Promise<unknown>;
  await assert.rejects(unsafeHealth({ enrollment: enrollmentHeaders(false) }), /forbidden/);
  assert.equal(calls, 0);

  const unsafeCancel = client.methods.cancelDeviceEnrollment as unknown as (input: Record<string, unknown>) => Promise<unknown>;
  await assert.rejects(unsafeCancel({
    path: { enrollmentId: requestId }, body: { expectedEnrollmentRevision: 1, reason: "x" },
    idempotencyKey: "0123456789abcdef", enrollment: enrollmentHeaders(true),
  }), /CSRF token/);
  assert.equal(calls, 0);
  await unsafeCancel({
    path: { enrollmentId: requestId }, body: { expectedEnrollmentRevision: 1, reason: "x" },
    idempotencyKey: "0123456789abcdef", enrollment: enrollmentHeaders(false),
  });
  assert.equal(calls, 1);

  assert.deepEqual(
    Object.entries(operationDefinitions).filter(([, definition]) => definition.authorization === "enrollment").map(([id]) => id),
    [
      "createDeviceEnrollment", "getDeviceEnrollment", "proveDeviceEnrollment",
      "getDeviceEnrollmentActivationPackage", "activateDeviceEnrollment",
      "cancelDeviceEnrollment", "resumeDeviceEnrollment",
    ],
  );
});

test("cross-origin configuration and missing CSRF fail before fetch", async () => {
  Object.defineProperty(globalThis, "location", { value: new URL("https://control.test/app"), configurable: true });
  assert.throws(() => createControlPlaneClient("https://other.test"), /same-origin/);
  assert.throws(() => createControlPlaneClient("https://control.test/api"), /forbidden/);
  assert.throws(() => createControlPlaneClient("http://control.test"), /same-origin|forbidden/);
  let called = false;
  const client = createControlPlaneClient("https://control.test", async () => { called = true; return jsonResponse({}); });
  await assert.rejects(client.methods.logout({ idempotencyKey: "0123456789abcdef" }), /CSRF token/);
  const unsafePathCall = client.methods.getProfile as unknown as (input: Record<string, unknown>) => Promise<unknown>;
  await assert.rejects(unsafePathCall({}), /missing path parameter/);
  assert.equal(called, false);
});

test("caller AbortSignal is composed with the 30 second client deadline", async () => {
  Object.defineProperty(globalThis, "location", { value: new URL("https://control.test/app"), configurable: true });
  const controller = new AbortController();
  const client = createControlPlaneClient("https://control.test", async (_input, init) => new Promise<Response>((_resolve, reject) => {
    const signal = init?.signal;
    assert.ok(signal);
    signal.addEventListener("abort", () => reject(signal.reason), { once: true });
  }));
  const pending = client.methods.pollDeviceSessionCommands({ query: { waitSeconds: 25 }, signal: controller.signal });
  controller.abort(new Error("caller cancelled"));
  await assert.rejects(pending, /caller cancelled/);
});

test("stable and unknown JSON errors map safely; non-JSON never succeeds", async () => {
  Object.defineProperty(globalThis, "location", { value: new URL("https://control.test/app"), configurable: true });
  const stable = createControlPlaneClient("https://control.test", async () => jsonResponse({ apiVersion: "v1", error: { code: "invalid_argument", message: "bad", requestId, details: [] } }, 400));
  await assert.rejects(stable.methods.getHealth(), (error: unknown) => error instanceof ControlPlaneError && error.code === "invalid_argument" && error.requestId === requestId);
  const unknown = createControlPlaneClient("https://control.test", async () => jsonResponse({ apiVersion: "v1", error: { code: "future_code", message: "bad", requestId, details: [] } }, 500));
  await assert.rejects(unknown.methods.getHealth(), (error: unknown) => error instanceof ControlPlaneError && error.code === "unknown_error");
  const html = createControlPlaneClient("https://control.test", async () => new Response("<html>", { status: 502, headers: { "Content-Type": "text/html" } }));
  await assert.rejects(html.methods.getHealth(), (error: unknown) => error instanceof ControlPlaneError && error.code === "unknown_error");
});

if (false) {
  const typed = createControlPlaneClient("https://control.test");
  // @ts-expect-error query names are generated per operation; no arbitrary query escape hatch exists.
  void typed.methods.listDeviceEnrollments({ query: { unknownFilter: "x" } });
  // @ts-expect-error generated required requestBody makes the method input mandatory.
  void typed.methods.createProfile();
  // @ts-expect-error generated required requestBody cannot be omitted from a supplied input.
  void typed.methods.createProfile({ idempotencyKey: "0123456789abcdef" });
  // @ts-expect-error generated required path makes the method input mandatory.
  void typed.methods.getProfile();
  // @ts-expect-error generated required path cannot be omitted from a supplied input.
  void typed.methods.getProfile({});
  // @ts-expect-error Enrollment operations require the distinct Enrollment authorization input.
  void typed.methods.getDeviceEnrollment({ path: { enrollmentId: requestId } });
  // Enrollment authorization cannot collide with Device authorization.
  void typed.methods.getDeviceEnrollment({
    path: { enrollmentId: requestId }, enrollment: enrollmentHeaders(false),
    // @ts-expect-error the Enrollment input union forbids a second authorization class.
    device: { authorization: "Device token", deviceId: requestId, timestamp: "x", nonce: "x", contentSHA256: "x", signature: "x" },
  });

  type RequiredQueryFixture = ControlPlaneCallShape<{
    parameters: { query: { cursor: string }; header?: never; path?: never; cookie?: never };
    requestBody?: never;
  }>;
  const acceptsRequiredQuery = (_input: RequiredQueryFixture) => undefined;
  // @ts-expect-error the same production generic preserves a generated required query group.
  acceptsRequiredQuery({});
  acceptsRequiredQuery({ query: { cursor: "cursor" } });
}
