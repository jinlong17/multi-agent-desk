# Phase 2 Codex Vertical Slice: P4 Security Review Handoff

This is an input package, not a Security Review verdict or risk acceptance.

## Review target

- Feature: `phase2-codex-vertical-slice`
- Owner: `provider`; impacts: `core`, `security`, `project-system`
- Exact supported vertical slice: Codex CLI `0.144.2` on Linux x86_64
- Additional evidence: exact macOS 26.5.2 arm64 schema/empty-home handshake;
  Windows amd64 build/protocol baseline only
- Security Gate: open until an independent `security-review` verdict

## Credential and process boundary

- Vault v1 uses Argon2id-derived wrapping plus AES-GCM, private files, explicit
  lock/unlock, authenticated metadata, bounded items, and atomic revision CAS.
- Enrollment is owner-bound and time-bounded, pins exact executable bytes,
  invokes official `codex login`, validates `account/read`, imports only the
  exact private bounded `auth.json`, zeroes import buffers, and deletes complete
  staging on terminal transitions.
- Official login, enrollment validation, and runtime inherit only validated
  credential-free HTTP(S) proxy/no-proxy entries; userinfo, non-HTTP schemes,
  paths, query/fragment, controls, oversize, unrelated variables, and secret
  environment are rejected or omitted.
- One `CredentialRuntime` owns one materialized writable home and app-server;
  multiple Sessions use independent thread bindings. Lease refresh, CAS,
  quarantine, crash fan-out, and last-binding finalization are deterministic.

## Authorization and protocol boundary

- Account/Profile/Workspace/Credential and compatibility capabilities are read
  from Store/daemon authority; client overrides and `danger-full-access` fail
  before a Provider frame.
- JSON-RPC has one bounded writer and one reader multiplexer. Unknown methods,
  fields, IDs, threads, turns, oversized frames, or changed schemas fail closed.
- Approval requests retain Provider request IDs but persist only bounded
  identity, summary, and payload digest. Claim/write/complete is atomic and
  bounded; timeout becomes ambiguous and cannot replay.
- Only standard command/file `accept|decline|cancel` is enabled. Permissions,
  `acceptForSession`, policy amendments, invented aliases, and persistent
  variants produce no response and fail closed.
- Input/stop/kill require the current ControllerLease. Conversation resize and
  Resume are typed unsupported; live Resume tests create no Provider frame or
  new local Session.

## Evidence to inspect

- `docs/reviews/phase2-codex-vertical-slice/2026-07-16-feature-verify-p3b.md`
- `docs/workflow/features/phase2-codex-vertical-slice/test.md`
- `docs/workflow/features/phase2-codex-vertical-slice/dev_log.md`
- `docs/PROVIDER_COMPATIBILITY.md`
- `docs/THREAT_MODEL.md`
- `docs/adr/0014-codex-app-server-single-writer-auth.md`
- implementation under `internal/providers/codex`, `internal/vault`,
  `internal/storage`, `internal/runtime`, `internal/app`, and `cmd/multidesk`

## Required adversarial focus

1. Proxy allowlist and child-environment secret exclusion.
2. Enrollment staging/auth-only import, symlink/private-file/bounds, cleanup,
   owner/deadline/idempotency, and prior-credential preservation.
3. Vault crypto/AAD/tamper/wrong-key/corruption/init-race/item-CAS behavior.
4. Shared writer lease/CAS/quarantine and app-server crash/finalization races.
5. JSON-RPC bounded writer, event/request routing, unknown schema failure, and
   non-persistence of status/patch/diff payloads.
6. Approval claim/write ambiguity, replay/conflict, disabled permission /
   persistent / policy-amendment variants, and stale-lease rejection.
7. Secret-like artifact scan and truthful platform/version claims.

## Explicit residuals and non-claims

- Provider/root/admin/live-process compromise can read usable materialized
  credentials; Vault encryption protects storage, not a compromised runtime.
- No multi-writer refresh, completed device-auth, 48-hour stability, dynamic
  policy amendment, permissions grant, or Provider continuation claim.
- No real Windows Codex support. Current Windows evidence is build/protocol plus
  the unchanged native Phase 1 IPC baseline.
- macOS supports only the exact `0.144.2` schema/handshake smoke in this feature;
  bundled `0.144.5` remains unsupported.
- No Control Plane credential storage, remote grant, Desktop packaging, release,
  deployment, or Ship authorization is included.

## Expected handoff

After independent final-phase feature verification returns `READY_TO_SHIP`, run
`security-review` against this package. Only `ACCEPTED` may clear the open
Security Gate; it does not itself authorize Ship, merge, push, release, or
deployment.
