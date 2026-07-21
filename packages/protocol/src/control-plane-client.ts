import type { components, operations } from "./generated/control-plane-v1.js";

type OperationId = keyof operations;
type Method = "GET" | "POST" | "PATCH" | "DELETE";
const enrollmentOperationIds = [
  "createDeviceEnrollment",
  "getDeviceEnrollment",
  "proveDeviceEnrollment",
  "getDeviceEnrollmentActivationPackage",
  "activateDeviceEnrollment",
  "cancelDeviceEnrollment",
  "resumeDeviceEnrollment",
] as const satisfies readonly OperationId[];
type EnrollmentOperationId = typeof enrollmentOperationIds[number];

type PropertyIsRequired<T, Key extends PropertyKey> = T extends object
  ? Key extends keyof T
    ? {} extends Pick<T, Key> ? false : true
    : false
  : false;
type RequestBodyFor<Operation> = Operation extends { requestBody?: infer RequestBody }
  ? Exclude<RequestBody, undefined> extends { content: { "application/json": infer Body } }
    ? Body
    : never
  : never;
type ParameterFor<Operation, Group extends "header" | "path" | "query"> =
  Operation extends { parameters: infer Parameters }
    ? Group extends keyof Parameters
      ? Exclude<Parameters[Group], undefined>
      : never
    : never;
type BodyInputFor<Operation> = [RequestBodyFor<Operation>] extends [never]
  ? { body?: never }
  : PropertyIsRequired<Operation, "requestBody"> extends true
    ? { body: RequestBodyFor<Operation> }
    : { body?: RequestBodyFor<Operation> };
type ParameterInputFor<Operation, Group extends "path" | "query"> = [ParameterFor<Operation, Group>] extends [never]
  ? { [Key in Group]?: never }
  : Operation extends { parameters: infer Parameters }
    ? PropertyIsRequired<Parameters, Group> extends true
      ? { [Key in Group]: Readonly<ParameterFor<Operation, Group>> }
      : { [Key in Group]?: Readonly<ParameterFor<Operation, Group>> }
    : { [Key in Group]?: never };

export type ControlPlaneCallShape<Operation> = BodyInputFor<Operation> &
  ParameterInputFor<Operation, "path"> &
  ParameterInputFor<Operation, "query">;
type ResponseMap<K extends OperationId> = operations[K] extends { responses: infer Responses } ? Responses : never;
type JsonContent<T> = T extends { content: { "application/json": infer Body } } ? Body : never;
type SuccessBody<K extends OperationId> = JsonContent<
  ResponseMap<K>[Extract<keyof ResponseMap<K>, 200 | 201 | 202 | 204>]
>;

export interface DeviceRequestHeaders {
  authorization: `Device ${string}`;
  deviceId: string;
  timestamp: string;
  nonce: string;
  contentSHA256: string;
  signature: string;
}

type EnrollmentGeneratedHeaders = ParameterFor<operations["createDeviceEnrollment"], "header">;

export interface EnrollmentRequestHeaders {
  enrollmentId: RequestBodyFor<operations["createDeviceEnrollment"]>["enrollmentId"];
  timestamp: EnrollmentGeneratedHeaders["X-MAD-Timestamp"];
  nonce: EnrollmentGeneratedHeaders["X-MAD-Nonce"];
  contentSHA256: EnrollmentGeneratedHeaders["X-MAD-Content-SHA256"];
  signature: EnrollmentGeneratedHeaders["X-MAD-Enrollment-Signature"];
  browserCandidate: boolean;
}

type StandardAuthorizationInput =
  | { bootstrapToken?: undefined; device?: undefined; enrollment?: never }
  | { bootstrapToken: string; device?: never; enrollment?: never }
  | { bootstrapToken?: never; device: DeviceRequestHeaders; enrollment?: never };
type AuthorizationInput<K extends OperationId> = K extends EnrollmentOperationId
  ? { bootstrapToken?: never; device?: never; enrollment: EnrollmentRequestHeaders }
  : StandardAuthorizationInput;

export type ControlPlaneCallInput<K extends OperationId> = ControlPlaneCallShape<operations[K]> & AuthorizationInput<K> & {
  idempotencyKey?: string;
  ifMatch?: `"rev-${number}"`;
  signal?: AbortSignal;
};

export class ControlPlaneError extends Error {
  readonly code: components["schemas"]["ApiError"]["code"] | "unknown_error";
  readonly requestId?: string;
  readonly status: number;

  constructor(status: number, code: string, message: string, requestId?: string) {
    super(message);
    this.name = "ControlPlaneError";
    this.status = status;
    this.code = stableErrorCodes.has(code)
      ? code as components["schemas"]["ApiError"]["code"]
      : "unknown_error";
    this.requestId = requestId;
  }
}

interface OperationDefinition {
  method: Method;
  path: `/v1/${string}`;
  csrf: boolean;
  authorization: "standard" | "enrollment";
}

const mutation = (path: `/v1/${string}`, csrf = true): OperationDefinition => ({ method: "POST", path, csrf, authorization: "standard" });
const get = (path: `/v1/${string}`): OperationDefinition => ({ method: "GET", path, csrf: false, authorization: "standard" });
const enrollmentMutation = (path: `/v1/${string}`): OperationDefinition => ({ method: "POST", path, csrf: false, authorization: "enrollment" });
const enrollmentGet = (path: `/v1/${string}`): OperationDefinition => ({ method: "GET", path, csrf: false, authorization: "enrollment" });

export const operationDefinitions = {
  getHealth: get("/v1/healthz"),
  getReady: get("/v1/readyz"),
  getVersion: get("/v1/version"),
  getCurrentAuth: get("/v1/auth/current"),
  createPasskeyAuthenticationOptions: mutation("/v1/auth/passkeys/options", false),
  verifyPasskeyAuthentication: mutation("/v1/auth/passkeys/verify", false),
  createPasskeyRegistrationOptions: mutation("/v1/auth/passkeys/registration/options"),
  verifyPasskeyRegistration: mutation("/v1/auth/passkeys/registration/verify"),
  createUvOptions: mutation("/v1/auth/uv/options"),
  verifyUv: mutation("/v1/auth/uv/verify"),
  verifyRecoveryCode: mutation("/v1/auth/recovery/verify", false),
  rotateRecoveryCodes: mutation("/v1/auth/recovery-codes/rotate"),
  logout: mutation("/v1/auth/logout"),
  listPasskeys: get("/v1/auth/passkeys"),
  deletePasskey: { method: "DELETE", path: "/v1/auth/passkeys/{passkeyId}", csrf: true, authorization: "standard" },
  listBrowserSessions: get("/v1/auth/sessions"),
  deleteBrowserSession: { method: "DELETE", path: "/v1/auth/sessions/{sessionId}", csrf: true, authorization: "standard" },
  getBootstrapStatus: get("/v1/bootstrap/status"),
  createBootstrapOptions: mutation("/v1/bootstrap/options", false),
  verifyBootstrap: mutation("/v1/bootstrap/verify", false),
  getBootstrapCeremony: get("/v1/bootstrap/ceremonies/{ceremonyId}"),
  listDevices: get("/v1/devices"),
  getDevice: get("/v1/devices/{deviceId}"),
  listDeviceEnrollments: get("/v1/device-enrollments"),
  createDeviceEnrollment: enrollmentMutation("/v1/device-enrollments"),
  getDeviceEnrollment: enrollmentGet("/v1/device-enrollments/{enrollmentId}"),
  proveDeviceEnrollment: enrollmentMutation("/v1/device-enrollments/{enrollmentId}/prove"),
  approveDeviceEnrollment: mutation("/v1/device-enrollments/{enrollmentId}/approve", false),
  getDeviceEnrollmentActivationPackage: enrollmentGet("/v1/device-enrollments/{enrollmentId}/activation-package"),
  activateDeviceEnrollment: enrollmentMutation("/v1/device-enrollments/{enrollmentId}/activate"),
  cancelDeviceEnrollment: enrollmentMutation("/v1/device-enrollments/{enrollmentId}/cancel"),
  resumeDeviceEnrollment: enrollmentGet("/v1/device-enrollments/{enrollmentId}/resume"),
  updateDeviceCapabilities: mutation("/v1/devices/{deviceId}/capabilities", false),
  revokeDevice: mutation("/v1/devices/{deviceId}/revoke"),
  createDeviceAuthChallenge: mutation("/v1/device-auth/challenges", false),
  exchangeDeviceAuth: mutation("/v1/device-auth/exchange", false),
  listAccounts: get("/v1/accounts"),
  listCredentialStatuses: get("/v1/credential-statuses"),
  listProfiles: get("/v1/profiles"),
  createProfile: mutation("/v1/profiles"),
  listWorkspaces: get("/v1/workspaces"),
  listSessions: get("/v1/sessions"),
  listUsage: get("/v1/usage"),
  listAuditEvents: get("/v1/audit-events"),
  getAccount: get("/v1/accounts/{id}"),
  getProfile: get("/v1/profiles/{id}"),
  deleteProfile: { method: "DELETE", path: "/v1/profiles/{id}", csrf: true, authorization: "standard" },
  updateProfile: { method: "PATCH", path: "/v1/profiles/{id}", csrf: true, authorization: "standard" },
  getSession: get("/v1/sessions/{id}"),
  pushDeviceSync: mutation("/v1/device/sync/push", false),
  getDeviceSyncSnapshot: get("/v1/device/sync/snapshot"),
  commitDeviceSyncSnapshot: mutation("/v1/device/sync/snapshot/commit", false),
  pullDeviceSync: get("/v1/device/sync/pull"),
  ackDeviceSync: mutation("/v1/device/sync/ack", false),
  heartbeatDevicePresence: mutation("/v1/device/presence/heartbeat", false),
  getOverview: get("/v1/overview"),
  listSessionCommands: get("/v1/session-commands"),
  createSessionCommand: mutation("/v1/session-commands"),
  getSessionCommand: get("/v1/session-commands/{commandId}"),
  pollDeviceSessionCommands: get("/v1/device/session-commands"),
  getDeviceSessionCommandState: get("/v1/device/session-commands/{commandId}"),
  claimSessionCommand: mutation("/v1/device/session-commands/{commandId}/claim", false),
  ackSessionCommand: mutation("/v1/device/session-commands/{commandId}/ack", false),
  completeSessionCommand: mutation("/v1/device/session-commands/{commandId}/result", false),
  reconcileSessionCommand: mutation("/v1/device/session-commands/{commandId}/reconcile", false),
} as const satisfies Record<OperationId, OperationDefinition>;

const stableErrorCodes = new Set<string>([
  "invalid_argument", "unauthenticated", "permission_denied", "not_found", "conflict",
  "resource_exhausted", "rate_limited", "request_too_large", "unsupported_api_version",
  "schema_incompatible", "idempotency_key_required", "idempotency_key_reused", "if_match_required",
  "sync_conflict", "sync_history_missing", "sync_base_digest_mismatch", "sync_next_digest_mismatch",
  "sync_patch_mismatch", "sync_patch_too_large", "invalid_cursor", "stale_resurrection",
  "snapshot_required", "snapshot_in_progress", "snapshot_expired", "snapshot_page_invalid",
  "snapshot_commit_conflict", "snapshot_page_too_large", "cross_server_identity_rebind",
  "bootstrap_unavailable", "bootstrap_expired", "bootstrap_replayed", "bootstrap_anchor_required",
  "origin_mismatch", "rp_id_mismatch", "webauthn_challenge_expired", "webauthn_challenge_replayed",
  "webauthn_verification_failed", "passkey_counter_regressed", "recovery_invalid_or_rate_limited",
  "recovery_consumed", "one_time_result_unavailable", "recent_uv_required", "last_passkey_required",
  "recovery_batch_replaced", "csrf_invalid", "session_expired", "device_not_enrolled", "device_revoked",
  "device_key_changed", "key_digest_mismatch", "device_key_envelope_corrupt", "device_key_envelope_conflict",
  "pin_mismatch", "attestation_invalid", "attestation_expired", "attestation_replayed",
  "cross_type_signature_replay", "enrollment_preauth_invalid", "approver_not_pinned", "capability_denied",
  "capability_revision_conflict", "capability_not_recognized", "activation_receipt_invalid",
  "enrollment_cancelled", "signature_invalid", "request_replayed", "clock_skew", "command_expired",
  "command_claimed", "command_state_conflict", "command_digest_mismatch", "projection_read_only",
  "command_attempt_stale", "command_execution_ambiguous", "command_reconciliation_required",
  "command_receipt_inconsistent", "delivery_attempts_exhausted", "phase4b_controller_required",
  "mapping_quarantined", "forbidden_metadata_field", "provider_control_unsupported",
  "provider_session_start_unsupported", "provider_resume_unsupported", "provider_stop_unsupported",
  "provider_kill_unsupported", "daemon_shutting_down", "daemon_unavailable",
]);

type RequiredKeys<Value> = {
  [Key in keyof Value]-?: {} extends Pick<Value, Key> ? never : Key;
}[keyof Value];
type ControlPlaneMethod<K extends OperationId> = K extends EnrollmentOperationId
  ? (input: ControlPlaneCallInput<K>) => Promise<SuccessBody<K>>
  : [RequiredKeys<ControlPlaneCallShape<operations[K]>>] extends [never]
    ? (input?: ControlPlaneCallInput<K>) => Promise<SuccessBody<K>>
    : (input: ControlPlaneCallInput<K>) => Promise<SuccessBody<K>>;

export type ControlPlaneMethods = { [K in OperationId]: ControlPlaneMethod<K> };

export interface ControlPlaneClient {
  readonly methods: ControlPlaneMethods;
  setCsrfToken(token: string | undefined): void;
  clearCsrfToken(): void;
}

export function createControlPlaneClient(baseURL: string, fetchImpl: typeof fetch = fetch): ControlPlaneClient {
  const parsedBase = new URL(baseURL, globalThis.location?.origin);
  if (globalThis.location && parsedBase.origin !== globalThis.location.origin) {
    throw new Error("Control Plane base URL must be same-origin");
  }
  const developmentHTTP = parsedBase.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(parsedBase.hostname);
  if ((parsedBase.protocol !== "https:" && !developmentHTTP) || parsedBase.username || parsedBase.password || parsedBase.search || parsedBase.hash || parsedBase.pathname !== "/") {
    throw new Error("Control Plane base URL contains forbidden components");
  }
  let csrfToken: string | undefined;

  const enrollmentOperations = new Set<OperationId>(enrollmentOperationIds);

  async function invoke<K extends OperationId>(id: K, input?: ControlPlaneCallInput<K>): Promise<SuccessBody<K>> {
    const definition = operationDefinitions[id];
    const call = (input ?? {}) as ControlPlaneCallInput<K>;
    const suppliedAuthorization = [call.bootstrapToken, call.device, call.enrollment].filter((value) => value !== undefined).length;
    if (suppliedAuthorization > 1) throw new Error(`${id}: authorization classes are mutually exclusive`);
    if (definition.authorization === "enrollment" && !call.enrollment) throw new Error(`${id}: Enrollment pre-auth headers are required`);
    if (definition.authorization !== "enrollment" && call.enrollment) throw new Error(`${id}: Enrollment pre-auth is forbidden for this operation`);
    if (enrollmentOperations.has(id) !== (definition.authorization === "enrollment")) {
      throw new Error(`${id}: Enrollment operation classification drifted`);
    }
    let path = definition.path as string;
    for (const [name, value] of Object.entries(call.path ?? {} as Record<string, string>)) {
      path = path.replace(`{${name}}`, encodeURIComponent(value));
    }
    if (path.includes("{")) throw new Error(`${id}: missing path parameter`);
    const url = new URL(path, parsedBase);
    for (const [name, value] of Object.entries(call.query ?? {} as Record<string, string | number | boolean | undefined>)) {
      if (value !== undefined) url.searchParams.set(name, String(value));
    }
    const headers = new Headers({ Accept: "application/json" });
    if (definition.method !== "GET") headers.set("Content-Type", "application/json");
    if (call.idempotencyKey) headers.set("Idempotency-Key", call.idempotencyKey);
    if (call.ifMatch) headers.set("If-Match", call.ifMatch);
    if (definition.csrf || (definition.method !== "GET" && call.enrollment?.browserCandidate === true)) {
      if (!csrfToken) throw new Error(`${id}: CSRF token is not loaded`);
      headers.set("X-CSRF-Token", csrfToken);
    }
    if (call.bootstrapToken) headers.set("Authorization", `Bootstrap ${call.bootstrapToken}`);
    if (call.device) {
      headers.set("Authorization", call.device.authorization);
      headers.set("X-MAD-Device-ID", call.device.deviceId);
      headers.set("X-MAD-Timestamp", call.device.timestamp);
      headers.set("X-MAD-Nonce", call.device.nonce);
      headers.set("X-MAD-Content-SHA256", call.device.contentSHA256);
      headers.set("X-MAD-Signature", call.device.signature);
    }
    if (call.enrollment) {
      const pathEnrollmentId = (call.path as { enrollmentId?: string } | undefined)?.enrollmentId;
      const bodyEnrollmentId = (call.body as { enrollmentId?: string } | undefined)?.enrollmentId;
      if ((pathEnrollmentId && pathEnrollmentId !== call.enrollment.enrollmentId) ||
          (bodyEnrollmentId && bodyEnrollmentId !== call.enrollment.enrollmentId)) {
        throw new Error(`${id}: Enrollment authorization does not match the request enrollmentId`);
      }
      headers.set("Authorization", `Enrollment ${call.enrollment.enrollmentId}`);
      headers.set("X-MAD-Timestamp", call.enrollment.timestamp);
      headers.set("X-MAD-Nonce", call.enrollment.nonce);
      headers.set("X-MAD-Content-SHA256", call.enrollment.contentSHA256);
      headers.set("X-MAD-Enrollment-Signature", call.enrollment.signature);
    }
    const timeout = AbortSignal.timeout(30_000);
    const signal = call.signal ? AbortSignal.any([call.signal, timeout]) : timeout;
    const response = await fetchImpl(url, {
      method: definition.method,
      credentials: "include",
      headers,
      body: definition.method === "GET" ? undefined : JSON.stringify(call.body ?? {}),
      signal,
    });
    const contentType = response.headers.get("content-type")?.split(";", 1)[0].trim();
    if (contentType !== "application/json") throw new ControlPlaneError(response.status, "unknown_error", "server returned a non-JSON response");
    const payload: unknown = await response.json();
    if (!response.ok) {
      const envelope = payload as Partial<components["schemas"]["ErrorEnvelope"]>;
      throw new ControlPlaneError(response.status, envelope.error?.code ?? "unknown_error", envelope.error?.message ?? "request failed", envelope.error?.requestId);
    }
    return payload as SuccessBody<K>;
  }

  const methods = Object.fromEntries(
    (Object.keys(operationDefinitions) as OperationId[]).map((id) => [id, (input?: ControlPlaneCallInput<typeof id>) => invoke(id, input)]),
  ) as unknown as ControlPlaneMethods;
  return {
    methods,
    setCsrfToken(token) { csrfToken = token; },
    clearCsrfToken() { csrfToken = undefined; },
  };
}
