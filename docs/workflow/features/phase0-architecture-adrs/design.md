# Design: Phase 0 architecture ADR batch

## Decision snapshot

- Owner: `project-system`; documentation only.
- One build phase creates ADR 0001–0008, indexes them, and adds the initial
  research and Provider compatibility scaffolds.
- Each ADR records an implementation-plan decision but explicitly labels
  Phase 0.5-dependent details as unresolved rather than freezing them.

## ADR structure

Every ADR contains Status, Date, Owner module, Context, Decision,
Spike-gated details, Consequences, and References. Status is `Accepted` only
for the broad v0.1 boundary already accepted in Plan v0.2. A broad decision
must not imply that Provider versions, undocumented behavior, key storage,
Windows transport, or E2EE protocol details have been proven.

The batch maps as follows:

1. unified Go + React + Tauri architecture;
2. Device Daemon ownership of secrets and Provider processes;
3. metadata/ciphertext-only Control Plane;
4. asymmetric built-in Provider integration boundary;
5. SQLite for v0.1 with PostgreSQL deferred;
6. external adapters over stdio JSON-RPC rather than Go plugins;
7. system recommends and user confirms, with no automatic rotation;
8. non-secret configuration sync separated from explicit credential grants.

## Research and compatibility scaffolds

`RESEARCH_LOG.md` defines a dated, source-scoped ledger for external research,
including the AGPL clean-room boundary. It makes no claim that research has
already occurred. `PROVIDER_COMPATIBILITY.md` defines evidence columns and
lists Phase 0.5 gates as pending; it contains no supported-version claim.

## Failure and rollback

Broken links, decision drift, unmarked Spike assumptions, or compatibility
claims without evidence fail verification. The phase is documentation-only and
can be rolled back as one commit without data or runtime migration.
