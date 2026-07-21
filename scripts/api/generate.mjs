import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dirname, "../..");
const specPath = join(root, "api/openapi/control-plane-v1.yaml");
const configPath = join(root, "api/openapi/oapi-codegen-v1.yaml");
const goOutput = join(root, "internal/controlplane/api/generated/control_plane_v1.gen.go");
const tsOutput = join(root, "packages/protocol/src/generated/control-plane-v1.ts");
const mode = process.argv[2] ?? "generate";
const env = { ...process.env, LC_ALL: "C", LANG: "C", TZ: "UTC" };

function run(command, args) {
  execFileSync(command, args, { cwd: root, env, stdio: "inherit" });
}

function validateContract() {
  const spec = JSON.parse(readFileSync(specPath, "utf8"));
  if (spec.openapi !== "3.0.3") throw new Error("OpenAPI authority must remain 3.0.3");
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
      if (method !== "get" && !operation.parameters.some((entry) => entry.$ref?.endsWith("/IdempotencyKey"))) {
        throw new Error(`${operation.operationId}: Idempotency-Key missing`);
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
  ];
  for (const name of requiredSchemas) {
    if (!spec.components.schemas[name]) throw new Error(`missing v0.7 schema: ${name}`);
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
  run("go", ["tool", "oapi-codegen", "-config", configPath, "-o", goPath, specPath]);
  run("pnpm", ["exec", "openapi-typescript", specPath, "--output", tsPath]);
}

validateContract();
if (mode === "generate") {
  generate(goOutput, tsOutput);
  console.log("generated deterministic OpenAPI artifacts");
} else if (mode === "verify") {
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
