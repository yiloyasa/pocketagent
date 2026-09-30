# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 09:07:06 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
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
| 缺 key 错误形状 | —(单项运行模式或该项受阻,见对应小节) |
| 冷启动耗时 | —(单项运行模式或该项受阻,见对应小节) |

## 1. rc.3 对 rc.2 旧 sessionId 的失败形态(session/list 在列性 + resume 错误形状) —— 通过

概念命令:`复制真实 DSH_HOME 副本 → initialize → session/list → session/resume <旧id> / <不存在的id>`

agent 版本=0.0.1;list 在列=true(共 57 条);A1=失败 code=-32602 message="Invalid params";A2=成功 sessionId=;A3=失败 code=-32602 message="Invalid params: session is already active: 00fa6671-e1ca-4723-b13c-3533200f3b71";幽灵id(带cwd)=失败 code=-32602 message="Invalid params: session is not resumable: 00000000-0000-0000-0000-000000000000";恢复后原id prompt=凭证错误(会话已被受理,原 id 可寻址): code=-32603 message="Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"

原始帧:

```
[副本] 跳过 3 个 node_modules junction 目录(pnpm workspace 链接,dsh 自建): profiles\acp\.dsh-module-fallback\node_modules, profiles\acp\node_modules, profiles\node_modules
副本来源=C:\Users\yanzi\AppData\Local\PocketAgent\data\dsh-home;旧会话 57 个;resume 目标=00fa6671-e1ca-4723-b13c-3533200f3b71
[list] dbdc8d22-65ac-47a3-bd40-28c0f0b08131 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 73dc1cdb-55b8-443e-a602-783f8fdd0028 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] d3655cb1-50d6-4e2e-93ee-0bda0d54fabc cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] e4a72576-487d-4f4b-ad6a-ce68aadde66c cwd="G:\\试验区"
[list] 7a5ff7d6-8dd6-4119-bdf8-c6cbc078fce4 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] f2306a8c-d009-4e81-a717-38ef2655c701 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] fec7df89-6325-4c08-a107-4db2e31b5792 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 8b7a5910-9742-49d0-b116-292f475653c3 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 5431f67d-8fd3-4d93-891e-8e380511ca81 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 4410c28c-fead-44a0-85c7-36d007d0c870 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] c400f5e7-fbc7-461c-bb73-8715e7867b5b cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 23da0430-0645-4345-9abe-30b5d8f0f537 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 71af0bdd-cde6-4db0-b8b0-ac38696a9e8c cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 4bfeaaac-37eb-4abc-a323-88239d5d90bc cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] fec8e514-f7e4-43e2-a1f9-e29e35064b76 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 00fa6671-e1ca-4723-b13c-3533200f3b71 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp" ← 目标
[list] 0a5476a1-5349-4c05-8df8-f9adaa72a956 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 3d86352e-1205-403a-b67c-4918cd8d56ec cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] e85de5dc-f16e-44af-a4ae-d9a95902f971 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] d22c2988-9ea3-425d-b45b-e17cee34cb90 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] ee5ce67f-2cd9-4825-8606-58a2b1d9c792 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] a8842864-576e-4b1a-a2e0-73f9e94ee27e cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 78dc3444-dfe8-4309-a23a-4b2c82a097ac cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] d6333427-0d70-434a-84b7-079b57896827 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 17723e76-7507-498a-b212-e5dab93dca39 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] aa78d467-d92f-42ad-9d97-2a8fef45fc7a cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 88310366-8348-4687-a439-3bbfca4a7bac cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] b4d7a8da-31cf-4f20-88b4-1ffb8aa22463 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 826e7c88-1b9a-4750-ad79-eb56ce2e6cbd cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] ae7fb516-78b8-484b-ac01-27cba9669a7c cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 87457d15-6af0-4dcc-a71e-f65a1a52fa66 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 19c0cf53-a2dc-4575-8e51-40eb8c0f5a2f cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 6c245c18-0e96-46a6-b8a7-c12cc21c377d cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 4efae359-2d28-49fe-9c3c-60db7f0c8e3b cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 4dda3da8-650e-4fbd-ba74-068ee907ba26 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 2a89483c-46c5-4c36-92ea-703385142e96 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] f368ee44-6af4-4c47-8f27-c748d0b98caa cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 68ea5613-d686-459f-902c-d185f0783819 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] c21d91bf-7ea3-49db-895d-b1e34796ca2c cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 0f2527c5-1f47-4f4d-9a27-40414eb6a9ec cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] e6ce63cd-64c2-474f-bc08-b99559301af3 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 1ceafe5b-da02-4106-9d36-5dccb1f0bb06 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] d59eb341-604e-4b65-ab74-df04309b8286 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 53747d76-4415-458d-91e4-61ba6bb04056 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 79e8cbf7-2cd7-4a6b-be6c-b23ae28b783f cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 34b891ca-b1a3-4a58-8b7d-537120e2d124 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 275e88e1-463c-4b8a-bd39-9fd27c3b956b cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] f315df1f-82cd-409a-a954-b8c39b8de50b cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] bf66b0de-0fca-4e54-95cb-382ac180b755 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 898c205c-7dd3-4847-a3cc-41d58de55ac8 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] c2d98596-e834-46b5-8de4-2042cd4734f1 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] d8723288-43b4-4b90-ba8f-7667db0fafee cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 2a1e0892-105c-40c1-a9a2-fcbe0c683d6b cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 33792ea1-f3ff-4e67-b00e-0047f404c358 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] 2b070521-93cf-4c56-971a-d16538e1a2bc cwd="G:\\试验区"
[list] 4ebb9493-53c5-40fe-af62-3969e663bc47 cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
[list] fe3bdc8c-05a1-4fc9-bf6b-c8b6a973de0d cwd="C:\\Users\\yanzi\\AppData\\Local\\Temp"
共 57 条;目标 00fa6671-e1ca-4723-b13c-3533200f3b71 在列=true
── A1 {sessionId} error ──
{"code":-32602,"message":"Invalid params","data":{"_errors":[],"cwd":{"_errors":["Invalid input: expected string, received undefined"]}}}
── A2 {sessionId,cwd} 应答 ──
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
── A3 {sessionId,cwd,mcpServers:[]} error ──
{"code":-32602,"message":"Invalid params: session is already active: 00fa6671-e1ca-4723-b13c-3533200f3b71"}
── B 幽灵id {sessionId,cwd} error ──
{"code":-32602,"message":"Invalid params: session is not resumable: 00000000-0000-0000-0000-000000000000"}
── 恢复后 prompt error ──
{"code":-32603,"message":"Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"}
```

**结论**:旧 sessionId 在 rc.3 的 session/list 中在列=true;A1(现行发法)=失败 code=-32602 message="Invalid params";A2(补 cwd)=成功 sessionId=;A3(补 cwd+mcpServers)=失败 code=-32602 message="Invalid params: session is already active: 00fa6671-e1ca-4723-b13c-3533200f3b71";参数合法时幽灵 id=失败 code=-32602 message="Invalid params: session is not resumable: 00000000-0000-0000-0000-000000000000";恢复后原 id 寻址=凭证错误(会话已被受理,原 id 可寻址): code=-32603 message="Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\"; store DEEPSEEK_API_KEY through the credentials service (the web Models page writes it), or export DEEPSEEK_API_KEY in the launching environment"。若 A2/A3 能恢复旧会话 → daemon 侧 SessionResume 补 cwd 即治本,降级只作其他失败的兜底;若补参仍失败 → 升级兼容问题确认,按实测形状定降级条件(list 不在列 = 数据错位,维持显式拒绝)。

**对 DshProvider 设计的影响**:agent/dshapp SessionResume 参数与 agent/dsh.go remountSession 的降级条件以本帧为准;副本目录已删除(清零),真实 DSH_HOME 未被触碰。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
