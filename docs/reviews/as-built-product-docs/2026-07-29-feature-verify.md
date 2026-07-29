# Feature verification: As-built product documentation authority P1

## Verdict

`BLOCKED`

P1's inventory is structurally clean and preserves the intended support and
trust boundaries, but it does not meet its approved complete-source-heading
coverage criterion. The coverage map omits three substantive User Guide
subheadings: `#9.1` Control Plane deployment, `#9.2` device pairing, and `#9.3`
Web/Desktop use. Their statements cannot be assumed covered by their parent
heading under the approved source-heading/anchor contract.

The clearing role is `feature-build`: add stable ledger coverage for each
omitted source heading, with its normalized scope, authority/evidence,
destination or retained conflict, then return P1 to `READY_FOR_VERIFY` for a
fresh independent verification. This verdict does not authorize P2.

## Scope and state inspected

- Owner: `project-system`; secondary impacts: `core`, `provider`,
  `control-plane`, `web`, `desktop`, and `security`.
- Build receipt: `a54d3336a6bc352165a33dbd517c148f9e186b80`.
- Diff scope: only the P1 claim ledger and its build lifecycle receipt.
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
| Source-heading coverage inspection | fail: no ledger source or coverage-map entry exists for `docs/USER_GUIDE.md#9.1-部署-control-plane`, `#9.2-配对设备`, or `#9.3-使用-web-或-desktop` |

## Finding

### F1 — complete coverage is not established (blocking)

`docs/workflow/features/as-built-product-docs/design.md` defines the P1
universe as the source heading/anchor of every substantive product-facing
support or trust statement. The User Guide's three level-3 headings contain
such statements, but the ledger covers only the level-2 `#9-启用远程访问规划中phase-4a4b`
entry. A parent-row inference would make the coverage map non-auditable and
would weaken the intended destination/conflict traceability.

Required repair: `feature-build` must append or amend P1 ledger coverage for
each of the three headings without deleting retained conflict history, then
request a new independent P1 verification.

## Non-findings

- No unsupported support expansion was found in the committed P1 ledger.
- No positive Provider claim lacked an exact reachable receipt.
- The Provider and Security Gates remain open and unresolved.

