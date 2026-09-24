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
// 【明确不收清单】——桌面产品面通知,手机端没有消费方,别再翻一遍了:
// thread/realtime/*(语音实时会话)、app/*(桌面应用管理)、fs/changed
// (桌面文件监视)、remoteControl/*(远程控制)、externalAgentConfig/*(桌面
// 导入)、fuzzyFileSearch/*(桌面模糊搜索)、windowsSandbox/*(Windows 沙箱
// 安装向导)、windows/worldWritableWarning、project/changed。
// 0.156.1 新增的请求面方法(userVerification/*、project/*、memory/*、
// environment/*、account/bedrock/*、process/spawn、mock/experimentalMethod、
// server/diagnostics 等)同属桌面/探针面,同样不收;完整 diff 见上述证据日志。
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
	"codex\x1fthread/plan/updated":        "Codex 更新了本会话的执行计划",
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
}
