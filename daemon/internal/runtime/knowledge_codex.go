// codexEventSummaries 是 codex 方法的中文人话词表(键规则见 eventKnowledgeKey)。
// 按 agent 分文件:各 provider 适配只动自家词表,互不踩文件。
//
// 【0.156.1 收编批】(2026-09,证据 docs/verify/evidence/codex-0156-probe.log)
// daemon 基线协议面是 0.155.1 vendored schema(95 请求 + 75 通知);0.156.1
// 通知面无消失(二进制逐名全命中),以下 13 条是此批收编的"daemon 代码此前
// 零引用"通知,分类一律 KNOWN_PASSTHROUGH(codex.go 的 codexGlobalNoticeKind):
// 其中 configWarning 在 0.156.1 握手窗口实测到真实帧(本机 config.toml 有
// 被忽略的设置项),词表文案与实测语义对齐。
//
// 【0.159.2 收编批】(2026-09-30,证据:codexplanprobe/codexsurfaceprobe/
// codexliveturnprobe 活体实测 + `codex app-server generate-json-schema` 重导出
// diff;快照已刷新至 0.159.2)。通知面 75 → 85(零消失),新增 8 条里只收编
// 2 + 1 条:authRecovery 两连(模型服务登录态自动恢复,能解释"轮次莫名失败")
// 与 item/commandExecution/terminalInteraction(0.159.2 起有配套的 writeStdin
// 审批卡,见 codex_requests.go,从"已知但未登记"补登)。其余新通知不动:
// thread/realtime/item/*(语音实时,桌面面)、account/gatewayOAuth/changed
// (桌面 OAuth 网关,配套请求同样不收)、mcpServer/event/stream/notification
// (桌面 MCP 流,留观)、thread/attachment/updated(daemon 无附件特性;
// 真到来会如实落 UNKNOWN 透析,不冒充已知)。
//
// 【明确不收清单】——桌面产品面通知,手机端没有消费方,别再翻一遍了:
// thread/realtime/*(语音实时会话,0.159.2 起含 item/started|transcript/delta|
// item/completed 三条正式 schema 通知)、app/*(桌面应用管理)、fs/changed
// (桌面文件监视)、remoteControl/*(远程控制)、externalAgentConfig/*(桌面
// 导入)、fuzzyFileSearch/*(桌面模糊搜索)、windowsSandbox/*(Windows 沙箱
// 安装向导)、windows/worldWritableWarning、project/changed、
// account/gatewayOAuth/*(桌面 OAuth 网关)。
// 0.156.1/0.159.2 新增的请求面方法(gatewayOAuth/*、userVerification/*、
// project/*、memory/*、environment/*、account/bedrock/*、process/spawn、
// mock/experimentalMethod、server/diagnostics、thread/attachment/*、
// plugin/reconcile 等)同属桌面/探针面,同样不收;完整 diff 见
// docs/2026-09-30-agent-capability-matrix.md 的 codex 0.159.2 适配记录。
//
// 【2026-09-30 未知台账积压收编批】活跃台账 200 条滚动窗实测(本机
// AppData\Local\PocketAgent\data\unknown-events.json,seq=1289):UNKNOWN 只剩
// 三组 —— codex "response" 迟到错误应答 24 条(按 error.message 两家族收编为
// "response:account/rateLimits/read" / "response:account/usage/read",见
// codex.go 的 codexLateResponseLedger)、dsh 裸 "session/update" 13 条(全是
// 认识的变体,dsh.go 全局钩子拆壳 + dshNoticeKind 补档后消灭)、remoteControl/
// status/changed 2 条(属上方"明确不收清单"的 remoteControl/*,维持不收,
// 如实留 UNKNOWN)。
package runtime

var codexEventSummaries = map[string]string{
	"codex\x1fturn.completed":             "Codex 完成了这一轮任务",
	"codex\x1fturn.failed":                "Codex 这一轮任务失败了",
	"codex\x1fcontextCompaction":          "Codex 正在压缩上下文,给后续对话腾出空间",
	"codex\x1ftoken_count":                "Codex 报告了本轮 token 用量",
	"codex\x1fsession_configured":         "Codex 完成了会话初始化(模型与工作目录已确认)",
	"codex\x1ferror":                      "Codex 上报了一次错误",
	"codex\x1fwarning":                    "Codex 发出了一条警告提示",
	"codex\x1fitem/completed:userMessage": "Codex 回显了你发过的那条消息(仅留痕,不产生新内容)",
	"codex\x1fitem/completed:summary":     "Codex 记录了这段会话的摘要",
	"codex\x1fitem/started:reasoning":     "Codex 输出了一段思考过程(已投影为思考流)",
	"codex\x1fitem/completed:reasoning":   "Codex 输出了一段思考过程(已投影为思考流)",
	"codex\x1fthread/started":             "Codex 开启了一个新会话",
	"codex\x1fthread/archived":            "Codex 把一个会话归档了",
	"codex\x1fthread/unarchived":          "Codex 把一个会话从归档中恢复了",
	"codex\x1fthread/deleted":             "Codex 删除了一个会话",
	"codex\x1fthread/reverted":            "Codex 把会话回退到了较早的状态",
	"codex\x1fthread/closed":              "Codex 关闭了一个会话",
	"codex\x1fthread/name/updated":        "Codex 更新了会话标题",
	"codex\x1fthread/queue/changed":       "Codex 的任务排队发生了变化",
	"codex\x1fthread/status/changed":      "Codex 的会话状态变化了(如空闲/运行中)",
	"codex\x1fturn/plan/updated":          "Codex 更新了本会话的执行计划",
	"codex\x1fthread/goal/updated":        "Codex 更新了本会话的目标",
	"codex\x1fthread/goal/cleared":        "Codex 清除了本会话的目标",
	"codex\x1fskills/changed":             "Codex 的可用技能列表发生了变化",
	"codex\x1fturn/diff/updated":          "Codex 更新了本轮的代码改动汇总",
	"codex\x1fmodel/verification":         "Codex 上报了模型校验信息",
	"codex\x1fguardianWarning":            "Codex 的安全防护机制发出了警告",
	"codex\x1fserverRequest/resolved":     "Codex 桌面端已处理完一个反向请求",

	// ---- 0.156.1 收编批(证据与不收清单见文件头注释)----
	"codex\x1fthread/environment/connected":    "Codex 的执行环境已接通,可以开始执行命令了",
	"codex\x1fthread/environment/disconnected": "Codex 的执行环境掉线了,可能影响后续命令执行",
	"codex\x1fthread/settings/updated":         "这个会话的设置被修改了(通常是桌面端改的),以最新设置为准",
	"codex\x1fconfigWarning":                   "Codex 的配置文件有问题,已忽略无效项继续运行",
	"codex\x1fdeprecationNotice":               "Codex 提示某个配置将来会弃用,建议按提示迁移",
	"codex\x1fitem/reasoning/summaryPartAdded": "Codex 输出了一段新的思考摘要",
	"codex\x1fturn/moderationMetadata":         "Codex 上报了本轮内容安全审核的元数据",
	"codex\x1faccount/updated":                 "Codex 的账号信息有变化(如套餐或登录方式)",
	"codex\x1faccount/login/completed":         "Codex 的登录流程结束了",
	"codex\x1fmcpServer/oauthLogin/completed":  "一个 MCP 服务的授权登录结束了",
	"codex\x1fprocess/exited":                  "Codex 拉起的一个后台进程退出了",
	"codex\x1fhook/started":                    "Codex 开始执行一个生命周期钩子",
	"codex\x1fhook/completed":                  "Codex 执行完一个生命周期钩子",

	// ---- 0.159.2 收编批(证据与不收清单见文件头注释)----
	"codex\x1fmodelProvider/authRecoveryStarted":         "模型服务的登录状态出了问题,Codex 正在自动恢复",
	"codex\x1fmodelProvider/authRecoveryCompleted":       "模型服务登录状态的自动恢复已完成",
	"codex\x1fitem/commandExecution/terminalInteraction": "有一条命令在等终端输入(比如密码或确认提示)",

	// ---- 2026-09-30 未知台账积压收编批(证据:活跃台账 200 条滚动窗实测)----
	// 能力探测轮询超时放弃后,app-server 迟到补发的错误应答(codex.go 的
	// codexLateResponseLedger 按 error.message 精确短语归家族,登记名带请求方法)。
	"codex\x1fresponse:account/rateLimits/read": "Codex 补报了一次限额数据拉取失败(对应的轮询早已超时降级,此处仅留痕)",
	"codex\x1fresponse:account/usage/read":      "Codex 补报了一次累计用量拉取失败(对应的轮询早已超时降级,此处仅留痕)",
}
