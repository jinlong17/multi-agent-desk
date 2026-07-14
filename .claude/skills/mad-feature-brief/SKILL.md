---
name: mad-feature-brief
description: Convert a raw MultiAgentDesk feature request into a canonical, reviewable Feature Brief with module ownership, goals, non-goals, trust boundaries, provider assumptions, acceptance criteria, risks, and handoff. Use for new features, major changes, ambiguous product requests, or before invoking feature-plan.
---

# Feature brief

Read `docs/IMPLEMENTATION_PLAN.md`, the module registry, and
`docs/workflow/templates/feature-brief.md`. Invoke `mad-module-classify` logic
first. Choose a stable lowercase hyphenated slug and write:

`docs/reviews/<slug>/<YYYY-MM-DD>-feature-brief.md`.

The brief must contain motivation, measurable outcome, scope, non-goals,
primary module, impacted modules, user journeys, data/trust boundaries,
Provider assumptions, dependencies, acceptance criteria, risks, unresolved
questions, and evidence links. Mark claims that require a Spike; do not present
them as established facts.

Do not implement code, approve the feature, create branches, or make priority
decisions. End with a handoff to `feature-plan`.
