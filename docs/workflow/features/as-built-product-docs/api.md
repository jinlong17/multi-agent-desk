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

## Claim record contract

Before a substantive statement is published or changed, implementation must
record it in a reviewable claim inventory with at least these fields:

| Field | Contract |
|---|---|
| `id` | stable local identifier for review and reconciliation |
| `surface` | source document and heading containing the statement |
| `claim` | concise normalized statement; no secret or raw Provider payload |
| `class` | `target`, `preview`, `supported`, `experimental`, `unsupported`, or `unknown` |
| `scope` | Provider/product area, exact version/platform where applicable, and user boundary |
| `authority` | authoritative source path(s), including a feature log/review or compatibility artifact where required |
| `evidence_date` | evidence observation or verdict date; `unknown` where no evidence exists |
| `fallback_or_gate` | safe alternative, owning gate, or explicit absence of one |
| `reviewers` | provider owner and/or security reviewer when the claim type requires it |
| `destination` | canonical and derived documents that must use the resolved wording |

The inventory may be a Markdown table unless feature review determines a
machine-readable format is necessary. It is review evidence, not a runtime API
or dashboard authority.

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
4. receive provider-owner fact review before acceptance; and
5. open a provider spike instead of being published as `supported` when the
   evidence is missing, contradictory, stale, or outside the recorded scope.

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
2. Update the claim inventory and canonical product document in the same
   documentation build phase.
3. Reconcile every listed derived surface in that phase or record an explicit
   bounded blocker with its owner.
4. Run structural checks and required human fact reviews.
5. If a local link fails, a claim lacks evidence, a Provider scope differs, or
   a trust qualification is lost, the documentation feature remains blocked or
   returns for revision. It must not claim completion because prose renders.
