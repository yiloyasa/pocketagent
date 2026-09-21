# PocketAgent 多 Agent 协同工作 —— 调研与分层实施方案

> 2026-09-20 · 状态：**调研定稿，未动任何生产代码**
> 结论先行：分三层递进——**L1 原生协同透传**（让 harness 自带的子代理在手机上可见，乙级小改）、**L2 跨 Agent 委派桥**（daemon 内嵌一个本地 MCP 服务器，任何 harness 通过标准 MCP 工具把子任务委派给其他 agent，零 proto 改动）、**L3 Mission 编排服务**（声明式多 agent 流水线 + 手机端协同总控视图，甲级）。
> 核心洞见：**MCP 是四家 harness 唯一共用的"工具语言"**。把 daemon 从"只读 codex MCP 状态"升级为"本地 MCP 服务器宿主"，一个接缝打通跨 agent 委派，且完全符合项目两条铁律——"daemon 只做翻译官"与"能透传就透传"。

---

## 1. 范围界定："多 Agent 协同"是三个层次的问题

| 层次 | 问题 | 现状 |
|---|---|---|
| **可见性** | harness 内部**已经在协同**（codex 子代理/审查者、dsh subagents/workflows），手机看不见 | ❌ codex 的 `subagentStart/subagentStop` 通知目前落入 `codex_events.go` 默认静默丢弃分支（agent 包 grep 零处理） |
| **委派** | 让 A agent 把子任务交给 B agent（**跨 harness**：codex 实现、zcode 审查、dsh 测试…） | ❌ 无通道 |
| **编排** | 多 agent 的计划/并行/汇聚/断线恢复，手机上有总控视图 | ❌ 无任务级聚合；TaskCenter 只跟踪单线程消息/下载 |

**非目标**：不在 daemon 里自研"第五个 agent 大脑"（协同决策权要么留在原生 harness，要么交给用户手机）；不做云侧编排（零云原则）；不动 proto 除非走 L3（甲级流程）。

---

## 2. 现状盘点：可复用资产与缺口

### 2.1 资产（协同的基建大多已在）

| 资产 | 位置 | 对多 Agent 协同的意义 |
|---|---|---|
| StartTurn 双向流 + 事件总线 | `server/thread.go` + `eventstore`（全局单调 seq） | **多 agent 事件天然可交织**：每帧带 seq 落盘，断线重放不丢不重——多线程并发轮次的事件汇流只差聚合视图 |
| 跨 provider 并行事实 | 独立子进程 + 各自 turnMu/lifeMu | codex/dsh/pi/zcode 四方轮次本就并行；**同 provider 串行排队**（README 铁律"不可宣称并行"） |
| 审批中继 + 断线补交 | `approval_resume.go`（thread_id+request_id，2min 上限） | 子 agent 的审批卡可复用整条链路，只需加来源标注 |
| "/计划"审查流 | `StartReview` RPC + `codex_review.go` | **已上线的"第二意见"原语**：审查事件回原线程时间线直接可见——L1 透传的现成范式 |
| 审查者桥 | `codex_guardian.go` | guardian 本身就是 codex 的子代理：结论可见、用户可推翻、绝不伪造——审批语义的黄金先例 |
| 运行中插话 | `turn_queue.go` + `Steerer` | coordinator 等待子任务时可被插话/排队续跑 |
| TaskCenter | 手机本地组件（消息/下载任务，杀进程不丢） | L3 协同视图的壳 |
| MCP 只读面 | `model_ext.go` `ListMcpServers`（转发 codex 自己的 MCP 状态） | L2 的种子：daemon 已"看得见"MCP，只差"当宿主" |
| 会话重挂载 | ExternalSession + 各 app 包 resume 纪律 | 子任务线程断电后各自恢复的能力已验证 |

### 2.2 缺口

1. 原生子代理活动**不可见**（codex 通知被静默丢弃；dsh 的子代理走 tool_call 帧应已可见，待验收核对渲染）；
2. 无**跨 agent 委派**通道：四家 harness 各说各的协议，谁也不能叫谁；
3. 无**任务级**聚合：用户在手机上看不到"这个大活拆成了哪几个 agent、各自到哪了"；
4. 无跨线程**取消级联**与用量合计。

---

## 3. 业界参考与设计准则提炼

| 来源 | 关键机制 | 借鉴点 |
|---|---|---|
| **pi-subagents**（本机实装，`~/.pi/agent/npm/node_modules/pi-subagents`） | 有界委派（`{agent,task}`）、编排=代码（`runs.run/runs.all/lanes`）、async+resume、worktree 隔离、审批/证据门、steer | 父持决策权、子不再生子（默认一层）、委派必须**被授权**、编排用声明式 lanes 而非自由脚本（首版） |
| **dsh**（官方文档） | subagent 是**能力接缝**（`ctx.subagents` 注册表，模型只见一个委派工具，传输可换）；workflow 接缝（模型写编排脚本）；`dsh-subagent-acp` 用 ACP 当子代理传输 | "委派工具对模型只有一个，后端多提供商"——跨 harness 委派对 coordinator 暴露为单个 `delegate` 工具；编排脚本化是远期而非首版 |
| **ACP 规范** | 子代理活动以**通用 tool_call** 呈现（kind/title/content/raw_output），嵌套代理对客户端就是工具卡 | L1 的呈现标准：子代理=特殊工具卡，不发明新帧类型 |
| **codex app-server schema**（本仓库 `codex-schema/`） | `SubagentStart/Stop` 通知、`SubAgentActivityThreadItem`、`subagents` 配置、`ExternalAgentConfig*`（外部 agent 配置探测/导入）、审批路由 `auto_review`（"carefully prompted subagent"） | L1 的协议事实来源；codex 自己已在把"外部 agent"当一等公民 |
| Claude Code / OpenCode 等 | subagents.toml、并行任务 | 印证"子代理=工具"是行业收敛形态 |

**提炼七条准则**（贯穿下文设计）：

1. 委派对模型是**一个工具**，后端多 agent（dsh 接缝哲学）；
2. 子代理活动以**工具卡**呈现（ACP 哲学），不新造帧；
3. **父持决策权**，子返回有界摘要，不灌全文（pi-subagents）；
4. 委派必须**被授权**：默认首次跨 agent 委派要用户在手机上确认（红线 1 的推广）；
5. 子不生子（默认深度 1），防环防失控；
6. 失败**如实回传**给 coordinator（工具结果=错误文本），重试与否由模型/用户决定，daemon 不代拟；
7. 同 provider 排队是**事实约束**，UI 如实显示"排队中"，不伪造并行。

---

## 4. 总体架构：三层递进

```
                 ┌────────────────────────────────────────────┐
   L3 编排层     │ Mission 服务:声明式流水线/并行评审 + 手机协同视图 │  (甲级,新 proto)
                 ├────────────────────────────────────────────┤
   L2 委派层     │ daemon 内嵌本地 MCP 服务器 "pocketagent-collab" │  (乙级,零 proto)
                 │   工具: agent_delegate / mission_status          │
                 │   codex ─┐ pi ─┐ dsh ─┐ zcode(能力门控)          │
                 │          └──MCP──┴──MCP──┘ → 子线程/子轮次         │
                 ├────────────────────────────────────────────┤
   L1 透传层     │ 原生子代理活动 → 工具卡帧(codex subagentStart/Stop…)│  (乙级,零 proto)
                 └────────────────────────────────────────────┘
```

**为什么是这三层而不是别的**（决策矩阵）：

| 备选架构 | 否决理由 |
|---|---|
| daemon 自研编排循环（daemon 当第五个大脑） | 违反"daemon 不重写"铁律；openai_compat 的自跑循环是 BYOK 例外，不是架构方向；模型决策质量拼不过原生 harness |
| 手机端哑编排（UI 顺序发轮次） | 无模型驱动决策、无失败重试智能；但 L3 的"用户手动排布"可以退化到它，作为兜底交互保留 |
| 每家 harness 各写私有委派桥 | 4×3 对接矩阵，静态名单债（README §7 红线）；MCP 一条接缝全通 |
| 等单一 harness 内部协同成熟（只做 L1） | 跨 harness 组合（codex 写+zcode 审）是本项目独有价值——四家都是一等公民是产品事实 |

---

## 5. L1：原生协同透传（先让"已经在发生的协同"可见）

**改动面**：`agent/codex_events.go`（或新 `codex_subagents.go`）+ 手机端工具卡渲染核对。**零 proto**。

1. **codex**：`subagentStart`/`subagentStop`（ServerNotification，本机 0.149.1 schema 在库）→ 翻成 `ToolCallBlock` 风格卡片（"子代理启动：〈描述〉/ 子代理结束：〈摘要〉"），未知字段按主线 A 原样透传进诊断视图；
2. **dsh**：subagent 走 `session/update` 的 tool_call 帧（§3 ACP 哲学），预期**天然可见**——接入 dsh 后（见 dsh 接入方案）验收核对渲染即可；
3. **zcode/pi**：能力门控——有类似通知就同样处理，没有就不误报；
4. 验收场景：codex 桌面端把 approvals_reviewer 设为 auto_review 时，手机能看到审查者子代理的活动轨迹与结论卡（与 guardian 桥互补：guardian 是结论，L1 是过程）。

**审核**：乙级（daemon agent + android ui）。

---

## 6. L2：跨 Agent 委派桥（核心设计）

### 6.1 接缝选型：daemon 内嵌本地 MCP 服务器

**为什么是 MCP**——四家 harness 的工具扩展面支持度：

| harness | MCP 支持 | 接入路径 |
|---|---|---|
| codex | ✓（app-server 自带 MCP，schema 在库） | 见 6.3 红线处理 |
| dsh | ✓（ACP 能力协商 `mcpCapabilities`，stdio + Streamable HTTP） | 隔离 DSH_HOME 的 profile 配置里注入——daemon 自有领地，零碰用户 `~/.dsh` |
| pi | ✓（MCP 服务器配置；piapp 已有注入守护扩展的先例 `guard.go`） | 启动参数/配置注入，具体机制探针定 |
| zcode | ？未证实 | 能力门控：缺席不挡其他三家 |

daemon 侧实现一个标准 MCP 服务器（Go，stdio + Streamable HTTP 双传输），注册名如 `pocketagent-collab`。**这同时是 daemon 既有 P3 规划"MCP 透传"的第一块砖**——宿主能力以后还能暴露文件/git/终端工具给任意 harness（远期）。

### 6.2 工具面（首版最小集，宁可少勿滥）

| MCP 工具 | 语义 | 实现 |
|---|---|---|
| `agent_delegate {agent_id, task, workspace?, model?, effort?}` | 把一个有界子任务委派给指定 agent：daemon 建子 Thread → 跑 StartTurn → **阻塞返回子任务终稿摘要**（有界，如 ≤8K 字符，准则 3） | 复用 `providerForThread` + Run；子线程是真 Thread（ExternalSession 各自落库） |
| `agent_status {thread_id?}` | 查询某 agent/子任务当前状态（排队/运行/等待审批/终态） | 读 activeTurns/turnQueue |
| MCP progress notification | 子任务过程事件（工具卡/审批请求）推给 coordinator 的工具卡更新 | 子线程 sink 桥接：子帧→MCP progress→父线程 ToolCallBlock 内容更新 |

**不做的**（首版）：`agent_spawn` 异步多点开花、子代理再委派（准则 5，深度锁 1）、模型自选 agent 的智能路由（`agent_id` 由 coordinator 模型显式给，daemon 不猜）。

### 6.3 各家接入路径与红线处理

- **dsh**：隔离 HOME 的 acp profile 加一条 MCP server 配置（daemon 写自己的领地）→ 最干净；
- **codex**：**绝不改写用户 `~/.codex/config.toml`（红线）**。两条路：① 探针验证 app-server 是否支持会话级/进程级 MCP 注册 API（schema 里 `ExternalAgentConfig*` 暗示有配置面），有则走 API；② 没有则首启引导用户自己粘一条配置（用户的手用户自己动，daemon 只给一键复制的 JSON）；
- **pi**：探针定注入机制（配置文件/环境变量/扩展）；
- **zcode**：`GetHostInfo` 能力协商，缺席时 `agent_delegate` 对该 agent_id 如实报"不可用"，不静默。

### 6.4 生命周期语义（复用为主）

| 语义 | 方案 |
|---|---|
| 子任务=真线程 | ExternalSession 各自落库 → daemon 重启/手机断线后**各自重挂载**（R10：轮次与手机连接解耦，子任务在电脑上照跑） |
| 取消级联 | coordinator 轮次取消 → MCP `$/cancelRequest` → 子线程 `CancelTurn`；反向（直接取消子线程）父工具得到 cancelled 结果 |
| 审批路由 | 子线程审批卡**带来源徽章**（"codex@子任务 3"），走既有 SubmitApproval + approval_resume 断线补交；2min 超时=拒绝（现状语义不变） |
| 并发闸 | per-agent 信号量：同 provider 串行（准则 7），跨 provider 并行；排队态经 progress 通知可见 |
| 写冲突隔离 | 默认同工作区 + 委派卡提示；可选 worktree（复用 Git 服务 CreateBranch/Checkout 基建，pi-subagents `worktree:true` 思路）——首版先做提示，worktree 二版 |
| 授权门 | **首次**跨 agent 委派弹手机确认卡（"codex 想把任务委派给 dsh，允许？"），可"本会话不再问"；设置页总开关默认开（准则 4） |

### 6.5 失败语义

- 子任务失败 → `agent_delegate` 返回**错误文本**（含 daemon 结构化 ErrorInfo），coordinator 自行决定重试/换 agent/上报用户——daemon 绝不代拟"成功"；
- MCP 服务器自身崩溃 → coordinator 的工具调用超时报错，**不影响任何已跑的子线程**（它们是独立 Thread）；
- 死锁防护：深度锁 1 + 每 mission 子任务数上限（如 16）+ 子任务时长上限（对齐轮次 30min）。

**审核**：乙级为主（daemon 新模块 + 注入路径）；**零 proto 改动**（事件全部走 coordinator 线程既有 ToolCallBlock/审批帧）。

---

## 7. L3：Mission 编排服务（远期，本次只立蓝图）

- **proto 草案**（甲级，RPC 计数变化需提交信息说明）：`MissionService`：`CreateMission / WatchMission(stream) / CancelMission / ListMissions / GetMission`；
- **Mission 模型**：声明式 lanes/stages（借鉴 pi-subagents `runs.lanes`）：每 stage = {agent_id, task, depends_on, workspace?}；**不用自由脚本**（dsh workflow 思路好但手机端不可渲染不可审，首版声明式可画成依赖图）；
- **执行器**：daemon 侧 Mission Runner——本质是"L2 委派 + DAG 调度"的壳，不含模型决策；
- **持久化**：`missions.json`（对齐 threads.json 风格）+ 事件入 eventstore（全局 seq 天然支持多线程交织回放）；
- **手机协同视图**：TaskCenter 升级——mission 卡片展开子任务树、各 agent 状态徽章（运行/排队/等审批/终态）、聚合审批队列、用量合计；
- **恢复**：daemon 重启 → 各子线程按 ExternalSession 重挂载，mission 状态从 eventstore 终态重建（已在跑的不打断，未开始的等用户或自动续跑——默认停下问人，准则 4）。

---

## 8. 风险登记册

| # | 风险 | 等级 | 缓解 |
|---|---|---|---|
| R1 | MCP 宿主实现的协议面工程量（stdio+HTTP 双传输、progress、cancel） | 中 | Go 生态有 MCP SDK 可评估；也可先只做 stdio（codex/pi/dsh 都认） |
| R2 | codex 会话级 MCP 注入不可行 → 只能引导用户自改 config | 中 | 引导方案体验可接受（一次性）；探针先行验证 API 面 |
| R3 | 同 provider 排队被误读为"卡死" | 中 | 排队态显式可见（progress + 工具卡）；文档如实写"同 agent 串行是事实" |
| R4 | 跨 agent 写同一工作区冲突 | 中 | 首版提示 + 二版 worktree；mission 声明里可指定不同 workspace |
| R5 | 委派工具被滥用（agent 互相踢皮球/烧 token） | 中 | 深度锁 1 + 数量/时长上限 + 首次确认 + 用量合计上报 |
| R6 | 子任务审批在手机后台被淹 | 低 | 来源徽章 + 聚合审批队列（L3）；2min 超时拒绝语义既有 |
| R7 | codex subagent 通知形状随版本漂移 | 低 | 对着本机 schema 实现 + 未知字段透传（主线 A） |
| R8 | L2 与 dsh 接入方案的交付顺序耦合 | 低 | 互相独立：L2 对已注册的任意 provider 生效，dsh 晚到也能进委派池 |

---

## 9. 分阶段实施与审核影响

| 阶段 | 内容 | 分级 | 前置 |
|---|---|---|---|
| P0 探针 | ① codex `subagentStart/Stop` 实测形状（触发 guardian/审查即得）② codex 会话级 MCP 注册 API 可行性 ③ pi MCP 注入机制 ④ MCP Go SDK 选型 | 丙/纯探针 | 无 |
| L1 | 原生子代理透传（codex 优先） | 乙 | P0-① |
| L2a | daemon MCP 服务器（stdio）+ `agent_delegate`/`agent_status` + dsh/pi 接入 | 乙 | P0-③④；建议在 dsh 接入合入后 |
| L2b | codex 接入（API 或引导）+ 授权门 + 取消级联 + 排队可见 | 乙 | P0-② |
| L3 | Mission 服务 + 手机协同视图 | **甲** | L2 稳定运行一个里程碑 |

**DoD（每层）**：L1=真机可见子代理卡（guardian 场景）；L2=真机完成一次"codex 委派 dsh 跑测试并拿回结果"全链路（含断线恢复与审批徽章）；L3=一个两 lane 流水线（codex 实现+zcode 审查）从创建到恢复全程手机可视。全部走 AUDIT 对应级别人工 + `-race` + JVM `--rerun` 全量。

---

## 10. 附录：参考资料

- 本仓库：`codex-schema/ServerNotification.json`（SubagentStart/Stop）、`daemon/internal/server/model_ext.go`（ListMcpServers 现状）、`codex_guardian.go` / `codex_review.go`（审批/第二意见先例）、`turn_queue.go`、`approval_resume.go`
- pi-subagents（本机 `~/.pi/agent/npm/node_modules/pi-subagents/skills/`）：pi-subagents / council-mode 两个 SKILL.md
- dsh 官方文档：reference/subsystems/subagent（`ctx.subagents` 接缝）、reference/subsystems/workflow（`ctx.workflowEngine`）、`packages/subagent/tool-subagent/README.md`
- ACP 规范：agentclientprotocol.com v1/v2 tool-calls（子代理=工具卡的呈现模型）
- 关联文档：`docs/plans/2026-09-20-dsh-integration-plan.md`（dsh 接入——L2 委派池的第一批新成员）
