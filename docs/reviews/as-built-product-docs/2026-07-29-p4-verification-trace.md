# P4 verification preparation trace: As-built product documentation authority

## Purpose and boundary

This is a build-produced trace packet for the next independent
`feature-verify` run. It maps existing ledger evidence to the canonical and
derived documentation surfaces, records receipt and trust-boundary inputs, and
captures structural-check results. It is **not** a feature-verification
verdict, Provider-risk acceptance, Security review, release, ship, merge, or
push record.

The packet is bound to the repository state at `db6fc80`, the verified P1/P2/P3
receipts, and claim-ledger revision 2. The ledger's immutable reconciliation
baseline remains `397194bfaea6983c4c0a54289d8bb1844711cfd6`.

## Ledger-to-surface trace

| Ledger claim | Canonical destination | Derived surface trace | Verification focus |
|---|---|---|---|
| CLM-001 | `PRODUCT.md#product-position`, `#current-status-snapshot` | `README.md` opening; `USER_GUIDE.md` preamble | pre-release `source-built` only; no release/deployment uplift |
| CLM-002 | `PRODUCT.md#non-goals-and-policy-boundaries` | `README.md` opening; `USER_GUIDE.md#2-v01-计划解决什么问题`; `ARCHITECTURE.md#boundaries-that-do-not-change` | no rotation, bypass, proxy, cookie scraping, or silent switching |
| CLM-003 | `PRODUCT.md#current-status-snapshot` | `README.md#user-documentation`, `#quick-start`; `USER_GUIDE.md#1-先判断你现在能做什么` | repository tooling/dashboard is not product acceptance |
| CLM-004 | `PRODUCT.md#evidence-and-documentation-authority` | `ARCHITECTURE.md` authority links; unchanged scoped roadmap | implementation plan/ADRs are target, not as-built proof |
| CLM-005 | `PRODUCT.md#evidence-and-documentation-authority` | `PROVIDER_ADAPTER.md` preamble and public-protocol row | no public adapter protocol claim |
| CLM-006 | `PRODUCT.md#current-capability-matrix`, `#trust-and-data-boundaries` | `USER_GUIDE.md#3-平台和阶段可用性`; `ARCHITECTURE.md#as-built-boundary`; unchanged data-model authority | source-built Unix/schema boundary is not broad Windows support |
| CLM-007 | `PRODUCT.md#current-capability-matrix` | `USER_GUIDE.md#7-创建-runtime-profile开发者预览`; unchanged data model | Fake/schema constraints are not real Provider runtime proof |
| CLM-008 | `PRODUCT.md#current-status-snapshot` | `USER_GUIDE.md#3-平台和阶段可用性`, `#4-安装前准备规划中` | packages/release remain planned |
| CLM-009 | `PRODUCT.md#trust-and-data-boundaries` | `USER_GUIDE.md#5-初始化本地设备和-daemon开发者预览` | explicit source-built/Vault handling only |
| CLM-010 | `PRODUCT.md#current-capability-matrix` | `USER_GUIDE.md#6-登录-provider-和管理账号`; `PROVIDER_ADAPTER.md#current-bounded-positions` | broad Provider account/profile statement remains unknown |
| CLM-011 | `PRODUCT.md#current-capability-matrix` | `USER_GUIDE.md#3-平台和阶段可用性`; `ARCHITECTURE.md#as-built-boundary` | browser/Windows mechanism evidence is experimental, not product support |
| CLM-012 | `PRODUCT.md#current-capability-matrix` | `USER_GUIDE.md#3-平台和阶段可用性`, `#8-启动和控制-sessioncodex-开发者预览`; `PROVIDER_ADAPTER.md#current-bounded-positions` | exact receipt-bound Codex Linux `0.144.2`; macOS identity pending and Windows unsupported |
| CLM-013 | `PRODUCT.md#current-capability-matrix` | `PROVIDER_ADAPTER.md#current-bounded-positions` | broad combined Codex behavior stays unknown |
| CLM-014 | `PRODUCT.md#current-capability-matrix` | `USER_GUIDE.md#3-平台和阶段可用性`; `PROVIDER_ADAPTER.md#current-bounded-positions` | managed Claude scope stays unsupported with stated fallback |
| CLM-015 | `PRODUCT.md#current-capability-matrix`, `#trust-and-data-boundaries` | `USER_GUIDE.md#9-启用远程访问规划中phase-4a4b` through `#12-离线和断线行为` | remote/grant/revocation flows remain planned |
| CLM-016 | `PRODUCT.md#evidence-and-documentation-authority` | `USER_GUIDE.md#13-常见问题与安全恢复`, `#14-发布前如何确认手册真的可执行` | troubleshooting/release guidance does not imply shipment |
| CLM-017 | `PRODUCT.md#trust-and-data-boundaries`, `#evidence-and-documentation-authority` | `ARCHITECTURE.md#as-built-boundary`, `#boundaries-that-do-not-change` | threat model remains primary trust authority |
| CLM-018 | `PRODUCT.md#trust-and-data-boundaries` | `USER_GUIDE.md#9-启用远程访问规划中phase-4a4b` through `#13-常见问题与安全恢复`; `ARCHITECTURE.md#boundaries-that-do-not-change` | Passkey/E2EE, pinning, future-use-only revocation, and residual risk retained |
| CLM-019 | `PRODUCT.md#current-capability-matrix`, `#trust-and-data-boundaries` | `ARCHITECTURE.md#as-built-boundary` | aggregate production security posture remains unknown |
| CLM-020 | `PRODUCT.md#current-capability-matrix` | `PROVIDER_ADAPTER.md#current-bounded-positions`; `USER_GUIDE.md#3-平台和阶段可用性` | schema evidence is exact/fail-closed, not identity or cross-platform support |
| CLM-021 | `PRODUCT.md#current-capability-matrix`, `#trust-and-data-boundaries` | `USER_GUIDE.md#91-部署-control-plane` | Control Plane/bootstrap is planned; browser is not initial E2EE root |
| CLM-022 | `PRODUCT.md#trust-and-data-boundaries` | `USER_GUIDE.md#92-配对设备` | pairing is planned; server key directory is not a trust anchor |
| CLM-023 | `PRODUCT.md#current-capability-matrix`, `#trust-and-data-boundaries` | `USER_GUIDE.md#93-使用-web-或-desktop` | Web/Desktop is planned; unpaired browser is metadata-only |

The source headings, classes, full-revision evidence references, destinations,
conflicts, and stale-data rules remain in the [claim ledger](claim-ledger.md).
This packet adds no ledger row and changes no claim class.

## Positive Provider receipt trace

Only the following rows are positive Provider claims. The next verifier must
confirm each receipt remains reachable from the ledger baseline and matches the
row's Provider, tool, version, platform, capability, result, evidence date, and
fallback before retaining its class.

| Ledger claim | Immutable receipt(s) | Exact retained scope | Fallback/gate that must remain visible |
|---|---|---|---|
| CLM-012 | `docs/reviews/phase2-codex-vertical-slice/2026-07-16-feature-verify-p3b.md@250bf57f2ec21beb5f02e03b58f030b9d67e5ff4`; `docs/reviews/codex-multi-account-selector/2026-07-20-feature-verify.md@b041240646c337aa67a2f3c078a498356218b44d` | Codex CLI `0.144.2`; Linux `x86_64`/`amd64`; pre-release `source-built` vertical slice and explicit selector only | official interactive login on accepted Linux; typed failure otherwise; no raw-ID/default-account/auto-rotation fallback |
| CLM-020 | `docs/reviews/phase2-codex-vertical-slice/2026-07-16-feature-verify-p3b.md@250bf57f2ec21beb5f02e03b58f030b9d67e5ff4` | exact Codex `0.144.2` canonical schema and narrow Phase 2 Linux behavior; macOS schema/empty-home smoke is distinct from identity acceptance | exact-schema probe and fail closed; retained platform/identity gate |

All other Provider treatment is non-positive: CLM-013 is `unknown`; CLM-014 is
`unsupported`; CLM-011 is exact-scope non-product `experimental` mechanism
evidence. The Provider Gate remains open and this packet cannot resolve it.

## Trust-boundary trace

| Boundary | Ledger authority | Surface locations to inspect | Required retained qualification |
|---|---|---|---|
| Passkey and E2EE Device key | CLM-018, CLM-021, CLM-023 | `PRODUCT.md#trust-and-data-boundaries`; `USER_GUIDE.md#91-部署-control-plane`, `#93-使用-web-或-desktop`; `ARCHITECTURE.md#boundaries-that-do-not-change` | Passkey authenticates user access only; it is not E2EE decryption authority or an initial browser trust root. |
| Pinned Device identity and pairing | CLM-017, CLM-018, CLM-022 | `PRODUCT.md#trust-and-data-boundaries`; `USER_GUIDE.md#92-配对设备`; `ARCHITECTURE.md#boundaries-that-do-not-change` | server directory is an index, local key changes require re-pairing, and pairing remains planned. |
| Credential Grant and revocation | CLM-015, CLM-018, CLM-023 | `PRODUCT.md#trust-and-data-boundaries`; `USER_GUIDE.md#10-把凭据授权给指定设备规划中phase-5`, `#11-撤销设备或凭据`; `ARCHITECTURE.md#boundaries-that-do-not-change` | explicit target scope; future-use-only revocation; no remote erasure of copied plaintext. |
| Provider/authentication exposure | CLM-017, CLM-018, CLM-019 | `PRODUCT.md#trust-and-data-boundaries`; `ARCHITECTURE.md#as-built-boundary`; `PROVIDER_ADAPTER.md#consumer-rules` | Provider-readable runtime state and host/Provider/target compromise remain residual risk. |
| Remote Web/Desktop behavior | CLM-015, CLM-021, CLM-022, CLM-023 | `PRODUCT.md#current-capability-matrix`; `USER_GUIDE.md#9-启用远程访问规划中phase-4a4b` | planned only; unpaired browser metadata-only; no remote control/terminal/grant support claim. |

The Security Gate is still open. The next `feature-verify` may evaluate this
trace for readiness but may not accept security risk; only the later independent
`security-review` may do so after the workflow reaches `READY_TO_SHIP`.

## Freshness and structural inputs

- Snapshot: 2026-07-29 PDT, ledger revision 2. At snapshot day 31, every
  `preview`, `supported`, or `experimental` matrix/ledger statement must
  downshift to `unknown` with `evidence_state=stale` and `snapshot refresh`.
- Provider receipts: at day 91, scope mismatch, contradiction, or unreachable
  revision, each positive Provider row must downshift to `unknown` with
  `Provider Gate: provider evidence refresh`.
- The final verifier must treat structural commands as document/workflow checks
  only; they do not prove Provider, security, deployment, platform, or release
  readiness.

| Input | Build record |
|---|---|
| local links | `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run ci:links` passed: 314 Markdown files. Structural result only. |
| workflow/dashboard structure | `PATH=/Users/jinlong/.nvm/versions/node/v24.11.1/bin:$PATH /opt/homebrew/bin/pnpm run project:verify` passed: workflow `agents=10, skills=3, docs=17, edges=20, statuses=15`; dashboard static verification passed. Structural result only. |
| positive-receipt reachability | Both positive receipt commits are ancestors of `397194bfaea6983c4c0a54289d8bb1844711cfd6`; `git cat-file -e` found both pinned verification-report paths. This records input availability, not Provider-risk acceptance. |
| diff hygiene | `git diff --check` passed. |

## Next verifier inputs

1. Inspect every row in the ledger-to-surface trace for a missing destination,
   source link, class change, or contradictory derived wording.
2. Re-run positive Provider receipt reachability and exact-scope checks; block
   if either positive row is stale, unreachable, mismatched, or broadened.
3. Compare the trust-boundary trace with the threat model; block if a
   qualification or residual risk is lost. Do not issue a security verdict.
4. Consume the recorded structural command results and exact staged-file list.
