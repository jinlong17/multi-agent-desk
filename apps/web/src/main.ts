import type { components } from "@multi-agent-desk/protocol";
import { ControlPlaneError } from "@multi-agent-desk/protocol";
import { controlPlane } from "./api/control-plane.js";
import {
  decodeBase64URL,
  serializeAssertionCredential,
  serializeRegistrationCredential,
  toCreationOptions,
  toRequestOptions,
} from "./webauthn.js";
import "./styles.css";

type CurrentAuth = components["schemas"]["CurrentAuth"];
type BootstrapAnchor = components["schemas"]["BootstrapAnchorV1"];
type BootstrapDescriptor = components["schemas"]["BootstrapAnchorDescriptorV1"];
type BootstrapChallenge = components["schemas"]["BootstrapAnchorChallengeV1"];
type BootstrapReceipt = components["schemas"]["BootstrapCommitReceiptV1"];
type RecoveryCodes = components["schemas"]["RecoveryCodesResultV1"];
type Passkey = components["schemas"]["PasskeyV1"];
type BrowserSession = components["schemas"]["BrowserSessionV1"];

interface BootstrapProof {
  version: 1;
  ceremonyId: string;
  serverOrigin: string;
  anchorDeviceId: string;
  signingProof: string;
  exchangeProof: string;
}

const uuidV7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/u;
const deviceCapabilities = new Set([
  "mad.v1.metadata.read",
  "mad.v1.metadata.write",
  "mad.v1.sync.pull",
  "mad.v1.sync.push",
  "mad.v1.presence.write",
  "mad.v1.device.enroll_request",
  "mad.v1.device.enroll_approve",
  "mad.v1.device.revoke",
  "mad.v1.session.command_create",
  "mad.v1.session.command_claim",
  "mad.v1.session.command_ack",
  "mad.v1.session.command_result",
]);

let currentAuth: CurrentAuth | undefined;
let descriptor: BootstrapDescriptor | undefined;
let challenge: BootstrapChallenge | undefined;
let receipt: BootstrapReceipt | undefined;
let recoveryCodes: RecoveryCodes | undefined;
let passkeys: Passkey[] = [];
let sessions: BrowserSession[] = [];

function element<T extends HTMLElement>(id: string): T {
  const value = document.getElementById(id);
  if (!value) throw new Error(`required element #${id} is missing`);
  return value as T;
}

const live = element<HTMLOutputElement>("status-live");
const errorOutput = element<HTMLOutputElement>("error-live");
const bootstrapState = element<HTMLOutputElement>("bootstrap-state");
const authState = element<HTMLOutputElement>("auth-state");
const anchorSummary = element<HTMLElement>("anchor-summary");
const challengeSummary = element<HTMLElement>("challenge-summary");
const recoveryPanel = element<HTMLElement>("recovery-result");
const recoveryList = element<HTMLOListElement>("recovery-list");
const receiptSummary = element<HTMLElement>("receipt-summary");
const authenticatedPanel = element<HTMLElement>("authenticated-panel");
const normalSessionPanel = element<HTMLElement>("normal-session-panel");
const recoverySessionPanel = element<HTMLElement>("recovery-session-panel");
const passkeyList = element<HTMLUListElement>("passkey-list");
const sessionList = element<HTMLUListElement>("session-list");

function setStatus(message: string): void {
  live.value = message;
  errorOutput.value = "";
}

function describeError(error: unknown): string {
  if (error instanceof ControlPlaneError) {
    const request = error.requestId ? ` Request ID: ${error.requestId}.` : "";
    return `${error.code}: ${error.message}.${request}`;
  }
  if (error instanceof DOMException) return `${error.name}: ${error.message}`;
  if (error instanceof Error) return error.message;
  return "The operation failed.";
}

function setError(error: unknown): void {
  live.value = "";
  errorOutput.value = describeError(error);
}

async function run(button: HTMLButtonElement, task: () => Promise<void>): Promise<void> {
  if (button.disabled) return;
  button.disabled = true;
  button.setAttribute("aria-busy", "true");
  errorOutput.value = "";
  try {
    await task();
  } catch (error) {
    setError(error);
  } finally {
    button.disabled = false;
    button.removeAttribute("aria-busy");
  }
}

function bind(id: string, task: (button: HTMLButtonElement) => Promise<void>): void {
  const button = element<HTMLButtonElement>(id);
  button.addEventListener("click", () => void run(button, () => task(button)));
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function exactObject(value: unknown, keys: readonly string[], label: string): Record<string, unknown> {
  if (!isObject(value)) throw new Error(`${label} must be a JSON object`);
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  if (actual.length !== expected.length || actual.some((key, index) => key !== expected[index])) {
    throw new Error(`${label} has missing or unknown fields`);
  }
  return value;
}

function requiredString(value: unknown, label: string, maximum = 256): string {
  if (typeof value !== "string" || value.length === 0 || value.length > maximum) {
    throw new Error(`${label} is invalid`);
  }
  return value;
}

function exactInteger(value: unknown, expected: number, label: string): number {
  if (value !== expected) throw new Error(`${label} must be ${expected}`);
  return expected;
}

function positiveInteger(value: unknown, label: string): number {
  if (!Number.isSafeInteger(value) || (value as number) < 1) throw new Error(`${label} is invalid`);
  return value as number;
}

function fixedBase64URL(value: unknown, size: number, label: string): string {
  const encoded = requiredString(value, label, 512);
  if (decodeBase64URL(encoded).byteLength !== size) throw new Error(`${label} has the wrong byte length`);
  return encoded;
}

function requiredUUIDv7(value: unknown, label: string): string {
  const id = requiredString(value, label, 36);
  if (!uuidV7.test(id)) throw new Error(`${label} is not a canonical UUIDv7`);
  return id;
}

function requiredUTCDate(value: unknown, label: string): string {
  const text = requiredString(value, label, 64);
  if (!text.endsWith("Z") || Number.isNaN(Date.parse(text))) throw new Error(`${label} is not a UTC date-time`);
  return text;
}

function parseAnchor(input: unknown): BootstrapAnchor {
  const value = exactObject(input, [
    "deviceId", "kind", "name", "platform", "architecture", "clientVersion",
    "signingPublicKey", "exchangePublicKey", "signingKeyDigest", "exchangeKeyDigest",
    "pinDigest", "storageMode", "keyEnvelopeAssertion", "capabilities",
  ], "anchor");
  const assertion = exactObject(value.keyEnvelopeAssertion, [
    "formatVersion", "keyRevision", "recordRevision", "status", "sealedAt",
  ], "anchor.keyEnvelopeAssertion");
  exactInteger(assertion.formatVersion, 1, "anchor.keyEnvelopeAssertion.formatVersion");
  exactInteger(assertion.keyRevision, 1, "anchor.keyEnvelopeAssertion.keyRevision");
  positiveInteger(assertion.recordRevision, "anchor.keyEnvelopeAssertion.recordRevision");
  if (assertion.status !== "pending") throw new Error("anchor.keyEnvelopeAssertion.status must be pending");
  requiredUTCDate(assertion.sealedAt, "anchor.keyEnvelopeAssertion.sealedAt");

  requiredUUIDv7(value.deviceId, "anchor.deviceId");
  if (value.kind !== "daemon") throw new Error("The bootstrap anchor must be a Daemon");
  requiredString(value.name, "anchor.name", 128);
  if (value.platform !== "darwin" && value.platform !== "linux" && value.platform !== "windows") {
    throw new Error("anchor.platform is invalid");
  }
  requiredString(value.architecture, "anchor.architecture", 32);
  requiredString(value.clientVersion, "anchor.clientVersion", 64);
  fixedBase64URL(value.signingPublicKey, 32, "anchor.signingPublicKey");
  fixedBase64URL(value.exchangePublicKey, 32, "anchor.exchangePublicKey");
  fixedBase64URL(value.signingKeyDigest, 32, "anchor.signingKeyDigest");
  fixedBase64URL(value.exchangeKeyDigest, 32, "anchor.exchangeKeyDigest");
  fixedBase64URL(value.pinDigest, 32, "anchor.pinDigest");
  if (value.storageMode !== "portable_vault_v1") throw new Error("anchor.storageMode must be portable_vault_v1");
  if (!Array.isArray(value.capabilities) || value.capabilities.length === 0 || value.capabilities.length > 32) {
    throw new Error("anchor.capabilities is invalid");
  }
  const capabilities = value.capabilities.map((capability) => requiredString(capability, "anchor capability", 64));
  if (capabilities.some((capability) => !deviceCapabilities.has(capability))) throw new Error("anchor.capabilities contains an unknown capability");
  if (capabilities.some((capability, index) => index > 0 && capability <= capabilities[index - 1])) {
    throw new Error("anchor.capabilities must be sorted and unique");
  }
  return value as unknown as BootstrapAnchor;
}

function parseDescriptor(text: string): BootstrapDescriptor {
  if (new TextEncoder().encode(text).byteLength > 64 * 1024) throw new Error("Bootstrap descriptor exceeds 64 KiB");
  const value = exactObject(JSON.parse(text) as unknown, ["version", "serverOrigin", "anchor"], "descriptor");
  exactInteger(value.version, 1, "descriptor.version");
  if (requiredString(value.serverOrigin, "descriptor.serverOrigin", 2048) !== window.location.origin) {
    throw new Error("Bootstrap descriptor origin does not match this page");
  }
  return { version: 1, serverOrigin: value.serverOrigin as string, anchor: parseAnchor(value.anchor) };
}

function parseProof(text: string): BootstrapProof {
  if (new TextEncoder().encode(text).byteLength > 4096) throw new Error("Bootstrap proof exceeds 4 KiB");
  const value = exactObject(JSON.parse(text) as unknown, [
    "version", "ceremonyId", "serverOrigin", "anchorDeviceId", "signingProof", "exchangeProof",
  ], "proof");
  exactInteger(value.version, 1, "proof.version");
  const proof: BootstrapProof = {
    version: 1,
    ceremonyId: requiredUUIDv7(value.ceremonyId, "proof.ceremonyId"),
    serverOrigin: requiredString(value.serverOrigin, "proof.serverOrigin", 2048),
    anchorDeviceId: requiredUUIDv7(value.anchorDeviceId, "proof.anchorDeviceId"),
    signingProof: fixedBase64URL(value.signingProof, 64, "proof.signingProof"),
    exchangeProof: fixedBase64URL(value.exchangeProof, 32, "proof.exchangeProof"),
  };
  if (!challenge || proof.ceremonyId !== challenge.ceremonyId || proof.serverOrigin !== challenge.serverOrigin || proof.anchorDeviceId !== challenge.anchor.deviceId) {
    throw new Error("Daemon proof does not match the in-memory bootstrap challenge");
  }
  return proof;
}

async function fileText(input: HTMLInputElement, maximumBytes: number): Promise<string> {
  const file = input.files?.item(0);
  if (!file) throw new Error("Choose a JSON file first");
  if (file.size > maximumBytes) throw new Error("The selected file is too large");
  return file.text();
}

function idempotencyKey(): string {
  return crypto.randomUUID();
}

function formatDate(value: string | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString();
}

function clearChildren(node: Element): void {
  while (node.firstChild) node.firstChild.remove();
}

function appendDefinition(list: HTMLElement, term: string, detail: string): void {
  const dt = document.createElement("dt");
  const dd = document.createElement("dd");
  dt.textContent = term;
  dd.textContent = detail;
  list.append(dt, dd);
}

function renderAnchor(): void {
  clearChildren(anchorSummary);
  if (!descriptor) {
    anchorSummary.textContent = "No validated Daemon anchor loaded.";
    return;
  }
  const list = document.createElement("dl");
  appendDefinition(list, "Daemon", descriptor.anchor.name);
  appendDefinition(list, "Device ID", descriptor.anchor.deviceId);
  appendDefinition(list, "Platform", `${descriptor.anchor.platform}/${descriptor.anchor.architecture}`);
  appendDefinition(list, "Pin digest", descriptor.anchor.pinDigest);
  appendDefinition(list, "Storage assertion", `portable_vault_v1 · revision ${descriptor.anchor.keyEnvelopeAssertion.recordRevision}`);
  anchorSummary.append(list);
}

function renderChallenge(): void {
  clearChildren(challengeSummary);
  const download = element<HTMLButtonElement>("download-challenge");
  download.disabled = !challenge;
  if (!challenge) {
    challengeSummary.textContent = "No in-memory challenge.";
    return;
  }
  const list = document.createElement("dl");
  appendDefinition(list, "Ceremony", challenge.ceremonyId);
  appendDefinition(list, "Expires", formatDate(challenge.expiresAt));
  appendDefinition(list, "Storage digest", challenge.storageAssertionDigest);
  appendDefinition(list, "Server ephemeral key", challenge.serverEphemeralExchangePublicKey);
  challengeSummary.append(list);
}

function setAuth(value: CurrentAuth | undefined): void {
  currentAuth = value;
  controlPlane.setCsrfToken(value?.csrfToken);
  authenticatedPanel.hidden = !value;
  normalSessionPanel.hidden = value?.authenticationMethod !== "passkey";
  recoverySessionPanel.hidden = value?.authenticationMethod !== "recovery";
  if (!value) {
    authState.value = "Signed out";
    passkeys = [];
    sessions = [];
    renderPasskeys();
    renderSessions();
    return;
  }
  authState.value = `${value.authenticationMethod === "recovery" ? "Restricted recovery" : "Passkey"} session · expires ${formatDate(value.expiresAt)}`;
}

function showRecoveryCodes(value: RecoveryCodes, source: string): void {
  recoveryCodes = value;
  clearChildren(recoveryList);
  for (const code of value.codes) {
    const item = document.createElement("li");
    item.textContent = code;
    recoveryList.append(item);
  }
  element<HTMLInputElement>("recovery-saved").checked = false;
  recoveryPanel.hidden = false;
  recoveryPanel.querySelector<HTMLElement>("h3")!.textContent = `${source}: save these codes now`;
  recoveryPanel.focus();
}

function clearRecoveryCodes(): void {
  recoveryCodes = undefined;
  clearChildren(recoveryList);
  recoveryPanel.hidden = true;
  element<HTMLInputElement>("recovery-saved").checked = false;
}

function renderReceipt(): void {
  clearChildren(receiptSummary);
  element<HTMLButtonElement>("download-receipt").disabled = !receipt;
  if (!receipt) {
    receiptSummary.textContent = "No bootstrap receipt.";
    return;
  }
  const list = document.createElement("dl");
  appendDefinition(list, "Anchor", receipt.anchorDeviceId);
  appendDefinition(list, "Ceremony", receipt.ceremonyId);
  appendDefinition(list, "Activated", formatDate(receipt.activatedAt));
  appendDefinition(list, "Signing proof digest", receipt.signingProofDigest);
  appendDefinition(list, "Exchange proof digest", receipt.exchangeProofDigest);
  receiptSummary.append(list);
}

function renderPasskeys(): void {
  clearChildren(passkeyList);
  if (passkeys.length === 0) {
    const item = document.createElement("li");
    item.className = "empty-row";
    item.textContent = "No Passkeys loaded.";
    passkeyList.append(item);
    return;
  }
  for (const passkey of passkeys) {
    const item = document.createElement("li");
    item.className = "resource-row";
    const detail = document.createElement("div");
    const name = document.createElement("strong");
    const metadata = document.createElement("span");
    name.textContent = `${passkey.name}${passkey.current ? " · current" : ""}`;
    metadata.textContent = `Created ${formatDate(passkey.createdAt)} · revision ${passkey.credentialRevision}`;
    detail.append(name, metadata);
    const remove = document.createElement("button");
    remove.type = "button";
    remove.className = "danger secondary";
    remove.textContent = "Delete";
    remove.addEventListener("click", () => void run(remove, async () => {
      if (!window.confirm(`Delete Passkey “${passkey.name}”? Recent user verification is required.`)) return;
      const result = await controlPlane.methods.deletePasskey({
        path: { passkeyId: passkey.id },
        ifMatch: `"rev-${passkey.credentialRevision}"`,
        idempotencyKey: idempotencyKey(),
        body: {},
      });
      setStatus(`Passkey deleted; ${result.data.revokedSessionCount} browser session(s) revoked.`);
      if (result.data.currentSessionRevoked) setAuth(undefined);
      else await loadPasskeys();
    }));
    item.append(detail, remove);
    passkeyList.append(item);
  }
}

function renderSessions(): void {
  clearChildren(sessionList);
  if (sessions.length === 0) {
    const item = document.createElement("li");
    item.className = "empty-row";
    item.textContent = "No browser sessions loaded.";
    sessionList.append(item);
    return;
  }
  for (const session of sessions) {
    const item = document.createElement("li");
    item.className = "resource-row";
    const detail = document.createElement("div");
    const name = document.createElement("strong");
    const metadata = document.createElement("span");
    name.textContent = `${session.authenticationMethod} session${session.current ? " · current" : ""}`;
    metadata.textContent = `Last seen ${formatDate(session.lastSeenAt)} · expires ${formatDate(session.expiresAt)} · state revision ${session.revision} · activity revision ${session.activityRevision}`;
    detail.append(name, metadata);
    const disabled = document.createElement("button");
    disabled.type = "button";
    disabled.className = "danger secondary";
    disabled.textContent = "Revoke after P2 verification";
    disabled.disabled = true;
    disabled.title = "The per-item flow is implemented and tested; P6 enables this control only after P2 is independently verified.";
    item.append(detail, disabled);
    sessionList.append(item);
  }
}

async function loadBootstrapStatus(): Promise<void> {
  const result = await controlPlane.methods.getBootstrapStatus();
  const suffix = result.data.expiresAt ? ` · expires ${formatDate(result.data.expiresAt)}` : "";
  bootstrapState.value = `${result.data.state}${suffix}`;
}

async function loadCurrentAuth(): Promise<void> {
  try {
    const result = await controlPlane.methods.getCurrentAuth();
    setAuth(result.data);
  } catch (error) {
    if (error instanceof ControlPlaneError && (error.code === "unauthenticated" || error.code === "session_expired")) {
      setAuth(undefined);
      return;
    }
    throw error;
  }
}

async function loadPasskeys(): Promise<void> {
  if (currentAuth?.authenticationMethod !== "passkey") throw new Error("Passkey listing requires a normal Passkey session");
  const result = await controlPlane.methods.listPasskeys();
  passkeys = result.data.passkeys;
  renderPasskeys();
}

async function loadSessions(): Promise<void> {
  if (currentAuth?.authenticationMethod !== "passkey") throw new Error("Session listing requires a normal Passkey session");
  const result = await controlPlane.methods.listBrowserSessions();
  sessions = result.data.sessions;
  renderSessions();
}

async function createPublicKey(options: PublicKeyCredentialCreationOptions): Promise<PublicKeyCredential> {
  if (!window.isSecureContext || !navigator.credentials) throw new Error("Passkeys require a secure browser context");
  const result = await navigator.credentials.create({ publicKey: options });
  if (!(result instanceof PublicKeyCredential)) throw new Error("Passkey registration was cancelled or returned no credential");
  return result;
}

async function getPublicKey(options: PublicKeyCredentialRequestOptions): Promise<PublicKeyCredential> {
  if (!window.isSecureContext || !navigator.credentials) throw new Error("Passkeys require a secure browser context");
  const result = await navigator.credentials.get({ publicKey: options });
  if (!(result instanceof PublicKeyCredential)) throw new Error("Passkey verification was cancelled or returned no credential");
  return result;
}

function downloadJSON(filename: string, value: unknown): void {
  const blob = new Blob([`${JSON.stringify(value, null, 2)}\n`], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

bind("refresh-bootstrap", async () => {
  await loadBootstrapStatus();
  setStatus("Bootstrap state refreshed.");
});

bind("load-descriptor", async () => {
  descriptor = parseDescriptor(element<HTMLTextAreaElement>("descriptor-json").value);
  renderAnchor();
  setStatus("Daemon anchor descriptor validated in memory.");
});

element<HTMLInputElement>("descriptor-file").addEventListener("change", async (event) => {
  const input = event.currentTarget as HTMLInputElement;
  try {
    const text = await fileText(input, 64 * 1024);
    element<HTMLTextAreaElement>("descriptor-json").value = text;
    descriptor = parseDescriptor(text);
    renderAnchor();
    setStatus("Daemon anchor descriptor file validated in memory.");
  } catch (error) {
    setError(error);
  } finally {
    input.value = "";
  }
});

bind("begin-bootstrap", async () => {
  if (!descriptor) throw new Error("Load and validate a Daemon anchor descriptor first");
  const displayName = element<HTMLInputElement>("bootstrap-display-name").value.trim();
  const tokenInput = element<HTMLInputElement>("bootstrap-token");
  const token = tokenInput.value;
  if (displayName.length === 0 || displayName.length > 128) throw new Error("Display name must be 1–128 characters");
  if (token.length === 0 || token.length > 4096) throw new Error("Bootstrap token is invalid");
  const result = await controlPlane.methods.createBootstrapOptions({
    bootstrapToken: token,
    idempotencyKey: idempotencyKey(),
    body: { displayName, anchor: descriptor.anchor },
  });
  challenge = result.data;
  tokenInput.value = "";
  renderChallenge();
  await loadBootstrapStatus();
  setStatus("Bootstrap challenge created. Download it, prove it with the same Daemon, then import the proof.");
});

bind("download-challenge", async () => {
  if (!challenge) throw new Error("No in-memory bootstrap challenge is available");
  downloadJSON(`multidesk-bootstrap-challenge-${challenge.ceremonyId}.json`, challenge);
  setStatus("Bootstrap challenge downloaded. It remains only in page memory.");
});

element<HTMLInputElement>("proof-file").addEventListener("change", async (event) => {
  const input = event.currentTarget as HTMLInputElement;
  try {
    const text = await fileText(input, 4096);
    element<HTMLTextAreaElement>("proof-json").value = text;
    parseProof(text);
    setStatus("Daemon proof file matches the in-memory challenge.");
  } catch (error) {
    setError(error);
  } finally {
    input.value = "";
  }
});

bind("finish-bootstrap", async () => {
  if (!challenge) throw new Error("Create a bootstrap challenge first");
  const proof = parseProof(element<HTMLTextAreaElement>("proof-json").value);
  const tokenInput = element<HTMLInputElement>("bootstrap-token-verify");
  const token = tokenInput.value;
  if (token.length === 0 || token.length > 4096) throw new Error("Enter the current bootstrap token again");
  const credential = await createPublicKey(toCreationOptions(challenge.passkeyCreationOptions));
  const result = await controlPlane.methods.verifyBootstrap({
    bootstrapToken: token,
    idempotencyKey: idempotencyKey(),
    body: {
      ceremonyId: challenge.ceremonyId,
      credential: serializeRegistrationCredential(credential),
      signingProof: proof.signingProof,
      exchangeProof: proof.exchangeProof,
    },
  });
  tokenInput.value = "";
  element<HTMLTextAreaElement>("proof-json").value = "";
  descriptor = undefined;
  challenge = undefined;
  renderAnchor();
  renderChallenge();
  setAuth(result.data.currentAuth);
  receipt = result.data.receipt;
  renderReceipt();
  showRecoveryCodes(result.data.recoveryCodes, "Bootstrap complete");
  await loadBootstrapStatus();
  setStatus("Bootstrap committed atomically. Save recovery codes and activate the downloaded receipt on the Daemon.");
});

bind("download-receipt", async () => {
  if (!receipt) throw new Error("No bootstrap receipt is available");
  downloadJSON(`multidesk-bootstrap-receipt-${receipt.ceremonyId}.json`, receipt);
  setStatus("Bootstrap receipt downloaded for Daemon activation.");
});

bind("passkey-login", async () => {
  const options = await controlPlane.methods.createPasskeyAuthenticationOptions({ idempotencyKey: idempotencyKey(), body: {} });
  const credential = await getPublicKey(toRequestOptions(options.data));
  const result = await controlPlane.methods.verifyPasskeyAuthentication({
    idempotencyKey: idempotencyKey(),
    body: { ceremonyId: options.data.ceremonyId, credential: serializeAssertionCredential(credential) },
  });
  setAuth(result.data);
  setStatus("Signed in with Passkey. The CSRF value is held only in page memory.");
});

bind("recovery-login", async () => {
  const input = element<HTMLInputElement>("recovery-code");
  const code = input.value;
  input.value = "";
  const result = await controlPlane.methods.verifyRecoveryCode({
    idempotencyKey: idempotencyKey(),
    body: { code },
  });
  setAuth(result.data);
  setStatus("Restricted recovery session started. Register one replacement Passkey to return to a normal session.");
});

async function registerPasskey(): Promise<void> {
  if (!currentAuth) throw new Error("Authentication is required");
  const options = await controlPlane.methods.createPasskeyRegistrationOptions({ idempotencyKey: idempotencyKey(), body: {} });
  const credential = await createPublicKey(toCreationOptions(options.data));
  const result = await controlPlane.methods.verifyPasskeyRegistration({
    idempotencyKey: idempotencyKey(),
    body: { ceremonyId: options.data.ceremonyId, credential: serializeRegistrationCredential(credential) },
  });
  const wasRecovery = currentAuth.authenticationMethod === "recovery";
  setAuth(result.data);
  if (wasRecovery) setStatus("Replacement Passkey registered. Recovery ended and all older browser sessions were revoked.");
  else setStatus("Passkey registered and the browser session was rotated.");
}

bind("register-passkey", registerPasskey);
bind("register-replacement-passkey", registerPasskey);

bind("uv-step-up", async () => {
  if (currentAuth?.authenticationMethod !== "passkey") throw new Error("A normal Passkey session is required");
  const options = await controlPlane.methods.createUvOptions({ idempotencyKey: idempotencyKey(), body: {} });
  const credential = await getPublicKey(toRequestOptions(options.data));
  const result = await controlPlane.methods.verifyUv({
    idempotencyKey: idempotencyKey(),
    body: { ceremonyId: options.data.ceremonyId, credential: serializeAssertionCredential(credential) },
  });
  setAuth(result.data);
  setStatus("Recent user verification completed; session and CSRF were rotated.");
});

bind("rotate-recovery", async () => {
  if (currentAuth?.authenticationMethod !== "passkey") throw new Error("A normal Passkey session is required");
  const result = await controlPlane.methods.rotateRecoveryCodes({ idempotencyKey: idempotencyKey(), body: {} });
  showRecoveryCodes(result.data, "Recovery codes rotated");
  setStatus("Old recovery codes were invalidated. Save the ten replacement codes now.");
});

bind("load-passkeys", async () => {
  await loadPasskeys();
  setStatus("Passkey list refreshed.");
});

bind("load-sessions", async () => {
  await loadSessions();
  setStatus("Browser session list refreshed with current item revisions.");
});

bind("logout", async () => {
  await controlPlane.methods.logout({ idempotencyKey: idempotencyKey(), body: {} });
  setAuth(undefined);
  clearRecoveryCodes();
  setStatus("Signed out; the session cookie and in-memory CSRF value were cleared.");
});

bind("copy-recovery", async () => {
  if (!recoveryCodes) throw new Error("No one-time recovery codes are available");
  await navigator.clipboard.writeText(`${recoveryCodes.codes.join("\n")}\n`);
  setStatus("Recovery codes copied. Clear the clipboard after storing them safely.");
});

bind("download-recovery", async () => {
  if (!recoveryCodes) throw new Error("No one-time recovery codes are available");
  downloadJSON(`multidesk-recovery-codes-${recoveryCodes.batchId}.json`, recoveryCodes);
  setStatus("Recovery codes downloaded. Protect the file as a secret.");
});

bind("clear-recovery", async () => {
  if (!element<HTMLInputElement>("recovery-saved").checked) throw new Error("Confirm that the recovery codes are saved before clearing them");
  clearRecoveryCodes();
  setStatus("One-time recovery code plaintext was cleared from the page.");
});

window.addEventListener("pagehide", () => {
  descriptor = undefined;
  challenge = undefined;
  receipt = undefined;
  recoveryCodes = undefined;
  currentAuth = undefined;
  passkeys = [];
  sessions = [];
  controlPlane.clearCsrfToken();
});

renderAnchor();
renderChallenge();
renderReceipt();
renderPasskeys();
renderSessions();
setAuth(undefined);
void Promise.allSettled([loadBootstrapStatus(), loadCurrentAuth()]).then((results) => {
  const failure = results.find((result): result is PromiseRejectedResult => result.status === "rejected");
  if (failure) setError(failure.reason);
});
