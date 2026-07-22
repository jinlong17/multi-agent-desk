# Feature Review v9: Phase 4a Control Plane Core

- Date: `2026-07-22`
- Role: `feature-review`
- Plan version: `v0.9`
- Owner module: `control-plane`
- Reviewed state: `FEATURE_DEV / NEEDS_REVIEW`
- Exact reviewed head: `e8221a3b8a2ed535f21b4ab8bdf0fc3f29f74aed`
- P2 checkpoint retained: `fc2a38e9fb3802015c687f37c751bc3d807c7d78`
- Verdict: `APPROVED`

## Conclusion

Plan v0.9 closes both Feature Review v8 findings without reopening the approved
P2 product boundary. All thirteen P2 POST/DELETE operations now share one
server-global normalized-key digest and one fixed 24-hour uniqueness window.
Actor, operation, method, concrete path, RFC 8785/JCS canonical strict body, and
normalized If-Match occur in one request-identity frame. Any live-key identity
mismatch has one redacted error before touch, ceremony consumption, audit
success, or product side effect.

The migration, endpoint matrix, OpenAPI/generated contract, durable receipt,
cleanup/store dispatch, and mandatory tests share the same complete thirteen-
value `AuthIdempotencyOperationV1`. Header normalization, bodyless `{}`, exact
database keys/indexes/CHECK expectations, constant-time digest comparison,
restart behavior, bounded concurrent-loser behavior, non-sliding expiry, and
first-winner-only secret results are decision-complete. The P2 writer can
reconcile checkpoint `fc2a38e` to v0.9 without inventing protocol semantics.

No material finding remains. P0/P1 stay verified, Provider Gate stays none,
and the Security Gate stays open for its later independent review.

## Ranked findings

None.

## v8 finding closure

### 1. Global key domain and canonical request identity

- `NormalizedIdempotencyKeyV1` accepts exactly one header field, strips only
  HTTP SP/HTAB outer whitespace, preserves case and punctuation, and rejects
  comma/coalesced, non-ASCII, control, or out-of-bound values.
- `keyDigest` contains only the normalized key in its versioned frame and is
  the single server-global primary lookup across all thirteen P2 mutations.
- `CanonicalStrictJSONV1` validates the endpoint's strict schema before RFC
  8785 JCS serialization; bodyless mutations use exact ASCII `{}`. Invalid JSON
  does not create or look up idempotency state.
- One `requestIdentityDigest` binds server origin, the endpoint-mapped actor
  class and 32-byte actor identity, operation, method, resolved concrete `/v1`
  path, canonical-body digest, and normalized DELETE If-Match or empty value.
  Actor identity is not duplicated in the key or body domains.
- Lookup is global by `keyDigest`; fixed-length digests compare in constant
  time. Any actor/scope/request difference returns HTTP 409
  `idempotency_key_reused` before side effects and without identifying which
  field changed.

### 2. Complete stored-operation discriminator

- The endpoint matrix has exactly thirteen keyed rows and maps them one-to-one
  to `bootstrap_options`, `bootstrap_verify`, `passkey_login_options`,
  `passkey_login_verify`, `passkey_registration_options`,
  `passkey_registration_verify`, `passkey_delete`, `uv_options`, `uv_verify`,
  `recovery_verify`, `recovery_codes_rotate`, `logout`, and `session_delete`.
- The same closed enum is normative for migration CHECKs, storage/cleanup,
  OpenAPI and generated Go/TypeScript, runtime operation coverage, public
  receipt/status projections, and regression tests. There is no second
  finish-only enum.
- The exact storage contract uses global `key_digest BLOB(32) PRIMARY KEY`,
  independently UNIQUE `operation_id`, cleanup index
  `(expires_at,key_digest)`, and partial UNIQUE non-null `ceremony_id`; no
  composite scope key can permit cross-operation or cross-actor key reuse.

## Regression review

- The macOS-first boundary is unchanged: macOS arm64 requires real current
  Chrome and Safari journeys plus real Safari Touch ID/platform Passkey.
  Windows remains Experimental compile/build/native-DACL only, and Ubuntu
  remains repository compile/build only. No Windows or Ubuntu Claude product
  acceptance is claimed.
- Claude remains outside Phase 4a. The retained later smoke boundary is macOS
  existing-subscription interactive PTY only, with no API key, print mode,
  dollar budget, Usage Credits, or Linux/Windows claim.
- The v0.8 HMAC CSRF construction, secret-nonreplayable first-winner rules,
  public replay classes, item-authoritative Passkey/session revisions,
  coalesced activity revision, rollback/backup unit, and safe lost-response
  actions remain unchanged.
- The exact P2 checkpoint remains evidence rather than verification. v0.9 owns
  the migration/OpenAPI/generated/runtime/server/Web/test reconciliation and
  preserves verified P0/P1 bytes while every P3+ route remains outside the P2
  write set.

## Evidence and checks

- Read the complete current repository governance, workflow, module registry,
  feature-review role, v0.9 Feature Brief/design/API/test/state authority, and
  Feature Review v8. Classified ownership as `control-plane`, with secondary
  `security`, `core`, `web`, `desktop`, and `project-system` impacts.
- Verified a clean local branch exactly synchronized with its remote at
  `e8221a3b8a2ed535f21b4ab8bdf0fc3f29f74aed`. The v0.9 commit changes only the
  five authorized planning/state artifacts; no implementation, generated,
  dependency, dashboard, or implementation-plan file changed.
- Cross-checked the thirteen endpoint rows against the exact enum and scanned
  the five authorities for the retired P2 scope/raw-body/principal formulas;
  the API matrix matches all thirteen values and no retired P2 formula remains.
- `npm run workflow:verify`: pass (`agents=10`, `skills=3`, `docs=17`,
  `edges=20`, `statuses=15`).
- `npm run ci:static`: pass (seven Actions checks, fifteen pinned actions,
  CODEOWNERS).
- `npm run ci:fixtures`: pass.
- `npm run ci:links`: pass (`markdown_files=314`).
- `npm run ci:licenses`: pass (`pnpm_groups=6`, `cargo_packages=418`).
- `git diff --check`: pass before verdict persistence.
- Local `dashboard:verify` reports only `generated commit is stale` at the new
  documentation head; this verdict did not mutate generated or manual dashboard
  state. PR #32 is bound to the exact reviewed head; project, governance,
  license, link, and three-platform E2EE checks passed, while Ubuntu/Windows
  build jobs were still running at review persistence. Remote build completion
  is not a plan-contract approval claim.
