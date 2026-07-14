# Test plan: Phase 0 repository identity and root governance

## P1

1. Assert all five root documents exist and contain their required policy
   markers.
2. Scan Markdown links and run the repository's local link validation when it
   exists; until Phase 0 CI adds that tool, record unavailable checks as
   `unknown` rather than pass.
3. Run `npm run project:verify`.
4. Negative-test F1 by removing each required gated edge in an isolated
   temporary copy; each mutation must fail with a specific missing-edge error.
5. Exercise a stale focus fixture and confirm the F2 hint names the
   operator-directed refresh rule.

## P2

1. `basename "$PWD"` equals `multi-agent-desk` after reopening the checkout.
2. `git remote get-url origin` resolves to the prepared `multi-agent-desk`
   repository.
3. `git status --short` is clean and `git worktree list --porcelain` reports
   the canonical path.
4. Run `npm run project:verify` after the move.

Windows hardware is not involved in this feature. Any unavailable remote or
maintenance-window evidence remains `unknown` or `BLOCKED` as applicable.
