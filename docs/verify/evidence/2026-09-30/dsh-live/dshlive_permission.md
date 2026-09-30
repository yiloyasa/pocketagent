# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 10:21:52 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
> node=C:\Users\yanzi\AppData\Local\Programs\Nodejs\node.exe
> bin=G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js version=0.2.0-rc.2
> 隔离:全程 DSH_HOME=<临时目录>\home-*,未读写用户 ~/.dsh(红线);DEEPSEEK_API_KEY=SET(经环境透传)
> 设计蓝本:2026-09-20-dsh-integration-plan.md §7 P0(8 项)

结果:**1 通过 / 0 受阻(缺 key) / 0 失败**(共 1 项)

## 关键结论速览(汇报五要点)

| 要点 | 实测结论 |
|---|---|
| 流式粒度 | —(单项运行模式或该项受阻,见对应小节) |
| 跨进程 resume(R5) | —(单项运行模式或该项受阻,见对应小节) |
| config-option 模型选项形状(Models() 契约) | —(单项运行模式或该项受阻,见对应小节) |
| 缺 key 错误形状 | —(单项运行模式或该项受阻,见对应小节) |
| 冷启动耗时 | —(单项运行模式或该项受阻,见对应小节) |

## 1. session/request_permission 反向请求(allow/reject 双路) —— 通过

概念命令:`session/prompt {"prompt":[{"type":"text","text":"运行命令 echo dshprobe-permission-test …"}]} → 反向请求自动应答`

allow: stopReason=end_turn tool_call 轨迹=[tool_call:in_progress tool_call_update:completed];审批(allow)=(未收到反向请求 —— 工具未被调用或被策略自动放行/拒绝,见原始帧) | reject: stopReason=end_turn tool_call 轨迹=[tool_call:in_progress tool_call_update:completed];审批(reject)=(未收到反向请求 —— 工具未被调用或被策略自动放行/拒绝,见原始帧)

原始帧:

```
── allow 路 ──
[+ 860ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"usage_update","used":6190,"size":1000000}}}
[+ 861ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"tool_call","toolCallId":"call_00_ET_2rTJvZIxE0729rHuMo0W3850","title":"pwsh","kind":"other","status":"in_progress","rawInput":{"command":"echo dshprobe-permission-test","description":"Echo the probe test string"}}}}
[+1948ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"tool_call_update","toolCallId":"call_00_ET_2rTJvZIxE0729rHuMo0W3850","status":"completed","content":[{"type":"content","content":{"type":"text","text":"dshprobe-permission-test\r\n"}}]}}}
[+2862ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"agent_message_chunk","messageId":"2c58c0ba-d7fc-49d7-b4f9-554a7b9abf8f","content":{"type":"text","text":"命令已运行完成。\n\n**输出：**\n```\ndshprobe-permission-test\n```\n\n`echo dshprobe-permission-test` 按预期回显了该字符串，退出码为 0。"}}}}
[+2863ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"usage_update","used":6256,"size":1000000}}}
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-3","result":{"stopReason":"end_turn"}}
── reject 路 ──
[+ 832ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"usage_update","used":6349,"size":1000000}}}
[+ 832ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"tool_call","toolCallId":"call_00_ET_WyD3R5eNE2yBbojQePvD3635","title":"pwsh","kind":"other","status":"in_progress","rawInput":{"command":"echo dshprobe-permission-test-2","description":"Echo second probe test string"}}}}
[+1561ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"tool_call_update","toolCallId":"call_00_ET_WyD3R5eNE2yBbojQePvD3635","status":"completed","content":[{"type":"content","content":{"type":"text","text":"dshprobe-permission-test-2\r\n"}}]}}}
[+2214ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"agent_message_chunk","messageId":"4ef2c561-9e43-4756-bc06-191bc933e472","content":{"type":"text","text":"命令已运行完成。\n\n**输出：**\n```\ndshprobe-permission-test-2\n```\n\n`echo dshprobe-permission-test-2` 按预期回显了该字符串，退出码为 0。"}}}}
[+2217ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"76bdbd57-0134-45bd-a9e1-9da2c28ddfa5","update":{"sessionUpdate":"usage_update","used":6421,"size":1000000}}}
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-4","result":{"stopReason":"end_turn"}}
```

**结论**:请求形状(options 数组/kind 字段/optionId)与应答形状(outcome={outcome:"selected",optionId})以原始帧为准;allow 路命令应实际执行(输出含 dshprobe-permission-test),reject 路命令不应执行。

**对 DshProvider 设计的影响**:审批桥映射:dsh session/request_permission{options:[...]} → ToolCallBlock + ApprovalReason → 手机弹卡 → SubmitApproval 翻译为 outcome{type:"selected",optionId};只做一次性 allow/reject,无'始终允许'(R3,审批卡如实两键);超时(2min)/断线 → 拒绝路径,绝不伪造'同意'(红线 1);未识别的反向请求回 JSON-RPC error(codex_requests.go 精确匹配范式)。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
