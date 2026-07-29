# Architecture

This is a concise derived architecture view. The dated [as-built product
snapshot](PRODUCT.md) and its [claim ledger](reviews/as-built-product-docs/claim-ledger.md)
control product wording; the [implementation plan](IMPLEMENTATION_PLAN.md) and
[ADRs](adr/README.md) control the reviewed target. A feature `dev_log.md`,
compatibility row, or security evidence remains more specific authority than
this page.

## As-built boundary

| Surface | Current class | Bounded interpretation |
|---|---|---|
| Device Kernel | `preview` | `source-built` local schema/storage and developer-path foundations exist; this is not a release, broad runtime, or equivalent Windows ACL claim. |
| Provider adapters | `unknown` | Provider behavior is asymmetric and must be read from the exact [compatibility matrix](PROVIDER_COMPATIBILITY.md) and receipt. The only positive Codex scope is the product snapshot's exact Linux CLI `0.144.2` evidence. |
| Control Plane | `planned` | Deployment, pairing, metadata synchronization, encrypted relay, and remote command flows are not current product capabilities. |
| Web and Desktop | `planned` | Remote terminal, approval, Credential Grant, and Desktop product surfaces remain gated. Narrow browser/Windows mechanism evidence is not product support. |
| Security posture | `unknown` | The [threat model](THREAT_MODEL.md) mixes exact verified mechanisms, planned enforcement, and retained residual risk; this page does not collapse them into a general guarantee. |

## Boundaries that do not change

- Provider credentials remain device-local; the Control Plane is not a Provider
  plaintext store or a key trust anchor.
- A Passkey authenticates a user but does not authorize E2EE Device-key
  decryption. Local key pinning cannot be silently replaced by the server.
- Credential Grants are planned explicit, target-device-scoped flows; revocation
  cannot remotely erase plaintext already copied to a compromised target.
- Automatic account rotation, quota/rate-limit bypass, Provider proxying,
  browser-cookie scraping, and silent mid-session credential switching are not
  product behaviors.

For a dated capability matrix and freshness downgrade, read [PRODUCT.md](PRODUCT.md).
For the exact physical layout, target components, and irreversible decisions,
read the [implementation plan](IMPLEMENTATION_PLAN.md) and [ADR index](adr/README.md).
