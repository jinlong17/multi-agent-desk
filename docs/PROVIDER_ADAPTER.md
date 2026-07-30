# Provider adapter

This is a derived Provider-boundary view, not a public adapter protocol or a
new compatibility authority. Provider differences remain intentional. The
dated [product snapshot](PRODUCT.md), exact [compatibility matrix](PROVIDER_COMPATIBILITY.md),
and immutable receipt linked by the [claim ledger](reviews/as-built-product-docs/claim-ledger.md)
control every Provider claim. ADR [0004](adr/0004-asymmetric-codex-and-claude-integration.md)
and [0006](adr/0006-external-adapters-over-stdio-json-rpc.md) remain the target
boundary decisions.

## Current bounded positions

| Area | Class | Exact boundary and fallback |
|---|---|---|
| Codex vertical slice and explicit selector | `supported` | Only CLI `0.144.2` on exact Linux `x86_64`/`amd64`, in the pre-release `source-built` scope documented by [CLM-012](reviews/as-built-product-docs/claim-ledger.md#clm-012). Use official interactive login; unknown version/schema fails closed. macOS selector identity acceptance is pending and real Windows Codex is unsupported. |
| Other combined Codex account/auth/usage behavior | `unknown` | No single current receipt covers the broad combined scope; bind future wording to an exact matrix row and receipt or open a provider spike. |
| Claude managed subscription/Team CLI-PTY, quota dashboard, setup-token grant, long session | `unsupported` | Use direct official Claude Code outside the managed surface, or a separately planned user-supplied API-key/supported-cloud path with explicit billing; see [CLM-014](reviews/as-built-product-docs/claim-ledger.md#clm-014). |
| Public Adapter SDK/protocol | `unknown` | This document defines none. The v0.1 target remains asymmetric built-in adapters; do not infer a stable extension contract. |

## Consumer rules

- A configured binary, schema handshake, CI run, dashboard card, or account
  metadata is not a Provider support claim.
- A positive Provider statement needs a current, reachable, exact immutable
  receipt for the tool, version, platform, capability, result, and fallback.
  Missing, stale, contradictory, or out-of-scope evidence is `unknown` and
  opens `Provider Gate: provider evidence refresh`.
- No adapter may automate account rotation, quota bypass, rate-limit evasion,
  cookie scraping, Provider request proxying, or transparent mid-session
  credential switching.

For the complete per-row evidence and fallback, use
[PROVIDER_COMPATIBILITY.md](PROVIDER_COMPATIBILITY.md), not this summary.
