# Phase 4a P1 Feature Verification v3

- Date: `2026-07-21`
- Role: `feature-verify`
- Plan version: `v0.7`
- Owner module: `control-plane`
- Reviewed transition: `FEATURE_DEV / READY_FOR_VERIFY`
- Exact committed head: `6bbad01f17db7a40f0dc43bc45a41738f302640b`
- Clearing chain after v2 verdict: `a9e5eec..6bbad01`
- Pull request: `#32`
- Verdict: `VERIFIED`

## Conclusion

P1 is independently verified and may advance to P2. All four findings from the
first verification, the Windows backup finding from v2, and both Windows
process-launch/install-layout findings discovered during v3 are closed on one
exact committed head. Local hostile, full, race, deterministic-generation,
typed-client, clean-Web, license, and structural gates pass. PR #32 reports all
ten protected checks successful at the same SHA, including a native Windows
full scaffold run that passes the owner/DACL tests, prior-schema backup,
cross-platform process runner, deterministic generation, API licenses, Web,
and Desktop check/build chain.

No P2+ runtime handler, authority-bearing table, or side effect was introduced.
P1 verification does not close the feature's final Security Gate and does not
authorize shipping, merging, or release.

## Findings

None.

## Windows backup and private-path closure

- `Store.Backup` still creates the backup with `VACUUM INTO`, applies
  `protectPrivateFile`, performs a real durable `Sync`, closes the handle, and
  then runs SQLite `integrity_check` and `foreign_key_check` before returning.
- The only backup change is the minimum Windows-compatible writable open mode:
  `os.OpenFile(path, os.O_RDWR, 0)`. The test independently opens the protected
  backup read-write, syncs it, closes it, re-verifies private permissions, then
  verifies integrity, foreign keys, schema version, and preserved data.
- Windows permission code remains fail closed: exact current-logon owner,
  protected DACL, one explicit allow ACE, current-logon SID, and exact
  `FILE_ALL_ACCESS`. Product-backed positive fixtures, the unprotected-DACL
  negative, Unix broad-directory/symlink negatives, and bounded concurrent
  first creation remain present.
- Exact-head Windows job `88765596965` passes both its initial full Go suite and
  the repeated Go suite inside `scaffold:verify`. This directly closes the
  owner/DACL and prior-schema backup failures seen at earlier heads.

## Windows process-runner and toolchain closure

### No-shell execution boundary

- All new child processes are represented as an executable plus a distinct
  argument array and reach `execFileSync` through one wrapper that overwrites
  any caller option with `shell: false`.
- The runner never launches a shell, `cmd.exe`, PowerShell, or `pnpm.cmd`.
  `PNPM_HOME/bin/pnpm.cmd` is only canonicalized and checked as a contained
  regular-file marker for the active action layout; its contents are not parsed
  and it is never selected as an executable.
- Windows pnpm execution uses `process.execPath` with the uniquely validated
  `pnpm.cjs` as argument zero. Metacharacter-bearing arguments remain one
  literal argument in the execution regression.

### Pinned active-pnpm resolution

- The expected pnpm version is derived from the repository's exact
  `packageManager: pnpm@10.23.0` pin; an unpinned value fails module loading.
- Windows requires an absolute, canonical `PNPM_HOME`, a contained active
  `bin` marker, and a contained canonical `global/v11` root. Only strictly
  named global install records are examined.
- Each install record must declare the exact pinned pnpm dependency. Its
  canonical package root must remain within the canonical global root or the
  exact versioned store root under `PNPM_HOME`; escapes fail closed.
- The package must be named `pnpm`, have exact version `10.23.0`, declare
  `bin.pnpm = bin/pnpm.cjs`, and resolve to a contained regular `pnpm.cjs`.
  Canonical candidates are deduplicated and exactly one is required.
- The eight protected runner tests model the real stale bootstrap v11 plus
  active action-installed v10 layout, actually execute the resolved v10 CLI,
  and reject missing/relative homes, wrong package metadata, trusted-root
  escape, ambiguous canonical installs, shell enablement, and argument joining.

### Deterministic generator

- The generator no longer invokes a package manager. It executes the locked
  workspace `node_modules/openapi-typescript/bin/cli.js` directly through the
  current Node runtime and a separate argument array.
- `openapi-typescript` remains exactly `7.13.0` in package metadata and the
  frozen lockfile. `api:verify` generates Go and TypeScript twice into a new
  temporary directory and byte-compares both runs with checked-in artifacts.
- Exact-head native Windows logs show runner tests 8/8, both TypeScript
  generations, deterministic byte verification, and `api:licenses` completing
  before the remaining scaffold gates.

## Prior-finding regression

### Clean Protocol/Web topology

The Protocol package remains private and source-exported, and Web consumes it
through `workspace:*`. A truly fresh exact-head clone with no `node_modules`,
`dist`, or `target` completed frozen install, API verification/licenses, Web
typecheck/build, and scaffold structure while Protocol `dist` was absent before
the first Web check. Native Ubuntu, macOS, and Windows full scaffolds also pass.

### Enrollment pre-auth and generated requiredness

- OpenAPI remains at 65 operations and 270 schemas. Exactly seven Enrollment
  operations retain sole `EnrollmentPreAuth` security and the exact timestamp,
  nonce, content-digest, and enrollment-signature header refs.
- Runtime tests pass exact Enrollment authorization and signed headers for GET
  and mutation, cookies and conditional browser CSRF, authorization-class
  mutual exclusion, operation coverage, and path/body Enrollment ID matching.
- Compile negatives continue to reject missing required body/path/query groups;
  the unresolved-path runtime negative rejects before fetch. Protocol tests are
  8/8.

### Foundation-only boundary

The full Go suite preserves the 65-operation route inventory, exactly three
mounted foundation handlers, 62 P2+ JSON 404 responses without side effects,
strict/hostile transport and configuration behavior, migration ledger and
checksum rules, private database/sidecar paths, verified backup, and license
boundaries.

## Independent local evidence

All commands below passed; final Go and focused checks ran directly at exact
head `6bbad01f17db7a40f0dc43bc45a41738f302640b`.

- `npm run api:verify`: process runner 8/8; deterministic checked-in Go/TS
  artifacts pass.
- `npm run api:licenses`; `npm run ci:licenses`; `npm run project:verify`:
  pass.
- `pnpm --filter @multi-agent-desk/protocol test`: 8/8 pass.
- `pnpm --filter @multi-agent-desk/web check`: pass.
- Fresh no-output clone, frozen install, `api:verify`, `api:licenses`, Web
  check/build, and `scaffold:structure`: pass during the v3 exact-head chain;
  final changes are limited to the runner and its tests, and final native clean
  scaffolds pass on all three platforms.
- `go test -count=1 ./...`: pass.
- `go test -race -count=1 ./internal/controlplane ./internal/transport`: pass.
- `go vet ./...`: pass.
- Twenty repetitions of prior-schema backup, concurrent migration,
  busy/corrupt/future/partial storage, and cancelled-context/no-listen tests:
  pass.
- `git diff --check` for each scoped clearing and the final combined verdict:
  pass.

During this independent run, native evidence correctly rejected two interim
heads rather than being masked by local simulation: `3b2aa28` could not launch
`pnpm` from Windows Node, and `45c7da5` assumed stale sibling package metadata.
Both are superseded by, and specifically regressed at, the final exact head.

## Exact-head remote checks

PR #32 reports head `6bbad01f17db7a40f0dc43bc45a41738f302640b`.

- CI run `29869408994`:
  - project verify `88765603068`: `SUCCESS`
  - build Ubuntu `88765596972`: `SUCCESS`
  - build macOS `88765596945`: `SUCCESS`
  - build Windows `88765596965`: `SUCCESS` (full scaffold, 7m05s)
- Governance run `29869408966`:
  - license `88765596746`: `SUCCESS`
  - DCO `88765596731`: `SUCCESS`
  - link `88765596722`: `SUCCESS`
- E2EE vectors run `29869409039`:
  - Ubuntu `88765597051`: `SUCCESS`
  - macOS `88765597016`: `SUCCESS`
  - Windows `88765597010`: `SUCCESS`

All 10/10 checks are completed successfully on the same exact SHA.

## Scope and security boundary

The verified implementation remains P1-scoped. Provider Gate remains `none`.
Security Gate remains open for later bootstrap, authentication/recovery, Device
identity and pinning, signed requests, sync/commands, and Web-origin behavior.
The next legal lifecycle action is `feature-build P2`; it must complete only P2
and stop again at `READY_FOR_VERIFY`.
