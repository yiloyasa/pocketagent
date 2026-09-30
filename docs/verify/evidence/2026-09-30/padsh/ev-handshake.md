# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 09:05:27 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
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

## 1. initialize 握手 + 能力集 dump —— 通过

概念命令:`initialize {"protocolVersion":1,"clientCapabilities":{"fs":{"readTextFile":false,"writeTextFile":false}}}`

protocolVersion=1 agentInfo=map[name:deepseek-harness-acp version:0.0.1] 冷启动(start→initialize 应答)=1.298s

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

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
