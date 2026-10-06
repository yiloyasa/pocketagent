# hermes 字段级覆盖矩阵(2026-10-07)

- 事实源:本机实装 hermes 的**只读**静读(未运行任何程序):
  - 安装目录:`G:\Claude Code\node_modules\hermes-agent\runtime\hermes-agent\`(由 `hermesapp/locate.go` 派生链定位:`NPM_CONFIG_PREFIX` 缺省 → `~/.npmrc prefix=G:\Claude Code` → `<prefix>\node_modules\hermes-agent` → venv python 在场);
  - 适配器协议面:`acp_adapter/`(server.py / events.py / tools.py / commands.py / session.py / permissions.py / content.py);
  - 传输方法全集:venv `acp/meta.py` 的 `CLIENT_METHODS`(适配器→daemon 方向的全部方法)与 `AGENT_METHODS`;
  - daemon 侧现状:`daemon/internal/hermesapp/{client.go,acp.go}`、`daemon/internal/agent/hermes.go` 及其测试;
  - HEAD:ad15c5d。
- 适配规则:新映射全部落**既有 pb TurnServerMessage 变体**(TextDelta / ThinkingDelta / ToolCallUpdate / CodexEvent / TurnServerMessage_Usage)与 provider `Run` 的 `*pb.Usage` 返回通道;未改 proto/pb 生成码。

## 一、传输层帧型全集(适配器 → daemon)

| 帧型 | 来源 | daemon 处置 | 结论 |
|---|---|---|---|
| 应答帧(id,无 method) | JSON-RPC | pending 表配对;迟到走 abandoned 认领留痕 | 已覆盖 |
| 应答帧 id 非 int64 | 上游异常 | **改前静默丢 → 改后日志留痕**(client.go) | 本轮收口 |
| 反向请求 `session/request_permission` | permissions.py / edit_approval.py | 审批卡 + selected 应答;超时拒绝侧 | 已覆盖 |
| 反向请求 `fs/read_text_file`、`fs/write_text_file`、`terminal/*` | acp/meta.py CLIENT_METHODS(daemon 未声明该能力,实测不会来) | readLoop 回 -32601,不伪造 decision | 已覆盖 |
| 通知 `session/update` | server.py/events.py 全部投影 | 逐变体见下表 | 见下表 |
| 其他通知(非 session/update) | 上游新版扩展 | **改前 readLoop default 静默丢 → 改后经 UnknownNotificationHook 上抛进未知事件台账(UNKNOWN_NOTIFICATION)**(client.go + agent/hermes.go 注入) | 本轮收口 |
| stderr 文本流 | 适配器日志面 | drainStderr 刻意不透传(防刷屏;死因经进程退出+RPC error data 可见)——**书面理由,非旁路** | 保留 |
| stdout 非协议帧 | 库噪音 | 静默丢(stderr 噪音混入 stdout 的唯一防线)——书面理由 | 保留 |

## 二、session/update 变体 × 字段覆盖矩阵

适配器可发送的变体全集(静读 server.py/events.py/commands.py:`update_agent_message_text`、`update_agent_thought_text`、`update_user_message_text`、`start_tool_call`、`update_tool_call`、`AgentPlanUpdate`、`UsageUpdate`、`SessionInfoUpdate`、`AvailableCommandsUpdate`、`CurrentModeUpdate`):

| 变体 | 关键字段 | 改前处置 | 改后处置(本轮) |
|---|---|---|---|
| `agent_message_chunk` | content.text;messageId(分泡用);`_meta.hermes.compactionSummary`(仅回放) | TextDelta + final 累计 | 不变。messageId 是 Zed 分泡语义,daemon 流式模型无此维度(不丢内容);`_meta` 压缩标记只出现在 load 回放(重挂载路径刻意弃帧不重投影,见下节)——无静默丢弃 |
| `agent_thought_chunk` | content.text | ThinkingDelta(HideReasoning 过滤) | 不变 |
| `tool_call` / `tool_call_update` | toolCallId/title/kind/status/content(text、diff)/rawInput/**locations** | STARTED/UPDATE/FINISHED;diff 并入 JsonArgs;**locations 整字段丢** | **locations 补收**:解析层新增 `Locations[]{Path,Line}`,工具卡 JsonArgs 增 `locations`(工具卡维度的文件位置)。status 枚举 pending/in_progress/completed/failed 全被 phase 映射覆盖(abandoned 终态=status failed + 说明文本,照常 FINISHED) |
| `plan` | entries[]{content,priority,status} | 计划卡("status\|text") | 不变;priority 适配器恒填 "medium"(events.py:51),常量无信息,书面确认丢弃 |
| `usage_update` | size(上下文窗)/used(占用) | TurnServerMessage_Usage 窗口口径 | 不变 |
| `session_info_update` | title/updatedAt/`_meta.hermes.sessionProvenance`(压缩头轮转) | **静默丢** | **带标题 → STATUS 卡「hermes · 会话命名」+ 台账 KNOWN_PASSTHROUGH**;无标题(纯轮转出处帧)→ 台账留痕。标题→thread 落库的全量同步(codex/zcode/pi 的 TitleHandler→server 同款)需 server 层接线,移交清单见 §五 |
| `user_message_chunk` | content.text(排队 prompt 回显 + load 回放) | **静默丢** | 台账 KNOWN_PASSTHROUGH 留痕;不重投影(手机端已展示用户自己的输入,dsh 同口径) |
| `current_mode_update` | currentModeId | **静默丢** | 台账 KNOWN_PASSTHROUGH 留痕(现装适配器静读未发现发送点,协议成员预收) |
| `available_commands_update` | availableCommands[]{name,description,input.hint}(commands.py 九个斜杠命令) | **静默丢(只留条数)** | 台账 KNOWN_PASSTHROUGH 留痕(daemon 暂无斜杠命令消费方;dsh available_commands_update 同档)。命令面透出手机属产品决策,移交清单见 §五 |
| 认不出的变体 | — | **静默丢(解析层留 Raw,投影层无人消费)** | **台账 UNKNOWN_NOTIFICATION + recordAs 限频上屏**(每方法至多 2 张卡) |
| 已知变体空转(空文本 chunk、零值 usage 等) | — | 静默(无分支命中) | 台账 KNOWN_PROJECTED/KNOWN_PASSTHROUGH 留痕,不冒充未知(hermesNoticeKind 分档器,dshNoticeKind 同口径) |

## 三、prompt 终局应答(stopReason + usage)

| 字段 | 改前 | 改后 |
|---|---|---|
| `stopReason`(end_turn / refusal / cancelled) | 已映射(StopReasonIsError + 正文判据) | 不变 |
| `usage.{input_tokens,output_tokens,total_tokens,thought_tokens,cached_read_tokens}`(server.py `_finish_turn`) | **整块静默丢(Prompt 只回 stopReason)** | **`Prompt` 返回 `*PromptOutcome`(stopReason+usage);Run 映射为 `*pb.Usage{InputTokens,OutputTokens,CacheReadTokens,ReasoningTokens}` 经返回值交 server 层落轮次记录与线程累计(thread.go usagePtr 通道)**。total_tokens 为前两者可推导和无独立 pb 位,书面确认;不另发流帧——流帧通道在 hermes 承载 usage_update 的"窗口占用"语义,混用会污染 ContextUsageCard |
| 应答体解析失败 | 静默按空 stopReason | 行为不变(空终局走正文判据),增加一条日志留痕 |

## 四、会话建立/审批/取消面(核对结论,无新增缺口)

- initialize 能力快照、session/new、load(resume/fork/list)、set_model:既有实现逐字段对表无缺口(acp.go;`models`/`modes` 应答面已收)。
- `session/request_permission`:五选项(allow_once/allow_session/allow_always/deny/deny_always)与编辑审批两选项形状均已按 kind 前缀归侧,超时/无通道拒绝侧,不伪造 decision。
- 取消链:session/cancel 通知 + 看门狗三闸(150s 静默/60min 上限/审批豁免)+ CallTurnCtx 放弃路径的 abandoned 认领留痕(client.go)——均已覆盖。
- 非本轮会话的 session/update 帧(notif 闭包 `ev.SessionID != sid` 过滤)与泵卸钩后的尾帧:**刻意过滤,非旁路**(与 codex `n.ThreadId != codexTID → return`、dsh/zcode 的同位过滤完全同款;记录进台账反会被共享适配器上其他会话的合法流量刷爆 200 条滚动窗)。

## 五、移交清单(需主模型/后续波次裁决)

1. **标题落库**:session_info_update 的 title 目前只有轮内 STATUS 卡;要像 codex/zcode/pi 一样落到 thread 记录(手机会话列表改名),需给 HermesProvider 加 TitleHandler 字段并在 server/server.go 接线——本轮禁碰 server/。
2. **斜杠命令面**:`available_commands_update` 的九个 hermes 斜杠命令(help/model/tools/context/reset/compress/steer/queue/version)如需透出手机命令面板,属产品决策 + pb 通道设计。
3. **echo 结论(B 部分)**:`agent/echo.go` + `echo_runtime.go` 全量核对——echo 是合成 provider,事件面即其源码字面量(ThinkingDelta → ToolCall STARTED/FINISHED(带 Result)→ TextDelta → 终稿 blocks + `*pb.Usage`),**回传面与 pb 映射一一对应,无缺口,无旁路**,一行结论:完备。
4. 工具卡 `Result` 块:hermes 工具产出经 JsonArgs.output 承载(与 codex 同款),未设 ToolResultBlock.Result——非丢弃(内容在卡上),仅记录形状差异。
