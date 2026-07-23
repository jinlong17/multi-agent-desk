import type { components } from "@multi-agent-desk/protocol";

type CreationOptions = components["schemas"]["WebAuthnCreationOptionsV1"];
type RequestOptions = components["schemas"]["WebAuthnRequestOptionsV1"];
type RegistrationCredential = components["schemas"]["WebAuthnRegistrationCredentialV1"];
type AssertionCredential = components["schemas"]["WebAuthnAssertionCredentialV1"];

export function decodeBase64URL(value: string): ArrayBuffer {
  if (!/^[A-Za-z0-9_-]+$/.test(value)) throw new Error("Server returned non-canonical Base64url");
  const padded = value.replace(/-/g, "+").replace(/_/g, "/") + "=".repeat((4 - value.length % 4) % 4);
  const binary = atob(padded);
  const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
  if (encodeBase64URL(bytes) !== value) throw new Error("Server returned non-canonical Base64url");
  return bytes.buffer;
}

export function encodeBase64URL(value: ArrayBuffer | ArrayBufferView): string {
  const bytes = value instanceof ArrayBuffer
    ? new Uint8Array(value)
    : new Uint8Array(value.buffer, value.byteOffset, value.byteLength);
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/u, "");
}

export function toCreationOptions(value: CreationOptions): PublicKeyCredentialCreationOptions {
  return {
    challenge: decodeBase64URL(value.publicKey.challenge),
    rp: value.publicKey.rp,
    user: {
      id: decodeBase64URL(value.publicKey.user.id),
      name: value.publicKey.user.name,
      displayName: value.publicKey.user.displayName,
    },
    pubKeyCredParams: value.publicKey.pubKeyCredParams,
    timeout: value.publicKey.timeout,
    excludeCredentials: value.publicKey.excludeCredentials.map((credential) => ({
      type: "public-key",
      id: decodeBase64URL(credential.id),
      transports: credential.transports as AuthenticatorTransport[] | undefined,
    })),
    authenticatorSelection: {
      ...value.publicKey.authenticatorSelection,
      authenticatorAttachment: value.publicKey.authenticatorSelection.authenticatorAttachment,
    },
    attestation: "none",
    extensions: {},
  };
}

export function toRequestOptions(value: RequestOptions): PublicKeyCredentialRequestOptions {
  return {
    challenge: decodeBase64URL(value.publicKey.challenge),
    timeout: value.publicKey.timeout,
    rpId: value.publicKey.rpId,
    allowCredentials: value.publicKey.allowCredentials.map((credential) => ({
      type: "public-key",
      id: decodeBase64URL(credential.id),
      transports: credential.transports as AuthenticatorTransport[] | undefined,
    })),
    userVerification: "required",
    extensions: {},
  };
}

function extensionResults(credential: PublicKeyCredential): Record<string, never> {
  const result = credential.getClientExtensionResults();
  if (Object.keys(result).length !== 0) throw new Error("Unexpected WebAuthn extension output was returned");
  return {};
}

function authenticatorAttachment(credential: PublicKeyCredential): "platform" | "cross-platform" | undefined {
  const value = credential.authenticatorAttachment;
  if (value === null) return undefined;
  if (value !== "platform" && value !== "cross-platform") throw new Error("Unexpected authenticator attachment");
  return value;
}

export function serializeRegistrationCredential(credential: PublicKeyCredential): RegistrationCredential {
  if (!(credential.response instanceof AuthenticatorAttestationResponse)) throw new Error("Passkey registration returned the wrong response type");
  const id = encodeBase64URL(credential.rawId);
  const transports = credential.response.getTransports?.() as RegistrationCredential["response"]["transports"] | undefined;
  return {
    id,
    rawId: id,
    type: "public-key",
    authenticatorAttachment: authenticatorAttachment(credential),
    clientExtensionResults: extensionResults(credential),
    response: {
      clientDataJSON: encodeBase64URL(credential.response.clientDataJSON),
      attestationObject: encodeBase64URL(credential.response.attestationObject),
      ...(transports && transports.length > 0 ? { transports } : {}),
    },
  };
}

export function serializeAssertionCredential(credential: PublicKeyCredential): AssertionCredential {
  if (!(credential.response instanceof AuthenticatorAssertionResponse)) throw new Error("Passkey authentication returned the wrong response type");
  const id = encodeBase64URL(credential.rawId);
  return {
    id,
    rawId: id,
    type: "public-key",
    authenticatorAttachment: authenticatorAttachment(credential),
    clientExtensionResults: extensionResults(credential),
    response: {
      clientDataJSON: encodeBase64URL(credential.response.clientDataJSON),
      authenticatorData: encodeBase64URL(credential.response.authenticatorData),
      signature: encodeBase64URL(credential.response.signature),
      ...(credential.response.userHandle ? { userHandle: encodeBase64URL(credential.response.userHandle) } : {}),
    },
  };
}
