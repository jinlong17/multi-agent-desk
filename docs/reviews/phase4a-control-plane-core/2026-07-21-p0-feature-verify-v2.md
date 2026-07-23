# Feature verification v2: `phase4a-control-plane-core` P0

- Date: 2026-07-21
- Role: `feature-verify`
- Phase verified: `P0 — Contract freeze`
- Owner module: `control-plane`
- Exact implementation/evidence head: `37d517bd6f9f5f70c8f21a9ac8e544b5a106a993`
- Verdict: `VERIFIED`

## Conclusion

P0 is independently verified. Both defects from the first verification are
closed: TypeScript now validates calendar/time components and microsecond
lifetime bounds consistently with Go, and exchange-proof HMAC verification now
uses a length-safe `timingSafeEqual` path. Independent hostile probes cover
impossible dates, non-leap February 29, hour 24, leap seconds, excessive
fraction precision, valid leap-day transition, the exact ten-minute boundary,
ten minutes plus one microsecond, wrong HMAC content, and 31/33-byte HMACs.

PR #32 independently reports all ten checks successful on the same exact head,
including native Ubuntu, macOS, and Windows E2EE vector jobs. Their logs each
emit result hash
`55bff1decd0b3419df4d43e32fe933e397a9167253c89f7a7d71552c178520f5`.
The direct local Go and TypeScript outputs remain byte-identical with SHA-256
`3ab9bbe3477ee64293d229c50b364b5c3721ae8190c6c443b33b2b585adf8dde`.

No material finding remains. P1 may begin under the approved v0.4 phase plan;
the final Security Gate remains open.

## Scope and state

```text
Owner: control-plane
Impacts: security, core, web, desktop, project-system
Workflow transition: READY_FOR_VERIFY -> VERIFIED
Provider Gate: none
Security Gate: open
Baseline plan: a2f14f9
Verified head: 37d517bd6f9f5f70c8f21a9ac8e544b5a106a993
Committed P0 diff: 14 documentation/vector/verdict-state files
Uncommitted input: dev_log receipt/status update only
Product/runtime/migration/dependency-lock changes: none
```

The complete current diff was inspected. This verifier changed only this v2
report and the target `dev_log.md` Status Panel, Evidence Ledger, and one Work
Log row.

## Prior finding closure

### Strict UTC RFC3339 parity — closed

`parseStrictUTCRFC3339` parses exact components, rejects invalid ranges and
calendar normalization, round-checks the UTC date components, represents the
instant in microseconds with `BigInt`, and enforces the ten-minute lifetime in
microseconds. Independent modified-vector probes produced:

| Probe | Expected | Go | TypeScript | Cross-language result |
|---|---:|---:|---:|---|
| `2026-02-30` | reject | reject | reject | parity |
| non-leap `2023-02-29` | reject | reject | reject | parity |
| hour `24` | reject | reject | reject | parity |
| second `60` | reject | reject | reject | parity |
| valid `2024-02-29T23:59:59.999999Z` boundary | accept | accept | accept | byte-identical |
| exactly ten minutes at microsecond precision | accept | accept | accept | byte-identical |
| ten minutes plus one microsecond | reject | reject | reject | parity |
| seven fractional digits | reject | reject | reject | parity |

The committed harness also asserts invalid calendar/hour rejection and valid
leap-day acceptance in both implementations.

### Length-safe constant-time HMAC verification — closed

TypeScript `verifyPop` now routes expected/candidate exchange proofs through
`constantTimeEqual`, which verifies equal lengths before invoking Node
`timingSafeEqual`. The actual PoP path rejects changed content, a 31-byte proof,
and a 33-byte proof in both Go and TypeScript. The old
`expectedExchangeProof.equals(exchangeProof)` path is absent.

### Native three-platform revised-vector evidence — closed

PR #32 is an open Draft targeting `main`; its head, the local head, the remote
branch, and all three workflow-run `head_sha` values are exactly
`37d517bd6f9f5f70c8f21a9ac8e544b5a106a993`.

| Workflow / check | Run | Job | Conclusion |
|---|---:|---:|---|
| E2EE `e2ee-vectors-ubuntu` | `29850227870` | `88700932340` | `success` |
| E2EE `e2ee-vectors-macos` | `29850227870` | `88700932375` | `success` |
| E2EE `e2ee-vectors-windows` | `29850227870` | `88700932424` | `success` |
| CI `project-verify` | `29850228526` | `88700973142` | `success` |
| CI `build-ubuntu` | `29850228526` | `88700973107` | `success` |
| CI `build-macos` | `29850228526` | `88700973080` | `success` |
| CI `build-windows` | `29850228526` | `88700973127` | `success` |
| Governance `license-gate` | `29850228178` | `88700933439` | `success` |
| Governance `dco` | `29850228178` | `88700933431` | `success` |
| Governance `link-check` | `29850228178` | `88700933374` | `success` |

All three native vector logs contain the expected current result hash and the
complete negative-case pass receipt.

## P0 acceptance results

| Criterion | Result | Evidence |
|---|---|---|
| Full 32-byte pin and six-group 120-bit presentation | PASS | Go/TS byte parity and presentation/truncation negatives |
| Strict typed attestation with both full key digests | PASS | signature/digest/schema/JCS/UUID/time/capability positives and negatives, including independent hostile time probes |
| X25519/HKDF/HMAC/Ed25519 PoP | PASS | shared secret, pop key, proofs and transcript byte-match; field/replay/restart/all-zero and HMAC content/length negatives reject |
| Prior Pairwise E2EE vectors | PASS | wrap/payload AAD, wrong pin, cross-peer open/forge, nonce/sequence, replay-window and old-root negatives remain green |
| Portable Vault and 4a/4b/5 truth | PASS | Daemon portable-Vault-v1 anchor; no 4a WSS/HPKE/Terminal/Approval/Grant claim |
| Dependency provenance/license/toolchain | PASS | exact recorded pins/integrity remain; all five provenance URLs return HTTP 200; repository and remote license gates pass |
| Linux/macOS/Windows revised vectors | PASS | exact-head jobs `88700932340`, `88700932375`, `88700932424` |
| P0 scope/no product behavior | PASS | no `cmd`, `internal`, `apps`, `packages`, `api`, migration, root manifest, or lockfile change; `openapi-fetch` absent |

## Commands and results

```text
node docs/spikes/e2ee/verify.mjs
PASS: resultSha256=55bff1decd0b3419df4d43e32fe933e397a9167253c89f7a7d71552c178520f5

go run . ../vectors.json; node ../typescript/validate.mjs ../vectors.json; cmp
PASS: byte-identical
PASS: SHA-256=3ab9bbe3477ee64293d229c50b364b5c3721ae8190c6c443b33b2b585adf8dde

eight independent UTC/lifetime modified-vector probes
PASS: all accept/reject outcomes match; accepted outputs are byte-identical

PoP output/source audit
PASS: wrong-content/short/long exchange proofs reject through verifyPop;
      length-safe timingSafeEqual is used

go test -count=1 ./...
PASS

node --check docs/spikes/e2ee/verify.mjs
node --check docs/spikes/e2ee/typescript/validate.mjs
PASS

npm run project:verify
PASS: workflow agents=10 skills=3 docs=17 edges=20 statuses=15;
      dashboard branch=codex/control-plane/phase4a-core dirty=1 phases=9

npm run ci:verify
PASS: Actions checks=7/actions=15; CODEOWNERS; fixtures; 306 Markdown
      links; current pnpm/cargo license graphs

curl -L <five dependency provenance URLs>
PASS: 200 for go-webauthn, oapi-codegen, kin-openapi, google/uuid,
      and registry.npmjs.org/openapi-typescript/7.13.0

gh pr view 32; gh api actions/runs/{id}; gh api actions/runs/{id}/jobs
PASS: exact head on PR/branch/three runs; 10/10 listed jobs completed success

gh run view 29850227870 --log
PASS: Ubuntu/macOS/Windows each emit current verifier hash

git diff --check a2f14f9..HEAD; git diff --check
PASS

changed-path, manifest/lock, openapi-fetch, DCO and high-confidence secret scans
PASS: 14 P0 documentation/vector/state files; no product/lock/openapi-fetch/
      secret hit; both P0 commits carry Signed-off-by
```

## Findings

None.

## Residual gates

- This is P0 contract/vector verification, not Control Plane runtime evidence.
- The Security Gate remains open for bootstrap, identity/pinning, signed
  transport, sync/commands, Web origin, and final residual-risk acceptance.
- PR #32 remains Draft; this verification does not authorize merge, release,
  deployment, or risk acceptance.
