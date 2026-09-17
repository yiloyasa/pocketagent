package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

// 这组测试钉死「审查结论 → GuardianAssessmentEvent」的转写表。
//
// 【为什么必须钉】这是整个 codex 接入里**最容易静默失效**的一段:
// codex 把 thread/approveGuardianDeniedAction 的 event 参数定义成无类型透传
// (JSON Schema 里是 JsonValue),结构一个字都没写在协议里。我们是对着本机
// codex 0.149.1 逐字段试探 serde 报错才问出来的。
//
// 而两边的大小写**故意不一样**:
//
//	通知侧(camelCase)  事件侧(snake_case)
//	timedOut            timed_out
//	applyPatch          apply_patch
//	networkAccess       network_access
//	unifiedExec         unified_exec
//	socks5Tcp           socks5_tcp
//	toolName            tool_name
//
// 只要哪天有人"顺手"把转写改成原样回填,界面照旧工作、按钮照旧显示 ——
// 只有用户点下去才会被 codex 拒绝,而且错在电脑端日志里。这种错最难查,
// 所以在单测里先炸掉。
func TestGuardianEnumTranscription(t *testing.T) {
	statuses := map[string]string{
		"inProgress": "in_progress",
		"timedOut":   "timed_out",
		// 两边同名的必须保持不变(别把它们也一起"翻译"了)
		"approved": "approved",
		"denied":   "denied",
		"aborted":  "aborted",
	}
	for in, want := range statuses {
		if got := guardianStatusToWire(in); got != want {
			t.Errorf("guardianStatusToWire(%q) = %q, want %q", in, got, want)
		}
	}

	types := map[string]string{
		"command":            "command",
		"execve":             "execve",
		"applyPatch":         "apply_patch",
		"networkAccess":      "network_access",
		"mcpToolCall":        "mcp_tool_call",
		"requestPermissions": "request_permissions",
	}
	for in, want := range types {
		if got := guardianActionTypeToWire(in); got != want {
			t.Errorf("guardianActionTypeToWire(%q) = %q, want %q", in, got, want)
		}
	}

	if got := guardianSourceToWire("unifiedExec"); got != "unified_exec" {
		t.Errorf("source unifiedExec → %q, want unified_exec", got)
	}
	if got := guardianSourceToWire("shell"); got != "shell" {
		t.Errorf("source shell → %q, want shell", got)
	}
	if got := guardianProtocolToWire("socks5Tcp"); got != "socks5_tcp" {
		t.Errorf("protocol socks5Tcp → %q, want socks5_tcp", got)
	}
	if got := guardianProtocolToWire("https"); got != "https" {
		t.Errorf("protocol https → %q, want https", got)
	}
}

// 每一种被审查的动作都要能完整转写成事件侧写法,且内层字段一起转。
func TestGuardianActionToWirePerVariant(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		check func(t *testing.T, m map[string]interface{})
	}{
		{
			name: "command(带 unifiedExec 来源)",
			in:   `{"type":"command","command":"rm -rf build","cwd":"C:\\work","source":"unifiedExec"}`,
			check: func(t *testing.T, m map[string]interface{}) {
				if m["type"] != "command" || m["source"] != "unified_exec" {
					t.Errorf("type/source 未转写: %v", m)
				}
				if m["command"] != "rm -rf build" || m["cwd"] != `C:\work` {
					t.Errorf("命令/目录被改动: %v", m)
				}
			},
		},
		{
			name: "execve",
			in:   `{"type":"execve","program":"node","argv":["node","-e","1"],"cwd":"C:\\work","source":"shell"}`,
			check: func(t *testing.T, m map[string]interface{}) {
				if m["type"] != "execve" {
					t.Errorf("type 未保持: %v", m["type"])
				}
				if argv, ok := m["argv"].([]interface{}); !ok || len(argv) != 3 {
					t.Errorf("argv 丢失: %v", m["argv"])
				}
			},
		},
		{
			name: "applyPatch",
			in:   `{"type":"applyPatch","cwd":"C:\\work","files":["C:\\work\\a.go"]}`,
			check: func(t *testing.T, m map[string]interface{}) {
				if m["type"] != "apply_patch" {
					t.Errorf("applyPatch 未转成 apply_patch: %v", m["type"])
				}
				if files, ok := m["files"].([]interface{}); !ok || len(files) != 1 {
					t.Errorf("files 丢失: %v", m["files"])
				}
			},
		},
		{
			name: "networkAccess",
			in:   `{"type":"networkAccess","host":"api.example.com","port":443,"protocol":"socks5Tcp","target":"api.example.com"}`,
			check: func(t *testing.T, m map[string]interface{}) {
				if m["type"] != "network_access" || m["protocol"] != "socks5_tcp" {
					t.Errorf("type/protocol 未转写: %v", m)
				}
			},
		},
		{
			name: "mcpToolCall(内层 toolName)",
			in:   `{"type":"mcpToolCall","server":"fs","toolName":"read_file","toolTitle":"读文件"}`,
			check: func(t *testing.T, m map[string]interface{}) {
				if m["type"] != "mcp_tool_call" {
					t.Errorf("type 未转写: %v", m["type"])
				}
				if m["tool_name"] != "read_file" {
					t.Errorf("toolName 未转成 tool_name: %v", m)
				}
				if _, still := m["toolName"]; still {
					t.Errorf("camelCase 键残留(会被 codex 拒): %v", m)
				}
			},
		},
		{
			name: "requestPermissions",
			in:   `{"type":"requestPermissions","permissions":{"network":true},"reason":"需要联网"}`,
			check: func(t *testing.T, m map[string]interface{}) {
				if m["type"] != "request_permissions" {
					t.Errorf("type 未转写: %v", m["type"])
				}
				if _, ok := m["permissions"].(map[string]interface{}); !ok {
					t.Errorf("permissions 丢失: %v", m["permissions"])
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, err := guardianActionToWire(json.RawMessage(c.in))
			if err != nil {
				t.Fatalf("转写失败: %v", err)
			}
			c.check(t, m)
		})
	}
}

// 转写不能"顺手"改掉 codex 给的其余字段 —— 协议是无类型透传,以后新增的必填
// 字段必须照原样带过去,否则按钮会在某次 codex 升级后突然开始报 missing field。
func TestGuardianActionToWireKeepsUnknownFields(t *testing.T) {
	m, err := guardianActionToWire(json.RawMessage(
		`{"type":"command","command":"ls","cwd":"/tmp","source":"shell","futureFieldOfCodex":"must-survive"}`))
	if err != nil {
		t.Fatalf("转写失败: %v", err)
	}
	if m["futureFieldOfCodex"] != "must-survive" {
		t.Errorf("未知字段被丢掉了: %v", m)
	}
}

func TestGuardianActionToWireRejectsGarbage(t *testing.T) {
	if _, err := guardianActionToWire(nil); err == nil {
		t.Error("空 action 应当报错(说明没记录到被审查的动作)")
	}
	if _, err := guardianActionToWire(json.RawMessage(`{`)); err == nil {
		t.Error("非法 JSON 应当报错")
	}
}

// 未知动作类型**故意透传**,不在这里拦。
//
// 【为什么】转写表只覆盖"两边大小写不一样"的六种动作。codex 以后新增动作类型时,
// 如果新名字本来就是 snake_case(协议内部命名习惯如此),透传能直接可用;
// 拦下来则要等我们发版。真要是名字需要转换,codex 会用 serde 报错把
// "unknown variant `xxx`, expected one of ..." 原样告诉我们,再经由
// ApproveGuardianDenied 带回手机上 —— 失败是可诊断的,不是静默的。
func TestGuardianActionToWirePassesUnknownTypeThrough(t *testing.T) {
	m, err := guardianActionToWire(json.RawMessage(`{"type":"teleport","cwd":"C:\\"}`))
	if err != nil {
		t.Fatalf("未知类型不该在这里失败(留给 codex 判定): %v", err)
	}
	if m["type"] != "teleport" {
		t.Errorf("未知类型应当原样透传, got %v", m["type"])
	}
}

// 还原出来的事件必须是 codex 期望的三件套,且 id 用的是 reviewId。
func TestGuardianAssessmentEventShape(t *testing.T) {
	rec := &GuardianReview{
		ReviewID: "rev-1",
		Status:   "timedOut",
		ActionRaw: json.RawMessage(
			`{"type":"applyPatch","cwd":"C:\\work","files":["C:\\work\\a.go"]}`),
	}
	ev, err := guardianAssessmentEvent(rec)
	if err != nil {
		t.Fatalf("还原失败: %v", err)
	}
	if ev["id"] != "rev-1" {
		t.Errorf("id 应当等于 reviewId, got %v", ev["id"])
	}
	if ev["status"] != "timed_out" {
		t.Errorf("status 未转写: %v", ev["status"])
	}
	act, ok := ev["action"].(map[string]interface{})
	if !ok || act["type"] != "apply_patch" {
		t.Errorf("action 未转写: %v", ev["action"])
	}
	// 编码一遍,确认发出去的确实是 JSON 对象(而不是被转成字符串)
	raw, _ := json.Marshal(ev)
	if !strings.Contains(string(raw), `"status":"timed_out"`) {
		t.Errorf("序列化结果不对: %s", raw)
	}
}

// ---- 记录与取用 ----

func TestGuardianRecordAndLatest(t *testing.T) {
	p := NewCodexProvider()
	thread := "codex-thread-1"

	// started 通知:还没有结论
	p.recordGuardianNotification("item/autoApprovalReview/started", json.RawMessage(`{
		"threadId":"`+thread+`","turnId":"t1","reviewId":"rev-1","startedAtMs":1000,
		"targetItemId":"item-9",
		"review":{"status":"inProgress"},
		"action":{"type":"command","command":"rm -rf /","cwd":"C:\\","source":"shell"}}`))
	if g := p.GuardianLatest(thread); g != nil {
		t.Errorf("进行中的审查不该提供推翻入口, got %+v", g)
	}

	// completed:被驳回 —— 这才是要给用户"仍然执行"入口的时刻
	p.recordGuardianNotification("item/autoApprovalReview/completed", json.RawMessage(`{
		"threadId":"`+thread+`","turnId":"t1","reviewId":"rev-1","startedAtMs":1000,"completedAtMs":1500,
		"targetItemId":"item-9",
		"review":{"status":"denied","riskLevel":"high","rationale":"会删除系统目录"},
		"action":{"type":"command","command":"rm -rf /","cwd":"C:\\","source":"shell"}}`))
	g := p.GuardianLatest(thread)
	if g == nil {
		t.Fatal("被驳回的审查应当可推翻")
	}
	if g.ReviewID != "rev-1" || g.TargetItemID != "item-9" {
		t.Errorf("记录不完整: %+v", g)
	}
	// started 与 completed 是同一条审查的两次通知:起始时间要保留下来
	if g.StartedAtMs != 1000 {
		t.Errorf("startedAtMs 应当保留第一次通知的值, got %d", g.StartedAtMs)
	}
	if !strings.Contains(g.ActionLabel, "rm -rf /") {
		t.Errorf("动作摘要不对: %q", g.ActionLabel)
	}

	// 后续结论变成"已放行":入口必须撤掉,否则用户会对着一个没有驳回的审查点"仍然执行"
	p.recordGuardianNotification("item/autoApprovalReview/completed", json.RawMessage(`{
		"threadId":"`+thread+`","reviewId":"rev-1","review":{"status":"approved"},
		"action":{"type":"command","command":"ls","cwd":"C:\\","source":"shell"}}`))
	if g := p.GuardianLatest(thread); g != nil {
		t.Errorf("已放行的审查不该再提供推翻入口, got %+v", g)
	}
}

// 找不不到结论时必须给出"人能照做的"提示,而不是一句内部错误。
func TestApproveGuardianDeniedExplainsMissingReview(t *testing.T) {
	p := NewCodexProvider()
	err := p.ApproveGuardianDenied("thread-x", "", "")
	if err == nil {
		t.Fatal("没有记录时必须报错")
	}
	if !strings.Contains(err.Error(), "重新发起") {
		t.Errorf("错误信息应当告诉用户怎么办, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "重启") {
		t.Errorf("应当指出最可能的原因(daemon 重启), got %q", err.Error())
	}
}

func TestApproveGuardianDeniedRejectsNonActionable(t *testing.T) {
	p := NewCodexProvider()
	p.recordGuardianNotification("item/autoApprovalReview/completed", json.RawMessage(`{
		"threadId":"thread-y","reviewId":"rev-y","review":{"status":"approved"},
		"action":{"type":"command","command":"ls","cwd":"C:\\","source":"shell"}}`))
	// 已放行:即使显式带上 reviewId 也要拒绝
	if err := p.ApproveGuardianDenied("thread-y", "rev-y", ""); err == nil {
		t.Fatal("对已放行的审查点『仍然执行』应当被拒绝")
	}
}

func TestApproveGuardianDeniedRejectsWrongThread(t *testing.T) {
	p := NewCodexProvider()
	p.recordGuardianNotification("item/autoApprovalReview/completed", json.RawMessage(`{
		"threadId":"thread-a","reviewId":"rev-a","review":{"status":"denied"},
		"action":{"type":"command","command":"ls","cwd":"C:\\","source":"shell"}}`))
	if p.GuardianLatest("thread-a") == nil {
		t.Fatal("前置条件不成立:应当记录到可推翻的结论")
	}
	// 用另一个线程去推翻它:必须被挡住(否则会串会话执行动作)
	if err := p.ApproveGuardianDenied("thread-b", "rev-a", ""); err == nil {
		t.Fatal("跨会话推翻应当被拒绝")
	}
}

func TestGuardianRecordBounded(t *testing.T) {
	p := NewCodexProvider()
	thread := "thread-z"
	for i := 0; i < guardianKeep+50; i++ {
		p.recordGuardianNotification("item/autoApprovalReview/completed", json.RawMessage(`{
			"threadId":"`+thread+`","reviewId":"rev-`+itoa(i)+`","review":{"status":"inProgress"},
			"action":{"type":"command","command":"ls","cwd":"C:\\","source":"shell"}}`))
	}
	p.gMu.Lock()
	n := len(p.guardianReviews)
	p.gMu.Unlock()
	if n > guardianKeep {
		t.Errorf("记录数无上限增长: %d > %d", n, guardianKeep)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
