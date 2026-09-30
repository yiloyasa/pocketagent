# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 10:22:41 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
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

## 1. session/cancel 中断语义 —— 通过

概念命令:`session/prompt {"text":"从 1 数到 100000，每个数字单独一行输出"} → 2s 后 session/cancel 通知`

cancel 发出后 6ms 收到 prompt 应答 stopReason=cancelled;期间 update 帧=1 变体=map[agent_thought_chunk:1]

原始帧:

```
[+2006ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"35adca48-d14d-43da-99c3-8fbcdd823eef","update":{"sessionUpdate":"agent_thought_chunk","messageId":"61fa8a7f-224f-45d7-b9f1-88035ecee9c2","content":{"type":"text","text":"The user wants to print numbers from 1 to 100000, each on a separate line. I should use the pwsh tool. But the output would be huge — 100000 lines. Better to write to a file? The request says output each number on its own line. I could generate a file and report, or just emit. Given output truncation, best approach: write to a file and confirm, maybe also output. Hmm, \"输出\" means output. But dumping 100000 lines into the tool result would be truncated anyway. Let me generate a file in the workspace and also show first/last lines, mentioning truncation. Actually the user may want the actual output text. The tool truncates long output to the tail and saves full output to a file. I could run a command that outputs all 100000 lines and let the harness save it. That would be a massive tool result though.\n\nBetter: write numbers to a file, then tell user. But that's not directly \"output\". I think the sensible action: produce the output as a file and display it. Let me do: generate numbers.txt with 1..100000 and report line count and head"}}}}
```

**结论**:session/cancel(通知,无 id)发出后 prompt 终局 stopReason=cancelled(期望 cancelled);update 流是否出现 cancelled 变体见原始帧。

**对 DshProvider 设计的影响**:CancelTurn → session/cancel 通知;stopReason=cancelled → TurnFailed(cancelled);prompt 终局由应答驱动而非超时(dshapp 的 callTimeout 不管 prompt 在飞,§4.2)。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
