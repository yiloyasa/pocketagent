// codex_fieldcoverage_20261007_test.go —— 2026-10-07 codex 0.160.0 字段级全适配
// 审计批的钉子。对照物:本机安装的 codex 0.160.0(npm 全局 @openai/codex,
// 只读翻二进制请求面与 vendored 0.159.2 schema 快照,两者逐名对齐),
// 覆盖矩阵见 docs/reviews/2026-10-07/field-coverage-codex.md。
//
// 【钉什么】
//  1. currentTime/read(0.160.0 新增反向请求):归类为 reqCurrentTime,
//     daemon 以 {"currentTimeAt": <unix 秒>} 如实应答(fail-closed 语义,
//     不应答本轮直接停);
//  2. turn/completed 的结束原因(Turn.status):interrupted 出中断卡、
//     failed 无 error 对象按失败收口,不冒充成功;
//  3. 工具类 ThreadItem(fileChange/mcpToolCall/webSearch):投影成
//     工具卡(STARTED/FINISHED),不再降级为不透明透传;
//  4. modelProvider/authRecovery 两连:分类升为 PROJECTED 且出 WARNING 卡;
//  5. thread/status/changed 与 account/rateLimits/updated 的枚举/字段人话
//     (状态枚举、activeFlags、resetsAt、windowDurationMins、
//     rateLimitReachedType);
//  6. guardian 动作第七变体 writeStdin 的审查卡人话。
//
// 全部用 vendored schema / 活体探针(docs/verify/evidence/
// codex-01592-liveturn-20261002.log)里的 payload 形状构造,不依赖本机 codex。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- 1) currentTime/read ----

func TestClassifyCurrentTimeRead(t *testing.T) {
	if got := classifyCodexRequest("currentTime/read"); got != reqCurrentTime {
		t.Fatalf("classifyCodexRequest(currentTime/read) = %d, want reqCurrentTime(0.160.0 新增反向请求)", got)
	}
	// 旧方法归类不受影响(回归钉)。
	for m, want := range map[string]codexRequestKind{
		"item/commandExecution/requestApproval": reqApprovalCommand,
		"item/fileChange/requestApproval":       reqApprovalFileChange,
		"applyPatchApproval":                    reqApprovalPatch,
		"item/permissions/requestApproval":      reqApprovalPermissions,
		"item/tool/requestUserInput":            reqAskUser,
		"mcpServer/elicitation/request":         reqAskMcp,
		"item/tool/call":                        reqUnsupported,
		"attestation/generate":                  reqUnsupported,
		"some/future/method":                    reqUnknown,
	} {
		if got := classifyCodexRequest(m); got != want {
			t.Errorf("classifyCodexRequest(%q) = %d, want %d", m, got, want)
		}
	}
}

// TestCodexCurrentTimeReadAnswered 活体链路:fake app-server 在轮内反向问时,
// daemon 必须回 {"currentTimeAt": <unix 秒>}(fail-closed:不回/回错形状本轮即停),
// 且本轮照常收口(fake 应答 turn/completed 后 Run 正常返回)。
func TestCodexCurrentTimeReadAnswered(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_CURRENT_TIME", "1")
	ws := t.TempDir()
	runOneTurn(t, p, &store.Thread{ID: "daemon-ct"}, ws)

	reqs := fakeCodexRequests(t, logPath)
	var answered bool
	for _, r := range reqs {
		if r.Method != "<client-response>" {
			continue
		}
		var frame struct {
			ID     int64 `json:"id"`
			Result struct {
				CurrentTimeAt int64 `json:"currentTimeAt"`
			} `json:"result"`
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(r.Params, &frame) != nil {
			continue
		}
		if frame.ID != 777001 {
			continue
		}
		if len(frame.Error) > 0 && string(frame.Error) != "null" {
			t.Fatalf("currentTime/read 应答带了 error(应如实回时间): %s", string(frame.Error))
		}
		if frame.Result.CurrentTimeAt <= 0 {
			t.Fatalf("currentTime/read 应答应带正数 currentTimeAt(unix 秒), got=%d", frame.Result.CurrentTimeAt)
		}
		answered = true
	}
	if !answered {
		t.Fatalf("fake 没有记录到 id=777001 的 currentTime/read 应答帧 —— daemon 未应答或形状不符, reqs=%v", reqs)
	}
}

// ---- 2) turn/completed 的结束原因 ----

// TestCodexTurnInterruptedEmitsCard 状态 interrupted:部分正文照常交回,
// 且必须出一张"本轮已中断"卡 —— 用户得知道话为什么说到一半停了。
func TestCodexTurnInterruptedEmitsCard(t *testing.T) {
	p, _ := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_TURN_STATUS", "interrupted")
	ws := t.TempDir()
	msgs := runOneTurn(t, p, &store.Thread{ID: "daemon-int"}, ws)
	found := false
	for _, m := range msgs {
		if ev := m.GetCodexEvent(); ev != nil && ev.Code == "turnInterrupted" {
			found = true
			if ev.Kind != pb.CodexEvent_WARNING {
				t.Fatalf("中断卡应为 WARNING, got=%v", ev.Kind)
			}
			if !strings.Contains(ev.Title, "中断") {
				t.Fatalf("中断卡标题应说明中断, got=%q", ev.Title)
			}
		}
	}
	if !found {
		t.Fatalf("turn/completed(status=interrupted) 未产生中断卡, msgs=%d 条", len(msgs))
	}
}

// TestCodexTurnFailedWithoutErrorSurfaces failed 但不带 error 对象:
// 如实按失败收口(Run 返回 codexRunError),不冒充成功。
func TestCodexTurnFailedWithoutErrorSurfaces(t *testing.T) {
	p, _ := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_TURN_STATUS", "failed")
	ws := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, err := p.Run(ctx, agentTurnRequest(&store.Thread{ID: "daemon-fail"}, ws), func(*pb.TurnServerMessage) {}, nil)
	if err == nil {
		t.Fatalf("turn.status=failed 应让 Run 以失败收口,得到 nil")
	}
	var runErr *codexRunError
	if !asCodexRunError(err, &runErr) {
		t.Fatalf("失败应包装成 codexRunError, got=%T(%v)", err, err)
	}
	if runErr.ErrorInfo() == nil || runErr.ErrorInfo().Code != "turnFailed" {
		t.Fatalf("失败原因应带 code=turnFailed, got=%+v", runErr.ErrorInfo())
	}
}

// ---- 3) 工具类 ThreadItem 的工具卡 ----

func TestCodexItemToolCardShapes(t *testing.T) {
	const tid = "th-x"
	// 形状按 vendored schema:FileChangeThreadItem / McpToolCallThreadItem /
	// WebSearchThreadItem(0.160.0 二进制同面)。
	fileStarted := `{"threadId":"` + tid + `","turnId":"tn","startedAtMs":1,"item":{"type":"fileChange","id":"fc-1","status":"inProgress","changes":[{"path":"a.txt","diff":"x","kind":{"type":"update"}},{"path":"b.txt","diff":"y","kind":{"type":"add"}}]}}`
	fileDone := `{"threadId":"` + tid + `","turnId":"tn","completedAtMs":2,"item":{"type":"fileChange","id":"fc-1","status":"completed","changes":[{"path":"a.txt","diff":"x","kind":{"type":"update"}}]}}`
	mcpDone := `{"threadId":"` + tid + `","turnId":"tn","completedAtMs":3,"item":{"type":"mcpToolCall","id":"mc-1","status":"failed","server":"fs","tool":"read_file","error":{"message":"file not found"}}}`
	webDone := `{"threadId":"` + tid + `","turnId":"tn","completedAtMs":4,"item":{"type":"webSearch","id":"ws-1","query":"pocketagent"}}`

	s := codexItemToolCard("item/started", json.RawMessage(fileStarted), tid)
	if s == nil || s.GetToolCall() == nil {
		t.Fatalf("fileChange 开始卡缺失: %+v", s)
	}
	tc := s.GetToolCall()
	if tc.Phase != pb.ToolCallUpdate_STARTED || tc.Call.Name != "file_change" || tc.Call.Id != "fc-1" {
		t.Fatalf("fileChange 开始卡形状不符: %+v", tc)
	}
	var files struct {
		Files []string `json:"files"`
	}
	if json.Unmarshal([]byte(tc.Call.JsonArgs), &files) != nil || len(files.Files) != 2 || files.Files[0] != "a.txt" {
		t.Fatalf("fileChange 开始卡应带路径清单(不带 diff), got=%s", tc.Call.JsonArgs)
	}

	d := codexItemToolCard("item/completed", json.RawMessage(fileDone), tid)
	if d == nil || d.GetToolCall() == nil || d.GetToolCall().Phase != pb.ToolCallUpdate_FINISHED {
		t.Fatalf("fileChange 收尾卡缺失或阶段不对: %+v", d)
	}
	if res := d.GetToolCall().Result; res == nil || !res.Ok || !strings.Contains(res.Output, "1 个文件") {
		t.Fatalf("fileChange 收尾结果应 Ok 且带文件数, got=%+v", d.GetToolCall().Result)
	}

	m := codexItemToolCard("item/completed", json.RawMessage(mcpDone), tid)
	if m == nil || m.GetToolCall() == nil || m.GetToolCall().Call.Name != "mcp_tool" {
		t.Fatalf("mcpToolCall 收尾卡缺失: %+v", m)
	}
	if res := m.GetToolCall().Result; res == nil || res.Ok || !strings.Contains(res.Output, "file not found") {
		t.Fatalf("mcpToolCall 失败结果应 Ok=false 且带错误消息, got=%+v", m.GetToolCall().Result)
	}

	w := codexItemToolCard("item/completed", json.RawMessage(webDone), tid)
	if w == nil || w.GetToolCall() == nil || w.GetToolCall().Call.Name != "web_search" {
		t.Fatalf("webSearch 收尾卡缺失: %+v", w)
	}

	// 异线程/未知类型 → nil(调用方回落既有通路),不伪造卡片。
	if codexItemToolCard("item/completed", json.RawMessage(fileDone), "th-other") != nil {
		t.Fatalf("异线程 item 不应出卡")
	}
	unknown := `{"threadId":"` + tid + `","item":{"type":"futureThing","id":"x"}}`
	if codexItemToolCard("item/completed", json.RawMessage(unknown), tid) != nil {
		t.Fatalf("未知 item 类型不应出卡(留给透传/台账)")
	}
}

// ---- 4) authRecovery 升为投影 ----

func TestCodexAuthRecoveryProjected(t *testing.T) {
	for _, m := range []string{"modelProvider/authRecoveryStarted", "modelProvider/authRecoveryCompleted"} {
		if got := codexGlobalNoticeKind(m); got != runtime.KindKnownProjected {
			t.Fatalf("%s 应为 PROJECTED(2026-10-07 升级), got=%s", m, got)
		}
	}
	ev := codexNotificationEvent("modelProvider/authRecoveryStarted",
		[]byte(`{"threadId":"th-a","turnId":"tn","provider":"openai","message":"refresh token expired"}`), "th-a", "")
	if ev == nil || ev.Kind != pb.CodexEvent_WARNING {
		t.Fatalf("authRecoveryStarted 应投影成 WARNING 卡, got=%+v", ev)
	}
	if !strings.Contains(ev.Message, "openai") || !strings.Contains(ev.Message, "refresh token expired") {
		t.Fatalf("WARNING 卡应带 provider 与原文消息, got=%q", ev.Message)
	}
}

// ---- 5) 状态/限额的人话与字段 ----

func TestCodexThreadStatusLabels(t *testing.T) {
	cases := map[string]string{
		"notLoaded": "未加载", "idle": "空闲", "active": "执行中", "systemError": "系统异常",
		"brandNewState": "brandNewState", // 枚举外原样透出,不猜
	}
	for in, want := range cases {
		if got := codexThreadStatusLabel(in); got != want {
			t.Errorf("codexThreadStatusLabel(%q)=%q, want %q", in, got, want)
		}
	}
	if got := codexThreadFlagsLabel([]string{"waitingOnApproval", "waitingOnUserInput", "oddFlag"}); got != "等待审批、等待输入、oddFlag" {
		t.Errorf("activeFlags 人话不符: %q", got)
	}
	ev := codexNotificationEvent("thread/status/changed",
		[]byte(`{"threadId":"th-a","status":{"type":"active","activeFlags":["waitingOnApproval"]}}`), "th-a", "")
	if ev == nil || !strings.Contains(ev.Message, "执行中") || !strings.Contains(ev.Message, "等待审批") {
		t.Fatalf("thread/status/changed 卡应出人话, got=%+v", ev)
	}
}

func TestCodexRateLimitFields(t *testing.T) {
	// 形状按 schema RateLimitSnapshot:primary/secondary 可空,resetsAt 为 unix 秒。
	ev := codexNotificationEvent("account/rateLimits/updated", []byte(`{
		"threadId":"th-a",
		"rateLimits":{"planType":"plus",
			"primary":{"usedPercent":100,"resetsAt":1791990400,"windowDurationMins":300},
			"secondary":{"usedPercent":40},
			"rateLimitReachedType":"rate_limit_reached"}}`), "th-a", "")
	if ev == nil || ev.Kind != pb.CodexEvent_RATE_LIMIT {
		t.Fatalf("限额卡缺失: %+v", ev)
	}
	for _, want := range []string{"主要窗口已使用 100%", "次要窗口已使用 40%", "主窗口 300 分钟", "重置", "已触发限流"} {
		if !strings.Contains(ev.Message, want) {
			t.Errorf("限额卡应含 %q, got=%q", want, ev.Message)
		}
	}
	// 空窗与无限流类型:卡片照常出,不追加空段。
	ev2 := codexNotificationEvent("account/rateLimits/updated",
		[]byte(`{"threadId":"th-a","rateLimits":{}}`), "th-a", "")
	if ev2 == nil || ev2.Message == "" {
		t.Fatalf("空限额载荷也应出卡: %+v", ev2)
	}
}

func TestCodexRateLimitReachedLabels(t *testing.T) {
	cases := map[string]string{
		"":                                     "",
		"rate_limit_reached":                   "已触发限流",
		"workspace_owner_credits_depleted":     "工作区额度已用尽",
		"workspace_member_usage_limit_reached": "工作区成员",
		"brand_new_reason":                     "限流原因:brand_new_reason",
	}
	for in, want := range cases {
		got := codexRateLimitReachedLabel(in)
		if want == "" {
			if got != "" {
				t.Errorf("空类型应返回空串, got=%q", got)
			}
			continue
		}
		if !strings.Contains(got, want) {
			t.Errorf("codexRateLimitReachedLabel(%q)=%q, 应含 %q", in, got, want)
		}
	}
}

// ---- 6) guardian writeStdin 动作标签 ----

func TestGuardianActionLabelWriteStdin(t *testing.T) {
	// 0.160.0 GuardianApprovalReviewAction oneOf[2]:{type:"writeStdin",
	// approvalId, processId, stdin, cwd}。
	label := guardianActionLabel(json.RawMessage(`{"type":"writeStdin","approvalId":"ap-1","processId":"p-9","stdin":"y\n","cwd":"C:/w"}`))
	if !strings.Contains(label, "写入输入") || !strings.Contains(label, "y") {
		t.Fatalf("writeStdin 审查动作应翻成人话, got=%q", label)
	}
	// 回写 codex 侧未知类型原样透传(不猜 snake_case,与既有政策一致)。
	wire, err := guardianActionToWire(json.RawMessage(`{"type":"writeStdin","stdin":"y\n"}`))
	if err != nil || wire["type"] != "writeStdin" {
		t.Fatalf("writeStdin 转写应原样透传, got=%+v err=%v", wire, err)
	}
}

// ---- helpers ----

func asCodexRunError(err error, target **codexRunError) bool {
	if e, ok := err.(*codexRunError); ok {
		*target = e
		return true
	}
	return false
}
