# Security Review: Codex Vertical Slice

## Verdict

`REVISE`

The Vault, enrollment, materialization, shared-runtime, JSON-RPC, Approval,
lease, and version/schema boundaries have strong test and live evidence, and
the focused security race suite passes. Two P1 issues prevent accepting the
Security Gate: the inherited `NO_PROXY` value is not structurally allowlisted
despite the credential-free claim, and the current repository evidence records
operator identity/MFA metadata that is unnecessary for proving the flow.

## Scope and evidence

- Owner: `provider`; impacts: `core`, `security`, `project-system`
- Reviewed the P4 Security handoff, threat model, ADR 0014, compatibility
  matrix, P3B/P4 verification, feature state authority, and implementation
  under Provider, Vault, Store, runtime, app, and CLI boundaries
- `go test -count=1 -race ./internal/vault ./internal/storage
  ./internal/providers/codex ./internal/runtime ./internal/app ./cmd/multidesk`
  passes
- Exact macOS `0.144.2` smoke, full Go/vet/race, three-platform build,
  Windows-target compilation, native Windows CI, governance, and diff checks
  are retained by the final P4 verification record
- A changed-and-untracked artifact scan found no token-shaped credential value;
  it did identify unnecessary identity/MFA metadata in the feature log

## Findings

### P1 — `NO_PROXY` crosses the Provider boundary without a structural allowlist

`internal/providers/codex/environment.go:39-45` accepts every non-empty
`no_proxy` / `NO_PROXY` value up to 4096 bytes after only NUL/CR/LF filtering.
Unlike the HTTP proxy branches, it does not reject userinfo-like content,
key/value data, paths, queries, fragments, whitespace, or non-host tokens.
All three official child paths then inherit the value. This contradicts the
documented "credential-free ... no-proxy entries" boundary and can disclose
arbitrary ambient environment content to the Provider child.

Clearing condition: define and implement a bounded `NO_PROXY` host/IP/CIDR /
optional-port grammar (or remove inheritance), reject non-network tokens, add
positive IPv4/IPv6/domain/CIDR tests and adversarial secret/key-value/path /
userinfo/control/count/size tests, and re-run focused race plus live Provider
health evidence.

### P1 — Repository evidence contains unnecessary operator identity/MFA metadata

The P3B Evidence Ledger in
`docs/workflow/features/phase2-codex-vertical-slice/dev_log.md` includes an
operator account display name and the final digits of the MFA phone number in
two historical rows. Neither value is needed to prove explicit owner selection
or MFA completion, and the P4 artifact scan incorrectly called the evidence
sanitized while these identifiers remained in the diff.

Clearing condition: security-redact those values to non-identifying labels,
record that redaction without changing the event outcome, scan every modified
and untracked artifact for token/email/account/MFA identifiers, and update the
P4/security evidence wording to the exact clean result.

## Controls that passed

- Portable Vault v1 crypto/AAD/tamper/wrong-key/corruption/init-race/item-CAS
  contracts and private-file enforcement pass.
- Enrollment is owner/deadline/idempotency bound, binary-pinned, imports only
  the exact bounded private `auth.json`, preserves prior credentials on
  failure, zeroes the import buffer, and removes terminal staging.
- One CredentialRuntime owns the writable home/app-server; lease, revision CAS,
  digest validation, quarantine, crash fan-out, and finalization tests pass.
- The single bounded JSON-RPC writer/reader, strict method/field routing,
  non-persistence of patch/diff/status payloads, and unknown-schema failures
  remain closed.
- Approval claim/write/ambiguity/replay, disabled permissions/persistent/policy
  variants, stale-lease rejection, typed resize, and Resume no-mutation tests
  pass.
- HTTP(S) proxy URLs reject credentials, non-HTTP schemes, paths, query,
  fragment, and control characters; only the separate `NO_PROXY` branch needs
  revision.

## Residual risk

Even after the findings are cleared, root/admin, a compromised daemon or
Provider binary, same-user live-process inspection, backups, or crash tooling
can read usable materialized credentials. Multi-writer refresh, completed
device auth, Provider continuation, dynamic policy amendments, permissions
grants, real Windows Codex, bundled macOS `0.144.5`, remote credential grants,
packaging, release, and deployment remain unsupported or outside this feature.
Revocation cannot erase secrets already copied by an authorized or compromised
host.

## Handoff

**Target**: `phase2-codex-vertical-slice credential/runtime and evidence boundary`
**Completed**: `security-review`
**Verdict**: `REVISE`
**Summary**: `Core Vault, enrollment, runtime, protocol, Approval, and lease controls pass, but unrestricted NO_PROXY inheritance and repository identity/MFA metadata must be removed before the Security Gate can be accepted.`
**Findings**: `P1 unrestricted NO_PROXY content crosses into Provider children; P1 feature evidence contains unnecessary operator identity/MFA metadata.`
**Evidence**: `code and artifact inspection; focused full security race suite; P3B/P4 live and platform verification records; changed-and-untracked sensitive-data scan.`
**Residual Risk**: `runtime-readable credentials under host or Provider compromise; unsupported multi-writer/device-auth/continuation/real-Windows/remote-grant/release boundaries remain explicit.`

### Next Step

Run `feature-plan` to incorporate the two security clearing conditions, then re-enter review/build/verify/security-review.
