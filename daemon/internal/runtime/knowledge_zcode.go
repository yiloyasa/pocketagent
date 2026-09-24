// zcodeEventSummaries 是 zcode 方法的中文人话词表(键规则见 eventKnowledgeKey)。
// 按 agent 分文件:各 provider 适配只动自家词表,互不踩文件。

package runtime

var zcodeEventSummaries = map[string]string{
	"zcode\x1fmodel.streaming":               "ZCode 正在流式输出内容",
	"zcode\x1ftool.updated":                  "ZCode 更新了工具执行状态",
	"zcode\x1fturn.started":                  "ZCode 开始处理这一轮",
	"zcode\x1fturn.completed":                "ZCode 完成了这一轮任务",
	"zcode\x1fturn.failed":                   "ZCode 这一轮任务失败了",
	"zcode\x1fturn.steerQueued":              "ZCode 把一条新输入排进队列(当前轮结束后执行)",
	"zcode\x1fturn.steerDrained":             "ZCode 清空了排队中的插话输入",
	"zcode\x1finteraction/requestPermission": "ZCode 请求批准一项操作",
	"zcode\x1fstate.updated":                 "ZCode 同步了运行状态(daemon 已据此更新界面)",
	"zcode\x1fsession.updated":               "ZCode 同步了会话状态(daemon 已据此更新界面)",
	"zcode\x1fsettings.updated":              "ZCode 同步了设置变化(daemon 已据此更新界面)",
	"zcode\x1fsession.modelUpdated":          "ZCode 切换了会话使用的模型(daemon 已据此更新界面)",
}
