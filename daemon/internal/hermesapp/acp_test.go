// acp_test.go —— hermesapp 解析层行为契约(夹具形状逐字对 acp_adapter 实测:
// permissions.py/edit_approval.py/tools.py + 真机探针帧):
//   - ParseSessionUpdate 逐变体对表(ACP 0.9.0 全部已知成员 + 未知变体上抛);
//   - ParseRequestPermission(危险命令 + 编辑提案两种 toolCall 形状);
//   - PermissionOutcomeSelected 应答形状;
//   - 能力谓词(loadSession 布尔 + sessionCapabilities 对象在场即支持)。
package hermesapp

import (
	"encoding/json"
	"testing"
)

func env(sessionUpdate string, update string) json.RawMessage {
	return json.RawMessage(`{"sessionId":"s1","update":{"sessionUpdate":"` + sessionUpdate + `",` + update + `}}`)
}

// TestParseSessionUpdateVariants 逐变体对表:每个已知 sessionUpdate 成员解析后
// 落在具名字段,串到别的字段算失败(投影层按互斥消费)。
func TestParseSessionUpdateVariants(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want StreamEvent
	}{
		{
			name: "agent_message_chunk",
			raw:  env("agent_message_chunk", `"content":{"type":"text","text":"你好"}`),
			want: StreamEvent{SessionID: "s1", MessageChunk: "你好"},
		},
		{
			name: "agent_thought_chunk",
			raw:  env("agent_thought_chunk", `"content":{"type":"text","text":"想一想"}`),
			want: StreamEvent{SessionID: "s1", ThoughtChunk: "想一想"},
		},
		{
			name: "user_message_chunk(排队回显/历史回放)",
			raw:  env("user_message_chunk", `"content":{"type":"text","text":"排队的话"}`),
			want: StreamEvent{SessionID: "s1", UserChunk: "排队的话"},
		},
		{
			name: "tool_call 新开(text content + rawInput)",
			raw: env("tool_call", `"toolCallId":"tc-abc","title":"Running terminal","kind":"execute",`+
				`"status":"pending","content":[{"type":"content","content":{"type":"text","text":"$ ls"}}],`+
				`"rawInput":{"command":"ls"}`),
			want: StreamEvent{SessionID: "s1", ToolCall: &ToolCallEvent{
				Kind: "tool_call", ToolCallID: "tc-abc", Title: "Running terminal",
				Status: "pending", ToolKind: "execute", Output: "$ ls",
				RawInput: json.RawMessage(`{"command":"ls"}`),
			}},
		},
		{
			name: "tool_call_update 完成",
			raw: env("tool_call_update", `"toolCallId":"tc-abc","status":"completed",`+
				`"content":[{"type":"content","content":{"type":"text","text":"done"}}]`),
			want: StreamEvent{SessionID: "s1", ToolCall: &ToolCallEvent{
				Kind: "tool_call_update", ToolCallID: "tc-abc", Status: "completed", Output: "done",
			}},
		},
		{
			name: "tool_call_update 编辑提案(diff 内容块,tools.py build_tool_start edit_diff)",
			raw: env("tool_call", `"toolCallId":"tc-edit","title":"write_file","kind":"edit","status":"pending",`+
				`"content":[{"type":"diff","path":"C:\\tmp\\a.txt","oldText":null,"newText":"hi"}]`),
			want: StreamEvent{SessionID: "s1", ToolCall: &ToolCallEvent{
				Kind: "tool_call", ToolCallID: "tc-edit", Title: "write_file",
				Status: "pending", ToolKind: "edit",
				Diff: &ToolDiff{Path: `C:\tmp\a.txt`, NewText: "hi"},
			}},
		},
		{
			name: "plan",
			raw:  env("plan", `"entries":[{"content":"第一步","status":"pending"},{"content":"第二步","status":"completed"}]`),
			want: StreamEvent{SessionID: "s1", Plan: []PlanEntry{
				{Content: "第一步", Status: "pending"}, {Content: "第二步", Status: "completed"},
			}},
		},
		{
			name: "current_mode_update",
			raw:  env("current_mode_update", `"currentModeId":"accept_edits"`),
			want: StreamEvent{SessionID: "s1", CurrentMode: "accept_edits"},
		},
		{
			name: "usage_update(server.py _build_usage_update:size=上下文窗,used=估算)",
			raw:  env("usage_update", `"size":128000,"used":4200`),
			want: StreamEvent{SessionID: "s1", Usage: &UsageSnapshot{Size: 128000, Used: 4200}},
		},
		{
			name: "available_commands_update",
			raw:  env("available_commands_update", `"availableCommands":[{"name":"/model"},{"name":"/compress"}]`),
			want: StreamEvent{SessionID: "s1", CommandsSeen: 2},
		},
		{
			name: "session_info_update(自动起名/压缩轮转)",
			raw:  env("session_info_update", `"title":"修复登录","updatedAt":"2026-10-02T00:00:00Z"`),
			want: StreamEvent{SessionID: "s1", SessionInfo: &SessionInfoPatch{Title: "修复登录", UpdatedAt: "2026-10-02T00:00:00Z"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseSessionUpdate(tc.raw)
			if got.SessionID != tc.want.SessionID {
				t.Fatalf("sessionId = %q, want %q", got.SessionID, tc.want.SessionID)
			}
			if got.MessageChunk != tc.want.MessageChunk || got.ThoughtChunk != tc.want.ThoughtChunk || got.UserChunk != tc.want.UserChunk {
				t.Fatalf("文本面不符: %+v", got)
			}
			if got.CurrentMode != tc.want.CurrentMode || got.CommandsSeen != tc.want.CommandsSeen {
				t.Fatalf("模式/命令面不符: %+v", got)
			}
			if (got.Usage == nil) != (tc.want.Usage == nil) || (got.Usage != nil && *got.Usage != *tc.want.Usage) {
				t.Fatalf("usage 面: got %+v want %+v", got.Usage, tc.want.Usage)
			}
			if (got.SessionInfo == nil) != (tc.want.SessionInfo == nil) ||
				(got.SessionInfo != nil && *got.SessionInfo != *tc.want.SessionInfo) {
				t.Fatalf("sessionInfo 面: got %+v want %+v", got.SessionInfo, tc.want.SessionInfo)
			}
			if (got.ToolCall == nil) != (tc.want.ToolCall == nil) {
				t.Fatalf("toolCall 在场不符: got %+v", got.ToolCall)
			}
			if got.ToolCall != nil {
				assertToolCall(t, got.ToolCall, tc.want.ToolCall)
			}
			if len(got.Plan) != len(tc.want.Plan) {
				t.Fatalf("plan 条数: got %d want %d", len(got.Plan), len(tc.want.Plan))
			}
		})
	}
}

// assertToolCall 逐字段比对(ToolCallEvent 含切片/指针字段,不可 == 全等)。
func assertToolCall(t *testing.T, got, want *ToolCallEvent) {
	t.Helper()
	if got.Kind != want.Kind || got.ToolCallID != want.ToolCallID || got.Title != want.Title ||
		got.Status != want.Status || got.ToolKind != want.ToolKind || got.Output != want.Output {
		t.Fatalf("toolCall 标量面: got %+v want %+v", got, want)
	}
	if string(got.RawInput) != string(want.RawInput) {
		t.Fatalf("rawInput: got %s want %s", got.RawInput, want.RawInput)
	}
	if (got.Diff == nil) != (want.Diff == nil) {
		t.Fatalf("diff 在场不符: got %+v want %+v", got.Diff, want.Diff)
	}
	if got.Diff != nil && *got.Diff != *want.Diff {
		t.Fatalf("diff: got %+v want %+v", *got.Diff, *want.Diff)
	}
}

// TestParseSessionUpdateUnknownVariant 未知变体:如实带 Raw 上抛,不炸不吞。
func TestParseSessionUpdateUnknownVariant(t *testing.T) {
	raw := env("future_variant_x", `"foo":1`)
	ev := ParseSessionUpdate(raw)
	if ev.MessageChunk != "" || ev.ToolCall != nil || len(ev.Plan) != 0 {
		t.Fatalf("未知变体不该落任何具名字段: %+v", ev)
	}
	if string(ev.Raw) != string(raw) {
		t.Fatalf("未知变体必须带 Raw 原文")
	}
}

// TestParseSessionUpdateBadJSON 坏帧:返回零值事件(带 Raw),不 panic。
func TestParseSessionUpdateBadJSON(t *testing.T) {
	ev := ParseSessionUpdate(json.RawMessage(`{`))
	if ev.SessionID != "" {
		t.Fatalf("坏帧不该解析出 sessionId: %+v", ev)
	}
}

// TestParseRequestPermissionCommand 危险命令审批帧(permissions.py
// _build_permission_tool_call + _build_permission_options 实证形状)。
func TestParseRequestPermissionCommand(t *testing.T) {
	raw := json.RawMessage(`{
		"sessionId":"s1",
		"toolCall":{"toolCallId":"perm-check-1","title":"运行命令: rm -rf /","kind":"execute","status":"pending",
			"content":[{"type":"content","content":{"type":"text","text":"运行命令: rm -rf /\n$ rm -rf /"}}],
			"rawInput":{"command":"rm -rf /","description":"运行命令"}},
		"options":[
			{"optionId":"allow_once","kind":"allow_once","name":"Allow once"},
			{"optionId":"allow_session","kind":"allow_always","name":"Allow for session"},
			{"optionId":"allow_always","kind":"allow_always","name":"Allow always"},
			{"optionId":"deny","kind":"reject_once","name":"Deny"},
			{"optionId":"deny_always","kind":"reject_always","name":"Deny always"}
		]}`)
	p, err := ParseRequestPermission(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if p.SessionID != "s1" || p.ToolCall == nil || p.ToolCall.ToolCallID != "perm-check-1" || p.ToolCall.Kind != "execute" {
		t.Fatalf("toolCall 面不符: %+v", p)
	}
	if got := p.AllowOptionID(); got != "allow_once" {
		t.Fatalf("AllowOptionID = %q, want allow_once", got)
	}
	if got := p.RejectOptionID(); got != "deny" {
		t.Fatalf("RejectOptionID = %q, want deny", got)
	}
}

// TestParseRequestPermissionEditOnly 编辑审批帧只有 allow_once/deny 两选项
// (edit_approval.py make_acp_edit_approval_requester 实证)。
func TestParseRequestPermissionEditOnly(t *testing.T) {
	raw := json.RawMessage(`{
		"sessionId":"s1",
		"toolCall":{"toolCallId":"edit-approval-1","title":"Approve edit: a.txt","kind":"edit","status":"pending",
			"content":[{"type":"diff","path":"a.txt","oldText":null,"newText":"hi"}],
			"rawInput":{"tool":"write_file","arguments":{"path":"a.txt","content":"hi"}}},
		"options":[
			{"optionId":"allow_once","kind":"allow_once","name":"Allow edit"},
			{"optionId":"deny","kind":"reject_once","name":"Deny"}
		]}`)
	p, err := ParseRequestPermission(raw)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := p.AllowOptionID(); got != "allow_once" {
		t.Fatalf("AllowOptionID = %q", got)
	}
	if got := p.RejectOptionID(); got != "deny" {
		t.Fatalf("RejectOptionID = %q", got)
	}
}

// TestParseRequestPermissionErrors 坏参必须显式报错(上层 RespondError,不代答)。
func TestParseRequestPermissionErrors(t *testing.T) {
	if _, err := ParseRequestPermission(json.RawMessage(`{`)); err == nil {
		t.Fatal("坏 JSON 应报错")
	}
	if _, err := ParseRequestPermission(json.RawMessage(`{"sessionId":"s1"}`)); err == nil {
		t.Fatal("缺 options 应报错")
	}
}

// TestPermissionOutcomeSelected 应答形状钉死(permissions.py AllowedOutcome)。
func TestPermissionOutcomeSelected(t *testing.T) {
	b, _ := json.Marshal(PermissionOutcomeSelected("allow_once"))
	var o struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		t.Fatalf("应答解析失败: %s", b)
	}
	if o.Outcome.Outcome != "selected" || o.Outcome.OptionID != "allow_once" {
		t.Fatalf("形状漂移: %s", b)
	}
}

// TestAgentCapabilities 谓词:loadSession 布尔 + fork/list/resume 对象在场即支持
// (initialize 实测形状);nil 快照一律 false。
func TestAgentCapabilities(t *testing.T) {
	full := &AgentCapabilities{
		LoadSession: true,
		SessionCapabilities: &SessionCapabilitiesSnapshot{
			Fork: &struct{}{}, List: &struct{}{}, Resume: &struct{}{},
		},
	}
	if !full.CanLoadSession() || !full.CanFork() || !full.CanList() || !full.CanResume() {
		t.Fatalf("全集能力判定不符: %+v", full)
	}
	bare := &AgentCapabilities{LoadSession: false}
	if bare.CanLoadSession() || bare.CanFork() || bare.CanList() || bare.CanResume() {
		t.Fatalf("裸能力判定不符: %+v", bare)
	}
	var none *AgentCapabilities
	if none.CanLoadSession() || none.CanFork() {
		t.Fatal("nil 快照必须全 false")
	}
}

// TestInitializeResultUnmarshal initialize 应答快照(实测形状含 agentCapabilities)。
func TestInitializeResultUnmarshal(t *testing.T) {
	raw := `{"protocolVersion":1,
		"agentInfo":{"name":"hermes-agent","version":"0.6.2"},
		"agentCapabilities":{"loadSession":true,"promptCapabilities":{"image":true},
			"sessionCapabilities":{"fork":{},"list":{},"resume":{}}},
		"authMethods":[{"id":"terminal-setup","name":"Terminal setup"}]}`
	out := &InitializeResult{}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if out.AgentInfo.Name != "hermes-agent" || !out.AgentCapabilities.CanLoadSession() || !out.AgentCapabilities.CanList() {
		t.Fatalf("能力快照不符: %+v", out)
	}
	if !out.AgentCapabilities.PromptCapabilities.Image {
		t.Fatal("promptCapabilities.image 应为 true")
	}
}

// TestSessionNewResultUnmarshal session/new 应答(models/modes 内嵌面)。
func TestSessionNewResultUnmarshal(t *testing.T) {
	raw := `{"sessionId":"sess-9","models":{"currentModelId":"deepseek:deepseek-flash",
		"availableModels":[{"modelId":"deepseek:deepseek-flash","name":"DeepSeek Flash"}]},
		"modes":{"currentModeId":"default","availableModes":[{"id":"default","name":"Default"}]}}`
	out := &SessionNewResult{}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if out.SessionID != "sess-9" || out.Models.CurrentModelID != "deepseek:deepseek-flash" || len(out.Models.AvailableModels) != 1 {
		t.Fatalf("应答面不符: %+v", out)
	}
	if out.Modes.CurrentModeID != "default" {
		t.Fatalf("modes 面不符: %+v", out.Modes)
	}
}

// TestIsNullJSON load/resume 查无此会话时适配器回 result=null。
func TestIsNullJSON(t *testing.T) {
	if !isNullJSON(json.RawMessage(`null`)) || !isNullJSON(nil) {
		t.Fatal("null/空应判为 null")
	}
	if isNullJSON(json.RawMessage(`{"sessionId":"x"}`)) {
		t.Fatal("对象不应判为 null")
	}
}
