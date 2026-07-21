import assert from "node:assert/strict";
import test from "node:test";

import { ControlPlaneError, createControlPlaneClient, operationDefinitions } from "./control-plane-client.js";

const requestId = "018f47a0-7b1c-7cc2-8000-000000000001";

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

test("cross-origin configuration and missing CSRF fail before fetch", async () => {
  Object.defineProperty(globalThis, "location", { value: new URL("https://control.test/app"), configurable: true });
  assert.throws(() => createControlPlaneClient("https://other.test"), /same-origin/);
  assert.throws(() => createControlPlaneClient("https://control.test/api"), /forbidden/);
  assert.throws(() => createControlPlaneClient("http://control.test"), /same-origin|forbidden/);
  let called = false;
  const client = createControlPlaneClient("https://control.test", async () => { called = true; return jsonResponse({}); });
  await assert.rejects(client.methods.logout({ idempotencyKey: "0123456789abcdef" }), /CSRF token/);
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
}
