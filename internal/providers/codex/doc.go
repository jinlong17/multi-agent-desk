// Package codex contains the version-gated, fail-closed Codex app-server
// contract. P2 adds the typed Vault-to-Codex credential materialization
// boundary, isolated auth homes, one canonical writer, revisioned CAS, and
// quarantine recovery. P3 adds a bounded ProviderSession event/Usage/Approval
// adapter with explicit stop/resume fail-closed behavior; it still does not
// claim a live Provider Session without credentialed Linux evidence.
package codex
