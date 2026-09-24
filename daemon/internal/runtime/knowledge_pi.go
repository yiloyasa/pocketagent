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
	"pi\x1fabort":                 "pi 中止了当前操作",
	"pi\x1fcompaction_start":      "pi 开始压缩上下文",
	"pi\x1fcompaction_end":        "pi 完成了上下文压缩",
	"pi\x1fauto_retry_start":      "pi 遇到问题,正在自动重试",
	"pi\x1fauto_retry_end":        "pi 的自动重试结束了",
	"pi\x1fqueue_update":          "pi 的消息排队状态有变化",
	"pi\x1fextension_ui_request":  "pi 的扩展请求界面交互(由 daemon 守护栏处理)",
}
