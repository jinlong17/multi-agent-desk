# Contributing to MultiAgentDesk

MultiAgentDesk is in early development. Start by reading [AGENTS.md](AGENTS.md),
[CLAUDE.md](CLAUDE.md), and the
[workflow policy](docs/workflow/project/workflow.md). Work must be classified
into one owning module and follow the repository's document-driven lifecycle.

## Developer Certificate of Origin

Every commit must include a Developer Certificate of Origin sign-off:

```text
Signed-off-by: Your Name <your-email@example.com>
```

Use `git commit -s` to add it. By signing off, you certify the contribution
under the [Developer Certificate of Origin 1.1](https://developercertificate.org/).
MultiAgentDesk does not require a Contributor License Agreement.

## Before requesting review

Run the checks defined by the target feature. For current project-system work:

```bash
npm run project:verify
```

Record failures as failures and checks that were not run as `unknown`. Do not
commit secrets, Provider credentials, browser cookies, generated auth files,
or terminal/session contents. Contributions must preserve the security and
Provider boundaries in [CLAUDE.md](CLAUDE.md).

Third-party code reuse must identify its source and license, remain compatible
with Apache-2.0, and update
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). AGPL projects may be studied
only under the research constraints in the implementation plan; do not copy
their source, tests, constants, or identifiable implementation details.
