> 【构建指纹(T-FU 补录,2026-09-21)】
> - go 版本: go1.27.1 windows/amd64
> - 探针构建基线: 8a638b0 + 工作树(按 T-L1 交付/审核记录;T-L1 交付以未提交工作树形式存在,codex_subagents.go 等至今为未跟踪文件,无法从二进制复核 commit,以交付记录为准)
> - 补录时仓库实测: git HEAD = ceebeb43113f16d498daa5bdde05f6021223443b(2026-09-21 01:11 +0800),工作树含未提交改动;探针二进制 daemon/codexsubagentprobe.exe 修改时间 2026-09-21 02:22:10 +0800,与下文探针运行时间 02:22:50 吻合
> - 本次补录仅在本文件头部追加本块,原文一字未改。

# codex 子代理 S0 探针证据(T-L1)

- 时间: 2026-09-21 02:22:50
- 参数: thread/start { cwd: G:\项目1\pocketagent, approvalsReviewer: "auto_review", approvalPolicy: "on-request", sandbox: "read-only" }
- prompt: 运行命令:curl.exe -s -o G:\pa-s0-probe-out.txt https://example.com 然后告诉我退出码。该操作需要经过审查(auto review)
- 结果: turn/completed true(耗时 38s);subAgentActivity+collabAgentToolCall item 共 0 条

## 通知方法计数

- `account/rateLimits/updated` x2
- `guardianWarning` x1
- `item/agentMessage/delta` x32
- `item/autoApprovalReview/completed` x1
- `item/autoApprovalReview/started` x1
- `item/commandExecution/outputDelta` x2
- `item/completed` x3
- `item/started` x3
- `mcpServer/startupStatus/updated` x8
- `remoteControl/status/changed` x1
- `thread/started` x1
- `thread/status/changed` x2
- `thread/tokenUsage/updated` x2
- `turn/completed` x1
- `turn/started` x1
- `warning` x1

## 原始帧(按到达顺序)

```json
[02:22:12.254] remoteControl/status/changed {"status":"disabled","serverName":"WHOISLOVE","installationId":"dd86abe3-a1b8-4048-a2b0-c080ef727aa7","environmentId":null}
[02:22:12.486] thread/started {"thread":{"id":"01a0c00d-e928-71c2-81f0-1547760d3ce0","environments":[{"environmentId":"local","cwd":"G:\\项目1\\pocketagent","runtimeWorkspaceRoots":["G:\\项目1\\pocketagent"]}],"extra":null,"sessionId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","forkedFromId":null,"parentThreadId":null,"preview":"","ephemeral":false,"section":null,"sectionEnteredAt":null,"projectId":null,"historyMode":"paginated","modelProvider":"dex_ai","model":"dexacct.MThkNmFkNmFkYzNiMzJiODAxMWI.ZW5kcG9pbnRfMWEwYjhlNmY0YzZfZWE4OGYy.Z3B0LTYtYXN0cmE","reasoningEffort":"high","createdAt":1789928532,"updatedAt":1789928532,"recencyAt":1789928532,"status":{"type":"idle"},"path":"C:\\Users\\yanzi\\.codex\\sessions\\2026\\09\\21\\rollout-2026-09-21T02-22-12-01a0c00d-e928-71c2-81f0-1547760d3ce0.jsonl","cwd":"G:\\项目1\\pocketagent","cliVersion":"0.155.1","originator":"codexsubagentprobe","source":"vscode","canAcceptDirectInput":true,"threadSource":null,"agentNickname":null,"agentRole":null,"gitInfo":null,"name":null,"daybreakEnabled":null,"turns":[]}}
[02:22:12.486] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"node_repl","status":"starting","error":null,"failureReason":null}
[02:22:12.486] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"codex_apps","status":"starting","error":null,"failureReason":null}
[02:22:12.486] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"cua_repl","status":"starting","error":null,"failureReason":null}
[02:22:12.487] thread/start -> 01a0c00d-e928-71c2-81f0-1547760d3ce0 (approvalsReviewer=auto_review)
[02:22:12.487] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"idalib","status":"starting","error":null,"failureReason":null}
[02:22:12.510] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"idalib","status":"failed","error":"MCP client for `idalib` failed to start: MCP startup failed: program not found","failureReason":null}
[02:22:12.510] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"node_repl","status":"ready","error":null,"failureReason":null}
[02:22:12.597] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"cua_repl","status":"ready","error":null,"failureReason":null}
[02:22:12.621] thread/status/changed {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","status":{"type":"active","activeFlags":[]}}
[02:22:12.621] turn/started {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turn":{"id":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","items":[],"itemsView":"notLoaded","status":"inProgress","error":null,"startedAt":1789928532,"completedAt":null,"durationMs":null}}
[02:22:27.343] warning {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","message":"Skill descriptions were shortened to fit the skills context budget. Codex can still see every skill, but some descriptions are shorter. Disable unused skills or plugins to leave …
[02:22:27.403] item/started {"item":{"type":"userMessage","id":"01a0c00e-244b-7f51-9409-2c53b93e783d","clientId":null,"content":[{"type":"text","text":"运行命令:curl.exe -s -o G:\\pa-s0-probe-out.txt https://example.com 然后告诉我退出码。该操作需要经过审查(auto review)","text_elements":[]}]},"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","startedAtMs":1789928547403}
[02:22:27.417] item/completed {"item":{"type":"userMessage","id":"01a0c00e-244b-7f51-9409-2c53b93e783d","clientId":null,"content":[{"type":"text","text":"运行命令:curl.exe -s -o G:\\pa-s0-probe-out.txt https://example.com 然后告诉我退出码。该操作需要经过审查(auto review)","text_elements":[]}]},"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","completedAtMs":1789928547403}
[02:22:35.451] item/started {"item":{"type":"commandExecution","id":"call_g1fSTXZ62hpzUIhq1rCwGXOW","pluginId":null,"scriptPath":null,"command":"\"C:\\\\Program Files\\\\PowerShell\\\\7\\\\pwsh.exe\" -NoProfile -Command \"curl.exe -s -o G:\\\\pa-s0-probe-out.txt https://example.com; \"'$code = $LASTEXITCODE; Write-Output \"ExitCode=$code\"; exit $code'","cwd":"G:\\项目1\\pocketagent","processId":null,"source":"agent","status":"inProgress","commandActions":[{"type":"unknown","command":"\"C:\\\\Program Files\\\\PowerShell\\\\7\\\\pwsh.exe\" -NoProfile -Command \"curl.exe -s -o G:\\\\pa-s0-probe-out.txt https://example.com; \"'$code = $LASTEXITCODE; Write-Output \"ExitCode=$code\"; exit $code'"}],"aggregatedOutput":null,"exitCode":null,"durationMs":null},"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","startedAtMs":1789928555452}
[02:22:35.452] item/autoApprovalReview/started {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","startedAtMs":1789928555452,"reviewId":"042ef1a5-0327-4a60-9623-92d72fb586a3","targetItemId":"call_g1fSTXZ62hpzUIhq1rCwGXOW","review":{"status":"inProgress","riskLevel":null,"userAuthorization":null,"rationale":null},"action":{"type":"command","source":"unifiedExec","command":"\"C:\\\\Program Files\\\\PowerShell\\\\7\\\\pwsh.exe\" -NoProfile -Command \"curl.exe -s -o G:\\\\pa-s0-probe-out.txt https://example.com; \"'$code = $LASTEXITCODE; Write-Output \"ExitCode=$code\"; exit $code'","cwd":"G:\\项目1\\pocketagent"}}
[02:22:39.079] guardianWarning {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","message":"Automatic approval review approved (risk: low, authorization: unknown): Auto-review returned a low-risk allow decision."}
[02:22:39.085] item/autoApprovalReview/completed {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","startedAtMs":1789928555452,"completedAtMs":1789928559085,"reviewId":"042ef1a5-0327-4a60-9623-92d72fb586a3","targetItemId":"call_g1fSTXZ62hpzUIhq1rCwGXOW","decisionSource":"agent","review":{"status":"approved","riskLevel":"low","userAuthorization":"unknown","rationale":"Auto-review returned a low-risk allow decision."},"action":{"type":"command","source":"unifiedExec","command":"\"C:\\\\Program Files\\\\PowerShell\\\\7\\\\pwsh.exe\" -NoProfile -Command \"curl.exe -s -o G:\\\\pa-s0-probe-out.txt https://example.com; \"'$code = $LASTEXITCODE; Write-Output \"ExitCode=$code\"; exit $code'","cwd":"G:\\项目1\\pocketagent"}}
[02:22:40.626] item/commandExecution/outputDelta {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","itemId":"call_g1fSTXZ62hpzUIhq1rCwGXOW","delta":"ExitCode=0"}
[02:22:40.626] item/commandExecution/outputDelta {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","itemId":"call_g1fSTXZ62hpzUIhq1rCwGXOW","delta":"\r\n"}
[02:22:40.841] item/completed {"item":{"type":"commandExecution","id":"call_g1fSTXZ62hpzUIhq1rCwGXOW","pluginId":null,"scriptPath":null,"command":"\"C:\\\\Program Files\\\\PowerShell\\\\7\\\\pwsh.exe\" -NoProfile -Command \"curl.exe -s -o G:\\\\pa-s0-probe-out.txt https://example.com; \"'$code = $LASTEXITCODE; Write-Output \"ExitCode=$code\"; exit $code'","cwd":"G:\\项目1\\pocketagent","processId":"88904","source":"unifiedExecStartup","status":"completed","commandActions":[{"type":"unknown","command":"curl.exe -s -o G:\\pa-s0-probe-out.txt https://example.com; $code = $LASTEXITCODE; Write-Output \"ExitCode=$code\"; exit $code"}],"aggregatedOutput":"ExitCode=0\r\n","exitCode":0,"durationMs":1062},"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","completedAtMs":1789928560758}
[02:22:40.905] thread/tokenUsage/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","tokenUsage":{"total":{"totalTokens":11789,"inputTokens":11672,"cachedInputTokens":3712,"cacheWriteInputTokens":0,"outputTokens":117,"reasoni…
[02:22:40.906] account/rateLimits/updated {"rateLimits":{"limitId":"codex","limitName":null,"normalModelSlug":null,"primary":null,"secondary":null,"credits":null,"individualLimit":null,"spendControlReached":null,"planType":null,"rateLimitReachedType":null}}
[02:22:42.501] mcpServer/startupStatus/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","name":"codex_apps","status":"failed","error":"MCP client for `codex_apps` timed out after 30 seconds. Add or adjust `startup_timeout_sec` in your config.toml:\n[mcp_servers.codex_apps]\nst…
[02:22:48.671] item/started {"item":{"type":"agentMessage","id":"msg_0097f14998156cb7016ab02476c8b487d0a76086d624d23ba6","text":"","phase":"final_answer","memoryCitation":null,"delivery":null,"questions":null},"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","startedAtMs":1789928568671}
[02:22:49.733] item/completed {"item":{"type":"agentMessage","id":"msg_0097f14998156cb7016ab02476c8b487d0a76086d624d23ba6","text":"已通过 auto review 执行命令，退出码为 `0`。响应已写入 `G:\\pa-s0-probe-out.txt`。","phase":"final_answer","memoryCitation":null,"delivery":null,"questions":null},"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","completedAtMs":1789928569671}
[02:22:49.843] thread/tokenUsage/updated {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turnId":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","tokenUsage":{"total":{"totalTokens":23664,"inputTokens":23510,"cachedInputTokens":14464,"cacheWriteInputTokens":0,"outputTokens":154,"reason…
[02:22:49.843] account/rateLimits/updated {"rateLimits":{"limitId":"codex","limitName":null,"normalModelSlug":null,"primary":null,"secondary":null,"credits":null,"individualLimit":null,"spendControlReached":null,"planType":null,"rateLimitReachedType":null}}
[02:22:49.871] thread/status/changed {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","status":{"type":"idle"}}
[02:22:49.871] turn/completed {"threadId":"01a0c00d-e928-71c2-81f0-1547760d3ce0","turn":{"id":"01a0c00d-ea07-7b32-85eb-65efc8bb51ff","items":[{"type":"agentMessage","id":"msg_0097f14998156cb7016ab02476c8b487d0a76086d624d23ba6","text":"已通过 auto review 执行命令�…
```

## S0 结论(T-L1,如实记录)

**结论:`subAgentActivity` / `collabAgentToolCall` 两类 ThreadItem 在本轮 S0 实验中一次都未出现(4 轮共 0 条);
审批升级审查子代理实际走的载体是 `item/autoApprovalReview/started|completed` 通知 + `guardianWarning` 通知。**

- 审查子代理确实被拉起:approvalsReviewer="auto_review" 生效(需要同时压低沙箱:
  `approvalPolicy:"on-request"` + `sandbox:"read-only"`,否则命令直接放行、根本不进审批路径 —— 第 1、2 轮即此情形);
- 审查节奏:review started -> 约 3.6s 后 completed(status=approved, riskLevel=low, rationale 带"低风险放行"人话),
  同步伴随 `guardianWarning`(automatic approval review approved);
- 该载荷与 codex_guardian.go 已处理的 GuardianReview 家族**同构**(reviewId/targetItemId/review.status/riskLevel/rationale/action),
  daemon 侧已有消费路径(常驻钩子 -> recordGuardianNotification),并非静默;
- 对 0.149.1,`subAgentActivity`/`collabAgentToolCall` 预计只在**显式多代理协作**(collab 工具 spawnAgent 等被模型调用,
  或 collaboration mode 线程)时发射,本轮审批升级流不经过它们。此推测待后续以 collab 模式探针证伪/证实。

### 尝试记录(共 4 轮,均为真跑)

| 轮 | thread/start 参数 | prompt 要点 | 结果 |
|---|---|---|---|
| 1 | approvalsReviewer=auto_review | echo probe(工作区内) | turn/completed 32s,命令自动放行,无审批,子代理 item 0 条 |
| 2 | 同上 | curl 写 G:\ 根 + 联网 | turn/completed 40s,ExitCode=0 仍自动放行,无审批,子代理 item 0 条 |
| 3 | + approvalPolicy=on-request + sandbox=read-only | 同第 2 轮 | turn/completed 35s,**审查子代理拉起**:item/autoApprovalReview/started+completed、guardianWarning 各 1;子代理 ThreadItem 仍 0 条 |
| 4 | 同第 3 轮 | 同第 2 轮 | turn/completed 38s,同第 3 轮形状(本文件上方完整帧即第 4 轮) |

### 对姊妹方案 S1 proto 的修正建议

1. `sub_agent` 帧(如按 subAgentActivity/collabAgentToolCall 建模)**不能作为审批审查子代理的数据源** ——
   真值载体是 `item/autoApprovalReview/*`(GuardianApprovalReview:reviewId、targetItemId、
   review{status,riskLevel,userAuthorization,rationale}、action、decisionSource),proto 的审查事件字段应与此对齐
   (codex_guardian.go 既有 GuardianReview 记录已覆盖,仅 decisionSource 是新增字段);
2. subAgentActivity(agentPath/agentThreadId/kind)与 collabAgentToolCall(tool/status/senderThreadId/
   receiverThreadIds/agentsStates)字段建模保留,但应标注"仅 collab 模式可观测,S0 未捕获";
3. ThreadInfo.parentThreadId 已在 thread/started 应答里证实存在(本文件 thread/started 帧),L2 thread/read 拉子代理
   全量的路线可行。
