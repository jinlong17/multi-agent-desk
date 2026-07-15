# Windows Tauri sidecar lifecycle Spike

Status: **supported with security and Windows 11 acceptance gates**.

## Scope

This Spike tests the selected Desktop-owned continuity policy with a real Tauri
v2 `externalBin` package on GitHub-hosted Windows x64. It covers target-suffixed
sidecar resolution, NSIS inclusion, Rust-side launch, graceful owner shutdown,
Desktop crash survival, restart discovery, duplicate prevention, owner-only
shutdown, pre-existing daemon separation, descendant cleanup, and sanitized
evidence capture.

It does not claim that the fixture token-file control protocol is production
authorization or that an unsigned, uninstalled CI bundle is release-ready.

## Official contract checked

- [Tauri sidecars](https://v2.tauri.app/develop/sidecar/) documents
  `externalBin`, target-triple-suffixed source binaries, and `ShellExt::sidecar`
  resolution.
- [Tauri shell plugin](https://v2.tauri.app/plugin/shell/) documents the shell
  plugin and frontend capability model. The fixture uses the fixed Rust-side
  `ShellExt::sidecar("mad-sidecar")` path and exposes no generic shell command to
  frontend code.

Source inspection of `tauri-plugin-shell 2.3.5` showed that the plugin's app-exit
cleanup is tied to children in its command child store. A child returned by the
direct Rust-side `ShellExt::sidecar().spawn()` path is owned by the caller and
is not inserted into that frontend command store. The test therefore makes
crash continuity an explicit MultiAgentDesk lifecycle policy instead of
depending on undocumented automatic cleanup.

## Environment

| Component | Version |
|---|---|
| Windows runner | `10.0.26100.32995`, AMD64, image `win25-vs2026` `20260628.158.1` |
| Tauri | `2.11.5` |
| Tauri CLI | `2.11.4` |
| tauri-plugin-shell | `2.3.5` |
| Rust/Cargo | `1.91.1` |
| Go | `1.26.5` |

## Reproduction

GitHub Actions workflow `.github/workflows/spike-windows-sidecar.yml`:

1. builds the Go sidecar as
   `mad-sidecar-x86_64-pc-windows-msvc.exe`;
2. builds the pinned Rust/Tauri host and an NSIS bundle with `externalBin`;
3. runs `run-sidecar-probe.ps1` against the packaged release executables;
4. uploads only `tauri-sidecar-result.json`.

The first run `29383265674` proved the Tauri/NSIS build but was cancelled when
the PowerShell output pipeline waited on console handles intentionally retained
by the crash-surviving sidecar. The harness was corrected to wait on the Tauri
host process handle rather than pipeline EOF. This did not change a lifecycle
assertion. Final run
[`29383875862`](https://github.com/jinlong17/multi-agent-desk/actions/runs/29383875862)
passed in 8m12s at commit `ebbc21990e56463c05ad0ae0ca012f506b918775`.

## Results

| Scenario | Result |
|---|---|
| Tauri target-suffixed sidecar resolution | launched PID matched daemon ready-state PID |
| NSIS external binary | archive contained `mad-sidecar.exe` |
| Graceful Desktop-owned stop | daemon and grandchild exited within five seconds |
| Hard Desktop abort | daemon and grandchild remained alive across the two-second crash window |
| Desktop restart | reused the same daemon PID and did not spawn a duplicate |
| Wrong-owner stop | denied; owned daemon remained alive |
| Pre-existing system-style daemon | observed but never claimed, duplicated, or stopped by Desktop |
| Correct-owner final stop | daemon and grandchild exited; final orphan count `0` |

The SHA-256 digests and executable/installer sizes are preserved in
[tauri-sidecar-result.json](tauri-sidecar-result.json).

## Candidate decision and fallback

The candidate policy is:

1. discover and authenticate an existing system-installed Daemon first;
2. otherwise start only the fixed, signed, packaged sidecar from Rust;
3. maintain a single-instance Daemon lock plus authenticated Named Pipe owner
   identity;
4. allow the owned daemon to survive Desktop crash/exit and reconnect to it;
5. stop only a sidecar whose ownership/instance identity is authenticated;
6. keep generic frontend shell execution disabled.

Production must replace the fixture token file with ADR 0013 authenticated
Named Pipe IPC, bind ownership to a stable instance ID and Desktop Device ID,
verify packaged binary signature/hash/provenance before launch, and define
updater/service authority. If Windows 11 packaging/lifecycle acceptance fails,
the fallback is a separately installed signed Daemon service; Windows Desktop
remains Experimental and must not silently spawn a second daemon.

## Limitations

- GitHub CI uses Windows Server, not a physical Windows 11 workstation.
- The NSIS archive was inspected but not installed, signed, upgraded, or rolled
  back.
- Power loss, OS logoff, Fast User Switching, sleep/resume, updater replacement,
  service installation, endpoint security software, and real Codex/Claude
  continuity remain Windows 11 acceptance items.
- The Spike fixture owns a simple daemon/grandchild tree. Production Job Object,
  service, updater, and Provider process-tree policy remains implementation work.
