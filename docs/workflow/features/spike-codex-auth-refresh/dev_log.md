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
| Time-box | `5 days (48h soak included)` |
| Current Phase | `PROVIDER_SPIKE` |
| Status | `SPIKE_READY` |
| Executor | `Codex CLI 0.144.2 (ChatGPT auth)` |
| Updated | `2026-07-14 14:43 -0700` |
| Suggested Next | `provider-spike` |
| Security Gate | `open — file credential store, device auth, and concurrent refresh touch credentials (SOP_SPIKE rule 5); security-review required on evidence` |
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

## Result, limitations, and fallback

In progress. Schema discovery, rate-limit/usage reads, proactive refresh, and the
first same-account two-device refresh passed. Device-auth initiation passed on
macOS and Linux but completed isolated login is not yet proven. The 48-hour soak
is running. Fallback remains one canonical refresh writer with revisioned CAS;
multi-writer support is not a compatibility claim unless the soak and security
review accept it.

## Risks and Blockers

- Blocks Phase 2 design freeze (not Phase 1) until the 48-hour soak and security review finish.
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
