# Test plan: Phase 0 architecture ADR batch

1. Assert ADR numbers 0001–0008 each map to exactly one Markdown file and all
   eight are linked from `docs/adr/README.md` with title and Accepted status.
2. Assert every ADR has the required sections and a Phase 0.5 marker wherever
   Provider, auth, key storage, Windows transport/sidecar, or E2EE evidence is
   discussed.
3. Search for unsupported certainty such as tested/supported Provider versions
   or completed Phase 0.5 conclusions; inspect every match.
4. Assert `RESEARCH_LOG.md` and `PROVIDER_COMPATIBILITY.md` contain their
   required schemas and truthful empty/pending states.
5. Run a local Markdown link scan across the index, ADR batch, and scaffolds.
   The durable CI link checker remains `unknown` until phase0-ci-governance.
6. Run `npm run project:verify` and `git diff --check`.

No runtime, migration, Provider invocation, network research, or Windows
hardware acceptance is part of this documentation phase.
