# Spike log: Codex distinct-account Provider Home isolation

## Status Panel

| Field | Value |
|---|---|
| Workflow | `SPIKE` |
| Target | `spike-codex-distinct-account-homes` |
| Title | `Codex distinct-account Provider Home isolation` |
| Owner Module | `provider` |
| Impacted Modules | `core, desktop, security` |
| Hypothesis | `Two different operator-owned Codex accounts can remain simultaneously authenticated in two Daemon-managed CODEX_HOME directories, with app-server identity/usage and scoped logout bound to the intended Home and no cross-profile credential mutation on macOS and Linux` |
| Time-box | `8 hands-on hours plus up to 24 hours passive observation; requires two accounts and one Linux target` |
| Current Phase | `PROVIDER SPIKE` |
| Status | `EVIDENCE_READY` |
| Executor | `Codex (GPT-5) as provider-spike` |
| Updated | `2026-07-16 16:31 PDT` |
| Suggested Next | `security-review` |
| Security Gate | `open — distinct OAuth identities, file credentials, refresh ownership, browser callback binding and logout are in scope` |
| Evidence Path | `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes-spike.md`; sanitized JSON sibling |
| Decision Record | `pending — ADR 0014 addendum or new ADR plus PROVIDER_COMPATIBILITY.md` |

## Success and failure criteria

- Supported when: two distinct identities complete official login into clean
  isolated Homes; exact-version app-server reads prove the expected binding;
  concurrent reads and a scoped logout/re-login never change the other Home;
  sanitized evidence reproduces on macOS and target Linux.
- Falsified when: either Home inherits/overwrites the other identity, browser
  callback cannot be bound/verified, app-server reports an unexpected identity,
  logout is global, a second refresh writer is required, or support depends on
  copying Cookie/session/auth state outside the reviewed Vault boundary.

## Environment

| Field | Value |
|---|---|
| Tool + version | initial macOS candidate Codex CLI `0.144.2`; exact Linux version to pin |
| OS | macOS 26.5.2 arm64 + operator-selected headless Linux |
| Auth mode | two distinct ChatGPT/Codex accounts; official interactive login; device auth is a separate experimental arm |

## Evidence Ledger

| Time | Command/evidence | Result | Artifact |
|---|---|---|---|
| 2026-07-15 01:24 PDT | fresh current CLI schema generation; prior ADR 0014/Spike review | rate-limit/usage schemas and file-store boundary exist; prior evidence only compared the same account on two devices and did not complete isolated device auth | Feature evidence ledger; prior Codex Spike |
| 2026-07-15 01:31 PDT | feature-plan intake | exact distinct-account, scoped-logout, callback and cross-mutation hypothesis frozen; Security Gate opened | this log |
| 2026-07-16 16:31 PDT | exact Linux `0.144.2` official-login experiment with two operator-owned identities; two concurrent Sessions/Usage reads; target-B active-logout negative, scoped stop/logout/re-login, and second concurrent run | exact Linux arm supported: identities and Provider sessions distinct, both auth files `0600`, Usage account-bound, B revision `2 -> 3`, and A unchanged throughout; final running Sessions/materialized auth files `0`; no secret or raw identity persisted | `docs/spikes/codex-distinct-accounts/2026-07-16-distinct-account-homes-spike.md`; sanitized JSON sibling |

## Result, limitations, and fallback

Supported for the exact Linux `x86_64` Codex CLI `0.144.2` arm, pending Security
Review. Two official interactive logins produced two isolated Vault-backed
managed Homes, two distinct concurrent Provider sessions, and account-bound
Usage. Stopping, logging out, and re-enrolling B did not change A; B reused the
same CredentialInstance with a monotonic revision. Raw identity values and
Provider secrets were not persisted.

This does not satisfy the original cross-platform criterion: a two-distinct-
identity macOS run, a real Windows run, and passive soak remain unexecuted.
Stable fallback is target-local official login in one explicitly selected Home,
explicit Profile selection, no automatic switch, and fail-closed quarantine on
identity/revision ambiguity. Device auth remains experimental.

## Risks and Blockers

- Requires two operator-owned accounts and explicit user participation in each
  official browser login; no CAPTCHA/session bypass is allowed.
- Identity values must be compared only in process memory and reduced to safe
  booleans/digests before evidence is persisted.
- ADR 0014 single-writer/CAS remains mandatory even if concurrent reads pass.
- No Provider Cookie, authorization URL/code or auth-file content may enter the
  repository, chat handoff, terminal log or dashboard state.

## Work Log (append only)

| Time | Executor | Action | Files/commit | Result | Next |
|---|---|---|---|---|---|
| 2026-07-15 01:31 PDT | Codex (GPT-5) as feature-plan spike intake | Created a bounded distinct-account Codex hypothesis from the new P0 requirement, retained ADR 0014, defined two-platform canaries and opened the required Security Gate | this log; parent Feature Brief/design/test | `SPIKE_READY` | `provider-spike` after operator supplies/selects two test accounts and Linux target |
| 2026-07-16 16:31 PDT | Codex (GPT-5) as provider-spike | Ran the exact Linux `0.144.2` two-identity official-login experiment, verified isolated `0600` Homes, distinct concurrent Provider sessions, concurrent account-bound Usage, active-logout fail-closed behavior, scoped B stop/logout/re-login with revision `2 -> 3`, unchanged A bytes/session/Usage, then stopped both Sessions, removed materializations, stopped the Daemon, and retained only sanitized evidence | spike report; sanitized JSON; `docs/PROVIDER_COMPATIBILITY.md` | `EVIDENCE_READY`; exact Linux arm supported without persisting identity or secret material; macOS distinct-identity, Windows, and passive-soak claims remain open | `security-review` |
| 2026-07-16 16:34 PDT | Codex (GPT-5) as operator-directed provider-spike writer | Bound dashboard manual focus to this unit's `EVIDENCE_READY` state, regenerated workflow/dashboard facts, and verified dashboard plus all local Markdown links with the bundled Node runtime | `docs/workflow/project/dashboard-state.json`; generated dashboard unchanged | workflow verify PASS; dashboard verify PASS; `252` Markdown files link-clean | `security-review` |
