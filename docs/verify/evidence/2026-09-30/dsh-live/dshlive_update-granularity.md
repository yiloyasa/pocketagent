# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 10:21:24 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
> node=C:\Users\yanzi\AppData\Local\Programs\Nodejs\node.exe
> bin=G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js version=0.2.0-rc.2
> 隔离:全程 DSH_HOME=<临时目录>\home-*,未读写用户 ~/.dsh(红线);DEEPSEEK_API_KEY=SET(经环境透传)
> 设计蓝本:2026-09-20-dsh-integration-plan.md §7 P0(8 项)

结果:**1 通过 / 0 受阻(缺 key) / 0 失败**(共 1 项)

## 关键结论速览(汇报五要点)

| 要点 | 实测结论 |
|---|---|
| 流式粒度 | assistant 文本以 1 帧到齐(1 字符)= 块级(已提交消息文本块),非逐 delta 流式;thought 帧 0 个(各 [] 字符);tool_call 帧 0 个(纯问答无工具,符合预期);帧内 usage/context 字段在场=true;终局 stopReason=end_turn。 |
| 跨进程 resume(R5) | —(单项运行模式或该项受阻,见对应小节) |
| config-option 模型选项形状(Models() 契约) | —(单项运行模式或该项受阻,见对应小节) |
| 缺 key 错误形状 | —(单项运行模式或该项受阻,见对应小节) |
| 冷启动耗时 | —(单项运行模式或该项受阻,见对应小节) |

## 1. session/prompt 逐帧记录 session/update(流式粒度量化) —— 通过

概念命令:`session/prompt {"sessionId":"40501de0-6659-4028-aff8-aea3d0963ef4","prompt":[{"type":"text","text":"1+1 等于几？直接回答"}]}`

update 帧=2 变体分布=map[agent_message_chunk:1 usage_update:1] prompt 应答 stopReason=end_turn

原始帧:

```
[+1144ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"40501de0-6659-4028-aff8-aea3d0963ef4","update":{"sessionUpdate":"agent_message_chunk","messageId":"b5069f8b-166e-4b81-b991-012e9c1a26af","content":{"type":"text","text":"2"}}}}
[+1145ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"40501de0-6659-4028-aff8-aea3d0963ef4","update":{"sessionUpdate":"usage_update","used":6122,"size":1000000}}}
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-3","result":{"stopReason":"end_turn"}}
```

**结论**:assistant 文本以 1 帧到齐(1 字符)= 块级(已提交消息文本块),非逐 delta 流式;thought 帧 0 个(各 [] 字符);tool_call 帧 0 个(纯问答无工具,符合预期);帧内 usage/context 字段在场=true;终局 stopReason=end_turn。

**对 DshProvider 设计的影响**:agent_message_chunk → pb.AssistantDelta;块级粒度下手机端'逐字上屏'实为'按块上屏'(R2 实锤,UI 预期按此校准,跟踪上游 discussion #594);agent_thought_chunk → ThinkingDelta(HideReasoning 过滤);usage/context 字段在场性决定 UsageUpdate(上下文环)的数据源与兜底;未识别变体 → 主线 A 原样包装透传(native_notifications 范式)。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
