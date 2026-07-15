# Phase 0.5 evidence reconciliation

- Reconciled: 2026-07-14
- Owner: `project-system`
- Input authority: protected `main` at
  `1e027573f401ee8115ba0a5e321a0540052d7a9c`
- Decision scope: Phase 0.5 design/compatibility gates only
- Production/release readiness: not implied by this report

## Completion verdict

`PASS — 7/7 Phase 0.5 decision gates are resolved.`

Every planned Spike has a `GATE_RESOLVED` state authority, reproducible
evidence, an accepted ADR, a bounded compatibility result, and an explicit
fallback or later acceptance gate. Phase 0.5 can therefore close at the
decision level. The accepted mechanisms still require implementation and the
later-phase acceptance work listed below.

## Exact decision set

| Spike authority | Accepted decision | Reproducible evidence | Bounded result and fallback | Owning consumer | Reconciliation |
|---|---|---|---|---|---|
| `spike-browser-key-storage` | ADR 0010 | browser report plus Chrome/Edge/Firefox/Safari/WebKit JSON artifacts and independent security review | Chrome/Edge/Firefox native at tested versions; Safari/WebKit `software_wrapped`; failed/unknown probe becomes `metadata_only` | Phase 4b `web`, `security`, `control-plane` | `PASS` |
| `spike-e2ee-protocol-vectors` | ADR 0011 | protocol spec, shared vectors, Go and TypeScript implementations, result hash, independent cryptographic review | fixed v1 pairwise-root suite; unknown suite/state fails closed; no group-root or recovery claim | Phase 4b `security`, `control-plane`, `web`, `core` | `PASS` |
| `spike-windows-conpty` | ADR 0012 | Windows x64 runner probe, result JSON, retained failure and final pass | native ConPTY mechanism selected; stable interactive support still needs Windows 11 real-provider/IME/accessibility/lifecycle acceptance | Phase 1 PTY abstraction and Phase 3 provider acceptance | `PASS` |
| `spike-windows-named-pipe-ipc` | ADR 0013 | Windows x64 message-mode probe, result JSON, negative harnesses, independent security review | protected current-logon Named Pipe plus mutual protocol auth/capability/lease; no silent fallback; Windows 11 multi-user/service acceptance retained | Phase 1 `core`; Phase 6 Windows acceptance | `PASS` |
| `spike-codex-auth-refresh` | ADR 0014 | exact-version app-server matrix, file-store/refresh artifacts, four-sample 10812.845-second two-device short run, independent security review | one canonical writable app-server/auth home with revisioned CAS; interactive login fallback; no multi-writer, completed device-auth, or 48-hour claim | Phase 2 Codex slice; Phase 5 materialization | `PASS` |
| `spike-windows-desktop-sidecar` | ADR 0015 | signed-path Windows x64 Tauri probe, result JSON, lifecycle/security review | discover-first crash-surviving authenticated sidecar; separately installed service fallback; Desktop remains Experimental pending Windows 11 signed lifecycle acceptance | Phase 5 preview; Phase 6 packaging/acceptance | `PASS` |
| `spike-claude-config-keychain` | ADR 0016 | macOS config/Keychain profile matrix, macOS/Linux auth-health JSON, PTY control artifact, independent security review | target-profile official interactive login for one-account scope; setup-token CredentialGrant disabled; unknown auth schema fails closed | Phase 3 Claude slice; Phase 5 grant boundary | `PASS` |

The exact input set is deliberate: adding an unrelated resolved feature cannot
compensate for a missing row.

## Protected integration evidence

| PR | Decision | Merge commit | Required check result |
|---:|---|---|---|
| #4 | Codex auth/refresh | `93e0ac02b6b5e252090304d406fb48031b960722` | seven required checks successful |
| #5 | Claude profile/login | `1e027573f401ee8115ba0a5e321a0540052d7a9c` | seven required checks successful |
| #6 | Browser key storage | `8656ef25ca70ac59aa6b60270cb8c8557fd62ed5` | seven required checks plus browser platform jobs successful |
| #7 | E2EE vectors | `5a30002479e844bf3cc9c774113cdbf2261fd8bd` | seven required checks plus three-platform vectors successful |
| #8 | Windows ConPTY | `67eb1547c5b6f0304a60d518cfc669ebfe2074bc` | seven required checks plus ConPTY job successful |
| #9 | Windows Named Pipe | `ccf18fcb0575bcdd0e6efab787a6d0f2e9e08592` | seven required checks plus Named Pipe/ConPTY jobs successful |
| #10 | Windows Sidecar | `56fe33c62bf73ec05cae5e11fac21b26305f97c1` | seven required checks plus Sidecar job successful |

Authenticated protection readback retains strict required checks
`project-verify`, `build-ubuntu`, `build-macos`, `build-windows`,
`license-gate`, `dco`, and `link-check`; admin enforcement, conversation
resolution, linear history, and force-push/deletion prohibitions remain on.
The operator-approved single-account policy retains zero required approvals
and does not require CODEOWNER review.

## Platform coverage conclusion

| Platform | Phase 0.5 conclusion | What is proven | What is deliberately not proven |
|---|---|---|---|
| macOS | decision inputs ready | browser paths, Codex exact-version APIs/file refresh, Claude Config Dir/Keychain/auth health, all protected builds | production daemon/provider slice, credential grant, signed/notarized release |
| Linux | decision inputs ready | Codex/Claude auth-health boundaries, E2EE vectors, protected build | production service/IPC/provider sessions and deployment acceptance |
| Windows | decision inputs ready; stable release not ready | Edge key path; ConPTY, Named Pipe, Sidecar mechanisms on Windows x64 runners; protected build | Windows 11 real-device provider TUI, IME/accessibility, multi-user/service, logoff/sleep/reboot, security software, signed install/upgrade/rollback/uninstall |

Windows is now covered as a first-class implementation target in Phase 1 and a
first-class acceptance lane in Phase 3/6. It remains correctly labeled
Experimental for Desktop and is not misrepresented as release-ready.

## Residual gate routing

| Residual gate | Owning phase | Required behavior |
|---|---|---|
| Cross-platform Device Kernel, Unix socket/Named Pipe, Fake PTY/session lifecycle | Phase 1 | implement and pass macOS/Linux/Windows Fake Session exit criteria |
| Codex exact-version production adapter and canonical single writer | Phase 2 | fail closed on schema/capability drift; interactive login fallback |
| Claude real Linux PTY and Windows real-provider ConPTY acceptance | Phase 3 | official target-profile login; stable setup-token disabled; quota limitation visible |
| Browser runtime probe and pairwise E2EE production implementation | Phase 4b | native/wrapped/metadata-only mode and ADR 0011 protocol enforced |
| Credential materialization and Desktop | Phase 5 | no Claude setup-token grant; Codex revisioned CAS; Windows preview only |
| Windows 11, signing, packaging, lifecycle, multi-user/service, security tools | Phase 6 | Experimental lane may ship only with explicit limitations; failures do not become pass |

## Negative assertions retained

- The canceled Codex observation is recorded as a four-sample short run, not a
  48-hour soak; one-writer CAS is the selected production boundary.
- Headless Codex device-auth completion was not verified.
- Claude setup-token issuance, injection, long-session behavior, per-token
  revocation, and distinct-account isolation were not verified.
- GitHub-hosted Windows Server evidence is not Windows 11 workstation or signed
  release acceptance.
- Accepted ADRs select designs; none claims its production implementation is
  complete.

## Phase 0.5 exit mapping

| Exit requirement | Evidence | Verdict |
|---|---|---|
| each blocking hypothesis has a supported/unsupported conclusion | exact seven-row decision set and compatibility matrix | `PASS` |
| each conclusion has reproducible evidence | Spike reports, JSON/protocol artifacts, retained run IDs | `PASS` |
| each conclusion has a fallback or fail-closed boundary | ADR 0010-0016 and compatibility rows | `PASS` |
| each claim is version/platform bounded | compatibility matrix and ADR consequences | `PASS` |
| E2EE vectors pass Go and TypeScript | shared vector artifact, result hash, three-platform vector jobs | `PASS` |
| required project/platform governance remains green | merged PR check rollups and branch-protection readback | `PASS` |

P1 is complete and ready for independent verification. P2 may update the
project phase and dashboard only after that verification succeeds.
