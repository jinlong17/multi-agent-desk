# Test strategy: 面向用户的操作手册与看板入口

## Acceptance matrix

| Requirement | Level | Command/scenario | Expected evidence |
|---|---|---|---|
| Pre-release truthfulness | Manual/contract | inspect guide status legend and command sections | current developer tools and planned product commands are unmistakably separated |
| Journey coverage | Manual | map guide headings to brief acceptance criteria | install/readiness through revoke/troubleshooting are covered |
| Discovery | Static | inspect README, implementation-plan table, dashboard docs section | all three identify `docs/USER_GUIDE.md` |
| Required-doc fact | Contract | run dashboard generator and inspect state | guide appears with `exists: true` and non-zero bytes |
| Missing-doc rejection | Negative contract | run verifier against an isolated generated-state fixture with guide absent/empty | verifier fails specifically for required docs |
| Link health | Static | local Markdown-link scanner | all repository-relative links resolve |
| Repository contracts | Integration | `npm run project:verify` | workflow and dashboard verification pass |

## Unit and property tests

No runtime units change. Static assertions cover the generator/verifier
contract and required guide marker.

## Contract and fixture tests

Use an isolated temporary copy or in-memory mutation of generated state for
the missing-guide negative case. Do not rename or delete the real guide from
the shared worktree during verification.

## Integration and E2E

Generate the dashboard, verify it, and optionally serve it on loopback for a
manual visual check. Confirm the user-guide card is readable in the static
fallback and the feature log is present in generated facts.

## Security/adversarial tests

Search the guide for secret-like examples and forbidden claims: real-looking
tokens, Cookies, recovery codes, `--password` values, remote-erasure promises,
or claims that unpaired Passkey clients can control terminals.

## Cross-platform matrix

Documentation validation is platform-neutral. Content must distinguish the
planned stable/experimental platform matrix without claiming current runtime
evidence. Windows-specific product behavior remains gated by its Spike logs.

## Failure injection and recovery

Mutate a copied generated state to set the guide `exists` false or `bytes` to
zero and confirm the verifier rejects it. Restore/generate the canonical state
after the test.

## Manual acceptance

Read the guide as a first-time user and answer:

1. Can I use the product today?
2. What can I run today?
3. What will the v0.1 happy path be?
4. Which steps are blocked by unfinished phases?
5. What must I do when revoking a credential or losing a browser device key?

All five answers must be findable without reading the implementation plan.
