package agent

// zcode_test.go —— ZCode 事件映射与审批回路的单测。
// 全部输入形状来自 cmd/zcprobe 实测(docs/verify/evidence/zcode-p0-probe.md),
// 是"payload.kind → pb"映射的地面真值钉子:改协议形状必须先改这里再改实现。

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// ---- 流式增量(model.streaming)----

func TestMapZcodeStreamDelta(t *testing.T) {
	// 探针实测:text_delta / reasoning_delta 都带 {delta, done, assistantMessageId}
	text := json.RawMessage(`{"assistantMessageId":"msg_1","delta":"PONG","done":false,"kind":"text_delta"}`)
	kind, delta, ok := mapZcodeStreamDelta(text)
	if !ok || kind != "text" || delta != "PONG" {
		t.Fatalf("text_delta 映射失败: kind=%q delta=%q ok=%v", kind, delta, ok)
	}
	reasoning := json.RawMessage(`{"assistantMessageId":"msg_1","delta":"The","done":false,"kind":"reasoning_delta"}`)
	kind, delta, ok = mapZcodeStreamDelta(reasoning)
	if !ok || kind != "reasoning" || delta != "The" {
		t.Fatalf("reasoning_delta 映射失败: kind=%q delta=%q ok=%v", kind, delta, ok)
	}
	// 空增量与未知 kind 不产生帧
	if _, _, ok := mapZcodeStreamDelta(json.RawMessage(`{"kind":"text_delta","delta":""}`)); ok {
		t.Fatal("空 delta 不应产出帧")
	}
	if _, _, ok := mapZcodeStreamDelta(json.RawMessage(`{"kind":"tool_input_delta","delta":"x"}`)); ok {
		t.Fatal("未知 kind 不应产出帧")
	}
	if _, _, ok := mapZcodeStreamDelta(json.RawMessage(`not json`)); ok {
		t.Fatal("坏 payload 不应产出帧")
	}
}

// ---- 工具生命周期(tool.updated)----

func TestMapZcodeToolCallStarted(t *testing.T) {
	// 探针实测:tool_call 带 toolCallId/toolName/input(完整工具入参对象)
	raw := json.RawMessage(`{"toolCallId":"call_1","toolName":"Write","input":{"file_path":"C:\\a.txt","content":"PONG"},"kind":"tool_call"}`)
	upd, ok := mapZcodeToolCall(raw)
	if !ok {
		t.Fatal("tool_call 应映射成功")
	}
	if upd.Phase != 1 /* STARTED */ {
		t.Fatalf("phase 应为 STARTED,得到 %v", upd.Phase)
	}
	if upd.Call.Id != "call_1" || upd.Call.Name != "Write" {
		t.Fatalf("工具标识错误: %+v", upd.Call)
	}
	if !json.Valid([]byte(upd.Call.JsonArgs)) || !strings.Contains(upd.Call.JsonArgs, "file_path") {
		t.Fatalf("JsonArgs 应保留完整 input: %s", upd.Call.JsonArgs)
	}
	// input 缺失时归一成 "{}",不允许空串
	raw2 := json.RawMessage(`{"toolCallId":"call_2","toolName":"Bash","kind":"tool_call"}`)
	upd2, ok := mapZcodeToolCall(raw2)
	if !ok || upd2.Call.JsonArgs != "{}" {
		t.Fatalf("缺 input 应归一为 {}: %+v", upd2.Call)
	}
	// 非 tool_call kind 不映射
	if _, ok := mapZcodeToolCall(json.RawMessage(`{"kind":"result"}`)); ok {
		t.Fatal("result kind 不应走 STARTED 映射")
	}
}

func TestMapZcodeToolResultFinished(t *testing.T) {
	// 探针实测:result 带 result{success, content},以及 perf 等附加字段(忽略)
	raw := json.RawMessage(`{"toolCallId":"call_1","result":{"success":true,"content":"File created successfully at: C:\\a.txt","perf":{"totalMs":18,"permissionWaitMs":4}},"duration":6,"kind":"result"}`)
	upd, ok := mapZcodeToolResult(raw)
	if !ok {
		t.Fatal("result 应映射成功")
	}
	if upd.Phase != 2 /* FINISHED */ {
		t.Fatalf("phase 应为 FINISHED,得到 %v", upd.Phase)
	}
	if !upd.Result.Ok || upd.Result.CallId != "call_1" {
		t.Fatalf("结果标识/状态错误: %+v", upd.Result)
	}
	if !strings.Contains(upd.Result.Output, "File created successfully") {
		t.Fatalf("输出应保留 content: %q", upd.Result.Output)
	}
	// 失败结果 Ok=false
	rawFail := json.RawMessage(`{"toolCallId":"call_2","result":{"success":false,"content":"boom"},"kind":"result"}`)
	updFail, ok := mapZcodeToolResult(rawFail)
	if !ok || updFail.Result.Ok {
		t.Fatal("success=false 应映射为 Ok=false")
	}
}

func TestMapZcodeToolError(t *testing.T) {
	raw := json.RawMessage(`{"toolCallId":"call_9","toolName":"Bash","error":{"message":"exit code 1","type":"exec"},"kind":"tool_error"}`)
	upd, ok := mapZcodeToolError(raw)
	if !ok {
		t.Fatal("tool_error 应映射成功")
	}
	if upd.Result.Ok {
		t.Fatal("tool_error 必须 Ok=false")
	}
	if !contains(upd.Result.Output, "exit code 1") {
		t.Fatalf("错误信息应进入输出: %q", upd.Result.Output)
	}
	// 无 error 字段时兜底文案,不 panic
	rawBare := json.RawMessage(`{"toolCallId":"call_10","kind":"tool_error"}`)
	if upd, ok := mapZcodeToolError(rawBare); !ok || upd.Result.Output == "" {
		t.Fatal("tool_error 兜底文案缺失")
	}
}

// ---- 轮次终局 ----

func TestMapZcodeTurnCompleted(t *testing.T) {
	// 探针实测:turn.completed 带 response/usage(input/output/cache/reasoning)/resultType
	raw := json.RawMessage(`{"response":"PONG","tokenCount":19758,"usage":{"source":"provider","modelRequestCount":1,"inputTokens":19741,"outputTokens":17,"totalTokens":19758,"cacheReadTokens":15872,"cacheWriteTokens":0,"reasoningTokens":14},"toolCallCount":0,"duration":4061,"resultType":"success"}`)
	text, usage, ok := mapZcodeTurnCompleted(raw)
	if !ok || text != "PONG" {
		t.Fatalf("turn.completed 解析失败: text=%q ok=%v", text, ok)
	}
	if usage.InputTokens != 19741 || usage.OutputTokens != 17 || usage.CacheReadTokens != 15872 || usage.ReasoningTokens != 14 {
		t.Fatalf("用量映射错误: %+v", usage)
	}
	if _, _, ok := mapZcodeTurnCompleted(json.RawMessage(`bad`)); ok {
		t.Fatal("坏 payload 不应成功")
	}
}

func TestMapZcodeTurnFailed(t *testing.T) {
	raw := json.RawMessage(`{"error":{"type":"api_error","message":"上游超时"},"turnPhase":"awaiting_model"}`)
	msg, ok := mapZcodeTurnFailed(raw)
	if !ok || !strings.Contains(msg, "上游超时") {
		t.Fatalf("turn.failed 错误信息映射失败: %q", msg)
	}
	// 无 error 时按 turnPhase 兜底
	msg2, ok2 := mapZcodeTurnFailed(json.RawMessage(`{"turnPhase":"tooling"}`))
	if !ok2 || msg2 == "" {
		t.Fatal("turn.failed 兜底文案缺失")
	}
}

// ---- 模型表(快照解析)----

func TestParseZcodeModels(t *testing.T) {
	// 节选自探针 run5 的 create 快照(真实形状)
	snap := json.RawMessage(`{
	  "session":{"sessionId":"sess_x"},
	  "settings":{"model":{"available":[
	    {"ref":{"providerId":"p1","modelId":"MiniMax-M3"},"label":"MiniMax-M3","providerLabel":"minimax","contextWindow":1000000,
	     "reasoning":{"levels":[{"value":"enabled","label":"enabled"}],"defaultLevel":"enabled"},
	     "properties":{"inputFormat":{"supportsText":true,"supportsImage":true}}},
	    {"ref":{"providerId":"p2","modelId":"glm-5.2"},"label":"GLM-5.2","providerLabel":"glm","contextWindow":1000000,
	     "reasoning":{"levels":[{"value":"max","label":"max"}],"defaultLevel":"max"},
	     "properties":{"inputFormat":{"supportsText":true,"supportsImage":false}}}
	  ]}}}`)
	models, refs := parseZcodeModels(snap)
	if len(models) != 2 {
		t.Fatalf("应解析出 2 个模型,得到 %d", len(models))
	}
	if models[1].Id != "zcode/glm-5.2" || models[1].DisplayName != "GLM-5.2" {
		t.Fatalf("模型 id/名错误: id=%q name=%q", models[1].Id, models[1].DisplayName)
	}
	if models[1].ContextWindow != 1000000 {
		t.Fatalf("窗口错误: %d", models[1].ContextWindow)
	}
	// 审计 S-1 之 supportsImage:快照 properties.inputFormat.supportsImage 透出
	// 到 ModelInfo.SupportsImages(与 pi.go 的模型能力位同模式)。
	// 【vet】pb 消息含锁,不能整值 %v 进 t.Fatalf,逐字段断言。
	if !models[0].SupportsImages {
		t.Fatalf("MiniMax-M3 supportsImage=true 应透出: id=%q images=%v", models[0].Id, models[0].SupportsImages)
	}
	if models[1].SupportsImages {
		t.Fatalf("glm-5.2 supportsImage=false 不应透出: id=%q images=%v", models[1].Id, models[1].SupportsImages)
	}
	if refs["glm-5.2"].ProviderID != "p2" || refs["glm-5.2"].DefaultReasoningLv != "max" {
		t.Fatalf("引用映射错误: %+v", refs["glm-5.2"])
	}
}

// ---- workspace / 审批形状 ----

func TestZcodeWorkspaceObject(t *testing.T) {
	ws := zcodeWorkspace(`G:\proj`)
	if ws["workspacePath"] != ws["workspaceKey"] || ws["workspacePath"] != `G:\proj` {
		t.Fatalf("workspaceKey 必须等于 workspacePath(refine 规则): %+v", ws)
	}
}

func TestZcodeApprovalAnswerStrictShape(t *testing.T) {
	allow := zcodeApprovalAnswer(true, "")
	if len(allow) != 2 || allow["decision"] != "allow" {
		t.Fatalf("批准应答必须是 {decision,reason} 两键: %+v", allow)
	}
	deny := zcodeApprovalAnswer(false, "不许动这个文件")
	if deny["decision"] != "deny" || deny["reason"] != "不许动这个文件" {
		t.Fatalf("拒绝应答应带上用户备注: %+v", deny)
	}
	denyDefault := zcodeApprovalAnswer(false, "")
	if denyDefault["reason"] == "" {
		t.Fatal("无备注拒绝也要有人话 reason")
	}
	// 序列化后绝不允许出现 optionId(实测会让服务器判非法 → deny)
	raw, _ := json.Marshal(allow)
	if strings.Contains(string(raw), "optionId") {
		t.Fatalf("应答里不允许 optionId: %s", raw)
	}
}

func TestBuildZcodeApprovalCall(t *testing.T) {
	pr := zcodePermissionRequest{
		ToolCallID: "call_1",
		ToolName:   "Write",
		Input:      json.RawMessage(`{"file_path":"C:\\a.txt"}`),
	}
	call := buildZcodeApprovalCall(pr)
	if call.Id != "call_1" || call.Name != "Write" {
		t.Fatalf("审批卡标识错误: id=%q name=%q", call.Id, call.Name)
	}
	if !strings.Contains(call.JsonArgs, "file_path") {
		t.Fatalf("审批卡应带完整 input: %s", call.JsonArgs)
	}
}

// ---- 会话 id 解析(create 应答)----

func TestZcodeSnapshotSessionID(t *testing.T) {
	// create 应答的 sessionId 在 session.sessionId(不在顶层,实测踩过的坑)。
	// 解析统一走 zcodeapp.ParseSessionSnapshot(zcode.go 与 zcodeapp 包共用
	// 同一份快照形状知识,消除双份解析)。
	snap, err := zcodeapp.ParseSessionSnapshot(json.RawMessage(`{"session":{"sessionId":"sess_123"},"settings":{"model":{"available":[]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if snap.SessionID != "sess_123" {
		t.Fatalf("sessionId 解析错误: %q", snap.SessionID)
	}
}

// ---- 反问类反向请求(interaction/requestUserInput)----
// fixture 形状来自 zcode.cjs 内嵌 zod schema(DXn/AJt/jge,见 zcode.go 引用),
// 这里同时钉住 JSON tag 与形状:改协议必须先改这里。

func TestBuildZcodeUserInputCall(t *testing.T) {
	var req zcodeUserInputRequest
	if err := json.Unmarshal([]byte(`{
		"requestId":"ask_1","sessionId":"sess_x","prompt":"请选择继续方式",
		"questions":[{"question":"继续方式","header":"How","multiSelect":false,
		  "options":[{"value":"auto","label":"自动继续","description":"不用确认"},
		             {"value":"manual","label":"手动确认"}]}]}`), &req); err != nil {
		t.Fatal(err)
	}
	call, ok := buildZcodeUserInputCall(req, "ask_1")
	if !ok {
		t.Fatal("带问题的反问应可呈现")
	}
	if call.Id != "ask_1" || call.Name != toolNameAskUser {
		t.Fatalf("卡标识/名字错误: id=%q name=%q", call.Id, call.Name)
	}
	var args struct {
		Message   string `json:"message"`
		Questions []struct {
			ID       string `json:"id"`
			Question string `json:"question"`
			Options  []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(call.JsonArgs), &args); err != nil {
		t.Fatal(err)
	}
	if len(args.Questions) != 1 || args.Questions[0].ID != "q0" || args.Questions[0].Question != "继续方式" {
		t.Fatalf("问题映射错误: %+v", args.Questions)
	}
	// 【服务器收集规则】label=value:上屏的是 label,value 不上屏(答案按 label 回)
	opts := args.Questions[0].Options
	if len(opts) != 2 || opts[0].Label != "自动继续" || opts[1].Label != "手动确认" {
		t.Fatalf("选项应按 label 透传: %+v", opts)
	}
}

func TestBuildZcodeUserInputCallPromptOnly(t *testing.T) {
	// 纯 prompt 无 questions:兜底一个输入框(否则用户没有作答的地方)
	var req zcodeUserInputRequest
	_ = json.Unmarshal([]byte(`{"requestId":"ask_2","prompt":"随便说点什么"}`), &req)
	call, ok := buildZcodeUserInputCall(req, "ask_2")
	if !ok || call.Name != toolNameAskUser {
		// 【vet】pb 消息含锁,不能 %+v 整值打印,只打字段
		t.Fatalf("纯 prompt 应回兜底卡: ok=%v name=%q", ok, call.Name)
	}
	// prompt 与 questions 全空:呈现不了 → 走 -32601(ok=false)
	var empty zcodeUserInputRequest
	_ = json.Unmarshal([]byte(`{"requestId":"ask_3"}`), &empty)
	if _, ok := buildZcodeUserInputCall(empty, "ask_3"); ok {
		t.Fatal("无问题且无提示语应判定为不可呈现")
	}
}

func TestZcodeUserInputAnswerShape(t *testing.T) {
	var req zcodeUserInputRequest
	if err := json.Unmarshal([]byte(`{
		"requestId":"ask_1","prompt":"p",
		"questions":[{"question":"继续方式","header":"How","options":[{"value":"auto","label":"自动继续"}]},
		             {"question":"备注","header":"Note","options":[{"value":"none","label":"无"}]}]}`), &req); err != nil {
		t.Fatal(err)
	}
	// accept:content 按"问题原文→答案"收集(服务器 BCs 以 question 原文为键)
	ans := zcodeUserInputAnswer(true, `{"q0":{"answers":["自动继续"]},"q1":{"answers":["辛苦了"]}}`, req)
	if ans["action"] != "accept" {
		t.Fatalf("action 应为 accept: %+v", ans)
	}
	content, _ := ans["content"].(map[string]interface{})
	if content["继续方式"] != "自动继续" || content["备注"] != "辛苦了" {
		t.Fatalf("content 应按问题原文收集: %+v", content)
	}
	// jge strict:序列化后只允许 action/content/reason 三键
	raw, _ := json.Marshal(ans)
	for _, k := range []string{"optionId", "cancelled", "value", "confirmed"} {
		if strings.Contains(string(raw), k) {
			t.Fatalf("应答不允许出现 %s 键: %s", k, raw)
		}
	}
	// 批准但没有一个答案:content:{}(与桌面端放行路径同款,不是 decline)
	ansEmpty := zcodeUserInputAnswer(true, "", req)
	c, _ := ansEmpty["content"].(map[string]interface{})
	if ansEmpty["action"] != "accept" || len(c) != 0 {
		t.Fatalf("无答案的 accept 应回空 content: %+v", ansEmpty)
	}
	// 拒绝:decline + 人话 reason
	ansDeny := zcodeUserInputAnswer(false, "", req)
	if ansDeny["action"] != "decline" || ansDeny["reason"] == "" {
		t.Fatalf("拒绝应带人话 reason: %+v", ansDeny)
	}
}

// ---- 反问类反向请求(elicitation/create,MCP 标准两形态)----

func TestBuildZcodeElicitationCallFormMode(t *testing.T) {
	var req zcodeElicitationRequest
	if err := json.Unmarshal([]byte(`{
		"mode":"form","message":"配置数据库连接",
		"requestedSchema":{"type":"object","required":["env"],
		  "properties":{
		    "zeta":{"type":"string","title":"Z 用途"},
		    "env":{"type":"string","title":"环境","enum":["开发","生产"]}}}}`), &req); err != nil {
		t.Fatal(err)
	}
	call, names, ok := buildZcodeElicitationCall(req, "server-9")
	if !ok {
		t.Fatal("form 模式应可呈现")
	}
	if call.Name != toolNameAskUser {
		t.Fatalf("应复用反问卡通道: %q", call.Name)
	}
	// 属性按名字排序(map 遍历无序,卡片顺序必须稳定可复现;供应答侧 qN→属性名)
	if len(names) != 2 || names[0] != "env" || names[1] != "zeta" {
		t.Fatalf("属性名应排序: %v", names)
	}
	var args struct {
		Message   string `json:"message"`
		Questions []struct {
			ID       string `json:"id"`
			Question string `json:"question"`
			Options  []struct {
				Label string `json:"label"`
			} `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(call.JsonArgs), &args); err != nil {
		t.Fatal(err)
	}
	if len(args.Questions) != 2 || args.Questions[0].Question != "环境" {
		t.Fatalf("题干应取 title: %+v", args.Questions)
	}
	// enum 属性渲染成点选项;非 enum 属性是输入框(无选项)
	if len(args.Questions[0].Options) != 2 || args.Questions[0].Options[0].Label != "开发" {
		t.Fatalf("enum 应转点选项: %+v", args.Questions[0].Options)
	}
	if len(args.Questions[1].Options) != 0 {
		t.Fatalf("非 enum 属性不应有选项: %+v", args.Questions[1].Options)
	}
}

func TestBuildZcodeElicitationCallURLModeRejected(t *testing.T) {
	// url 模式手机呈现不了:-32601 只留给这种(buildZcodeElicitationCall ok=false)
	var req zcodeElicitationRequest
	_ = json.Unmarshal([]byte(`{"mode":"url","message":"去这里登录","elicitationId":"el_1","url":"https://example.com/login"}`), &req)
	if _, _, ok := buildZcodeElicitationCall(req, "server-9"); ok {
		t.Fatal("url 模式应判定为不可呈现")
	}
}

func TestZcodeElicitationAnswerContentKeys(t *testing.T) {
	var req zcodeElicitationRequest
	_ = json.Unmarshal([]byte(`{
		"mode":"form","message":"m",
		"requestedSchema":{"type":"object","properties":{
		  "env":{"type":"string","title":"环境","enum":["开发","生产"]},
		  "note":{"type":"string","title":"备注"}}}}`), &req)
	names := []string{"env", "note"}
	// content 键必须是 properties 键(MCP ElicitResult 约定)
	ans := zcodeElicitationAnswer(true, `{"q0":{"answers":["生产"]},"q1":{"answers":["本地测试"]}}`, req, names)
	content, _ := ans["content"].(map[string]interface{})
	if content["env"] != "生产" || content["note"] != "本地测试" {
		t.Fatalf("content 键应为属性名: %+v", content)
	}
	raw, _ := json.Marshal(ans)
	if strings.Contains(string(raw), "q0") || strings.Contains(string(raw), "q1") {
		t.Fatalf("content 键不允许残留卡片内部 id: %s", raw)
	}
	// 拒绝:decline(MCP 语义,无 reason 键要求)
	if d := zcodeElicitationAnswer(false, "", req, names); d["action"] != "decline" {
		t.Fatalf("拒绝应回 decline: %+v", d)
	}
}

// ---- 反问接线的辅助件 ----

func TestRawIDString(t *testing.T) {
	if got := rawIDString(json.RawMessage(`"server-6"`)); got != "server-6" {
		t.Fatalf("rawIDString = %q, want server-6", got)
	}
	if got := rawIDString(json.RawMessage(`42`)); got != "" {
		t.Fatalf("非字符串 id 应返回空串,得到 %q", got)
	}
}

func TestAskDenyReason(t *testing.T) {
	if askDenyReason("  不要装这个包  ") != "不要装这个包" {
		t.Fatal("用户备注应去空白后原样透传")
	}
	if askDenyReason("") == "" || askDenyReason("   ") == "" {
		t.Fatal("空备注也要有人话兜底")
	}
}
