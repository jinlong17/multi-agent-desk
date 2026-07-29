# Feature verification: As-built product documentation authority P1

## Verdict

`VERIFIED`

The narrow repair commit `1165a1330dca0d052be7d55b42a88951e573d3f4` closes F1.
Its append-only CLM-021 through CLM-023 entries cover the three previously
missing User Guide subheadings. Each preserves an explicit planned-only scope,
the authority/trust boundary, source and destination anchors, and open
security/feature gates. No existing claim, conflict, or Provider receipt was
changed. P1 is complete; P2 remains the next approved build phase.

## Scope and state inspected

- Owner: `project-system`; secondary impacts: `core`, `provider`,
  `control-plane`, `web`, `desktop`, and `security`.
- Initial build receipt: `a54d3336a6bc352165a33dbd517c148f9e186b80`.
- Repair receipt: `1165a1330dca0d052be7d55b42a88951e573d3f4`.
- Repair diff scope: only the P1 claim ledger and its build lifecycle receipt.
- No product documentation, Provider evidence, Security verdict, dashboard
  judgment, implementation, commit, push, merge, or release action was made
  by this verification.

## Evidence and checks

| Check | Result |
|---|---|
| `git diff --check a54d333^ a54d333` | pass |
| `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run ci:links` | pass: 309 Markdown files |
| `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run project:verify` | pass: workflow `agents=10, skills=3, docs=17, edges=20, statuses=15`; dashboard static verification passed |
| Stable IDs and vocabulary inspection | pass: 20 `CLM-###` headings, all unique; no invalid canonical class line |
| Baseline and receipt reachability | pass: baseline `397194bfaea6983c4c0a54289d8bb1844711cfd6` and the two positive Codex receipts `250bf57f2ec21beb5f02e03b58f030b9d67e5ff4` and `b041240646c337aa67a2f3c078a498356218b44d` are commits reachable from that baseline; both referenced report paths exist at their pinned revisions |
| Positive-claim scope review | pass: CLM-012 and CLM-020 retain exact Codex CLI `0.144.2` Linux bounds, source-built/pre-release boundary, fail-closed/typed-unsupported fallback, and no macOS identity or Windows runtime expansion |
| Conflict, trust, and freshness review | pass: CON-001 through CON-003 are retained; Provider gaps remain unknown/open-gated; the ledger and test contract specify deterministic day-31 snapshot and day-91 Provider downgrades to `unknown` with the required stale state and refresh gate |
| Initial source-heading coverage inspection | fail at `a54d333`: no ledger source or coverage-map entry exists for `docs/USER_GUIDE.md#9.1-部署-control-plane`, `#9.2-配对设备`, or `#9.3-使用-web-或-desktop` |

## Re-verification of repair `1165a13`

| Check | Result |
|---|---|
| Repair diff scope | pass: append-only CLM-021 through CLM-023, three coverage-map rows, ledger revision increment, and the clearing build receipt only |
| F1 source-heading coverage | pass: `docs/USER_GUIDE.md#91-部署-control-plane` maps to CLM-021; `#92-配对设备` maps to CLM-022; `#93-使用-web-或-desktop` maps to CLM-023, each both in the claim body and coverage map |
| Planned/no-support boundary | pass: all three rows are `planned`; each states no executable/shipped remote deployment, pairing, Web/Desktop terminal, approval, or Credential Grant support; Passkey/pinning/E2EE limits remain explicit |
| Existing claim and conflict preservation | pass: CLM-001 through CLM-020 and retained conflicts CON-001 through CON-003 are byte-for-byte unchanged from `1165a13^` |
| Receipt preservation | pass: exact positive Codex receipts remain reachable from baseline `397194bfaea6983c4c0a54289d8bb1844711cfd6` and their pinned report paths exist; repair rows are not Provider claims and require no receipt |
| `git diff --check 1165a13^ 1165a13` | pass |
| `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run workflow:verify` | pass: `agents=10, skills=3, docs=17, edges=20, statuses=15` |
| `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run ci:links` | pass: 310 Markdown files |

## Prior finding

### F1 — complete coverage was not established at `a54d333` (closed)

`docs/workflow/features/as-built-product-docs/design.md` defines the P1
universe as the source heading/anchor of every substantive product-facing
support or trust statement. The User Guide's three level-3 headings contain
such statements, but the ledger covers only the level-2 `#9-启用远程访问规划中phase-4a4b`
entry. A parent-row inference would make the coverage map non-auditable and
would weaken the intended destination/conflict traceability.

Repair outcome: `feature-build` appended CLM-021 through CLM-023 and their
coverage-map entries without deleting retained conflict history. Independent
re-verification above confirms the repair, so F1 no longer blocks P1.

## Non-findings

- No unsupported support expansion was found in the original ledger or repair.
- No positive Provider claim lacked an exact reachable receipt.
- The Provider and Security Gates remain open and unresolved.
