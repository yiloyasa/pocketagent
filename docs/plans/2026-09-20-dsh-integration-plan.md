# PocketAgent 接入 DeepSeek Harness（dsh）——调研与实施设计

> 2026-09-20 · 状态：**调研定稿，未动任何生产代码**
> 结论先行：**走 `dsh --profile acp`（Agent Client Protocol over stdio）桥接，架构模式完全复制 codexapp**（一进程多会话 + JSON-RPC 2.0 + 反向审批请求）。daemon 侧新增 `dshapp` 包 + `DshProvider`，**proto 与 Android 零改动**，其他 agent 注册路径零触碰。
> 本机前置已就绪：dsh **0.1.5-rc.2** 已装（`G:\Claude Code\node_modules\@deepseek-ai\dsh`），Node v26.3.0 ✓，ACP `initialize` 握手实测通过。

---

## 1. 背景与目标

### 1.1 dsh 是什么

DeepSeek 官方开源 agent harness（`github.com/deepseek-ai/deepseek-harness`，MIT，2026-08 进入 developer preview）。基于 Cordis 插件架构（"Everything is a Plugin"），自带：agent 循环、沙箱化 bash/文件系统工具、JSONL 会话持久化、压缩、子代理、工作流、hooks、MCP、审批策略。**与 codex/pi/zcode 同类——独立进程跑完整 agent 循环，daemon 只做"翻译官"。**

与现有 BYOK 路径的本质区别：`openai_compat.go` 里已有 `DEEPSEEK_API_KEY` 直连（daemon 自跑裸循环，无工具/沙箱/会话），dsh 是**完整 harness**。两条路径并存不冲突（agent_id 不同：`deepseek` vs `dsh`）。

### 1.2 目标

1. 手机上能用 dsh 干活：建会话、流式聊天、审批弹卡、取消、重启后恢复；
2. 与 dsh 的快速演进**保持统一**：版本可锁、能力可探、升级有流程，不被 developer preview 的破坏性变更突袭；
3. **其他 agent（codex/pi/zcode/echo/deepseek-BYOK）零回归**：注册、探测、列表、轮次、审批互不干扰。

### 1.3 非目标

- 不内嵌/不改写 dsh 源码（与"不改写 `~/.codex/config.toml`"同款红线：**绝不写用户自己的 `~/.dsh`**）；
- 不做 dsh 插件开发/自定义 profile 编排（P3 再议）；
- 不动 proto（64 RPC 计数不变；若后续需要再走甲级变更流程）。

---

## 2. 已验证事实（本机实测 + 官方文档，2026-09-20）

| 项 | 值 | 验证方式 |
|---|---|---|
| 安装 | `@deepseek-ai/dsh@0.1.5-rc.2`，npm 全局 prefix=`G:\Claude Code`，入口 `node_modules\@deepseek-ai\dsh\lib\bin.js`（ESM） | `dsh --version` |
| Node 引擎 | `^22.19.0 || >=24.0.0`；本机 v26.3.0 ✓ | package.json + `node --version` |
| ACP 握手 | `dsh --profile acp` + stdin JSON-RPC → `initialize` 应答：`protocolVersion:1`，`agentInfo{name:"deepseek-harness-acp",version:"0.0.1"}` | 实测（stdin 管道） |
| 会话能力 | `sessionCapabilities:{close:{},list:{},resume:{}}`（**0.1.5 新增**，0.1.1 只有 automation-only 老面） | 同上 |
| 图片输入 | `promptCapabilities.image:false`（当前路由不支持） | 同上 |
| MCP | 仅 stdio + Streamable HTTP（无 SSE） | 官方 ACP README |
| stdout 纯净性 | "Stdout is reserved for protocol frames"——只会有 JSON-RPC 帧，日志走 stderr | 官方源码注释 |
| 认证 | `DEEPSEEK_API_KEY` 优先级链：进程 env → `$DSH_HOME/.credentials.yaml` → cwd `.env` → `~/.dsh/.env`；ACP `authenticate` 恒成功（服务端不鉴权，key 在 harness 内部解析） | 官方 quickstart/credentials 文档 |
| 发布节奏 | npm dist-tags：`latest=next=0.1.5-rc.2`、`alpha=0.1.6-alpha.2`；**周更级**，developer preview 明示会有破坏性变更 | `npm view` |
| Windows | 官方 discussion #4326 成功案例（Win11/PS5.1）；本机握手通过 | — |

### 2.1 ACP 方法面（dsh 0.1.5 实现，ACP v1 + 扩展）

| 方法 | 语义 | pocketagent 对应 |
|---|---|---|
| `initialize` | 协商协议版本 + 能力集 | 握手（探测/重挂载前置） |
| `session/new` | 新建持久会话，返回**完整 config-option 状态**（model/reasoning_effort 选项，来自 live catalog） | CreateThread / Models() 数据源 |
| `session/list` | 最新在前分页列出持久化可恢复会话（可选 cwd 过滤） | 重挂载校验 /（P2）原生会话导入 |
| `session/resume` | 恢复持久化会话，log 恢复**不重放旧 update** | daemon 重启后 ExternalSession 重挂载 |
| `session/close` | 静默取消+排空+落盘+销毁该会话 | DeleteThread |
| `session/set_config_option` | 运行时改 `model` / `reasoning_effort` | 模型/强度切换（PrefsAware 子集） |
| `session/prompt` | 一轮输入（文本/资源链接/图），**每会话一次一个 in-flight**，等 agent idle 终局（`end_turn`/`cancelled`） | StartTurn 一轮 |
| `session/cancel` / `$/cancel_request` | 取消当前 prompt | CancelTurn |
| `session/update`（通知） | assistant 消息块、思考、工具生命周期、配置变化、**上下文用量**；按会话串行 | 事件流 → Sink → TurnServerMessage |
| `session/request_permission`（反向请求） | **一次性 allow/reject**，客户端可代答 | 审批桥（ApprovalHook ↔ SubmitApproval） |

**已知粒度限制**：`session/update` 目前对 assistant 文本按"已提交消息的文本块"发 `agent_message_chunk`（非逐 delta 流式）；转发 live delta 尚在官方 discussion #594 提议中。→ 影响逐字上屏体验，**P0 探针必须量化实际粒度**。

---

## 3. 路径选型

| 候选 | 接口 | 判定 |
|---|---|---|
| **`--profile acp`** ✅ | 标准 ACP（JSON-RPC 2.0 over stdio），有审批反向请求、会话持久化/恢复、模型选项、流式 update | **入选**。与 codexapp 桥接模式完全同构，产品核心能力（审批弹卡/断线恢复）全覆盖 |
| `--profile sdk` | 换行分隔 JSON-RPC，但**无审批面**（headless 驱动） | 排除：丢审批 = 丢产品灵魂 |
| `--profile headless` | 一次性跑完打终稿退出 | 仅作探针/冒烟素材 |
| `dsh web` | 浏览器 UI（127.0.0.1:3080） | 排除：产品 UI 非机器接口，逆向违反项目红线风格 |
| 维持 BYOK（openai_compat） | daemon 自跑裸循环 | 保留现状，与 dsh 并存（`deepseek` vs `dsh` 两个 agent_id） |

---

## 4. 架构设计

### 4.1 进程模型：复制 codexapp，不是 piapp

dsh ACP = **一个长驻子进程承载任意多会话**（"multiple independent sessions per connection"，session/new 按需开），与 `codex app-server` 同构；piapp 的"1 子进程=1 会话"模型不适用。

```
手机 ──gRPC──► daemon server.StartTurn
                 └─ providerForThread("dsh") → agent.DshProvider
                      └─ dshapp.Client（懒启动·单例·可重启）
                           └─ 子进程: node bin.js --profile acp   [DSH_HOME=~/.pocketagentd/dsh-home]
                                ├─ 正向请求: session/new|prompt|cancel|resume|close|set_config_option
                                ├─ 通知→ sink: session/update → TurnServerMessage 帧
                                └─ 反向请求→ 审批桥: session/request_permission → ApprovalHook
```

### 4.2 新增包 `daemon/internal/dshapp/`

对标 codexapp/client.go（450 行，含测试范式）逐项复刻：

| 决策点 | 方案 | 依据/先例 |
|---|---|---|
| 定位 `LocateDsh()` | `DSH_BIN` env（可指 dsh.cmd 或 bin.js）→ 固定路径 `node.exe + G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js` → PATH 兜底 | WMI 派生环境不保证 PATH（piapp 同款教训）；`.cmd` 需经 `cmd /c`（codexapp 同款） |
| 启动 | 懒启动：首个 dsh 线程/模型列举时才拉起；崩溃自动重启 + 会话重同步（session/list 对账） | codex Warmup/lifecycle 模式 |
| 分帧 | 换行分隔 JSON-RPC 2.0；`bufio.Reader` **≥8MB** | 防御性（图/长上下文单行；piapp 64KB 必炸的教训） |
| DSH_HOME | 默认 `~/.pocketagentd/dsh-home`（daemon 数据目录内）；**daemon env 已显式设 DSH_HOME 时尊重用户选择** | 见 §5.3 隔离原则 |
| 凭据 | 只透传 `DEEPSEEK_API_KEY` env 给子进程；**不读写用户 `~/.dsh/.credentials.yaml`** | BYOK 一致 + 红线（不碰用户桌面 dsh） |
| stderr | 逐行收 `[dsh-stderr]` 前缀落 daemon.log；收尸 `waitOnce` 独占 | codexapp 2026-09-20 补齐的三方一致范式 |
| 窗口 | `hidewin.Apply(cmd)` | 控制台程序不闪黑窗 |
| 超时 | 会话级 callTimeout（参考 codex 120s），prompt 终局由 update 流驱动不受它管 | — |

### 4.3 新增 `daemon/internal/agent/dsh.go`：DshProvider

**必须实现**（`Provider` 接口）：`ID()="dsh"`、`DisplayName()="DeepSeek Harness(桌面 agent)"`、`Models()`、`Run()`。

**编译期钉死可选能力**（provider.go 现有模式）：

```go
var _ PrefsAware = (*DshProvider)(nil)   // 桌面偏好子集
```

| 能力 | 实现程度 | 说明 |
|---|---|---|
| `PrefsAware` | **子集**：`Model`→`set_config_option(model)`；`Effort`→`set_config_option(reasoning_effort)`；`HideReasoning`→过滤 ThinkingDelta | 审批策略/沙箱**不映射**——dsh ACP 固定一次性审批策略，与 zcode 固定 `mode:"build"` 同款边界，如实写进文档 |
| `Steerer` | **不实现** | ACP 一会话一 prompt；运行中插话自动落入 turn_queue 排队续跑（pi/zcode 同款） |
| `CloseSession` | 实现：DeleteThread → `session/close` | server/thread.go 已按可选接口探测 |
| `ErrorInfoCarrier` | 实现：MISSING_CREDENTIAL / 协议错误带结构化 Error | 终局 TurnFailed 帧保真 |
| `ModelCatalogRefresher` | 实现：`InvalidateModels()` 清缓存 | ListModels refresh=true 路径自动生效 |
| `AgentSessionImporter` | **P1 不做**（隔离 HOME 下无"用户桌面 dsh 会话"可导）；P3 若改共享 HOME 再议 | builtinSessionImporters 不动 |
| `TextGenerator` | 不实现（会话标题由 dsh 自带 session-title） | — |

**Models() 数据源**：`session/new` 应答携带完整 config-option 状态（model/reasoning_effort，来自 live catalog）→ 缓存 + TTL；无会话时用一次性瞬态会话取样后 `session/close`。**无静态模型名单**（设计原则 §7 合规：新增模型自动出现）。

**Run() 事件映射**（ACP `session/update.update` 变体 → `pb.TurnServerMessage`）：

| ACP | pb 帧 |
|---|---|
| `agent_message_chunk` | AssistantDelta（已提交文本块粒度，见 §2.1 限制） |
| `agent_thought_chunk` | ThinkingDelta（HideReasoning 时过滤；thinkingRecorder 落库链路自动生效） |
| `tool_call`（pending/running/in_progress/finished） | ToolCallBlock 状态机 |
| `plan` / `available_commands` | 透传/忽略（走 native_notifications"识别不了≠出错"分支，未知变体原样包装进诊断视图——主线 A） |
| `context_usage` / usage 字段 | UsageUpdate（上下文环数据源） |
| prompt 终局 `end_turn` / `cancelled` | 轮次完成 / TurnFailed(cancelled) |

**审批桥**（反向请求，`ReqHandler` 路径，对标 `codex_requests.go` 精确匹配分派）：

```
dsh: session/request_permission{sessionId, options:[allow, reject], ...}
 → daemon: 转 ToolCallBlock + ApprovalReason → sink(ApprovalRequest) → 手机弹卡
 → 用户 SubmitApproval(thread_id, request_id)
 → daemon 翻译为 ACP permission outcome{type:"selected", optionId: <用户选择>}
 超时(2min)/断线 → outcome 对应拒绝路径，绝不伪造"同意"（红线 1）
```

**ExternalSession 重挂载**（daemon 重启后）：

```
Thread.ExternalSession.SessionID = ACP sessionId（SessionFile 留空）
重挂载 = 懒启动子进程 → initialize → session/list 校验存在
        ├─ 在列 → session/resume（log 恢复不重放 update，正好避免手机端重复帧）
        └─ 不在 → 显式报错（不静默新开——"不 resume 就静默吞请求"的三方共同教训）
```

### 4.4 注册点：`provider.go` DetectProviders（唯一现有代码触碰）

```go
// dsh(桌面 agent 桥接):node + @deepseek-ai/dsh 入口都定位得到才注册。
// 探测只做"文件存在 + --version"(有缓存),绝不在启动时做 ACP initialize
// 握手(PA-REAL-0002/0003 教训:一个 agent 的冷启动不许拖住别人)。
if node, bin, err := dshapp.LocateDsh(); err == nil {
    if v := dshapp.Version(node, bin); v != "" {
        log.Printf("[providers] dsh 探测成功: version=%s", v)
        m["dsh"] = NewDshProvider()
    }
}
```

**注册与否不依赖 API key**（key 缺失时线程照建，首条 prompt 才以 MISSING_CREDENTIAL 如实报错——与 BYOK "钥匙在 env" 的语义解耦）。

### 4.5 Android 侧：预期零改动

批 3 整改后全部动态派生，逐一核对：

| 触点 | 现状 | dsh 接入后 |
|---|---|---|
| agent 列表顺序 | 从 `cfg.Providers` 动态派生（PrefsAware 前/其余字典序/echo 兜底） | dsh 实现 PrefsAware → 自动入列，**无需改** |
| CreateThread 未知 agent | 报错文案动态列举可用 agent | 自动含 dsh |
| GetHostInfo capabilities | `agent:<id>` + `agent_settings:<id>`(PrefsAware) | 自动出现 `agent:dsh`、`agent_settings:dsh` |
| ListModels | 并行有界拉取（10s 预算，慢者如实报"初始化中"） | dsh 自动参与并行 |
| 设置页 | ZCode/Pi 分区按 capability 点亮 | dsh 分区自动出现（PrefsAware 生效） |
| 可选 | agent 图标（`tools/make_agent_icons.py` 产 lucide 矢量） | 建议补（乙级资源变更） |

---

## 5. 与 dsh 保持统一（版本/演进对齐策略）

### 5.1 版本锁定与显式指定

- 生产桥接**锁已验证版本**：`DSH_BIN` 显式指向具体安装（如 `G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js`），README §5 记录当前验证版本（现 0.1.5-rc.2）——与 `CODEX_BIN` 同款哲学；
- daemon 启动日志 + `agent:dsh:v<版本>` capability 报版本（复制 `agent:pi:v0.85.1` 格式，手机引导弹窗可显示"已装 vX"）；
- **升级是显式动作**：`npm i -g @deepseek-ai/dsh@latest` → 跑 §5.4 验证流程 → 更新 README 锚点。绝不隐式跟随。

### 5.2 能力协商驱动，不写死协议假设（设计原则 §7 的直接应用）

- 一切扩展方法的使用以 `initialize` 应答为准：`sessionCapabilities.list/resume` 在场才走重挂载路径；`promptCapabilities.image=false` 时图片输入**如实报错**（不静默丢弃）；
- 未识别的 `session/update` 变体 → 主线 A"原样包装透传"（原文 + 变体名进诊断视图）；未识别的反向请求 → 回 JSON-RPC error，**绝不伪造 decision**（红线 1）；
- dsh 破坏性变更的最坏表现 = 新版本握手失败/方法消失 → 能力门控自动降级为显式报错，daemon 与其他 agent 不受牵连。

### 5.3 与用户自己的 dsh 使用互不干扰（隔离原则）

| 关注点 | 策略 |
|---|---|
| 配置/会话 | 桥接用 `DSH_HOME=~/.pocketagentd/dsh-home`（daemon 数据目录内）；用户 `~/.dsh` 的 profiles/会话/补丁**零读写** |
| 凭据 | 只透传 env `DEEPSEEK_API_KEY`；不碰用户 `~/.dsh/.credentials.yaml` |
| 进程 | 各起各的 `dsh --profile acp` 子进程，无共享状态 |
| 高级逃生门 | 用户显式给 daemon 设 `DSH_HOME` → 尊重（想让手机桥接复用自定义插件层的进阶玩法，自行承担） |

### 5.4 升级验证流程（每次 dsh 升级必跑）

1. `dsh --version` 对齐预期；
2. `dsh --profile acp --dump-config` 组合树含 `dsh-base`+`dsh-acp-app`；
3. `dshprobe`（§7 P0 探针）全绿：initialize/session 生命周期/update 粒度/审批/取消/重启恢复；
4. daemon 冒烟：`go test ./internal/dshapp/... ./internal/agent/...`；
5. 更新 README §5/§10 锚点（版本号）。

---

## 6. 其他 agent 零影响的保证

### 6.1 触碰面收敛（改动隔离）

| 文件 | 改动类型 |
|---|---|
| `daemon/internal/dshapp/*`（新） | 纯新增 |
| `daemon/internal/agent/dsh*.go`（新） | 纯新增 |
| `daemon/internal/agent/provider.go` | **仅** DetectProviders 追加注册块 + 编译期断言（不改任何现有注册逻辑） |
| proto / android / store / server | **零改动** |
| `tools/make_agent_icons.py` 产物 | 可选资源新增 |

### 6.2 隔离矩阵（四 agent 零交叉污染的机制保证）

| 维度 | 机制 | 已有先例 |
|---|---|---|
| 探测 | dsh 探测 = 文件存在 + `--version`（缓存 5min，对标 `cachedPiVersion`）；**禁止启动期 ACP 握手** | PA-REAL-0002/0003（ListModels 已并行+10s 有界，dsh 自动受益） |
| 进程 | 每 provider 独立子进程 | 三 agent 现状 |
| 锁 | dsh 自有 lifeMu/turnMu；同 provider 排队、跨 provider 并行 | pi/zcode 同款 |
| 注册表 | map 追加，现有键零触碰 | — |
| 事件总线 | dsh 帧 sink 单写者走既有 turn_transport（先落 eventstore 拿 seq 再推流） | 全 provider 共用，无新路径 |

### 6.3 回归清单（合入门槛）

- `go build ./...` + `go test ./...` + `CGO_ENABLED=1 go test -race`（**新增 dshapp 进 -race 清单**，与 server/agent/store/codexapp/piapp/zcodeapp 并列）；
- JVM 全量：`python tools/device/_pa_gradle.py test --rerun` + XML 时间戳核验（448 用例 0 失败）；
- **四 agent 零交叉污染**：现有三 agent 用例扩为四 agent（dsh 会话运行中，codex/pi/zcode 各自建会话跑轮次互不串扰；dsh 子进程崩溃不影响其他三方）；
- 真机：131 用例受影响模块全量 + 其余冒烟（L2，见 §9）。

---

## 7. 分阶段实施计划

### P0 —— dshprobe 探针（先行，不写生产代码）

`daemon/cmd/dshprobe`（一次性探针，对标 `cmd/pi*` 探针族，证据落 `docs/verify/evidence/dsh-p0-probe.md`）：

1. `initialize` 握手 + 能力集 dump（协议版本/扩展方法在场性）；
2. `session/new` → config-option 状态（**模型/effort 选项的真实形状**）；
3. `session/prompt "1+1"` → 逐帧记录 `session/update`：**量化流式粒度**（块级还是 delta 级）、tool_call 状态机形状、context_usage 字段；
4. 触发一次 `session/request_permission`（如让它跑个命令）→ 验证 allow/reject 应答形状与超时行为；
5. `session/cancel` 中断语义；
6. kill 子进程 → 新进程 `session/list` + `session/resume` → **跨进程恢复**（ExternalSession 重挂载的可行性实证）；
7. 无 key 场景：MISSING_CREDENTIAL 错误形状（ErrorInfoCarrier 映射素材）；
8. 首次 profile 自动初始化耗时（冷启动预算）。

### P1 —— MVP（甲级变更，一次合入）

- `dshapp` 包（§4.2 全项）+ 单测（framing/重启/审批并发，对标 codexapp 测试文件族）；
- `DshProvider`（Provider + PrefsAware 子集 + CloseSession + ErrorInfoCarrier + ModelCatalogRefresher）；
- DetectProviders 注册 + `agent:dsh:v` capability；
- ExternalSession 重挂载 + DeleteThread 收尾；
- 审批桥（含 2min 超时拒绝路径）；
- 回归清单 §6.3 全过 + 真机 L2。

### P2 —— 打磨

- agent 图标、设置页 dsh 分区真机核对；
- README 全面更新（§3 目录树、§6 外部依赖表加 dsh 行、§10 锚点：代码行数/测试基线/版本）；
- 未知 update 变体透传的 B 开关（全局设置，默认关）对 dsh 的适配核对。

### P3 —— 远期（不在本次范围）

- 原生会话导入（需重新评估 HOME 隔离策略）；MCP 透传；dsh 插件层自定义。

---

## 8. 风险登记册

| # | 风险 | 等级 | 缓解 |
|---|---|---|---|
| R1 | developer preview 破坏性变更（周更节奏） | 高 | §5.1 版本锁定 + §5.2 能力门控 + §5.4 升级流程 |
| R2 | 流式粒度=已提交块，非逐字（#594 未落地） | 中 | P0 量化；UI 按块上屏仍可用；跟踪上游 discussion |
| R3 | 审批只有一次性 allow/reject（无"始终允许"） | 低 | 审批卡如实只出两键；文档写明边界 |
| R4 | 图片输入当前路由不支持 | 低 | 如实报错，不静默丢；能力协商自动随版本放开 |
| R5 | 跨进程 resume 未实证（文档说持久化，没说跨进程） | 中 | P0-6 专门验证；失败则降级方案=重开新会话+显式报错 |
| R6 | ACP 扩展方法形状与文档漂移（0.1.6-alpha 已在路上） | 中 | 探针钉形状；实现层用 RawMessage 透传未知字段 |
| R7 | 长 prompt/大上下文单行帧 | 低 | 8MB 读缓冲（piapp 教训） |
| R8 | dsh 子进程泄漏/僵死 | 低 | 复刻 reapWithLog + waitOnce + 空闲回收（可选 term reaper 思路） |
| R9 | 与 BYOK deepseek 的用户认知混淆 | 低 | 显示名区分："DeepSeek Harness(桌面 agent)" vs "DeepSeek"(直连) |

---

## 9. 审核与验收

- **变更分级：甲级**（AUDIT §1"新增 agent"）→ L0 闸门（P0=0 + 产物自证：daemon 新进程核验/APK 不涉）+ L1 全量 + L2 全量 + 契约双向核验；
- **proto 不动**：6 服务/64 RPC 计数不变，提交信息中显式说明（红线 3 的反向合规）；
- **设计原则三问**：无新增静态名单（模型来自 live catalog、agent 顺序动态派生、无版本二分判断）；未识别输入走透传/error 两路；UI 显示原生原文；
- **缺陷即测试**：R5（跨进程 resume）等探针发现的问题固化成 go 回归用例；
- **真值锚点**：README §10 增补 dsh 行（版本/探测结果/代码行数/测试基线），同次提交更新；
- **DoD**：真机上 dsh 会话完成 建会话→聊天→审批→取消→daemon 重启恢复→删除 全链路；四 agent 零交叉污染；既有 131 用例无回归（FAIL=0，BLOCKED=0 或带归因）；`-race` 全绿。

---

## 10. 附录

### A. 参考实现对照表（写代码时逐个对标）

| 关注点 | 参考文件 |
|---|---|
| 子进程 JSON-RPC 客户端骨架（pending map/handler 快照/waitOnce/stderr drain） | `daemon/internal/codexapp/client.go` |
| 一进程多会话的生命周期锁/withClient/重启 | `daemon/internal/agent/codex.go`（withClient/withClientCtx/Warmup） |
| 审批反向请求分派（精确匹配 + 不伪造 decision） | `daemon/internal/agent/codex_requests.go` |
| 未知事件透传 | `daemon/internal/agent/native_notifications.go` |
| 定位/版本探测（不依赖 PATH + 缓存） | `daemon/internal/piapp/client.go` LocateNode/LocateCLI + `server/host.go` cachedPiVersion |
| ExternalSession 落库/合并 | `daemon/internal/store/store.go` ExternalSessionRef |
| 重挂载握手纪律（校验不能省） | `pi.go` switch_session+get_state 注释 |

### B. 本机环境快照（2026-09-20）

- dsh 0.1.5-rc.2 @ `G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js`（npm prefix `G:\Claude Code`，`dsh.cmd` 在 PATH）
- Node v26.3.0（`C:\Users\yanzi\AppData\Local\Programs\nodejs\node.exe`）
- `~/.dsh` 尚不存在（用户未自用 dsh；隔离 HOME 方案无存量包袱）
- DEEPSEEK_API_KEY：当前 shell 未见（接入前需配 daemon 环境变量）

### C. 信息来源

- 官方仓库与文档：github.com/deepseek-ai/deepseek-harness（apps/cli/README.md、packages/acp/acp/README.md、docs/user/guide/python-sdk.md）、deepseek-harness.github.io（reference/、guide/providers）、agentclientprotocol.com（ACP v1 规范）
- 本机实测：dsh --version / --help / --profile acp --dump-config / initialize 握手（2026-09-20）
- npm 元数据：dist-tags 与发布时间（`npm view @deepseek-ai/dsh`）
