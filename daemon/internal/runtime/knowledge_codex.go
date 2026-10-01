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
// (桌面文件监视)、remoteControl/*(远程控制;status/changed 已于 2026-10-02
// 单独收编,见上方)、externalAgentConfig/*(桌面导入)、fuzzyFileSearch/*(桌面
// 模糊搜索)、windowsSandbox/*(Windows 沙箱安装向导)、project/changed 与
// thread/project/updated(桌面 Projects 面:全局项目变更与会话↔项目绑定)、
// account/gatewayOAuth/*(桌面 OAuth 网关)。
// windows/worldWritableWarning 原列于此清单,2026-10-02 移出:codex_events_ext.go
// 自始就有 SANDBOX_WARNING 投影分支,"手机端没有消费方"的前提不成立,已按
// PROJECTED 收编(见下方同日词条与 codex.go 的 codexGlobalNoticeKinds)。
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
//
// 【2026-10-01 通宵采样批】codexliveturnprobe 活体单轮(0.159.2)通知序列 46 条
// 逐一对表:轮次骨架族(warning、turn/started|completed、item/started|completed、
// item/agentMessage 与 reasoning 摘要与命令输出的 delta、thread/tokenUsage/
// updated、account/rateLimits/updated、mcpServer/startupStatus/updated、
// serverRequest/resolved)全局钩子路径此前不认识,一次最小轮就能灌 ~39 条
// UNKNOWN —— 分类与词表同批补齐(见 codexGlobalNoticeKind 与下方同日词条)。
// 同轮 dsh/pi 真实轮、zcode 控制面/设置面复跑均无新 UNKNOWN 来源(zcode 受限:
// 真实轮因本机凭据 401 不可跑,如实受限)。
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
	// 2026-10-02 收编:载荷 {status: enabled|disabled, serverName},远程控制开关启停
	"codex\x1fremoteControl/status/changed":    "Codex 桌面端的远程控制开关状态变了(开启/关闭),不影响手机端的连接",
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

	// ---- 2026-10-01 通宵采样批(codexliveturnprobe 活体单轮实证,0.159.2)----
	// 轮次骨架通知族:全局钩子路径此前不认识这些 method,一次最小轮 46 条通知
	// 里 ~39 条灌 UNKNOWN(分类与词表同批补齐,见 codexGlobalNoticeKind 与下方
	// 同日词条;warning 与 serverRequest/resolved 词条更早已有,此处只补其余十条)。
	"codex\x1fturn/started":                      "Codex 已接收并开始执行这一轮任务",
	"codex\x1fturn/completed":                    "Codex 完成了这一轮任务",
	"codex\x1fitem/started":                      "Codex 开始处理一个条目(命令/压缩/子代理等,手机端出对应卡片)",
	"codex\x1fitem/completed":                    "Codex 完成了一个条目(结果已投影到对应卡片或终稿)",
	"codex\x1fitem/agentMessage/delta":           "Codex 正在流式输出回复正文",
	"codex\x1fitem/reasoning/summaryTextDelta":   "Codex 正在流式输出思考摘要(已投影为思考流)",
	"codex\x1fitem/commandExecution/outputDelta": "Codex 的命令正在回传执行输出(已投影为工具卡进度)",
	"codex\x1fthread/tokenUsage/updated":         "Codex 上报了 token 用量(已投影为用量帧与用量卡)",
	"codex\x1faccount/rateLimits/updated":        "Codex 上报了套餐用量限制(已投影为限额卡)",
	"codex\x1fmcpServer/startupStatus/updated":   "一个 MCP 服务的启动状态有变化(已投影为状态卡)",

	// ---- 2026-10-02 深接入差距修复批(证据:vendored 0.159.2 ServerNotification
	// 逐名 diff + daemon 事件翻译层逐名核对;活体轮探针日志
	// docs/verify/evidence/codex-01592-liveturn-20261002.log)----
	// 这 14 个方法 daemon 事件翻译层早有投影分支(codex_events.go /
	// codex_events_ext.go / Run 的 changeFiles 累积),但分类表此前漏登记、词表也
	// 没词条:轮次外到达灌 UNKNOWN、真到达时诊断页只能给"未知事件"兜底。同批
	// 已在 codexGlobalNoticeKinds 补登记(全部 PROJECTED),此处补人话;其中
	// error 与 guardianWarning 词条早已有,不再重复。
	"codex\x1fitem/autoApprovalReview/started":         "审查者开始评估 agent 想做的动作",
	"codex\x1fitem/autoApprovalReview/completed":       "审查者给出了评估结论(手机端出审查卡)",
	"codex\x1fautoApprovalReview/strictReviewRequired": "接下来的动作会被逐个送审后再执行",
	"codex\x1fitem/plan/delta":                         "Codex 正在生成执行计划(已投影为计划卡)",
	"codex\x1fcommand/exec/outputDelta":                "一条命令正在回传执行输出(旧版协议名,已投影为工具卡进度)",
	"codex\x1fprocess/outputDelta":                     "一个后台进程正在回传输出(已投影为工具卡进度)",
	"codex\x1fitem/fileChange/outputDelta":             "文件改动的执行输出正在回传(已投影为状态卡)",
	"codex\x1fitem/fileChange/patchUpdated":            "Codex 更新了文件改动清单(已投影为审批卡的文件列表)",
	"codex\x1fitem/mcpToolCall/progress":               "一个 MCP 工具调用有进度更新(已投影为状态卡)",
	"codex\x1fitem/reasoning/textDelta":                "Codex 正在流式输出思考过程(已投影为思考流)",
	"codex\x1fmodel/rerouted":                          "Codex 把任务切换到了另一个模型(手机端出切换卡)",
	"codex\x1fmodel/safetyBuffering/updated":           "模型正在安全缓冲,回复可能稍慢",
	"codex\x1fthread/compacted":                        "会话上下文已压缩,后续对话使用更紧凑的上下文",
	"codex\x1fwindows/worldWritableWarning":            "电脑上有目录对所有用户可写,沙箱保护会打折扣",
}
