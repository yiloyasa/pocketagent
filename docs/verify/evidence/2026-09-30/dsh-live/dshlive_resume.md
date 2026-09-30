# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 10:23:52 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
> node=C:\Users\yanzi\AppData\Local\Programs\Nodejs\node.exe
> bin=G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js version=0.2.0-rc.2
> 隔离:全程 DSH_HOME=<临时目录>\home-*,未读写用户 ~/.dsh(红线);DEEPSEEK_API_KEY=SET(经环境透传)
> 设计蓝本:2026-09-20-dsh-integration-plan.md §7 P0(8 项)

结果:**1 通过 / 0 受阻(缺 key) / 0 失败**(共 1 项)

## 关键结论速览(汇报五要点)

| 要点 | 实测结论 |
|---|---|
| 流式粒度 | —(单项运行模式或该项受阻,见对应小节) |
| 跨进程 resume(R5) | 可行=true(list 含目标=true,重放=0 帧)回答="刚才你只问了我「1+1 等于几」，除此之外我们没有聊过别的。" |
| config-option 模型选项形状(Models() 契约) | —(单项运行模式或该项受阻,见对应小节) |
| 缺 key 错误形状 | —(单项运行模式或该项受阻,见对应小节) |
| 冷启动耗时 | —(单项运行模式或该项受阻,见对应小节) |

## 1. 跨进程 resume(kill 子进程 → 新进程 session/list + session/resume) —— 通过

概念命令:`kill → node bin.js --profile acp → initialize → session/list → session/resume → session/prompt "我们刚才聊了什么？一句话"`

sessionId 在 session/list 中=true;resume 应答="{\"configOptions\":[{\"id\":\"model\",\"name\":\"Model\",\"category\":\"model\",\"type\":\"select\",\"currentValue\":\"[\\\"deepseek-official\\\",\\\"deepseek-v4-flash\\\"]\",\"options\":[{\"group\":\"deepseek-official\",\"name\":\"DeepSee…";resume 重放旧 update=0 帧;恢复后回答="刚才你只问了我「1+1 等于几」，除此之外我们没有聊过别的。"(命中标记=[1+1]);stopReason=end_turn

原始帧:

```
── session/list 应答 ──
{
  "sessions": [
    {
      "sessionId": "f13f5f75-87b8-45c6-ba8c-032a881c8e37",
      "cwd": "C:\\Users\\yanzi\\AppData\\Local\\Temp\\dshprobe-4065962751\\ws"
    }
  ]
}
── session/resume 应答 ──
{
  "configOptions": [
    {
      "id": "model",
      "name": "Model",
      "category": "model",
      "type": "select",
      "currentValue": "[\"deepseek-official\",\"deepseek-v4-flash\"]",
      "options": [
        {
          "group": "deepseek-official",
          "name": "DeepSeek",
          "options": [
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-flash\"]",
              "name": "deepseek-v4-flash"
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-flash\"]",
              "name": "DeepSeek-V41-Flash"
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-pro\"]",
              "name": "DeepSeek-V4-Pro",
              "description": "Stronger agentic coding, knowledge, and difficult reasoning; suited to complex or quality-critical tasks at higher cost."
            }
          ]
        }
      ]
    },
    {
      "id": "reasoning_effort",
      "name": "Reasoning effort",
      "category": "thought_level",
      "type": "select",
      "currentValue": "high",
      "options": [
        {
          "value": "off",
          "name": "Off",
          "description": "Use for simple tasks that do not need reasoning."
        },
        {
          "value": "low",
          "name": "Low",
          "description": "Prefer for routine or latency-sensitive tasks."
        },
        {
          "value": "high",
          "name": "High",
          "description": "The default balance for most tasks."
        },
        {
          "value": "max",
          "name": "Max",
          "description": "Reserve for the hardest quality-first tasks."
        }
      ]
    }
  ]
}
── resume 后 1s 内收到的 update 帧(重放检测):0 帧 ──
[+1243ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"f13f5f75-87b8-45c6-ba8c-032a881c8e37","update":{"sessionUpdate":"agent_thought_chunk","messageId":"aede5a86-6792-4480-8fd6-391ca1d45608","content":{"type":"text","text":"The user asks: \"What did we just talk about? One sentence.\" But the conversation only contains the runtime context and the previous message \"1+1 等于几？直接回答\" — which I haven't answered yet. So we haven't actually talked about anything substantive. I should answer honestly: the only prior thing was you asking 1+1.\n\nAnswer in Chinese, one sentence."}}}}
[+1243ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"f13f5f75-87b8-45c6-ba8c-032a881c8e37","update":{"sessionUpdate":"agent_message_chunk","messageId":"aede5a86-6792-4480-8fd6-391ca1d45608","content":{"type":"text","text":"刚才你只问了我「1+1 等于几」，除此之外我们没有聊过别的。"}}}}
[+1243ms] {"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"f13f5f75-87b8-45c6-ba8c-032a881c8e37","update":{"sessionUpdate":"usage_update","used":6227,"size":1000000}}}
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-4","result":{"stopReason":"end_turn"}}
```

**结论**:跨进程 resume 可行=true:子进程被 kill 后,新进程 session/list 列出 目标会话,session/resume 成功,恢复后追问引用旧上下文(回答:"刚才你只问了我「1+1 等于几」，除此之外我们没有聊过别的。")。resume 是否重放旧 update:0 帧(0 帧=符合'log 恢复不重放'预期,手机端不会收到重复历史帧)。

**对 DshProvider 设计的影响**:R5(跨进程 resume)关闭/收敛:ExternalSession.SessionID = ACP sessionId 落 threads.json;重挂载 = 懒启动 → initialize → session/list 校验存在 → session/resume;'不在列 → 显式报错不静默新开'(三方共同教训);不重放旧 update → 手机端重挂载后不收重复帧,无需去重逻辑。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
