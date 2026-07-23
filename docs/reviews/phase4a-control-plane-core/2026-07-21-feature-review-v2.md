# Feature Review v2: Phase 4a Control Plane Core

- Date: 2026-07-21
- Role: `feature-review`
- Owner module: `control-plane`
- Reviewed plan version: `v0.2`
- Verdict: `REVISE`

## Conclusion

Plan v0.2 materially improves the feature and closes most of the seven v1
findings. The TypeScript contract/runtime boundary, CSRF/recovery/Passkey
lifecycle, enrollment actors and public receipt, capability evolution,
deletion watermarks, snapshots, reverse UUID mappings, and command receipt
states now have concrete designs and verification gates. P0 and P1 are directly
executable as written.

The complete P0-P6 plan is not approved because P2 depends on the remote Device
key envelope/migration that is still assigned to P3, and the remaining sync and
command replay contracts require material choices. These are phase-ordering and
durability issues, not implementation details.

## V1 finding closure audit

1. **Portable Vault/bootstrap — partially closed.** `DeviceKeyEnvelopeV1`, its
   dedicated table, AAD, AEAD/DEK wrapping, CAS/lifecycle, assertion truthfulness,
   and local bootstrap-token rotation are frozen. The implementation phase is
   ordered after its first required use; see Finding 1.
2. **OpenAPI/TypeScript — closed.** The plan now truthfully generates Go
   server/client/models and TypeScript types, while a named first-party runtime
   client is exhaustively type-bound. Exact generation, validation, drift, and
   license gates are named.
3. **Cookie/CSRF/recovery/Passkeys — closed.** The endpoint matrix, CSRF
   delivery/rotation, exact recovery format/hashing/rotation, recent UV,
   replacement transition, list/delete, and last-Passkey guard are frozen.
4. **Session Commands — partially closed.** Claim/ack/result DTOs and daemon
   reconciliation states are now explicit, but lease-expiry attempt rebinding
   remains undefined; see Finding 3.
5. **Tombstone/sync — partially closed.** Lifetime deletion watermarks,
   snapshots, revision history, and backup rules close stale resurrection. The
   digest/diff wire format remains incomplete; see Finding 2.
6. **Enrollment/capability evolution — closed.** Actor-complete CLI/Web flows,
   public activation receipt, no activation secret, kind/storage maximums,
   preserved-but-ineffective unknown capabilities, and signed monotonic
   elevation are frozen. One stale test assertion remains; see Finding 4.
7. **UUID mappings — closed.** Device-origin mappings, target-owned
   server-created Profiles, topological snapshot application, restore, orphan,
   collision, and quarantine behavior are specified.

## Ranked findings

### 1. Critical — P2 requires `DeviceKeyEnvelopeV1`, but P3 owns its migration and implementation

Files: `design.md` P2/P3 and Device integration; `test.md` P2/P3;
`dev_log.md` Phase Plan P2/P3.

P2 bootstrap requires the Daemon to commit and later open the exact
`DeviceKeyEnvelopeV1` plus UUID mapping before `bootstrap/options`. Its P2 tests
require official Daemon envelope integration evidence. However, `design.md`
P3 says "Add migration 0008/DeviceKeyEnvelopeV1", `test.md` first creates and
tests migration `0008` under P3, and the Phase Plan assigns the exact envelope
lifecycle to P3 after P2 has independently verified.

P2 cannot bootstrap the required portable-Vault anchor on the verified P1
baseline without implementing P3 scope early. Move the minimal migration,
envelope store/API, mapping, pending->active bootstrap lifecycle, and exact
Daemon bootstrap actor flow into P2 (leaving general enrollment/revocation in
P3), or reorder/split the phases so the envelope foundation is independently
verified before bootstrap. Update all three artifacts and phase acceptance
consistently.

### 2. High — sync revision-digest and field-diff encoding is not wire-complete

Files: `design.md` "Revisioned sync and tombstones"; `api.md` Sync DTOs;
`test.md` P4.

The server hashes a "typed canonical base", but no canonicalization algorithm,
domain separation, or exact digest input is defined. Create uses
`baseRevision=0` and `base=null`, yet the verifier requires equality with a
stored `(type,id,revision)` digest even though no revision-zero history row
exists. Implementers must choose the null/create digest rule.

`SyncConflict.differences` permits only `scalar|null`, while Profile,
Workspace, Session, and Usage contain arrays, maps, and nested windows. The
plan does not define whether these are atomic top-level values, recursively
diffed JSON pointers, or rejected from conflict detail. Freeze a domain-
separated canonical encoding/digest for create and every revision, exact
revision-zero behavior, and deterministic bounded diff rules/schema for maps,
arrays, and nested objects. Add cross-language/golden and hostile ambiguity
tests.

### 3. High — an expired unacked claim cannot safely reuse an existing `reserved` daemon receipt

Files: `design.md` "Asynchronous Session Commands"; `api.md` command/receipt
DTOs; `test.md` P5.

The daemon persists `reserved` with attempt N before ack. If the claim expires
before ack commits, the server requeues and issues attempt N+1. On redelivery,
the daemon already has the command/digest receipt bound to N, while ack requires
the current attempt. The plan says duplicates return the stored state but does
not permit or define a safe attempt rebind. Reusing N yields a stale ack;
creating a second receipt risks duplicate execution.

Freeze the server/daemon rule for this boundary—for example, an atomic
`reserved`-only attempt rebind that changes no local operation identity and
recomputes the receipt digest, with `executing|later` states forbidden from
rebind—or adopt another explicit reconciliation protocol. Test claim expiry
before/while ack, lost ack request/response, concurrent reclaim, and restart.

### 4. Medium — test gates retain two contradictions with the revised crypto/enrollment contract

Files: `test.md` P0 Key PoP vector and P3 enrollment happy path; corresponding
`design.md`/`api.md` contracts.

The P3 happy-path test still says activation "returns bootstrap device-auth
material once", while v0.2 deliberately freezes a public activation receipt
with no secret/connection credential and later Device-auth PoP. Replace the
stale assertion with the public receipt/no-secret contract.

The PoP transcript now binds `storageMode` and `storageAssertionDigest`, but the
P0 negative-vector gate does not require either mutation to fail. Add both
mutations to the Go/TypeScript vector gate so P0 verifies the new bootstrap
storage assertion cannot be detached or relabeled.

## Positive findings

- P0 correctly remains documentation/vector-only and preserves all prior
  pairwise E2EE negatives while updating pin/JCS/PoP authority.
- P1 has exact OpenAPI 3.0.3 generation paths and commands, a pinned tool graph,
  deterministic byte comparison, and a truthful first-party TypeScript runtime
  boundary.
- Authentication and recovery now have executable entropy, persistence,
  endpoint-security, session-transition, and hostile-input rules.
- The 4a/4b/5 boundary remains truthful: no WSS, HPKE, Pairwise Root, terminal,
  Approval response, Credential Grant, Desktop key store, or release claim is
  pulled into Phase 4a.
- Rollback remains forward-migration plus verified backup/prior binary; local
  Sessions and outboxes are not destructively reinterpreted.
- The Provider Gate remains correctly `none`, and the final Security Gate
  remains open for independent review rather than being self-accepted by tests.

## Evidence

- Re-read `AGENTS.md`, `CLAUDE.md`, the `feature-review` role, workflow policy,
  module registry/classification skill, implementation-plan authority, v0.2
  feature brief, complete `design.md`, `api.md`, `test.md`, `dev_log.md`, and
  the v1 review report.
- Compared every v1 finding against v0.2 DTOs, phase ownership, acceptance
  gates, rollback, and later-phase boundaries.
- External pins remain the versions verified in v1 and frozen by v0.2; this
  review does not claim a new transitive resolution.
- Coordinator-recorded `project:verify`, `ci:verify`, and `git diff --check`
  success is acknowledged as structural baseline evidence and was not
  represented as rerun by this reviewer.

## Verdict

`REVISE`. Return to `feature-plan` to close Findings 1-4 and resubmit a new plan
version. No product build phase is authorized by this verdict.
