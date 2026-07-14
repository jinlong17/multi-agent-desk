# Contract: Phase 0 CI and remote governance

## Required checks

Exactly these unique job/check names form the Phase 0 protection contract:

1. `project-verify`
2. `build-ubuntu`
3. `build-macos`
4. `build-windows`
5. `license-gate`
6. `dco`
7. `link-check`

No second workflow may reuse a name. Aggregator checks are not substitutes for
the platform jobs. A skipped, neutral, stale, pending, or missing check is not
success.

## Workflow contract

- `.github/workflows/ci.yml`: one project job plus a three-entry include
  matrix `{id, os}` whose job display name is `build-${{ matrix.id }}`.
- `.github/workflows/governance.yml`: the three governance jobs.
- triggers: pull requests to `main`, pushes to `main`, `workflow_dispatch`;
- top-level `permissions: contents: read`; no job may elevate permissions;
- checkout uses full SHA pin, `persist-credentials: false`; DCO checkout also
  uses `fetch-depth: 0`;
- pnpm/Cargo installs are frozen/locked; action versions are full commit SHAs
  with tag comments; no `@main`, `@master`, or floating major tag;
- no `secrets.*`, `id-token`, `packages: write`, `contents: write`, release,
  upload, deployment, or PR-write step.

## CI command contract

`project-verify` runs frozen pnpm setup then `npm run project:verify` and the
static CI contract validator. Each platform build runs
`npm run scaffold:verify`; Windows uses `shell: pwsh` only where setup requires
it, while npm scripts remain cross-platform. Rust uses exact 1.91.1 from
`rust-toolchain.toml`; Go uses `.go-version`; Node uses `.node-version`; pnpm
uses root `packageManager`.

## Local validator CLI contract

- `node scripts/ci/verify-actions.mjs`
- `node scripts/ci/verify-codeowners.mjs [--write] [--owner @handle]`
- `node scripts/ci/verify-dco.mjs --base <sha> --head <sha>` or
  `--fixture <json>`
- `node scripts/ci/check-local-links.mjs [paths...]`
- `node scripts/ci/verify-licenses.mjs` or `--fixture <json>`

Each exits 0 only on full pass and emits a specific path/commit/license/check
on failure. Fixture input is test-only and never changes the real inventory.

## CODEOWNERS contract

`.github/CODEOWNERS` starts with a generated warning and default `*` owner,
then emits one normalized pattern for every `module-registry.json` owned path
in module order. Generation is deterministic and verification compares exact
bytes. It does not assign product priority or security approval authority.

## License policy contract

Allowed identifiers are limited to `0BSD`, `Apache-2.0`, `BSD-2-Clause`,
`BSD-3-Clause`, `CC0-1.0`, `ISC`, `MIT`, `MIT-0`, `MPL-2.0`, `Unicode-3.0`,
`Unlicense`, and `Zlib`, including approved SPDX exceptions such as
`LLVM-exception`. `/` legacy separators are treated as OR only for existing
Cargo metadata. Unknown or any identifier not in the allowlist fails. This
means GPL/AGPL/custom/restrictive licenses fail without special-casing a test.

## Remote receipt contract

P2 records repository, branch, test PR URL/number, workflow run IDs/URLs,
per-check conclusions, failed GPL fixture run and green recovery, branch-rule
JSON subset, Actions permission JSON subset, actor/time, and previous settings
for rollback. Tokens, cookies, authorization headers, secrets, and environment
contents are never persisted.

