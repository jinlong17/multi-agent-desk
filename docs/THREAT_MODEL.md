# MultiAgentDesk threat model

- Status: Phase 0 model with evidence-backed browser storage and E2EE protocol decisions; production mitigations are not yet verified
- Date: 2026-07-14
- Owner module: `security`
- Baseline: [Implementation Plan v0.2](IMPLEMENTATION_PLAN.md),
  [security invariants](../CLAUDE.md), and [ADR index](adr/README.md)

## 1. Scope and non-goals

This model covers v0.1 Device Daemon, CLI/TUI, Web/PWA, Desktop shell, Control
Plane, built-in Provider processes, local Vault/materialization, device pairing,
CredentialGrant, metadata sync, ciphertext relay, and dependency/update
boundaries.

It does not prove that planned controls are implemented. ADR 0010 selects the
browser key-storage modes and ADR 0011 selects the E2EE protocol candidate from
reproducible Spike evidence, but neither is production runtime evidence. This
model does not attest a Provider credential format, validate Windows
ConPTY/Named Pipe/Tauri behavior, or promise that revocation remotely erases a
secret already copied to another device.

Evidence states used below:

- **accepted design**: reviewed requirement; not runtime evidence;
- **planned**: required mitigation is not yet proven in product code;
- **pending evidence**: the named Spike must resolve the mechanism or claim;
- **verified design evidence**: a linked, reproducible Spike proves the stated
  compatibility/protocol property, not production implementation;
- **deferred**: explicitly open acceptance outside the current environment.

Unknown, planned, pending, partial, and deferred evidence never count as pass.

## 2. Assets and security objectives

| Asset | Confidentiality objective | Integrity/authorization objective | Availability objective |
|---|---|---|---|
| Provider credentials and Vault keys | plaintext remains on authorized devices and out of Control Plane/logs | only explicit authorized flows create or update a CredentialInstance | lock/failure does not destroy or overwrite the Vault |
| Device signing/exchange keys and pins | private keys remain device-local | key changes never silently replace a local pin | loss triggers re-enrollment, not server recovery of the private key |
| CredentialGrant bundles | Control Plane sees ciphertext only | target, source, revision, purpose, expiry, and grant identity are authenticated and replay-safe | failure is explicit and retry-safe |
| Session terminal/model content | plaintext remains on session device and approved endpoints | inputs, approvals, resize, stop, and kill require current authorization/lease | detach or relay loss does not silently kill the Provider process |
| Account/Profile/Workspace configuration | secrets are excluded from ordinary sync | revisions and ownership prevent stale or cross-account overwrite | local operation uses last valid device state when Control Plane is offline |
| Session/usage/device metadata | exposure is minimized and documented | server and clients cannot forge authoritative device/session facts | bounded outage and queue failure are observable |
| Audit records | never contain credentials or terminal/model plaintext | security-sensitive actions are attributable and append-oriented | enough evidence remains for diagnosis without becoming a secret store |
| Build and release artifacts | no embedded credentials or unreviewed copied code | dependencies, provenance, signing, and licenses are gated before release | rollback paths remain available |

## 3. Attacker model

| Attacker | Assumed capability | Not assumed away |
|---|---|---|
| Network attacker | observe, replay, delay, reorder, drop, or modify traffic outside correctly authenticated encryption | TLS or relay availability alone does not establish device trust |
| Compromised Control Plane/operator | read or alter server databases, public-key directory, metadata, queued ciphertext, and service behavior | server is not trusted with Provider plaintext or as the pin trust anchor |
| Unapproved local process | run as the same or another OS user, probe IPC, inspect permissive files, race lifecycle operations | host root/admin compromise can defeat local isolation |
| Compromised authorized device/server | read memory/files accessible to that device and use credentials already granted to it | revocation cannot guarantee erasure of copied secrets |
| Malicious or compromised Web origin/dependency | execute script in the application origin, steal accessible keys/content, issue authorized requests | XSS can defeat client-side E2EE while the client is active |
| Malicious Provider process/config | read its materialized credential area, emit hostile terminal content, hang, crash, or alter auth files | Provider behavior and credential layout require separate evidence |
| Supply-chain attacker | compromise a dependency, toolchain, update, adapter, workflow, or release artifact | CI success alone is not provenance or signing proof |
| Authorized but mistaken user | approve the wrong fingerprint/device, grant the wrong account, disclose recovery material, or kill a session | UI confirmation reduces but cannot eliminate human error |

## 4. Trust boundaries

| Boundary | Trusted input/identity | Required checks | Residual exposure |
|---|---|---|---|
| User ↔ Web/PWA | Passkey-authenticated user plus separately enrolled Web Device | origin security, CSP, enrollment, locally pinned keys, capability check | active XSS can access decrypted content; site-data loss loses the Web key |
| Web/Desktop ↔ Control Plane | TLS-authenticated service, but server may be read/modified | signed requests, bounded metadata, ciphertext-only protected payloads | traffic and metadata patterns remain visible; service can deny availability |
| Control Plane ↔ Device Daemon | enrolled Device identity whose keys match local pins/attestations | signature, exchange-key match, audience, purpose, expiry, replay, revision | compromised server can suppress or replay traffic until client checks reject it |
| CLI/TUI/Desktop ↔ local Daemon | local OS identity plus mutually authenticated local Device/client over IPC | 0600 Unix socket or protected current-logon Named Pipe, server/client authentication, protocol/version checks, capability and lease rules, bounded requests | same-logon malware or root/admin may race ownership, consume resources, or act with local user authority |
| Daemon ↔ Vault/database/filesystem | device-local process and OS protection | least privilege, restrictive permissions, authenticated encryption, transactions | root/admin or live-process compromise can read plaintext/memory |
| Daemon ↔ Provider process | configured binary and isolated runtime home | argument/env allowlists, explicit capability, path/version checks, output handling | Provider must read materialized credentials and can change its files |
| Source Device ↔ target Device grant | directly pinned keys or valid attestation from a locally pinned approver | source/target/capability binding, freshness, revision, replay protection, signed receipt | an already compromised approved target receives usable plaintext by design |
| Build/research input ↔ repository/release | reviewed source and compatible license | DCO, license, dependency, link, provenance/signing gates | compromised tooling or reviewer error remains possible |

## 5. Non-negotiable security invariants

1. A Passkey authenticates a user to the Control Plane; it does not derive,
   replace, recover, or authorize use of an E2EE Device private key.
2. The Control Plane public-key directory is an index, not a trust anchor. It
   cannot silently replace locally pinned signing or exchange keys.
3. Credential grants are explicit, target-device scoped, encrypted,
   revocable for future use, and never described as remotely erasable after a
   target or host has copied the plaintext secret.
4. Credential refresh/materialization has one writer and uses monotonic
   credential revision/CAS semantics; mtime is only a change hint.
5. MultiAgentDesk never implements automatic account rotation, quota bypass,
   rate-limit evasion, or transparent mid-session credential switching.
6. Realtime E2EE roots are random and pairwise per Host↔Peer; no Peer receives
   a group root that can derive another Peer's traffic keys, and cryptographic
   possession never replaces capability or ControllerLease checks.

## 6. Threats, required mitigations, and evidence

| ID | Asset/boundary and attacker scenario | Impact | Required mitigation | Evidence state | Residual risk |
|---|---|---|---|---|---|
| T-01 | Compromised Control Plane substitutes a Device key | credential/session decryption by attacker or unauthorized grants | local pin match; key change is a new device; attestation only from directly pinned approver; fingerprint ceremony | verified design evidence: ADR 0011 and `spike-e2ee-protocol-vectors`; production enforcement planned | user may approve a malicious fingerprint; compromised pinned approver can attest a bad key |
| T-02 | Relay replays/reorders/tampers with enrollment, grant, command, or terminal envelopes | duplicated grant, stale control, forged state, content corruption | pairwise root; authenticated source/target/purpose/audience/epoch/message ID; JCS AAD; nonce recomputation; durable replay window | verified design evidence and negative vectors in `spike-e2ee-protocol-vectors`; production persistence planned | availability attacks and bounded state loss remain possible |
| T-03 | Control Plane DB/log/queue captures Provider credential or terminal plaintext | broad remote secret/content disclosure | data-classification allowlist, ciphertext-only protected payloads, redaction tests, no plaintext trace | planned | metadata, traffic patterns, device/account/session identifiers remain exposed |
| T-04 | Vault or Device DB is copied, permissions are weak, or unlock material leaks | offline credential/key theft | OS key store or password-derived wrapping, authenticated encryption, restrictive permissions, explicit locked state | browser storage modes have verified design evidence in ADR 0010; Daemon/Desktop Vault implementation remains planned | host root/admin, unlocked-process compromise, weak user password, and memory disclosure remain |
| T-05 | Daemon crash or concurrent refresh writes an older credential over a newer one | account lockout, stale token reuse, credential corruption | single materialization writer, monotonic `credentialRevision` CAS, digest validation, transactional recovery, quarantine ambiguous leases | Codex design evidence verified by ADR 0014; Claude ADR 0016 uses target-local interactive login and disables stable setup-token materialization; production enforcement planned | Provider-side mutation can remain ambiguous and require re-login; multi-writer Codex refresh and Claude setup-token grant are unsupported |
| T-06 | Materialized auth home, temp file, process env, crash dump, or backup exposes plaintext | local credential theft | per-session least-privilege directory, minimal env, cleanup after process exit, quarantine on uncertain recovery, secret-safe diagnostics | Codex `0600` layout observed under ADR 0014; Claude Config Dir/Keychain slot isolation observed under ADR 0016; production cleanup/redaction remains planned | **Provider-readable plaintext/authenticated state exists at runtime; host root/admin or Provider compromise can copy/use it** |
| T-07 | Unauthorized local process connects to IPC, impersonates a pipe endpoint, or steals a ControllerLease | session observation/input/resize/stop/kill by attacker | 0600 Unix socket; protected current-logon Named Pipe with Network deny, remote rejection, and first-instance fail-closed ownership; mutual peer authentication; request capability checks; expiring lease with owner identity; bounds and audit events | Windows transport verified by ADR 0013 and `spike-windows-named-pipe-ipc`; protocol authorization and Unix/Windows production enforcement planned for Phase 1 | same-logon malware and root/admin can race availability or act with the user's local authority |
| T-08 | Multiple clients race control, stale lease holder sends input, or detach kills the process | command confusion, loss of work, unintended termination | single ControllerLease, monotonic lease revision/expiry, explicit acquire/release, detach separate from process lifecycle, idempotent stop/kill | planned for Phase 1 | network partitions can delay awareness; forced takeover may discard unsent input |
| T-09 | Provider binary/path/config arguments are malicious or wrong account files are reused | code execution, secret crossover, wrong-account actions | explicit binary path/version evidence, argument/env construction without shell injection, isolated runtime homes, pinned session account/profile/capabilities | Codex boundary accepted by ADR 0014; Claude target-profile Config Dir/version/account boundary accepted by ADR 0016; production enforcement planned | a user-approved or compromised Provider binary executes with granted local access |
| T-10 | Hostile terminal/model output injects control sequences or content into TUI/Web/Desktop | UI spoofing, clipboard abuse, XSS, data exfiltration | terminal parser hardening, output encoding, strict CSP, no untrusted HTML, bounded buffers, security tests | ConPTY mechanism evidence is verified by ADR 0012; production provider, renderer, IME/accessibility, and parser enforcement remain planned | terminal emulation and browser dependencies retain parser bugs |
| T-11 | Web XSS or malicious dependency accesses enrolled Device keys and decrypted content | live session/credential grant compromise | no third-party scripts, strict CSP/Trusted Types where supported, dependency audit, non-exportable keys, metadata-only fallback | storage compatibility verified by ADR 0010; origin hardening and production enforcement planned | active same-origin code can use keys/content even if key bytes are non-exportable |
| T-12 | CredentialGrant targets wrong/revoked/incapable device or is replayed | unauthorized credential copy | explicit user confirmation, `credentials.store` capability, direct pin/attestation validation, target-bound HPKE, revision/expiry/replay checks, signed receipt; exclude unverified Provider formats | pin/attestation/HPKE mechanism verified by ADR 0011; ADR 0016 excludes Claude setup-token from stable grant; implementation remains planned | user error or already compromised approved target remains; **revocation cannot erase copied plaintext** |
| T-13 | Revoked Device/pairwise root continues receiving new content | post-revocation access | reject revoked identity before decrypt, close WSS, tombstone/rotate affected pairs, invalidate leases, audit revocation | old-root and cross-peer rejection have verified design evidence in ADR 0011; production orchestration planned | previously decrypted or copied data remains outside remote control |
| T-14 | Secrets or terminal content enter logs, metrics, traces, crash reports, generated dashboard, or audit export | secondary disclosure | structured allowlisted telemetry, secret-field blacklist, payload exclusion, test fixtures, bounded audit schema | Claude auth JSON PII/raw-payload exclusion accepted by ADR 0016; dashboard secret-field check exists; broader production redaction planned | novel fields or third-party crash tooling may bypass redaction until tested |
| T-15 | Control Plane outage, relay suppression, or local DB corruption causes unsafe fallback | loss of control/data or bypassed authorization | local-first operation with last valid local state, fail closed for new remote grants/control, explicit offline/stale status, backups and migration rollback | planned | remote observation/control is unavailable; local disk failure can lose unbacked metadata |
| T-16 | Dependency, update, external adapter, copied research, or CI workflow is compromised | arbitrary code execution, license contamination, release compromise | compatible-license gate, DCO, dependency review, pinned toolchains, provenance/signing/SBOM before release, external adapters out of process | planned; release controls deferred to Phase 6 | upstream compromise, maintainer key theft, and review error remain possible |
| T-17 | Automatic recommendation silently rotates accounts or uses misleading quota data | policy evasion, wrong-account actions, Provider enforcement risk | user confirmation, pinned session identity, source/freshness labels, no automatic rotation or mid-session switch | Codex rate-limit/usage verified by ADR 0014; ADR 0016 classifies Claude quota/session-limit as non-auth state and provides no official remaining-quota claim | user may choose poorly based on stale or estimated usage |
| T-18 | Desktop starts a second Daemon or an untrusted sidecar | split-brain Vault/session state or code execution | discover/authenticate system service first; fixed Rust externalBin; single-instance lock; signed/provenance/version verification; authenticated instance ownership; Desktop exit does not stop daemon; explicit stop only for owned sidecar | lifecycle design evidence verified by ADR 0015; production signing/updater/IPC enforcement and Windows 11 acceptance planned | signed supply-chain, updater, same-logon malware, administrator, stale-instance, and service-boundary compromise remain |

## 7. Failure and recovery rules

- Key mismatch, invalid attestation, replay, wrong audience/purpose, revoked
  Device, or unauthorized capability fails closed and requires re-pairing or
  explicit remediation; the Control Plane cannot override a local pin.
- A locked Vault keeps metadata queries available but denies new secret
  materialization/session start with an explicit `vault_locked` error. Unlock
  failure never clears or overwrites the Vault.
- Ambiguous or unauthenticated credential lease residue is quarantined. It is
  not written back over the Vault merely because its mtime is newer.
- Detach releases a client relationship; it does not stop the Provider process.
  Stop requests graceful termination; Kill is explicit forced termination.
- Control Plane outage disables remote coordination but does not weaken local
  authorization or silently switch accounts/credentials.
- Audit and diagnostic paths record identifiers, decisions, and error classes,
  not Provider secrets or terminal/model plaintext.

## 8. Resolved Spike decisions and deferred production evidence

| Area | Required evidence | Current state |
|---|---|---|
| Codex credential store, auth refresh, concurrent revision behavior | [`spike-codex-auth-refresh`](workflow/features/spike-codex-auth-refresh/dev_log.md) | `GATE_RESOLVED`; ADR 0014 selects one canonical writable app-server/auth home and interactive-login fallback; multi-writer and completed device-auth unsupported |
| Claude config/keychain isolation, auth status/setup token, refresh behavior | [`spike-claude-config-keychain`](workflow/features/spike-claude-config-keychain/dev_log.md) | `GATE_RESOLVED`; ADR 0016 selects target-local interactive login and version-gated/redacted auth health; setup-token grant, distinct-account isolation, and long session unsupported by Spike |
| Browser non-exportable/wrapped key storage and metadata-only fallback | [`spike-browser-key-storage`](workflow/features/spike-browser-key-storage/dev_log.md) | `GATE_RESOLVED`; verified design evidence in ADR 0010; production implementation pending |
| E2EE envelope, AAD binding, replay, pairwise roots, vectors, and security review | [`spike-e2ee-protocol-vectors`](workflow/features/spike-e2ee-protocol-vectors/dev_log.md) | `GATE_RESOLVED`; verified design evidence in ADR 0011; production implementation pending |
| Windows terminal/ConPTY behavior | [`spike-windows-conpty`](workflow/features/spike-windows-conpty/dev_log.md) | `GATE_RESOLVED`; native ConPTY selected by ADR 0012; Windows 11 real-provider acceptance pending |
| Windows local IPC/Named Pipe behavior | [`spike-windows-named-pipe-ipc`](workflow/features/spike-windows-named-pipe-ipc/dev_log.md) | `GATE_RESOLVED`; protected native Named Pipe selected by ADR 0013; production protocol and Windows 11 multi-session/service acceptance pending |
| Windows Tauri sidecar lifecycle | [`spike-windows-desktop-sidecar`](workflow/features/spike-windows-desktop-sidecar/dev_log.md) | `GATE_RESOLVED`; ADR 0015 selects discover-first crash-surviving sidecar with authenticated ownership and signed fixed launch; Windows 11 release acceptance pending |

Windows Server CI now provides native ConPTY and Named Pipe transport evidence,
but Windows 11 workstation acceptance remains open. GitHub Actions cannot
replace real Provider TUI/IME/accessibility, two-user Named Pipe/service
contexts, Fake Session, sleep/resume, packaging, or Tauri sidecar acceptance.

## 9. Explicit residual risk

- An authorized device or server must receive usable credentials for local
  Provider execution. If that host is compromised, the attacker may copy them;
  later revocation prevents future authorized flows but cannot prove erasure.
- Provider-readable credentials exist in plaintext at the point of use. Vault
  encryption protects storage, not a compromised root/admin, live Daemon, or
  Provider process.
- A compromised Control Plane can observe metadata/traffic patterns, deny or
  delay service, and attempt key substitution/replay; ADR 0011 and the shared
  vectors verify the candidate rejection behavior, while production
  enforcement remains unimplemented.
- XSS in an active approved Web Device can use keys and decrypted content even
  when private key material is non-exportable.
- Human fingerprint/grant/account confirmation can be mistaken or coerced.
- Supply-chain controls reduce but do not eliminate compromised upstreams,
  toolchains, signing keys, or reviewers.
- Cross-platform process, terminal, filesystem, key-store, and IPC behavior
  differs. Windows interactive behavior remains unverified and deferred.

## 10. Assumptions and update triggers

The v0.1 baseline assumes one user, self-hosted deployment, TLS to the Control
Plane, supported host OS security primitives, and user control of enrolled
devices. Root/admin compromise and compromise of an already authorized target
are not assumed preventable by application cryptography alone.

Re-review this model when any asset, trust boundary, credential flow, key or
envelope protocol, Provider integration, external adapter boundary, logging or
retention behavior, supported platform, update/signing path, or multi-user
scope changes. Each implementation feature must link its security tests and
new residual risks rather than changing an evidence state without proof.
