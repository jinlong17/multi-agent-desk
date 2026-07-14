# Feature review: phase0-ci-governance

## Verdict

`APPROVED`

P1 is executable without remote authority, and P2 defines a bounded,
read-before-write/readback remote change with explicit operator authorization.
The seven check names are unique and stable enough for branch protection.

## Findings

No blocking or revision findings.

## Builder notes

1. Resolve every Action tag to a full 40-character commit SHA and retain the
   human-readable tag in a comment. Verify the SHA/tag relationship with the
   upstream repository; do not copy a search-result snippet blindly.
2. Use only official Tauri Linux prerequisite packages from the current v2
   prerequisites page. Rust setup should use the pinned `rust-toolchain.toml`
   through rustup already present on hosted runners, avoiding another
   third-party setup action.
3. Give the matrix job `name: build-${{ matrix.id }}` and assert the rendered
   contract statically; GitHub required checks use job display names, not just
   YAML job IDs.
4. DCO must handle PR merge refs by checking the PR base SHA through PR head
   SHA after full-history checkout; never inspect only the synthetic merge
   commit. Push-to-main uses `before..sha`, with the all-zero initial case
   failing closed/using the available parent rather than skipping.
5. The SPDX parser needs tests for precedence, parentheses, legacy `/`, WITH,
   OR allowed fallback, AND disallowed branch, unknown, GPL/AGPL, and
   LicenseRef/custom. Do not implement substring-only license checks.
6. `go-licenses` must be pinned to v2.0.1 and configured to include tests and
   ignore only the first-party module; stderr warnings remain visible.
7. Link validation must exclude generated/ignored build trees and avoid
   treating code-fence examples/templates as real links. Lychee failure remains
   fatal; no issue-writing action or token permission is needed.
8. P2 must stop before each remote mutation until the operator authorizes the
   exact settings. If GitHub plan support differs, return BLOCKED rather than
   lowering approval/admin/CODEOWNER/check requirements.

## Security and supply-chain review

Top-level `contents: read`, disabled credential persistence, no secret/OIDC/
write/deploy/release surface, SHA-pinned actions, frozen locks, and fork-safe
jobs form an appropriate Phase 0 baseline. Third-party actions are limited to
pnpm setup and lychee, both under the same SHA-pin requirement. Caches are
untrusted performance inputs; builds still verify frozen lockfiles.

## Evidence

- feature brief, revised Plan v0.2 §17/§19 dependencies, design/api/test
- GitHub Docs on branch protection, unique required checks, permissions,
  matrix/concurrency, and Node 24-compatible official actions
- official pnpm action, lychee action, and Google go-licenses v2 documentation
- local `git ls-remote` proves go-licenses v2.0.1 tag exists
- `npm run project:verify` passed at `NEEDS_REVIEW`.

## Blockers

None for P1. P2 remains a mandatory operator gate, not a current BLOCKED
verdict.

