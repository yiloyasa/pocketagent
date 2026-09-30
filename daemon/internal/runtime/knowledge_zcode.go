// zcodeEventSummaries 是 zcode 方法的中文人话词表(键规则见 eventKnowledgeKey)。
// 按 agent 分文件:各 provider 适配只动自家词表,互不踩文件。
//
// 【0.16.9 实证校准(2026-09-24,docs/verify/evidence/zcode-0169-probe.log)】
// settings.updated / session.modelUpdated 在 0.16.9 已消失(事件流零出现 +
// bundle 静态 0 次),模型/思考档变化改由顶层通知 state.updated 携带
// (reason="model_changed"/"thought_level_changed")。两键保留并改注:旧
// bundle(≤0.16.5)连过来时分类与处理分支仍是活的,人话不能缺;新词表键
// 全部对应 zcodeNoticeKind 的 PASSTHROUGH 收编批与 stdio union 新面孔。
//
// 【2026-09-30 协议复验补齐(zcprobe 全子模式重跑,bundle 自报版本仍 0.16.9,
// 文件 mtime 2026-09-29;union 全集取自 bundle 静态 zod 枚举 + 基线/当日实证)】
//  1. 台账复合键:全局钩子把 session/event 拆壳登记成 "session/event:<type>"
//     (agent.zcodeLedgerMethod),翻译层按 SourceMethod 原样查词表 —— 此前只有
//     平键,轮次事件在诊断页全部掉进"未知事件"。逐条补上复合键,措辞与平键
//     一致。session/event:session.updated 按 0.16.9 实测载荷措辞(它实际携带
//     model_request_started/model_request_failed 遥测;与 settings.updated/
//     session.modelUpdated 的消失结论不矛盾,那两个键 bundle 静态确为 0 次)。
//  2. union 新面孔(message.*/part.*/streamRecovery.updated):bundle 静态在
//     stdio session/event union(zod 枚举)内,本轮前无词表条目,真出现即灌
//     UNKNOWN。它们是桌面端记录流的镜像粒度,daemon 的 Run 链不投影(流式
//     正文唯一来源仍是 model.streaming,0.16.5 探针实证,daemon 稳定消费中)。
//  3. 顶层通知补收(v4/telemetry/event、computer-use/operation-event):
//     0.16.9 探针(2026-09-25 zc-subagents-probe.log ×8/×4)与本次复验都在发,
//     一次委派轮 8 条遥测,当时收编批漏掉,一直灌 UNKNOWN。
//  4. interaction/requestOfficialMcpAuthHeaders:每次 session/create 服务器都
//     反问官方 MCP 鉴权头(桌面端职责),daemon 一律安全拒绝(-32601)并记台账;
//     给一条人话让"已知拒绝"在诊断页可识别。
//  5. 刻意不收:permission.requested/permission.resolved/userInput.requested/
//     userInput.resolved 四件套(union 在,但投影形状需带审批的真实轮次实证,
//     维持遗留⑤"证不出就留 UNKNOWN(不猜)")。

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

	// ---- 2026-09-30 复验补齐:台账复合键(session/event 拆壳后的登记名)----
	"zcode\x1fsession/event":                        "ZCode 发来一条拆不开壳的会话事件(信封异常,请看原文)",
	"zcode\x1fsession/event:turn.started":           "ZCode 开始处理这一轮",
	"zcode\x1fsession/event:turn.completed":         "ZCode 完成了这一轮任务",
	"zcode\x1fsession/event:turn.failed":            "ZCode 这一轮任务失败了",
	"zcode\x1fsession/event:turn.steerQueued":       "ZCode 把一条新输入排进队列(当前轮结束后执行)",
	"zcode\x1fsession/event:turn.steerDrained":      "ZCode 清空了排队中的插话输入",
	"zcode\x1fsession/event:model.streaming":        "ZCode 正在流式输出内容",
	"zcode\x1fsession/event:tool.updated":           "ZCode 更新了工具执行状态",
	"zcode\x1fsession/event:session.updated":        "ZCode 上报了模型请求生命周期遥测(daemon 另有投影,此处留痕)",
	"zcode\x1fsession/event:session.titleUpdated":   "ZCode 更新了会话标题(daemon 已同步到手机)",
	"zcode\x1fsession/event:session.created":        "ZCode 创建了会话(daemon 主动管理,此为回执留痕)",
	"zcode\x1fsession/event:session.resumed":        "ZCode 恢复了会话(daemon 主动管理,此为回执留痕)",
	"zcode\x1fsession/event:session.closed":         "ZCode 关闭了会话(daemon 主动管理,此为回执留痕)",
	"zcode\x1fsession/event:checkpoint.created":     "ZCode 创建了会话检查点",
	"zcode\x1fsession/event:rewind.triggered":       "ZCode 执行了会话回挡",
	"zcode\x1fsession/event:settings.updated":       "ZCode 同步了设置变化(旧版桌面端载体,daemon 已据此更新界面)",
	"zcode\x1fsession/event:session.modelUpdated":   "ZCode 切换了会话使用的模型(旧版桌面端载体,daemon 已据此更新界面)",
	"zcode\x1fsession/event:message.upserted":       "ZCode 写入了一条会话消息记录(桌面端记录流,daemon 只留痕)",
	"zcode\x1fsession/event:message.removed":        "ZCode 移除了一条会话消息记录(桌面端记录流,daemon 只留痕)",
	"zcode\x1fsession/event:part.started":           "ZCode 开始了一条消息分段(桌面端记录流,daemon 只留痕)",
	"zcode\x1fsession/event:part.delta":             "ZCode 正在流式更新一条消息分段(记录流增量,正文仍以流式投影为准)",
	"zcode\x1fsession/event:part.upserted":          "ZCode 更新了一条消息分段(桌面端记录流,daemon 只留痕)",
	"zcode\x1fsession/event:part.removed":           "ZCode 移除了一条消息分段(桌面端记录流,daemon 只留痕)",
	"zcode\x1fsession/event:streamRecovery.updated": "ZCode 上报了流式恢复状态(断流重连等,daemon 只留痕)",

	// ---- 2026-09-30 复验补齐:顶层通知与已知拒绝 ----
	"zcode\x1fv4/telemetry/event":                        "ZCode 上报了内核遥测(模型请求状态/轮次终态等诊断数据,daemon 只留痕)",
	"zcode\x1fcomputer-use/operation-event":              "ZCode 上报了桌面端轮次操作镜像事件(诊断数据,daemon 只留痕)",
	"zcode\x1finteraction/requestOfficialMcpAuthHeaders": "ZCode 反问官方 MCP 鉴权头(桌面端职责,daemon 已按协议错误安全拒绝)",
}
