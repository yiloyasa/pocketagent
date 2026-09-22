# 输出契约分层夹具（方案 §4.4 / §8-D3）

## 这组夹具回答什么

用户报「任务完成但没有输出」时，第一件事是判断**正文到底在不在、在哪一层**：

| 层 | 来源 | 代码位置 | 覆盖测试 |
|---|---|---|---|
| ① 增量 | `agentMessage/delta` | `codex.go` 的 streamText 累积 | `server/output_state_test.go` |
| ② item 完成 | `item/completed` → `agentMessage` | `agentMessageTextFromNotification` | `agent/codex_output_contract_test.go` |
| ③ turn 完成 | `turn/completed.turn.items[]` | `agentMessageTextsFromTurn` | `agent/codex_output_contract_test.go` |
| ④ 原生历史 | rollout 的 `task_complete.last_agent_message` | `readCodexNativeTurn` | `server/output_recovery_test.go` |

**四层全空才是「真的没话说」。** 任意一层拿到正文，就不该报 EMPTY。

## 判定与用户可见结果

| 情形 | `output_state` | 手机端显示 |
|---|---|---|
| ①②③ 任一拿到 | `TEXT` | 正文（照常） |
| ④ 拿到 | `RECOVERED_FROM_NATIVE_HISTORY` | 正文（不额外提示） |
| 无文本、无产物 | `EMPTY` | 「本轮没有正文」 |
| 只有非文本产物 | `ARTIFACT_ONLY` | 「本轮只有附件」 |
| ④ 读取失败/超时 | `ADAPTER_LOST_OUTPUT` | 「正文可能丢了…可以重试」 |
| 旧 daemon 未上报 | `UNSPECIFIED` | 不提示（不替对方表态） |

## 方案 §4.4 的 7 类场景

| # | 场景 | 期望落点 |
|---|---|---|
| 1 | 正常 `agentMessage/delta` | ① 命中 |
| 2 | 只有 `item/completed.agentMessage` | ② 命中 |
| 3 | 只有 `turn/completed.turn.items.agentMessage` | ③ 命中 |
| 4 | 压缩前有 delta、压缩后没有 | ①②③ 可能全空 → 走 ④ 补偿 |
| 5 | `turn/completed` 结构变化 | ②③ 安全返回空 → 走 ④ |
| 6 | 只有图片 / 工具 / 浏览器产物 | 无正文 → `ARTIFACT_ONLY` |
| 7 | 真正无文本 | `EMPTY` |

## 未完成（如实标注，别当成已交付）

**真机抓包样本尚未落盘。** 上表判定目前由两个测试文件用**按协议形状构造**的样本钉住
（`daemon/internal/agent/codex_output_contract_test.go`、
`daemon/internal/server/output_recovery_test.go`）。

尤其场景 4（上下文压缩边界）与场景 5（结构变化）需要**实际触发后**把真实 rollout
片段补进本目录 —— 构造的样本只能证明"我们按这个形状处理"，不能证明"上游确实发这个形状"。

补入时应连同：daemon 版本、codex 版本、触发步骤、原始 rollout 行（脱敏后）。
