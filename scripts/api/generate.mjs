import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createHash } from "node:crypto";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

import { directInvocation, execFileNoShell, nodeCLIInvocation } from "./process-runner.mjs";

const root = resolve(import.meta.dirname, "../..");
const specPath = join(root, "api/openapi/control-plane-v1.yaml");
const configPath = join(root, "api/openapi/oapi-codegen-v1.yaml");
const goOutput = join(root, "internal/controlplane/api/generated/control_plane_v1.gen.go");
const tsOutput = join(root, "packages/protocol/src/generated/control-plane-v1.ts");
const openapiTypescriptCLI = join(root, "node_modules/openapi-typescript/bin/cli.js");
const mode = process.argv[2] ?? "generate";
const env = { ...process.env, LC_ALL: "C", LANG: "C", TZ: "UTC" };

function run(command) {
  execFileNoShell(command, { cwd: root, env, stdio: "inherit" });
}

function validateContract() {
  const spec = JSON.parse(readFileSync(specPath, "utf8"));
  if (spec.openapi !== "3.0.3") throw new Error("OpenAPI authority must remain 3.0.3");
  if (spec.info?.version !== "1.0.0-v0.9" || !spec.info.description?.includes("P2 mounts the 18") || spec.info.description.includes("P1 mounts only")) {
    throw new Error("OpenAPI Phase 4a P2 authority description is stale");
  }
  const expectedOperations = new Set([
    "getHealth", "getReady", "getVersion", "getCurrentAuth",
    "createPasskeyAuthenticationOptions", "verifyPasskeyAuthentication",
    "createPasskeyRegistrationOptions", "verifyPasskeyRegistration", "createUvOptions", "verifyUv",
    "verifyRecoveryCode", "rotateRecoveryCodes", "logout", "listPasskeys", "deletePasskey",
    "listBrowserSessions", "deleteBrowserSession", "getBootstrapStatus", "createBootstrapOptions",
    "verifyBootstrap", "getBootstrapCeremony", "listDevices", "getDevice", "listDeviceEnrollments",
    "createDeviceEnrollment", "getDeviceEnrollment", "proveDeviceEnrollment", "approveDeviceEnrollment",
    "getDeviceEnrollmentActivationPackage", "activateDeviceEnrollment", "cancelDeviceEnrollment",
    "resumeDeviceEnrollment", "updateDeviceCapabilities", "revokeDevice", "createDeviceAuthChallenge",
    "exchangeDeviceAuth", "listAccounts", "listCredentialStatuses", "listProfiles", "createProfile",
    "listWorkspaces", "listSessions", "listUsage", "listAuditEvents", "getAccount", "getProfile",
    "updateProfile", "deleteProfile", "getSession", "pushDeviceSync", "getDeviceSyncSnapshot",
    "commitDeviceSyncSnapshot", "pullDeviceSync", "ackDeviceSync", "heartbeatDevicePresence", "getOverview",
    "createSessionCommand", "listSessionCommands", "getSessionCommand", "pollDeviceSessionCommands",
    "getDeviceSessionCommandState", "claimSessionCommand", "ackSessionCommand", "completeSessionCommand",
    "reconcileSessionCommand",
  ]);
  const operations = [];
  const operationsById = new Map();
  const p2IdempotentOperationMap = new Map([
    ["createBootstrapOptions", "bootstrap_options"],
    ["verifyBootstrap", "bootstrap_verify"],
    ["createPasskeyAuthenticationOptions", "passkey_login_options"],
    ["verifyPasskeyAuthentication", "passkey_login_verify"],
    ["createPasskeyRegistrationOptions", "passkey_registration_options"],
    ["verifyPasskeyRegistration", "passkey_registration_verify"],
    ["deletePasskey", "passkey_delete"],
    ["createUvOptions", "uv_options"],
    ["verifyUv", "uv_verify"],
    ["verifyRecoveryCode", "recovery_verify"],
    ["rotateRecoveryCodes", "recovery_codes_rotate"],
    ["logout", "logout"],
    ["deleteBrowserSession", "session_delete"],
  ]);
  const p2IdempotentOperations = new Set(p2IdempotentOperationMap.keys());
  const p2Operations = new Set([
    "getBootstrapStatus", "createBootstrapOptions", "verifyBootstrap", "getBootstrapCeremony",
    "getCurrentAuth", "createPasskeyAuthenticationOptions", "verifyPasskeyAuthentication",
    "createPasskeyRegistrationOptions", "verifyPasskeyRegistration", "createUvOptions", "verifyUv",
    "verifyRecoveryCode", "rotateRecoveryCodes", "logout", "listPasskeys", "deletePasskey",
    "listBrowserSessions", "deleteBrowserSession",
  ]);
  const p2EmptyBodyOperations = new Set([
    "createPasskeyAuthenticationOptions", "createPasskeyRegistrationOptions", "createUvOptions",
    "rotateRecoveryCodes", "logout", "deletePasskey", "deleteBrowserSession",
  ]);
  const p2SecretOperations = new Set([
    "verifyBootstrap", "verifyPasskeyAuthentication", "verifyPasskeyRegistration",
    "verifyUv", "verifyRecoveryCode", "rotateRecoveryCodes",
  ]);
  const p2IssuedCookieOperations = new Set([
    "verifyBootstrap", "verifyPasskeyAuthentication", "verifyPasskeyRegistration",
    "verifyUv", "verifyRecoveryCode",
  ]);
  const p2ClearedCookieOperations = new Set(["logout", "deletePasskey", "deleteBrowserSession"]);
  const p2NamedBodyOperations = new Map([
    ["createBootstrapOptions", "#/components/schemas/BootstrapOptionsRequestV1"],
    ["verifyBootstrap", "#/components/schemas/BootstrapVerifyRequestV1"],
    ["verifyPasskeyAuthentication", "#/components/schemas/WebAuthnAssertionVerifyRequestV1"],
    ["verifyPasskeyRegistration", "#/components/schemas/WebAuthnRegistrationVerifyRequestV1"],
    ["verifyUv", "#/components/schemas/WebAuthnAssertionVerifyRequestV1"],
    ["verifyRecoveryCode", "#/components/schemas/RecoveryVerifyRequestV1"],
  ]);
  for (const [path, item] of Object.entries(spec.paths)) {
    if (!path.startsWith("/v1/")) throw new Error(`unversioned path: ${path}`);
    for (const [method, operation] of Object.entries(item)) {
      if (!operation.operationId) throw new Error(`${method} ${path}: operationId missing`);
      if (!Array.isArray(operation.security)) throw new Error(`${operation.operationId}: security missing`);
      const success = Object.entries(operation.responses).find(([status]) => /^2/.test(status));
      if (!success) throw new Error(`${operation.operationId}: success response missing`);
      const successMedia = success[1].content?.["application/json"];
      if (!successMedia?.schema || successMedia.example === undefined) throw new Error(`${operation.operationId}: typed schema/example missing`);
      for (const status of ["400", "401", "403", "409", "413", "422", "429", "500", "503"]) {
        if (!operation.responses[status]) throw new Error(`${operation.operationId}: stable ${status} error missing`);
      }
      const idempotencyParameter = operation.parameters.find((entry) => entry.$ref?.endsWith("/IdempotencyKey") || entry.$ref?.endsWith("/P2IdempotencyKey"));
      if (method !== "get" && !idempotencyParameter) {
        throw new Error(`${operation.operationId}: Idempotency-Key missing`);
      }
      const wantsP2Key = p2IdempotentOperations.has(operation.operationId);
      if (method !== "get" && idempotencyParameter.$ref !== `#/components/parameters/${wantsP2Key ? "P2IdempotencyKey" : "IdempotencyKey"}`) {
        throw new Error(`${operation.operationId}: wrong Idempotency-Key contract`);
      }
      operations.push(operation.operationId);
      operationsById.set(operation.operationId, operation);
    }
  }
  if (operations.length !== 65) throw new Error(`expected 65 operations, got ${operations.length}`);
  if (new Set(operations).size !== operations.length) throw new Error("operationId values must be unique");
  for (const operation of operations) {
    if (!expectedOperations.delete(operation)) throw new Error(`unexpected operation: ${operation}`);
  }
  if (expectedOperations.size) throw new Error(`missing v0.7 operations: ${[...expectedOperations].join(", ")}`);
  if (p2IdempotentOperations.size !== 13) throw new Error("P2 idempotency endpoint closure drifted");
  if (p2Operations.size !== 18 || p2EmptyBodyOperations.size !== 7 || p2SecretOperations.size !== 6 ||
      p2IssuedCookieOperations.size !== 5 || p2ClearedCookieOperations.size !== 3) {
    throw new Error("P2 route/schema/header closure drifted");
  }
  const expectedStoredOperations = [...p2IdempotentOperationMap.values()];
  if (JSON.stringify(spec.components.schemas.AuthIdempotencyOperationV1?.enum) !== JSON.stringify(expectedStoredOperations)) {
    throw new Error("P2 endpoint to stored idempotency operation mapping drifted");
  }
  const p2Key = spec.components.parameters.P2IdempotencyKey;
  if (p2Key?.schema?.pattern !== "^[\\x21-\\x2b\\x2d-\\x7e]{16,128}$" || p2Key.schema.minLength !== 16 || p2Key.schema.maxLength !== 128) {
    throw new Error("P2 Idempotency-Key normalization contract drifted");
  }

  const enrollmentOperations = [
    "createDeviceEnrollment", "getDeviceEnrollment", "proveDeviceEnrollment",
    "getDeviceEnrollmentActivationPackage", "activateDeviceEnrollment",
    "cancelDeviceEnrollment", "resumeDeviceEnrollment",
  ];
  const enrollmentHeaderRefs = [
    "RequestTimestamp", "RequestNonce", "RequestContentSHA256", "EnrollmentSignature",
  ];
  for (const operationId of enrollmentOperations) {
    const operation = operationsById.get(operationId);
    if (JSON.stringify(operation.security) !== JSON.stringify([{ EnrollmentPreAuth: [] }])) {
      throw new Error(`${operationId}: EnrollmentPreAuth must be the sole authorization scheme`);
    }
    const actualHeaderRefs = operation.parameters
      .flatMap((parameter) => parameter.$ref?.startsWith("#/components/parameters/") ? [parameter.$ref.split("/").at(-1)] : [])
      .filter((name) => enrollmentHeaderRefs.includes(name));
    if (JSON.stringify(actualHeaderRefs) !== JSON.stringify(enrollmentHeaderRefs)) {
      throw new Error(`${operationId}: exact Enrollment pre-auth signed headers are missing or reordered`);
    }
  }
  const exactEnrollmentParameters = {
    RequestTimestamp: ["X-MAD-Timestamp", 20, 27, undefined],
    RequestNonce: ["X-MAD-Nonce", 22, 22, 16],
    RequestContentSHA256: ["X-MAD-Content-SHA256", 43, 43, 32],
    EnrollmentSignature: ["X-MAD-Enrollment-Signature", 86, 86, 64],
  };
  for (const [name, [header, minimum, maximum, decodedBytes]] of Object.entries(exactEnrollmentParameters)) {
    const parameter = spec.components.parameters[name];
    if (!parameter || parameter.name !== header || parameter.in !== "header" || parameter.required !== true ||
        parameter.schema.minLength !== minimum || parameter.schema.maxLength !== maximum ||
        (decodedBytes !== undefined && parameter.schema["x-mad-decoded-bytes"] !== decodedBytes)) {
      throw new Error(`${name}: Enrollment pre-auth header contract drifted`);
    }
  }

  const requiredSchemas = [
    "WebAuthnCreationOptionsV1", "WebAuthnRequestOptionsV1", "WebAuthnRegistrationCredentialV1",
    "WebAuthnAssertionCredentialV1", "DeviceAuthChallengeV1", "DeviceAuthExchangeResultV1",
    "EnrollmentSummaryV1", "EnrollmentTranscriptV1", "ActivationReceiptV1",
    "EnrollmentActivationPackageV1", "SubjectActivationAckV1", "EnrollmentActivateRequestV1",
    "ProfileMutationV1", "ProfileConflictV1", "SyncSnapshotPageV1", "OverviewV1", "UsageWindowV1",
    "CanonicalSessionCommandRequestV1", "SessionCommandDeliveryListResultV1", "SessionCommandClaimRequestV1",
    "SessionCommandAckRequestV1", "SessionCommandResultRequestV1", "SessionCommandReconcileRequestV1",
    "DeviceCommandStateV1", "CommandReservationV1", "KindProofV1", "SessionCommandOutcomeV1",
    "DaemonCommandReceiptV1", "ReservedReceiptV1", "ExecutingReceiptV1", "LocalCommittedReceiptV1",
    "AmbiguousReceiptV1", "CompletedReceiptV1",
    "AuthIdempotencyOperationV1", "AuthOperationReceiptV1", "BrowserSessionRevokeResultV1",
  ];
  for (const name of requiredSchemas) {
    if (!spec.components.schemas[name]) throw new Error(`missing v0.7 schema: ${name}`);
  }

  const digestSchema = spec.components.schemas.Base64UrlDigestV1;
  if (digestSchema?.["x-mad-decoded-bytes"] !== 32 || digestSchema.minLength !== 43 || digestSchema.maxLength !== 43) {
    throw new Error("Base64UrlDigestV1 must encode exactly 32 bytes as 43 unpadded Base64url characters");
  }
  const validateFixedBase64url = (value, location) => {
    if (!value || typeof value !== "object") return;
    if (Number.isSafeInteger(value["x-mad-decoded-bytes"])) {
      const decodedBytes = value["x-mad-decoded-bytes"];
      const encodedLength = Math.ceil((decodedBytes * 8) / 6);
      const allowedPatterns = new Set(["^[A-Za-z0-9_-]+$", `^[A-Za-z0-9_-]{${encodedLength}}$`]);
      if (value.format !== "base64url" || !allowedPatterns.has(value.pattern) || value.minLength !== encodedLength || value.maxLength !== encodedLength) {
        throw new Error(`${location}: fixed Base64url contract does not match ${decodedBytes} decoded bytes`);
      }
    }
    for (const [key, child] of Object.entries(value)) validateFixedBase64url(child, `${location}/${key}`);
  };
  validateFixedBase64url(spec, "OpenAPI");
  const scanTruncatedBase64Fixtures = (value, location) => {
    if (typeof value === "string" && /^A+$/u.test(value) && (value.length === 42 || value.length === 85)) {
      throw new Error(`${location}: truncated unpadded Base64url fixture remains`);
    }
    if (!value || typeof value !== "object") return;
    for (const [key, child] of Object.entries(value)) scanTruncatedBase64Fixtures(child, `${location}/${key}`);
  };
  scanTruncatedBase64Fixtures(spec, "OpenAPI");

  // P2 owns this closed request-schema subset. Keep its fixed raw Base64url
  // spelling consistent with decoded byte counts without widening the check to
  // later-phase schemas that retain separately planned historical contracts.
  const p2FixedBase64url = [
    ["BootstrapAnchorV1", "signingPublicKey", 32],
    ["BootstrapAnchorV1", "exchangePublicKey", 32],
    ["BootstrapAnchorV1", "signingKeyDigest", 32],
    ["BootstrapAnchorV1", "exchangeKeyDigest", 32],
    ["BootstrapAnchorV1", "pinDigest", 32],
    ["BootstrapVerifyRequestV1", "signingProof", 64],
    ["BootstrapVerifyRequestV1", "exchangeProof", 32],
    ["BootstrapCommitReceiptV1", "signingKeyDigest", 32],
    ["BootstrapCommitReceiptV1", "exchangeKeyDigest", 32],
    ["BootstrapCommitReceiptV1", "storageAssertionDigest", 32],
    ["BootstrapCommitReceiptV1", "signingProofDigest", 32],
    ["BootstrapCommitReceiptV1", "exchangeProofDigest", 32],
  ];
  for (const [schemaName, propertyName, decodedBytes] of p2FixedBase64url) {
    const property = spec.components.schemas[schemaName]?.properties?.[propertyName];
    const encodedLength = Math.ceil((decodedBytes * 8) / 6);
    if (property?.["x-mad-decoded-bytes"] !== decodedBytes || property.minLength !== encodedLength || property.maxLength !== encodedLength) {
      throw new Error(`${schemaName}.${propertyName}: P2 fixed Base64url length drifted`);
    }
  }
  const sharedCapabilities = spec.components.schemas.DeviceCapabilityListV1;
  if (sharedCapabilities.minItems !== 0 || sharedCapabilities.maxItems !== 12 || sharedCapabilities.uniqueItems !== true) {
    throw new Error("DeviceCapabilityListV1: shared P3-P6 baseline drifted");
  }
  const p2Capabilities = spec.components.schemas.P2BootstrapDeviceCapabilityListV1;
  if (p2Capabilities.minItems !== 1 || p2Capabilities.maxItems !== 12 || p2Capabilities.uniqueItems !== true) {
    throw new Error("P2BootstrapDeviceCapabilityListV1: nonempty capability contract drifted");
  }
  if (spec.components.schemas.BootstrapAnchorV1.properties.capabilities?.$ref !== "#/components/schemas/P2BootstrapDeviceCapabilityListV1") {
    throw new Error("BootstrapAnchorV1: P2-only capability contract is missing");
  }
  const recoveryCodePattern = spec.components.schemas.RecoveryVerifyRequestV1?.properties?.code?.pattern;
  if (recoveryCodePattern !== "^[Mm][Aa][Dd]-[Rr][Cc]1-(?:[A-Za-z2-7]{4}-){7}[A-Za-z2-7]{4}$") {
    throw new Error("RecoveryVerifyRequestV1: ASCII case-folding contract drifted");
  }
  for (const name of ["PublicKeyCredentialOptions", "CredentialResponse", "EnrollmentActionRequest", "ProfileUpdateRequest", "SessionCommandMutationRequest"]) {
    if (spec.components.schemas[name]) throw new Error(`stale pre-v0.7 schema remains: ${name}`);
  }

  function validateObjects(value, location) {
    if (!value || typeof value !== "object") return;
    if (value.type === "object" && value.additionalProperties === undefined) {
      throw new Error(`${location}: additionalProperties discipline missing`);
    }
    if (value.type === "object" && value.additionalProperties === true) {
      throw new Error(`${location}: unbounded additionalProperties is forbidden`);
    }
    for (const [key, child] of Object.entries(value)) validateObjects(child, `${location}/${key}`);
  }
  for (const [name, schema] of Object.entries(spec.components.schemas)) {
    validateObjects(schema, name);
  }

  const webauthnTuple = spec.components.schemas.WebAuthnCreationPublicKeyV1.properties.pubKeyCredParams;
  if (JSON.stringify(webauthnTuple["x-mad-fixed-algorithm-order"]) !== "[-7,-8,-257]" || webauthnTuple.minItems !== 3 || webauthnTuple.maxItems !== 3) {
    throw new Error("WebAuthn algorithm tuple/order drifted");
  }
  if (Object.keys(spec.components.schemas.WebAuthnExtensionResultsV1.properties).length !== 0) {
    throw new Error("WebAuthn v1 extension object must remain exactly empty");
  }
  if ("enabled" in spec.components.schemas.ProfileCreateRequestV1.properties) {
    throw new Error("browser Profile create must not accept enabled");
  }
  const activationReceipt = spec.components.schemas.ActivationReceiptV1.properties;
  for (const forbidden of ["signingPublicKey", "exchangePublicKey", "attestationSignature", "receiptSignature"]) {
    if (forbidden in activationReceipt) throw new Error(`ActivationReceiptV1 leaked wrapper field ${forbidden}`);
  }
  for (const required of ["transcript", "attestation", "attestationSignature", "receipt", "receiptSignature", "approverKeys"]) {
    if (!(required in spec.components.schemas.EnrollmentActivationPackageV1.properties)) throw new Error(`activation package missing ${required}`);
  }
  const outcomeRefs = spec.components.schemas.SessionCommandOutcomeV1.oneOf.map((entry) => entry.$ref.split("/").at(-1));
  const exactOutcomes = [
    "StartSucceededV1", "StartFailedV1", "StartUnsupportedV1", "ResumeSucceededV1", "ResumeFailedV1",
    "ResumeUnsupportedV1", "StopSucceededV1", "StopFailedV1", "StopUnsupportedV1", "KillSucceededV1",
    "KillFailedV1", "KillUnsupportedV1", "AcquireUnsupportedV1", "ReleaseUnsupportedV1",
  ];
  if (JSON.stringify(outcomeRefs) !== JSON.stringify(exactOutcomes)) throw new Error("P5 terminal outcome union drifted");
  const receiptRefs = spec.components.schemas.DaemonCommandReceiptV1.oneOf.map((entry) => entry.$ref.split("/").at(-1));
  if (JSON.stringify(receiptRefs) !== JSON.stringify(["ReservedReceiptV1", "ExecutingReceiptV1", "LocalCommittedReceiptV1", "AmbiguousReceiptV1", "CompletedReceiptV1"])) {
    throw new Error("P5 receipt state union drifted");
  }
  const serialized = JSON.stringify(spec);
  for (const stale of ["\"execution_ambiguous\"", "\"fake\"", "\"secretRef\"", "\"usedValue\"", "\"limitValue\""]) {
    if (serialized.includes(stale)) throw new Error(`stale or forbidden v0.7 wire token remains: ${stale}`);
  }
  const errorCodes = spec.components.schemas.ApiError.properties.code.enum;
  for (const code of ["snapshot_page_too_large", "cross_server_identity_rebind", "cross_type_signature_replay", "enrollment_preauth_invalid", "delivery_attempts_exhausted", "phase4b_controller_required"]) {
    if (!errorCodes.includes(code)) throw new Error(`stable v0.7 error missing: ${code}`);
  }
  for (const p2Only of ["idempotency_in_progress", "ceremony_restart_required", "session_integrity_invalid", "session_revision_conflict"]) {
    if (errorCodes.includes(p2Only)) throw new Error(`shared ApiError contains P2-only code: ${p2Only}`);
  }
  const sharedDetails = spec.components.schemas.ApiError.properties.details;
  if (sharedDetails.type !== "array" || sharedDetails.oneOf !== undefined || sharedDetails.items?.$ref !== "#/components/schemas/ErrorDetail") {
    throw new Error("shared ApiError details union drifted from the P3-P6 baseline");
  }

  const p2StandardCodes = spec.components.schemas.P2StandardApiErrorV1.properties.code.enum;
  for (const code of ["idempotency_in_progress", "ceremony_restart_required", "session_integrity_invalid"]) {
    if (!p2StandardCodes.includes(code)) throw new Error(`P2 standard error missing: ${code}`);
  }
  for (const special of ["one_time_result_unavailable", "session_revision_conflict"]) {
    if (p2StandardCodes.includes(special)) throw new Error(`P2 special error leaked into standard details union: ${special}`);
  }
  if (spec.components.schemas.P2OneTimeResultUnavailableApiErrorV1.properties.details?.$ref !== "#/components/schemas/AuthReceiptErrorDetailsV1" ||
      spec.components.schemas.P2SessionRevisionConflictApiErrorV1.properties.details?.$ref !== "#/components/schemas/SessionRevisionConflictDetailsV1") {
    throw new Error("P2 code-discriminated error details drifted");
  }

  const emptyBodyRef = "#/components/schemas/P2EmptyObjectRequestV1";
  const emptyBodySchema = spec.components.schemas.P2EmptyObjectRequestV1;
  if (emptyBodySchema.type !== "object" || emptyBodySchema.additionalProperties !== false ||
      Object.keys(emptyBodySchema.properties ?? {}).length !== 0 || emptyBodySchema.required !== undefined ||
      emptyBodySchema["x-go-type"] !== "transport.EmptyJSONObjectV1" ||
      JSON.stringify(emptyBodySchema["x-go-type-import"]) !== JSON.stringify({
        path: "github.com/jinlong17/multi-agent-desk/internal/transport",
        name: "transport",
      })) {
    throw new Error("P2EmptyObjectRequestV1 no longer describes exactly {}");
  }
  const expectedP2Headers = {
    P2IssuedSessionCookieV1: {
      description: "Issues the host-only P2 browser session cookie. Domain is forbidden.",
      schema: {
        type: "string",
        pattern: "^__Host-mad_session=[A-Za-z0-9_-]{43}; Path=/; Expires=[A-Z][a-z]{2}, [0-9]{2} [A-Z][a-z]{2} [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} GMT; HttpOnly; Secure; SameSite=Strict$",
      },
      example: "__Host-mad_session=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA; Path=/; Expires=Fri, 01 Mar 2030 12:00:00 GMT; HttpOnly; Secure; SameSite=Strict",
    },
    P2ClearedSessionCookieV1: {
      description: "Clears the host-only P2 browser session cookie. Domain is forbidden.",
      schema: {
        type: "string",
        enum: ["__Host-mad_session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Max-Age=0; HttpOnly; Secure; SameSite=Strict"],
      },
      example: "__Host-mad_session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:01 GMT; Max-Age=0; HttpOnly; Secure; SameSite=Strict",
    },
    P2RetryAfterV1: {
      description: "Present with the exact value 1 only for idempotency_in_progress.",
      schema: { type: "string", enum: ["1"], default: "1" },
      example: "1",
    },
  };
  for (const [name, expected] of Object.entries(expectedP2Headers)) {
    if (JSON.stringify(spec.components.headers[name]) !== JSON.stringify(expected)) {
      throw new Error(`${name}: P2 header component semantics drifted`);
    }
  }
  const standardErrorRef = "#/components/schemas/P2StandardErrorEnvelopeV1";
  for (const operationId of p2Operations) {
    const operation = operationsById.get(operationId);
    const body = operation.requestBody;
    if (p2EmptyBodyOperations.has(operationId)) {
      if (body?.required !== true || body.content?.["application/json"]?.schema?.$ref !== emptyBodyRef) {
        throw new Error(`${operationId}: exact required empty-object request body is missing`);
      }
    } else if (p2IdempotentOperations.has(operationId)) {
      const expectedBodyRef = p2NamedBodyOperations.get(operationId);
      if (!expectedBodyRef || body?.required !== true || body.content?.["application/json"]?.schema?.$ref !== expectedBodyRef || expectedBodyRef === emptyBodyRef) {
        throw new Error(`${operationId}: required named P2 request body drifted`);
      }
    }
    const success = Object.entries(operation.responses).find(([status]) => /^2/.test(status))[1];
    const setCookieRef = success.headers?.["Set-Cookie"]?.$ref;
    const expectedCookieRef = p2IssuedCookieOperations.has(operationId)
      ? "#/components/headers/P2IssuedSessionCookieV1"
      : p2ClearedCookieOperations.has(operationId)
        ? "#/components/headers/P2ClearedSessionCookieV1"
        : undefined;
    if (setCookieRef !== expectedCookieRef) throw new Error(`${operationId}: P2 Set-Cookie response contract drifted`);
    for (const [status, response] of Object.entries(operation.responses)) {
      const actualCookieRef = response.headers?.["Set-Cookie"]?.$ref;
      const actualRetryRef = response.headers?.["Retry-After"]?.$ref;
      if (/^2/.test(status)) {
        if (actualCookieRef !== expectedCookieRef) throw new Error(`${operationId}: P2 ${status} Set-Cookie placement drifted`);
        if (actualRetryRef !== undefined) throw new Error(`${operationId}: P2 ${status} unexpectedly declares Retry-After`);
        continue;
      }
      if (actualCookieRef !== undefined) throw new Error(`${operationId}: P2 ${status} unexpectedly declares Set-Cookie`);
      const expectedRetryRef = status === "409" && p2IdempotentOperations.has(operationId)
        ? "#/components/headers/P2RetryAfterV1"
        : undefined;
      if (actualRetryRef !== expectedRetryRef) throw new Error(`${operationId}: P2 ${status} Retry-After placement drifted`);
      const actual = response.content?.["application/json"]?.schema?.$ref;
      const expected = operationId === "deleteBrowserSession" && status === "412"
        ? "#/components/schemas/P2SessionRevisionConflictErrorEnvelopeV1"
        : p2SecretOperations.has(operationId) && status === "409"
          ? "#/components/schemas/P2SecretConflictErrorEnvelopeV1"
          : standardErrorRef;
      if (actual !== expected) throw new Error(`${operationId}: P2 ${status} error schema drifted`);
    }
  }

  // P2 must be additive: the complete non-P2 path surface and every component
  // transitively reachable from it stay byte-semantically locked to 4fa86ff.
  const nonP2Paths = Object.fromEntries(Object.entries(spec.paths).filter(([, pathItem]) =>
    !Object.values(pathItem).some((operation) => p2Operations.has(operation.operationId))));
  const pendingRefs = [];
  const scanRefs = (value) => {
    if (Array.isArray(value)) {
      for (const child of value) scanRefs(child);
    } else if (value && typeof value === "object") {
      if (typeof value.$ref === "string" && value.$ref.startsWith("#/components/")) pendingRefs.push(value.$ref);
      for (const child of Object.values(value)) scanRefs(child);
    }
  };
  scanRefs(nonP2Paths);
  const reachable = {};
  while (pendingRefs.length) {
    const ref = pendingRefs.pop();
    if (reachable[ref]) continue;
    const [, , category, name] = ref.split("/");
    const component = spec.components[category]?.[name];
    if (!component) throw new Error(`missing transitive component: ${ref}`);
    reachable[ref] = component;
    scanRefs(component);
  }
  const canonicalize = (value) => Array.isArray(value)
    ? value.map(canonicalize)
    : value && typeof value === "object"
      ? Object.fromEntries(Object.keys(value).sort().map((key) => [key, canonicalize(value[key])]))
      : value;
  const p3P6Digest = createHash("sha256").update(JSON.stringify(canonicalize({ paths: nonP2Paths, components: reachable }))).digest("hex");
  if (p3P6Digest !== "8eb65ed3fcbbec590aa2c365f5297843bca8bbeaca899a37daf45b8b2ef64420") {
    throw new Error(`P3-P6 transitive contract drifted: ${p3P6Digest}`);
  }

  const workspace = readFileSync(join(root, "pnpm-workspace.yaml"), "utf8");
  if (!workspace.includes("'js-yaml@4.2.0>argparse': '-'") || /argparse@|Python-2\.0/.test(readFileSync(join(root, "pnpm-lock.yaml"), "utf8"))) {
    throw new Error("license-safe js-yaml CLI-edge override is missing or stale");
  }
  const goMod = readFileSync(join(root, "go.mod"), "utf8");
  for (const pin of ["github.com/oapi-codegen/oapi-codegen/v2 v2.8.0", "github.com/getkin/kin-openapi v0.142.0", "github.com/oapi-codegen/runtime v1.6.0", "github.com/google/uuid v1.6.0"]) {
    if (!goMod.includes(pin)) throw new Error(`required Go contract/tool pin missing: ${pin}`);
  }
  const lock = readFileSync(join(root, "pnpm-lock.yaml"), "utf8");
  if (!lock.includes("openapi-typescript@7.13.0") || lock.includes("openapi-fetch")) {
    throw new Error("TypeScript generator graph drifted or openapi-fetch was introduced");
  }
}

function generate(goPath, tsPath) {
  run(directInvocation("go", ["tool", "oapi-codegen", "-config", configPath, "-o", goPath, specPath]));
  run(nodeCLIInvocation(openapiTypescriptCLI, [specPath, "--output", tsPath]));
  validateGeneratedGo(goPath);
}

function validateGeneratedGo(path) {
  const source = readFileSync(path, "utf8");
  if (!source.includes('"github.com/jinlong17/multi-agent-desk/internal/transport"') ||
      !source.includes("type P2EmptyObjectRequestV1 = transport.EmptyJSONObjectV1") ||
      source.includes("type P2EmptyObjectRequestV1 = map[string]interface{}")) {
    throw new Error("generated Go strict empty-object transport alias drifted");
  }
}

validateContract();
if (mode === "generate") {
  generate(goOutput, tsOutput);
  console.log("generated deterministic OpenAPI artifacts");
} else if (mode === "verify") {
  validateGeneratedGo(goOutput);
  const temporary = mkdtempSync(join(tmpdir(), "mad-api-v1-"));
  try {
    const firstGo = join(temporary, "first.go");
    const firstTS = join(temporary, "first.ts");
    const secondGo = join(temporary, "second.go");
    const secondTS = join(temporary, "second.ts");
    generate(firstGo, firstTS);
    generate(secondGo, secondTS);
    for (const [expected, actual, label] of [
      [goOutput, firstGo, "checked-in Go"], [tsOutput, firstTS, "checked-in TypeScript"],
      [firstGo, secondGo, "Go determinism"], [firstTS, secondTS, "TypeScript determinism"],
    ]) {
      if (!readFileSync(expected).equals(readFileSync(actual))) throw new Error(`${label} drift detected`);
    }
    console.log("verified OpenAPI contract and deterministic generated artifacts");
  } finally {
    rmSync(temporary, { recursive: true, force: true });
  }
} else {
  throw new Error(`unknown mode: ${mode}`);
}
