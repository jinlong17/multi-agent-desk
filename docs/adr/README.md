# Architecture Decision Records

One file per irreversible architecture decision. Numbering is permanent; a
superseded ADR stays in place and links forward.

ADR 0001–0008 are reserved for the first batch defined in
`docs/IMPLEMENTATION_PLAN.md` §18 and are a Phase 0 deliverable
(`phase0-architecture-adrs`):

| # | Recorded decision | Status |
|---|---|---|
| [0001](0001-unified-go-react-tauri-architecture.md) | Unified Go, React, and Tauri architecture | Accepted |
| [0002](0002-device-daemon-owns-secrets-and-provider-processes.md) | Device Daemon owns secrets and Provider processes | Accepted |
| [0003](0003-control-plane-metadata-and-ciphertext-only.md) | Control Plane routes metadata and ciphertext only | Accepted |
| [0004](0004-asymmetric-codex-and-claude-integration.md) | Codex and Claude integrations remain asymmetric | Accepted |
| [0005](0005-sqlite-for-v0-1.md) | SQLite for v0.1; PostgreSQL deferred | Accepted |
| [0006](0006-external-adapters-over-stdio-json-rpc.md) | External adapters use stdio JSON-RPC, not Go plugins | Accepted |
| [0007](0007-user-confirms-account-selection.md) | System recommends; user confirms account selection | Accepted |
| [0008](0008-config-sync-separated-from-credential-grants.md) | Configuration sync is separate from credential grants | Accepted |
| [0009](0009-repository-layout-authority.md) | Repository layout authority | Accepted |
| [0010](0010-browser-device-key-storage-modes.md) | Browser device key storage modes | Accepted |
| [0011](0011-pairwise-e2ee-session-protocol.md) | Pairwise E2EE session protocol | Accepted |
| [0012](0012-windows-conpty-pty-backend.md) | Windows ConPTY PTY backend | Accepted |
| [0013](0013-windows-named-pipe-local-ipc.md) | Windows Named Pipe local IPC | Accepted |
| [0014](0014-codex-app-server-single-writer-auth.md) | Codex app-server single-writer authentication boundary | Accepted |

ADR 0001–0008 accept only the broad Plan v0.2 boundaries. Their
`Spike-gated details` sections are authoritative markers for decisions that
remain pending reproducible Phase 0.5 evidence.

ADR 0010 through ADR 0014 are Phase 0.5 evidence-backed decisions. ADR 0010
resolves browser key storage compatibility; ADR 0011 resolves the E2EE protocol
candidate with per-peer roots and cross-language vectors; ADR 0012 selects the
Windows ConPTY backend; ADR 0013 selects Windows Named Pipes with a protected
current-logon boundary and mandatory protocol authorization; ADR 0014 selects
one canonical writable Codex app-server/auth home per CredentialInstance. Both
Windows ADRs preserve Windows 11 acceptance gates. ADR 0014 does not claim
multi-writer refresh or completed headless device auth. None of these ADRs
claims that its production implementation is complete.
