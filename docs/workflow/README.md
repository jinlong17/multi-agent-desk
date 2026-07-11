# MultiAgentDesk development workflow

This directory is the resumable project-development system.
The complete ownership tree is in `FILE_STRUCTURE.md`.

```text
docs/workflow/
├── project/                 # cross-project policy and machine-readable state
│   ├── workflow.md
│   ├── dev-dashboard.md
│   ├── dashboard-state.json
│   └── module-registry.json
├── templates/               # copied when a feature/bug starts
├── features/<slug>/         # design/api/test/dev_log authority per work item
├── SOP_NEW_FEATURE.md
├── SOP_BUGFIX.md
└── SOP_SPIKE.md
```

The dashboard is a view over these files. It is not an independent state store.
