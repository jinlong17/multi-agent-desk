# Feature Review v8: Phase 4a Control Plane Core

- Date: `2026-07-22`
- Role: `feature-review`
- Plan version: `v0.8`
- Owner module: `control-plane`
- Reviewed state: `FEATURE_DEV / NEEDS_REVIEW`
- Exact reviewed head: `8de93d53b46d694dcf981a541588785a88e77a32`
- P2 checkpoint inspected: `fc2a38e9fb3802015c687f37c751bc3d807c7d78`
- Verdict: `REVISE`

## Conclusion

Plan v0.8 correctly adopts the operator-approved macOS-first boundary and
closes the CSRF and browser-session revision ambiguities with consistent,
executable contracts. It also defines a credible secret-nonreplayable
idempotency design: thirteen P2 mutation endpoints are classified, only the
transaction winner can return a cookie/CSRF/recovery plaintext, and every
restart/lost-response path has a public receipt and safe next action.

The idempotency contract is not yet decision-complete, however. Its normative
scope/request formulas and its mandatory test semantics disagree, and the
strict migration's closed operation discriminator omits the four
ceremony-begin values that must be persisted. A P2 writer would have to invent
whether keys are reusable across endpoint/actor scopes, whether request bodies
are raw-byte or canonical-JSON identities, and which exact values enter the
database constraint. Because these choices affect replay isolation, database
uniqueness, OpenAPI errors, and concurrency tests, P2 cannot resume until
`feature-plan` resolves them consistently across `design.md`, `api.md`, and
`test.md`.

No other material finding remains. The checkpoint reconciliation scope,
schema/OpenAPI/generated-client delta, browser receipt rules, rollback, and
P3+ isolation are precise. The Provider Gate remains none and the final
Security Gate remains open.

## Ranked findings

### 1. [P1] Idempotency key scope and request identity contradict the mandatory test

- `api.md:1673-1692` makes `principalClass`, principal digest, method, canonical
  path, and key part of `scopeDigest`; changing endpoint or actor therefore
  produces a different primary scope and is not a same-scope key reuse.
- `test.md:711-717` instead requires the same key with any different scope or
  actor to reject. It also requires the request digest to contain the
  authenticated actor class, although the API places actor identity in the
  scope rather than the request digest.
- The test requires a canonical strict-JSON body identity, while the normative
  request formula uses `contentSHA256Raw` and defines no JSON canonicalization
  transform for this digest.

Required correction: choose and freeze one rule. Either keys are scoped by
principal/method/path and may be independently reused outside that scope, or a
separate uniqueness domain rejects cross-scope reuse. Then make the formula,
database keys/indexes, error behavior, and tests identical. Explicitly define
whether the body component is raw-content SHA-256 or a named canonical JSON
encoding and place actor identity in exactly one frozen domain.

### 2. [P1] The strict idempotency operation discriminator is incomplete

- `api.md:1694-1705` requires a closed `operation` column and requires
  ceremony-begin rows for bootstrap options, Passkey login options, Passkey
  registration options, and UV options.
- `AuthOperationReceiptV1` at `api.md:1712-1725` enumerates only finish/public
  mutation values. No authoritative enum names the four begin-operation values
  that the strict migration, store, and restart cleanup must persist.

Required correction: define one complete storage discriminator for all
thirteen keyed P2 operations, or explicitly split begin and completion
discriminators/tables. Bind every endpoint-matrix row to one exact stored value
and add schema/generation/migration tests for the complete enum.

## Confirmed closures

### Restart-safe CSRF

- The raw value is exactly HMAC-SHA-256 keyed by the raw 32-byte session token
  over a versioned frame binding canonical origin, session ID, and generation.
- Storage is digest plus generation only; restart reconstruction, constant-time
  submitted/derived/digest comparison, corruption revocation, full-session
  rotation, and same-session generation CAS are consistent across brief,
  design, API, tests, and rollback.
- The checkpoint's session-ID-only HMAC and random-then-overwritten allocation
  are explicitly named as P2 clearing work rather than accepted evidence.

### One-time result safety

- All thirteen P2 POST/DELETE mutations and all five GET reads are exhaustively
  classified. Ceremony begin is same-boot replay only; session/recovery secret
  results are first-winner-only; logout/delete replay only public bodies and a
  non-secret clear-cookie directive.
- Bootstrap, login, registration, UV, recovery login, and recovery-code rotation
  have executable lost-response next actions without persisting Set-Cookie,
  raw CSRF, challenges, proofs, or Recovery Codes.
- The two findings above concern exact key/request/storage identities, not the
  secret-nonreplay safety policy itself.

### Item-authoritative browser sessions and Passkeys

- `revision` and `activityRevision` are separate per-item CAS domains; lists
  expose no maximum or collection revision.
- The five-minute touch window changes only activity state and uses half-open
  absolute/idle validity. Revoke/delete increments the selected state revision
  once, stale revoke returns bounded `session_revision_conflict`, and Web must
  refetch and obtain explicit confirmation.
- Counter-regression and Passkey deletion session revocation, current-cookie
  clearing, concurrency, restart, and exact-boundary tests are specified.

### macOS-first support and Provider isolation

- macOS arm64 is the stable product row. Current Chrome and Safari must execute
  real registration/login/recovery/replacement/delete/logout on the frozen P2
  SHA and origin; Safari additionally requires real Touch ID/platform Passkey.
- Windows is Experimental with compile/build/generated-contract/native DACL
  gates only. Ubuntu retains repository compile/build only. Windows browser
  acceptance is deferred to Phase 6 or a later stable-support lifecycle unit.
- Claude is outside Phase 4a. The only retained later v0.1 boundary is macOS
  existing-subscription interactive PTY, with no API key, print/`-p`, dollar
  budget, usage credits, or Linux/Windows Claude acceptance.
- The older project-wide support wording is truthfully routed to a separate
  `project-system` documentation action before Phase 6/release claims; it does
  not authorize P2 to make those claims.

### Checkpoint reconciliation, rollback, and phase isolation

- The exact `fc2a38e` implementation confirms the declared gaps: provisional
  CSRF derivation, collection-max list revisions, disabled revoke UI, and
  generic response idempotency remain and are not represented as verified.
- P2 explicitly owns amendments to the unverified server migration, OpenAPI,
  generated Go/TypeScript, runtime client, services/store, Web conflict flow,
  tests, and macOS receipt harness while preserving verified P0/P1 and leaving
  every P3+ route unimplemented.
- Whole-snapshot backup/restore, digest/FK/private-permission validation,
  one-time-secret nonrecoverability, and prior-binary rollback boundaries are
  explicit. Migration ownership remains 0008/0009/0010/0011 across P2-P5.

## Evidence and checks

- Read the complete current AGENTS/CLAUDE governance, implementation plan,
  workflow, module registry, feature-review role, Feature Brief, design, API,
  test plan, state authority, and Feature Review v7.
- Verified a clean local/remote branch at exact head
  `8de93d53b46d694dcf981a541588785a88e77a32`; v0.8 changes only the five
  authorized planning/state artifacts above checkpoint `fc2a38e`.
- Inspected the checkpoint migration, CSRF/session store and handlers, list/
  revoke Web behavior, OpenAPI/generated delta locations, and phase route
  inventory only to validate the clearing scope; no implementation was changed.
- `npm run workflow:verify`: pass (`agents=10`, `skills=3`, `docs=17`,
  `edges=20`, `statuses=15`).
- `npm run ci:static`: pass (seven Actions checks, fifteen pinned actions,
  CODEOWNERS).
- `npm run ci:fixtures`: pass.
- `npm run ci:links`: pass (`markdown_files=313`).
- `npm run ci:licenses`: pass (`pnpm_groups=6`, `cargo_packages=418`).
- `git diff --check`: pass before verdict persistence.
- Local `dashboard:verify` reports only `generated commit is stale` at the new
  documentation head; no dashboard file was mutated by this verdict. PR #32 is
  bound to the exact reviewed head and all ten checks are successful, including
  `project-verify`, Ubuntu/macOS/Windows builds and vectors, license, DCO, and
  link-check. This generated-fact refresh is not a P2 contract finding.

