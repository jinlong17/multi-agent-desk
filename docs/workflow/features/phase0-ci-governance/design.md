# Design: Phase 0 CI and remote governance

## Decision snapshot

- Owner: `project-system`; no Provider or Security Gate.
- P1 creates read-only GitHub Actions and deterministic local validators with
  positive/negative evidence. P2 is a separate operator-gated remote phase.
- CI never receives Provider credentials, repository write permission, OIDC,
  release permission, deployment environment, or untrusted secret access.
- Required check names are unique and stable across workflows.

## P1: workflows and local gates

Create two workflows:

1. `ci.yml`: jobs named `project-verify`, `build-ubuntu`, `build-macos`, and
   `build-windows`. The three build jobs share one matrix definition but use
   explicit unique display names. Each installs pinned Go/Node/pnpm/Rust,
   frozen dependencies, platform Tauri prerequisites, and runs
   `npm run scaffold:verify`. Ubuntu installs the official Tauri Linux system
   dependency list; macOS uses Xcode runner tools; Windows uses hosted MSVC and
   WebView2 prerequisites. Go/pnpm/Cargo dependency caches key off lockfiles.
2. `governance.yml`: unique jobs `license-gate`, `dco`, and `link-check`.
   License runs the repository validator plus pinned
   `github.com/google/go-licenses/v2@v2.0.1`; DCO validates only the PR/push
   commit range; link-check runs a deterministic local-target validator and
   lychee for HTTP links.

All jobs use `permissions: contents: read`, checkout with persisted credentials
disabled, no secrets, and SHA-pinned actions with readable version comments.
Workflows run on pull requests targeting `main`, pushes to `main`, and manual
dispatch. Concurrency cancels superseded branch/PR runs.

## Validator contracts

- `scripts/ci/verify-codeowners.mjs` generates/compares `.github/CODEOWNERS`
  directly from `module-registry.json`; the repository owner handle is an
  explicit generator input/default (`@jinlong17`), not a second module map.
- `verify-dco.mjs` requires a `Signed-off-by: Name <email>` trailer in every
  commit in the supplied Git range. Fixture mode proves signed pass and missing
  or malformed sign-off fail.
- `check-local-links.mjs` parses Markdown links, ignores code fences and remote
  URLs, resolves anchors/files, and fails missing local targets/anchors.
- `verify-licenses.mjs` evaluates pnpm and Cargo SPDX expressions: unknown,
  LicenseRef/custom, GPL, AGPL, SSPL, BUSL, Commons-Clause, and other unapproved
  expressions fail. OR passes only when at least one branch is allowed; AND
  requires every branch. Fixture mode includes clean and seeded GPL-3.0-only
  inventories. Go dependencies are independently checked by go-licenses with
  tests included and the project module ignored as first-party.
- `verify-actions.mjs` parses workflows/CODEOWNERS/contracts, asserts exact job
  names/triggers/minimal permissions/action SHA pins/matrix platforms and fails
  drift before remote execution.

P1 local verification runs all positive fixtures and explicit expected-failure
fixtures for GPL, unknown/custom license, missing DCO, broken link, CODEOWNERS
drift, and workflow permission/check-name drift. Expected failures must exit
nonzero with specific messages and then restore/leave the worktree unchanged.

## P2: remote governance and proof

P2 begins only after explicit operator authorization for push, test PR, and
GitHub settings. It will:

1. push the feature branch and create a test PR without merging;
2. observe the seven exact check names and record Linux/macOS/Windows logs;
3. create a temporary test-PR commit/fixture that makes `license-gate` fail on
   GPL-3.0-only, record the failed run, then remove it and require green rerun;
4. configure/audit `main` branch protection with strict required checks:
   `project-verify`, `build-ubuntu`, `build-macos`, `build-windows`,
   `license-gate`, `dco`, `link-check`; for this operator-owned single-account
   repository require zero approving reviews and do not require CODEOWNER
   review, while retaining conversation resolution, linear history,
   enforcement for admins, and disabled force pushes/deletions. CODEOWNERS
   remains deterministic ownership/routing metadata rather than a merge gate;
5. set default workflow token permissions to read-only and disallow Actions
   from approving pull requests; no release/deployment write permission;
6. query the remote configuration back and persist sanitized evidence.

If the GitHub plan/repository cannot enforce a proposed setting, P2 returns
BLOCKED for operator choice; it does not silently weaken protection or accept
risk. The operator explicitly accepted the single-account/no-review policy on
2026-07-14 and authorized direct `main` completion as the highest priority.
Push, PR creation, protection mutation, and permission mutation otherwise
remain separate explicit human authorization even though local commits are
allowed.

## Failure, security, and rollback

Workflow or validator failures remain failures. External-link network failures
are distinguished from broken local links but still do not become pass.
Actions are read-only; fork PRs receive no secrets. P1 rollback removes local
workflows/scripts. P2 rollback restores the recorded prior GitHub settings and
closes the unmerged test PR/branch only with operator authorization.
