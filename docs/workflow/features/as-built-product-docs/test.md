# Test plan: As-built product documentation authority

## Structural acceptance matrix

| Requirement | Command or inspection | Expected evidence |
|---|---|---|
| canonical path | `test -f docs/PRODUCT.md && test ! -e PRODUCT.md` | exactly the planned canonical path exists; no root duplicate |
| ledger path and coverage | inspect `docs/reviews/as-built-product-docs/claim-ledger.md` | fixed path exists; every named source surface heading, each product capability-matrix row, and every newly changed product-facing support/trust statement maps through a stable `CLM-###` row to destination heading(s) or an explicit conflict ID |
| ledger retention | inspect superseded rows and `Unresolved conflicts` | rows are appended/superseded, never deleted by prose rollback; an explicit `none` or retained conflict list is present |
| authority map | inspect `docs/PRODUCT.md` and claim ledger | implementation plan, feature logs/verdicts, compatibility evidence, and security evidence have distinct roles |
| status vocabulary | inspect every capability-matrix row and derived summary | each is exactly planned, preview, supported, experimental, unsupported, or unknown; a preview scope literally says `source-built`; no implied upgrade from phase/CI/build/dashboard state |
| evidence binding | trace each substantive claim to ledger and authority paths | rows carry `path@full-commit`, exact scope/date/state/fallback; Provider positive claims include a current producer receipt; trust statements retain source and residual risk |
| local links | `npm run ci:links` | all local file/anchor links pass after additions and rewrites |
| project structure | `npm run project:verify` | workflow/dashboard structural contracts pass; result is recorded only as a structural check |
| diff hygiene | `git diff --check` and exact staged-file review | no whitespace errors; no unrelated file is staged |

## Provider receipt matrix

| Claim type | Reviewer | Required proof | Failure handling |
|---|---|---|---|
| Provider capability/version/platform support | prior provider-owned `feature-verify` verdict or completed `provider-spike` decision | exact `docs/PROVIDER_COMPATIBILITY.md` row; producer path(s) pinned in `provider_receipt`; version/platform/capability/result/fallback/date match ledger scope | P2 may publish only unknown/planned/unsupported; feature-plan opens a provider spike for a needed positive result |
| account/auth/usage/session/PTY semantics | same immutable provider receipt producer | version/platform-bounded live or reproducible evidence, full revision, and fallback | remove broad wording; do not infer from configuration or CI |
| cross-platform conclusion | same immutable provider receipt producer | distinct receipt evidence for each claimed platform | retain the unverified platform as unsupported/unknown |

## Security fact-review matrix

| Claim type | Reviewer | Required proof | Failure handling |
|---|---|---|---|
| credential grant, revocation, or copied-secret boundary | independent `security-review` | threat model, ADR, and reviewed implementation evidence | restore explicit future-use-only revocation and residual risk |
| Passkey, enrollment, pinning, E2EE, or device-key boundary | independent `security-review` | authoritative protocol/ADR/threat-model wording | remove any inference that authentication equals decryption authority |
| secrecy/logging/terminal-content boundary | independent `security-review` | data classification, threat model, and implementation evidence | narrow statement and name planned/unknown scope |

## Freshness and failure scenarios

1. Set the inspectable fixture ledger header to `Verified on: 2026-07-29` and
   `Snapshot as of: 2026-06-28` (31 days). If a `preview`, `supported`, or
   `experimental` row or its product matrix destination remains positive, the
   inspection fails. Passing output changes each affected row/destination to
   `unknown`, records `evidence_state=stale`, and names `snapshot refresh`.
2. Set a positive Provider row's `evidence_date` to `2026-04-29` with
   `Verified on: 2026-07-29` (91 days), while retaining a matching pinned
   receipt. The inspection fails if the row remains `supported` or
   `experimental`. Passing output is `class=unknown`,
   `evidence_state=stale`, and `fallback_or_gate=Provider Gate: provider
   evidence refresh`; the Provider Gate remains open. Repeat with a scope
   mismatch and with an unreachable `path@full-commit`; both have the same
   expected downgrade.
3. For a missing positive Provider receipt, verify that P2 is blocked from
   publishing a positive class, a ledger conflict/gate names the provider-owned
   spike, and no documentation phase marks that gate resolved. A provider-spike
   `GATE_RESOLVED` decision plus updated compatibility row may be recorded only
   in a later ledger revision.

## Manual reconciliation scenarios

1. Start at README, follow the product-document link, then follow a current
   capability to its feature evidence. Confirm the route never turns a target
   plan or structural check into a support claim.
2. Compare every Provider/platform row in `docs/PRODUCT.md` with the
   compatibility matrix. Confirm version, platform, result, fallback, and
   remaining gate match exactly.
3. Compare all credential, Passkey, E2EE, revocation, and remote-control text
   with the threat model and relevant security review. Confirm no residual risk
   or explicit user confirmation requirement was omitted.
4. Compare README, user guide, architecture, data model, provider adapter,
   compatibility, threat model, and roadmap to the fixed ledger. Confirm every
   covered source heading maps to each resolved destination or retained
   conflict, and each surface either uses resolved wording or states its
   narrower authority.
5. Confirm P4 contains only check results and trace material. After final
   `feature-verify` reaches `READY_TO_SHIP`, inspect the independent
   `docs/reviews/as-built-product-docs/<date>-security-review.md` and feature
   log update before any ship step; P4 itself cannot be security acceptance.
6. Simulate a missing compatibility artifact, contradictory feature verdict,
   stale snapshot/Provider date, unreachable source revision, and broken local
   link. Each must fail acceptance or force the specified visible
   `unknown`/open-gate state; none may pass by prose-only review.
7. Confirm documentation includes no credentials, private account identifiers,
   raw auth output, cookies, terminal content, or unsafe copy/paste commands.

## Evidence boundaries

Green `project:verify`, `ci:links`, or rendering checks prove only document and
workflow structure. Provider and security review evidence proves only the
bounded claim it evaluates. A documentation feature does not prove a product
release, deployment, remote integration, or platform acceptance.
