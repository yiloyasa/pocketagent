# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 09:05:31 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
> node=C:\Users\yanzi\AppData\Local\Programs\Nodejs\node.exe
> bin=G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js version=0.2.0-rc.2
> 隔离:全程 DSH_HOME=<临时目录>\home-*,未读写用户 ~/.dsh(红线);DEEPSEEK_API_KEY=EMPTY(本机 shell 无 key)
> 设计蓝本:2026-09-20-dsh-integration-plan.md §7 P0(8 项)

结果:**1 通过 / 0 受阻(缺 key) / 0 失败**(共 1 项)

## 关键结论速览(汇报五要点)

| 要点 | 实测结论 |
|---|---|
| 流式粒度 | —(单项运行模式或该项受阻,见对应小节) |
| 跨进程 resume(R5) | —(单项运行模式或该项受阻,见对应小节) |
| config-option 模型选项形状(Models() 契约) | —(单项运行模式或该项受阻,见对应小节) |
| 缺 key 错误形状 | 无 key 下 prompt 如实失败,错误形状见原始帧 | 缺 key 错误形状实测:session/prompt 返回 JSON-RPC error code=-32603,message="Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK… |
| 冷启动耗时 | —(单项运行模式或该项受阻,见对应小节) |

## 1. 无 DEEPSEEK_API_KEY 的错误形状(ErrorInfoCarrier 素材) —— 通过

概念命令:`env 剔除 DEEPSEEK_API_KEY → initialize → session/new → session/prompt "1+1 等于几？直接回答"`

无 key 下 prompt 如实失败,错误形状见原始帧

原始帧:

```
── initialize 应答(无 key 下握手不受影响)──
{
  "protocolVersion": 1,
  "agentInfo": {
    "name": "deepseek-harness-acp",
    "version": "0.0.1"
  },
  "agentCapabilities": {
    "mcpCapabilities": {
      "http": true
    },
    "promptCapabilities": {
      "image": false,
      "audio": false,
      "embeddedContext": false
    },
    "sessionCapabilities": {
      "close": {},
      "list": {},
      "resume": {}
    }
  },
  "authMethods": []
}
── session/new 应答 ──
{
  "sessionId": "d864235d-4006-436d-8a3b-8f4800cda80a",
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
── session/prompt 应答 ──
{"jsonrpc":"2.0","id":"dshprobe-3","error":{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}}
── session/prompt JSON-RPC error ──
{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}
```

**结论**:缺 key 错误形状实测:session/prompt 返回 JSON-RPC error code=-32603,message="Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK…"(无结构化 data 字段);initialize/session/new 均不受影响。ErrorInfoCarrier 映射:按 code+message 归一化为 MISSING_CREDENTIAL,原样 message 给人话诊断;session/new 无 key 仍成功 → '注册不依赖 key、首条 prompt 才报错'(§4.4)成立。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
