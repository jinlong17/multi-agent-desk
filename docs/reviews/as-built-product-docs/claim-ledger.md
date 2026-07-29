# Claim ledger: As-built product documentation authority

This is the durable P1 evidence inventory for `as-built-product-docs`. It is
review evidence, not a runtime registry, support promise, dashboard authority,
or replacement for a feature `dev_log.md`, the compatibility matrix, or the
threat model. Rows are append-only: a correction adds a new row that names the
superseded `CLM-###`; rolling back prose must not remove this record.

## Snapshot binding

| Field | Value |
|---|---|
| Ledger revision | `1` |
| Snapshot as of | `2026-07-29 PDT` |
| Verified on | `2026-07-29 PDT` |
| Verification baseline | `397194bfaea6983c4c0a54289d8bb1844711cfd6` |
| Freshness rule | At day 31 after `Snapshot as of`, every `preview`, `supported`, or `experimental` row is rewritten as `unknown`, `evidence_state=stale`, `fallback_or_gate=snapshot refresh`. A positive Provider row also becomes `unknown` at evidence day 91, on scope mismatch, contradiction, or an unreachable receipt, with `fallback_or_gate=Provider Gate: provider evidence refresh`. |
| Canonical vocabulary | `planned`, `preview`, `supported`, `experimental`, `unsupported`, `unknown`; `stale` is evidence state, not a class. Every `preview` scope literally includes `source-built`. |
| Unresolved conflicts | `CON-001`, `CON-002`, and `CON-003` below; no other conflict is silently resolved by this inventory. |

All `evidence_ref` values pin the snapshot source at the full verification
baseline unless a more specific immutable receipt is named. A receipt is
current only when it is reachable from that baseline and its date, exact
tool/version/platform/capability, result, and fallback match the row.

## Claims

### CLM-001

- **source:** `README.md#multiagentdesk`
- **claim:** MultiAgentDesk is a local-first, self-hostable workspace, but is
  pre-release and not a supported end-user application; integrated phases and
  structural/feature completion do not establish packaging, release, or
  deployment readiness.
- **class / scope:** `preview` — `source-built` repository preview; no
  installer, release, deployment, or general platform-support promise.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `README.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; feature state and
  independent verdicts linked from the source; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** read the target feature `dev_log.md`; packaging,
  release, and deployment remain separate gates.
- **reviewers:** independent feature verification for any new current claim;
  security review before ship for changed substantive trust wording.
- **destination:** `docs/PRODUCT.md#current-status-snapshot`; `README.md#multiagentdesk`; `docs/USER_GUIDE.md#1-先判断你现在能做什么`.
- **conflict:** `CON-001`.

### CLM-002

- **source:** `README.md#multiagentdesk`; `docs/USER_GUIDE.md#2-v01-计划解决什么问题`
- **claim:** Automatic account rotation, quota/rate-limit bypass, Provider
  request proxying, browser-cookie scraping, and silent in-session credential
  switching are prohibited product behaviors.
- **class / scope:** `unsupported` — all Providers and platforms; policy and
  security boundary, not a feature-completeness claim.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/THREAT_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `README.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** explicit user selection and official Provider login.
- **reviewers:** independent security review before ship if wording changes.
- **destination:** `docs/PRODUCT.md#non-goals-and-policy-boundaries`; all
  derived user-facing policy summaries.
- **conflict:** none.

### CLM-003

- **source:** `README.md#user-documentation`; `README.md#quick-start`; `docs/USER_GUIDE.md#1-先判断你现在能做什么`
- **claim:** The Node/pnpm workflow and local dashboard are repository
  development tooling only; they do not start, validate, or release the
  MultiAgentDesk product.
- **class / scope:** `preview` — `source-built` development tooling on the
  repository's pinned Node 24/pnpm 10.23.0 environment.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `README.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/USER_GUIDE.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** product capability must be traced to its own feature
  evidence; dashboard facts and green structural checks are not support proof.
- **reviewers:** feature-verify for product claims.
- **destination:** `docs/PRODUCT.md#current-status-snapshot`; `README.md#user-documentation`.
- **conflict:** none.

### CLM-004

- **source:** `docs/ARCHITECTURE.md#architecture`; `docs/ROADMAP.md#roadmap`
- **claim:** The implementation plan and accepted ADRs are the reviewed target
  architecture/roadmap. The current architecture and roadmap documents are
  placeholders and make no as-built runtime claim.
- **class / scope:** `planned` — v0.1 target architecture and phase order.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/IMPLEMENTATION_PLAN.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/ARCHITECTURE.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/ROADMAP.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** feature logs and independent verdicts own current
  lifecycle state.
- **reviewers:** feature-verify for future as-built component claims.
- **destination:** `docs/PRODUCT.md#evidence-and-documentation-authority`; `docs/ARCHITECTURE.md#architecture`; `docs/ROADMAP.md#roadmap`.
- **conflict:** none.

### CLM-005

- **source:** `docs/PROVIDER_ADAPTER.md#provider-adapter`
- **claim:** ADR 0004 and ADR 0006 define Provider boundaries; this placeholder
  defines no public adapter protocol and does not itself establish compatibility.
- **class / scope:** `unknown` — public adapter protocol and general Provider
  compatibility.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/PROVIDER_ADAPTER.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `missing`.
- **provider_receipt:** not-applicable; no public protocol claim is published.
- **fallback_or_gate:** Provider Gate: exact compatibility row and upstream
  receipt required before a positive Provider statement.
- **reviewers:** provider-owned feature-verify or provider-spike decision.
- **destination:** `docs/PRODUCT.md#evidence-and-documentation-authority`; `docs/PROVIDER_ADAPTER.md#provider-adapter`.
- **conflict:** none.

### CLM-006

- **source:** `docs/DATA_MODEL.md#authority-and-storage-contract`; `docs/DATA_MODEL.md#device-and-local-clients`
- **claim:** The documented Phase 1 P1 Device schema has one production
  database writer, SQLite/WAL transactional storage, Unix private-root/file
  permissions, immutable migration checks, and no database-direct CLI/TUI API;
  Windows ACL enforcement and cryptographic provisioning remain later work.
- **class / scope:** `preview` — `source-built` Phase 1 P1 schema and Unix
  storage boundary; it is not an equivalent Windows security claim.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/DATA_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/workflow/features/phase1-device-kernel/design.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** later Phase 1/Windows acceptance; do not treat Go mode
  bits as Windows ACL protection.
- **reviewers:** independent security review before ship for changed storage or
  trust wording.
- **destination:** `docs/PRODUCT.md#trust-and-data-boundaries`; `docs/DATA_MODEL.md#authority-and-storage-contract`.
- **conflict:** none.

### CLM-007

- **source:** `docs/DATA_MODEL.md#workspace-runtimeprofile-and-fake-credentialinstance`; `docs/DATA_MODEL.md#session`; `docs/DATA_MODEL.md#attachment-and-controllerlease`; `docs/DATA_MODEL.md#structural-events-and-audit-metadata`; `docs/DATA_MODEL.md#transaction-and-recovery-evidence`
- **claim:** The P1 schema is Fake-Provider-only for profiles and credential
  instances; Session identity/lease/audit invariants are model contracts, while
  real credential materialization, runtime homes, and later recovery behavior
  are not established by that schema document.
- **class / scope:** `planned` — real Provider materialization and runtime
  behavior; `preview` only for the `source-built` P1 schema constraints.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/DATA_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** use exact Provider/feature receipts rather than a schema
  table for runtime behavior.
- **reviewers:** feature-verify; security review for credential wording.
- **destination:** `docs/PRODUCT.md#current-capability-matrix`; `docs/DATA_MODEL.md#workspace-runtimeprofile-and-fake-credentialinstance`.
- **conflict:** `CON-002`.

### CLM-008

- **source:** `docs/USER_GUIDE.md#3-平台和阶段可用性`; `docs/USER_GUIDE.md#4-安装前准备规划中`
- **claim:** The platform table is a target/phase map, not a general support
  statement. Installation packages, upgrade/uninstall guidance, and release
  distribution are planned and no download is currently authorized by the guide.
- **class / scope:** `planned` — v0.1 platform and release target.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/USER_GUIDE.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/IMPLEMENTATION_PLAN.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** source-built developer preview only; Phase 6/release
  evidence required for installation support.
- **reviewers:** feature-verify and ship/release gates.
- **destination:** `docs/PRODUCT.md#current-status-snapshot`; `docs/USER_GUIDE.md#3-平台和阶段可用性`.
- **conflict:** `CON-001`.

### CLM-009

- **source:** `docs/USER_GUIDE.md#5-初始化本地设备和-daemon开发者预览`
- **claim:** The guide provides source-built local initialization, daemon,
  status, and Vault command examples, while installer behavior and automatic
  unlock are not product support claims; password input must avoid argv, logs,
  and shell history.
- **class / scope:** `preview` — `source-built` local developer path with an
  explicit Device root; no package or host-compromise protection promise.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/USER_GUIDE.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/THREAT_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** manual source build and explicit Vault handling;
  packaging/release gate remains open.
- **reviewers:** independent security review before ship for changed Vault
  confidentiality wording.
- **destination:** `docs/PRODUCT.md#trust-and-data-boundaries`; `docs/USER_GUIDE.md#5-初始化本地设备和-daemon开发者预览`.
- **conflict:** none.

### CLM-010

- **source:** `docs/USER_GUIDE.md#6-登录-provider-和管理账号`; `docs/USER_GUIDE.md#7-创建-runtime-profile开发者预览`
- **claim:** The guide describes a bounded Codex source-built account/profile
  path and isolated homes, but Claude login/isolation, Provider usage, and
  profile materialization remain limited to their exact compatibility evidence.
- **class / scope:** `unknown` — broad Provider account/profile support; the
  exact Linux Codex selector is separately recorded in CLM-012.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/USER_GUIDE.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `contradictory`.
- **provider_receipt:** missing for a broad cross-Provider claim.
- **fallback_or_gate:** Provider Gate: use the exact compatibility row and
  official interactive login; open a provider-owned spike before broadening.
- **reviewers:** provider-owned feature-verify or provider-spike decision.
- **destination:** `docs/PRODUCT.md#current-capability-matrix`; `docs/USER_GUIDE.md#6-登录-provider-和管理账号`.
- **conflict:** `CON-002`.

### CLM-011

- **source:** `docs/PROVIDER_COMPATIBILITY.md#evidence-schema` (rows: Web Device
  Key, Windows ConPTY/Named Pipe, Windows Tauri sidecar)
- **claim:** The listed browser, Windows terminal/IPC, and Desktop-sidecar
  results are narrow compatibility evidence with the exact versions/platforms
  and stated fallback/acceptance gates; they are not broad Web, Windows, or
  Desktop product support.
- **class / scope:** `experimental` — exact rows dated 2026-07-14 only; Windows
  11 and product acceptance gates remain open.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-14`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** the matrix's per-row fallback; retained Windows 11,
  security, and product-feature acceptance gates.
- **reviewers:** relevant feature-verify and independent security review.
- **destination:** `docs/PRODUCT.md#current-capability-matrix`; `docs/PROVIDER_COMPATIBILITY.md#evidence-schema`.
- **conflict:** `CON-003`.

### CLM-012

- **source:** `docs/PROVIDER_COMPATIBILITY.md#evidence-schema` (Codex Phase 2
  vertical slice and explicit multi-account selector rows); `docs/USER_GUIDE.md#8-启动和控制-sessioncodex-开发者预览`
- **claim:** A Codex CLI `0.144.2` Linux x86_64/amd64 path has independent
  evidence for the bounded Phase 2 vertical slice and the explicit selector P2;
  selector runtime is rejected before materialization/spawn on macOS, Windows,
  other architectures, versions, or schemas as stated by the matrix.
- **class / scope:** `supported` — exact Linux `0.144.2` only; `source-built`
  use remains pre-release. macOS has schema/empty-home evidence, not selector
  identity acceptance; real Windows Codex is unsupported.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/reviews/phase2-codex-vertical-slice/2026-07-16-feature-verify-p3b.md@250bf57f2ec21beb5f02e03b58f030b9d67e5ff4`;
  `docs/reviews/codex-multi-account-selector/2026-07-20-feature-verify.md@b041240646c337aa67a2f3c078a498356218b44d`; `2026-07-20`; `current`.
- **provider_receipt:** `phase2-codex-vertical-slice feature-verify@250bf57f2ec21beb5f02e03b58f030b9d67e5ff4; codex-multi-account-selector feature-verify@b041240646c337aa67a2f3c078a498356218b44d; provider=Codex; tool=CLI 0.144.2; platform=Linux x86_64/amd64; capability=vertical slice/explicit selector; result and fallback=matrix rows`.
- **fallback_or_gate:** official interactive login on accepted Linux; typed
  unsupported/identity-pending result elsewhere; no raw-ID/default-account or
  auto-rotation fallback.
- **reviewers:** the named independent provider feature-verify receipts;
  security review before ship for changed credential/trust wording.
- **destination:** `docs/PRODUCT.md#current-capability-matrix`; `docs/USER_GUIDE.md#3-平台和阶段可用性`; `docs/USER_GUIDE.md#8-启动和控制-sessioncodex-开发者预览`.
- **conflict:** `CON-002`.

### CLM-013

- **source:** `docs/PROVIDER_COMPATIBILITY.md#evidence-schema` (Codex account
  APIs, distinct-account Homes, managed refresh, and device-auth rows)
- **claim:** Additional Codex results are version/platform-specific and split:
  exact account APIs and single-writer refresh have evidence; device-auth
  completion is experimental; stable alias launch and broader identity/platform
  conclusions remain open.
- **class / scope:** `unknown` — no single current immutable provider receipt
  covers this combined broad capability scope under the P1 receipt contract.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-20`; `missing`.
- **provider_receipt:** missing for this normalized broad claim; do not publish
  a positive combined statement.
- **fallback_or_gate:** Provider Gate: split future claims to exact matrix rows
  and bind a valid upstream receipt, or open a provider spike.
- **reviewers:** provider-owned feature-verify or provider-spike decision.
- **destination:** `docs/PRODUCT.md#current-capability-matrix`; `docs/PROVIDER_COMPATIBILITY.md#evidence-schema`.
- **conflict:** none.

### CLM-014

- **source:** `docs/PROVIDER_COMPATIBILITY.md#evidence-schema` (Claude profile
  auth health, subscription, Team CLI/PTY, and setup-token rows); `docs/USER_GUIDE.md#6-登录-provider-和管理账号`
- **claim:** Claude support is asymmetric and bounded: direct target-local
  official login is the fallback; a stable managed subscription surface,
  subscription quota dashboard, setup-token grant, and long session are not
  established, while the exact Team CLI/PTY evidence is negative.
- **class / scope:** `unsupported` — stable managed Claude subscription/Team
  CLI-PTY, quota dashboard, and setup-token grant in the stated matrix scopes.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/reviews/spike-claude-distinct-account-usage/2026-07-16-security-review.md@5d4c363f6295845ff47abec9bc68093ae1a83b88`;
  `docs/reviews/spike-claude-subscription-cli-pty-compatibility/2026-07-20-security-review.md@dc1aeb00fbfc24e9b0986094ef534b2e3b90305b`; `2026-07-20`; `current`.
- **provider_receipt:** not required for an unsupported claim; the cited
  security-reviewed negative evidence is retained.
- **fallback_or_gate:** direct official Claude Code outside the managed surface,
  or separately planned user-supplied API-key/supported-cloud path with explicit
  billing source.
- **reviewers:** named security reviews; provider spike/feature-verify before
  any future positive claim.
- **destination:** `docs/PRODUCT.md#current-capability-matrix`; `docs/USER_GUIDE.md#6-登录-provider-和管理账号`.
- **conflict:** none.

### CLM-015

- **source:** `docs/USER_GUIDE.md#9-启用远程访问规划中phase-4a4b`; `docs/USER_GUIDE.md#10-把凭据授权给指定设备规划中phase-5`; `docs/USER_GUIDE.md#11-撤销设备或凭据`; `docs/USER_GUIDE.md#12-离线和断线行为`
- **claim:** Control Plane, browser/Desktop remote control, enrollment,
  credential grants, revocation orchestration, and offline behavior are planned
  product flows. A future grant is explicit and target-scoped; revocation can
  stop future flows but cannot erase plaintext already copied to a compromised
  target.
- **class / scope:** `planned` — Phase 4a/4b/5 remote and grant product flows.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/USER_GUIDE.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/THREAT_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** local-only source-built workflow; Control Plane,
  security, and feature acceptance gates remain open.
- **reviewers:** independent security review before ship for substantive
  credential, key, E2EE, revocation, or residual-risk wording.
- **destination:** `docs/PRODUCT.md#trust-and-data-boundaries`; `docs/USER_GUIDE.md#9-启用远程访问规划中phase-4a4b`; `docs/USER_GUIDE.md#10-把凭据授权给指定设备规划中phase-5`.
- **conflict:** none.

### CLM-016

- **source:** `docs/USER_GUIDE.md#13-常见问题与安全恢复`; `docs/USER_GUIDE.md#14-发布前如何确认手册真的可执行`
- **claim:** Troubleshooting and release-readiness text must preserve the
  non-release boundary, official Provider recovery path, and the distinction
  between structural checks and current product acceptance.
- **class / scope:** `planned` — final operational/release guidance.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/USER_GUIDE.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** exact compatibility and security evidence; release
  remains a human-gated ship responsibility.
- **reviewers:** feature-verify and ship; security review for changed recovery
  or credential claims.
- **destination:** `docs/PRODUCT.md#evidence-and-documentation-authority`; `docs/USER_GUIDE.md#14-发布前如何确认手册真的可执行`.
- **conflict:** none.

### CLM-017

- **source:** `docs/THREAT_MODEL.md#scope-and-non-goals`; `docs/THREAT_MODEL.md#assets-and-security-objectives`; `docs/THREAT_MODEL.md#attacker-model`; `docs/THREAT_MODEL.md#trust-boundaries`
- **claim:** The threat model, not overview prose, is the authority for assets,
  attacker assumptions, trust boundaries, and residual exposure. It expressly
  excludes the Control Plane as a Provider-plaintext or key-trust authority.
- **class / scope:** `planned` — full production enforcement varies by feature;
  the trust-model boundary itself is authoritative.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/THREAT_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** link to the relevant threat/ADR/security evidence; do
  not condense a risk qualification into a general security promise.
- **reviewers:** independent security review before ship if wording changes.
- **destination:** `docs/PRODUCT.md#trust-and-data-boundaries`; `docs/PRODUCT.md#evidence-and-documentation-authority`.
- **conflict:** none.

### CLM-018

- **source:** `docs/THREAT_MODEL.md#non-negotiable-security-invariants`; `docs/THREAT_MODEL.md#failure-and-recovery-rules`; `docs/THREAT_MODEL.md#explicit-residual-risk`; `docs/THREAT_MODEL.md#assumptions-and-update-triggers`
- **claim:** Passkey authentication does not grant E2EE decryption authority;
  pinned Device keys are not replaceable by the Control Plane; credential
  grants are explicit/target-scoped and revocable only for future use; host
  compromise, Provider-readable plaintext at runtime, copied secrets, metadata
  exposure, and human error remain residual risks.
- **class / scope:** `planned` — security invariants and residual-risk
  boundaries; no claim that every production enforcement point is implemented.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/THREAT_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `current`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** fail closed/re-pair/re-login or local remediation as
  specified by the threat model; never promise remote erasure.
- **reviewers:** independent security review before ship.
- **destination:** `docs/PRODUCT.md#trust-and-data-boundaries`; every derived
  credential, Passkey, E2EE, grant, revocation, and recovery statement.
- **conflict:** none.

### CLM-019

- **source:** `docs/THREAT_MODEL.md#threats-required-mitigations-and-evidence`; `docs/THREAT_MODEL.md#resolved-spike-decisions-and-deferred-production-evidence`
- **claim:** Threat rows and resolved Spike decisions contain mixed verified
  design/mechanism evidence, exact narrow implementation evidence, and planned
  production enforcement. A resolved compatibility decision does not itself
  prove full platform/product acceptance.
- **class / scope:** `unknown` — aggregate production security posture; each
  underlying row retains its own evidence state and platform boundary.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/THREAT_MODEL.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`; `2026-07-29`; `contradictory`.
- **provider_receipt:** not-applicable.
- **fallback_or_gate:** retain the per-threat evidence state, exact platform
  scope, and owning feature/security gate.
- **reviewers:** independent security review before ship.
- **destination:** `docs/PRODUCT.md#trust-and-data-boundaries`; `docs/PRODUCT.md#current-capability-matrix`.
- **conflict:** `CON-003`.

### CLM-020

- **source:** `docs/PROVIDER_COMPATIBILITY.md#resolved-phase-05-decision-gates`; `docs/PROVIDER_COMPATIBILITY.md#phase-2-codex-schema-clarification`
- **claim:** `GATE_RESOLVED` records a design/compatibility decision only.
  Exact Canonical Codex schema evidence is not identity acceptance or general
  cross-platform session support; unknown versions/schemas fail closed.
- **class / scope:** `supported` — exact Codex schema rows and narrow Phase 2
  Linux behavior only; no broader Provider/platform conclusion.
- **authority / evidence_ref / evidence_date / evidence_state:**
  `docs/PROVIDER_COMPATIBILITY.md@397194bfaea6983c4c0a54289d8bb1844711cfd6`;
  `docs/reviews/phase2-codex-vertical-slice/2026-07-16-feature-verify-p3b.md@250bf57f2ec21beb5f02e03b58f030b9d67e5ff4`; `2026-07-16`; `current`.
- **provider_receipt:** `phase2-codex-vertical-slice feature-verify@250bf57f2ec21beb5f02e03b58f030b9d67e5ff4; provider=Codex; tool=CLI 0.144.2; platform=Linux live plus macOS schema smoke; capability=canonical schema/vertical slice; fallback=fail closed`.
- **fallback_or_gate:** probe exact schema and return typed failure; retain
  platform/identity acceptance gates.
- **reviewers:** named independent provider feature-verify receipt.
- **destination:** `docs/PRODUCT.md#current-capability-matrix`; `docs/PROVIDER_COMPATIBILITY.md#phase-2-codex-schema-clarification`.
- **conflict:** `CON-002`.

## Coverage map

This map records every substantive source heading in the approved reconciliation
universe. `CLM-*` means the heading's product-facing support/trust statements
are normalized above; `scope-only` means the heading has no additional claim
beyond the linked row's explicit limited authority.

| Source heading | Ledger coverage | Planned destination |
|---|---|---|
| `README.md#multiagentdesk` | CLM-001, CLM-002 | `docs/PRODUCT.md#current-status-snapshot`, `#non-goals-and-policy-boundaries` |
| `README.md#user-documentation` | CLM-003 | `docs/PRODUCT.md#evidence-and-documentation-authority` |
| `README.md#development-system` | CLM-003, scope-only | `docs/PRODUCT.md#evidence-and-documentation-authority` |
| `README.md#quick-start` | CLM-003 | `docs/PRODUCT.md#current-status-snapshot` |
| `README.md#start-work` | CLM-003, scope-only | `docs/PRODUCT.md#evidence-and-documentation-authority` |
| `README.md#governance-and-security` | CLM-002, scope-only | `docs/PRODUCT.md#non-goals-and-policy-boundaries` |
| `docs/USER_GUIDE.md#1-先判断你现在能做什么` | CLM-001, CLM-003 | `docs/PRODUCT.md#current-status-snapshot` |
| `docs/USER_GUIDE.md#2-v01-计划解决什么问题` | CLM-002, CLM-015 | `docs/PRODUCT.md#product-position`, `#non-goals-and-policy-boundaries` |
| `docs/USER_GUIDE.md#3-平台和阶段可用性` | CLM-008, CLM-012, CLM-014 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/USER_GUIDE.md#4-安装前准备规划中` | CLM-008 | `docs/PRODUCT.md#current-status-snapshot` |
| `docs/USER_GUIDE.md#5-初始化本地设备和-daemon开发者预览` | CLM-009 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/USER_GUIDE.md#6-登录-provider-和管理账号` | CLM-010, CLM-012, CLM-014 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/USER_GUIDE.md#7-创建-runtime-profile开发者预览` | CLM-007, CLM-010 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/USER_GUIDE.md#8-启动和控制-sessioncodex-开发者预览` | CLM-012 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/USER_GUIDE.md#9-启用远程访问规划中phase-4a4b` | CLM-015 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/USER_GUIDE.md#10-把凭据授权给指定设备规划中phase-5` | CLM-015, CLM-018 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/USER_GUIDE.md#11-撤销设备或凭据` | CLM-015, CLM-018 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/USER_GUIDE.md#12-离线和断线行为` | CLM-015 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/USER_GUIDE.md#13-常见问题与安全恢复` | CLM-016, CLM-018 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/USER_GUIDE.md#14-发布前如何确认手册真的可执行` | CLM-016 | `docs/PRODUCT.md#evidence-and-documentation-authority` |
| `docs/ARCHITECTURE.md#architecture` | CLM-004 | `docs/PRODUCT.md#evidence-and-documentation-authority` |
| `docs/DATA_MODEL.md#authority-and-storage-contract` | CLM-006 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/DATA_MODEL.md#device-and-local-clients` | CLM-006 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/DATA_MODEL.md#workspace-runtimeprofile-and-fake-credentialinstance` | CLM-007 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/DATA_MODEL.md#session` | CLM-007 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/DATA_MODEL.md#attachment-and-controllerlease` | CLM-007 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/DATA_MODEL.md#structural-events-and-audit-metadata` | CLM-007 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/DATA_MODEL.md#transaction-and-recovery-evidence` | CLM-007 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/PROVIDER_ADAPTER.md#provider-adapter` | CLM-005 | `docs/PRODUCT.md#evidence-and-documentation-authority` |
| `docs/PROVIDER_COMPATIBILITY.md#evidence-schema` | CLM-011 through CLM-014 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/PROVIDER_COMPATIBILITY.md#resolved-phase-05-decision-gates` | CLM-020 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/PROVIDER_COMPATIBILITY.md#phase-2-codex-schema-clarification` | CLM-020 | `docs/PRODUCT.md#current-capability-matrix` |
| `docs/THREAT_MODEL.md#scope-and-non-goals` | CLM-017, CLM-018 | `docs/PRODUCT.md#non-goals-and-policy-boundaries` |
| `docs/THREAT_MODEL.md#assets-and-security-objectives` | CLM-017 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/THREAT_MODEL.md#attacker-model` | CLM-017 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/THREAT_MODEL.md#trust-boundaries` | CLM-017 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/THREAT_MODEL.md#non-negotiable-security-invariants` | CLM-018 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/THREAT_MODEL.md#threats-required-mitigations-and-evidence` | CLM-019 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/THREAT_MODEL.md#failure-and-recovery-rules` | CLM-018 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/THREAT_MODEL.md#resolved-spike-decisions-and-deferred-production-evidence` | CLM-019 | `docs/PRODUCT.md#evidence-and-documentation-authority` |
| `docs/THREAT_MODEL.md#explicit-residual-risk` | CLM-018 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/THREAT_MODEL.md#assumptions-and-update-triggers` | CLM-018 | `docs/PRODUCT.md#trust-and-data-boundaries` |
| `docs/ROADMAP.md#roadmap` | CLM-004 | `docs/PRODUCT.md#evidence-and-documentation-authority` |

## Retained conflicts

| ID | Sources | Conflict | Safe P1 disposition | Owner to clear |
|---|---|---|---|---|
| `CON-001` | README phase wording; user-guide dated snapshot and platform table | Phase/branch/remote-main language can be read as a release or broad support statement, while both sources retain a pre-release/source-built boundary. | Preserve `preview` only; P2 must link live feature state and must not convert integration or structural checks into release support. | `project-system` documentation build with feature evidence. |
| `CON-002` | User-guide commands/profile/session wording; data-model P1 Fake-only scope; compatibility matrix | Broad developer-preview commands coexist with much narrower exact Codex Provider/platform/selector receipts and planned/Fake schema descriptions. | Broad cross-Provider/platform claims are `unknown`; only CLM-012/020 retain their exact receipt-bound Codex scope. | `provider` evidence producer, then P1 ledger supersession. |
| `CON-003` | Compatibility rows and threat-model evidence states | Narrow Spike/mechanism evidence and planned production enforcement are adjacent; a reader could mistake a resolved decision or CI platform result for product acceptance. | Preserve exact scope and use `experimental`, `planned`, or `unknown`; no general Web/Windows/Desktop/security support statement. | owning feature verification and, for trust wording, independent `security-review`. |
