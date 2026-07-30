# Bug verification v5: Phase 4a P2 FIFO `lsof` device-field receipt repair

## Verdict

`READY_TO_SHIP` for the scoped macOS FIFO `lsof -F` device-field repair.

Native macOS evidence shows a declared FIFO writer FD whose `lsof` record has
no `D` device field. The repair accepts only that absent FIFO field. It still
requires FIFO kind, expected access, exact inode, canonical path, owner, mode,
and link count; it preserves holder inventory, sole `/bin/cat` reader, and TTY
checks. A reported FIFO device remains an exact match. Regular-file database
and process-lock device checks are unchanged and unconditional.

This scoped verdict does not complete P2. P2 remains `BLOCKED` on the
pre-existing clean exact-SHA Chrome/Safari, physical Safari Touch ID/platform
Passkey, machine-scan, and final-receipt evidence.

## Scope and audit

- Owner: `control-plane` (high confidence); secondary impacts: `security`
  receipt evidence and `project-system` CI.
- Audited working-tree files: the receipt verifier/test plus its P2 template,
  test contract, and development log.
- `git diff --exit-code -- cmd internal apps api packages migrations` passed:
  production implementation is zero diff.
- The verifier change is limited to `fifoBinding`: `fd.device` may be absent
  only for a FIFO, and if present must equal the lstat device. `processLockBinding`
  still requires a matching `REG` device unconditionally.
- No browser, P2 product runtime, implementation, test, plan, dashboard,
  commit, push, or PR action was performed by this verifier.

## Reproduction and regression evidence

1. `node --test --test-name-pattern='macOS global lsof holder inventory finds real FIFO writer and cat without device field' scripts/acceptance/p2-browser-receipt.test.mjs` passed. It created an owner-only FIFO with a real writer and `/bin/cat` reader; the writer stdout was `FIFO`, access `w`, exact inode and canonical path, and had no `device` property.
2. `node --test --test-name-pattern='declared-process log proof binds each writer to one cat-to-TTY reader and rejects malformed bindings' scripts/acceptance/p2-browser-receipt.test.mjs` passed. It accepts absent FIFO `device`, rejects a wrong present device, and rejects missing inode, path, and access.
3. `node --test scripts/acceptance/p2-browser-receipt.test.mjs` passed `64/64`.
4. `npm run acceptance:p2-browser:test` passed `64/64`.
5. `npm run project:verify` passed: workflow `10/3/17/20/15`; dashboard generation and verification passed.
6. `npm run ci:verify` passed: 7 Actions checks, 15 pinned actions, CODEOWNERS, CI fixtures, receipt `64/64`, 327 local Markdown files, 6 pnpm license groups, and 418 Cargo packages.
7. `node --check scripts/acceptance/p2-browser-receipt.mjs`; `node --check scripts/acceptance/p2-browser-receipt.test.mjs`; `git diff --exit-code -- cmd internal apps api packages migrations`; and `git diff --check` all passed.

## Findings

None in the scoped repair.

## Blockers

None for scoped ship. The separate P2 feature-level evidence gap remains for
the coordinator and later independent `feature-verify P2`.
