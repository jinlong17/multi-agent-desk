# Spike log: Codex auth, usage, and concurrent refresh

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-codex-auth-refresh` |
| Title | `Codex auth, usage, and concurrent refresh` |
| Owner Module | `provider` |
| Impacted Modules | `core, security` |
| Hypothesis | `Codex supports app-server schema discovery, usage methods, a file credential store, headless device auth, and two devices refreshing one account concurrently for ≥48h without corruption` |
| Time-box | `operator-shortened: ~3h two-device run + conservative fallback` |
| Current Phase | `SECURITY_REVIEW` |
| Status | `ACCEPTED` |
| Executor | `Codex (GPT-5), security-review` |
| Updated | `2026-07-14 19:18 -0700` |
| Suggested Next | `feature-plan` |
| Security Gate | `resolved — ACCEPTED only with canonical single refresh writer, revisioned CAS, secret-safe evidence, and interactive-login fallback` |
| Evidence Path | `docs/spikes/codex/` |
| Decision Record | `pending — PROVIDER_COMPATIBILITY.md entry` |

## Success and failure criteria

- Supported when: each sub-claim reproduces on a second machine with pinned Codex version.
- Falsified when: any sub-claim fails or requires undocumented behavior.

## Environment

| Field | Value |
|---|---|
| Tool + version | Codex CLI `0.144.2` (current probe; two earlier supported versions still required for replay) |
| OS | macOS 26.5.2 arm64 + Linux 5.4.0 x86_64 second device |
| Auth mode | same ChatGPT account active on both devices; isolated device-auth initiation and file credential store are explicit experiment arms |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-14 13:18 -0700 | Generated app-server schemas and replayed account reads with Codex `0.142.5`, `0.143.0`, and `0.144.2` | initialization, account, rate-limit, and usage methods passed on all three versions; proactive refresh passed on `0.144.2` | `docs/spikes/codex/app-server-account-matrix.json` |
| 2026-07-14 14:33 -0700 | Started device auth in empty isolated `CODEX_HOME` on macOS and Linux | both produced device-auth prompts and authorization URLs; no code/URL/credential persisted; completion intentionally not claimed | `docs/spikes/codex/2026-07-14-auth-refresh-spike.md` |
| 2026-07-14 14:36 -0700 | Concurrent `account/read {refreshToken:true}` on the same account from macOS and Linux | both succeeded, both `auth.json` files changed, both post-refresh reads succeeded, same account before/after | `docs/spikes/codex/app-server-account-matrix.json` |
| 2026-07-14 14:43 -0700 | Started sanitized hourly two-device soak (PID `35221`) | first sample passed; minimum completion `2026-07-16T21:43:11Z` | `/private/tmp/mad-codex-soak-20260714T2143Z.jsonl`, harness in `docs/spikes/codex/` |
| 2026-07-14 19:16 -0700 | Operator cancelled the 48-hour requirement and terminated the harness after four samples | all four macOS/Linux account, rate-limit, and usage samples passed across `10812.845s`; first concurrent refresh passed; no 48h or production multi-writer claim; canonical single-writer CAS fallback selected | `docs/spikes/codex/two-device-short-run.json` |

## Result, limitations, and fallback

Evidence ready. Schema discovery, rate-limit/usage reads, proactive refresh,
and four short-run same-account two-device samples passed. Device-auth
initiation passed on macOS and Linux, but completed isolated login is not
claimed. The operator cancelled the 48-hour requirement. The original
long-duration/multi-writer hypothesis is therefore unsupported, and the
selected production boundary is one canonical refresh writer with revisioned
CAS. Security review must accept that boundary before the compatibility
decision is recorded.

## Risks and Blockers

- Blocks Phase 2 design freeze (not Phase 1) until security review accepts the single-writer boundary.
- Official documentation does not define concurrent refresh-token rotation semantics; absence of a failure in the first sample is not a safety guarantee.
- The remote SSH transport reports that its current key exchange is not post-quantum; no Provider credential was sent through SSH, but the infrastructure warning remains recorded for operator remediation.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-10 20:56 -0700 | Claude Code (Fable 5), lifecycle-readiness build | Spike unit created from Phase 0.5 breakdown | this file | `DRAFT` | feature-plan |
| 2026-07-10 21:50 -0700 | Claude Code (Fable 5), lifecycle-readiness P2 build | Security Gate opened per R2 review P0-C (SOP_SPIKE rule 5: credentials/auth in scope) | this file | `DRAFT`, gate `open` | feature-plan |
| 2026-07-14 12:43 -0700 | Codex CLI 0.144.2, feature-plan | Froze the five-part provider hypothesis, pinned the first CLI/OS/auth environment, and retained the 48h/two-device/three-version exit criteria | this file | `SPIKE_READY` | provider-spike |
| 2026-07-14 14:43 -0700 | Codex CLI 0.144.2, provider-spike | Completed three-version schema/live account replay, verified device-auth initiation on macOS/Linux, proved the first same-account concurrent refresh, and started the sanitized 48-hour soak | `docs/spikes/codex/`; this file | `SPIKE_READY`, experiment running | provider-spike after `2026-07-16T21:43:11Z` |
| 2026-07-14 14:43 -0700 | Codex CLI 0.144.2, provider-spike | Refreshed the operator-owned dashboard to Phase 0.5 active with a status binding to this Spike; regenerated and verified machine facts | `docs/workflow/project/dashboard-state.json`; `npm run dashboard` equivalent | dashboard verified, focus `SPIKE_READY` | continue provider-spike |
| 2026-07-14 19:16 -0700 | Codex CLI 0.144.2, provider-spike | Applied the operator cancellation, stopped the 48-hour harness, persisted four sanitized samples, rejected a production multi-writer claim, and selected the canonical single-writer revisioned-CAS fallback | `docs/spikes/codex/two-device-short-run.json`; provider evidence; this file | `EVIDENCE_READY`, gate remains `open` | security-review |
| 2026-07-14 19:18 -0700 | Codex (GPT-5), security-review | Reviewed credential trust boundaries, sanitized evidence, file mutation, lease/CAS, crash recovery, account pinning, device-auth limits, revocation, audit safety, and residual risk | `docs/reviews/spike-codex-auth-refresh/2026-07-14-security-review.md`; this file | `ACCEPTED`; Security Gate resolved only for the canonical single-writer boundary | feature-plan decision |
