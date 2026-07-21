# Phase 4a P1 Feature Verification

- Date: `2026-07-21`
- Role: `feature-verify`
- Plan version: `v0.7`
- Owner module: `control-plane`
- Reviewed transition: `FEATURE_DEV / READY_FOR_VERIFY`
- Exact committed head: `467067002f0b14716ef0720cb0f6ed455458936c`
- Diff: `e1fe69dc4950bf5822b477d6cd55ce2fb0efd9d8..467067002f0b14716ef0720cb0f6ed455458936c`
- Pull request: `#32`
- Verdict: `BLOCKED`

## Conclusion

P1 cannot advance to `VERIFIED`. The checked-in OpenAPI authority and generated
artifacts satisfy the local deterministic generation, operation/schema
inventory, reference-completeness, strict-object, foundation-only runtime,
storage, transport, and license checks. However, the exact PR head fails two
required native platform jobs, and independent source inspection found that the
first-party runtime client cannot express the frozen Enrollment pre-auth
contract. The protected three-platform gate and exhaustive typed-client
acceptance therefore both fail.

No P2+ runtime handler is mounted and no P2+ database table or authority-bearing
side effect was observed. This verdict does not reopen P0 and does not resolve
the open Security Gate.

## Findings

### 1. High — clean macOS workspace cannot typecheck the sole Web import

PR #32 CI run `29861479555`, native macOS job `88739004136`, fails at the Web
check:

```text
apps/web check: src/api/control-plane.ts(1,42): error TS2307:
Cannot find module '@multi-agent-desk/protocol' or its corresponding type declarations.
```

`@multi-agent-desk/protocol` exports only `dist/index.js` and
`dist/index.d.ts`, while its `check` script is `tsc --noEmit`. A clean recursive
check therefore reaches `apps/web` without a built Protocol `dist`. The local
Web check passed only after `pnpm --filter @multi-agent-desk/protocol test` had
created ignored `packages/protocol/dist/`; `git status --ignored` and
`git check-ignore` confirmed that hidden prerequisite. P1 requires clean-tree
Protocol/Web checks, so a pre-existing ignored build artifact is not valid
evidence.

### 2. High — native Windows private-path tests reject the runner owner

PR #32 CI run `29861479555`, native Windows job `88739004154`, fails ten
Control Plane test paths at the same guard:

```text
Windows path owner is not the current logon SID
```

Failures include config loading, the foundation-only route inventory, hostile
middleware, cancelled-context/no-listen, empty/restart/concurrent migration,
prior-schema backup, and busy/corrupt/future-schema cases. The implementation
verifies the path owner before using existing test/config directories. On the
real runner, the created fixtures do not satisfy that owner equality, so the
native acceptance suite cannot reach the DACL, migration, or runtime assertions.

The independent mask audit did not find an arithmetic mask defect:
`STANDARD_RIGHTS_ALL` is `0x001f0000`; the explicitly ORed file rights produce
`0x001f01ff`, matching Win32 `FILE_ALL_ACCESS`/SDDL `FA`. The observed failure
is specifically the owner/fixture/application semantics at
`privatefs_windows.go:74-76`, before the ACE-mask comparison at lines 93-95.
The clearing writer must make the product contract and real Windows fixtures
agree without weakening the current-logon/protected-DACL invariant, then obtain
a fresh native Windows receipt.

### 3. High — the exhaustive runtime client cannot send Enrollment pre-auth

The v0.7 contract requires candidate create/prove/activate/cancel/resume and
activation-package calls to carry `Authorization: Enrollment <id>`, timestamp,
nonce, content digest, and `X-MAD-Enrollment-Signature`; a Web candidate also
needs cookie/CSRF. The OpenAPI operations declare only the generic
`EnrollmentPreAuth` bearer scheme, and
`packages/protocol/src/control-plane-client.ts` exposes only
`bootstrapToken` and `device` header inputs. Its request builder has no
Enrollment bearer/signature branch.

Consequently the named client can enumerate all 65 operations but cannot make
a contract-valid Enrollment pre-auth call. Because the Web has exactly one
Control Plane import point and the client deliberately exposes no arbitrary
path escape hatch, later P3 code would have to bypass or modify the supposedly
complete P1 client. This violates the P1 acceptance requirement that every
P1-P6 operation be represented by an exhaustive first-party typed runtime
client.

### 4. Medium — required generated inputs are globally optional in the client

`ControlPlaneCallInput<K>` marks `body`, `path`, and `query` optional for every
operation, and every method accepts an omitted input. Required-body operations
therefore typecheck without a body and serialize `{}`; required path parameters
fail only at runtime. The generated types are used when fields are supplied,
but the client does not preserve requiredness from the generated operation
contract. The clearing writer should make operation inputs conditionally
required and add compile-time negative fixtures for missing required body/path
members.

## Independent evidence

### Contract and generation

- Complete read of repository governance, `feature-verify` role, implementation
  plan, plan v0.7 brief/design/API/test/dev log, and Feature Reviews v5-v7.
- Full diff inventory: 36 changed files, 120,370 insertions and 28 deletions;
  generated Go/TypeScript artifacts were assessed through their OpenAPI source,
  pinned generators, drift checks, compile/tests, and independent reference
  scans rather than blind line-by-line review.
- Independent OpenAPI traversal: `65` operations, `270` schemas, `1,337`
  local references, `0` unresolved references, and `0` object schemas missing
  explicit `additionalProperties` discipline.
- Two independent `npm run api:verify` executions: pass; each generates Go and
  TypeScript twice in a fresh temp directory and byte-compares both runs with
  checked-in artifacts.
- SHA-256: OpenAPI
  `a02bf6839c2b7a4b85ecc7275983d32b74bd3f2a4db47bdf2b0872a8e2232545`;
  generated Go
  `4cfc1890744f50afecfd00616850b124a0cbbee0c8679f30626302592ce81383`;
  generated TypeScript
  `2938d490c1aa9d2da4e2587606984482a4286accfd83c90843b1dffbbaa1d3c7`.
- `npm run api:licenses`: pass, including exact Go tool graph and
  `openapi-typescript@7.13.0`; `argparse` and `Python-2.0` are absent and
  `openapi-fetch` was not introduced.

### Runtime, storage, hostile input, and local regression

- `go test -count=1 ./...`: pass locally.
- `go test -race -count=1 ./internal/controlplane ./internal/transport`: pass.
- Twenty repetitions of the concurrent-migration, busy/cancellation, and
  cancelled-context/no-listen tests: pass locally.
- `go vet ./...`; native `go build ./cmd/...`; Linux amd64 and macOS arm64
  cross-builds; Windows amd64 Control Plane/transport compile probe using a
  no-exec harness: pass.
- Foundation route inventory: 65 contract operations, exactly three mounted
  health/readiness/version operations, and all 62 P2+ routes return JSON 404
  with no cookie, redirect, DB change, idempotency row, or success audit row.
- Existing hostile tests pass locally for strict configuration, duplicate
  query/body/compression/header bounds, cancelled context, UUIDv7, UTC time,
  Base64url, If-Match, cursor binding/tamper/expiry, duplicate/unknown/deep/
  oversized JSON, migration ledger/checksum/FK/future/partial/corrupt/busy/
  restart/concurrency, verified backup, Unix private paths, and symlinks.
- Protocol tests: 6/6 pass after explicitly building its ignored `dist`; Web
  check passes only in that non-clean condition, as Finding 1 records.
- `npm run project:verify`, `npm run ci:verify`, `git diff --check`, DCO, scope,
  dependency, generated-hash, and status checks pass locally. These structural
  passes do not override the native CI failures or client-contract findings.

### Exact-head remote checks

PR #32 reports head `467067002f0b14716ef0720cb0f6ed455458936c`.

- E2EE protocol vectors: Ubuntu `88739004086`, macOS `88739004099`, Windows
  `88739004088` — `SUCCESS`.
- Governance: license `88739004497`, DCO `88739004420`, link
  `88739004419` — `SUCCESS`.
- CI project verify `88739004137` — `SUCCESS`.
- CI macOS `88739004136` — `FAILURE`, Finding 1.
- CI Windows `88739004154` — `FAILURE`, Finding 2.
- CI Ubuntu `88739004182` was still `IN_PROGRESS` when this blocking verdict
  was persisted. Its eventual result cannot clear the two already-failed
  required platforms or the runtime-client findings.

## Scope and security boundary

The committed implementation remains within P1 paths. Runtime source mounts
only `/v1/healthz`, `/v1/readyz`, and `/v1/version`; server migrations create
only foundation metadata, idempotency, pre-user audit, and migration-ledger
tables. No bootstrap user, Passkey, Device, enrollment, sync, command execution,
WSS, HPKE, terminal, Approval, Credential Grant, Provider credential, or
Provider plaintext behavior was added. Provider Gate remains `none`; Security
Gate remains `open`.

## Clearing role

The original `feature-build P1` writer must:

1. make clean macOS/Ubuntu/Windows workspace checks resolve the sole Protocol
   import without an ignored prior build artifact;
2. reconcile Windows owner/DACL setup with the current-logon fail-closed
   contract and pass the real native test suite;
3. add exact typed Enrollment pre-auth headers/request construction to the
   OpenAPI/runtime client and preserve browser cookie/CSRF composition;
4. preserve generated required body/path/query semantics in method inputs and
   add compile-time negative coverage;
5. rerun deterministic generation, local hostile/full/race/vet/cross-platform
   gates, and all exact-head protected checks, then restore
   `READY_FOR_VERIFY` without starting P2.
