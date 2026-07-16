# MultiAgentDesk 用户操作手册（预发布）

> 当前状态（2026-07-16）：Phase 1 Device Kernel 与 Phase 2 Codex Vertical
> Slice 均已进入远端 `main`；Phase 2 已完成本地 Ship、独立验证、Security
> Review 和七项检查保护的 PR 集成。仓库可从源码构建开发者预览，但仍没有
> 受支持的安装包、正式 Release、Control Plane、Web 远程终端或 Desktop
> 成品。不要把开发者预览当成生产版本。

## 1. 先判断你现在能做什么

| 标记 | 含义 |
|---|---|
| **当前可执行** | 仓库开发工具，以及从源码构建的 Phase 1/2 本地开发者预览 |
| **规划中** | 已写入 v0.1 实施计划，但对应产品代码尚不可用 |
| **待验证** | 还需要对应 Phase、Provider Spike、安全审查或跨平台测试证明 |
| **Experimental** | 计划提供预览，但不属于 v0.1 稳定承诺 |

无需构建产品即可运行的入口是本地开发看板：

```bash
npm run project:verify
npm run dashboard:serve
```

然后打开 `http://127.0.0.1:4178`。这组命令只展示仓库开发状态，不会启动
MultiAgentDesk 产品。仓库要求 Node.js 24 和 pnpm 10.23.0；如果当前 shell
没有 `npm`，可使用仓库配置的 bundled Node/pnpm 运行相同脚本。

## 2. v0.1 计划解决什么问题

MultiAgentDesk 面向同时使用多台本地/远程设备和多个 Codex、Claude Code
Profile 的单个开发者。计划中的完整闭环是：

1. 在每台设备安装一个 `multidesk` CLI/Daemon。
2. 在可信设备上登录 Codex 或 Claude Code，并把账号保存到本地 Vault。
3. 为账号建立互不污染的 Runtime Profile。
4. 用户确认推荐账号后启动 Session。
5. 从另一台本地 CLI、浏览器或 macOS Desktop 查看并接管 Session。
6. 把指定 CredentialInstance 显式授权给指定远程设备。
7. 随时撤销设备或授权；必要时同时到 Provider 官方安全入口彻底吊销凭据。

MultiAgentDesk 不会自动轮换账号、规避额度或限流、代理 Provider 请求、抓取
浏览器 Cookie，也不会在运行中的 Session 里静默切换凭据。

## 3. 平台和阶段可用性

下面是目标，不是当前支持声明。只有相应 Phase 的 `dev_log.md` 完成验证后，
该行才能升级为可用能力。

| 能力 | 目标平台 | 解锁阶段 | 当前状态 |
|---|---|---|---|
| CLI / Device Daemon | macOS、Windows、Linux | Phase 1 | 开发者预览；三平台基础已验证 |
| Codex 真实 Session | Linux x86_64、CLI `0.144.2` | Phase 2 + Codex Spike | 已进入 `main` 的源码开发者预览 |
| Codex schema/handshake | macOS arm64、CLI `0.144.2` | Phase 2 | 已验证 smoke；非完整 Session 支持 |
| Windows Codex | Windows amd64 | 后续平台验收 | build/protocol only；真实运行不支持 |
| Claude Code PTY Session | macOS、Windows、Linux | Phase 3 + Claude/ConPTY Spike | 待验证 |
| Control Plane 元数据页面 | Linux 自托管 Server + 浏览器 | Phase 4a | 规划中 |
| Web 远程终端、审批和控制权 | 现代桌面浏览器 | Phase 4b + E2EE/Browser Spike | 待验证 |
| macOS Desktop | macOS | Phase 5 | 规划中 |
| Windows Desktop | Windows | Phase 5/6 | Experimental |
| 安装包、升级/卸载和 Release | 发布平台 | Phase 6 | 规划中 |

## 4. 安装前准备（规划中）

> 本节描述发布后的前置条件。当前没有可用安装包或下载地址。

准备以下内容：

- 你要使用的 Codex CLI 或 Claude Code CLI；Provider 版本必须出现在未来的
  `PROVIDER_COMPATIBILITY.md` 已验证范围内。
- 至少一台可信设备，用于保存本地 Vault 和作为初始 Trust Anchor。
- 若使用远程功能：一台运行 Control Plane 的 Linux 主机、稳定域名、HTTPS，
  以及时间同步（NTP）。
- 若使用 Web 远程终端：支持安全保存 Device Key 的浏览器。不能可靠保存
  密钥的浏览器只能查看元数据。
- Headless Linux 若没有系统密钥环，需要由用户设置 Vault 密码；不要把密码
  写进命令行参数、脚本日志或 shell history。

最终的安装、升级、卸载和数据保留步骤由 Phase 6 提供。在 Phase 6 验证前，
不要根据本手册自行下载来源不明的二进制。

## 5. 初始化本地设备和 Daemon（开发者预览）

当前没有安装包。开发者可以从已验证源码构建并使用显式 Device root：

```bash
go build -o ./bin/multidesk ./cmd/multidesk
./bin/multidesk init --root <device-root> --name "My Device" --json
./bin/multidesk daemon serve --root <device-root>
./bin/multidesk daemon status --root <device-root> --json
./bin/multidesk vault status --root <device-root> --json
```

计划行为：

1. `multidesk init` 创建 Device ID、设备签名/交换密钥和本地数据目录。
2. Daemon 成为账号、Vault、Provider 进程和 Session 的本地事实源。
3. CLI 通过本地 IPC 与 Daemon 通信，不直接把 SQLite 当作 UI API。
4. Vault 锁定时仍可查看非秘密元数据，但不能启动需要凭据的新 Session。

首次 Vault 初始化需要从标准输入读取两次匹配密码；后续解锁读取一次。密码
不得放进 argv、脚本日志或 shell history：

```bash
./bin/multidesk vault initialize --root <device-root> --password-stdin
./bin/multidesk vault unlock --root <device-root> --secret-stdin
```

自动解锁会降低安全边界。只有理解“它只能保护静态数据库，不能抵抗主机
root 或磁盘读取”后，才考虑受限 keyfile 或 systemd `LoadCredential=`。

## 6. 登录 Provider 和管理账号

Codex Phase 2 开发者预览提供以下薄 CLI；Claude 登录仍属于 Phase 3：

```bash
./bin/multidesk accounts create --root <device-root> --provider codex --name <display-name> --json
./bin/multidesk accounts list --root <device-root> --json
./bin/multidesk auth begin --root <device-root> --profile-id <profile-id> --json
./bin/multidesk auth status --root <device-root> --credential-id <credential-id> --json
./bin/multidesk provider health --root <device-root> --json
./bin/multidesk usage --root <device-root> --provider codex --account <account-id> --json
```

`auth begin` 只启动官方、owner-bound、十分钟有效的 `codex login` 流程；不会
读取浏览器 Cookie 或接收 raw token 参数。稳定支持范围只包括 Linux x86_64
Codex CLI `0.144.2`，其他版本/schema 必须明确返回 unsupported。

计划流程：

1. 选择 Provider 并完成它的官方登录流程。
2. MultiAgentDesk 创建逻辑 Account 和当前设备上的 CredentialInstance。
3. 运行账号列表/详情，确认 Provider、健康状态、最近验证时间和授权设备。
4. 查看 Usage 时同时核对来源和采集时间。Claude Code 没有经过验证的官方
   订阅剩余额度时，界面不得把估算值标成“官方”。

Codex 采用单个 CredentialInstance 单写者、revision CAS 和隔离
`CODEX_HOME`；multi-writer refresh 与已完成 device auth 不受支持。Claude 的
Config Dir/Keychain 隔离、`auth status` 和 setup-token 路径仍以 Phase 3
计划与 ADR 0016 为准。

## 7. 创建 Runtime Profile（开发者预览）

```bash
./bin/multidesk profiles create --root <device-root> --device-id <device-id> \
  --account-id <account-id> --provider codex --name <profile-name> \
  --settings '{"approval_policy":"on-request","sandbox":"workspace-write"}' --json
./bin/multidesk profiles list --root <device-root> --json
./bin/multidesk profiles validate --root <device-root> --json <profile-id>
```

创建时按交互提示选择 Provider、账号、模型偏好、非秘密环境变量、MCP、Skill、
Hook 和工作区默认值。每个 Profile 应使用独立运行目录：Codex 使用独立
`CODEX_HOME`；Claude 的隔离方式以最终兼容矩阵为准。

不要在 Profile 的非秘密环境变量里保存 Token、Cookie、恢复码或 setup-token。
修改 Profile 只影响以后启动的 Session，不应改变已经运行的 Session。

## 8. 启动和控制 Session（Codex 开发者预览）

以下命令已存在于 Phase 2 源码；占位符不是实际 ID。真实 Codex 支持仍受
精确 Linux `0.144.2` 兼容矩阵约束，Claude 命令尚未实现：

```bash
./bin/multidesk run codex --root <device-root> --workspace <workspace-id> \
  --device-id <device-id> --credential-id <credential-id> \
  --profile-id <profile-id> --account-id <account-id> --json
./bin/multidesk sessions list --root <device-root> --json
./bin/multidesk sessions attach --root <device-root> --mode observer --json <session-id>
./bin/multidesk sessions observe --root <device-root> --from-sequence 0 --json <session-id>
./bin/multidesk control acquire --root <device-root> --json <session-id>
./bin/multidesk terminal input --root <device-root> --revision <lease-revision> \
  --sequence 1 --payload <text> --json <session-id>
./bin/multidesk sessions stop --root <device-root> --revision <lease-revision> \
  --json <session-id>
```

启动前必须显示并让用户确认：Provider、Account、RuntimeProfile、Device、Usage
来源和最近验证时间。系统只做推荐排序，不会自动切换账号。确认后 Session
固定账号、Profile、设备和能力快照。

一个 Session 可以有多个 observer，但同一时间只有 ControllerLease 持有者
可以输入、调整终端尺寸或响应审批。客户端断开不会自动停止 Provider 进程。
需要结束时使用规划中的 `sessions stop`；只有正常停止无效时才使用 `kill`。

## 9. 启用远程访问（规划中，Phase 4a/4b）

### 9.1 部署 Control Plane

Control Plane 的最终 Docker Compose、Caddy/Traefik TLS 和备份步骤要到
Phase 6 才会发布。当前不要假设镜像名、端口或环境变量已经稳定。

首次启动计划生成 10 分钟有效、只显示一次的 Bootstrap Token。Bootstrap
Ceremony 必须在同一次流程完成：

1. 创建首个单用户账号。
2. 注册 Passkey，并保存一次性 Recovery Codes。
3. 登记一台具有 OS Vault 的 Daemon/Desktop 为初始 Trust Anchor。

纯浏览器不能单独成为初始 E2EE 信任根。生产部署必须使用稳定域名/RP ID
和 HTTPS；更换域名会使原 Passkey RP ID 失效。

### 9.2 配对设备

以下命令当前不可执行：

```bash
multidesk devices pair
multidesk devices list
```

新旧两端会显示六组四字符指纹。必须在两个独立屏幕上逐组核对，完全一致后
才批准。Control Plane 返回的公钥只是索引，不能替代本地 pin；密钥变化应
视为新设备并重新配对。

### 9.3 使用 Web 或 Desktop

计划中的 Web/Desktop 页面包括 Overview、Devices、Accounts、Profiles、
Sessions、Terminal、Approvals 和 Settings。

Passkey 登录后，新的浏览器只拥有元数据访问权。它还必须作为 Web Device
由已批准设备完成 Enrollment，才能收到 Session Key、解密终端、响应审批或
发起 Credential Grant。清除浏览器站点数据会丢失 Web Device 私钥，需要
创建新 Device ID 并重新配对。

## 10. 把凭据授权给指定设备（规划中，Phase 5）

以下命令当前不可执行：

```bash
multidesk credentials grant
multidesk credentials status
multidesk credentials revoke
```

授权时只选择一个来源 CredentialInstance 和一个目标 Device，核对目标设备、
Provider、账号、有效期和指纹后再确认。Control Plane 最多暂存短时密文，
不能看到 Provider 明文凭据。

CredentialBundle 只能包含必要的 Provider 标识、认证方式、秘密、最小 Profile
引用和有效期；不能包含 Cookie jar、浏览器 LocalStorage、整个 Home 目录或
无关 Session 文件。

## 11. 撤销设备或凭据

计划中的撤销会阻止目标设备创建新 Session，并关闭被撤销设备的新连接；
相关活动 Session 需要轮换 Session Key。

但是：**撤销 MultiAgentDesk 中的 Device 或 Credential Grant，不能保证远程
擦除目标设备已经复制的秘密。** 如果设备丢失、被入侵或不再可信，还必须到
Codex/Claude 对应的官方账号安全入口撤销会话、重置 Token 或重新登录。

撤销前后都应查看设备、授权和审计事件，确认目标 ID 正确。不要把“界面显示
revoked”当作 Provider 侧凭据已经失效的证明。

## 12. 离线和断线行为

- Control Plane 离线：本地 CLI/Daemon 和已有本地授权应继续工作；远程 UI
  显示离线。
- Web/Desktop 断线：本地 Provider Session 不停止；重连后按 sequence 请求
  Ring Buffer 回放。
- 回放超出 Buffer：界面标记 `truncated`，可能丢失更早的滚屏上下文。
- ControllerLease 持有者失联：计划在租约到期后释放控制权，Session 继续运行。
- Daemon 重启且 Vault locked：仍可查询元数据；显式解锁后才能启动新 Session。

## 13. 常见问题与安全恢复

| 现象 | 计划中的安全处理 |
|---|---|
| `multidesk` 不存在 | 当前产品尚未发布；不要安装来源不明的二进制，等待 Phase 6 |
| Provider binary missing | 安装受支持的官方 Provider CLI，再运行 Profile 健康检查 |
| `vault_locked` | 使用标准输入显式解锁；不要把密码放到命令参数 |
| Credential expired | 阻止新 Session；重新进行官方登录，不自动换账号 |
| Profile unhealthy | 检查 Provider 二进制、版本门禁和 Profile 隔离配置 |
| Device key changed | 拒绝敏感操作，重新核对指纹和配对，不接受静默换钥 |
| 浏览器只能看元数据 | 浏览器尚未 Enrollment，或不能安全保存 Device Key |
| 不能输入终端 | 当前客户端可能只是 observer；申请 ControllerLease |
| 终端重连显示 `truncated` | 输出已超出内存 Ring Buffer；以 Provider 原生历史为补充 |
| Control Plane 不可用 | 继续本地操作；恢复 Server 后等待出站连接自动重连 |
| 怀疑服务器泄露凭据 | 撤销 Device/Grant，并立即在 Provider 官方入口彻底吊销 |

导出诊断信息前必须脱敏。不要在 Issue、截图或日志中提交 Token、Cookie、
setup-token、Recovery Code、Vault 密码、认证文件正文或完整终端秘密输出。

## 14. 发布前如何确认手册真的可执行

每次产品 Phase 改变命令或用户路径时，必须同步更新本手册。v0.1 发布前至少
检查：

1. 安装/升级/卸载命令来自真实 Release 产物，而不是示例。
2. macOS、Windows、Linux 的稳定/Experimental 标签与真实测试证据一致。
3. Codex/Claude 命令和版本范围与 `PROVIDER_COMPATIBILITY.md` 一致。
4. 从初始化到本地 Session 的路径在三平台按承诺通过。
5. Web 配对前只能看元数据，配对后才可解密和控制。
6. Credential Grant、撤销、Provider 侧吊销提示和离线行为完成 E2E 验证。
7. 没有未处理的 Critical/High 安全缺陷。

项目当前真实进度以各功能的
[`docs/workflow/features/<slug>/dev_log.md`](workflow/features/README.md) 和本地
开发看板为准。完整架构、阶段与验收基线见
[`docs/IMPLEMENTATION_PLAN.md`](IMPLEMENTATION_PLAN.md)。
