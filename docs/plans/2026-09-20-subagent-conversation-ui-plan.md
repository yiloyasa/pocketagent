# 子智能体消息与对话查看 —— UI/交互设计与实现方案

> 2026-09-20 · 状态：**设计定稿，未动任何生产代码**
> 需求：codex / pi / zcode / dsh（后续接入）各自派生的**子智能体**，其消息与对话要在 App 中可查看，且与主界面之间切换丝滑流畅。
> 姊妹篇：`2026-09-20-multi-agent-collaboration-plan.md`（L1 透传/L2 委派桥，本文的**数据来源**）、`2026-09-20-dsh-integration-plan.md`（dsh 接入）。
> 一句话方案：**主时间线放"子智能体卡"（折叠一行、展开快照），点击进入全屏子对话页（复用现有气泡/工具卡渲染管线，只读），兄弟子代理在子对话页内横滑切换（HorizontalPager），导航转场对齐现有 NavHost 动效，共享元素过渡点睛。**

---

## 1. 需求与设计原则

### 1.1 三个用户时刻

| 时刻 | 用户想知道 | UI 回答 |
|---|---|---|
| **扫一眼**（主对话里） | "它派了几个小的？都在干活吗？" | 时间线内联卡：图标+任务名+状态点+最后一条消息滚动预览 |
| **看过程**（进子对话） | "这个子智能体具体在说什么/做什么？" | 全屏子对话页：完整消息+工具卡+思考卡，流式实时 |
| **来回切**（多子代理） | "3 个并行的，我逐个看" | 子对话页内**横滑**切换兄弟，不回列表再进 |

### 1.2 设计原则（对齐项目铁律）

1. **不发明新渲染管线**：子对话页复用 `ChatItem`/气泡/`ToolCallCard`/思考卡组件，只是数据源与交互模式不同；
2. **不伪造内容**：原生 harness 只给摘要时（如 codex 的 activity item），子页**如实显示"原生仅提供活动摘要"**，绝不编造对话（红线 1 的 UI 版）；
3. **能力门控**：zcode 暂无子代理事件 → 不显示入口、不误报；未知 agent id 用占位图标（`agentBrandIcon` 返回 null 的既有约定）；
4. **轻量优先**：主时间线只收轻帧（状态+预览+计数），完整子对话按需拉取/按需推流；
5. **只增不改**：proto 帧号续排（14/15…），现有 1–13 帧零触碰。

---

## 2. 前置：daemon 侧统一数据模型（UI 的地基）

> UI 长什么样取决于 daemon 给什么。四家 harness 的原生事件形状各异（见 §7 边界表），daemon 必须先归一化。

### 2.1 proto 扩展草案（甲级：新帧 + 新 RPC，RPC 计数 64→65 需提交信息说明）

```protobuf
// thread.proto —— TurnServerMessage.oneof 追加(只增不改):
  SubAgentUpdate sub_agent = 14;     // 子智能体状态/元数据(轻帧,内联卡数据源)
  SubAgentMessage sub_agent_msg = 15; // 子智能体消息追加(流式,子页打开时的全量流)

message SubAgentUpdate {
  string sub_id = 1;        // 轮内唯一(codex itemId / dsh toolCallId / L2 委派 runId)
  string agent_id = 2;      // 子智能体实际跑在哪个 agent 上(L2 跨 agent 时 ≠ 父)
  string title = 3;         // 任务一句话(daemon 从原生事件提取,提不出则"子任务")
  SubAgentStatus status = 4; // QUEUED / RUNNING / WAITING_APPROVAL / DONE / FAILED / CANCELLED
  string message_preview = 5; // 最后一条消息摘要(≤120 字,内联卡滚动预览用)
  int32  message_count = 6;   // 累计消息数(未读徽章基数)
  Usage  usage = 7;           // 子智能体用量(可得则填)
  string child_thread_id = 8; // L2 委派时的真线程 id(非空=可"作为独立会话打开")
  string raw_event = 9;       // 原生事件原文透传(主线 A,进诊断视图)
}

message SubAgentMessage {
  string sub_id = 1;
  int64  seq = 2;            // 子对话内单调序(分页/重放锚点)
  string role = 3;           // assistant / tool / thinking(与 ChatItem.role 对齐)
  string text = 4;
  string tool_name = 5;      // role=tool 时
  string tool_status = 6;
  string json_args = 7;
}

// ThreadService 追加:
  rpc GetSubAgentHistory(GetSubAgentHistoryRequest) returns (GetSubAgentHistoryResponse);
  // 入参: thread_id, sub_id, before_seq(游标分页,对齐 GetThreadHistory 既有分页风格)
  // 出参: messages(倒序页) + has_more
```

### 2.2 daemon 侧职责

- **归一器**：codex `subagentStart/Stop`+activity item、pi 扩展事件、dsh tool_call(subagent) 帧、L2 委派进度 → 统一 `SubAgentUpdate/SubAgentMessage`；
- **子对话落库**：per-thread 的子代理 transcript（`threads.json` 扩展或旁挂 `subagents/` 文件——实现期定），历史恢复与 daemon 重启后可回看；
- **推送策略**：`SubAgentUpdate` 恒推（轻）；`SubAgentMessage` 恒推但**手机端未开子页时只更新计数不进时间线**（daemon 无需知道手机端状态，简单优先）；
- 全部帧走既有 eventstore 全局 seq——断线重放/事件回放自动正确。

---

## 3. 信息架构与导航

```
home ── chat/{threadId} ── chat/{threadId}/sub/{subId}     ← 新路由(全屏子对话页)
              │                      │
              │ 内联卡(点击)          │ HorizontalPager 左右滑 = 兄弟子智能体
              │ 长按 → 底部弹层快览    │ 头部"返回" → 回主对话(popBackStack)
              │                      └ 审批条:子智能体等待审批时底部出现
 taskcenter(任务中心):运行中子智能体条目 → 深链 chat/{tid}/sub/{sid}
```

**为什么是独立路由而不是 Dialog/BottomSheet**：要支持任务中心深链、通知跳转、进程死重建（NavStack 自动恢复）；BottomSheet 只做**长按快览**的轻交互（§4.3）。

**转场规格**（对齐现状 `MainActivity.kt` NavHost 全局过渡，不另起炉灶）：

| 动效 | 规格 |
|---|---|
| 主对话 → 子对话页 | 复用全局 `fadeIn(220ms) + slideInHorizontally(it/20)`；本路由再加 **shared-axis X** 语义（父页左移出、子页右移入，产生"下钻一层"的方向感） |
| 子对话页 → 返回 | 全局 pop 过渡（`fadeOut(140ms) + slideOutHorizontally(it/20)`） |
| 共享元素 | 卡片上的 **agent 图标 + 任务标题** → 子页头部同名元素做 shared bounds 过渡（Compose `SharedTransitionLayout`；若当前 BOM 不支持则降级：两处图标同尺寸同位置 + fade，视觉连续性仍成立） |
| 预测性返回 | Android 14+ 手势返回时子页缩放预览（`PredictiveBackHandler`），低版本走普通 pop |
| 兄弟切换 | `HorizontalPager`，`beyondBoundsPageCount = 1`（预取相邻子对话首屏），页面滑入时头部状态 chip 交叉淡入 |

---

## 4. 主时间线内联卡（SubAgentCard）

### 4.1 单卡（该轮只有 1 个子智能体）

```
┌────────────────────────────────────────────┐
│ ◉ [codex图] 重构登录模块            运行中  │  ← 状态点(呼吸动画)+agent图标+标题+状态chip
│   "正在读取 AuthService.kt 第 214 行…"      │  ← 最后一条消息预览(流式滚动,≤2行,渐隐遮罩)
│   12 条消息 · 1.2k tok · 38s               │  ← 计数徽章区(有新消息时"12 条 · 3 新")
│                          [ 查看对话 → ]     │  ← 整卡可点(触觉反馈),右侧箭头暗示可下钻
└────────────────────────────────────────────┘
```

视觉规格（贴现有 ToolCallCard 语言）：
- 容器：`Pa.Card` 背景、`RoundedCornerShape(12.dp)`、水平 12dp/垂直 10dp 内边距（与工具卡一致）；
- **与工具卡的区分**：左缘 3dp agent 品牌色竖条 + `agentBrandIcon` 图标（16dp）——工具卡是"动作"，子代理卡是"一个在干活的分身"，身份感优先；
- 状态点：`Queued`=灰 / `Running`=Pa.Accent 呼吸 / `WaitingApproval`=Pa.Red 闪烁 / `Done`=Pa.Green / `Failed`=Pa.Red / `Cancelled`=灰（对齐工具卡三色语义并扩展）；
- 折叠交互：整卡 `pressable(haptic = true)`（PaAnim 既有）点击进子页；**不做二级展开**——快览走长按弹层，全量走子页，层级干净；
- 完成后：卡保留（历史可回看），预览定格为最终摘要，状态点变绿。

### 4.2 组卡（该轮 ≥2 个子智能体，含并行评审/流水线场景）

```
┌────────────────────────────────────────────┐
│ ⊞ 3 个子智能体在协同          1 运行 1 排队 │
│    [codex图]实现登录重构      ◉ 运行中      │
│    [zcode图]审查改动          ○ 排队中      │
│    [dsh 图]跑回归测试         ● 已完成 ✓    │
└────────────────────────────────────────────┘
```

- 每行 = 迷你单卡（图标+标题+状态点），点击直接进**该**子对话页；
- 组卡可整卡折叠为"⊞ 3 个子智能体 · 1 运行 1 排队 1 完成"一行（`animateContentSize` 弹性展开/收起），运行中默认展开，全部终态默认折叠；
- 行间顺序 = 启动顺序（daemon 帧序），不做智能排序（透传原则）。

### 4.3 长按快览（BottomSheet）

长按单卡/组卡行 → ModalBottomSheet（peek 高度 45%）：
- 内容：该子智能体**最近 5 条消息**快照（气泡复用）+ "查看完整对话"按钮（导航进子页）+ 运行中时"取消子任务"操作；
- 定位：不打断主对话浏览的"偷看"；sheet 收起不影响任何状态。

---

## 5. 全屏子对话页（SubAgentScreen）

### 5.1 布局

```
┌────────────────────────────────────────────┐
│ ← [codex图] 重构登录模块      ◉ 运行中  ⋮  │  头部:返回/图标+标题/状态chip/溢出菜单
│   来自主对话 · 第 3/14 轮                    │  来源行(父线程+轮次;兄弟多时显示 ◀ 1/3 ▶)
├────────────────────────────────────────────┤
│  (assistant) 我先看一下现有目录结构…         │
│  [tool ▸ list_files src/auth]  ✓ 完成       │  ← 完整复用 ChatItem 气泡/ToolCallCard/
│  (thinking) 认证逻辑应该拆到…               │     思考卡渲染管线(只读模式)
│  (assistant) 找到了三个耦合点:               │
│  …流式光标▌                                 │
├────────────────────────────────────────────┤
│ [⊥ 等待审批: 运行 npm install  ✓允许 ✗拒绝] │  审批条(该子代理等审批时替换快捷条)
│ 或 [💬 向主智能体追问此子任务…]              │  快捷条(默认态,见 5.3)
└────────────────────────────────────────────┘
```

- **头部**：56dp 高；返回键/pop 手势回主对话；溢出菜单：`取消子任务`(运行中) / `在任务中心查看` / `作为独立会话打开`(仅 `child_thread_id` 非空的 L2 委派) / `导出对话`(复用 ExportThreadHtml 基建,远期)；
- **消息区**：`LazyColumn`；打开时先拉 `GetSubAgentHistory`(末页)再接流式增量(seq 续接,不重不漏)；**自动吸底**仅当用户本就在底部(对齐主对话既有行为)；向上滚动触发游标分页加载更早消息；
- **状态实时性**：`SubAgentUpdate` 到达时头部状态 chip 交叉淡切换；终态帧到达时底部条切换为终局摘要(用量/耗时)。

### 5.2 兄弟子智能体横滑切换（丝滑核心）

- 头部来源行右侧 `◀ 2/5 ▶` 计数 chip + 页面本体 `HorizontalPager`；
- 左滑 = 下一个兄弟、右滑 = 上一个，页面切换时**头部图标/标题/状态随之瞬切**（内容不重渲染等待：预取相邻页首屏历史）；
- 与返回手势不冲突：竖直滑动滚列表、水平滑动切页、边缘左滑/右滑返回（Pager 默认处理内嵌横向手势边界）；
- 轨道记忆：从组卡第 2 行进入 → pager 初始页 = 2，返回主对话后再次进入同卡 → 仍在记忆页（`rememberSaveable`）。

### 5.3 底部快捷条（不做输入框的理由）

子对话是**观察面**不是**对话面**（子智能体的任务是主智能体派的，用户直接插话会破坏父子任务契约）。但用户的真实冲动是"就这事说两句"——所以给：

- **"向主智能体追问此子任务"**：点击展开临时输入框，发送 = 向**父线程**注入一条带引用前缀的排队消息（`[关于子任务"重构登录模块"] <用户输入>`，复用 P1-4 turn_queue 插话语义 + Queued 回执徽章），然后自动跳回主对话时间线定位到该消息；
- L2 委派且 `child_thread_id` 非空时，额外给"打开独立会话"入口（那是真线程，拥有完整输入能力）。

### 5.4 审批条（子智能体的审批）

- 该子智能体 `WAITING_APPROVAL` 时，底部快捷条替换为审批条：动作描述(复用 `ApprovalRequest.action` 渲染) + **"来自子智能体"来源徽章** + 允许/拒绝；
- 应答走既有 `SubmitApproval`（thread_id=父线程, request_id 不变——daemon 归一器负责路由到子）；2min 超时=拒绝（既有语义）；
- 主时间线的审批卡同步显示来源徽章，卡上加"查看子对话"跳转链。

---

## 6. 状态管理与数据流（App 侧）

### 6.1 TurnSession 扩展（不动主时间线状态机）

```
TurnSession(既有:主时间线 items/流式/审批)
  └─ subAgents: Map<subId, SubAgentState>(新增)
        ├─ meta: SubAgentUpdate 最新值(卡片渲染源)
        ├─ unread: Int(未读计数;主时间线视角)
        └─ page: SubAgentPageState(仅子页打开时激活)
              ├─ items: List<ChatItem>(复用模型!)
              ├─ streamingCursor: seq
              └─ pagingCursor: before_seq
```

- `sub_agent` 帧 → 更新 meta + unread++（卡片徽章/TaskCenter 通知）；**不进主时间线 items**；
- `sub_agent_msg` 帧 → 仅当对应 page 激活时追加渲染；否则丢弃（历史已落库，进页时 `GetSubAgentHistory` 拉回——**流式与回放统一走 seq，逻辑只有一条**）；
- 断线重连：帧走全局 seq，`ResumeFrom` 自动补——子页打开期间断网重连不丢帧（既有 eventstore 机制白捡）。

### 6.2 历史恢复（HistoryMessageMapper）

- daemon 落库的子代理记录 → mapper 新增 `sub_agent` item 类型 → 主时间线恢复出内联卡（终态定格）；点击子页 → `GetSubAgentHistory` 拉全量；
- **诚实降级**：原生只提供活动摘要（如 codex 某些 activity item 无正文）时，子页显示"该 agent 原生仅提供活动摘要"+ 摘要时间线，不造对话。

### 6.3 TaskCenter 联动

- 运行中子智能体 = TaskCenter 新任务种类（kind=SUBAGENT，归属父线程条目下）；完成通知点击 → 深链子页；条目复用既有 unread/finish 生命周期。

---

## 7. 四家 agent 的数据可得性与门控（诚实边界表）

| agent | 子智能体消息可得性 | 子对话页呈现 | 门控 |
|---|---|---|---|
| **codex** | `subagentStart/Stop` + `SubAgentActivityThreadItem`（活动条目；正文可得性待 0.149.1 实测） | 活动时间线；正文可得则完整对话，否则摘要+原文 | 帧不到=无卡 |
| **pi** | pi-subagents 扩展事件（run 状态/输出引用；piapp 现未解析→L1 范畴） | 同上，按事件粒度如实 | 无扩展=无卡 |
| **zcode** | 未证实存在子代理事件 | —（不显示入口，不误报） | 能力协商 |
| **dsh** | 原生 subagent 走 tool_call content（过程可见）；**L2 委派的子任务是真线程** | tool 内容时间线；L2 下=完整对话+可开独立会话 | 接入后生效 |
| L2 跨 agent 委派（任意父） | daemon 全程知情（自己建的子线程） | **全量完整对话**（基准体验） | L2 落地后 |

> 这张表决定验收口径：**L2 委派路径是"完整对话"的基准体验；原生透传路径按各家实际能力如实呈现，摘要就是摘要。**

---

## 8. 动效与性能规格汇总（"丝滑"的工程定义）

| 项 | 规格 | 复用/新增 |
|---|---|---|
| 页面转场 | 全局 220/140ms fade+slide（现状）+ shared-axis 方向感 | 复用 NavHost 全局 |
| 共享元素 | 图标+标题 shared bounds；不可用则同位 fade 降级 | 新增（降级路径保底） |
| 卡片按压 | `pressable(haptic)`（现成） | 复用 PaAnim |
| 卡展开/组卡折叠 | `animateContentSize`(spring, dampingRatio=0.8) | 新增（对齐 PaAnim spring 参数） |
| 状态点呼吸 | 运行中 1.2s 周期 alpha 0.4↔1.0 | 新增（对齐工具卡运行色） |
| 兄弟切换 | HorizontalPager + 预取 1 页 + 头部 chip 交叉淡入 180ms | 新增 |
| 列表性能 | LazyColumn + 子页历史分页（页大小 50）+ 流式增量 diff 最小化 | 分页新增、渲染复用 |
| 预测性返回 | 14+ 缩放预览，低版本普通 pop | 新增（渐进增强） |
| 触觉 | 卡点击/页切换沿 edge(轻)/审批到达(中) | 复用 HapticFeedback |

---

## 9. 实施切片与审核影响

| 切片 | 内容 | 分级 | 依赖 |
|---|---|---|---|
| S0 探针 | codex subagent 事件正文可得性实测（0.149.1）→ 定 §7 表真值 | 纯探针 | 无 |
| S1 协议 | proto 新帧 14/15 + `GetSubAgentHistory` + 双端契约同步（64→65 RPC，提交信息说明） | **甲** | S0 |
| S2 daemon | 归一器（codex 先行）+ 子对话落库 + 轻帧/消息帧推送 | 乙 | S1 |
| S3 App·内联卡 | SubAgentCard/组卡 + TurnSession.subAgents + 历史恢复 | 乙 | S2 |
| S4 App·子对话页 | SubAgentScreen 路由 + pager + 审批条 + 快捷条 + 转场/共享元素 | 乙 | S3 |
| S5 联动 | TaskCenter 深链 + L2 child_thread_id 独立会话入口 | 乙 | S4、L2 |

**验收 DoD**（真机）：
1. codex 触发一次子代理（guardian/审查）→ 主时间线出卡 → 点进子页看到活动/消息流 → 横滑无卡顿（帧率 ≥50fps 目测/AGI 记录）；
2. 断网重连：子页打开时断网 10s 恢复 → 帧序补齐不重不漏；
3. 杀进程重建：NavStack 恢复子页 + 历史回放正确；
4. zcode 会话无子代理入口（不误报）；未知 sub 帧字段进诊断视图不崩；
5. 审批：子代理审批卡在主时间线与子页双入口可答，2min 超时拒绝；
6. 回归：131 用例受影响模块全量 + JVM 全量 `--rerun` 0 失败；四 agent 零交叉污染不回退。

---

## 10. 附录：与现有代码的对接点清单

| 设计元素 | 对接点 |
|---|---|
| agent 图标 | `ui/AgentBrand.kt`（**dsh 图标已在库**）；未知 id→占位 |
| 卡片视觉/按压/状态色 | `ChatScreen.kt` `ToolCallCard`(L633) + `Pa` 色板 + `anim/PaAnim.kt pressable` |
| 时间线项模型 | `ChatItem`(L193) 直接复用（子页 items 同构） |
| 导航与转场 | `MainActivity.kt` NavHost(L159-166) 全局过渡；新路由 `chat/{threadId}/sub/{subId}` |
| 插话队列 | `TurnDelivery.kt` 交付状态机 + turn_queue（快捷条发送） |
| 审批链路 | `SubmitApproval` + `approval_resume`（provenance 徽章新增） |
| 断线重放 | eventstore 全局 seq（子帧天然继承） |
| 任务中心 | `component/TaskCenter.kt`（kind 扩展） |
| 历史映射 | `HistoryMessageMapper.kt`（新增 sub_agent item） |
