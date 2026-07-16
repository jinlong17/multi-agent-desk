# Feature Verification: Codex Vertical Slice P3B

## Verdict

`VERIFIED`

The exact Linux Codex `0.144.2` credentialed vertical slice satisfies the
approved P3B acceptance criteria. Fresh deterministic, race, platform-build,
governance, and live-provider checks found no P3B architecture, security,
compatibility, lifecycle, or regression failure. P4 remains the next approved
phase; this verdict does not claim final Security acceptance, Ship, merge,
push, release, or deployment.

## Scope and authority

- Owner: `provider`; impacts: `core`, `security`, `project-system`
- Branch: `codex/provider/phase2-codex-vertical-slice`
- Re-read the current state authority, P3B test contract, P3A v3 verification,
  and the complete P3B implementation/live receipt diff
- Wrote no implementation, plan, compatibility, dashboard, generated, Git, or
  remote configuration state during verification

## Fresh commands and results

| Check | Result |
|---|---|
| `go test -count=1 ./...` | pass |
| `go vet ./...` | pass |
| `go test -count=1 -race ./...` | pass |
| CGO-disabled macOS arm64, Linux amd64, Windows amd64 `go build ./cmd/...` | pass |
| `gofmt -l cmd internal`; `git diff --check` | clean/pass |
| direct workflow and dashboard-static verification | pass: agents=`10`, skills=`3`, docs=`17`, edges=`20`, phases=`9`, dirty=`29` |
| Actions, CODEOWNERS, gate-fixture, local-link, and license verification | pass: checks=`7`, actions=`15`, links=`220`, pnpm groups=`5`, Cargo packages=`418` |

The composite npm wrappers were not invoked because npm is absent from this
shell. Their direct non-writing components were executed with the bundled Node
runtime and all passed. The verdict writer did not regenerate dashboard state.

## Independent Linux live evidence

- The first fresh SSH attempt was reset during key exchange before any remote
  command executed; the immediate retry succeeded and is the recorded result.
- `provider describe` reported Linux amd64 Codex `0.144.2`, canonical schema
  fingerprint
  `a1a35476587fe9bbfbe9e291b5200b8bc541df8c00241fe578d285ff26996e1c`,
  and the required capabilities as supported.
- `auth status` reported the owner-bound Vault credential healthy at revision
  2. No credential, token, email, OAuth URL, or raw provider payload was copied
  into repository evidence.
- A fresh Session started with a real Provider thread ID. A second CLI attached,
  acquired and heartbeated revision 1, sent a turn, and replayed five bounded
  chunks that reconstruct exactly `P3B_VERIFY_OK`. The Session remained running
  and official Usage returned a high-confidence `0.144.2` snapshot.
- A fresh stop transitioned the verifier Session to `exited`. Resume returned
  `provider_resume_unsupported`; local Session count remained `18 -> 18`.
- Read-only inspection of the writer receipt confirmed the standard fileChange
  Approval is `approved / written`, its disposable output is byte-exact, the
  stop receipt is `exited`, the kill receipt is `killed`, and the final daemon
  log is empty.

## Acceptance and safety review

- Official login, enrollment validation, and runtime inherit only the bounded,
  credential-free HTTP(S)/no-proxy allowlist. Secret and unrelated environment
  variables remain excluded.
- Enrollment reads only the exact private, bounded, regular `auth.json`; official
  login/app-server runtime residue is never imported and terminal enrollment
  cleanup removes the complete private staging home.
- Live-observed `thread/start`, status, item, patch, diff, and resolved-request
  shapes have exact allowlists. Their payload values are not persisted; unknown
  methods/fields continue to fail closed.
- Provider policy amendments, permissions Approval responses, persistent
  variants, conversation resize, and Provider continuation remain disabled or
  typed unsupported exactly as frozen.
- Approval claim/write/complete, Usage projection, binding-scoped stop/kill,
  second-CLI lease/input/observe, and Fake/cross-platform regressions remain
  green through the fresh full suite.

## Findings

None for P3B.

P4 still owns the exact macOS smoke, Windows CI/protocol baseline,
compatibility-matrix reconciliation, and final Security handoff.

## Handoff

**Target**: `phase2-codex-vertical-slice`
**Completed**: `feature-verify / P3B`
**Verdict**: `VERIFIED`
**Summary**: `The exact Linux Codex 0.144.2 credentialed Session, second-CLI control/output, Usage, standard Approval, stop/kill, typed resize, and Resume no-mutation acceptance all pass with no P3B finding.`
**Evidence**: `fresh full Go/vet/race, macOS/Linux/Windows builds, formatting/diff/governance checks, independent Linux Session/Usage/stop/resume smoke, and read-only Approval/kill receipt inspection.`
**Findings**: `none for P3B.`
**Blockers**: `none for P3B; P4 and final Security Review remain later gates.`

### Next Step

Run `feature-build` for `phase2-codex-vertical-slice` P4.
