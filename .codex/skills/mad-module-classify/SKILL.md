---
name: mad-module-classify
description: Classify a MultiAgentDesk requirement, bug, refactor, or documentation change into exactly one owning module and identify secondary impacts, branch naming, workflow, and security/provider gates. Use before starting any project task, when a change spans Web/Desktop/CLI/Daemon/Control Plane/Provider code, or when checking boundary drift.
---

# Module classification

Read `docs/workflow/project/module-registry.json`, `CLAUDE.md`, and the changed
paths. Match the request to exactly one owner:

- `core`: Daemon, local IPC, domain, database, session/attachment/lease.
- `provider`: Codex/Claude adapters, PTY, auth, usage, compatibility fixtures.
- `control-plane`: identity, device directory, metadata sync, commands, relay.
- `web`: browser dashboard, remote terminal, approval UI, Web Device Key.
- `desktop`: Tauri shell, OS Keychain, sidecar, tray, deep link, packaging.
- `security`: crypto protocol, Vault, attestation, credential grant, threats.
- `project-system`: plans, workflow, skills, agents, CI, dashboard, governance.

If two owners appear equal, stop and request an operator decision. Physical file
location does not override product responsibility: a Tauri command supporting
credential grants may be implemented in Desktop but owned by Security.

## Output

Return:

```text
Owner: <module>
Confidence: high | medium | low
Why: <matched signals and boundaries>
Impacts: <secondary modules or none>
Branch: codex/<module>/<feature>
Workflow: feature | bugfix | spike
Gates: <provider/security/phase gates or none>
Docs: <feature brief and feature-state paths>
```

Do not create a branch, approve priority, unfreeze a phase, or edit product code.
