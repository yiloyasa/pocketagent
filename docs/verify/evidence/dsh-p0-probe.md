# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-21 02:17:31 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
> node=C:\Users\yanzi\AppData\Local\Programs\Nodejs\node.exe
> bin=G:\Claude Code\node_modules\@deepseek-ai\dsh\lib\bin.js version=0.1.5-rc.2
> 隔离:全程 DSH_HOME=<临时目录>\home-*,未读写用户 ~/.dsh(红线);DEEPSEEK_API_KEY=EMPTY(本机 shell 无 key)
> 设计蓝本:2026-09-20-dsh-integration-plan.md §7 P0(8 项)

结果:**4 通过 / 4 受阻(缺 key) / 0 失败**(共 8 项)

## 关键结论速览(汇报五要点)

| 要点 | 实测结论 |
|---|---|
| 流式粒度 | 未测(本机无 DEEPSEEK_API_KEY,prompt 即时凭证失败,见第 3/7 项原始帧);P1 UI 先按'块级上屏'设计(R2 上限假设),配 key 后重跑本探针量化 |
| 跨进程 resume(R5) | 协议面已实证:list 在列=true,resume 成功,重放=0 帧;上下文保持待配 key 补测 |
| config-option 模型选项形状(Models() 契约) | config-option 真实形状:应答顶层即 sessionId + configOptions 数组,每项 {id,name,category,type,currentValue,options};实测 {id:"model" category:"model" type:"select" currentValue="[\"deepseek-official\",\"deepseek-v4-flash\"]" 选项数=1} ; {id:"reasoning_effort" category:"thought_level" type:"select" currentValue="high" … |
| 缺 key 错误形状 | 无 key 下 prompt 如实失败,错误形状见原始帧;缺 key 错误形状实测:session/prompt 返回 JSON-RPC error code=-32603,message="Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK…"… |
| 冷启动耗时 | 1.415s |

## 1. initialize 握手 + 能力集 dump —— 通过

概念命令:`initialize {"protocolVersion":1,"clientCapabilities":{"fs":{"readTextFile":false,"writeTextFile":false}}}`

protocolVersion=1 agentInfo=map[name:deepseek-harness-acp version:0.0.1] 冷启动(start→initialize 应答)=1.415s

原始帧:

```
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
```

**结论**:握手成功:protocolVersion=1;agentInfo.name=deepseek-harness-acp v0.0.1;sessionCapabilities={"close":{},"list":{},"resume":{}}(list=true resume=true close=true);promptCapabilities={"audio":false,"embeddedContext":false,"image":false};authMethods=0 个。能力协商面完整,与设计 §2.1 预期一致(0.1.5 新增 session 面在场)。

**对 DshProvider 设计的影响**:dshapp.Client 启动后第一呼叫固定为 initialize;sessionCapabilities.list/resume 在场才启用 ExternalSession 重挂载路径(§5.2 能力门控);promptCapabilities.image=false → 图片输入如实报错不静默丢(R4);authMethods 为空 → 无需 authenticate 步骤,凭据链在 harness 内部解析;探测路径(provider.go DetectProviders)绝不做此握手,只做文件存在+--version(PA-REAL-0002/0003 教训)。

## 2. session/new → config-option 状态(Models() 数据源契约) —— 通过

概念命令:`session/new {"cwd":"C:\\Users\\yanzi\\AppData\\Local\\Temp\\dshprobe-171104617\\ws","mcpServers":[]}`

sessionId=ab7a4a27-711a-4bcf-b232-faec5182c57b 顶层键=[sessionId configOptions]

原始帧:

```
{
  "sessionId": "ab7a4a27-711a-4bcf-b232-faec5182c57b",
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
              "value": "[\"deepseek-official\",\"deepseek-flash\"]",
              "name": "DeepSeek-V41-Flash"
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-flash\"]",
              "name": "DeepSeek-V4-Flash",
              "description": "Fast, efficient, and economical; suited to focused, routine, or parallel tasks."
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-pro\"]",
              "name": "DeepSeek-V4-Pro",
              "description": "Stronger agentic coding, knowledge, and difficult reasoning; suited to complex or quality-critical tasks at higher cost."
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-flash-vision-exp\"]",
              "name": "DeepSeek-V4-Flash-Vision-Exp"
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

## 3. session/prompt 逐帧记录 session/update(流式粒度量化) —— 受阻(缺 key)

概念命令:`session/prompt {"sessionId":"ab7a4a27-711a-4bcf-b232-faec5182c57b","prompt":[{"type":"text","text":"1+1 等于几？直接回答"}]}`

prompt 失败: session/prompt 返回 JSON-RPC error

原始帧:

```
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-3","error":{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}}
{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}
```

**结论**:无 DEEPSEEK_API_KEY 环境下的实际错误形状见原始帧 —— 本帧即第 7 项(ErrorInfoCarrier)素材。

**对 DshProvider 设计的影响**:缺 key 时线程仍可建(prompt 才报错) → 注册不依赖 key(§4.4);错误形状映射 ErrorInfoCarrier:MISSING_CREDENTIAL + 结构化 Error,终局 TurnFailed 帧保真。

## 4. session/request_permission 反向请求(allow/reject 双路) —— 受阻(缺 key)

概念命令:`session/prompt {"prompt":[{"type":"text","text":"运行命令 echo dshprobe-permission-test …"}]} → 反向请求自动应答`

allow: 受阻(缺 key) prompt 失败: session/prompt 返回 JSON-RPC error | reject: 受阻(缺 key) prompt 失败: session/prompt 返回 JSON-RPC error

原始帧:

```
── allow 路 ──
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-4","error":{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}}
{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}
── reject 路 ──
[prompt 应答] {"jsonrpc":"2.0","id":"dshprobe-5","error":{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}}
{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}
```

**结论**:请求形状(options 数组/kind 字段/optionId)与应答形状(outcome={outcome:"selected",optionId})以原始帧为准;allow 路命令应实际执行(输出含 dshprobe-permission-test),reject 路命令不应执行。

**对 DshProvider 设计的影响**:审批桥映射:dsh session/request_permission{options:[...]} → ToolCallBlock + ApprovalReason → 手机弹卡 → SubmitApproval 翻译为 outcome{type:"selected",optionId};只做一次性 allow/reject,无'始终允许'(R3,审批卡如实两键);超时(2min)/断线 → 拒绝路径,绝不伪造'同意'(红线 1);未识别的反向请求回 JSON-RPC error(codex_requests.go 精确匹配范式)。

## 5. session/cancel 中断语义 —— 受阻(缺 key)

概念命令:`session/prompt {"text":"从 1 数到 100000，每个数字单独一行输出"} → 2s 后 session/cancel 通知`

prompt 等待失败: session/prompt 返回 JSON-RPC error

原始帧:

```
{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}
```

**结论**:无 key 下 prompt 即时凭证失败,cancel 中断语义无法在本机验证(需配 DEEPSEEK_API_KEY 补测)。

## 6. 跨进程 resume(kill 子进程 → 新进程 session/list + session/resume) —— 受阻(缺 key)

概念命令:`kill → node bin.js --profile acp → initialize → session/list → session/resume → session/prompt "我们刚才聊了什么？一句话"`

resume 后 prompt 失败(list 在列=true,resume 本身成功): session/prompt 返回 JSON-RPC error

原始帧:

```
── session/list 应答 ──
{
  "sessions": [
    {
      "sessionId": "b29b9bef-5a6b-44c2-9a3c-b028d1fc07f0",
      "cwd": "C:\\Users\\yanzi\\AppData\\Local\\Temp\\dshprobe-171104617\\ws"
    },
    {
      "sessionId": "ab7a4a27-711a-4bcf-b232-faec5182c57b",
      "cwd": "C:\\Users\\yanzi\\AppData\\Local\\Temp\\dshprobe-171104617\\ws"
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
              "value": "[\"deepseek-official\",\"deepseek-flash\"]",
              "name": "DeepSeek-V41-Flash"
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-flash\"]",
              "name": "DeepSeek-V4-Flash",
              "description": "Fast, efficient, and economical; suited to focused, routine, or parallel tasks."
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-pro\"]",
              "name": "DeepSeek-V4-Pro",
              "description": "Stronger agentic coding, knowledge, and difficult reasoning; suited to complex or quality-critical tasks at higher cost."
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-flash-vision-exp\"]",
              "name": "DeepSeek-V4-Flash-Vision-Exp"
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

## 7. 无 DEEPSEEK_API_KEY 的错误形状(ErrorInfoCarrier 素材) —— 通过

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
  "sessionId": "a915f78c-0d98-4565-88e7-c227cec05296",
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
              "value": "[\"deepseek-official\",\"deepseek-flash\"]",
              "name": "DeepSeek-V41-Flash"
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-flash\"]",
              "name": "DeepSeek-V4-Flash",
              "description": "Fast, efficient, and economical; suited to focused, routine, or parallel tasks."
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-pro\"]",
              "name": "DeepSeek-V4-Pro",
              "description": "Stronger agentic coding, knowledge, and difficult reasoning; suited to complex or quality-critical tasks at higher cost."
            },
            {
              "value": "[\"deepseek-official\",\"deepseek-v4-flash-vision-exp\"]",
              "name": "DeepSeek-V4-Flash-Vision-Exp"
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

## 8. 冷启动耗时(start → initialize 应答,首次 profile 初始化) —— 通过

概念命令:`node bin.js --profile acp(virgin DSH_HOME)→ initialize`

耗时=1.415s(含进程 spawn + Node 启动 + dsh 首次 profile 自动初始化)

**结论**:冷启动实测 1.415s;daemon 启动期绝不做握手(PA-REAL-0002/0003),懒启动首个线程/模型列举时付一次;ListModels 10s 预算兼容性以此为锚。

**对 DshProvider 设计的影响**:dshapp 懒启动 + 崩溃自动重启成本锚点;超 10s 时 ListModels 如实报'初始化中';探测只做 --version(缓存 5min)。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
