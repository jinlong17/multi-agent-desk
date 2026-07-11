# Security policy

MultiAgentDesk is pre-release software. No released version is currently
supported for production or security-critical use.

## Reporting a vulnerability

Do not open a public issue containing vulnerability details, credentials,
tokens, private keys, terminal contents, or personal data. Contact the
repository owner through a private channel listed on the GitHub repository.
If GitHub private vulnerability reporting is enabled, use that facility.

Include the affected commit or version, platform, reproducible steps, impact,
and a minimal proof of concept with secrets removed. Do not test against
systems or accounts you do not own or have explicit permission to assess.

No response or remediation SLA is promised while the project is pre-release.
Remote repository reporting settings and support guarantees become effective
only when the operator explicitly configures and documents them.

## Security boundaries

The current security invariants are documented in [CLAUDE.md](CLAUDE.md) and
the reviewed [implementation plan](docs/IMPLEMENTATION_PLAN.md). In particular,
the Control Plane must not receive plaintext Provider credentials, credential
grants are explicit and device-scoped, and the project never claims it can
erase credentials already copied to a compromised device.
