# Windows ConPTY full-screen provider TUI Spike

## Verdict

**SUPPORTED with explicit scope limits.** The native Windows pseudoconsole
transport can host the project’s full-screen provider-TUI protocol shape on an
x64 GitHub-hosted Windows runner. The probe demonstrated interactive input,
three observable resizes, alternate-screen VT output, captured-history replay,
and bounded clean teardown.

This result supports choosing ConPTY for the Windows PTY backend. It is not a
Windows 11 desktop acceptance result and does not prove a real Codex or Claude
binary, IME, mouse, accessibility, or GPU-rendering behavior. Those remain
release acceptance items, not blockers to the Phase 1 IPC/runtime design.

## Frozen hypothesis

ConPTY can host a full-screen provider TUI reliably: render, resize, scrollback
replay, and clean teardown behave correctly for a sustained interactive
session.

## Reproducible method

The probe is implemented in
`docs/spikes/windows/conpty_probe_windows.go` and runs in
`.github/workflows/spike-windows-conpty.yml`.

1. Create synchronous input/output pipe pairs.
2. Create a native pseudoconsole at `120x40` with `CreatePseudoConsole`.
3. attach the probe child through `PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE` and
   `EXTENDED_STARTUPINFO_PRESENT`.
4. In the Go child, explicitly reopen `CONIN$` and `CONOUT$` after attachment.
5. Verify sizes `80x24`, `132x43`, and `100x30` from inside the child after
   `ResizePseudoConsole` calls.
6. Emit 512 history markers and a 15-second full-screen stress stream.
7. Compare semantic replay from the whole stream and 17-byte chunks.
8. Request a clean exit while a dedicated reader continues draining output,
   then close the pseudoconsole and require reader EOF.

Exact CI command:

```powershell
go build -trimpath -o docs/spikes/windows/.bin/conpty-probe.exe docs/spikes/windows/conpty_probe_windows.go
docs/spikes/windows/.bin/conpty-probe.exe -duration 15s -result docs/spikes/windows/conpty-result.json
```

## Evidence

Passing run: GitHub Actions `29377266540`, job `conpty-windows-x64`, completed
2026-07-14. The sanitized artifact is committed as
`docs/spikes/windows/conpty-result.json`.

| Assertion | Result |
|---|---:|
| OS/architecture | Windows build `10.0.26100.32995`, `amd64` |
| Go | `go1.26.5` |
| Initial size | `120x40` |
| Resize observations | `80x24`, `132x43`, `100x30` all exact |
| Interactive stress | `15,004 ms`, `1,405` frames |
| Captured history | `512` markers |
| Captured transport | `400,654` bytes |
| Transcript SHA-256 | `dbb6cf0826c1e93db737abd867fca7acb2315938b39d0525da52409055142709` |
| Whole vs 17-byte replay | equivalent |
| Alternate-screen enter/exit | both observed |
| Child exit | code `0` |
| Reader EOF | observed |
| Teardown | `1 ms` |

The initial run `29377146037` failed because the Go child retained the host
PowerShell standard handles. Reopening `CONIN$` and `CONOUT$` after the
pseudoconsole attachment resolved the failure. This is a required integration
rule for the eventual Windows runtime adapter.

## API contract checked

The probe follows Microsoft’s pseudoconsole contract:

- [CreatePseudoConsole](https://learn.microsoft.com/en-us/windows/console/createpseudoconsole)
  uses synchronous input/output streams and produces UTF-8 text interleaved
  with VT sequences.
- [Creating a Pseudoconsole Session](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session)
  requires `STARTUPINFOEX`, the pseudoconsole process attribute, and a separate
  drain path to avoid pipe deadlocks.
- [ResizePseudoConsole](https://learn.microsoft.com/en-us/windows/console/resizepseudoconsole)
  updates the dimensions visible to attached console applications.

## Scope limits and fallback

- GitHub `windows-latest` supplied Windows Server (`win25-vs2026`), not a
  physical Windows 11 workstation.
- The host application must own VT rendering and scrollback storage; ConPTY is
  the transport, not the terminal emulator.
- Before stable Windows CLI/Daemon release, run the same harness plus a real
  provider TUI on Windows 11 x64 and cover IME, mouse, accessibility, sleep,
  and repeated process-tree termination.
- If the Windows 11 acceptance lane fails, keep Windows Desktop Experimental
  and temporarily narrow the affected interactive feature while preserving
  non-interactive CLI/Daemon workflows.

## Decision recommendation

Adopt ConPTY as the Windows PTY backend and carry the explicit `CONIN$` /
`CONOUT$` rebinding and dedicated-output-reader rules into Phase 1/3 design.
Do not label Windows 11 UI acceptance complete from this CI result alone.
