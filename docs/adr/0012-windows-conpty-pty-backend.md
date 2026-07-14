# ADR 0012: Windows ConPTY PTY backend

- Status: Accepted
- Date: 2026-07-14
- Owner: `provider`
- Impacted modules: `core`, `desktop`
- Security gate: none; no credentials or trust boundary were in scope

## Context

The Windows CLI/Daemon must host interactive Codex and Claude sessions while
preserving the same terminal-stream contract used by Unix PTYs. The selected
mechanism has to carry UTF-8 text and virtual-terminal sequences, propagate
resizes to the attached application, support host-owned scrollback replay, and
terminate without deadlocking on synchronous pipes.

Phase 0.5 evidence exercised the native Windows pseudoconsole APIs on an x64
GitHub-hosted Windows build. A 15-second alternate-screen fixture produced
1,405 interactive frames and 512 history markers, observed three exact size
changes, replayed identically from whole and 17-byte chunks, exited with code
zero, and reached output EOF with a 1 ms teardown.

The first run also exposed an integration hazard: a Go child retained the
runner PowerShell standard handles even though it was associated with the
pseudoconsole. Explicitly opening `CONIN$` and `CONOUT$` after association made
the stream and console-size APIs operate through ConPTY.

## Decision

Use native ConPTY as the Windows PTY backend.

The Windows runtime adapter must:

- create synchronous pipe pairs and a pseudoconsole with
  `CreatePseudoConsole`;
- attach the child with `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE` and
  `EXTENDED_STARTUPINFO_PRESENT`;
- keep input and output work on independent execution paths and continuously
  drain output through child exit and `ClosePseudoConsole`;
- treat the output as UTF-8 plus VT sequences and leave rendering and
  scrollback storage to the shared terminal/session layer;
- forward character-grid changes with `ResizePseudoConsole`;
- validate the actual provider child's standard handles during Phase 3; when a
  runtime retains unrelated inherited handles, use a first-party Windows
  launcher/shim that opens the attached `CONIN$`/`CONOUT$` devices before
  launching the provider;
- bound graceful stop and process-tree termination, retaining an explicit kill
  path when the provider does not exit.

The stable Windows product target remains Windows 11 x64. The committed Spike
supports the mechanism on Windows build `10.0.26100.32995`/amd64 but does not
replace Windows 11 workstation acceptance. A real Codex/Claude full-screen
binary plus IME, mouse, accessibility, sleep/resume, and process-tree teardown
must pass before stable Windows interactive support is released.

## Consequences

### Positive

- Phase 1 can freeze one cross-platform PTY abstraction with Unix PTY and
  native ConPTY implementations.
- The remote terminal path can relay the same UTF-8/VT stream to xterm.js.
- Resize and host-owned replay have reproducible Windows x64 transport
  evidence instead of a plan-only claim.

### Obligations and residual limits

- A separate output reader is correctness-critical; a full pipe during close
  can deadlock older pseudoconsole implementations.
- ConPTY is not a renderer or scrollback database. Reflow, history bounds, and
  replay semantics stay in the shared session layer.
- Windows Server CI is not Windows 11 UI evidence. Stable release remains
  gated by the workstation/provider acceptance lane.
- If that lane fails, non-interactive Windows CLI/Daemon behavior may remain
  supported while the affected interactive feature is narrowed and Windows
  Desktop remains Experimental.

## Evidence

- `docs/spikes/windows/conpty_probe_windows.go`
- `docs/spikes/windows/conpty-result.json`
- `docs/spikes/windows/2026-07-14-windows-conpty-spike.md`
- GitHub Actions passing run `29377266540`
- GitHub Actions retained failure run `29377146037`
