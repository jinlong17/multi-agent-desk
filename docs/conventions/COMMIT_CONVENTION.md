# Commit convention

Use `type(scope): imperative summary`.

Allowed types: `feat`, `fix`, `docs`, `test`, `refactor`, `build`, `ci`, `chore`,
`security`. Scope should match the owning module or feature slug.

For non-trivial commits, include:

```text
Why: reason for the change
What: implementation summary
Scope: owning module and impacts
Risk: known risks and rollback
Docs: updated state/contract docs
Tests: exact commands and results
```

Stage exact files. Do not combine another agent's dirty work into the commit.
