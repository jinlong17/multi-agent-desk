# Test plan: Phase 0 CI and remote governance

## P1 acceptance matrix

| Requirement | Command/scenario | Expected evidence |
|---|---|---|
| workflow syntax | parse both YAML files plus `verify-actions.mjs` | exact triggers, jobs, permissions, matrix, pins, commands |
| CODEOWNERS | generate to temporary string and compare committed file | exact module-registry parity; mutation fails |
| DCO | signed and missing/malformed fixture; current feature range | signed pass; each bad fixture fails with commit/message |
| local links | scan repository Markdown; inject missing file/anchor fixture | real tree pass; both negative fixtures fail |
| pnpm/Cargo licenses | real lock inventories and clean/GPL/unknown fixtures | real and clean pass; GPL/unknown fail specifically |
| Go licenses | pinned go-licenses v2 with tests and project ignore | current no-third-party Go tree passes; unknown/forbidden would fail |
| secret/write surface | search workflows for secrets/write/id-token/releases/deploy | none present |
| scaffold regression | frozen/offline `npm run scaffold:verify` on macOS | pass |
| project integrity | `npm run project:verify`, diff/conflict-marker checks | pass |
| Windows interaction | inspect three Spike logs | `Windows acceptance: deferred (no local Windows machine)`; DRAFT |

Action execution on GitHub, including Linux/macOS/Windows results, is `unknown`
through P1. Static YAML validation is not runner evidence.

## P2 acceptance matrix

1. With explicit operator authorization, push and open an unmerged test PR.
2. Record all seven exact checks; require success for the clean commit on
   `ubuntu-latest`, `macos-latest`, and `windows-latest`.
3. Seed GPL-3.0-only fixture in the test PR; require `license-gate` failure and
   record its run URL/log conclusion. Remove fixture; require green rerun.
4. Query branch protection and Actions permissions before mutation. Apply only
   the operator-approved exact configuration, then query back and compare.
5. Confirm required checks are strict, names unique, review/CODEOWNER/admin/
   conversation/linear-history rules match, and force push/delete are false.
6. Confirm default token permissions read and PR approval false; no release
   permission/workflow exists.
7. Run `npm run project:verify` after every state/dashboard transition.

If remote APIs are unavailable, permissions insufficient, a check never
materializes, or a runner fails, record the exact state and return BLOCKED or
READY_FOR_VERIFY only as the workflow allows; never infer success.

## External primary evidence

- GitHub Docs: unique required-check names, branch protection, workflow
  permissions, matrix and concurrency behavior.
- Official action repositories: checkout/setup-node/setup-go/pnpm setup/cache
  current Node 24-compatible majors.
- Google go-licenses v2 README: unknown/forbidden/allowed license checking.
- lycheeverse/lychee-action v2 README: fail-on-error link checking and SHA pin
  recommendation.

