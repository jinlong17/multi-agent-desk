# Development log: Phase 4a Control Plane Core

## Status Panel

| Field | Value |
|---|---|
| Workflow | `FEATURE_DEV` |
| Target | `phase4a-control-plane-core` |
| Title | `Phase 4a Control Plane Core` |
| Owner Module | `control-plane` |
| Impacted Modules | `security`, `core`, `web`, `desktop`, `project-system` |
| Current Phase | `REVIEW` |
| Status | `APPROVED` |
| Executor | `Codex (GPT-5) as feature-review` |
| Updated | `2026-07-21 02:59 PDT` |
| Suggested Next | `feature-build P0` |
| Branch / Worktree | `codex/control-plane/phase4a-core` / `/Users/jinlong/Desktop/jinlong_project/agent-deck-worktrees/phase4a-control-plane-core` |
| Plan Version | `v0.4` |
| Provider Gate | `none` |
| Security Gate | `open — bootstrap, auth/recovery, device identity/pinning, signed network requests, sync/commands, Web origin` |

## Phase Plan

| Phase | Scope | Dependencies | Acceptance | Status |
|---|---|---|---|---|
| P0 Contract freeze | Reconcile pin/fingerprint/JCS+PoP vectors, portable Vault assertion, 4a/4b/5 capability split, UUIDv7, OpenAPI/tool/license pins, threat/data/protocol docs | approved v0.4 plan; ADR 0010/0011 evidence | Go/TS vectors agree, including negative `storageMode` and `storageAssertionDigest` mutations; authoritative docs and versioned capability reservations agree; exact WebAuthn/codegen/kin-openapi/TS/UUID tool graphs and licenses pass; no product behavior | `planned` |
| P1 Server/storage/OpenAPI foundation | server lifecycle/config/health/version; server SQLite/migrations; OpenAPI 3.0.3; generated Go server/client + TS types; first-party exhaustive TS runtime client; request/ID/cursor/idempotency/error middleware | P0 independently verified | migrations/rollback/hostile input, exact go-tool validation/generation, temp-byte drift, first-party credentials/CSRF/abort/timeout/error coverage, three-platform and explicit tool-license scans pass; no active user/device | `planned` |
| P2 Bootstrap/passkey/recovery/session | Before bootstrap: migration 0008, generic Device mapping, exact DeviceKeyEnvelopeV1 create/open/CAS and pending-active receipt lifecycle; prepare/import/prove/verify/activate actors; then atomic Daemon anchor, local token rotate, WebAuthn/CSRF/recovery/Passkey session lifecycle | P1 independently verified | migration/envelope/AAD/CAS/crash/actor/receipt/no-secret official Daemon integration passes before options; then atomic/failure/lost-token/pure-Web, Passkey/counter/replay/last-key, recovery/concurrency/one-time, endpoint/browser/secret tests pass | `planned` |
| P3 Remote Device identity/enrollment/presence/revocation | Reuse P2 identity foundation for additional Daemons; Web ADR 0010; Desktop contract; actor-complete enrollment/public activation receipt; capability attestation; signed REST/presence/revoke; snapshot-required gate | P2 independently verified; P0 vectors | no prerequisite reimplementation; start/approve/activate/cancel/resume/CLI, kind-storage/elevation, mapping/receipt/key-change/revocation pass; no activation secret; Desktop contract/build smoke only; no HPKE/WSS | `planned` |
| P4 Metadata projection/sync | migration 0009; exact typed RFC 8785 fullBase/fullNext/patch wire; create sentinel/history-missing; mixed batches; targeted mappings; out-of-band target Device + canonical snapshot manifest/page/final chain; push/pull/ack; lifetime watermarks | P3 independently verified | cross-language change/snapshot goldens, Account-first topology, first/empty/final pages, one-active/expiry/identical replay, mixed/reorder/omit/duplicate/truncate/token/cursor rejection, idempotent/conflicting commit, backup/quarantine/concurrency pass | `planned` |
| P5 Async Session Commands | migration 0010; durable 202 create/query; tokenless claim/ack/result/reconcile/TTL; reserved-only attempt rebind; executing/later no-rebind reconciliation; mapped local actions | P4 independently verified; Phase 1 local services | reaper/ack N->N+1 races, lost ack request/response, receipt digest/CAS, reserved rebind, later-state reconcile/quarantine/no-auto-reexec, offline/restart/duplicate/bounds pass; at-least-once explicit | `planned` |
| P6 Web metadata UI + integration/security handoff | Bootstrap/Passkey/Recovery/UV; Devices actor/capability flow; Overview/Accounts/Profiles/Sessions/Usage; responsive/a11y/browser/PWA; Desktop render smoke; final integration/threat/security evidence | P5 independently verified | all v0.4 states/journeys, WCAG/browser/responsive, end-to-end, three-platform/project/tool-license/secret/security gates pass; no Terminal/Approval/Grant; Security Gate remains reviewer-owned | `planned` |

Each `feature-build` owns exactly one approved row above and must stop at
`READY_FOR_VERIFY`. An independent `feature-verify` must set that phase
`VERIFIED` before the next row can begin. `feature-review` may split P6 into P6A
and P6B without broadening scope.

## Evidence Ledger

| Time | Phase | Command/evidence | Result | Artifact |
|---|---|---|---|---|
| 2026-07-21 01:05 PDT | PLAN | `git rev-parse origin/main`; `git rev-parse HEAD`; worktree/branch/status inventory | baseline is `origin/main@e3578390a23ddcf805ceb0bad24b1c41d36977fb`; intake checkout is clean at `71e0448de1624ae3c00cec82f800d0e5425a4dc5` on the operator-created `codex/control-plane/phase4a-core` worktree | Git facts; committed feature brief |
| 2026-07-21 01:15 PDT | PLAN | complete read of AGENTS/CLAUDE, implementation plan, workflow, module registry, feature-plan role, feature brief, ADR 0002/0003/0005/0008/0010/0011, threat model, feature template, current domain/migrations/Web/server/protocol scaffolds | owner is unambiguously `control-plane`; impacts `security/core/web/desktop/project-system`; existing prefixed IDs, portable Vault v1, Usage model, empty server/OpenAPI/Web boundaries recorded | `design.md`; `api.md`; `test.md` |
| 2026-07-21 01:30 PDT | P0 research | GitHub release/tag API and upstream manifests/licenses for `go-webauthn v0.17.4`; upstream release/manifests for `oapi-codegen v2.8.0`; npm registry metadata for `openapi-typescript 7.13.0`; repository Go/Node/pnpm pins | WebAuthn tag is PGP/GitHub verified, 2026-05-22, BSD-3-Clause, Go 1.25/toolchain 1.26.3; oapi-codegen is Apache-2.0/Go 1.25; openapi-typescript is MIT/Node-compatible; repo Go 1.26.5/Node 24 can proceed, subject to full locked graph license gates during build | exact pins in `api.md`; upstream URLs recorded in plan handoff evidence |
| 2026-07-21 01:40 PDT | PLAN | inspect `docs/spikes/e2ee/PROTOCOL.md`, Go/TS vector implementations, and current pin/attestation output | current vector stores full digest but displays eight full-hex groups and current attestation lacks both full digests; P0 authoritative/vector correction is mandatory | `design.md` P0 reconciliation; `test.md` P0 vector gate |
| 2026-07-21 01:49 PDT | PLAN | freeze v0.1 design/API/test/phase plan and run `git diff --check` | four canonical planning artifacts created; status is `NEEDS_REVIEW`; no product code, migration, dashboard, implementation plan, ADR, commit, or push performed | this feature directory |
| 2026-07-21 01:55 PDT | PLAN | bundled Node/pnpm `npm run project:verify`; `npm run ci:verify`; `git diff --check` | workflow/dashboard generation and verification, Actions/CODEOWNERS/fixtures/links/licenses, and diff integrity all pass; generated dashboard facts remain ignored | command output; generated dashboard state |
| 2026-07-21 02:19 PDT | PLAN v0.2 | read persisted feature-review v1; reconcile seven findings across brief/design/API/test without changing P0-P6 order | exact envelope/bootstrap rotate, TS runtime, CSRF/recovery, command receipt, watermark/diff, capability/enrollment, and mapping/snapshot contracts frozen; resubmitted `NEEDS_REVIEW` | `docs/reviews/phase4a-control-plane-core/2026-07-21-feature-review-v1.md`; v0.2 feature artifacts |
| 2026-07-21 02:42 PDT | PLAN v0.3 | read persisted feature-review v2 and all five current planning/state artifacts; reconcile four material findings without changing the P0-P6 sequence | moved executable 0008/envelope/mapping/bootstrap actors into P2; froze canonical full-base/full-next/patch sync wire and history behavior; froze reserved-only command attempt rebind/later-state reconcile; removed stale activation-material test wording and added both storage PoP negatives | `docs/reviews/phase4a-control-plane-core/2026-07-21-feature-review-v2.md`; v0.3 feature artifacts |
| 2026-07-21 02:53 PDT | PLAN v0.4 | read persisted feature-review v3 and reconcile its single snapshot finding across the five planning/state artifacts | made the authenticated/enrolled target Device an out-of-band prerequisite, changed snapshot topology to Account-first, and froze exact RFC 8785 manifest/page/final digest, continuity, expiry, replay, and commit contracts | `docs/reviews/phase4a-control-plane-core/2026-07-21-feature-review-v3.md`; v0.4 feature artifacts |

## Risks and Blockers

- No planning blocker. Product implementation is intentionally gated on
  independent `feature-review` approval.
- The committed brief used the older OS-Vault initial-anchor wording. The plan
  reconciles it to the executable portable password-derived Vault v1 Daemon
  anchor and keeps OS wrapping/Desktop product storage in Phase 5.
- Current protocol vectors and authoritative docs use the older full-hex human
  fingerprint/attestation shape. P0 must update them and preserve all prior
  pairwise negative vectors before P1.
- Phase 4a breadth and concurrency risk are controlled by the seven build rows
  and mandatory independent verification between rows. P6 may be review-split,
  but scope may not expand.
- The Security Gate is open. Planning/tests cannot self-accept bootstrap,
  WebAuthn, recovery, pinning, signed transport, revocation, sync/command replay,
  Web origin, or residual risk.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-21 01:49 PDT | Codex (GPT-5) as feature-plan | Classified owner/impacts, re-read repository truth, reconciled audit decisions, pinned external/toolchain evidence, and froze a decision-complete v0.1 design/API/test plan with P0-P6 build/verify gates | `docs/workflow/features/phase4a-control-plane-core/{design.md,api.md,test.md,dev_log.md}`; concrete brief conflict reconciliation; no commit | `NEEDS_REVIEW`; Provider Gate none; Security Gate open; no product/dashboard/plan/ADR mutation | `feature-review` |
| 2026-07-21 02:01 PDT | Codex (GPT-5) as feature-review | Independently reviewed scope, phase ordering, crypto/auth/API/migration/sync/command/Web boundaries, rollback/tests, dependency pins, and 4a/4b/5 truthfulness; persisted ranked executable-contract findings | `docs/reviews/phase4a-control-plane-core/2026-07-21-feature-review-v1.md`; this Status Panel and one Work Log row only; no commit | `REVISE`; P0 reconciliation is executable, but the plan and P1 TypeScript-client gate require the seven recorded contract revisions before build | `feature-plan` |
| 2026-07-21 02:19 PDT | Codex (GPT-5) as feature-plan | Revised plan to v0.2 and decision-completely closed all seven v1 findings while retaining owner, gates, Phase 4a boundary, and P0-P6 order | feature brief; `docs/workflow/features/phase4a-control-plane-core/{design.md,api.md,test.md,dev_log.md}`; no product/ADR/implementation-plan/dashboard/Git write | `NEEDS_REVIEW`; Provider Gate none; Security Gate open; no self-approval | `feature-review v2` |
| 2026-07-21 02:26 PDT | Codex (GPT-5) as feature-review | Independently re-reviewed plan v0.2 and audited closure of all seven v1 findings, P0-P6 executability, security boundaries, rollback, and verification gates | `docs/reviews/phase4a-control-plane-core/2026-07-21-feature-review-v2.md`; this Status Panel and one Work Log row only; no commit | `REVISE`; P0/P1 are executable, but envelope/bootstrap phase ordering, sync digest/diff encoding, command attempt rebinding, and stale test assertions require plan revision | `feature-plan` |
| 2026-07-21 02:42 PDT | Codex (GPT-5) as feature-plan | Revised plan to v0.3 and closed all four v2 findings while preserving owner, gates, Phase 4a boundary, and P0-P6 order | feature brief; `docs/workflow/features/phase4a-control-plane-core/{design.md,api.md,test.md,dev_log.md}`; no review report/product/ADR/implementation-plan/dashboard/Git write | `NEEDS_REVIEW`; executable P2 identity prerequisite, exact sync wire, exact command attempt recovery, and corrected crypto/enrollment gates frozen; no self-approval | `feature-review v3` |
| 2026-07-21 02:48 PDT | Codex (GPT-5) as feature-review | Independently reviewed v0.3, confirmed closure of all four v2 findings, and rechecked P0-P6 executability, security boundaries, rollback, and verification gates | `docs/reviews/phase4a-control-plane-core/2026-07-21-feature-review-v3.md`; this Status Panel and one Work Log row only; no commit | `REVISE`; the authoritative P4 snapshot cannot encode its declared Device resource and lacks canonical page/final digest construction | `feature-plan` |
| 2026-07-21 02:53 PDT | Codex (GPT-5) as feature-plan | Revised plan to v0.4 and closed the sole v3 snapshot finding without changing P0-P6 order or Phase 4a scope | feature brief; `docs/workflow/features/phase4a-control-plane-core/{design.md,api.md,test.md,dev_log.md}`; no review report/product/ADR/implementation-plan/dashboard/Git write | `NEEDS_REVIEW`; Account-first typed resources and exact snapshot manifest/page/final integrity, replay, expiry, and commit gates frozen; no self-approval | `feature-review v4` |
| 2026-07-21 02:59 PDT | Codex (GPT-5) as feature-review | Independently reviewed v0.4, verified closure of the sole v3 snapshot finding, and regressed P0-P6 contracts, ordering, security boundaries, rollback, and gates | `docs/reviews/phase4a-control-plane-core/2026-07-21-feature-review-v4.md`; this Status Panel and one Work Log row only; no commit | `APPROVED`; repository-valid ready-to-build state with no material finding; Provider Gate none and Security Gate remains open | `feature-build P0` |
