# 子智能体对话 UI —— S1 设计修正（基于 T-L1 S0 真值）

> 2026-09-21 · 状态：设计修正提案，**待用户确认后实施**（修订 2026-09-20-subagent-conversation-ui-plan.md 的 S1/S2）
> 触发：T-L1 S0 探针真值（docs/verify/evidence/codex-subagent-s0.md）推翻了原方案的两个前提。

---

## 1. S0 真值（T-L1 实测，4 轮）

1. **审批审查子代理不发 subAgentActivity/collabAgentToolCall**——走 `item/autoApprovalReview/started|completed` 通知族（GuardianApprovalReview 载荷：reviewId/targetItemId/review{status,riskLevel,rationale}/action/decisionSource），daemon 的 guardian 记录 + native 透传**已经在处理**（非静默）。
2. **子代理线程是真线程**：thread/started 应答带 `parentThreadId`（子代理线程非空）→ 子对话全量可走既有 `thread/read` 捷径，**不需要新 RPC**。
3. `subAgentActivity/collabAgentToolCall` 两类 item 属 **collab 多代理模式专属**，S0 四轮 0 条；解析代码已入库（codex_subagents.go，12 测试）待 collab 模式证伪/证实。

## 2. 对原方案 S1/S2 的修正

| 原方案 | 修正 | 理由 |
|---|---|---|
| S1 proto：新帧 sub_agent/sub_agent_msg + GetSubAgentHistory RPC（64→65） | **大幅收缩甚至归零**：codex 子代理=真线程，用既有 ThreadService（thread/read/历史/轮次）即得全量对话；L2 委派的子任务同样是真线程 | 原前提"子代理对话需要新通道"被 parentThreadId 捷径替代；新 RPC 是重复建设 |
| S2 daemon 归一器+子对话落库 | **收缩**：仅需 ①codex.go 两个 item 分支接入 collab 两类 item（T-L1 已完成入库）②L2 委派进度→父线程工具卡更新（L2 范畴） | 审批审查子代理已被 guardian 既有链路覆盖 |
| S3 内联卡 | **保留但数据源改**：卡数据=子线程 meta（thread/list 可得 parentThreadId 过滤）+ 终态摘要；非新帧 | |
| S4 子对话页 | **保留，数据源改**：路由改为打开子线程的只读 ChatView（复用度更高）；横滑兄弟=parentThreadId 相同的子线程组 | |

## 3. 修正后的实施切片

- **S1'（乙级，零 proto）**：codex.go 两 item 分支接入（✅ T-L1 已完成）；thread/list 按 parentThreadId 过滤的服务端查询辅助（如需要）。
- **S2'（乙级）**：L2 委派进度→父线程工具卡（随 L2 落地）。
- **S3'/S4'（乙级）**：内联卡+子对话页（数据源=子线程），依赖 S1' 与 L2。
- 原方案的新 proto 帧与 GetSubAgentHistory RPC：**撤销**（除非未来出现非线程形态的子代理）。

## 4. 待用户确认

1. 接受"子代理=真线程，子对话=打开子线程"的方案转向吗？（原方案需新 RPC+新帧；新方案零 proto、复用整条线程管线，且 L2/L3 的委派子任务天然同构）
2. 审批审查子代理的展示：现有 guardian 卡 + autoApprovalReview 透传是否已满足"可见性"需求，还是需要更醒目的独立卡片？
