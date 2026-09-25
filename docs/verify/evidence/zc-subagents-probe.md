# ZCode session/subagents 探针实证(2026-09-25 · AD 代理产出)

> 证据分级:`(A)`=运行时实测 / `(bundle)`=zcode.cjs 0.16.9 静态提取。
> 探针:`daemon/cmd/zcprobe -subagents`(原始日志:`zc-subagents-probe-run1.log` 第一轮、
> `zc-subagents-probe.log` 第二轮,同一目录)。红线遵守:mode 显式 build、临时工作区用完即毁、不写用户配置。

## 参数/应答 schema(bundle 静态提取,再经 A 项运行时反证)

- 参数(`iGt`,strict):`{sessionId, endedCursor?, endedLimit?}`;endedLimit 为 1..100 的正整数,缺省 20。
- 应答(`W5i`,strict):`{revision:int≥0, childSessionIds:string[], running:条目[], ended:{total:int, items:条目[], nextCursor?}}`。
- 条目(`Nsr` 基座):`{childSessionId, agentId?, toolCallId?, subagentType, title, summary?, startedAt?}` + `endedAt?`(仅 ended 页)。
  running 页 status ∈ `running|waiting|blocked`(`Lsr`);ended 页 ∈ `success|failed|cancelled|lost`(`Fsr`)。

## 运行时反证(A 级,第二轮 2026-09-25)

| 项 | 结果 |
|---|---|
| 缺 sessionId | `-32602 Invalid params — sessionId: expected string, received undefined`(必填实证) |
| 未知键 `bogus` | `-32602 Unrecognized key: "bogus"`(strict 实证) |
| **空态应答**(PONG 轮后) | `{"revision":3,"childSessionIds":[],"running":[],"ended":{"total":0,"items":[]}}` —— 与 bundle schema 逐字一致 |
| **建会话后立刻查** | `-32004 Session not found`(第一轮 + 第二轮均复现)——**本 RPC 只认 sessionStore 已持久化的会话**,create 应答到达时落库未完成。调用纪律:轮次终局后再查 |
| 轮后查询(轮次已跑过) | 正常应答(revision 随 stateRevision 递增:2→3→5) |

## 事件面旁证

`subagent.lifecycle`(词表已收)payload schema(bundle):`{kind, phase:"spawned"|"stopped", agentId, agentType?, childSessionId, parentToolCallId?, background, status?, errorMessage?}`。两轮探针均未观测到真实帧(见下)。

## 已知边界:本机无法实测"真实委派轮"的 running/ended 条目

两轮探针的真实轮次均 `turn.failed (auth_failed)`:
- SSR gpt 组(zorv.cc 代理)401 "Invalid token"(第一轮);
- DeepSeek 组 `provider_business auth_failed`(第二轮)。

根因:zcprobe 独立拉 app-server **不带 desktopAccounts 账户适配**(`interaction/requestProviderRuntimeHeaders` 与 `provider/updateAccountConfig` 答不了),个人 provider 凭据已失效;生产链路(zcodeapp.Client)不受影响(2026-09-23 有真实轮次完成记录)。
**补偿通道**:生产同款链路的 live 测试 `TestZcodeLiveSubagents`(`go test -tags pa_live -run TestZcodeLiveSubagents ./internal/agent/ -v`,需 `PA_LIVE_ZCODE=1` + 本机账户可用)已就位,会在真实委派轮后断言清单条目形状。
因此 running/ended 条目字段以 bundle zod schema(生成空态应答的同一 schema 源)为地面真值;空态信封已 A 级实证。

## 对接产出

- `zcodeapp.Subagents()`(参数装配/应答解析,单测钉形状)→
- `ZcodeProvider.SubagentsHandler` 钩子(轮次终局异步拉一次,endedLimit=50)→
- `server.registerZcodeSubagentStubs`(存根子线程,幂等/墓碑守卫/状态映射)。
