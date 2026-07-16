# Feature Brief: 多账号用量看板与显式调用

- Slug: `multi-account-usage-control`
- Date: `2026-07-15`
- Owner module: `provider`
- Impacted modules: `core, web, desktop, security, control-plane, project-system`
- Requested by: `operator — current highest priority`

## Motivation and outcome

一个用户可能同时拥有 3–6 个、甚至更多 Codex 或 Claude Code 账号。目前在
同一浏览器登录不同账号时，Provider Cookie/Session 会互相覆盖；在 Mac、
Linux 服务器和其他终端中，默认配置目录也会让后一次登录覆盖前一次登录。
用户因此无法可靠地回答“哪个账号已登录、哪个额度仍可用、这次任务实际会用
哪个账号”。

本功能的可测量结果是：用户能手动创建任意合理数量的账号和 Runtime Profile，
为每个 Profile 指定唯一别名（例如 `@A`），在同一看板查看经来源标记的登录
健康度、用量窗口和重置时间，并在启动任务前显式选择 Profile。账号选择在
Session 创建时冻结；系统不得自动轮换账号、绕过额度或在运行中透明切换。

## Scope

1. **手动账号注册表**
   - 手动添加、命名、禁用、重新登录、注销和删除 Account、
     CredentialInstance、RuntimeProfile。
   - 数量不写死；实现只受存储、分页和本机资源限制。测试至少覆盖 8 个 Profile。
   - 用户别名在本用户范围内大小写不敏感且唯一；显示形式为 `@<alias>`。
2. **Provider 认证隔离**
   - Codex：每个 CredentialInstance 使用 Daemon 管理的独立 `CODEX_HOME`，
     稳定路径使用 file credential store；同一 CredentialInstance 仍遵守
     ADR 0014 的单一可刷新凭据 writer。
   - Claude：每个 RuntimeProfile 使用独立 `CLAUDE_CONFIG_DIR`；稳定支持必须
     由两个不同真实身份的隔离、注销和实际请求证据确认。
   - Web 看板不保存、导入、抓取或复制 Provider Cookie、LocalStorage 或网页
     Session。Provider 浏览器登录由官方 CLI/OAuth 流程完成，可由用户选择独立
     浏览器 Profile 或 device-code 流程；最终凭据只能落入目标 Provider Home。
3. **统一用量模型与看板**
   - 将 Provider 返回的任意数量窗口归一化为 `UsageWindow[]`，支持滚动窗口、
     周窗口、月度/额度窗口、Spend Control 和未知窗口，不把 5 小时/周/月写死
     为固定 schema。
   - 每个账号卡片显示 Provider、别名、登录状态、可运行状态、窗口标签、已用或
     剩余百分比、重置时间、数据来源、置信度、采集时间和过期状态。
   - Provider 不提供某窗口时显示 `unavailable` 或 `unknown`，不得显示为 0%、
     “无限”或“官方剩余额度”。
4. **终端显式调用**
   - 稳定入口为 `multidesk run --profile @A ...` 和非交互等价入口；可提供
     `multidesk shell @A` 或安全生成的 shell completion/shim，但不得执行用户
     拼接的 shell 字符串。
   - 启动前输出 Provider、Account、RuntimeProfile、Device、认证健康度、
     Usage 来源和最近验证时间，并要求用户确认；非交互模式要求显式
     `--profile`，禁止默认猜测。
   - Session 持久化 `accountId + credentialInstanceId + runtimeProfileId` 快照，
     运行中不得切换。
5. **Mac 与服务器配置**
   - Mac 和 Linux 分别提供逐 Profile 的官方登录、健康检查、用量采集和注销
     流程。
   - 本地登录状态不能通过复制浏览器 Cookie “同步”到服务器。稳定远程授权只
     能使用经安全审查的 CredentialGrant；未支持时必须要求目标设备独立登录。
   - 所有远程授权都是单账号、单目标设备、显式确认、可撤销的操作。

## Non-goals

- 自动发现浏览器或磁盘中的账号。
- 自动添加、轮换、负载均衡或耗尽后切换账号。
- 把多个订阅账号聚合成代理池，或规避 Provider 额度/限流。
- 抓取 Provider 网页 Cookie、Session、LocalStorage 或未公开接口。
- 在 MultiAgentDesk WebView 中模拟或代理第三方账号登录。
- 宣称可以远程擦除已经复制到受权或失陷服务器上的凭据。
- 在缺少官方/可复现证据时伪造月额度、可用状态或重置时间。
- v0.1 自动迁移现有默认 Provider Home；迁移必须是显式、可预览且可回滚的后续
  阶段。

## User journeys

1. 用户在 Mac 上选择 Codex，输入别名 `A` 和显示名，MultiAgentDesk 创建隔离
   Provider Home，并在明确标识的官方登录流程中完成账号 A 登录；账号 B/C
   重复相同步骤但不会覆盖 A。
2. 用户为 Claude 创建 `@D`，通过该 Profile 的 `CLAUDE_CONFIG_DIR` 完成官方
   登录；完成后产品用经过版本门禁的 `auth status` 和真实请求身份验证绑定关系。
3. 用户打开 Accounts/Usage 看板，同时看到 A–D 的登录状态、可运行状态、各用量
   窗口、重置时间、来源和采集新鲜度；不可获取的月度窗口显示未知。
4. 用户运行 `multidesk run --profile @B --workspace <path>`；CLI 在创建 Session
   前展示绑定信息，用户确认后仅使用 B 的凭据。
5. 用户在远程 Linux 上创建同名 `@B`。若该 Provider 的 CredentialGrant 尚未
   被验证支持，CLI 明确要求在服务器为该 Profile 单独登录，不复制浏览器状态。
6. 用户禁用或删除 `@C`。系统先阻止新 Session，列出活动 Session/授权影响，
   再执行目标 Profile 的 Provider 注销和本地秘密删除；Provider 全局吊销仍由
   用户在官方安全入口完成。

## Data and trust boundaries

- `Account` 只保存逻辑身份和脱敏提示，不保存 Token、Cookie 或原始 auth JSON。
- `CredentialInstance` 的 `secretRef` 仅存在于所属 Device 的 Vault；Control Plane
  只接收脱敏状态与 UsageSnapshot 元数据。
- Provider Home 目录属于 Daemon，Unix 为 `0700/0600`，Windows 使用当前用户
  DACL；同一 Profile 不与其他 Profile 共享可写认证文件。
- Codex refresh 遵守“一 CredentialInstance 一 canonical writer + revision/CAS”；
  多 Session 可复用该 owner，但不能各自复制可刷新 `auth.json`。
- Claude macOS 凭据由官方 CLI 写入 Keychain。产品不得扫描或复制 Keychain；
  只调用目标 `CLAUDE_CONFIG_DIR` 下的官方登录、状态和注销命令。
- Provider Browser Cookie 始终位于用户选择的浏览器 Profile 中，不进入 Vault、
  Control Plane、日志或看板。OAuth 回调必须绑定发起登录的 Profile/nonce，并在
  完成后重新验证实际账号。
- Usage 只保留归一化窗口、来源、版本和采集时间；email、org、原始 Provider
  响应和终端捕获默认不持久化。

## Provider/external assumptions

- **已证实（Codex）**：仓库 Spike 在 CLI `0.142.5`、`0.143.0`、`0.144.2`
  上复现 `account/rateLimits/read` 与 `account/usage/read`；当前本机 `0.144.2`
  schema 仍包含 `RateLimitWindow.usedPercent/resetsAt/windowDurationMins` 以及
  primary/secondary/multi-bucket 结构。
- **需新 Spike（Codex）**：两个不同真实账号在两个 `CODEX_HOME` 的登录、
  app-server 账号绑定、局部注销、并发读取和浏览器回调隔离尚未由当前仓库证据
  覆盖。完成 headless device-auth 登录也仍未证实。
- **官方文档支持但需产品 Spike（Claude）**：`CLAUDE_CONFIG_DIR` 被官方文档
  明确用于并行多账号；status-line JSON 暴露 5 小时/7 天 used percentage 和
  reset epoch。但现有仓库 Spike 只有一个真实身份，且尚未验证将 status-line
  事件安全、无额外探测请求地归一化到 UsageSnapshot。
- **未知（Claude 月度）**：官方文档说明 `claude -p`/Agent SDK 自 2026-06-15
  使用独立月度 credit，但尚未发现稳定机器可读的剩余百分比/重置字段。该窗口
  在证据完成前必须显示 unknown/unavailable。
- **Provider policy gate（Claude）**：Anthropic 官方文档限制第三方产品提供
  Claude.ai 登录或 rate limits。稳定产品实现前必须得到适用于“用户自托管、
  调用本机官方 CLI、无请求代理”的合规结论；未经结论不得把 Claude 订阅登录/
  额度看板标成稳定支持。
- 所有 Provider schema 和 CLI 输出都必须按精确版本门禁；字段变化只允许降级，
  不允许猜测。

## Dependencies and gates

- 基线：远端 `main` 已于 `2026-07-15` 合并并 Ship Phase 1 Device Kernel；当前
  产品只实现 Fake Provider，真实 Codex/Claude Adapter、Account CRUD、Usage
  采集和 Web 产品页面均未实现。
- 依赖 ADR 0014（Codex 单 refresh writer）与 ADR 0016（当前 Claude
  target-local interactive login 边界）；新证据可通过新的 ADR 决策修改后者，
  不得静默绕过。
- 必须创建并关闭两个 Security-gated Provider Spike：
  `spike-codex-distinct-account-homes`、
  `spike-claude-distinct-account-usage`。
- Claude 稳定支持还需要 Provider policy/compliance 结论；未关闭时只能作为
  developer-only experimental evidence，不进入发布能力矩阵。
- 产品实施顺序：Provider 证据与公共合同 → Device account/profile/usage 内核 →
  Codex vertical slice → Claude vertical slice → Control Plane 元数据 → Web/Desktop
  看板。UI 不得先于真实数据能力宣称完成。
- Security Gate 保持 open，直至认证隔离、删除/注销、日志脱敏、Provider Home
  权限、远程授权和反串号测试全部接受。

## Acceptance criteria

- [ ] 用户可手动添加、编辑、禁用和删除至少 8 个混合 Provider Profile；实现中
      没有固定账号数常量，列表接口支持分页。
- [ ] 每个 Profile 有唯一 `@alias`，所有 CLI/API/UI 解析一致；歧义、缺失、禁用
      或跨 Provider 错配均 fail closed。
- [ ] 两个不同 Codex 账号在隔离 `CODEX_HOME` 中同时保持登录，局部注销和刷新
      不影响另一账号；app-server 实际身份与预期绑定一致。
- [ ] 两个不同 Claude 账号在隔离 `CLAUDE_CONFIG_DIR`/Keychain slot 中同时保持
      登录，局部注销和真实请求不影响另一账号；若不能证明则稳定能力保持阻塞。
- [ ] 看板对每个账号显示登录/可运行状态，以及 Provider 实际提供的 5h、周、
      月度或其他窗口；每一项包含 used/remaining、reset、source、confidence、
      observedAt，缺失值明确为 unknown/unavailable。
- [ ] Codex Usage 通过精确版本 schema 和 fixture 读取；Claude 5h/7d 只在
      status-line 结构实测后启用；Claude 月度字段未验证时不伪造。
- [ ] `multidesk run --profile @A` 与非交互入口在进程环境、Provider Home、
      Session 记录和审计事件中都绑定同一 Profile；反向 canary 测试证明不会串号。
- [ ] Mac、headless Linux 和 Windows CLI 路径均有明确登录/健康/调用说明；不支持
      的远程同步显示稳定错误和可执行的独立登录 fallback。
- [ ] 产品不读取或传输 Provider Cookie/LocalStorage；浏览器 Profile 仅用于用户
      完成官方登录，OAuth callback/state 绑定测试通过。
- [ ] 删除流程拒绝有活动 Session/Grant 的危险硬删，支持禁用、逐 Profile 注销、
      本地秘密删除和 Provider 侧全局吊销指引。
- [ ] 没有自动轮换、额度规避、请求代理、默认账号猜测或运行中凭据切换路径。
- [ ] Provider/Security/合规门禁、三平台测试、用户指南和开发看板全部与实际状态
      一致后，功能才可进入 Ship。

## Risks and open questions

- Claude 多真实身份的 macOS Keychain slot 命名与跨版本稳定性未知，需要用户
  提供两个测试账号或隔离测试环境。
- Claude status-line 数据只在首次 API 响应后出现；后台看板应复用真实 Session
  事件还是启动只读探针，必须在 Spike 中选择。默认不得为了刷新看板消耗额度。
- Claude 月度 Agent SDK credit 是否可从受支持 CLI/SDK 获取机器可读剩余量未知。
- Anthropic 对本地自托管管理器展示订阅登录/rate limits 的政策适用性需要明确。
- Codex 多账号浏览器 OAuth 若用户在错误浏览器 Profile 完成登录，必须依靠完成后
  的实际身份确认和显式重试，不能相信窗口标签。
- 账号删除、Provider 注销与 Provider 全局吊销不是同一个动作；UI 文案必须避免
  “已彻底撤销”的虚假承诺。
- 当前本地 `user-operations-guide` 分支尚未合入远端最新 Phase 1 基线；后续文档
  更新必须在独立集成决策后进行，不能把无关分支改动吸入本功能。

## Evidence

- `docs/IMPLEMENTATION_PLAN.md` §§2, 6, 8–11, 19–20, 25
- `docs/workflow/features/spike-codex-auth-refresh/dev_log.md`
- `docs/spikes/codex/2026-07-14-auth-refresh-spike.md`
- `docs/adr/0014-codex-app-server-single-writer-auth.md`
- `docs/workflow/features/spike-claude-config-keychain/dev_log.md`
- `docs/spikes/claude/2026-07-14-config-keychain-spike.md`
- `docs/adr/0016-claude-profile-interactive-login-boundary.md`
- `docs/PROVIDER_COMPATIBILITY.md`
- `docs/workflow/features/phase1-device-kernel/dev_log.md`
- OpenAI Codex manual: authentication, `CODEX_HOME`, credential storage and
  app-server schema generation (`https://learn.chatgpt.com/docs/auth`,
  `https://learn.chatgpt.com/docs/app-server`)
- Claude Code official docs: environment variables, authentication, CLI,
  status line, usage errors and Agent SDK/legal boundaries
  (`https://code.claude.com/docs/en/env-vars`,
  `https://code.claude.com/docs/en/authentication`,
  `https://code.claude.com/docs/en/cli-usage`,
  `https://code.claude.com/docs/en/statusline`,
  `https://code.claude.com/docs/en/errors`,
  `https://code.claude.com/docs/en/agent-sdk`,
  `https://code.claude.com/docs/en/legal-and-compliance`)
- Live local inventory on 2026-07-15: Codex CLI `0.144.2`, Claude Code
  `2.1.207`, one logged-in default Profile for each Provider, no additional
  `.codex-*`/`.claude-*` homes detected.

## Handoff

Next role: `feature-plan`.

Create the four canonical feature-state files, intake both blocking Provider
Spikes with open Security Gates, freeze the generic `UsageWindow[]` and alias
contracts, and sequence product implementation behind evidence. Do not claim
multi-account Claude, monthly Claude quota, headless Codex login, or stable
Claude subscription dashboard support until their named gates are resolved.
