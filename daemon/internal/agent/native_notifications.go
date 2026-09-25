package agent

import (
	"encoding/json"
	"strings"
	"sync"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
)

// RuntimeSelection is a partial update. Missing is not the same as explicitly
// choosing an empty/default effort; opaque IDs are never guessed from labels.
type RuntimeSelection struct {
	ModelID string
	Effort  *string
}

func decodeRuntimeSelection(agentID, method string, raw json.RawMessage) RuntimeSelection {
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil {
		return RuntimeSelection{}
	}
	// Both standalone snapshots and wrapped notification snapshots are used by
	// supported native runtimes. Unknown siblings remain untouched.
	if inner := value["snapshot"]; len(inner) > 0 {
		return decodeRuntimeSelection(agentID, method, inner)
	}
	text := func(raw json.RawMessage) string {
		var s string
		_ = json.Unmarshal(raw, &s)
		return s
	}
	var result RuntimeSelection
	if agentID == "pi" {
		var model map[string]json.RawMessage
		_ = json.Unmarshal(value["model"], &model)
		if id, provider := text(model["id"]), text(model["provider"]); id != "" && provider != "" {
			result.ModelID = "pi/" + provider + "/" + id
		}
		level := value["thinkingLevel"]
		if method == "thinking_level_changed" {
			level = value["level"]
		}
		if len(level) > 0 {
			var s string
			if json.Unmarshal(level, &s) == nil {
				result.Effort = &s
			}
		}
	}
	if agentID == "zcode" {
		var settings struct {
			Model struct {
				Current struct {
					ModelID string `json:"modelId"`
					Options struct {
						ReasoningLevel *string `json:"reasoningLevel"`
					} `json:"options"`
				} `json:"current"`
			} `json:"model"`
			ThoughtLevel struct {
				Current *string `json:"current"`
			} `json:"thoughtLevel"`
		}
		if json.Unmarshal(value["settings"], &settings) == nil {
			if settings.Model.Current.ModelID != "" {
				result.ModelID = "zcode/" + settings.Model.Current.ModelID
			}
			result.Effort = settings.Model.Current.Options.ReasoningLevel
			// 思考档回退与 zcodeConfirmedSelection 同语义:0.16.9 实测
			// (docs/verify/evidence/zcode-0169-probe.log)setThoughtLevel 的
			// state.updated patch 里 model.current.options.reasoningLevel 与
			// thoughtLevel.current 同轴联动,但只换思考档时 thinking 载体在
			// settings.thoughtLevel —— model.current 缺失时从这里取,不丢档。
			if result.Effort == nil {
				result.Effort = settings.ThoughtLevel.Current
			}
		}
	}
	return result
}

func emitRuntimeSelection(selection RuntimeSelection, sink Sink) {
	if selection.ModelID == "" && selection.Effort == nil {
		return
	}
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Started{Started: &pb.TurnStarted{
		ModelId: selection.ModelID, Effort: selection.Effort, ConfigurationConfirmed: true,
	}}})
}

// Unknown notifications are not protocol failures. Preserve bounded, redacted
// evidence in the existing durable event stream without noisy warning UI.
type nativeNoticeRecorder struct {
	mu    sync.Mutex
	count map[string]int
	// threadID 是**我们自己的**会话 id(不是原生 agent 的 thread id),供诊断台账
	// 关联到具体会话。为空 = 这条通知产生在没有会话归属的路径上 —— 如实留空,
	// 不拿原生 id 冒充。
	threadID string
}

// recordProviderEvent is the shared event-envelope projection used by provider
// protocol boundaries. It records known projections/passthroughs as well as
// genuinely unknown values; the raw payload is always bounded and redacted.
func recordProviderEvent(agentID, method, threadID, kind string, raw json.RawMessage) {
	if kind == "" {
		kind = runtime.KindUnknownNotification
	}
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       threadID,
		SourceAgent:    agentID,
		SourceMethod:   method,
		NormalizedKind: kind,
		RawPayload:     redactNativePayload(raw),
	})
}

// codexItemNoticeKind 给"落到主线 A 透传的 codex item 类型"分档(规格 §4.6 的
// normalized_kind 词表)。
//
// 【为什么必须有它】record 曾把每条 notice 一律记成 UNKNOWN_NOTIFICATION,可
// codex 每轮都会产生 reasoning / userMessage / summary —— 它们也走这条路,于是
// 被算作"未知"。诊断页就此被已知噪音填满:上游升级后真发来怪东西时,它淹在一堆
// "其实我们认识"的条目里看不见,分类也就失去了意义(方案 §9.4 验收第 1 条)。
//
// 【判定依据是"我们做了什么",不是"类型名眼熟不眼熟"】
//   - reasoning:daemon 已把它投影成协议内的 ThinkingDelta(Run() 的
//     reasoning/delta 分支)→ KNOWN_PROJECTED;
//   - userMessage / summary:没有正式投影,只原样透传/留痕 → KNOWN_PASSTHROUGH;
//   - 其余(含空串、从未见过的类型名)→ UNKNOWN_NOTIFICATION,这才是"真没见过"。
//     空串归未知而非已知:拿不到类型名时最该做的是让人看见它,而不是替它编一个
//     "已知"身份把问题盖住。
//
// 【为什么白名单五类也落到 UNKNOWN_NOTIFICATION】contextCompaction /
// commandExecution / agentMessage / subAgentActivity / collabAgentToolCall 各有
// 专案翻译分支,isOpaqueItemType 已先把它们挡在本函数之前 —— 真走到这里说明调用
// 路径与我们的预期对不上。词表里只有 PROJECTED / PASSTHROUGH 两档"已知",它们
// 既不投影也不纯透传,硬套哪一档都是错误的描述;而把这种"不该发生"如实标成未知,
// 至少让它在诊断页上可见,比用一个错误的"已知"标签把异常藏起来好。
func codexItemNoticeKind(itemType string) string {
	switch itemType {
	case "reasoning":
		return runtime.KindKnownProjected
	case "userMessage", "summary":
		return runtime.KindKnownPassthrough
	}
	return runtime.KindUnknownNotification
}

// codexMethodNoticeKind 给 codex 的**会话级 method** 分档。
// (与按 item 类型分的 [codexItemNoticeKind] 是两条独立路径,别混。)
//
// 【为什么"已知 method 的空转"不等于未知】turn/diff/updated 在 diff 为空时、
// model/verification 在校验列表为空时,codexNotificationEvent 会返回 nil,于是落到
// 兜底记录。这是**已知 method 这次没东西可投影**,不是"上游发来没见过的 method"。
// 记成 UNKNOWN 的话,诊断页天天被它占着,真正的新 method 反而显不出来。
func codexMethodNoticeKind(method string) string {
	switch method {
	case "turn/diff/updated", "model/verification":
		return runtime.KindKnownPassthrough
	}
	return runtime.KindUnknownNotification
}

// dshNoticeKind 给 dsh 通知的 method 分档。
//
// 【判定依据是"daemon 对这个**类别**做了什么",不是名字眼熟不眼熟】:
//   - context_usage / tool_call 系列**正常情形已被投影**成 Usage / ToolCall 帧;
//     走到留痕说明是已知路径上的意外(认不出的 token 字段 / unmarshal 失败 / 状态
//     字段不认识)—— 标 PROJECTED,让诊断页把它与"真未知"分开;
//   - plan / plan_update / plan_removed 形状可投影时已直接发计划卡帧(成功路径
//     与 tool_call 同款:投影成功不落台账,见 dsh_plan.go);走到留痕说明是已知
//     路径上的意外(形状认不出/空计划),标 PASSTHROUGH 与"只透传"的旧口径一致;
//   - available_commands_update 只透传进诊断视图,没有正式事件 → PASSTHROUGH;
//   - 其余(未知变体、非 session/update 的方法) → UNKNOWN_NOTIFICATION。
//
// 【为什么未知变体不硬套 PASSTHROUGH】session/update 的变体是开放的:上游加一个新
// kind 就会落到 default。硬说"我们认识它"会让诊断页失去存在的意义 —— 那正是本轮要修的病。
func dshNoticeKind(method string) string {
	switch {
	case strings.HasPrefix(method, "session/update:context_usage"),
		strings.HasPrefix(method, "session/update:tool_call"):
		return runtime.KindKnownProjected
	case strings.HasPrefix(method, "session/update:plan"),
		strings.HasPrefix(method, "session/update:available_commands_update"):
		return runtime.KindKnownPassthrough
	}
	return runtime.KindUnknownNotification
}

// piNoticeKind 给 pi 的事件类型分档。
//
// 【agent_start 为什么算"已知"】pi 协议里有它(piapp/client.go 的 Event 注释列了),
// 探针证据也抓到过真实帧(docs/verify/evidence/pi-p0-probe.md);daemon 只是选择不把它
// 投影成 TurnServerMessage —— 这是**我们的取舍**,不是"没见过这个东西"。
// 【为什么不归 UNKNOWN】诊断页的意义是把"上游发来怪东西"从已知噪音里挑出来;
// 把已知事件记成未知会让它重新淹进噪音 —— 正是本轮要修的病。
func piNoticeKind(eventType string) string {
	switch eventType {
	case "agent_start", "message_update", "tool_execution_start",
		"tool_execution_update", "tool_execution_end", "agent_end",
		"agent_settled", "abort", "compaction_start", "compaction_end",
		"auto_retry_start", "auto_retry_end", "queue_update",
		"extension_ui_request":
		return runtime.KindKnownPassthrough
	}
	return runtime.KindUnknownNotification
}

// zcodeNoticeKind 给 zcode 的通知分档(顶层通知 method 与 session/event 的
// event.type 共用这一张表 —— 两者都汇进 recordAs/recordProviderEvent;全局钩子
// 路径的 session/event 已由 zcodeLedgerMethod 拆壳成 "session/event:<type>"
// 再进来)。
//
// 【0.16.9 实证结论(2026-09-24,docs/verify/evidence/zcode-0169-probe.log)】
// 模型/思考档变化在 0.16.9 stdio 上的唯一载体是顶层通知 state.updated
// (params={scope:"session", sessionId, revision, reason:"model_changed"|
// "thought_level_changed", patch:<settings 段>}),事件流(session/event)里
// settings.updated 与 session.modelUpdated 零出现,bundle 静态检索也是 0 次
// —— 0.16.5 探针确认的两个事件已消失。daemon 的模型同步链路没断:Run 的
// state.updated 分支把 patch 包成 {"settings":…} 后照样解码(zcode.go);
// 下面的 session/event 三键是旧 bundle(≤0.16.5)连过来时的兼容路径,处理
// 分支保留作死代码防护,分档维持 PROJECTED 与词表改注一致。
//
// 【判定依据仍是"daemon 对这个类别做了什么"】
//   - state.updated / session.updated / settings.updated / session.modelUpdated:
//     记录前已走 emitRuntimeSelection 投影成协议内 Started 帧 → PROJECTED;
//   - session/event 内部五类(model.streaming/tool.updated/turn.completed/
//     turn.failed/turn.started):Run 的专案分支把前四类投影成流式文本/思考帧、
//     工具卡、用量帧与失败原因,turn.started 只落一条日志 → PROJECTED /
//     PASSTHROUGH。全局钩子先于 Run 的轮次钩子对每条通知都记一次账(zcodeapp
//     先 global 后 notif),所以"Run 处理过了"不等于"全局路径记它就是路径
//     不符" —— 不收编它们,轮次期间每条事件都灌 UNKNOWN(尾巴 4 的病根)。
//   - session.titleUpdated:Run 内已有 TitleHandler 投影分支(桌面改名→手机
//     同步,zcode.go)。0.16.9 bundle 静态可见(payload 含 previousTitle/
//     source/title),探针未触发(改名需轮次或桌面操作,证据分级:静态)。
//   - 其余 PASSTHROUGH:已知、只留痕、不投影。其中 usage.delta/subagent.
//     lifecycle/turn.terminal 属桌面桥投影族词表(bundle 静态),不在 stdio
//     session/event 校验 union 内 —— 真出现也只留痕;daemon 的同类事实各有
//     自己的载体(turn.completed payload.usage、tool.updated、session/usage)。
//     process/resourceSample 与 process/mcpResourceSamples 是 0.16.9 实测
//     每分钟灌台账的遥测(0.16.9 上线两天记了 193 条 UNKNOWN,把真未知挤出
//     200 条滚动窗),收编为已知是本轮分档的直接动机。
//   - permission.requested / userInput.requested:与反向请求
//     interaction/requestPermission 的关系探针证不出(审批流需要真实轮次,
//     本轮红线禁跑),按"证不出就留 UNKNOWN(不猜)"处理。
func zcodeNoticeKind(method string) string {
	switch method {
	case "state.updated", "session.updated", "settings.updated", "session.modelUpdated",
		"session.titleUpdated",
		"model.streaming", "tool.updated", "turn.completed", "turn.failed":
		return runtime.KindKnownProjected
	case "turn.started",
		"turn.steerQueued", "turn.steerDrained",
		"session.created", "session.resumed", "session.closed",
		"checkpoint.created", "rewind.triggered",
		"usage.delta", "subagent.lifecycle", "turn.terminal",
		"process/resourceSample", "process/mcpResourceSamples",
		"process/mcpTelemetry", "startup/storageState":
		return runtime.KindKnownPassthrough
	}
	return runtime.KindUnknownNotification
}

// record 是"未分类"入口:不关心分档的调用方(pi / zcode / dsh 等)继续用它,
// 内部回落 UNKNOWN_NOTIFICATION —— 分类先从 codex item 路径落地,不让其余
// runtime 的调用点为此被迫改动。
func (r *nativeNoticeRecorder) record(agentID, method string, raw json.RawMessage, sink Sink) {
	r.recordAs("", agentID, method, raw, sink)
}

// recordAs 与 record 同一件事,额外接受分类结果 kind(规格 §9 的 normalized_kind)。
// kind 为空回落 UNKNOWN_NOTIFICATION:调用方没分类 = 我们确实不知道它是什么,
// 如实记未知,不冒充已知。
func (r *nativeNoticeRecorder) recordAs(kind, agentID, method string, raw json.RawMessage, sink Sink) {
	r.recordLedger(kind, agentID, method, raw)

	r.mu.Lock()
	if r.count == nil {
		r.count = map[string]int{}
	}
	if r.count[method] >= 2 || len(r.count) >= 64 {
		r.mu.Unlock()
		return
	}
	r.count[method]++
	r.mu.Unlock()
	sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
		Kind: pb.CodexEvent_STATUS, Code: "native_notification", Title: agentID + " · " + method,
		Message: "原生状态通知", RawJson: redactNativePayload(raw),
	}}})
}

// recordLedger 只写诊断台账,不向 sink 推任何帧(recordAs 的台账半边;推送的
// 限频语义只属于 recordAs 自己 —— 台账是索引,必须记全真实频次,推送才是被
// 限频的对象)。
func (r *nativeNoticeRecorder) recordLedger(kind, agentID, method string, raw json.RawMessage) {
	if kind == "" {
		kind = runtime.KindUnknownNotification
	}
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       r.threadID,
		SourceAgent:    agentID,
		SourceMethod:   method,
		NormalizedKind: kind,
		RawPayload:     redactNativePayload(raw),
	})
}

// recordLedgerOnly 只落台账、不推 sink:给"已知但每轮必来、推给手机纯属噪音"
// 的事件用(现役只有 pi 0.87.1 的 turn_start/message_start/message_end,见
// pi.go 的 piLedgerOnlyEvent)。分类语义与 recordAs 完全一致(kind 为空回落
// UNKNOWN),缺的只是推送那半边。
func (r *nativeNoticeRecorder) recordLedgerOnly(kind, agentID, method string, raw json.RawMessage) {
	r.recordLedger(kind, agentID, method, raw)
}

func redactNativePayload(raw json.RawMessage) string {
	if len(raw) > 32*1024 {
		return `{"diagnostic":"native payload exceeded 32KiB; omitted"}`
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return `{"diagnostic":"invalid native JSON; omitted"}`
	}
	var redact func(any)
	redact = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, value := range x {
				key := strings.ToLower(strings.ReplaceAll(k, "_", ""))
				switch key {
				case "apikey", "authorization", "password", "secret", "accesstoken", "refreshtoken", "token", "cookie":
					x[k] = "[redacted]"
				default:
					redact(value)
				}
			}
		case []any:
			for _, item := range x {
				redact(item)
			}
		}
	}
	redact(v)
	out, _ := json.Marshal(v)
	if len(out) > 8000 {
		return `{"diagnostic":"redacted native payload exceeded 8KiB; omitted"}`
	}
	return string(out)
}
