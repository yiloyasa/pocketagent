# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 10:23:18 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
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

## 1. 0.2.0-rc.2 协议专项(image 准入/configOptions 新档/list 参数/resume 失败语义细化) —— 通过

概念命令:`initialize → session/new → prompt(image 块/resource_link 块)→ session/list{cwd,cursor} → set_config_option("" / 未知 id)→ resume 形态矩阵`

image=false;configOptions={id:model currentValue:"[\"deepseek-official\",\"deepseek-v4-flash\"]" 叶子值=[["deepseek-official","deepseek-v4-flash"] ["deepseek-official","deepseek-flash"] ["deepseek-official","deepseek-v4-pro"]]} {id:reasoning_effort currentValue:"high" 叶子值=[off low high ma…

原始帧:

```
[initialize] image=false agentVersion=0.0.1 冷启动=1.664s
── session/new 应答 ──
{
  "sessionId": "358fb14f-86e3-4fd9-84bd-0756d6f2c722",
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
[configOptions 摘要] {id:model currentValue:"[\"deepseek-official\",\"deepseek-v4-flash\"]" 叶子值=[["deepseek-official","deepseek-v4-flash"] ["deepseek-official","deepseek-flash"] ["deepseek-official","deepseek-v4-pro"]]} {id:reasoning_effort currentValue:"high" 叶子值=[off low high max]} 
── prompt(image 块) error ──
{"code":-32602,"message":"Invalid params: inline image prompts were not advertised by this connection"}
[image 准入判定] image=false(未翻真):code=-32602 message="Invalid params: inline image prompts were not advertised by this connection" —— 未广播就收图,准入层拒绝
[resource_link 判定] (resource_link prompt 成功?无 key 不应有此结果,需人工核对)
── session/list{cwd:workDir} 应答 ──
{
  "sessions": []
}
[list cursor 判定] 坏 cursor 被拒:{"code":-32602,"message":"Invalid params: session/list cursor is invalid"}
[set_config_option 空串] 空串档位被拒:{"code":-32602,"message":"Invalid params: unknown reasoning effort for deepseek-official/deepseek-v4-flash: "}
[set_config_option 未知 id] 未知 configId 被拒:{"code":-32602,"message":"Invalid params: unknown session config option: dshprobe-unknown"}
[resume·active 中] active 中 resume 被拒:{"code":-32602,"message":"Invalid params: session is already active: 358fb14f-86e3-4fd9-84bd-0756d6f2c722"}
[resume·缺 cwd(close 后)] 缺 cwd resume 被拒:{"code":-32602,"message":"Invalid params","data":{"_errors":[],"cwd":{"_errors":["Invalid input: expected string, received undefined"]}}}
── resume 应答(close 后,带 cwd) ──
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
[resume·close 后带 cwd] (close 后带 cwd resume 成功 → 持久化幸存)
[resume·cwd 不匹配] cwd 不匹配 resume 被拒:{"code":-32602,"message":"Invalid params: session is already active: 358fb14f-86e3-4fd9-84bd-0756d6f2c722"}
[resume·幽灵 id] 幽灵 id resume 被拒:{"code":-32602,"message":"Invalid params: session is not resumable: 00000000-0000-0000-0000-000000000000"}
[并发 prompt] 第一发:第一发成功(无 key 不应有此结果) | 第二发:并发第二发被拒:{"code":-32602,"message":"Invalid params: a prompt is already in flight for this session"}
```

**结论**:image 准入:image=false(未翻真):code=-32602 message="Invalid params: inline image prompts were not advertised by this connection" —— 未广播就收图,准入层拒绝;resource_link:(resource_link prompt 成功?无 key 不应有此结果,需人工核对);list 参数:cwd 过滤与 cursor 见原始帧(坏 cursor 被拒:{"code":-32602,"message":"Invalid params: session/list cursor is invalid"});空串 effort:空串档位被拒:{"code":-32602,"message":"Invalid params: unknown reasoning effort for deepseek-official/deepseek-v4-flash: "};未知 configId:未知 configId 被拒:{"code":-32602,"message":"Invalid params: unknown session config option: dshprobe-unknown"};resume 矩阵:active 中=active 中 resume 被拒:{"code":-32602,"message":"Invalid params: session is already active: 358fb14f-86e3-4fd9-84bd-0756d6f2…;缺 cwd=缺 cwd resume 被拒:{"code":-32602,"message":"Invalid params","data":{"_errors":[],"cwd":{"_errors":["Invalid input: expecte…;close 后带 cwd=(close 后带 cwd resume 成功 → 持久化幸存);cwd 不匹配=cwd 不匹配 resume 被拒:{"code":-32602,"message":"Invalid params: session is already active: 358fb14f-86e3-4fd9-84bd-0756d6f2c…;幽灵 id=幽灵 id resume 被拒:{"code":-32602,"message":"Invalid params: session is not resumable: 00000000-0000-0000-0000-000000000000…;并发:并发第二发被拒:{"code":-32602,"message":"Invalid params: a prompt is already in flight for this session"}。

**对 DshProvider 设计的影响**:daemon 适配的地面真值:image 块构造是否放行、resume 降级条件是否需按 message 细化、DeleteDshSession(close)后的持久化语义、config_option_update/空串档位对 Models() 的影响。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
