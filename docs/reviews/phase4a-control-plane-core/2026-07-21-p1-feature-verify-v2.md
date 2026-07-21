# Phase 4a P1 Feature Verification v2

- Date: `2026-07-21`
- Role: `feature-verify`
- Plan version: `v0.7`
- Owner module: `control-plane`
- Reviewed transition: `FEATURE_DEV / READY_FOR_VERIFY`
- Exact committed head: `54fc6258952a0bf85f2400578d188a871c4b07f6`
- Clearing diff: `53fcdc2..54fc625`
- Pull request: `#32`
- Verdict: `BLOCKED`

## Conclusion

The clearing change closes all four findings from P1 verification v1. A truly
fresh checkout resolves and builds the Web workspace without a Protocol
`dist`; the native Windows run now passes the current-logon owner/DACL fixture
boundary; the seven Enrollment pre-auth operations have exact generated and
runtime request construction; and required generated body/path/query semantics
are preserved at compile time and fail closed at runtime.

P1 nevertheless cannot advance to `VERIFIED`. The exact-head native Windows
job reaches the prior-schema backup test but fails when `Store.Backup` calls
`Sync` on a backup opened read-only. Ubuntu, macOS, the three E2EE vector jobs,
project verification, license, DCO, and link checks all pass on the same head;
Windows is the sole failing protected check. P2 must not start until the
original P1 writer clears this new native storage defect and returns a fresh
exact-head three-platform receipt.

## Finding

### 1. High — Windows verified backup cannot sync its read-only handle

PR #32 CI run `29864469590`, native Windows job `88749101666`, fails:

```text
--- FAIL: TestStorePriorSchemaBacksUpAndUpgrades
store_test.go:212: sync server backup: sync ...\\backups\\server-....sqlite: Access is denied.
```

The failure occurs after the fixture/DACL corrections, in the intended backup
upgrade path. `Store.Backup` creates the backup with `VACUUM INTO`, protects the
file, then opens it with `os.Open(path)` and immediately calls `file.Sync()`.
That read-only handle returns `Access is denied` on the real Windows runner.
This prevents the P1 verified-backup acceptance from passing on all three
native platforms. The repair must preserve the private-path DACL and durable,
verified-backup requirements; suppressing the sync or weakening the permission
check is not an acceptable clearing action.

## Prior-finding closure

### 1. Clean Protocol/Web topology — cleared

- Cloned exact head with `--no-hardlinks` into a new temp directory and
  confirmed no `node_modules`, `dist`, or `target` directory existed.
- `pnpm install --frozen-lockfile` completed and
  `packages/protocol/dist` remained absent before and after the first Web
  typecheck.
- `pnpm --filter @multi-agent-desk/web check`, the direct Web production build,
  `npm run web:check`, deterministic `api:verify`, license, layout, Go full
  tests, and native Desktop `cargo check` passed from that fresh checkout.
- Protocol exports `src/index.ts`, is `private: true`, and Web consumes it only
  through `workspace:*`; the actual Vite build bundles the source topology.
  This clears the clean-build failure without creating a publish contract or a
  second runtime client path.

### 2. Windows private-path fixtures and creation race — cleared

- Product code still requires exact current-logon ownership, a protected DACL,
  one explicit allow ACE, the current-logon SID, and exact `FILE_ALL_ACCESS`.
- Shared directory/file fixtures call `protectPrivateDirectory` and
  `protectPrivateFile`; the Windows positive and unprotected-DACL negative tests
  use those product helpers. The Unix broad-directory and symlink negatives
  remain.
- Concurrent first creation uses `O_CREATE|O_EXCL`; the losing opener waits only
  up to the bounded busy timeout for `verifyPrivateFile` to succeed.
- The new native Windows job passes every prior owner/DACL failure and proceeds
  to the distinct backup-sync failure above. Local race tests and 20 repeated
  concurrency/busy/cancellation cases pass.

### 3. Enrollment pre-auth construction — cleared

- Independent OpenAPI traversal finds exactly seven Enrollment operations:
  create, get, prove, activation-package get, activate, cancel, and resume.
  Each has sole `EnrollmentPreAuth` security and the ordered required
  timestamp, nonce, content-digest, and enrollment-signature parameter refs.
- The runtime client requires the distinct Enrollment input, emits exact
  `Authorization: Enrollment <id>`, `X-MAD-Timestamp`, `X-MAD-Nonce`,
  `X-MAD-Content-SHA256`, and `X-MAD-Enrollment-Signature`, includes browser
  cookies, conditionally requires CSRF for browser-candidate mutations, and
  sends no body for GET operations.
- Runtime tests pass for GET and mutation requests, bootstrap/device/enrollment
  mutual exclusion, missing Enrollment input, Enrollment/path/body ID mismatch,
  non-Enrollment rejection, conditional CSRF, and the exact seven-operation
  inventory.

### 4. Generated required input semantics — cleared

- Conditional types preserve required request bodies and required path/query
  groups. `@ts-expect-error` negative fixtures prove missing create-Profile body,
  missing get-Profile path, and a synthetic required query are rejected by
  compilation, while optional-input methods such as health remain callable
  without an input.
- The runtime unresolved-path negative rejects before fetch. Protocol's eight
  compiled tests pass, including all Enrollment and requiredness fixtures.

## Regression evidence

- Fresh-checkout frozen install, Web check/build, `web:check`, API deterministic
  regeneration, API licenses, layout, Go format/full test, and Desktop check:
  pass at exact head. The later redundant Desktop release-build repetition was
  stopped after the decisive Windows protected-check failure and is not used as
  passing evidence.
- Independent OpenAPI inventory: `65` operations, `270` schemas; generation
  verifier confirms checked-in Go/TypeScript byte equality and determinism.
- `pnpm --filter @multi-agent-desk/protocol test`: 8/8 pass.
- `go test -race -count=1 ./internal/controlplane ./internal/transport`: pass.
- Twenty repetitions of concurrent migration, busy/cancellation, and
  cancelled-context/no-listen coverage: pass.
- `go vet ./...`, `npm run project:verify`, `npm run ci:verify`, and
  `git diff --check 53fcdc2..54fc625`: pass.
- Full fresh-checkout Go tests retain the 65-operation route inventory, only
  three mounted foundation handlers, 62 P2+ JSON 404 responses without side
  effects, hostile transport/config/storage coverage, private paths, migrations,
  backup verification, and license gates on macOS. The exact Windows failure is
  recorded separately rather than masked by local success.

## Exact-head remote checks

PR #32 reports head `54fc6258952a0bf85f2400578d188a871c4b07f6`.

- CI project verify `88749101645`: `SUCCESS`.
- CI build Ubuntu `88749101697`: `SUCCESS`.
- CI build macOS `88749101725`: `SUCCESS`.
- CI build Windows `88749101666`: `FAILURE` — Finding 1.
- E2EE vectors Ubuntu `88749102397`, macOS `88749102368`, Windows
  `88749102369`: `SUCCESS`.
- Governance license `88749101266`, DCO `88749101249`, link
  `88749101367`: `SUCCESS`.

Thus 9/10 named exact-head checks pass, but the required native Windows build
does not.

## Scope and security boundary

The clearing diff remains P1-scoped. Runtime continues to mount only health,
readiness, and version; no P2+ authority-bearing table, handler, or side effect
was introduced. Provider Gate remains `none`; Security Gate remains open.

## Clearing role

The original `feature-build P1` writer must repair the Windows backup sync/open
mode while preserving private permissions and durable verified-backup behavior,
add a native-relevant regression, rerun the focused storage/full/race/vet gates,
and obtain a fresh exact-head 10/10 protected-check receipt before restoring
`READY_FOR_VERIFY`.
