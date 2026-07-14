# Test plan: Phase 0 threat model

## Acceptance matrix

| Requirement | Level | Command/scenario | Expected evidence |
|---|---|---|---|
| Required sections | document | heading assertion | every contract heading present |
| Five invariants | document/security | exact concept search plus manual comparison to `CLAUDE.md` | all five preserved without weaker wording |
| Assets, attackers, boundaries | document/security | matrix/schema assertion | non-empty inventories and stable threat IDs |
| Evidence fidelity | security | search `verified`, `planned`, `pending`, `deferred`; inspect every verified claim | no unsupported verified/pass claim |
| Spike linkage | security | enumerate seven Phase 0.5 slugs and compare file state | exact links; three Windows Spikes remain DRAFT |
| Residual risk | security | search compromise/revocation/erasure/materialization/metadata/XSS | explicit non-erasure and plaintext-at-use risks |
| Links | document | local Markdown link scan | no missing local targets; durable CI checker stays unknown until CI feature |
| Project integrity | repository | `npm run project:verify && git diff --check` | pass |
| Independent security gate | workflow | `security-review` after READY_TO_SHIP | ACCEPTED before ship |

No runtime test, cryptographic vector, Provider invocation, or local Windows
acceptance is performed in this documentation phase. Such checks remain
pending/deferred and cannot be converted to pass.
