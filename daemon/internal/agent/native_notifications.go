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
		}
		if json.Unmarshal(value["settings"], &settings) == nil {
			if settings.Model.Current.ModelID != "" {
				result.ModelID = "zcode/" + settings.Model.Current.ModelID
			}
			result.Effort = settings.Model.Current.Options.ReasoningLevel
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
	if kind == "" {
		kind = runtime.KindUnknownNotification
	}
	// 【顺序有意为之】先登记诊断台账,再过下面的推送限频。
	// 推送限频(每方法 2 条)是为了不让手机端被同一条未知通知刷屏;而台账是**索引**,
	// 必须反映真实频次 —— 否则用户点开诊断页只看到两条,分不清"偶尔来一次"还是
	// "每轮都在来",那正是这个功能要回答的问题。
	sharedRuntimeBackend().UnknownEvents().Record(runtime.EventEnvelope{
		ThreadID:       r.threadID,
		SourceAgent:    agentID,
		SourceMethod:   method,
		NormalizedKind: kind,
		RawPayload:     redactNativePayload(raw),
	})

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
