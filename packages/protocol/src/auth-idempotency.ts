import type { components } from "./generated/control-plane-v1.js";

export const authIdempotencyOperations = [
  "bootstrap_options",
  "bootstrap_verify",
  "passkey_login_options",
  "passkey_login_verify",
  "passkey_registration_options",
  "passkey_registration_verify",
  "passkey_delete",
  "uv_options",
  "uv_verify",
  "recovery_verify",
  "recovery_codes_rotate",
  "logout",
  "session_delete",
] as const satisfies readonly components["schemas"]["AuthIdempotencyOperationV1"][];

export type AuthIdempotencyOperationV1 = components["schemas"]["AuthIdempotencyOperationV1"];

const authOperationContracts = {
  bootstrap_options: { actor: "bootstrap_token", method: "POST", path: "/v1/bootstrap/options", ifMatch: false },
  bootstrap_verify: { actor: "bootstrap_token", method: "POST", path: "/v1/bootstrap/verify", ifMatch: false },
  passkey_login_options: { actor: "preauth_browser", method: "POST", path: "/v1/auth/passkeys/options", ifMatch: false },
  passkey_login_verify: { actor: "preauth_browser", method: "POST", path: "/v1/auth/passkeys/verify", ifMatch: false },
  passkey_registration_options: { actor: "browser_session", method: "POST", path: "/v1/auth/passkeys/registration/options", ifMatch: false },
  passkey_registration_verify: { actor: "browser_session", method: "POST", path: "/v1/auth/passkeys/registration/verify", ifMatch: false },
  passkey_delete: { actor: "browser_session", method: "DELETE", prefix: "/v1/auth/passkeys/", ifMatch: true },
  uv_options: { actor: "browser_session", method: "POST", path: "/v1/auth/uv/options", ifMatch: false },
  uv_verify: { actor: "browser_session", method: "POST", path: "/v1/auth/uv/verify", ifMatch: false },
  recovery_verify: { actor: "preauth_browser", method: "POST", path: "/v1/auth/recovery/verify", ifMatch: false },
  recovery_codes_rotate: { actor: "browser_session", method: "POST", path: "/v1/auth/recovery-codes/rotate", ifMatch: false },
  logout: { actor: "browser_session", method: "POST", path: "/v1/auth/logout", ifMatch: false },
  session_delete: { actor: "browser_session", method: "DELETE", prefix: "/v1/auth/sessions/", ifMatch: true },
} as const satisfies Record<AuthIdempotencyOperationV1, {
  actor: "bootstrap_token" | "preauth_browser" | "browser_session";
  method: "POST" | "DELETE";
  path?: `/v1/${string}`;
  prefix?: `/v1/${string}`;
  ifMatch: boolean;
}>;

const encoder = new TextEncoder();

function assertUnicodeScalarString(value: string): void {
  for (let index = 0; index < value.length; index++) {
    const codeUnit = value.charCodeAt(index);
    if (codeUnit >= 0xd800 && codeUnit <= 0xdbff) {
      const low = value.charCodeAt(index + 1);
      if (!(low >= 0xdc00 && low <= 0xdfff)) {
        throw new Error("JCS strings cannot contain lone UTF-16 surrogates");
      }
      index++;
    } else if (codeUnit >= 0xdc00 && codeUnit <= 0xdfff) {
      throw new Error("JCS strings cannot contain lone UTF-16 surrogates");
    }
  }
}

export function normalizeIdempotencyKeyV1(value: string): string {
  const normalized = value.replace(/^[ \t]+|[ \t]+$/g, "");
  if (!/^[\x21-\x2b\x2d-\x7e]{16,128}$/.test(normalized)) {
    throw new Error("invalid P2 Idempotency-Key");
  }
  return normalized;
}

export function frameV1(...fields: readonly Uint8Array[]): Uint8Array {
  const size = fields.reduce((total, field) => total + 4 + field.byteLength, 0);
  const result = new Uint8Array(size);
  const view = new DataView(result.buffer);
  let offset = 0;
  for (const field of fields) {
    view.setUint32(offset, field.byteLength, false);
    offset += 4;
    result.set(field, offset);
    offset += field.byteLength;
  }
  return result;
}

export function canonicalJSONV1(value: unknown): string {
  if (typeof value === "string") {
    assertUnicodeScalarString(value);
    return JSON.stringify(value);
  }
  if (value === null || typeof value === "boolean") {
    return JSON.stringify(value);
  }
  if (typeof value === "number") {
    if (!Number.isFinite(value)) throw new Error("JCS numbers must be finite IEEE-754 values");
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map((item) => canonicalJSONV1(item)).join(",")}]`;
  }
  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    return `{${Object.keys(record).sort().map((key) => {
      assertUnicodeScalarString(key);
      const item = record[key];
      if (item === undefined || typeof item === "bigint" || typeof item === "function" || typeof item === "symbol") {
        throw new Error("JCS objects cannot contain non-JSON values");
      }
      return `${JSON.stringify(key)}:${canonicalJSONV1(item)}`;
    }).join(",")}}`;
  }
  throw new Error("value is outside the JCS data model");
}

async function sha256(value: Uint8Array): Promise<Uint8Array> {
  const copy = new Uint8Array(value.byteLength);
  copy.set(value);
  return new Uint8Array(await crypto.subtle.digest("SHA-256", copy.buffer));
}

function text(value: string): Uint8Array { return encoder.encode(value); }

export async function authIdempotencyKeyDigestV1(normalizedKey: string): Promise<Uint8Array> {
  if (normalizeIdempotencyKeyV1(normalizedKey) !== normalizedKey) {
    throw new Error("P2 Idempotency-Key must already be normalized");
  }
  return sha256(frameV1(text("multidesk-auth-idempotency-key-v1"), text("1"), text(normalizedKey)));
}

export async function authIdempotencyBodyDigestV1(canonicalStrictJSON: string): Promise<Uint8Array> {
  if (canonicalJSONV1(JSON.parse(canonicalStrictJSON)) !== canonicalStrictJSON) {
    throw new Error("body is not CanonicalStrictJSONV1");
  }
  return sha256(frameV1(text("multidesk-auth-idempotency-body-v1"), text("1"), text(canonicalStrictJSON)));
}

export async function authIdempotencyRequestIdentityDigestV1(input: {
  serverOrigin: string;
  actorClass: "bootstrap_token" | "preauth_browser" | "browser_session";
  actorIdentityRaw: Uint8Array;
  operation: AuthIdempotencyOperationV1;
  method: "POST" | "DELETE";
  canonicalPath: `/v1/${string}`;
  bodyDigest: Uint8Array;
  canonicalIfMatch: "" | `"rev-${number}"`;
}): Promise<Uint8Array> {
  const contract = authOperationContracts[input.operation];
  const dynamicId = "prefix" in contract ? input.canonicalPath.slice(contract.prefix.length) : "";
  const pathValid = "path" in contract
    ? input.canonicalPath === contract.path
    : input.canonicalPath.startsWith(contract.prefix) && /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(dynamicId);
  if (input.actorIdentityRaw.byteLength !== 32 || input.bodyDigest.byteLength !== 32 ||
      contract.actor !== input.actorClass || contract.method !== input.method || !pathValid || input.canonicalPath.endsWith("/") ||
      input.canonicalPath.includes("?") || input.canonicalPath.includes("#") ||
      (contract.ifMatch ? !/^"rev-[1-9][0-9]*"$/.test(input.canonicalIfMatch) : input.canonicalIfMatch !== "")) {
    throw new Error("invalid auth idempotency request identity");
  }
  return sha256(frameV1(
    text("multidesk-auth-idempotency-request-identity-v1"), text("1"), text(input.serverOrigin),
    text(input.actorClass), input.actorIdentityRaw, text(input.operation), text(input.method),
    text(input.canonicalPath), input.bodyDigest, text(input.canonicalIfMatch),
  ));
}
