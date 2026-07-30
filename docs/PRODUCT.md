# MultiAgentDesk product

> Canonical as-built snapshot — **2026-07-29 PDT**
>
> Ledger revision: [2](reviews/as-built-product-docs/claim-ledger.md) ·
> verification baseline:
> `397194bfaea6983c4c0a54289d8bb1844711cfd6` · product state: **pre-release,
> source-built only**.

This is the dated product-facing synthesis for MultiAgentDesk. It is not a
release announcement, a replacement for a feature's live `dev_log.md`, or
evidence that a target architecture, green structural check, branch, or
dashboard is supported. Claims use the [verified claim ledger](reviews/as-built-product-docs/claim-ledger.md)
and its six classes: `planned`, `preview`, `supported`, `experimental`,
`unsupported`, and `unknown`.

## Product position

MultiAgentDesk is a local-first, self-hostable workspace intended to manage AI
coding-agent profiles, sessions, usage, devices, and explicitly authorized
credential grants across local machines and remote servers. The reviewed v0.1
model is a target, not a delivered product contract. The current repository is
for a **source-built developer preview**, with no supported installer, release,
deployment, or broad platform promise.

The intended product keeps Provider credentials device-local, uses explicit
user confirmation rather than automatic account switching, and treats remote
features as separately gated. For the complete target, see the [implementation
plan](IMPLEMENTATION_PLAN.md); for mutable lifecycle state, see the relevant
[feature logs](workflow/features/).

## Current status snapshot

This snapshot is current only for the dated ledger revision above. It does not
change the state of any feature. The P1 evidence inventory for this document is
[independently verified](reviews/as-built-product-docs/2026-07-29-feature-verify.md),
but this P2 document is itself awaiting independent verification.

| Area | Snapshot | What it does not mean |
|---|---|---|
| Repository tooling | `preview` — `source-built` workflow/dashboard tooling is runnable with the repository's pinned Node and pnpm environment. | A product daemon, installer, release, platform acceptance, or deployment is not started or proved by these tools. |
| Local product foundations | `preview` — the documented Device schema and bounded local developer path exist in source. | This is not equivalent Windows ACL protection, a release package, or a general runtime-support claim. |
| Remote product flows | `planned` — Control Plane deployment, pairing, Web/Desktop remote interaction, Credential Grants, and revocation orchestration remain gated. | No current remote deployment, remote terminal, approval, or credential-grant capability is claimed. |
| Release | `planned` — packaging, upgrade, distribution, and release acceptance remain later work. | Integration, CI, or a feature branch is not a ship decision. |

## Current capability matrix

Each row is constrained by its linked ledger record. A positive Provider row
appears only where the ledger records a current, reachable, scope-matching
immutable receipt. All other Provider claims remain narrowed or unknown.

| Capability | Class | Exact scope and evidence | Safe fallback or remaining gate |
|---|---|---|---|
| Repository workflow and dashboard | `preview` | `source-built` development tooling only; [CLM-003](reviews/as-built-product-docs/claim-ledger.md#clm-003). | Read feature evidence for product behavior; structural checks are not product acceptance. |
| Device schema/storage boundary | `preview` | `source-built` Phase 1 P1 schema: single writer, SQLite/WAL, Unix private-root/file boundary; [CLM-006](reviews/as-built-product-docs/claim-ledger.md#clm-006). | Windows ACL and broader runtime/security acceptance remain gated. |
| Codex vertical slice and explicit selector | `supported` | **Only** Codex CLI `0.144.2` on Linux `x86_64`/`amd64`, in a pre-release `source-built` path: bounded vertical-slice/selector behavior with explicit alias confirmation; [CLM-012](reviews/as-built-product-docs/claim-ledger.md#clm-012), [Phase 2 verification](reviews/phase2-codex-vertical-slice/2026-07-16-feature-verify-p3b.md), and [selector verification](reviews/codex-multi-account-selector/2026-07-20-feature-verify.md). | Use official interactive login on the accepted Linux scope. macOS selector identity acceptance is pending; real Windows Codex is unsupported; unknown version/schema fails closed. |
| Codex canonical schema scope | `supported` | **Only** the ledger's exact Codex `0.144.2` schema/Phase 2 Linux scope, with macOS schema/empty-home smoke explicitly distinct from identity acceptance; [CLM-020](reviews/as-built-product-docs/claim-ledger.md#clm-020). | Probe the exact schema and fail closed. This is not general cross-platform session support. |
| Other Codex account/auth/usage conclusions | `unknown` | No one current immutable receipt covers the broad combined scope; [CLM-013](reviews/as-built-product-docs/claim-ledger.md#clm-013). | Bind a future statement to an exact matrix row and Provider receipt, or open a provider-owned spike. |
| Claude managed subscription, Team CLI/PTY, quota dashboard, setup-token grant, and long session | `unsupported` | The named stable managed Claude surfaces are excluded by exact negative/security-reviewed evidence; [CLM-014](reviews/as-built-product-docs/claim-ledger.md#clm-014). | Use direct official Claude Code outside the managed surface, or a separately planned user-supplied API-key/supported-cloud path with explicit billing. |
| Browser/Windows/sidecar mechanisms | `experimental` | Only the dated compatibility rows' exact browser, runner, toolchain, and fallback scopes; [CLM-011](reviews/as-built-product-docs/claim-ledger.md#clm-011). | Product-feature, security, and Windows 11 acceptance gates remain open; this is not Web/Desktop product support. |
| Control Plane deployment, Device pairing, Web/Desktop control, grants, and revocation orchestration | `planned` | Phase 4a/4b/5 flows; [CLM-015](reviews/as-built-product-docs/claim-ledger.md#clm-015), [CLM-021](reviews/as-built-product-docs/claim-ledger.md#clm-021), [CLM-022](reviews/as-built-product-docs/claim-ledger.md#clm-022), [CLM-023](reviews/as-built-product-docs/claim-ledger.md#clm-023). | Use the local source-built workflow only; remote/E2EE/security gates cannot be bypassed. |
| Aggregate production security posture | `unknown` | Existing threat rows mix narrow verified mechanisms, planned enforcement, and residual risk; [CLM-019](reviews/as-built-product-docs/claim-ledger.md#clm-019). | Retain each threat's exact evidence state and owning gate. |

### Freshness and downgrade rule

The snapshot is not timeless. On day 31 after its snapshot date, every current
`preview`, `supported`, or `experimental` row must become `unknown` with
`evidence_state=stale` and the gate `snapshot refresh`. A positive Provider
claim also becomes `unknown` when its receipt is older than 90 days, unreachable
from the baseline, contradictory, or not an exact tool/version/platform/
capability match; its gate is `Provider Gate: provider evidence refresh`.
Those downgrades are defined by the [claim ledger](reviews/as-built-product-docs/claim-ledger.md#snapshot-binding), not waived by this document.

## Non-goals and policy boundaries

MultiAgentDesk does not provide automatic account rotation, quota or rate-limit
bypass, Provider request proxying, browser-cookie scraping, or silent
mid-session credential switching. It is not a hosted multi-tenant service,
Provider login proxy, or a mechanism for combining subscriptions into a pool.

Provider differences are deliberate. A configuration file, dashboard card, CI
result, binary presence, or app-server handshake is not a compatibility claim.
The exact [Provider compatibility matrix](PROVIDER_COMPATIBILITY.md) and its
linked immutable evidence control Provider wording.

## Trust and data boundaries

The [threat model](THREAT_MODEL.md) is authoritative for trust assumptions,
required mitigations, and residual risk. This snapshot preserves the following
boundaries without claiming that every production enforcement point is complete:

- A Passkey authenticates a user to the Control Plane; it does **not** grant
  E2EE Device-key decryption authority.
- The Control Plane public-key directory is an index, not a trust anchor; it
  cannot silently replace locally pinned Device keys.
- Credential Grants are explicit, target-device scoped, encrypted, and
  revocable for future use. They cannot remotely erase plaintext already copied
  to a compromised target or host.
- Provider-readable authentication state can exist at runtime. Host root/admin,
  a compromised Provider process, backup/crash tooling, and an authorized but
  compromised target remain meaningful risks.
- An unpaired browser is metadata-only. Future Web Device enrollment, pairing,
  remote control, terminal decryption, approvals, and Credential Grants remain
  gated; clearing browser site data requires a new Device ID and re-pairing.

Any substantive change to credential, key, E2EE, enrollment, revocation,
logging, or residual-risk wording requires the feature's later independent
security review. This document does not resolve that Security Gate.

## Evidence and documentation authority

| Question | Authority | How this document uses it |
|---|---|---|
| What v0.1 is intended to become | [Implementation plan](IMPLEMENTATION_PLAN.md) and accepted ADRs | Describes it as `planned`, never as current support by itself. |
| What a feature's lifecycle state is | Its [feature `dev_log.md`](workflow/features/) and independent verdicts | Links out; does not copy mutable phase state as authority. |
| What a Provider/platform combination proves | [Provider compatibility matrix](PROVIDER_COMPATIBILITY.md), exact receipt, and feature/spike evidence | Retains exact version, platform, capability, fallback, and evidence date. |
| What a trust statement means | [Threat model](THREAT_MODEL.md), ADRs, implementation evidence, and later security review | Retains qualifications and residual risk; does not self-accept security risk. |
| What this dated product snapshot says | [Claim ledger](reviews/as-built-product-docs/claim-ledger.md) | Synthesizes bounded evidence and visibly downgrades stale or missing proof. |

If sources differ, the most specific current independently reviewed evidence
controls. The safe response is to narrow the statement to `unknown`,
`planned`, `experimental`, or `unsupported`—never to choose the optimistic
interpretation. Report documentation drift through the relevant feature
workflow; do not edit a derived surface to override lifecycle, Provider, or
security evidence.
