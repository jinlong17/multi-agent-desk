# Agent source authority

`.agents/registry.json` and `.agents/roles/*.md` are the cross-runtime authority.
Do not hand-edit generated files under `.codex/agents/`, `.claude/agents/`, or
`.claude/skills/`. Regenerate them with `npm run workflow:generate`.
