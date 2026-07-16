# MultiAgentDesk

MultiAgentDesk is a terminal-first, self-hostable workspace for managing AI
coding-agent profiles, sessions, usage, devices, and explicitly authorized
credential grants across local machines and remote servers.

The product is pre-release and is not yet a supported end-user application.
Phase 0, the Phase 0.5 decision gates, the Phase 1 Device Kernel, and the
locally `SHIPPED` Phase 2 Codex vertical slice are integrated on remote
`main`. Phase 2's credentialed Linux exit, platform matrix, final verification,
Security Review, and seven-check protected PR are complete. Packaging, release,
and deployment are still gated. The reviewed
architecture baseline is [`docs/IMPLEMENTATION_PLAN.md`](docs/IMPLEMENTATION_PLAN.md),
and the resumable current state is recorded in the feature `dev_log.md` files.

MultiAgentDesk is local-first and self-hostable. It does not automate account
rotation, bypass quotas or rate limits, proxy Provider requests, scrape browser
cookies, or silently switch credentials during a Session.

## User documentation

See the [pre-release user operations guide](docs/USER_GUIDE.md) for the current
source-built Phase 1/2 developer preview plus the planned install-to-session,
device-pairing, credential-grant, revocation, offline, and troubleshooting
workflow. The guide distinguishes exact commands available in the current
source tree from v0.1 product capabilities that remain gated.

## Development system

```text
.agents/                         cross-runtime role authority and registry
.codex/agents/                   generated Codex role entrypoints
.codex/skills/                   project Skill authority
.claude/agents/                  generated Claude role entrypoints
.claude/skills/                  generated Skill mirrors
docs/workflow/project/           policy, modules, and manual dashboard state
docs/workflow/templates/         feature-state templates
docs/workflow/features/<slug>/   resumable per-feature state
docs/prototypes/dev-dashboard/   local human cockpit
scripts/workflow/                mirror generation and drift verification
scripts/dashboard/               state generation, verification, local server
```

## Quick start

Requires Node.js 24 and pnpm 10.23.0 as pinned by the repository. The workflow
and dashboard tools themselves do not add runtime application dependencies.

```bash
npm run workflow:generate
npm run project:verify
npm run dashboard:serve
```

Open `http://127.0.0.1:4178`. The dashboard combines human-maintained phase
judgment with generated Git, document, Agent, and Skill facts.

## Start work

1. Classify the request with `mad-module-classify`.
2. Normalize new work with `mad-feature-brief`.
3. Follow `docs/workflow/SOP_NEW_FEATURE.md`, `SOP_BUGFIX.md`, or `SOP_SPIKE.md`.
4. Update the feature `dev_log.md` on every transition.
5. Refresh the dashboard with `mad-dashboard-sync` or `npm run dashboard`.

See `AGENTS.md`, `CLAUDE.md`, and `docs/workflow/project/workflow.md` for the
full contract. Merge, push, Ship, release, and risk acceptance always require
explicit human authorization.

## Governance and security

- Contributions follow [`CONTRIBUTING.md`](CONTRIBUTING.md) and require a DCO
  sign-off; this project does not use a CLA.
- Report vulnerabilities using [`SECURITY.md`](SECURITY.md), not a public issue.
- The project is licensed under [Apache-2.0](LICENSE). Third-party attribution
  and research rules are recorded in
  [`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).
