# Architecture Decision Records

One file per irreversible architecture decision. Numbering is permanent; a
superseded ADR stays in place and links forward.

ADR 0001–0008 are reserved for the first batch defined in
`docs/IMPLEMENTATION_PLAN.md` §18 and are a Phase 0 deliverable
(`phase0-architecture-adrs`):

| # | Reserved title |
|---|---|
| 0001 | Go + React + Tauri unified architecture |
| 0002 | Device Daemon holds secrets and provider processes |
| 0003 | Control Plane handles metadata and ciphertext routing only |
| 0004 | Asymmetric integration: Codex app-server vs Claude PTY |
| 0005 | SQLite for v0.1, PostgreSQL deferred |
| 0006 | External adapters over stdio JSON-RPC, not Go plugins |
| 0007 | System recommends, user confirms; no automatic account rotation |
| 0008 | Config sync separated from credential grants |

| # | Recorded decision | Status |
|---|---|---|
| [0009](0009-repository-layout-authority.md) | Repository layout authority | Accepted |
