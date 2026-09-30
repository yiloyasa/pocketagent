# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 09:07:10 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
> node=C:\Users\yanzi\AppData\Local\Programs\Nodejs\node.exe
> bin=G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js version=0.2.0-rc.2
> 隔离:全程 DSH_HOME=<临时目录>\home-*,未读写用户 ~/.dsh(红线);DEEPSEEK_API_KEY=EMPTY(本机 shell 无 key)
> 设计蓝本:2026-09-20-dsh-integration-plan.md §7 P0(8 项)

结果:**0 通过 / 1 受阻(缺 key) / 0 失败**(共 1 项)

## 关键结论速览(汇报五要点)

| 要点 | 实测结论 |
|---|---|
| 流式粒度 | —(单项运行模式或该项受阻,见对应小节) |
| 跨进程 resume(R5) | 协议面已实证:list 在列=true,resume 成功,重放=0 帧;上下文保持待配 key 补测 |
| config-option 模型选项形状(Models() 契约) | —(单项运行模式或该项受阻,见对应小节) |
| 缺 key 错误形状 | —(单项运行模式或该项受阻,见对应小节) |
| 冷启动耗时 | —(单项运行模式或该项受阻,见对应小节) |

## 1. 跨进程 resume(kill 子进程 → 新进程 session/list + session/resume) —— 受阻(缺 key)

概念命令:`kill → node bin.js --profile acp → initialize → session/list → session/resume → session/prompt "我们刚才聊了什么？一句话"`

resume 后 prompt 失败(list 在列=true,resume 本身成功): session/prompt 返回 JSON-RPC error

原始帧:

```
── session/list 应答 ──
{
  "sessions": [
    {
      "sessionId": "641ea494-13ef-4aaf-b760-d0eb48e3d0c7",
      "cwd": "C:\\Users\\yanzi\\AppData\\Local\\Temp\\dshprobe-2673165884\\ws"
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
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-4","error":{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}}
{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}
```

**结论**:跨进程 resume 协议面已实证:子进程 kill 后,新进程 session/list 在列=true,session/resume 成功(应答含 configOptions,同 session/new 形状),且不重放旧 update(重放 0 帧);上下文保持(恢复后问答引用旧内容)因凭证失败未验证 —— 需配 DEEPSEEK_API_KEY 补测。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
