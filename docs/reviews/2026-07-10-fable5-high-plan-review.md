# Fable 5 High — MultiAgentDesk v0.1 Plan Review Disposition

> Review date: 2026-07-10
> Reviewer verdict: **APPROVE WITH REQUIRED CHANGES**
> Reviewed document: `docs/IMPLEMENTATION_PLAN.md` v0.1 (1094 lines)
> Disposition owner: Codex

## Executive disposition

The review is accepted as materially correct. It found four blocking gaps:

1. Web clients had no E2EE device identity or key lifecycle.
2. Credential grants trusted a Control Plane supplied public key without a local pin.
3. Concurrent sessions could race while refreshing and writing back one credential.
4. Claude multi-account isolation on macOS was treated as confirmed before a real spike.

It also found a phase dependency error, missing domain objects, incomplete session/transport semantics, and an over-scoped v0.1. These findings are incorporated into plan v0.2.

## Finding disposition

| Finding | Priority | Disposition | Resulting change |
|---|---:|---|---|
| Web E2EE identity missing | P0 | Accept | Web/Desktop clients become first-class devices; Passkey and E2EE identities are explicitly separate |
| Control Plane key substitution | P0 | Accept | Local pinned-key trust directory, explicit fingerprint format, re-pair on key change, AEAD AAD binding |
| Concurrent credential refresh race | P0 | Accept with refinement | Add one provider-specific materialization lease per CredentialInstance and monotonic credential revision; do not rely on mtime as conflict resolution |
| Claude macOS isolation assumption | P0 | Accept | Add blocking provider spike and deterministic setup-token fallback |
| Phase 3 depends on Phase 4 browser | P1 | Accept | Phase 3 exits through a second local CLI over IPC |
| Workspace/Grant/Approval/Tombstone missing | P1 | Accept | Add explicit entities, grant/approval state machines, inbox/outbox cursor and tombstones |
| Session state incomplete | P1 | Accept with refinement | Process lifecycle remains on Session; attachment/control is modeled separately because multiple clients may observe one Session |
| WS flow control/ACK/frame limits | P1 | Accept | Define direction-scoped sequence, ACK/NACK, 256 KiB frame cap, bounded queues and rate limits |
| Terminal snapshot unspecified | P1 | Accept | v0.1 uses reset + bounded chunk-aligned replay; server-side VT snapshot deferred |
| Headless Vault lifecycle | P1 | Accept | Add locked/unlocked state, stdin/FD unlock and opt-in keyfile/systemd credential mode |
| Revocation does not rotate keys | P1 | Accept | Rotate active Session Keys and rewrap only to surviving clients |
| REST incomplete | P1 | Accept | Add async command resource, workspaces, approvals, grants, health/version and concurrency conventions |
| Passkey deployment prerequisites | P2 | Accept | Require stable RP ID + TLS; document domain migration recovery |
| Retention policy missing | P2 | Accept | Add per-session and global quotas with truncation markers |
| Desktop daemon single instance | P2 | Accept | Connect to system daemon first; otherwise own a Sidecar protected by IPC lock |
| Provider-side revocation guidance | P2 | Accept | Add explicit residual-risk UI and official revocation links |
| DCO/license/AGPL evidence | P2 | Accept | Add DCO, license gates and architecture-research log; no AGPL source copying |
| Timestamp window | P2 | Accept | ±5 minutes as secondary validation; sequence and nonce remain authoritative |

## Scope disposition

- Keep in v0.1: Linux/macOS/Windows CLI and Daemon, Web desktop experience, macOS Desktop, Codex and Claude built-ins, E2EE grants.
- Experimental in v0.1: Windows Desktop preview, mobile PWA behavior, local Claude usage estimation.
- Move to v0.2: AgentDefinition UI/sync, public external Adapter SDK, stable third-party Adapter compatibility.

## Deliberate refinement: Session versus attachment

The review proposed `attached` and `detached` as Session states. That is not adopted literally. A provider process may keep running while zero, one, or several clients observe it, so attachment is not a process lifecycle state.

Plan v0.2 uses:

- `Session.status`: `starting | running | stopping | exited | failed | killed`.
- `SessionAttachment`: one record per connected client.
- `ControllerLease`: at most one client can send terminal input or answer approvals; other clients are read-only.
- Detaching removes an attachment but never changes the provider process state.

This removes concurrent input ambiguity and keeps Resume semantics independent from UI connectivity.

## Additional hardening discovered during disposition

Pairwise key pinning alone did not explain how a credential source trusts a target that was approved by a different client. Plan v0.2 therefore adds signed `DeviceAttestation` records:

- an already pinned approver signs the new device key digests;
- a device may accept that attestation only when it directly pins the approver;
- it then persists the subject as a direct pin before a sensitive operation;
- the Control Plane stores and routes attestations but cannot forge them.

The first E2EE trust anchor must be a Daemon/Desktop with an OS-backed Vault. A pure browser can register a Passkey but cannot bootstrap the cryptographic trust graph alone.

## Required spikes

Before Provider/E2EE design freeze, the implementation must verify:

- Codex auth file location, portability, concurrent refresh and official usage methods.
- Claude macOS Keychain isolation, `auth status` JSON and setup-token interactive PTY behavior.
- Browser non-exportable key storage and deterministic encrypted-key fallback.
- Windows ConPTY and Tauri Sidecar behavior before marking Windows Desktop beyond Experimental.

The plan is allowed to start Phase 0 and Phase 1 while these spikes run; Provider releases and browser E2EE remain gated by their results.
