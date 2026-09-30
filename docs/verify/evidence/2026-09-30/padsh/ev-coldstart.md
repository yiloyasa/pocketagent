# dsh ACP P0 探针记录(DeepSeek Harness 桥接前置验证)

> 生成:2026-09-30 09:05:28 工具:daemon/cmd/dshprobe(node 直拉 bin.js --profile acp,Go 手写 JSON-RPC 2.0 分帧,零外部依赖)
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
| 冷启动耗时 | 1.236s |

## 1. 冷启动耗时(start → initialize 应答,首次 profile 初始化) —— 通过

概念命令:`node bin.js --profile acp(virgin DSH_HOME)→ initialize`

耗时=1.236s(含进程 spawn + Node 启动 + dsh 首次 profile 自动初始化)

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

**结论**:冷启动实测 1.236s。对照预算:ListModels 并行拉取 10s 预算内可完成;daemon 启动期绝不做此握手(PA-REAL-0002/0003),只有懒启动首个线程/模型列举时付一次。

**对 DshProvider 设计的影响**:dshapp 懒启动 + 崩溃自动重启的成本锚点;ListModels 的 dsh 分支若遇冷启动超 10s → 如实报'初始化中'(既有并行有界机制自动兜底);探测只做 --version(有缓存 5min)。

> 未完成/需 DEEPSEEK_API_KEY 补测的项以「受阻(缺 key)」标注;其原始帧即错误形状素材(第 7 项口径),非失败口径伪造。
