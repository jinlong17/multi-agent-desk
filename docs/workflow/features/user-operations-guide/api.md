# Contracts: 面向用户的操作手册与看板入口

## Public interfaces

- `docs/USER_GUIDE.md`: canonical end-user operations guide.
- `README.md`: concise link and accurate maturity/toolchain statement.
- Dashboard `required_docs[]`: machine-readable existence and byte-size fact.
- Dashboard static “文档与来源” card: discoverable guide path and maturity
  description.

No runtime REST, WebSocket, CLI, database, or Provider interface changes.

## Requests, events, and responses

The guide may show planned `multidesk` commands already frozen in
`docs/IMPLEMENTATION_PLAN.md`. Each command block must be preceded by a
planned/not-currently-executable label until owning feature evidence proves
otherwise. Examples must use placeholders such as `<session-id>` and
`<device-id>` and must not include realistic secrets.

The dashboard generator emits:

```text
required_docs[] += {
  path: "docs/USER_GUIDE.md",
  label: "User operations guide",
  exists: boolean,
  bytes: number
}
```

## Error semantics

- Missing or empty guide: `dashboard:verify` fails with a required-document
  assertion.
- Static dashboard entry removed: `dashboard:verify` fails with a specific
  user-guide marker assertion.
- A product flow whose owning phase is not verified remains labeled planned or
  unavailable; documentation must not silently upgrade it to supported.

## Authentication and authorization

No authentication behavior changes. The guide preserves these rules:

- Passkey login does not authorize E2EE decryption.
- Sensitive remote actions require an enrolled, approved client device.
- Credential grants are explicit and target-device scoped.
- Only the current ControllerLease holder can send terminal input or respond
  to approvals.

## Idempotency, ordering, and replay

Not applicable to documentation generation. Running `npm run dashboard`
replaces the generated snapshot deterministically apart from timestamp and
current Git facts.

## Versioning and compatibility

The guide is a pre-v0.1 contract. Each phase verification that changes a user
command or flow must update it in the same slice. Planned syntax is not a
compatibility promise until released.

## Data retention and deletion

The guide and dashboard contain repository facts only. No secrets, local
credential paths, environment contents, terminal text, or Provider session
content may enter generated state.
