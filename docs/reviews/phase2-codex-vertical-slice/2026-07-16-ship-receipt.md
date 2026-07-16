# Ship receipt: Phase 2 Codex Vertical Slice

## Status

`SHIPPED`

## Authorized action

The operator explicitly requested `ship`. Under the repository's local Ship
contract, this authorized exact local staging, signed-off commits, the Ship
receipt, and the `SHIPPED` state transition. It did not authorize push, pull
request creation, merge, tag, package publication, release, or deployment.

## Local result

- Branch: `codex/provider/phase2-codex-vertical-slice`
- Product commit:
  `9f82fb6f05a0e3bd3c5e62d4021f384acc744de1`
- Product commit subject: `feat(provider): complete Codex vertical slice`
- Receipt/governance commit: the signed-off commit containing this receipt
- Remote branch was not changed and remained at `ee6c8ae` when the local
  product commit completed.
- No tag, release artifact, publication, merge, or deployment was created.

## Evidence

- Workflow state before Ship: final feature verification `READY_TO_SHIP`, final
  Security Review `ACCEPTED`, Security Gate `resolved`, P0–P4S all verified.
- Exact Linux x86_64 Codex `0.144.2` credentialed Session, second-CLI control,
  Usage, standard Approval, stop/kill, typed resize negative, and Resume
  no-mutation receipt passed; final daemon log was empty.
- Exact macOS arm64 Codex `0.144.2` canonical schema and empty-home handshake
  passed. Windows current build/protocol and retained native CI baseline passed;
  no real Windows Codex support is claimed.
- Final local matrix: `go test -count=1 ./...`, `go vet ./...`,
  `go test -count=1 -race ./...`, macOS/Linux/Windows builds, focused P4S
  repeated/race tests, exact license checks, workflow/dashboard verification,
  Actions/CODEOWNERS/fixture/link checks, and `git diff --check` passed.
- Changed/untracked sensitive scan was clean except for the exact named
  synthetic rejection fixture, reported only by file/class.
- Final governance facts before product commit: agents=`10`, skills=`3`,
  phases=`9`, links=`228`, pnpm groups=`5`, Cargo packages=`418`.

## Compatibility and residual risk

- Supported product boundary remains exact Linux Codex `0.144.2` only.
- macOS `0.144.2` is schema/handshake evidence; bundled `0.144.5` is outside
  the allowlist. Real Windows Codex is unsupported.
- Root/admin, compromised daemon/Provider, same-user process inspection,
  backups, or crash tooling can read materialized credentials.
- Multi-writer refresh, completed device auth, Provider continuation, dynamic
  policy amendments, permissions grants, remote credential grants, packaging,
  release, and deployment remain unsupported or outside this Ship.

## Rollback

Use reviewed revert commits, not history rewriting. Revert the local Phase 2
product commits in reverse order as applicable:

1. `9f82fb6` — P3A/P3B/P4/P4S completion and evidence;
2. `ee6c8ae` — P2B verification record;
3. `fe9737b` — P2B native platform evidence;
4. `31e501d` — P2B implementation.

Migration `0005` is forward-only. An older binary must refuse that schema;
restore a compatible backup or recreate a Device store rather than attempting
an in-place schema downgrade. No remote, tag, release, or deployment rollback
is required because none was performed.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `ship`
**Status**: `SHIPPED`
**Summary**: `Phase 2 was shipped locally as signed-off product and governance commits after final verification and Security acceptance; no remote or release action was performed.`
**Commit/Release**: `local product 9f82fb6; receipt commit contains this file; no push/PR/merge/tag/release/deployment`
**Tests**: `full Go/vet/race; focused P4S race; macOS/Linux/Windows builds; exact Linux and macOS live evidence; license/workflow/dashboard/CI/link/diff/sensitive checks — pass.`
**Blockers**: `none for local Ship; any push, PR, merge, tag, release, or deployment requires separate explicit authorization.`

### Next Step

`None for local Ship; optional remote integration requires separate explicit authorization.`
