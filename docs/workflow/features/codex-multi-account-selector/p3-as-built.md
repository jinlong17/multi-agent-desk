# P3 as-built: platform and documentation closure

## Platform gates

The multi-account selector now has one explicit platform gate used at both
external boundaries:

1. `sessions.preview` checks the discovered descriptor before schema probing,
   preview issuance, Session reservation, credential materialization, or spawn.
2. `Runtime.StartReserved` repeats the check after reservation and before
   binary fingerprinting/materialization/spawn, so an injected or drifted
   non-Linux descriptor fails the persisted Session closed.

The accepted matrix is deliberately narrower than general Codex app-server
schema compatibility:

| Platform | Selector result |
|---|---|
| Linux `amd64` | may continue to exact version/schema checks; live support remains Codex CLI `0.144.2` only |
| macOS `darwin` | `schema_compatible_identity_acceptance_pending`; no selector-owned Home or Provider process |
| Windows | `provider_platform_unsupported`; no selector-owned Home or Provider process |
| other Linux architecture or unknown platform | `provider_platform_unsupported`; no fallback |

macOS schema/empty-home smoke evidence is retained as schema evidence only. It
does not prove two distinct identities, scoped logout/re-login, or full Session
operation. Windows remains compilation/protocol evidence only.

## User and compatibility documentation

- `docs/USER_GUIDE.md` now uses the actual `accounts add`, public `@alias`
  login/status/Usage, human `run codex`, and TUI commands. The obsolete public
  raw Account/Profile/Credential ID start example was removed.
- The guide names the exact Linux acceptance, the macOS pending status, the
  Windows unsupported status, and the fact that this feature branch has not
  been shipped, merged, packaged, or released.
- `docs/PROVIDER_COMPATIBILITY.md` records the selector evidence and separates
  general app-server schema compatibility from selector identity acceptance.
- The feature API and test documents include the stable macOS pending code and
  the post-reservation platform defense.

## Verification boundary

The writer matrix passed full Go tests/vet/race, platform-gate race repetition,
Darwin arm64/Linux amd64/Windows amd64 builds, Web TypeScript/build, and Desktop
Rust fmt/check. Cross-build success is not runtime acceptance.

The feature Security Gate remains open. Independent P3 verification must issue
`READY_TO_SHIP` before the independent Security Review can decide whether the
gate is accepted. No push, merge, release, or ship is authorized by this phase.
