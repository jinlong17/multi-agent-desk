# SOP: New feature

1. Run module classification and choose a canonical slug.
2. Use `mad-feature-brief`; create `docs/reviews/<slug>/<date>-feature-brief.md`.
3. Copy the four templates into `docs/workflow/features/<slug>/`.
4. Run `feature-plan`; leave status `NEEDS_REVIEW`.
5. Run an independent `feature-review`; revise until `APPROVED`.
6. Create `codex/<module>/<slug>` only when implementation is authorized.
7. Run `feature-build` for exactly one phase.
8. Run independent `feature-verify`; return failures to the writer.
9. Repeat build/verify per approved phase.
10. Run `ship` only after explicit human authorization.
11. Refresh the dashboard after every state transition.
