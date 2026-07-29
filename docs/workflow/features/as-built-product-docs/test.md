# Test plan: As-built product documentation authority

## Structural acceptance matrix

| Requirement | Command or inspection | Expected evidence |
|---|---|---|
| canonical path | `test -f docs/PRODUCT.md && test ! -e PRODUCT.md` | exactly the planned canonical path exists; no root duplicate |
| authority map | inspect `docs/PRODUCT.md` and claim inventory | implementation plan, feature logs/verdicts, compatibility evidence, and security evidence have distinct roles |
| status vocabulary | inspect every capability-matrix row and derived summary | each is explicitly target, preview, supported, experimental, unsupported, or unknown; no implied upgrade from phase/CI/build/dashboard state |
| evidence binding | trace each substantive claim to inventory and authority paths | Provider/platform statements include exact scope/date/fallback; trust statements retain source and residual risk |
| local links | `npm run ci:links` | all local file/anchor links pass after additions and rewrites |
| project structure | `npm run project:verify` | workflow/dashboard structural contracts pass; result is recorded only as a structural check |
| diff hygiene | `git diff --check` and exact staged-file review | no whitespace errors; no unrelated file is staged |

## Provider fact-review matrix

| Claim type | Reviewer | Required proof | Failure handling |
|---|---|---|---|
| Provider capability/version/platform support | owning `provider` reviewer | exact `docs/PROVIDER_COMPATIBILITY.md` row and linked evidence | mark unsupported/unknown or open provider spike |
| account/auth/usage/session/PTY semantics | owning `provider` reviewer | version/platform-bounded live or reproducible evidence and fallback | remove broad wording; do not infer from configuration or CI |
| cross-platform conclusion | owning `provider` reviewer | distinct evidence for each claimed platform | retain the unverified platform as unsupported/pending |

## Security fact-review matrix

| Claim type | Reviewer | Required proof | Failure handling |
|---|---|---|---|
| credential grant, revocation, or copied-secret boundary | independent `security-review` | threat model, ADR, and reviewed implementation evidence | restore explicit future-use-only revocation and residual risk |
| Passkey, enrollment, pinning, E2EE, or device-key boundary | independent `security-review` | authoritative protocol/ADR/threat-model wording | remove any inference that authentication equals decryption authority |
| secrecy/logging/terminal-content boundary | independent `security-review` | data classification, threat model, and implementation evidence | narrow statement and name planned/unknown scope |

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
   compatibility, threat model, and roadmap to the claim inventory. Confirm
   each surface either uses resolved wording or states its narrower authority.
5. Simulate a missing compatibility artifact, contradictory feature verdict,
   stale snapshot date, and broken local link. Each must fail acceptance or
   force a visible `unknown`/blocked state; none may pass by prose-only review.
6. Confirm documentation includes no credentials, private account identifiers,
   raw auth output, cookies, terminal content, or unsafe copy/paste commands.

## Evidence boundaries

Green `project:verify`, `ci:links`, or rendering checks prove only document and
workflow structure. Provider and security review evidence proves only the
bounded claim it evaluates. A documentation feature does not prove a product
release, deployment, remote integration, or platform acceptance.
