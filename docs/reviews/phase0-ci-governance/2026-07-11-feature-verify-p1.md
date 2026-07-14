# Feature verification: phase0-ci-governance P1

## Verdict

`VERIFIED`

The local CI contracts and gates satisfy P1. This verdict does not claim any
GitHub runner result or remote repository setting; those remain P2 evidence.

## Independent evidence

- `pnpm install --offline --frozen-lockfile` — pass for all six workspaces.
- `npm run ci:verify` — pass: seven stable check names, 15 SHA-pinned action
  uses, generated CODEOWNERS parity, all positive/negative fixtures, 133 local
  Markdown files, five pnpm license groups, and 418 Cargo packages.
- `node scripts/ci/verify-dco.mjs --base c96dc95 --head HEAD` — pass for the
  single P1 builder commit; its Signed-off-by trailer is valid.
- Ruby 2.6 `YAML.safe_load` — both workflow files parse.
- `go-licenses v2.0.1 check --include_tests ...` installed into a temporary
  directory and executed with the workflow's allow list and module ignore —
  pass for the current dependency-free Go tree.
- `npm run scaffold:verify` with temporary pinned Go 1.26.5 — pass: formatting,
  Go tests/builds, TypeScript checks/builds, Cargo locked check, and Tauri
  release no-bundle build on macOS.
- `npm run project:verify`, conflict-marker scan, and `git diff --check` — pass.
- The three Windows Spikes remain `DRAFT`.

## Findings

No P1 implementation finding. The builder's initial scaffold rerun failed
because system PATH lacked Go; the verifier reproduced the full check with the
already-provisioned version-pinned temporary toolchain and did not rewrite that
failure history.

Linux, macOS, and Windows GitHub Actions executions are `unknown`. Required
checks, branch protection, DCO enforcement at GitHub, GPL test-PR rejection,
and remote Actions/release permissions are also `unknown` until authorized P2.

Windows acceptance: deferred (no local Windows machine). No interactive Fake
Session, ConPTY, Named Pipe, or Tauri sidecar acceptance is claimed.

## Scope

Verification made no implementation, plan, remote, push, merge, release, or
repository-setting change.
