// zcodeEventSummaries 是 zcode 方法的中文人话词表(键规则见 eventKnowledgeKey)。
// 按 agent 分文件:各 provider 适配只动自家词表,互不踩文件。
//
// 【0.16.9 实证校准(2026-09-24,docs/verify/evidence/zcode-0169-probe.log)】
// settings.updated / session.modelUpdated 在 0.16.9 已消失(事件流零出现 +
// bundle 静态 0 次),模型/思考档变化改由顶层通知 state.updated 携带
// (reason="model_changed"/"thought_level_changed")。两键保留并改注:旧
// bundle(≤0.16.5)连过来时分类与处理分支仍是活的,人话不能缺;新词表键
// 全部对应 zcodeNoticeKind 的 PASSTHROUGH 收编批与 stdio union 新面孔。

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
	// 0.16.9 上状态/模型/思考档变化的实测载体(daemon 已据此投影更新界面)。
	"zcode\x1fstate.updated": "ZCode 同步了会话状态或模型/思考档变化(daemon 已据此更新界面)",
	// 以下两键 0.16.9 已消失,仅为旧版桌面端(≤0.16.5)兼容保留。
	"zcode\x1fsession.updated":      "ZCode 同步了会话状态(daemon 已据此更新界面)",
	"zcode\x1fsession.modelUpdated": "ZCode 切换了会话使用的模型(0.16.9 起并入 state.updated,此为旧版兼容)",
	"zcode\x1fsettings.updated":     "ZCode 同步了设置变化(0.16.9 起并入 state.updated,此为旧版兼容)",
	// 0.16.9 收编批:stdio union 新面孔与实测高频遥测,只留痕不投影。
	"zcode\x1fsession.titleUpdated":       "ZCode 更新了会话标题(daemon 已同步到手机)",
	"zcode\x1fsession.created":            "ZCode 创建了会话(daemon 主动管理,此为回执留痕)",
	"zcode\x1fsession.resumed":            "ZCode 恢复了会话(daemon 主动管理,此为回执留痕)",
	"zcode\x1fsession.closed":             "ZCode 关闭了会话(daemon 主动管理,此为回执留痕)",
	"zcode\x1fcheckpoint.created":         "ZCode 创建了会话检查点",
	"zcode\x1frewind.triggered":           "ZCode 执行了会话回挡",
	"zcode\x1fusage.delta":                "ZCode 上报了用量增量(daemon 在轮次结束时汇总展示用量)",
	"zcode\x1fsubagent.lifecycle":         "ZCode 子代理生命周期变化(工具卡片中查看产出)",
	"zcode\x1fturn.terminal":              "ZCode 这一轮到达终态",
	"zcode\x1fprocess/resourceSample":     "ZCode 上报了进程资源采样(诊断数据,每分钟一条)",
	"zcode\x1fprocess/mcpResourceSamples": "ZCode 上报了 MCP 资源采样(诊断数据)",
	"zcode\x1fprocess/mcpTelemetry":       "ZCode 上报了 MCP 进程遥测(诊断数据)",
	"zcode\x1fstartup/storageState":       "ZCode 存储初始化进度(启动阶段,很快结束)",
}
