# Development dashboard machine contract

## Identity

- Human cockpit: `docs/prototypes/dev-dashboard/index.html`
- Manual state: `docs/workflow/project/dashboard-state.json`
- Module authority: `docs/workflow/project/module-registry.json`
- Agent/Skill authority: `.agents/registry.json` and `.codex/skills/`
- Feature state: `docs/workflow/features/*/dev_log.md`
- Generated facts: `docs/prototypes/dev-dashboard/state.generated.js`
- Generator: `scripts/dashboard/generate-state.mjs`
- Verifier: `scripts/dashboard/verify-static.mjs`
- Local server: `scripts/dashboard/serve.mjs`

## Authority model

The dashboard uses three classes of data:

1. **Manual judgment** — phase status, priority, next action, accepted risk,
   and the `focus` bindings. `dashboard-state.json` is operator judgment: a
   refresh may be executed by an operator-directed writer session, which must
   record the refresh in the target unit's Work Log; verdict writers never
   touch this file.
2. **Machine facts** — branch, commit, dirty files, existing docs, agent/skill
   mirrors, feature logs. The generator owns these fields.
3. **Derived display** — counts and summaries calculated from the first two.

The generator must never infer approval, Ship, release, risk acceptance, or
phase completion from Git activity or passing tests.

## Refresh and verification

```bash
npm run workflow:generate
npm run dashboard
npm run dashboard:verify
npm run dashboard:serve
```

The server binds to `127.0.0.1` only. The generated snapshot is safe to rebuild
and must contain no secrets, token values, environment contents, cookies, or
credential paths beyond repository-relative project files.

## Change rules

- Change manual priority/status only with operator direction or explicit plan evidence.
- Keep module definitions in `module-registry.json`; never add a second JS module map.
- Keep role definitions in `.agents/roles`; runtime files are generated mirrors.
- A dashboard UI change must preserve readable static fallback if generated state is absent.
- Verification failure remains failure until the exact check passes.
