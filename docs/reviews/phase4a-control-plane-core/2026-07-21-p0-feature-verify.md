# Feature verification: `phase4a-control-plane-core` P0

- Date: 2026-07-21
- Role: `feature-verify`
- Phase verified: `P0 — Contract freeze`
- Owner module: `control-plane`
- Baseline: plan commit `a2f14f9` plus the 13-file uncommitted P0 diff
- Verdict: `BLOCKED`

## Conclusion

P0 is not independently verified. The committed positive vector passes and the
Go and TypeScript outputs are byte-identical, but an additional strict-time
probe demonstrates that the TypeScript attestation codec accepts invalid
calendar timestamps that Go rejects. The TypeScript PoP verifier also compares
the HMAC with `Buffer.equals` despite the approved contract forbidding
non-constant-time proof checks. Finally, the revised vector has no fresh native
Ubuntu or Windows receipt, while P0 acceptance explicitly requires Linux,
macOS, and Windows execution.

The remaining P0 scope is otherwise correct: the full 32-byte pin digest and
120-bit six-group presentation are distinct, both subject key digests are
signed and recomputed, the exact X25519/HKDF/HMAC/Ed25519 values agree, both
storage-assertion mutations and prior E2EE negatives reject, the portable Vault
and Phase 4a/4b/5 boundary is consistent, dependency provenance matches the
record, and no product package, migration, lockfile, runtime dependency, or
`openapi-fetch` import was added.

## Scope and classification

```text
Owner: control-plane
Impacts: security, core, web, desktop, project-system
Workflow: FEATURE_DEV / P0
Provider Gate: none
Security Gate: open
Changed scope: 13 documentation/vector files only
Product/runtime/migration/dependency-lock changes: none
```

The complete 13-file diff from `a2f14f9` was inspected. This verifier changed
only this report and the target `dev_log.md` verdict surfaces.

## Acceptance results

| P0 criterion | Result | Independent evidence |
|---|---|---|
| Full pin digest and six-group presentation | PASS | digest decodes to 32 bytes; fingerprint is `C42Q-PA5I-GRM5-7XZ5-NDZT-ZEHX`; case/unhyphenated normalization passes and altered/invalid/23/25/full-hex/truncation misuse rejects |
| Typed attestation, both full key digests, signature | **FAIL** | committed vector passes, but hostile `issuedAt=2026-02-30T16:00:00Z` / `expiresAt=2026-02-30T16:10:00Z` is rejected by Go and accepted by TypeScript |
| Exact X25519/HKDF/HMAC/Ed25519 PoP and mutations | **FAIL** | bytes and negative vectors agree, but TypeScript verifies the exchange HMAC with non-constant-time `Buffer.equals` |
| Prior E2EE vectors and negatives | PASS locally | wrap/payload AAD, wrong pin, cross-peer open/forge, nonce/sequence, replay window, and old-root rotation negatives remain rejected |
| Portable Vault and 4a/4b/5 scope truth | PASS | authoritative documents consistently use a portable-Vault-v1 Daemon anchor; WSS/HPKE/Terminal/Approval remain 4b and Credential Grant/OS wrapping remain 5 |
| Dependency provenance/license/toolchain | PASS for P0 | exact module origins, commits, Go sums, Go/toolchain lines, licenses, npm integrity, and current repository license gate match the record; manifests/locks are unchanged |
| Native Linux/macOS/Windows revised-vector execution | **FAIL (evidence)** | macOS passes; remote branch has no `spike-e2ee.yml` run and `gh run list` returns `[]`, so Ubuntu and Windows are unverified |
| P0 scope/no product behavior | PASS | changed-path allowlist finds no `cmd`, `internal`, `apps`, `packages`, `api`, migration, root manifest, or lockfile change |

## Findings

### [P0] TypeScript accepts non-existent RFC3339 calendar timestamps

The approved API requires strict UTC RFC3339 times, and the typed attestation
contract is intended to reject malformed timestamps consistently. The
TypeScript codec checks a shape regex and then calls `Date.parse`; JavaScript
normalizes some invalid calendar values. The exact hostile vector with
`2026-02-30T16:00:00Z` and `2026-02-30T16:10:00Z` produces:

```text
go_status=1  -> panic: invalid attestation lifetime
ts_status=0  -> complete vector output accepted
```

This breaks strict typed validation and cross-language behavior for signed
attestation bytes. The P0 writer must add a strict component/round-trip UTC
RFC3339 validator in TypeScript and shared Go/TypeScript negative vectors for
invalid calendar dates and hour `24`, then rerun byte parity.

### [P1] TypeScript exchange-proof comparison is not constant-time

`docs/spikes/e2ee/typescript/validate.mjs:366` uses
`Buffer.from(expectedExchangeProof).equals(Buffer.from(exchangeProof))`.
The approved API contract at `api.md:536-537` explicitly forbids
non-constant-time proof checks. The module already imports and uses
`timingSafeEqual` for pin and key-digest comparisons. The P0 writer must use a
length-safe `timingSafeEqual` path for the 32-byte HMAC and retain wrong-length
and mutation negatives.

### [P1] Fresh Ubuntu and Windows receipts are absent

`test.md:64-65` requires the revised harness on Linux, macOS, and Windows. The
branch is not present on `origin` and the branch-scoped `spike-e2ee.yml` run
query returned no runs. Existing historical receipt `29375956127` predates the
P0 vector/hash and cannot prove the revised contract. After the two code fixes,
the writer must obtain successful `e2ee-vectors-ubuntu`,
`e2ee-vectors-macos`, and `e2ee-vectors-windows` results for the exact fixed
commit.

## Commands and results

```text
node docs/spikes/e2ee/verify.mjs
PASS: resultSha256=34da114390dec3e6c089313fe0e7c44dcb46f94af57458ee85004e01c0fa5ca8

go run . ../vectors.json; node ../typescript/validate.mjs ../vectors.json; cmp
PASS: byte-identical outputs
PASS: SHA-256=53c206b6d9450028f57e50246ece147cf037281f2f6ff741d082a29fe8c7bfa5

hostile invalid-calendar attestation probe
FAIL: Go rejects; TypeScript exits 0 and accepts

source audit of TypeScript PoP verifier
FAIL: exchange HMAC uses Buffer.equals rather than timingSafeEqual

go test -count=1 ./...
PASS

node --check docs/spikes/e2ee/verify.mjs
node --check docs/spikes/e2ee/typescript/validate.mjs
PASS

npm run project:verify
PASS: workflow agents=10 skills=3 docs=17 edges=20 statuses=15;
      dashboard branch=codex/control-plane/phase4a-core dirty=13 phases=9

npm run ci:verify
PASS: Actions checks=7/actions=15; CODEOWNERS; positive/negative fixtures;
      305 Markdown links; current pnpm/cargo license graphs

go mod download -json <four exact module pins>; npm registry metadata query
PASS: recorded origins, commits, sums, module toolchains, license files, and
      openapi-typescript 7.13.0 MIT integrity match

git diff --check
PASS

changed-path, dependency-manifest, openapi-fetch, and high-confidence secret scans
PASS: 13 allowed files; no product/lock hit; no openapi-fetch manifest hit;
      no high-confidence secret hit outside deterministic vector seeds

git ls-remote --heads origin refs/heads/codex/control-plane/phase4a-core
gh run list --branch codex/control-plane/phase4a-core --workflow spike-e2ee.yml
BLOCKED evidence: no remote branch ref; [] runs
```

## Clearing conditions

The original `feature-build` P0 writer must:

1. make TypeScript reject invalid calendar/time values exactly as Go does and
   add cross-language hostile timestamp vectors;
2. compare the exchange HMAC with a length-safe constant-time primitive and add
   the wrong-length regression;
3. rerun all local gates and obtain the three native revised-vector CI jobs on
   one exact commit; and
4. restore `READY_FOR_VERIFY` with a new build evidence row for independent
   re-verification.

No P1 implementation may begin while this P0 verdict is blocked.
