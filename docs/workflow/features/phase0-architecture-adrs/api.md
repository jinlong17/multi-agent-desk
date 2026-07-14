# Contract: Phase 0 architecture ADR batch

This feature provides documentation contracts, not runtime APIs.

## ADR contract

- Filenames: `docs/adr/0001-*.md` through `0008-*.md`.
- Index: each number links to exactly one file with title and `Accepted` status.
- Every ADR preserves `CLAUDE.md` security invariants and identifies its owner
  and impacted modules.
- Any dependency on Phase 0.5 evidence appears under an explicit
  `Spike-gated details` heading with the relevant Spike slug and pending state.
- No ADR may claim verified Provider versions, credential layout, Windows
  ConPTY/Named Pipe/Tauri behavior, browser key storage, or E2EE vectors.

## Research ledger contract

Each future `RESEARCH_LOG.md` entry records date, source URL, license, scope
read, conclusion, reused material, and reviewer. AGPL/unclear-license research
records architecture/documentation conclusions only and prohibits copying
source, tests, constants, or identifiable implementation detail.

## Provider compatibility contract

Each future compatibility row records Provider/tool, tested version, platform,
capability, evidence artifact, result, fallback, and date. The initial file
marks all Phase 0.5 conclusions pending and directs writers to the owning Spike
dev_log; pending is never rendered as supported.
