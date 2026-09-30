// dsh 前缀词表与精确词表:dsh 通知 method 形如 session/update:tool_call:completed,
// 状态后缀开放,逐个枚举追不上上游 —— 前缀匹配最长优先;无后缀方法进精确表。

package runtime

// 状态后缀是开放的;逐个枚举状态值会让词表永远追不上上游。
var knownEventPrefixes = []struct {
	agent, methodPrefix, summary string
}{
	{"dsh", "session/update:agent_message_chunk", "dsh 正在输出助手消息(流式)"},
	{"dsh", "session/update:agent_thought_chunk", "dsh 正在输出思考过程"},
	{"dsh", "session/update:user_message_chunk", "dsh 回显了用户消息片段"},
	{"dsh", "session/update:context_usage", "dsh 报告了上下文用量(daemon 已投影为用量帧)"},
	{"dsh", "session/update:usage_update", "dsh 报告了上下文窗口占用(daemon 已投影为用量帧)"},
	{"dsh", "session/update:config_option_update", "dsh 更新了配置目录(daemon 已刷新原生设置缓存)"},
	{"dsh", "session/update:tool_call", "dsh 更新了工具调用状态(daemon 已投影为工具卡片)"},
	{"dsh", "session/update:plan", "dsh 更新了任务计划(daemon 已投影为计划卡)"},
	{"dsh", "session/update:available_commands_update", "dsh 更新了可用命令列表(daemon 只透传留痕)"},
}

// dshEventSummaries 是 dsh 精确方法词表(当前为空:通知带状态后缀走前缀表;
// 上游出现无后缀方法时在这里加行)。
var dshEventSummaries = map[string]string{}
