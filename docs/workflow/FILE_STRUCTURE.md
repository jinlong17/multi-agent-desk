# Workflow and dashboard file structure

## Authority tree

```text
multi-agent-desk/
├── AGENTS.md                          Codex/session workflow rules
├── CLAUDE.md                          shared architecture and security rules
├── .agents/
│   ├── registry.json                  Agent/Skill and pipeline authority
│   └── roles/*.md                     role behavior + Handoff contracts
├── .codex/
│   ├── config.toml                    local agent concurrency policy
│   ├── agents/*.toml                  generated Codex role entrypoints
│   └── skills/*/SKILL.md              project Skill authority
├── .claude/
│   ├── agents/*.md                    generated Claude role entrypoints
│   ├── skills/*/SKILL.md              generated Skill mirrors
│   └── hooks/                         explicitly configured local hooks
├── .github/
│   ├── ISSUE_TEMPLATE/                intake forms
│   └── PULL_REQUEST_TEMPLATE.md       workflow/evidence gate
├── docs/
│   ├── IMPLEMENTATION_PLAN.md         product and architecture baseline
│   ├── reviews/<slug>/                feature briefs and independent reviews
│   ├── workflow/
│   │   ├── project/
│   │   │   ├── workflow.md            lifecycle/state-machine authority
│   │   │   ├── module-registry.json   module routing authority
│   │   │   ├── dashboard-state.json   human-maintained dashboard judgment
│   │   │   └── dev-dashboard.md       machine dashboard contract
│   │   ├── templates/                 feature document templates
│   │   ├── features/<slug>/           design/api/test/dev_log continuity
│   │   └── SOP_*.md                   executable entry procedures
│   └── prototypes/dev-dashboard/
│       ├── index.html                 human cockpit with static fallback
│       └── state.generated.js         generated, secret-free repo snapshot
└── scripts/
    ├── workflow/                      runtime mirror generation/verification
    └── dashboard/                     state generation/verification/server
```

## Edit ownership

| Surface | Edit directly? | Owner |
|---|---|---|
| `.agents/registry.json`, `.agents/roles/` | Yes | project-system/operator |
| `.codex/agents/`, `.claude/agents/` | No; regenerate | workflow generator |
| `.codex/skills/` | Yes | Skill author |
| `.claude/skills/` | No; regenerate | workflow generator |
| `dashboard-state.json` | Yes, for human judgment only | operator |
| `state.generated.js` | No; regenerate | dashboard generator |
| `features/<slug>/dev_log.md` | Yes, by current workflow writer | feature workflow |
| dashboard HTML | Yes, presentation only | project-system |

## Commands

```bash
npm run workflow:generate   # regenerate runtime mirrors
npm run workflow:verify     # detect role/Skill drift
npm run dashboard           # refresh Git/docs/registry facts
npm run dashboard:verify    # reject stale or incomplete snapshots
npm run dashboard:serve     # local-only human cockpit
npm run project:verify      # full workflow + dashboard gate
```
