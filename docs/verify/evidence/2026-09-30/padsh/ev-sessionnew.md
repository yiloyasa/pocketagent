# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 09:05:29 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
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
| config-option 模型选项形状(Models() 契约) | config-option 真实形状:应答顶层即 sessionId + configOptions 数组,每项 {id,name,category,type,currentValue,options};实测 {id:"model" category:"model" type:"select" currentValue="[\"deepseek-official\",\"deepseek-v4-flash\"]" 选项数=1} ; {id:"reasoning_effort" category:"thought_level" type:"select" currentValue="high" … |
| 缺 key 错误形状 | —(单项运行模式或该项受阻,见对应小节) |
| 冷启动耗时 | —(单项运行模式或该项受阻,见对应小节) |

## 1. session/new → config-option 状态(Models() 数据源契约) —— 通过

概念命令:`session/new {"cwd":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\dshprobe-3722640769\\ws","mcpServers":[]}`

sessionId=f2a0d1af-72c3-4ad2-ab98-a8d5f9daba59 顶层键=[sessionId configOptions]

原始帧:

```
{
  "sessionId": "f2a0d1af-72c3-4ad2-ab98-a8d5f9daba59",
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
```

**结论**:config-option 真实形状:应答顶层即 sessionId + configOptions 数组,每项 {id,name,category,type,currentValue,options};实测 {id:"model" category:"model" type:"select" currentValue="[\"deepseek-official\",\"deepseek-v4-flash\"]" 选项数=1} ; {id:"reasoning_effort" category:"thought_level" type:"select" currentValue="high" 选项数=4}。关键细节:model 的 value/currentValue 是 JSON 字符串编码的 [provider,model] 二元组(如 "[\"deepseek-official\",\"deepseek-v4-flash\"]",双层编码,set_config_option 时须原样回传该字符串);模型选项按 group 分组且 live catalog 随版本浮动;effort 档位实测 off/low/high/max。这是 DshProvider.Models() 的唯一数据源(无静态名单)。

**对 DshProvider 设计的影响**:Models() 数据源 = session/new 应答中的 config-option 状态:缓存 + TTL,InvalidateModels() 清缓存(ModelCatalogRefresher);无会话时用一次性瞬态会话取样后 session/close;set_config_option(model/reasoning_effort) 的 optionId 以本帧实测字段为准;严禁在 daemon 里写死模型名单(设计原则 §7)。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
