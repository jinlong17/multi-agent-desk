# P5 as-built: CLI/TUI and platform exit

P5 exposes the Phase 1 Device Kernel through thin authenticated clients. CLI
commands never open the SQLite database directly; they load the owner identity,
connect over the authenticated local endpoint, and call the same application
service used by the native E2E.

## Implemented command surface

- `daemon install|uninstall` renders the user-level launch specification for
  the current platform. It does not install, stop, or mutate a host service;
  `daemon serve` remains the explicit foreground entrypoint.
- `vault status|unlock|lock` uses the Vault gate and idempotency boundary.
- `run fake`, `sessions list|show|observe|attach|detach|stop|kill|resume`,
  `control acquire|heartbeat|release`, and `terminal input|resize` map to the
  authenticated service methods with bounded flags and lease revisions.
- `client list` returns redacted client metadata. Client provisioning,
  rotation, and revocation remain explicit offline-only operations in Phase 1;
  the thin CLI refuses those commands rather than printing or transporting a
  private key.
- `tui` is a minimal metadata view over `sessions.list`; it is intentionally
  not a terminal renderer or PTY/ConPTY implementation.

Every JSON response uses `schema_version: 1`, a deterministic request ID,
`ok`, and either `result` or `error`. Human output contains metadata only. No
service command mutates the host in automated tests.

## Evidence and limits

`cmd/multidesk` tests cover service-spec JSON stability, non-mutating rendering,
and safe refusal of offline client provisioning. The existing native two-client
IPC scenario remains the Phase 1 exit path for start, observe, attach/detach,
controller lease, input, resize, stop/kill, resume, and shutdown on all three
protected platforms.

P5 does not claim a real Provider, PTY/ConPTY, full interactive TUI, Windows 11
multi-user/service acceptance, signed packaging, release, or deployment. The
independent Phase 1 Security Gate remains required before ship.
