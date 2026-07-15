# DCO anchor correction note

The Phase 1 implementation and ship-receipt PRs were merged with GitHub's
squash action before the merge body included the repository's required
`Signed-off-by` trailer. The resulting main commits remain historical context;
the signed anchor is now the clean continuation point, and the main push DCO
range is green without adding grandfathered exceptions. Future squash merges
must include the trailer in the merge body. This note changes no product code
or release state.
