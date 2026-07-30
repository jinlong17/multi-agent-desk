# Bug verification: Phase 4a P2 receipt socket lifecycle

## Verdict

`READY_TO_SHIP` for the scoped `phase4a-p2-receipt-socket-lifecycle` bugfix.

The feature-level Phase 4a P2 status remains `BLOCKED`; this verdict does not
replace the separately required exact-head native Windows, clean-SHA Chrome and
Safari, or physical Safari Touch ID/platform-Passkey receipts.

## Scope and ownership

- Owner: `control-plane` (high confidence).
- Secondary impact: `project-system` acceptance tooling and state evidence.
- Audited implementation files:
  `scripts/acceptance/p2-browser-receipt.mjs` and
  `scripts/acceptance/p2-browser-receipt.test.mjs`.
- Base commit: `d4973ee`; branch: `codex/control-plane/phase4a-core`.
- No implementation, plan, dashboard judgment, commit, push, or PR write was
  performed by this verifier.

## Independent reproduction and regression evidence

Runtime: Node `v24.11.1`.

1. `node --test --test-name-pattern='Node 24 delayed keep-alive response' scripts/acceptance/p2-browser-receipt.test.mjs`
   passed 3/3. A real delayed, chunked, keep-alive HTTPS response reached user
   `end` with `response.socket === null`; the TLSSocket captured at response
   callback entry still proved authorized direct loopback, non-empty peer raw
   certificate bytes, and the exact expected leaf SHA-256. The same real server
   rejected an unknown CA before the response callback.
2. `node --test --test-name-pattern='version TLS socket validation' scripts/acceptance/p2-browser-receipt.test.mjs`
   passed 1/1. Missing socket, unauthorized socket, non-loopback peer, missing
   peer raw bytes, and leaf mismatch each rejected.
3. `node --test scripts/acceptance/p2-browser-receipt.test.mjs`
   passed 21/21.
4. `npm run acceptance:p2-browser:test`
   passed 21/21.
5. `node --check scripts/acceptance/p2-browser-receipt.mjs` and
   `node --check scripts/acceptance/p2-browser-receipt.test.mjs` passed.

## Fail-closed boundary audit

The repair captures and validates `response.socket` synchronously at the HTTPS
response callback entrance. It does not disable keep-alive and does not defer
trust decisions until `end`.

The following existing or strengthened boundaries remain mandatory:

- TLS verification retains `rejectUnauthorized: true`, hostname/SNI binding,
  the manifest CA, and minimum TLS 1.2.
- The captured socket must have `authorized === true` and a direct IPv4 or IPv6
  loopback peer.
- `getPeerCertificate()` must exist and return non-empty raw certificate bytes;
  their SHA-256 must exactly equal the manifest leaf fingerprint.
- End-phase acceptance still requires exact status 200, exact
  `application/json`, parseable object/data envelopes, a non-empty version, and
  exact equality between the returned commit and frozen implementation SHA.

No TLS, loopback, leaf-certificate, JSON, or commit check was removed or made
permissive.

## Adjacent and repository gates

- `npm run project:verify` passed: workflow verified with 10 agents, 3 skills,
  17 docs, 20 edges, and 15 statuses; dashboard generation/verification passed.
- `npm run ci:verify` passed: 7 Actions checks, 15 pinned actions, CODEOWNERS,
  positive/negative CI fixtures, the packaged 21-test receipt suite, 323 local
  Markdown links, 6 pnpm license groups, and 418 Cargo packages.
- `git diff --check` passed.
- Final dirty paths before verifier persistence were only the writer's two
  acceptance files and the target state authority. This report is the sole new
  verifier file.

## Findings

None.

## Blockers

None for this scoped bugfix. The feature-level P2 external receipt blockers are
unchanged and are not defects in this repair.
