// Package codex contains the version-gated, fail-closed Codex app-server
// contract. P2 adds the typed Vault-to-Codex credential materialization
// boundary, isolated auth homes, one canonical writer, revisioned CAS, and
// quarantine recovery. P3A adds one shared CredentialRuntime per credential,
// per-Session thread/turn bindings, a single-reader protocol multiplexer,
// exact Approval dispatch, Usage projection, and binding-scoped controls. It
// still does not claim a live Provider Session without credentialed Linux
// evidence.
package codex
