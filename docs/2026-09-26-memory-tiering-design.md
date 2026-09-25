# Memory 分层落地设计（2026-09-26 调研设计稿 · RD 代理 · 丁类文档）

> 性质：**只调研、只设计，不含实现代码**。对应 spec §6.3（`docs/superpowers/specs/2026-09-22-runtime-control-plane-design.md:427-497`）。
> 外部结论全部附出处；本地现状全部给 `文件:行号`。行号以 2026-09-26 工作区为准。

---

## 0. 一句话结论

PocketAgent 的 memory **契约与持久化已齐**（四层 kind、Compact+Covered 记账、checkpoint 落盘+重启 hydrate、每轮写入链），缺的是三个**执行点**：① 没有任何生产代码调用 `Compact`（收敛不发生）；② `summary` 槽无人写（`structuralDigest` 是结构摘要，语义摘要需要模型，runtime 包刻意不调模型）；③ 写入与资源预算（§6.5）不联动（超预算无动作）。落地路线因此是"补执行点"而不是"补存储"：P1 轮末自动收敛（结构摘要，零模型调用）+ 预算接线；P2 可插拔语义摘要 + 为 daemon 家（OpenAI 兼容）做上下文组装读者；P3 闲时后台整理 + artifact 独立登记。**不引入向量库，不让 daemon 重写轮次循环。**

---

## 1. 外部调研：主流实现的分层、摘要时机、检索与预算

### 1.1 Mem0（两层：上下文即工作记忆 + 抽取事实库）

- **分层**：working/short-term = LLM 上下文窗口本身；long-term = 从对话中**抽取的事实**存入向量库（约 19 种后端可选），可选再叠一层 graph memory（关系型检索）。按 user / session / agent 三个粒度隔离。（出处：[Towards AI — End-to-End Guide to Memory Architectures](https://pub.towardsai.net)、[arXiv — Governed Memory OS](https://arxiv.org)）
- **何时摘要/收敛**：`add()` 时两段管线——**extraction**（LLM 从最近消息抽候选事实）→ **update**（第二个 LLM 调用把候选与存量比对，决定 ADD / UPDATE / DELETE / NOOP 四种操作）。即收敛发生在**写入点**而非定时器。（出处：论文 [Mem0: Building Production-Ready AI Agents with Scalable Long-Term Memory, arXiv:2504.19413](https://arxiv.org/abs/2504.19413)；[Mem0 docs — memory-operations/add](https://docs.mem0.ai/core-concepts/memory-operations/add)）
- **怎么检索**：`search(query)` 向量相似度 + scope 过滤，返回 top-k **事实**（不是原始历史）。（出处：[Mem0 docs](https://docs.mem0.ai/open-source/node-quickstart)）
- **token 预算**：靠"存事实不存全文"压缩注入量——检索回的是紧凑事实而非完整 transcript，这是它对全上下文方案的主要 token 优化；论文报告比 OpenAI 方案相对准确率 +26%、延迟与 token 更低。（出处：[arXiv:2504.19413](https://arxiv.org/abs/2504.19413)）

### 1.2 Letta / MemGPT（OS 式三层 + 自编辑 + 后台整理）

- **分层**：① **core memory**——常驻上下文窗口内、**有字符上限**的可编辑记忆块（persona / human / 自定义），agent 只能通过工具调用（`core_memory_append` / `core_memory_replace`）改写；② **recall storage**——全部对话历史（含被逐出的），可检索；③ **archival memory**——窗口外长期存储（向量库），agent 按需"换页"进出，类比磁盘/虚存。（出处：[MemGPT 论文, arXiv:2310.08560](https://arxiv.org/abs/2310.08560)、[MemGPT 虚拟上下文管理解读](https://www.leoniemonigatti.com/blog/memgpt.html)、[Letta memory blocks 概览](https://blog.stackademic.com/letta-platform-for-stateful-llm-agents-a83b58a1c926)）
- **何时摘要**：**内存压力驱动**——上下文写满时，系统 flush 旧内容并把关键信息写进 core memory；对更早的对话做**摘要压缩**（recursive summarization），使可用上下文实际上无限延伸。（出处：[arXiv:2310.08560](https://arxiv.org/abs/2310.08560)、[20 个 agent harness 调研：15 个自动 compact](https://pub.towardsai.net/we-read-20-agent-harnesses-15-compact-your-context-automatically-10-let-you-set-when-2b36f2d57bf6)）
- **怎么检索**：agent 通过工具调用自主检索 recall/archival（语义搜索）。（出处：[arXiv:2310.08560](https://arxiv.org/abs/2310.08560)）
- **token 预算**：core memory 块**容量固定可配置**，块写满是显式失败模式——agent 被迫自己取舍；Letta 另有 **sleeptime（后台）agent** 把记忆整理算力挪到轮次之外。（出处：[Letta 记忆模型深读](https://medium.com/@piyush.jhamb4u/stateful-ai-agents-a-deep-dive-into-letta-memgpt-memory-models-a2ffc01a7ea1)、[Context Compression for Long-Horizon Agents](https://www.preprints.org/manuscript/202607.0924)）

### 1.3 OpenAI ChatGPT memory（显式清单 + 自动历史引用，四层上下文组装）

- **分层**：两个机制——① **saved memories**：显式保存、用户可查看编辑的事实清单；② **reference chat history**：自动检索/摘要相关旧对话。（出处：[OpenAI Help Center — Memory in ChatGPT](https://help.openai.com)）
- **何时摘要**：saved memories 在对话中即时产生；chat history 引用是**按需摘要**相关旧会话，不逐条搬运。（出处：[Memx — Reference Chat History vs Saved Memories](https://memx.app)、[Coursiv 解读](https://coursiv.app)）
- **怎么注入（=检索）**：逆向工程实测——开启 "Reference saved memories" 后，保存的记忆**带时间戳**注入系统提示的 "Model Set Context" 段；传给模型的上下文由四层组成：**会话元数据 → 用户记忆（saved memories）→ 近期对话摘要（chat history 引用）→ 当前会话消息**。（出处：[embracethered.com 实测](https://embracethered.com)、[LLMRefs 逆向](https://llmrefs.org)）
- **token 预算**：无公开数值；形态上是"清单条目数 + 摘要长度"两个隐式预算位，用户可整体删除。（出处：[OpenAI Help Center](https://help.openai.com)）

### 1.4 LangGraph / LangMem（三类型 × profile/collection × 两条形成时机）

- **分层（按记忆类型）**：**semantic**（事实：profile=单文档原地更新 / collection=增量集合+合并）、**episodic**（结构化经验：观察-思考-行动-结果，few-shot 复用）、**procedural**（规则与 prompt，可用轨迹反馈优化）。（出处：[LangChain 官方 Memory 概念文档](https://docs.langchain.com/oss/python/concepts/memory)、[LangMem 概念指南](https://langchain-ai.github.io/langmem/concepts/conceptual_guide/)）
- **何时摘要/形成**：**两条时机明码标价**——conscious（热路径，轮内即时，加延迟）vs subconscious（**后台/轮间**，不加延迟、召回更高、可做模式分析与摘要）。LangMem 推荐：关键更新走热路径，其余走后台。（出处：[LangMem 概念指南](https://langchain-ai.github.io/langmem/concepts/conceptual_guide/)）
- **怎么检索**：namespace 层级（org → user → context）存入 `BaseStore`；检索 = 语义相似度 + 直接按键 + 元数据过滤；**推荐把相似度与重要性、强度（recency/frequency）组合排序**，不做纯相似度。（出处：同上）
- **token 预算**：文档未给数值；约束体现在"过度抽取伤检索精度、抽取不足伤召回"的平衡指引上。（出处：同上）

### 1.5 横向小结（对 PocketAgent 的可迁移结论）

| 维度 | Mem0 | Letta/MemGPT | OpenAI | LangGraph/LangMem | PocketAgent 可迁移 |
|---|---|---|---|---|---|
| 分层 | 上下文=working；抽取事实=长期 | core（限容）/ recall / archival | saved 清单 + 历史引用 | semantic/episodic/procedural | 已有四层 kind，分层不是缺口 |
| 何时收敛 | **写入点**（add 管线） | **压力驱动**（写满即摘要） | 按需摘要旧会话 | **热路径 vs 后台**两档 | 轮末钩子（写入点）+ 软预算触发（压力），后台归 P3 |
| 检索 | top-k 事实 | agent 工具自检索 | 系统提示注入清单+摘要 | 相似度+重要性+强度 | ReadWindow 最近窗口（已有）+ P2 为 daemon 家做四层组装 |
| 预算 | 存事实不存全文 | 块字符上限显式失败 | 隐式（清单+摘要长度） | 抽取平衡指引 | ResourceLedger 软/硬限（已有机制，未与 memory 接线） |

三条最重要的外部经验：**① 收敛要有明确触发器（写入点或压力阈值），"等有空再做"的系统永远不做**；**② 摘要是压缩手段，必须带覆盖范围记账**（MemGPT 的 summarization、Mem0 的 UPDATE/DELETE 都改写历史，PocketAgent 的 `Covered` 已是同构设计）；**③ 预算约束要落在"常驻层"的容量上**（Letta 块上限），而不是靠检索时裁剪。

---

## 2. 本地现状（file:line）

### 2.1 已有（不是缺口）

| 能力 | 位置 |
|---|---|
| 四层 MemoryKind（working/conversation/summary/artifact） | `daemon/internal/runtime/memory.go:18-35` |
| MemoryKey（agent/thread/kind/scope）与 fail-closed 校验 | `memory.go:47-73` |
| MemorySnapshot：Covered 覆盖范围 + Digest 摘要正文 + Version（"不静默丢数据"不变量） | `memory.go:126-141`；完整性判据 `backend.go:1121-1144`（`CheckMemoryIntegrity`） |
| MemoryManager 六方法（Load/Append/ReadWindow/Compact/Checkpoint/Restore） | `memory.go:179-192` |
| Compact：KeepRecentItems/TargetBytes 压缩 + Covered 记账 + 结构摘要拼接 | `backend.go:357-413`；`structuralDigest` `backend.go:415-443` |
| checkpoint 落盘（临时文件→fsync→原子 rename→信封校验）与重启 hydrate（首次触碰即装载、高水位只升不降） | `checkpoint.go:77-128`、`backend.go:239-262`、`backend.go:477-527` |
| 每轮写入链：daemon 档写 conversation 原文；hybrid 档写 artifact 槽轻量索引（turn_id/块形/字节/形状指纹）；native 档不写；随后每轮 Checkpoint | `daemon/internal/server/turn_memory.go:74-117`（分档判据 `:32-52`），调用点在轮次心脏 `daemon/internal/server/thread.go:1345`（user 侧）、`:1398`、`:1431`（assistant 两处落库点） |
| 记忆清单与按会话清除（手机端内存管理页数据源） | `daemon/internal/runtime/memory_inventory.go:66-163`（List）、`:179-242`（Clear） |
| 观测面：AgentMemoryUsage.kinds 分层计数；ListThreadMemories/ClearThreadMemory RPC | `proto/v1/runtime.proto:338`；`daemon/internal/gen/pocketagent/v1/runtime_grpc.pb.go:32-33` |
| 原生压缩手动入口（pi/zcode/codex 走各自原生命令，server 类型断言路由） | `daemon/internal/server/thread.go:561-655` |

### 2.2 缺口（"主动收敛没有执行点"的精确分解）

1. **Compact 无生产调用点**：全仓 grep（排除 `_test` 与 runtime 包自身）对 `Memory().Compact` 零命中——`Compact` 只活在契约测试里。轮次心脏三个写入点之后没有任何水位检查。
2. **summary 槽无人写**：`structuralDigest` 是"条数+seq 范围+角色分布"的**结构摘要**，且注释明说"语义摘要要模型生成，本包不调模型"（`backend.go:415-419`）。因此 hybrid 家的 daemon 侧 `Digest` 永远只是结构行，`kinds["summary"]` 计数恒为 0（proto 字段 `runtime.proto:338` 有位无人用）。
3. **working 槽无人用**：轮次中状态（部分回复/当前审批）活在 `runTurnOnce` 的栈上，轮末随落库进 conversation/artifact，spec §6.3 的"轮次结束后归档或压缩"在现架构里由"轮末才写入"天然满足——**这层大概率不需要独立实现**（见 §5 不做清单）。
4. **artifact 槽被借用**：hybrid 家的"轮次索引"写进 artifact 槽并靠 `Meta["entry"]=turn_index` 自我描述（`turn_memory.go:45-62`，注释预留了迁移标记）；真正的 §6.3 ArtifactMemory 条目（工具输出大块、子 agent 产出，带 size/hash/retention）没有独立入库路径。
5. **ReadWindow 无生产读者**：daemon 不重写轮次循环（README §1："原生 agent = 干活者……daemon 不重写"），四家原生 agent 的上下文由原生侧自己管；`ReadWindow` 的"最近窗口"语义（`backend.go:333-336` 注释）目前只被契约测试消费。唯一规划中的读者是 OpenAI 兼容通道的请求前组装（spec §7:598-610 点名）。
6. **预算不联动**：ResourceLedger（`backend.go:822-941`）有软/硬限与压力分类，但 `recordTurnMemory` 写入既不 `Charge` 也不读压力——达到 `disk_conversation_bytes` 软上限时没有任何压缩动作（spec §6.5:565 的"软上限优先压缩落盘"无执行者）。
7. **无后台执行者**：daemon 无闲时 tick 去合并多轮 Digest（对照 LangMem subconscious 档 / Letta sleeptime）。

---

## 3. 关键设计决策（对照外部经验取舍）

**D1 收敛触发器 = 轮末钩子（写入点）为主，软预算为辅；后台整理推迟到 P3。**
采 LangMem "subconscious" 结论（后台不加延迟但建设成本高）与 Mem0 "写入点收敛"结论（触达确定）。P1 把收敛挂在 `runTurnOnce` 落库点之后（`thread.go:1398/1431` 下游），条件 = 既有阈值 + ResourceLedger 软限信号。理由：单锁 Backend（`backend.go:30`）按"单 daemon、少量 thread"设计，轮末串行收敛的代价可控；后台并发整理会撞上 ckptMu/b.mu 的锁纪律（`memory_inventory.go:184-186` 已有自死锁前科注释），P3 再议。

**D2 语义摘要 = 可插拔接口，runtime 包仍不 import 模型。**
`backend.go:24-27` 的边界（"不生成语义摘要（那要有模型）"）是对的，保留。在 runtime 包定义 `Summarizer` 接口（输入被压缩项+旧 Digest，输出摘要文本），server 层注入实现（复用 OpenAI 兼容/codex 的一次性调用）。**失败回退结构摘要，绝不阻断轮次**（`turn_memory.go:51-52` 既有纪律）。对照 MemGPT：摘要是"压缩失败安全的降级链"的顶格版，结构摘要是底格版——两格都要在。

**D3 hybrid 家收敛 daemon 侧索引，原生正文不动。**
MemoryHybrid 的合同是"原生存正文，daemon 存摘要与索引"（`descriptor.go:154-155`）。daemon 侧 Compact 只作用于自己槽里的索引项（`Covered` 记账）；原生压缩继续走既有手动入口（`thread.go:561-655`）。daemon 绝不重建原生正文摘要——那会构成"同一事实两个副本分叉"（`turn_memory.go:153-158` 注释明令禁止）。

**D4 daemon 家（OpenAI 兼容/echo）才是 ConversationMemory 的完整消费者，P2 给它做 spec §7 的四层上下文组装。**
spec §7:598-610 已点名："daemon 必须保存 conversation memory，并在每次请求前按预算生成上下文：system context → summary → recent messages → relevant artifacts → current user input"。这正是 OpenAI 官方 ChatGPT 的四层组装（§1.3），也是 `ReadWindow` 的第一个真实读者。原生四家不消费 daemon 上下文（不重写轮次循环红线）。

**D5 预算语义照搬 ResourceLedger 既有判定：软限=收敛，硬限=拒可选写。**
`backend.go:831-840` 已有"硬上限只拒绝可选资源，必需资源（用户消息）允许越限"的纪律——memory 写入直接复用：用户/assistant 原文=必需，artifact 索引=可选。不新发明预算机制。

---

## 4. 分阶段落地设计

### P1 —— 轮末自动收敛 + 预算接线（最小可用，零模型调用，零协议变更）

**做什么**：让 `Compact` 第一次在生产路径上被调用；让预算第一次对 memory 说话。

- **触发器**：`server/thread.go` 的 `runTurnOnce` 在两处 assistant 落库点（`:1398`、`:1431`）之后调用新函数 `afterTurnMemoryHousekeeping(provider, t)`：
  - 读该 (agent, thread) 各槽水位（`Load` 快照的 Items/Bytes）；
  - 超阈值（初始常量表：conversation 槽条数 > 200 或字节 > 2MB；artifact 索引槽条数 > 400，全部可被后续配置覆盖，量级对齐 spec §6.5 示例的 32MB/256MB 的保守零头）→ 调 `Memory().Compact(CompactPolicy{KeepRecentItems: 50, TargetBytes: 1MB, Reason: "轮末收敛"})` → 紧随 `Checkpoint`（与 `turn_memory.go:111-116` 同款失败只记日志）；
  - ResourceLedger 软限信号（`Usage` 返回 `OverSoft`）同样触发一次 Compact。
- **预算接线**：`turn_memory.go` 的两处 `Append` 成功后向 `Resources().Acquire/Charge`（daemon 档记 `disk_conversation_bytes`，hybrid 档记 `disk_artifact_bytes`；租约按 thread 生命周期 Acquire 一次，轮内 Charge）。诊断页 Observe 从此能看到记忆的真实占用压力。
- **改动文件**：`daemon/internal/server/thread.go`（钩子函数 + 两处调用点）、`daemon/internal/server/turn_memory.go`（预算接线）、`daemon/internal/server/` 新增 `memory_housekeeping.go`（阈值常量表 + 钩子实现，避免 runTurnOnce 再膨胀）、`daemon/internal/runtime/` 零改动（契约已够用）。
- **新增 RPC**：无（`kinds["summary"]` 计数会自然从 0 变为非 0，手机端零改动可见）。
- **测试策略**：
  - runtime 包：现有 `CheckMemoryIntegrity`（`backend.go:1121`）守住 Compact 不丢数据（已绿，不改）；
  - server 包：fake provider 连跑 N 轮 → 断言 conversation 槽出现 `Covered` 非空 + `Digest` 非空 + summary 计数 > 0 + 轮次返回不受影响（收敛失败只留日志）；
  - 预算：断言软限触发 Compact、硬限拒掉 artifact 索引但保留用户原文（对照 `backend.go:831-840` 语义）。

### P2 —— 可插拔语义摘要 + 第一个检索读者（daemon 家上下文组装）

- **Summarizer 接口**（runtime 新文件 `summarizer.go`）：`Summarize(ctx, covered []MemoryItem, prevDigest string) (string, error)`；server 层注入实现（走 OpenAI 兼容通道一次性调用；超时与重试自管）。Compact 在接口非 nil 且调用成功时用语义摘要替换 `structuralDigest` 产物，失败/未注入回退结构摘要。
- **摘要元数据**：spec §6.3:474-481 要求"覆盖 sequence 范围 / 生成时间 / 生成 agent / 摘要版本"——`MemorySnapshot.Covered`（范围）、`Version`（版本）、`At`（时间）已覆盖三项，生成 agent 即 MemoryKey.AgentID；无需改契约，只在 Digest 文本头部带机器可读首行（`summary-v1 seq=A..B by=agent`）保持可审计。
- **检索读者①（spec §7 落地）**：`openai_compat` 的请求前组装 `system context → summary（Digest）→ recent messages（ReadWindow 最近窗口）→ relevant artifacts（索引槽最近 K 条）→ current user input`；组装函数放 `daemon/internal/server/`（或 openai_compat 的 Run 前），预算 = MemoryLimit 的 MaxBytes。
- **检索读者②**：`ListThreadMemories`（`memory_inventory.go:66`）扩展 per-kind 列（手机端内存管理页显示"摘要占多少、索引占多少"）；proto 复用 `AgentMemoryUsage.kinds`，无新字段。
- **改动文件**：`runtime/summarizer.go`（新）、`runtime/backend.go`（Compact 内摘要产物选择，≤20 行）、`server/`（注入点 + openai_compat 组装函数）、`proto`（如做 per-kind 清单列，一个小响应字段；双副本同步纪律照 README §16）。
- **新增 RPC**：无强制；可选给 `ListThreadMemories` 响应加 per-kind 明细。
- **测试策略**：Summarizer fake（成功/失败/超时三路 → 断言回退与 Digest 可审计首行）；组装 golden 测试（四层顺序、预算截断）；完整性判据回归（语义摘要替换后 `CheckMemoryIntegrity` 仍绿）。

### P3 —— 闲时后台整理 + artifact 独立登记

- **后台整理**：daemon 低频 tick（如每 30 分钟、且仅空闲时），合并多轮 `Digest`（Mem0 update 阶段的 UPDATE/DELETE 语义在 daemon 侧的对应物 = "重写摘要 + 扩展 Covered"，绝不删 conversation 原文）；全部走既有 Append/Compact/Checkpoint 写路径，靠 hydrate 幂等，重启安全。
- **artifact 独立登记**：工具大输出（超过阈值的 tool_result）与子 agent 产出按 §6.3:483-497 字段（artifact_id/thread/turn/agent/kind/size/hash/retention）入 artifact 槽，`Meta["entry"]="artifact"` 与既有 `turn_index` 并列（迁移标记已预留，`turn_memory.go:45-49`）；retention 与 ResourceLedger `disk_artifact_bytes` 联动，达硬限拒新增可选 artifact。
- **测试策略**：后台整理的并发用例（整理与轮次写入交错 → 完整性判据仍绿）；artifact 生命周期（登记→越限→按 retention 淘汰只动可选项）。

**每阶段验收底线**：`go test ./...` 全绿；手机端内存管理页计数与盘上 checkpoint 载荷一致（`memory_inventory.go` 的双口径不变）；没有任何路径引入"静默删除"（完整性判据为证）。

---

## 5. 「不做」清单与理由

| 不做 | 理由 |
|---|---|
| **向量库 / embedding 检索**（Mem0/Letta 形态） | 规模不成立：单 daemon、少量 thread，ReadWindow 最近窗口 + Digest 已覆盖可预见读者；违反零第三方运行时依赖红线（README §0.5/§2）；检索质量要靠 embedding 模型，又与 D2 的"模型可插拔"叠加成两级不可复现 |
| **daemon 重写轮次循环 / 给原生四家注入 daemon 上下文** | README §1 铁律"daemon 不重写轮次循环"；原生家正文在原生侧，注入=双写分叉（`turn_memory.go:153-158`）+ source-mismatch 同族的边界违约 |
| **working 槽独立落盘/实现** | 轮次中状态天然活在 `runTurnOnce` 栈上、轮末落库即"归档"；为它建槽只会产生无人读的第二份副本（同 §2.2-3）。spec §6.3 的"不能无限累积"由"轮末即弃"天然满足 |
| **跨 thread / agent 级长期记忆**（user-level memory，Mem0 的 user scope） | 无消费者：手机端以会话为粒度管理记忆（`memory_inventory.go:13-17`）；跨会话偏好属产品决策，先留 `Scope` 维度（`memory.go:51-53`）不建执行点 |
| **自动删除用户原文**（Mem0 的 DELETE 操作作用于对话原文） | 规格 §6.1/§9 禁止把会话历史当缓存删；DELETE 语义只允许作用于 daemon 自己的摘要重写（P3），原文只能用户手动清（ClearThreadMemory 已有） |
| **摘要可编辑（手机端改写 Digest）** | 检索读者尚未稳定（P2 才有第一个），编辑界面在等一个稳定的语义；先只读展示 |
| **后台整理提前到 P1/P2** | 单锁 Backend 下并发整理的锁纪律风险（自死锁前科）与收益不成比例；轮末收敛已消除"永不收敛"的主要风险 |
| **为 memory 新起独立持久化通道** | checkpoint 信封+载荷即账本（`memory_inventory.go:19-30` 已论证"载荷就在盘上，读它永远不撒谎"），sidecar 索引是同一事实第二副本 |

---

## 6. 风险与开放问题

1. **轮末收敛的时延**：Compact 在锁内做裁剪、锁外 fsync（`backend.go:461-469`），大会话首收敛可能数十毫秒——挂在轮次交付之后不阻塞回复可见，但 P1 需实测最坏轮间隔。
2. **阈值定值缺压力数据**：spec §6.5:538 自认"初始基线，上线前要压力测试校准"——P1 常量表要标注"待校准"，并保证可配置覆盖。
3. **语义摘要的内容风险**（P2）：注入的摘要来自第三方模型，可能夹带原文没有的断言——Digest 首行的可审计元数据 + 手机端展示"摘要为机器生成"需在 P2 验收里显式列出。
4. **开放问题**：hybrid 家的原生压缩事件（codex contextCompaction 等）发生后，daemon 侧索引是否需要对齐重建（当前设计：不需要，索引只描述"daemon 见过的轮次"）——留给 P3 取证。

---

## 附：外部出处汇总

- Mem0：[arXiv:2504.19413](https://arxiv.org/abs/2504.19413)（两段管线、ADD/UPDATE/DELETE/NOOP、+26%）；[docs.mem0.ai add 管线](https://docs.mem0.ai/core-concepts/memory-operations/add)；[docs.mem0.ai quickstart（search/scope）](https://docs.mem0.ai/open-source/node-quickstart)
- Letta/MemGPT：[arXiv:2310.08560](https://arxiv.org/abs/2310.08560)（core/recall/archival、压力驱动摘要）；[虚拟上下文管理解读](https://www.leoniemonigatti.com/blog/memgpt.html)；[memory blocks 限容](https://medium.com/@piyush.jhamb4u/stateful-ai-agents-a-deep-dive-into-letta-memgpt-memory-models-a2ffc01a7ea1)、[Letta 平台概览](https://blog.stackademic.com/letta-platform-for-stateful-llm-agents-a83b58a1c926)；[sleeptime 后台整理](https://www.preprints.org/manuscript/202607.0924)；[20 harness compact 调研](https://pub.towardsai.net/we-read-20-agent-harnesses-15-compact-your-context-automatically-10-let-you-set-when-2b36f2d57bf6)
- OpenAI：[Help Center — Memory in ChatGPT](https://help.openai.com)；[embracethered 时间戳注入实测](https://embracethered.com)；[LLMRefs 四层上下文逆向](https://llmrefs.org)
- LangGraph/LangMem：[LangChain Memory 概念](https://docs.langchain.com/oss/python/concepts/memory)；[LangMem 概念指南（profile/collection、hot path vs background、检索排序）](https://langchain-ai.github.io/langmem/concepts/conceptual_guide/)；[LangMem SDK 发布文](https://www.langchain.com/blog/langmem-sdk-launch)
