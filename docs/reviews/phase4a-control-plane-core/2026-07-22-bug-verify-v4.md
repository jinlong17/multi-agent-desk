# Bug verification v4: Phase 4a P2 process-lock receipt contract

## Verdict

`READY_TO_SHIP` for the scoped
`phase4a-p2-process-lock-receipt-contract` repair.

The v3 P1 directory-inventory defect is closed. Non-excluded child
directories now participate in the stable inventory, unknown empty runtime
artifacts fail closed, directory changes cannot survive the second snapshot or
finalize comparison, and the persisted receipt remains non-content and
non-cleartext. The original exact process-lock manifest/vnode/FD/flock/global-
holder/freeze/scan/finalize contract also passes without any production
`process_lock.go` change.

This scoped verdict does not verify Phase 4a P2. P2 remains `BLOCKED` until the
coordinator creates a clean exact SHA and completes the required real macOS
Chrome and Safari journeys, physical Safari Touch ID/platform-Passkey proof,
fresh machine scan, and final receipt.

## Scope and ownership

- Owner: `control-plane` (high confidence).
- Secondary impacts: `security` receipt evidence and `project-system` CI.
- Audited files:
  `scripts/acceptance/p2-browser-receipt.mjs`,
  `scripts/acceptance/p2-browser-receipt.test.mjs`,
  `docs/workflow/features/phase4a-control-plane-core/p2-browser-receipt-template.md`,
  `docs/workflow/features/phase4a-control-plane-core/test.md`, and this target
  state authority.
- Base commit:
  `402925fae304056e34c9bc53c1543c268329fccb`; branch:
  `codex/control-plane/phase4a-core`.
- `internal/controlplane/process_lock.go` has no worktree diff.
- No implementation, test, plan, dashboard judgment, commit, push, browser,
  or P3 write was performed by this verifier.

## v3 finding closure

The directory walker now validates each directory before and after traversal
through both an `O_RDONLY | O_NOFOLLOW | O_DIRECTORY` descriptor and the path
vnode. It rejects a replaced directory before scanning and compares device,
inode, owner, group, mode, link count, size, mtime, and ctime after traversal.
Every non-excluded child directory contributes one sorted inventory entry.
The declared root itself remains governed by its root contract and is not
duplicated in the child inventory.

The inventory entry persisted into a scan snapshot contains only:

- `pathDigest`
- `kind`
- `size`
- `device`
- `inode`
- `mode`
- `mtimeNs`
- `ctimeNs`

The receipt exposes only inventory digests and bounded counts. It does not
persist an absolute path, cleartext path component or directory name, file or
directory contents, or a content digest. The transient empty `contents` value
used by the uniform detector path is stripped before snapshot construction.

The fail-closed classifications are complete:

- runtime accepts only the five exact declared files and counts every other
  file or directory as unexpected;
- logs accept only FIFO entries, so a directory is unexpected;
- transfer directory entries cannot parse as an allowed public JSON artifact
  and increment `unexpectedFileCount`;
- evidence explicitly counts every non-file entry as unexpected;
- the four known runtime subroots and exact server SQLite main/sidecars are
  excluded only from the runtime walk, then scanned under their own target
  classes.

The real two-row success fixture passes with those exclusions, demonstrating
that known roots are neither missed nor double-counted. Empty runtime
directories named `server.sqlite.unknown`, `server.sqlite.bak`,
`server.sqlite.backup`, `alternate.process.lock`, and `backups` each reject.
The same classification code makes arbitrary empty log, transfer, evidence,
and runtime subdirectories fail closed.

Directory add, remove, and rename between signed and fresh scans have direct
finalize regressions. Permission drift either violates the fresh private-mode
check or changes `mode`/`ctimeNs`; timestamp drift changes `mtimeNs`/`ctimeNs`;
vnode substitution changes `device`/`inode`/`ctimeNs`; and symlink
substitution rejects during `lstat` traversal. All of those outcomes prevent a
fresh scan from matching the signed scan. Traversal-time mutation and
directory symlink/private-mode negatives also pass.

## Process-lock regression

The v3 process-lock conclusions remain true:

- manifest schema v2 requires the exact absolute
  `databasePath + ".process.lock"` path and rejects alternate, aliased,
  hard-linked, cross-row, or missing declarations;
- the server must retain exactly one numeric O_RDWR FD for the exact private,
  single-link, empty lock vnode and the Daemon must not hold it;
- the global holder inventory must contain exactly that server PID and FD;
- whole-file `W` is accepted, while blank macOS `lsof` status requires a
  second process's nonblocking shared `flock` to fail;
- the normalized lock proof is frozen, revalidated during scan, included in
  runtime residue, passed through every detector, and compared at finalize;
- no database-prefix or `*.lock` wildcard exists, and the exact lock does not
  count as a server backup.

## Commands and results

1. Both `node --check` syntax checks passed.
2. The focused directory/process-lock/scan/finalize matrix passed 36/36.
3. `node --test scripts/acceptance/p2-browser-receipt.test.mjs` passed 63/63.
4. `npm run acceptance:p2-browser:test` passed 63/63.
5. `go test -count=10 -run '^TestProcessLockExcludesConcurrentServerAndMaintenance$' ./internal/controlplane`
   passed.
6. `npm run project:verify` passed: workflow `10/3/17/20/15` and dashboard
   generation/verification succeeded.
7. `npm run ci:verify` passed: 7 Actions checks, 15 pinned actions,
   CODEOWNERS, positive/negative CI fixtures, receipt 63/63, 326 links, 6 pnpm
   license groups, and 418 Cargo packages.
8. `git diff --exit-code -- internal/controlplane/process_lock.go`,
   `git diff --check`, and final scope inspection passed.

## Findings

No P0 or P1 finding remains in the scoped repair.

## Blockers

None for scoped ship. Feature-level P2 remains separately blocked on the clean
exact-SHA real-browser, physical Touch ID/platform-Passkey, fresh scan, and
final receipt evidence described above.
