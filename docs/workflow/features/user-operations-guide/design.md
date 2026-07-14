# Design: 面向用户的操作手册与看板入口

## Decision snapshot

- Selected option: one canonical Chinese pre-release guide at
  `docs/USER_GUIDE.md`, linked from README and surfaced as a required document
  in the development dashboard.
- Review evidence: the implementation plan already defines the user journeys,
  planned CLI surface, trust boundaries, phases, failures, and release gates;
  the repository has no task-oriented end-user guide.
- Frozen assumptions: product code has not started; only workflow/dashboard
  developer commands are currently executable; planned product commands must
  not be represented as live functionality.
- Rejected alternatives: expanding README into a long manual (hurts developer
  scanning); creating separate platform guides before installers exist
  (duplicates unstable instructions); using the dashboard as the sole manual
  (it is a development cockpit, not product documentation).

## Context and boundaries

This is a documentation/discoverability feature owned by `project-system`.
The guide describes journeys that cross all product modules but does not alter
their contracts or claim they are implemented. The implementation-plan phase
gates and each feature `dev_log.md` remain the source of truth for readiness.

## Components and ownership

### Canonical guide

`docs/USER_GUIDE.md` owns the user-facing sequence, status legend, safety
warnings, and troubleshooting map. It links to the implementation plan only
when a reader needs design detail.

### Discovery surfaces

- `README.md` owns the short project-status warning and links to the guide.
- `docs/IMPLEMENTATION_PLAN.md` owns the canonical document inventory.
- `docs/prototypes/dev-dashboard/index.html` owns the static cockpit card.
- `scripts/dashboard/generate-state.mjs` owns the machine fact that the guide
  exists and is non-empty.
- `scripts/dashboard/verify-static.mjs` enforces the required-doc fact and a
  static user-guide marker.

## Data flow and state transitions

1. Writer creates or updates the canonical guide.
2. The dashboard generator reads only file existence and byte length.
3. Generated state exposes the guide in `required_docs` and includes the
   feature `dev_log.md` in `feature_logs`.
4. The verifier rejects a missing/empty guide or a missing static cockpit
   entry; it does not infer feature approval, priority, Ship, or release.

## Failure and recovery

- Missing guide: dashboard verification fails; restore the file before merge.
- Stale planned command: mark it pending or update it only after the owning
  phase has verified evidence.
- Broken local link: fix the exact link; do not remove readiness warnings to
  make validation pass.
- Rollback: remove the three discovery references and required-doc assertion
  together, then remove the guide only if the feature is deliberately
  abandoned. Generated state can always be regenerated.

## Security and privacy

Examples use placeholders only. The guide explicitly prohibits putting Vault
passwords, Tokens, Cookies, recovery codes, setup tokens, or credential files
in command arguments, logs, screenshots, or issue reports. It preserves the
Passkey-versus-Device-Key and revocation-versus-remote-erasure distinctions.

## Compatibility and migration

The guide labels platform support according to the current plan: stable target
for CLI/Daemon on macOS, Windows, and Linux; macOS Desktop target; Windows
Desktop Experimental; Linux Control Plane. These are targets, not current
support claims. No data migration is introduced.

## Rollback

All changes are Markdown/static-dashboard contracts. Rollback is a file-local
revert with no runtime data, schema, Provider, or credential impact.
