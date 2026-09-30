// piEventSummaries 是 pi 方法的中文人话词表(键规则见 eventKnowledgeKey)。
// 按 agent 分文件:各 provider 适配只动自家词表,互不踩文件。

package runtime

var piEventSummaries = map[string]string{
	"pi\x1fsession_info_changed":  "pi 更新了会话信息(如标题)",
	"pi\x1fmessage_update":        "pi 正在推送消息内容(流式输出)",
	"pi\x1fagent_start":           "pi 开始处理任务",
	"pi\x1fagent_end":             "pi 结束了这一轮处理",
	"pi\x1fagent_settled":         "pi 完成了这一轮的收尾工作",
	"pi\x1ftool_execution_start":  "pi 开始执行一个工具",
	"pi\x1ftool_execution_update": "pi 的工具执行有了新进展",
	"pi\x1ftool_execution_end":    "pi 的工具执行结束了",
	// pi\x1fabort 词条保留作兜底:0.99.1 起上游事件全集已无 abort 事件
	// (2026-09-30 piprobe 实证),留着防回滚旧版 pi 时 abort 帧落 UNKNOWN 淹诊断页。
	"pi\x1fabort":                "pi 中止了当前操作",
	"pi\x1fcompaction_start":     "pi 开始压缩上下文",
	"pi\x1fcompaction_end":       "pi 完成了上下文压缩",
	"pi\x1fauto_retry_start":     "pi 遇到问题,正在自动重试",
	"pi\x1fauto_retry_end":       "pi 的自动重试结束了",
	"pi\x1fqueue_update":         "pi 的消息排队状态有变化",
	"pi\x1fextension_ui_request": "pi 的扩展请求界面交互(由 daemon 守护栏处理)",
	// ---- 0.86/0.87 新增事件(pi 0.87.1 探针实证 + bundle emit 点核对,
	// 证据 docs/verify/evidence/pi-087-usage-probe.log)----
	"pi\x1fturn_start":                        "pi 开始新的一轮处理",
	"pi\x1fturn_end":                          "pi 完成了这一轮对话",
	"pi\x1fmessage_start":                     "pi 开始组织一条消息",
	"pi\x1fmessage_end":                       "pi 写完了一条消息",
	"pi\x1fentry_appended":                    "pi 往会话记录里写入了一条条目",
	"pi\x1fsummarization_retry_scheduled":     "pi 的上下文摘要没成功,安排了自动重试",
	"pi\x1fsummarization_retry_attempt_start": "pi 正在重试上下文摘要",
	"pi\x1fsummarization_retry_finished":      "pi 的上下文摘要重试结束了",
	"pi\x1fextension_error":                   "pi 的一个扩展出了错(通常不影响主流程,重复出现再排查)",
	// ---- 0.99.1 增补(2026-09-30 探针实证 + 随包 docs/json.md 事件全集)----
	"pi\x1fbash_execution_update":  "pi 的 bash 命令正在流式回传输出(仅 RPC bash 命令会触发)",
	"pi\x1fthinking_level_changed": "pi 切换了思考强度",
}
