# ADR 0009: Repository layout authority

- Status: Accepted
- Date: 2026-07-10
- Owner module: `project-system`
- Related: `docs/IMPLEMENTATION_PLAN.md` §17, `docs/workflow/project/module-registry.json`,
  `docs/reviews/lifecycle-readiness/2026-07-10-lifecycle-readiness-review.md`

## Context

Two conflicting repository layouts were documented as authoritative:

1. `docs/IMPLEMENTATION_PLAN.md` §17 defines a Go workspace monorepo:
   `cmd/{multidesk,multidesk-server}` + `internal/*` + `apps/{web,desktop}` +
   `packages/{ui,protocol,config}` + `api/` + `migrations/` + `deploy/`.
2. `CLAUDE.md` and `docs/workflow/project/module-registry.json` described
   component boundaries as separate top-level packages: `apps/daemon`,
   `apps/cli`, `apps/server`, `packages/provider-*`, `packages/domain`,
   `packages/crypto`.

The conflict affects module ownership, branch naming, CODEOWNERS, CI path
filters, and the write scope of every workflow agent. It must be resolved
before the Phase 0 monorepo scaffold (`phase0-monorepo-scaffold`) is created.

## Decision

The physical repository layout in `docs/IMPLEMENTATION_PLAN.md` §17 is the
single authority.

Rationale:

- ADR 0001 (reserved) fixes the stack as one Go module plus a pnpm workspace.
  `cmd/` + `internal/` is the idiomatic layout for multiple Go binaries
  (daemon/CLI and server) sharing internal packages; separate `apps/daemon`,
  `apps/cli`, `apps/server` directories imply separate modules that do not
  exist in this stack.
- Provider adapters are Go code compiled into the daemon
  (`internal/providers/*`), not distributable TypeScript packages, so
  `packages/provider-*` misstates the artifact boundary.
- Plan v0.2 §17 already passed independent review
  (`docs/reviews/2026-07-10-fable5-high-plan-review.md`); the `apps/*`
  wording in `CLAUDE.md` predates it.

The `apps/daemon`-style names in earlier documents are retired as *logical
component names only*. The mapping from logical component to physical path is
owned by `docs/workflow/project/module-registry.json`:

| Module | Physical paths |
|---|---|
| `core` (Device Kernel: daemon + CLI/TUI) | `cmd/multidesk`, `internal/app`, `internal/domain`, `internal/runtime`, `internal/device`, `internal/vault`, `internal/storage`, `migrations/device` |
| `provider` | `internal/providers`, `internal/providerprotocol` |
| `control-plane` | `cmd/multidesk-server`, `internal/controlplane`, `internal/transport`, `api`, `migrations/server` |
| `web` | `apps/web`, `packages/ui`, `packages/protocol` |
| `desktop` | `apps/desktop` |
| `security` | `internal/crypto`, `docs/security`, `docs/THREAT_MODEL.md` |
| `project-system` | `.agents`, `.codex`, `.claude`, `.github`, `docs/adr`, `docs/workflow`, `docs/prototypes/dev-dashboard`, `scripts/dashboard`, `scripts/workflow` |

## Consequences

- `CLAUDE.md` architecture boundaries and the module registry `owns` arrays
  are rewritten against physical paths (done in `lifecycle-readiness`).
- `phase0-monorepo-scaffold` creates exactly the §17 skeleton; no
  `apps/daemon`, `apps/cli`, `apps/server`, or `packages/provider-*`
  directories may be created.
- Branch naming (`codex/<module>/<feature>`) and human gates are unchanged;
  module keys stay stable even though their owned paths changed.
- CODEOWNERS and CI path filters (Phase 0, `phase0-ci-governance`) must be
  generated from the module registry, not hand-written path lists.
- `internal/vault` is owned by `core` (Vault runtime behavior lives in the
  daemon); `internal/crypto` and the threat model are owned by `security`.
  Security review gates still apply to any change under either path.
