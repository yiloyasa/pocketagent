# pi 子代理对话可见性调研报告

环境:pi 0.85.1 + pi-subagents **0.68.0**(本机实测版本，非 0.85.1——扩展自己版本号)。静态精读 `~/.pi/agent/npm/node_modules/pi-subagents`(只读)+ `internal/piapp/client.go` + pi 官方 docs;受控实验在 `C:\Users\yanzi\AppData\Local\Temp\pisaprobe\`(隔离 PI_CODING_AGENT_DIR + PI_SUBAGENTS_TEMP_ROOT,证据保留：frames.jsonl / frames-raw.jsonl / work\ 全套产物)。用户 `~/.pi` 未写入任何内容。

---

## 1. pi-subagents 机制图(文字版)

```
手机 PocketAgent ←gRPC→ daemon ←JSONL/stdio→ pi --mode rpc(父会话进程)
                                              │ 注册 subagent 工具 + FleetView/async widget
        ┌─────────────────────────────────────┴──────────────────────────┐
        │ async:false(前台)                    │ async:true(后台,SKILL 默认) │
        ▼                                      ▼
  子会话建在父进程内(同一 pi 进程)        子会话建在独立 detached Node runner 进程
  进度 → 父会话 subagent 工具的              事件镜像 → 磁盘 artifacts
  onUpdate → RPC tool_execution_update      (status.json / events.jsonl / output-N.log)
  结果 → tool_execution_end                 活状态 → setWidget "subagent-async"
                                            完成 → 父会话注入 custom 消息(subagent-notify)
                                            → 触发父模型新回合向用户复述结果
```

- **不是** pi 的 Task 工具、不是独立 `pi` 进程:前台 = 父进程内 `createAgentSession`;后台 = runner 进程内同样的 in-process 子会话。
- **pi.events 全部 in-process**(`subagent:async-complete`、`subagent:child-status` 等**不会**出现在 RPC stdout)——daemon 能看到的只有 stdout 帧和磁盘文件。
- 子代理对话落盘(三处，全量)：①`<asyncDir>/events.jsonl`(子会话事件流,`message_update` 被丢弃,带 `subagentRunId/subagentStepIndex/subagentAgent` 注记)；②子会话文件 `<sessions>/<父会话目录>/<runId>/run-0/session.jsonl`(标准 pi 会话文件,路径直接印在完成通知里);③`<sessionDir>/subagent-artifacts/<runId>_<agent>_transcript.jsonl`。
- artifacts 根:`%TMP%/pi-subagents-<scope>/`,可用 **`PI_SUBAGENTS_TEMP_ROOT`** 环境变量重定向(daemon 可用它拿到确定路径；PocketAgent 自己的 agent dir 需在 settings.json packages 里加 `npm:pi-subagents` 才有此扩展)。

## 2. 事件真值表(RPC stdout 实测)

pi RPC stdout 上**没有**专门的"subagent 事件”帧类型；子代理活动分布在 5 个既有帧形状里:

| 阶段 | 帧形状 | 关键字段 | 实测摘录(截断) |
|---|---|---|---|
| **派发(前台)** | `tool_execution_start` | `toolName:"subagent"`, `args{agent,task,async}` | `{"toolCallId":"call_78f…","toolName":"subagent","args":{"agent":"delegate","async":false,"task":"回答“2+2等于几?”"}}` |
| **进度(前台)** | `tool_execution_update` ×N(实测 1 秒/条) | `partialResult.details.results[i].progress`: `status/recentTools[]/recentOutput[]`(子输出尾部行)/`toolCount/tokens/durationMs`;`content[0].text` 恒为 `"(running...)"` 直到结束 | `"progress":{"agent":"delegate","status":"running","recentOutput":["  \"noStagedFiles\": true,",…],"toolCount":0,"tokens":0,"durationMs":4093}` |
| **完成(前台)** | `tool_execution_end` | `result.content[0].text`=子代理最终回答全文;`details.results[i]`: `finalOutput/sessionFile/transcriptPath/usage{tokens,cost,turns}/exitCode/artifactPaths` | `"text":"2+2 = 4。"`, `"finalOutput":"2+2 = 4。"`, `"usage":{"output":209,"cost":0.0017,"turns":1}` |
| **派发(后台)** | 同上 `tool_execution_end`,**~0.15s 立即返回** | `details.asyncId`(=runId)、`details.asyncDir`(绝对路径)、`timeoutMs:1800000` | `"content":[{"text":"Run fan-out: 1/64 used…\nAsync: delegate [41cad659-…]…"}],"details":{"asyncId":"41cad659-a29b-…","asyncDir":"…\\async-subagent-runs\\41cad659-…"}}` |
| **进度(后台)** | `extension_ui_request`(fire-and-forget) `method:"setWidget"`,`widgetKey:"subagent-async"` | `widgetLines[0]` = `PI_SUBAGENT_ASYNC_JSON:{kind:"pi-subagents.async-status-snapshot",version:1,runs:[{id,label,state:"running|complete|failed",startedAt,updatedAt,activity}]}`。**只有状态机/标签/token,无对话文本**；单帧≤32KB、字符串≤160 字符。实测频率高(~120ms~1s 一帧) | `{"type":"extension_ui_request","method":"setWidget","widgetKey":"subagent-async","widgetLines":["PI_SUBAGENT_ASYNC_JSON:{…\"state\":\"running\"…}"]}` |
| **完成(后台)** | ①`message_start/end` `role:"custom"`,`customType:"subagent-notify"`,`display:false`;②随后父模型新回合(`agent_start`→`message_update`→`agent_settled`)向用户复述 | ①`content` = "Background task completed: **delegate**\n<结果预览>\nRetention-managed async directory: <asyncDir>\nSession file: <子会话.jsonl 路径>";②普通 assistant 消息流(daemon 现有渲染已覆盖) | `"message":{"role":"custom","customType":"subagent-notify","content":"Background task completed: **delegate**\n\ndelegate:\n1+1等于2。理由:…\n\nSession file: …\\run-0\\session.jsonl","display":false}` |
| **按需拉全量对话** | daemon 发 `{"type":"prompt","message":"/subagents-inspect-rpc <requestId> <asyncId> [childId] [--lines N]"}`(扩展命令,inline 执行,**不耗模型回合**)；应答 = `setWidget` `widgetKey:"subagent-inspect"` 一帧即撤(下一帧同 key 无 lines 清屏) | `widgetLines[0]`=`PI_SUBAGENT_INSPECT_JSON:{kind:"pi-subagents.inspect-reply",requestId,asyncId,status,task,messages:[{role,kind:"text|toolCall|toolResult",text}],finalOutput,truncated}` | 实测拿到完整问答:`"messages":[{"role":"user","text":"Task: 回答“1+1等于几?”…"},{"role":"assistant","text":"1+1等于2。理由:…```acceptance-report…"}],"finalOutput":"1+1等于2。理由:…"` |

真值补充：实测后台子代理 30s 跑完；完成通知帧后 5s 内父模型自动复述完毕;`agent_settled` 每回合一条，与现有 client.go 语义一致。

## 3. daemon 可达性结论与建议接线

**结论：子代理对话完全可达，且无需读文件即可覆盖 90% 需求。** 数据三源，按性价比排序：

1. **透传现有帧(工作量 S,先做)**——daemon 已收到这些帧(client.go `dispatch`→`emit` 全量分发，无需改 piapp):
   - `tool_execution_start/update/end`(toolName=="subagent")→ 前台子代理卡片：进度用 `partialResult.details.results[i].progress`,终稿用 `result.content[0].text`/`details.results[i].finalOutput`;
   - `setWidget subagent-async` → 后台运行态树(UI 侧 **debounce ≥500ms**,实测帧频最高 ~8/s);
   - `message customType=subagent-notify` → 后台完成事件(带结果预览 + asyncDir + 子会话文件路径)。注意 `display:false`,不要当普通消息渲染，转成通知/卡片。
   - 后台运行期间**禁止**杀/重挂载 pi 子进程：detached runner 是孙进程，杀了父 pi 完成通知即丢(pi-subagents 文档明说 "What is lost is the notification, not the work");判活看快照 state 而非消息流量。
2. **按需拉对话(工作量 S)**——手机点开某个子代理时，经现有 `CallCtx` 发 `/subagents-inspect-rpc <requestId> <asyncId> --lines 200`,`UIRequestHandler` 里缓冲 `subagent-inspect` 帧按 requestId 配对、丢弃清屏帧。上限 64KB/200 条，含全部问答与工具调用摘要。**这是“子代理对话画面”的正解。**
3. **读文件补全量(工作量 M,可选)**——仅当需要 >64KB 历史时读 `<asyncDir>/events.jsonl`(完整子会话事件)或 `subagent-artifacts/*_transcript.jsonl`;建议 daemon 启动时设 `PI_SUBAGENTS_TEMP_ROOT` 到自己管理的目录拿确定路径。

## 4. 边界清单(原生不可达，需如实标注)

- **后台子代理的逐 token 流式文本不可达**：RPC 通道只有有界 `recentOutput` 尾行;`message_update` 只流父会话自己的消息。手机上“子代理打字机效果”做不到，只能轮询 inspect 刷 bounded 快照。
- `subagent:child-status`、`subagent:async-complete` 等 pi.events 事件 in-process only,daemon 收不到。
- widget 快照、完成通知预览均有界截断(160 字符/32KB/previewTruncated),不代表全文。
- inspect 是会话作用域只读快照：跨会话报 `foreign_session`,run 已清理报 `stale`,未知 id 报 `not_found`;回复中无文件系统路径。
- 前台 `tool_execution_update` 的 `content[0].text` 在结束前恒为 `"(running...)"`,不能当输出渲染。
- daemon 自有 agent dir 默认只装 pi-mcp-adapter;用户环境变量显式设了 PI_CODING_AGENT_DIR 时 daemon 尊重不注入(mcpsetup.go 现纪律)——两种部署下子代理可用性不同，需在 UI 探测(`get_commands` 查 `subagents-inspect-rpc` 是否存在)后降级为“不可见”标注。
