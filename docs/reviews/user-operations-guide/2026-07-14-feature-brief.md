# Feature Brief: 面向用户的操作手册与看板入口

- Slug: `user-operations-guide`
- Date: 2026-07-14
- Owner module: `project-system`
- Impacted modules: `core`, `provider`, `control-plane`, `web`, `desktop`, `security`
- Requested by: operator request on 2026-07-14

## Motivation and outcome

仓库已有详细实施计划、开发者 README、工作流 SOP 和本地开发看板，
但没有一份按最终用户任务组织的操作文档。现有材料可以回答产品准备
如何实现，却不能让用户快速判断“现在能不能用”以及在 v0.1 可用后如何
完成安装、初始化、Provider 登录、Profile 配置、会话启动、远程接管、
凭据授权和撤销。

预期结果是一份中文、任务导向的 `docs/USER_GUIDE.md`，并在 README、
实施计划文档体系和本地开发看板中提供可发现入口。手册必须把当前可执行
的开发工具与尚未实现的 v0.1 产品命令明确分开。

## Scope

1. 新增预发布用户操作手册，包含当前成熟度、可用性矩阵、完整用户路径、
   常见故障、安全注意事项和发布前验收映射。
2. 使用实施计划中已经冻结的 `multidesk` 命令与页面名称作为“规划接口”，
   每处均标明当前不可执行，直到对应 Phase 验证通过。
3. 从根 README 增加用户文档入口，并纠正文档中的运行时版本陈述，使其与
   当前 `package.json` 一致。
4. 在 `docs/IMPLEMENTATION_PLAN.md` 的文档体系中登记用户手册。
5. 在开发看板“文档与来源”区域展示用户手册，并把它加入机器校验的必需
   文档列表。

## Non-goals

- 不实现 `multidesk`、Web、Desktop、Daemon 或 Control Plane。
- 不声明 Phase 1–6 的能力已经可用或已经通过跨平台验证。
- 不编造安装包、下载地址、默认端口、服务名称、真实输出或 Provider
  兼容版本。
- 不改变项目优先级、阶段完成度、风险接受、Ship、发布或远端状态。
- 不撰写开发者部署指南、API 参考或 Provider Adapter 开发指南。

## User journeys

1. 新用户先看到“产品尚不可用”，并知道当前只能运行开发看板。
2. 未来 v0.1 用户按平台安装 `multidesk`，初始化并启动本地 Daemon。
3. 用户登录 Codex/Claude，创建 Runtime Profile，查看健康度与用量来源。
4. 用户确认系统推荐后启动 Session，并从第二个本地客户端 attach/control。
5. 用户部署 Control Plane，配对 Linux/Web/Desktop 设备，远程查看或控制会话。
6. 用户显式授权一个 CredentialInstance 给指定设备，并理解撤销不能远程
   擦除已经复制的秘密。
7. 遇到 Vault locked、Provider 缺失、凭据过期、设备换钥或 Control Plane
   离线时，用户能找到安全的恢复路径。

## Data and trust boundaries

- 文档不包含真实 Token、Cookie、认证文件、用户名、设备指纹或本地绝对
  凭据路径。
- Passkey 只表示 Control Plane 登录；Web/Desktop 仍需 Device Enrollment
  才能解密远程内容。
- Provider 凭据由 Device Vault 持有；Control Plane 只处理元数据和密文。
- 撤销 MultiAgentDesk 授权不等于远程擦除目标主机上的已复制秘密，彻底
  吊销仍需使用 Provider 官方安全入口。

## Provider/external assumptions

- Codex app-server、Claude CLI/PTY、认证隔离、用量和跨设备刷新行为仍受
  Phase 0.5 Spike 与兼容矩阵约束。
- 手册只描述实施计划已经定义的意图；未有 Spike 证据的行为必须标记为
  “待验证”，不能作为现状陈述。

## Dependencies and gates

- Authority: `docs/IMPLEMENTATION_PLAN.md` v0.2 and live feature logs.
- Provider Gate: none for this documentation slice; Provider claims remain
  visibly gated by their own Phase 0.5 work units.
- Security Gate: none; this change documents existing trust boundaries and
  does not modify a credential, key, crypto, or remote-control protocol.
- Dashboard verification is required because generator and static cockpit
  surfaces change.

## Acceptance criteria

- [ ] `docs/USER_GUIDE.md` starts with an unmistakable pre-release warning and
      separates currently runnable developer tooling from planned product use.
- [ ] The guide covers install/readiness, init/daemon, Provider login,
      profiles, sessions, attach/control, device pairing, remote UI,
      credential grant/revoke, offline behavior, troubleshooting, and safety.
- [ ] Every product command is labeled as planned until its owning Phase is
      verified; no fabricated output or distribution URL appears.
- [ ] README and the implementation-plan document table link to the guide.
- [ ] Dashboard static content lists the guide, generated state treats it as a
      required document, and the verifier fails if the file is absent.
- [ ] Local Markdown links resolve and `npm run project:verify` passes.
- [ ] The feature state is visible in generated dashboard facts without
      changing operator-owned priority or release judgment.

## Risks and open questions

- Planned CLI flags may change during implementation; mitigation is a visible
  status legend and a per-Phase revalidation checklist.
- A user may mistake a design-flow example for a usable build; mitigation is
  to repeat the pre-release warning before command sections.
- Final installer/download instructions remain unresolved until Phase 6 and
  must stay explicitly pending.

## Evidence

- `README.md` states the application is not yet usable and product code has
  not started.
- `docs/IMPLEMENTATION_PLAN.md` sections 5, 12, 16, 19, 22, and 25 define the
  planned user flows, phase gates, failure behavior, and release acceptance.
- `scripts/dashboard/generate-state.mjs` currently omits a user guide from
  `required_docs`.
- `docs/prototypes/dev-dashboard/index.html` currently has no user-guide card.

## Handoff

Next role: `feature-plan`.
