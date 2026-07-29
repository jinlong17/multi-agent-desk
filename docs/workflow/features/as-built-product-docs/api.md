# Contract: As-built product documentation authority

## Canonical path contract

The future canonical as-built product document is exactly:

```text
docs/PRODUCT.md
```

`PRODUCT.md` at repository root is out of scope and must not be created. The
canonical document does not replace the reviewed target baseline, a feature
`dev_log.md`, an ADR, the compatibility matrix, or a security review. It
consumes those authorities and links to them.

## Claim ledger contract

The durable claim ledger is exactly
`docs/reviews/as-built-product-docs/claim-ledger.md`. Before a substantive
statement is published or changed, implementation must record it there. Its
header contains `Ledger revision`, `Snapshot as of`, `Verified on`, full
`Verification baseline` Git revision, and an explicit `Unresolved conflicts`
list. The initial ledger covers every source heading in the reconciliation
universe defined in `design.md`; later product-facing support/trust additions
must add a row in the same change.

Each row has at least these fields:

| Field | Contract |
|---|---|
| `id` | stable local identifier for review and reconciliation |
| `source` | source document plus stable heading/anchor containing the statement |
| `claim` | concise normalized statement; no secret or raw Provider payload |
| `class` | exactly one of `planned`, `preview`, `supported`, `experimental`, `unsupported`, or `unknown` |
| `scope` | Provider/product area, exact version/platform where applicable, user boundary, and literal `source-built` when class is `preview` |
| `authority` | authoritative source path(s), including a feature log/review or compatibility artifact where required |
| `evidence_ref` | repo-relative authority/receipt path pinned as `path@full-commit`; `none` only for a genuinely unknown/planned row |
| `evidence_date` | evidence observation or verdict date; `unknown` only for a genuinely unknown/planned row |
| `evidence_state` | `current`, `stale`, `missing`, or `contradictory`; this is not a capability class |
| `provider_receipt` | current provider receipt tuple or `not-applicable`; required for positive substantive Provider claims |
| `fallback_or_gate` | safe alternative, owning gate, or explicit absence of one |
| `reviewers` | provider receipt producer and/or independent security-review requirement when the claim type requires it |
| `destination` | canonical and derived document plus heading/anchor that must use the resolved wording |
| `conflict` | `none` or an ID in the header's unresolved-conflict list |

IDs are `CLM-###`, never reused. The ledger is append-only after P1: a changed
claim creates a superseding row that references its predecessor, and a
resolved conflict remains recorded. Reverting product prose or a derived
surface must not delete the ledger/history. It is review evidence, not a
runtime API or dashboard authority.

## Freshness contract

At verification date `V`, a snapshot is current only if `V - Snapshot as of`
is at most 30 calendar days and all referenced evidence revisions are reachable
from the `Verification baseline`. A Provider receipt is current only if that
condition holds, the recorded exact Provider/tool/version/platform/capability
matches the row scope, and `V - evidence_date` is at most 90 calendar days.

At snapshot day 31, every currently represented `preview`, `supported`, or
`experimental` row must be written as `class=unknown`,
`evidence_state=stale`, `fallback_or_gate=snapshot refresh`; the corresponding
product matrix row has the same class and gate. At Provider evidence day 91,
scope mismatch, unreachable receipt, or contradiction, a positive Provider row
must be written as `class=unknown`, `evidence_state=stale` (or
`contradictory`), `fallback_or_gate=Provider Gate: provider evidence refresh`.
The documentation build must not retain a positive class under either failure.

## Required canonical-document sections

The future `docs/PRODUCT.md` must expose these stable, linkable sections:

```text
Product position
Current status snapshot
Current capability matrix
Non-goals and policy boundaries
Trust and data boundaries
Evidence and documentation authority
```

The status snapshot must include an `As of` date and state whether it describes
a source-built preview, a released product, or planned target behavior. The
capability matrix must identify each row's class, scope, evidence, and gate or
fallback. A reader must not have to infer support from a phase name.

## Provider-claim contract

A statement about a Provider, CLI, app-server, authentication, usage,
session/PTY behavior, account isolation, approval, resume, or platform support
is substantive when it says or implies that a user can rely on it. Such a
statement must:

1. bind to the exact Provider/tool version and platform when the evidence is
   version/platform dependent;
2. link to the compatibility row and its underlying feature/spike/review
   artifact;
3. preserve unsupported, fallback, and gate wording;
4. bind a current immutable provider evidence receipt before P2 publishes a
   positive claim; and
5. open a separate provider-owned spike instead of being published as
   `supported` or `experimental` when the evidence is missing, contradictory,
   stale, or outside the recorded scope.

The only valid receipt producers are a provider-owned feature's independent
`feature-verify` report at
`docs/reviews/<provider-feature-slug>/<date>-feature-verify*.md`, or a
provider-spike evidence artifact at `docs/spikes/<provider-area>/` together
with its `GATE_RESOLVED` feature log and compatibility-matrix row. The ledger's
`provider_receipt` pins the selected path(s) at full Git revisions and records
the exact fields that were validated. This creates no new documentation
reviewer or verdict role.

## Trust-claim contract

A statement about credentials, Vault material, keys, passkeys, enrollment,
E2EE, device pinning, revocation, remote control, audit/log confidentiality,
or residual risk is substantive when it says or implies a security guarantee.
Such a statement must:

1. link to the relevant threat-model, ADR, security-review, and implementation
   evidence as appropriate;
2. preserve that a Passkey authenticates user access but does not itself grant
   E2EE decryption authority;
3. preserve that CredentialGrant is explicit, target-scoped, encrypted, and
   revocable for future use but cannot remotely erase already copied plaintext;
4. preserve the no-automatic-rotation/no-quota-bypass policy; and
5. receive independent security review before ship when the wording changes.

## Producer and consumer contract

| Producer | May establish | Consumer rule |
|---|---|---|
| implementation plan / accepted ADR | target architecture and intended scope | never state as current support without implementation evidence |
| feature log + independent verdict | workflow state and phase acceptance | link to it; do not overwrite its status from product prose |
| Provider compatibility row + artifacts | exact compatibility scope | do not broaden version, platform, or capability |
| threat model + security review | trust requirements and accepted residual risk | retain qualifications and security-review requirement |
| `docs/PRODUCT.md` | dated product-facing synthesis | link downstream documents here for overview, not proof |
| README/user guide/specialist docs | task-specific derived presentation | reconcile from accepted claim inventory |

When a consumer and producer differ, the consumer must be corrected or
narrowed. It must not update the producer's lifecycle status, compatibility
result, or risk acceptance.

## Update and failure contract

1. Establish or correct primary evidence first.
2. Update the claim ledger and canonical product document in the same
   documentation build phase.
3. Reconcile every listed derived surface in that phase or record an explicit
   bounded blocker with its owner.
4. Run structural checks, validate required Provider receipts, and route the
   final Security Gate through its independent workflow review.
5. If a local link fails, a claim lacks evidence, a Provider scope differs, or
   a trust qualification is lost, the documentation feature remains blocked or
   returns for revision. It must not claim completion because prose renders.

P4 prepares verification evidence only. The independent `feature-verify` role
may advance the completed documentation feature to `READY_TO_SHIP` only when
the ledger shows no invalid positive Provider claim; it cannot resolve the
Provider Gate. With this feature's open Security Gate, only the workflow's
subsequent `security-review` transition may issue
`docs/reviews/as-built-product-docs/<date>-security-review.md` and accept or
return the trust wording. No documentation build phase self-accepts Provider or
security risk.
