# SOP: Provider or security Spike

1. State one falsifiable hypothesis, environment matrix, time box, and success/failure criteria.
2. Remove secrets from commands and artifacts before recording evidence.
3. Run the smallest experiment; record exact versions, commands, output summary, and limitations.
4. Produce a deterministic fallback even when the hypothesis succeeds.
5. Run `security-review` when credentials, keys, auth, remote control, or trust boundaries are affected.
6. Update the compatibility matrix/ADR/plan gate; never ship Spike code as production by default.
