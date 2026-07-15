# P5 CLI correction plan after Security Gate REVISE

## Owner and scope

- Owner module: `core`; secondary impact: `security`, `project-system`.
- Correct only the two findings in
  `docs/reviews/phase1-device-kernel/2026-07-14-security-review.md`.
- Do not add production Vault crypto, real Provider credentials, host service
  mutation, or a new transport.

## Corrections

1. Derive the CLI idempotency key and bounded request ID from the canonical
   method, JSON body, and optional lease revision. Distinct operations must not
   collide, while retrying the exact same operation must replay the stored
   response.
2. Replace argv-based Vault unlock with `--secret-stdin`. Read a bounded,
   newline-trimmed value from stdin and never accept a secret flag. The daemon
   still receives the same bounded fake Vault request and retains no secret.

## Acceptance and rollback

- Unit/black-box tests prove different session/input bodies produce different
  idempotency keys and exact retries retain the same key.
- CLI tests reject `vault unlock --secret`, accept bounded stdin input, and do
  not print the input.
- Full Go/race/vet, three-target command compilation, exact license,
  project/CI/scaffold, and protected macOS/Linux/Windows runs remain green.
- Rollback is a single revert of the correction commit; the previously
  verified P5 surface remains intact and no database migration is required.
