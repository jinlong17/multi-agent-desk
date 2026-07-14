# Feature verification: Phase 0 CI governance P2 single-account policy

## Verdict

`READY_TO_SHIP`

The operator-approved single-account/no-review policy is implemented exactly,
the prior CODEOWNER identity deadlock is resolved, the current pull-request
head has all seven required checks successful, and the complete local scaffold
regression passes. No product, provider, credential, or security boundary was
changed by the exception.

## Scope and authority

- Target: `phase0-ci-governance`, final phase P2.
- Owner: `project-system`; impacted modules: none.
- Verified head: `36dfe6fbca57fe1e8157ffbee0ac62de74ce9bf1`.
- Pull request: [#1](https://github.com/jinlong17/multi-agent-desk/pull/1).
- Human gate: on 2026-07-14 the operator explicitly accepted one account,
  zero review, and direct completion into `main` as the highest priority.
- Security Gate: none. Actions remain read-only and hold no provider secrets.

## Remote acceptance evidence

Authenticated readback proves:

1. PR #1 head exactly equals the verified local and remote branch head.
2. PR #1 is `OPEN`, `MERGEABLE`, and `CLEAN`, with no required review decision.
3. CI run
   [`29359454753`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29359454753)
   completed successfully for `project-verify`, `build-ubuntu`, `build-macos`,
   and `build-windows`.
4. Governance run
   [`29359454802`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29359454802)
   completed successfully for `license-gate`, `dco`, and `link-check`.
5. `main` protection requires those exact seven contexts with strict
   up-to-date checks. Approval count is `0` and CODEOWNER review is disabled,
   matching the operator exception. Admin enforcement, conversation
   resolution, and linear history remain enabled; force push and deletion
   remain disabled.
6. Default Actions token permissions remain `read`, and Actions cannot approve
   pull-request reviews.
7. The retained GPL-negative run `29315247924` still proves that
   `GPL-3.0-only` is rejected; the later clean runs do not erase that evidence.

## Local verification

The first dependency attempt intentionally used the bundled pnpm 11.7.0 and
was rejected by the repository's `>=10 <11` engine constraint before any test
ran. The verifier then used pinned pnpm 10.23.0 and obtained:

| Check | Result |
|---|---|
| offline frozen install | pass; six workspace projects, lockfile unchanged |
| workflow generate/verify | pass; 10 agents, 3 skills, 17 docs, 20 edges, 15 statuses |
| dashboard generate/verify | pass; clean tree, 9 phases, 10 agents, 3 skills |
| Actions/CODEOWNERS | pass; 7 checks, 15 pinned actions, owner `@jinlong17` |
| deterministic gate fixtures | pass; positive and negative cases |
| local links | pass; 136 Markdown files |
| dependency licenses | pass; 5 pnpm groups and 418 Cargo packages |
| DCO feature range | pass; 29 commits, 3 exact grandfathered commits |
| scaffold layout | pass; 27 directories, 49 files, 7 modules |
| Go format/test/build | pass; 15 formatted files, all packages, both commands |
| TypeScript checks/builds | pass for Web and shared packages |
| Web production build | pass with Vite 7.3.6 |
| Cargo format/check | pass with locked dependencies |
| Tauri release build | pass with `--no-bundle`; executable produced |
| diff/worktree | pass; no tracked change before verdict write |

## Acceptance mapping

- Seven stable check names: proven locally and on the current GitHub head.
- Linux/macOS/Windows execution: all three current jobs succeeded.
- GPL rejection and clean recovery: retained failed negative run plus current
  successful license gate.
- Exact branch protection: independent GET matches the operator-approved
  single-account policy and all retained safeguards.
- Least-privilege Actions: read-only/no-approval settings and only CI plus
  Governance workflows.
- Documentation/state: plan, test, receipt, and resumable log agree on the
  current policy; historical BLOCKED evidence remains intact.

## Findings and residual risk

No blocking finding remains. The accepted residual risk is that this
single-account repository has no independent human code review gate. That risk
was explicitly accepted by the operator; CI, DCO, licenses, cross-platform
builds, linear history, admin enforcement, and destructive-update protections
continue to provide automated and repository-level safeguards.

The unknown pre-mutation value of the full-length Action SHA setting remains a
rollback-evidence limitation, not a forward acceptance failure: current
readback proves full-length SHA pinning is enabled and the workflows contain 15
pinned action references.

## Conclusion

P2 is technically and procedurally ready for the explicitly authorized linear
merge. The ship role must verify the same head and protection subset immediately
before merging, then record the actual `main` commit before marking `SHIPPED`.
