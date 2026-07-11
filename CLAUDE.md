# MultiAgentDesk project instructions

MultiAgentDesk is a terminal-first, self-hostable workspace for managing AI
coding-agent profiles, sessions, usage, devices, and explicitly authorized
credential grants across local machines and remote servers.

## Architecture boundaries

- `apps/daemon`: device-local fact source; owns Vault, credential materialization,
  provider processes, PTY, sessions, attachments, and controller leases.
- `apps/cli`: terminal/TUI client over local IPC; never reads the database as an API.
- `apps/server`: Control Plane; owns identity, device metadata, encrypted sync,
  commands, and ciphertext relay; never receives provider plaintext credentials.
- `apps/web`: dashboard and approved remote-control client; unpaired clients are metadata-only.
- `apps/desktop`: Tauri shell around shared Web UI plus OS integration and sidecar lifecycle.
- `packages/provider-*`: provider-specific adapters. Codex and Claude capabilities
  remain asymmetric and must be represented by a capability matrix.
- `packages/protocol`, `packages/domain`, and `packages/crypto`: shared contracts
  only; provider business behavior stays in its adapter.

## Security invariants

- Passkey authenticates a user; a pinned Device Key authorizes decryption.
- The Control Plane is not a key trust anchor and cannot replace pinned keys.
- Credential grants are explicit, target-device scoped, encrypted, revocable,
  and never described as remotely erasable after compromise.
- Credential refresh has one writer and uses revision/CAS semantics.
- Never implement automatic account rotation, quota bypass, or rate-limit evasion.

## Workflow

Follow `docs/workflow/project/workflow.md`, `AGENTS.md`, and the role source in
`.agents/roles/`. Runtime files under `.codex/agents/` and `.claude/agents/` are
generated mirrors; edit the role source and run `npm run workflow:generate`.

Project Skills are authored in `.codex/skills/` and mirrored to
`.claude/skills/` by the same generator. Update the source Skill, not its mirror.

For every feature, create:

- `docs/reviews/<slug>/<date>-feature-brief.md`
- `docs/workflow/features/<slug>/design.md`
- `docs/workflow/features/<slug>/api.md`
- `docs/workflow/features/<slug>/test.md`
- `docs/workflow/features/<slug>/dev_log.md`

Run `npm run project:verify` after changing workflow, agent, skill, registry, or
dashboard files.
