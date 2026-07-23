# Bug verification: Phase 4a P2 process-lock receipt contract

## Verdict

`BLOCKED` for the scoped `phase4a-p2-process-lock-receipt-contract` repair.

The production process-lock implementation is unchanged and the new lock
binding itself passes its focused and full regression suites. However, the
same repair documents that every unknown `server.sqlite.*`, alternate-lock,
and backup-directory artifact fails closed while the runtime walker omits
directory objects from both the unexpected-path count and the post-snapshot
inventory. An empty unknown directory can therefore survive `scan` and
`finalize` without evidence. That semantic gap must be cleared before this
security receipt contract is shipped.

## Scope and ownership

- Owner: `control-plane` (high confidence).
- Secondary impacts: `security` receipt evidence and `project-system` CI.
- Audited files:
  `scripts/acceptance/p2-browser-receipt.mjs`,
  `scripts/acceptance/p2-browser-receipt.test.mjs`,
  `docs/workflow/features/phase4a-control-plane-core/p2-browser-receipt-template.md`,
  `docs/workflow/features/phase4a-control-plane-core/test.md`, and the target
  state authority.
- Base commit:
  `402925fae304056e34c9bc53c1543c268329fccb`; branch:
  `codex/control-plane/phase4a-core`.
- No implementation, test, plan, dashboard judgment, commit, push, browser,
  or P3 write was performed by this verifier.

## Verified process-lock boundaries

Inspection and regression evidence confirm all of the following:

- `internal/controlplane/process_lock.go` and
  `internal/controlplane/process_lock_test.go` have no worktree diff.
- Manifest schema v2 requires an exact absolute
  `serverProcessLockPath = databasePath + ".process.lock"`; it rejects missing,
  wildcard, alternate, cross-row, aliased, or hard-linked lock declarations.
- The declared server must expose exactly one numeric regular-file FD for the
  exact lock vnode with O_RDWR access. The lock must be an owner-owned `0600`,
  single-link, empty regular file. The Daemon may not hold that vnode.
- Global `lsof` holder evidence must contain exactly the declared server PID
  and FD. A real macOS diagnostic also showed the system's blank BSD `flock`
  status and enumerated two holders when a second process retained the same
  vnode, so the uniqueness gate rejects the extra holder.
- `lsof` whole-file `W` is accepted directly. When macOS reports blank lock
  status, a second O_RDWR process must fail to acquire a nonblocking shared
  whole-file `flock`; success or probe error rejects.
- The normalized lock binding is frozen into the server process context,
  re-collected by `scanEnvironment`, compared during finalize, and included in
  exact process-lock scan evidence.
- The exact lock is not part of `existingServerDatabaseFiles`; it is included
  in `runtime_residue`, read by `stableReadFile`, passed through all six secret
  detectors, tied back to the frozen vnode, and does not increment
  `serverBackupCount`.
- Unknown regular files named `server.sqlite.unknown`, `server.sqlite.bak`, or
  `server.sqlite.backup` are correctly rejected as unexpected runtime residue.

## Commands and passing results

1. `node --check scripts/acceptance/p2-browser-receipt.mjs` and the matching
   test-file syntax check passed.
2. Focused receipt run for manifest/context/scan/process-lock/process-binding/
   roundtrip gates passed 34/34.
3. `node --test scripts/acceptance/p2-browser-receipt.test.mjs` passed 56/56.
4. `npm run acceptance:p2-browser:test` passed 56/56.
5. `go test -count=10 -run '^TestProcessLockExcludesConcurrentServerAndMaintenance$' ./internal/controlplane`
   passed.
6. `npm run project:verify` passed: workflow `10/3/17/20/15`; dashboard
   generation and verification passed with exactly the five writer-owned dirty
   paths.
7. `npm run ci:verify` passed: 7 Actions checks, 15 pinned actions,
   CODEOWNERS, positive/negative CI fixtures, receipt 56/56, 325 links, 6 pnpm
   license groups, and 418 Cargo packages.
8. `git diff --check` and both Node syntax checks passed; the final dirty scope
   before verdict persistence was exactly the five writer-owned receipt/docs
   files.

## Finding

### [P1] Empty unknown runtime directories are invisible to the claimed fail-closed inventory

`walkPrivateTree` validates and recursively visits a directory but appends
nothing for the directory itself. Only files and allowed FIFOs become entries.
`scanEnvironment` computes `unexpectedRuntime` solely by filtering those
entries, and both initial and second snapshot digests are likewise derived only
from entry inventories.

Consequently, an empty directory such as any of the following under a row's
runtime root is not represented in `targetClasses`, `unexpectedPathCount`, or
`postSnapshotSha256`:

- `server.sqlite.unknown`
- `server.sqlite.bak`
- `server.sqlite.backup`
- an alternate process-lock directory
- an otherwise unknown backup directory

Adding, removing, or renaming such an empty directory between snapshots is
also invisible. The current negative matrix exercises regular files only.

This contradicts `test.md`, which says unknown `server.sqlite.*` and
backup-directory artifacts fail closed and that full post-scan inventory
rejects unknown paths and mutation. The receipt template repeats the same
claim. Because the final receipt can certify `PASS` while an explicitly
forbidden artifact exists, this is a blocking evidence-integrity defect rather
than a documentation-only issue.

## Required clearing evidence

`bug-fix` must make non-excluded directory objects participate in the stable
inventory/unexpected-path policy, or reject unknown directories before
receipt creation, without weakening the four declared excluded subroots.
Regression coverage must include empty unknown `server.sqlite.*`, `.bak`,
`.backup`, alternate-lock and backup directories, plus add/remove/rename
mutation between the two snapshots. Then rerun focused/full receipt,
process-lock Go, project/CI, syntax, links/licenses, and diff checks.

## Blockers

The P1 finding above. Feature-level P2 remains separately blocked on the exact
clean-SHA Chrome/Safari journeys, physical Safari Touch ID, and final machine
receipt.
