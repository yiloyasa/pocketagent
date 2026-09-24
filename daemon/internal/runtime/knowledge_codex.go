// codexEventSummaries 是 codex 方法的中文人话词表(键规则见 eventKnowledgeKey)。
// 按 agent 分文件:各 provider 适配只动自家词表,互不踩文件。

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
}
