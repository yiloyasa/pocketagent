// unknown_event_e2e_test.go —— 未知事件 E2E(发布验证计划 2026-10-04 Task 3)。
//
// 【与单测的分工】unknown_actions_test / unknown_events_test / unknown_events_
// summary_test 钉的是各层零件;本文件把**真 Server + 真台账(落盘)+ testagent
// 注入的未知事件**接成完整轮次链路:StartTurn 真流 → 真 provider 调度 → 真审批
// 通路(feedTurnStream 只是线级替身,服务器的审批中继/取消/落库全部是真的)→
// 真 RPC 消费台账。七条验证逐条一个测试函数:
//
//	① TestUnknownE2ENotificationDoesNotEndTurn   未知通知不结束轮次,台账有条目,协议帧零泄漏;
//	② TestUnknownE2ERequestNeverForgesApproval   未知反向请求走真审批通路被"用户拒绝",
//	                                              台账 UNKNOWN_REQUEST(安全拒绝),轮次照常完成,
//	                                              任何路径不产生 approve(KNOWN_PASSTHROUGH 零条目);
//	③ TestUnknownE2ESafeActionsWithRevision      摘要动作表含下划线四动作;执行器带当前
//	                                              revision 成功、带旧 revision 回结构化冲突;
//	④ TestUnknownE2ETechnicalDetailStaysRedacted 技术详情全链路只透出 [redacted] 占位;
//	⑤ TestUnknownE2EDispositionSurvivesRebuild   处置状态跨 daemon 重建(同 dataDir)持久;
//	⑥ TestUnknownE2EOldLedgerEntryStillGetsPresentation
//	                                              旧形态台账条目(新字段全缺)读出时人话摘要
//	                                              照常生成(PresentUnknownEvent 不依赖新字段);
//	⑦ TestUnknownE2EDetailFailureStaysHumanReadable
//	                                              详情失败不甩堆栈:损坏台账 fail-soft 为空 +
//	                                              人话 NotFound;坏载荷条目透析留空、翻译兜底。
//
// 【同构保证】testagent.FakeAgent 编译期钉住与真 provider 的完整能力面
// (agent.Provider/Steerer/CollabAware/ErrorInfoCarrier/TestFakeSurface),
// server 对二者不可区分;provider 表(Config.Providers)是唯一灌入点。
// 已知差异(如实声明):fake 的台账写入按脚本原文裁剪落账、**不做** redactNativePayload
// —— 真实通道的脱敏发生在 agent 层登记之前(native_notifications.go),fake 把它
// 委托给剧本。因此验证 ④ 的剧本按"登记后形态"注入凭据(键名风格取 stderrdrain
// 脱敏先例的样例),钉的是"台账 → 详情 RPC → 动作 inspect"全链路不再复露凭据;
// redactNativePayload 本身的钉子在 agent 层(codex_item_passthrough_test.go)。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/testagent"
)

const (
	// unknownE2EAgentID fake provider 在 provider 表里的键(线程 AgentID 与之一致)。
	unknownE2EAgentID = "fake"
	// unknownE2ENotifyMethod 未知通知的方法名(词表外,真实的"没见过"形态)。
	unknownE2ENotifyMethod = "brandNew/notify"
	// unknownE2ENotifyMarker 载荷探针串:绝不允许出现在任何 TurnServerMessage 帧
	// 或会话历史里 —— 它只该活 in 台账。
	unknownE2ENotifyMarker = "UNKNOWN-NOTIFY-E2E-MARKER-7f3a"
	// unknownE2EReqMethod 未知反向请求的方法(被"用户"拒绝的那个)。
	unknownE2EReqMethod = "native/remote.exec"
	// unknownE2EReqParams 该请求的参数原文。
	unknownE2EReqParams = `{"cmd":"deploy --all"}`
	// unknownE2ERespCallID/unknownE2ERespRaw 对不上在途请求的响应(UNKNOWN_RESPONSE)。
	unknownE2ERespCallID = "resp-42"
	unknownE2ERespRaw    = `{"result":"orphan"}`
)

// unknownE2EFile 与 runtime 的 unknown-events.json 同名(runtime 内常量未导出;
// 两处手写一份,改名时编译不了的那种护栏由 runtime 的落盘测试把着)。
const unknownE2EFile = "unknown-events.json"

// ---- 环境装配 ---------------------------------------------------------------

// newUnknownE2EEnv 造一套"真 Server + 真台账(落盘)+ fake provider"的 E2E 环境:
// store / runtime 后端 / 事件存储落在同一个临时 dataDir,fake 的台账经 Factory.Ledger
// 写进同一后端 —— 与生产同构(provider 与 server 共享进程后端,同一本账)。
func newUnknownE2EEnv(t *testing.T, sc testagent.Scenario) (*Server, *store.Thread, *testagent.FakeAgent) {
	t.Helper()
	dir := t.TempDir()
	backend := runtime.NewFileBackend(dir)
	factory := &testagent.Factory{Ledger: backend}
	fake := factory.Provider(unknownE2EAgentID, sc).(*testagent.FakeAgent)
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{
		Store:          st,
		Providers:      map[string]agent.Provider{unknownE2EAgentID: fake},
		RuntimeBackend: backend,
		DataDir:        dir,
		Workspace:      dir,
	})
	t.Cleanup(func() { _ = srv.events.Close() })
	th := &store.Thread{
		ID: "th-ue-e2e", Title: "未知事件 E2E", Workspace: dir,
		AgentID: unknownE2EAgentID, ModelID: unknownE2EAgentID + "/m1",
		Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, th, fake
}

// newUnknownE2EServerAt 在既有 dataDir 上重建 Server(store 与 runtime 后端全部
// 重新打开同一目录)—— "daemon 进程重启"的测试同构,处置持久化/损坏台账场景用。
func newUnknownE2EServerAt(t *testing.T, dir string) *Server {
	t.Helper()
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("重建 store 失败: %v", err)
	}
	srv := New(Config{Store: st, RuntimeBackend: runtime.NewFileBackend(dir), DataDir: dir})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

// startUnknownE2ETurn 用一条真 StartTurn 流跑一轮;返回流(供轮中交互与帧快照)
// 与收尾函数(close(in) = 手机结束发送,再等 StartTurn 返回)。
func startUnknownE2ETurn(t *testing.T, srv *Server, th *store.Thread) (*feedTurnStream, func()) {
	t.Helper()
	stream := &feedTurnStream{ctx: context.Background(), in: make(chan *pb.TurnClientMessage, 4)}
	stream.push(t, textInput(th.ID, "跑一轮未知事件剧情"))
	done := make(chan error, 1)
	go func() { done <- srv.StartTurn(stream) }()
	return stream, func() {
		close(stream.in)
		if err := <-done; err != nil {
			t.Fatalf("StartTurn: %v", err)
		}
	}
}

// rejectUnknownE2ERequest 等 unknown_request 通用确认卡出现在真审批通路上,再以
// "用户拒绝"应答 —— 走 TurnClientMessage_Approval 这条与真手机完全相同的链路
// (不是测试捷径:server 的审批中继/挂起/结算全程真实参与)。返回被拒绝的 request_id。
func rejectUnknownE2ERequest(t *testing.T, stream *feedTurnStream) string {
	t.Helper()
	reqID := ""
	waitFor(t, "unknown_request 确认卡出现在审批通路", func() bool {
		for _, m := range stream.snapshot() {
			if r := m.GetApprovalRequest(); r != nil && r.GetAction().GetName() == "unknown_request" {
				reqID = r.GetRequestId()
				return true
			}
		}
		return false
	})
	stream.push(t, &pb.TurnClientMessage{Msg: &pb.TurnClientMessage_Approval{Approval: &pb.ApprovalDecision{
		RequestId: reqID,
		Approved:  false,
		Note:      "E2E:用户明确拒绝未知请求",
	}}})
	return reqID
}

// ---- 断言辅助 ---------------------------------------------------------------

// unknownE2EFrameLeaks 扫全部 TurnServerMessage 帧的 JSON 形态,返回包含任何敏感串
// 的帧描述(空 = 无泄漏)。未知事件只许走台账,绝不允许有任何协议帧。
func unknownE2EFrameLeaks(frames []*pb.TurnServerMessage, needles ...string) []string {
	var hits []string
	for i, m := range frames {
		raw, err := json.Marshal(m)
		if err != nil {
			hits = append(hits, fmt.Sprintf("frame#%d 无法序列化: %v", i, err))
			continue
		}
		s := string(raw)
		for _, n := range needles {
			if strings.Contains(s, n) {
				hits = append(hits, fmt.Sprintf("frame#%d 泄漏 %q", i, n))
			}
		}
	}
	return hits
}

// unknownE2EThreadText 拼出线程落库消息里的全部文本(断言"未知事件不进会话历史")。
func unknownE2EThreadText(th *store.Thread) string {
	var b strings.Builder
	for _, m := range th.Messages {
		for i := range m.Blocks {
			if tb, ok := m.Blocks[i].Block.(*pb.ContentBlock_Text); ok {
				b.WriteString(tb.Text.Text)
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

// unknownE2EAssertNoAllowAction 安全红线:动作表不得出现"允许"类动作(UNKNOWN_*
// 默认不显示"允许"按钮),所有动作必须声明 Safe 且非破坏(daemon 只发安全动作)。
func unknownE2EAssertNoAllowAction(t *testing.T, where string, actions []*pb.UserActionDescriptor) {
	t.Helper()
	for _, a := range actions {
		id := strings.ToLower(a.GetActionId())
		for _, banned := range []string{"allow", "approve", "confirm", "execute", "grant"} {
			if strings.Contains(id, banned) {
				t.Fatalf("%s 的动作表出现允许类动作 %q", where, a.GetActionId())
			}
		}
		if !a.GetSafe() || a.GetDestructive() {
			t.Fatalf("%s 的动作 %q 安全声明不对: safe=%v destructive=%v",
				where, a.GetActionId(), a.GetSafe(), a.GetDestructive())
		}
	}
}

// unknownE2ESecretSamples stderrdrain 脱敏先例的凭据样例(api_key=sk-…/Bearer
// eyJ…/token: 'glm-…'/DEEPSEEK dsk-…)。它们绝不允许出现在任何未知事件详情里。
func unknownE2ESecretSamples() []string {
	return []string{
		"sk-abcdefghijklmnopqrstuvwxyz012345",
		"eyJhbGciOiJIUzI1NiJ9",
		"glm-ABCDEFGHIJKLMNOPQRSTUVWXYZ",
		"dsk-1234567890abcdefg",
	}
}

// ---- 验证 ①:notification 不结束轮次 -----------------------------------------

func TestUnknownE2ENotificationDoesNotEndTurn(t *testing.T) {
	sc := testagent.Scenario{Name: "unknown-e2e-notification", Events: []testagent.AgentEvent{
		testagent.Text("分析中"),
		testagent.UnknownNotification(unknownE2ENotifyMethod,
			`{"progress":3,"total":9,"marker":"`+unknownE2ENotifyMarker+`"}`),
		testagent.Text("继续收尾"),
		testagent.Completed("完成"),
	}}
	srv, th, _ := newUnknownE2EEnv(t, sc)
	stream, waitDone := startUnknownE2ETurn(t, srv, th)
	waitDone()
	frames := stream.snapshot()

	// 轮次正常结束:恰一个 Completed、零 Failed;线程回 IDLE。
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetCompleted() != nil }); n != 1 {
		t.Fatalf("未知通知不应结束轮次:Completed=%d(帧序 %s)", n, frameNames(frames))
	}
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetFailed() != nil }); n != 0 {
		t.Fatalf("未知通知不应把轮次打成失败(帧序 %s)", frameNames(frames))
	}
	if got, _ := srv.cfg.Store.Get(th.ID); got.Status != pb.ThreadStatus_THREAD_STATUS_IDLE {
		t.Fatalf("轮次结束后线程应回 IDLE,得到 %v", got.Status)
	}

	// TurnServerMessage 无该事件帧:方法名与载荷标记不得出现在任何帧里。
	if leaks := unknownE2EFrameLeaks(frames, unknownE2ENotifyMethod, unknownE2ENotifyMarker); len(leaks) != 0 {
		t.Fatalf("未知通知泄漏进了协议帧: %v", leaks)
	}
	// 也不进会话历史:落库文本只有正常对话,终稿原样在。
	thGot, _ := srv.cfg.Store.Get(th.ID)
	hist := unknownE2EThreadText(thGot)
	if strings.Contains(hist, unknownE2ENotifyMethod) || strings.Contains(hist, unknownE2ENotifyMarker) {
		t.Fatalf("未知通知泄漏进会话历史: %q", hist)
	}
	if !strings.Contains(hist, "完成") {
		t.Fatalf("终稿应正常落库: %q", hist)
	}

	// 台账有条目:kind/来源/方法/处置状态俱全,并带人话呈现。
	resp, err := srv.ListUnknownEvents(context.Background(), &pb.ListUnknownEventsRequest{
		Kind: runtime.KindUnknownNotification, AgentId: unknownE2EAgentID,
	})
	if err != nil {
		t.Fatalf("ListUnknownEvents: %v", err)
	}
	if len(resp.GetItems()) != 1 || resp.GetTotal() < 1 {
		t.Fatalf("台账应有 1 条未知通知,得到 %d 条(total=%d)", len(resp.GetItems()), resp.GetTotal())
	}
	item := resp.GetItems()[0]
	if item.GetSourceMethod() != unknownE2ENotifyMethod || item.GetSourceAgent() != unknownE2EAgentID ||
		item.GetProcessingStatus() != runtime.StatusNew {
		t.Fatalf("台账条目字段不对: %+v", item)
	}
	if item.GetPlainTitle() != "电脑端传来了新消息" || item.GetImpact() != "不影响正在进行的任务" {
		t.Fatalf("摘要人话字段缺失: %+v", item)
	}
	unknownE2EAssertNoAllowAction(t, "未知通知摘要", item.GetActions())
	detail, err := srv.GetUnknownEvent(context.Background(), &pb.GetUnknownEventRequest{EventId: item.GetEventId()})
	if err != nil {
		t.Fatalf("GetUnknownEvent: %v", err)
	}
	if !strings.Contains(detail.GetRawJson(), unknownE2ENotifyMarker) {
		t.Fatalf("台账原文应保真: %q", detail.GetRawJson())
	}
}

// ---- 验证 ②:unknown request 不伪造批准 --------------------------------------
//
// "-32601 类安全拒绝"的 wire 形态发生在真 provider 内部(dsh/codex 的未知请求分支
// 回 RespondError(-32601),见 agent 层钉子);fake 世界里它的可观测对应物是:
// 确认卡走真审批通路 → 被拒 → 台账 UNKNOWN_REQUEST(安全拒绝语义)+ 轮次照常完成。
func TestUnknownE2ERequestNeverForgesApproval(t *testing.T) {
	sc := testagent.Scenario{Name: "unknown-e2e-request", Events: []testagent.AgentEvent{
		testagent.Text("开始"),
		testagent.UnknownRequest(unknownE2EReqMethod, unknownE2EReqParams),
		testagent.Completed("完成"),
	}}
	srv, th, fake := newUnknownE2EEnv(t, sc)
	stream, waitDone := startUnknownE2ETurn(t, srv, th)
	reqID := rejectUnknownE2ERequest(t, stream)
	waitDone()
	frames := stream.snapshot()

	// 审批通路:恰好一张确认卡 + 一张结算卡;卡面如实是"未识别的操作",
	// 不冒充任何已知动作类型。
	var cards, resolved int
	for _, m := range frames {
		if r := m.GetApprovalRequest(); r != nil {
			cards++
			if r.GetAction().GetName() != "unknown_request" {
				t.Fatalf("确认卡不得冒充已知动作类型: %q", r.GetAction().GetName())
			}
			if r.GetReason() != "agent 请求执行一个未识别的操作" {
				t.Fatalf("确认卡理由不对: %q", r.GetReason())
			}
		}
		if d := m.GetApprovalResolved(); d != nil {
			resolved++
			if d.GetRequestId() != reqID {
				t.Fatalf("结算卡应回执同一 request_id: %q ≠ %q", d.GetRequestId(), reqID)
			}
		}
	}
	if cards != 1 || resolved != 1 {
		t.Fatalf("确认/结算卡应各一张,得到 %d/%d(帧序 %s)", cards, resolved, frameNames(frames))
	}

	// fake 观察面:卡进过审批钩子,但决定永远是"不允许"—— 任何路径都没产生 approve。
	apprs := fake.Approvals()
	if len(apprs) != 1 {
		t.Fatalf("应恰一次未知请求确认,得到 %d", len(apprs))
	}
	obs := apprs[0]
	if obs.Allow {
		t.Fatal("红线:未知请求被批准了!任何路径都不得产生 approve")
	}
	if obs.Err != nil {
		t.Fatalf("用户拒绝路不应带错误: %v", obs.Err)
	}
	if !strings.Contains(obs.Call.GetJsonArgs(), "未识别的操作") ||
		!strings.Contains(obs.Call.GetJsonArgs(), unknownE2EReqMethod) {
		t.Fatalf("确认卡应带 reason 与 method 原文: %q", obs.Call.GetJsonArgs())
	}

	// 轮次不因此失败(按真实语义:agent 收到安全拒绝后继续跑):照常 Completed、
	// 线程回 IDLE。
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetCompleted() != nil }); n != 1 {
		t.Fatalf("未知请求被拒后轮次应照常完成(帧序 %s)", frameNames(frames))
	}
	if n := countFrame(frames, func(m *pb.TurnServerMessage) bool { return m.GetFailed() != nil }); n != 0 {
		t.Fatalf("未知请求被拒不得打成轮次失败(帧序 %s)", frameNames(frames))
	}
	if got, _ := srv.cfg.Store.Get(th.ID); got.Status != pb.ThreadStatus_THREAD_STATUS_IDLE {
		t.Fatalf("线程应回 IDLE,得到 %v", got.Status)
	}

	// 台账:UNKNOWN_REQUEST 恰一条(安全拒绝),绝无 KNOWN_PASSTHROUGH(批准路痕迹)。
	ctx := context.Background()
	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{Kind: runtime.KindUnknownRequest})
	if err != nil {
		t.Fatalf("ListUnknownEvents: %v", err)
	}
	if len(resp.GetItems()) != 1 {
		t.Fatalf("应恰 1 条 UNKNOWN_REQUEST 台账,得到 %d", len(resp.GetItems()))
	}
	e := resp.GetItems()[0]
	if e.GetSourceMethod() != unknownE2EReqMethod || e.GetProcessingStatus() != runtime.StatusNew {
		t.Fatalf("台账条目不对: %+v", e)
	}
	passthrough, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{Kind: runtime.KindKnownPassthrough})
	if err != nil {
		t.Fatalf("ListUnknownEvents: %v", err)
	}
	if len(passthrough.GetItems()) != 0 {
		t.Fatalf("红线:出现了 KNOWN_PASSTHROUGH(批准路)条目: %+v", passthrough.GetItems())
	}
	// 详情人话:进「需要我处理」、下一步指向电脑端、动作表无允许类。
	detail, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: e.GetEventId()})
	if err != nil {
		t.Fatalf("GetUnknownEvent: %v", err)
	}
	if !detail.GetSummary().GetRequiresUser() {
		t.Fatalf("未知请求应进「需要我处理」: %+v", detail.GetSummary())
	}
	if !strings.Contains(detail.GetRecommendedNextStep(), "电脑端") {
		t.Fatalf("下一步应指向电脑端: %q", detail.GetRecommendedNextStep())
	}
	// "-32601 类安全拒绝"对用户可见的形态:详情处置句如实写明"已回协议错误拒绝"。
	if !strings.Contains(detail.GetTranslation(), "已回协议错误拒绝") {
		t.Fatalf("详情处置句应写明安全拒绝(-32601 语义): %q", detail.GetTranslation())
	}
	unknownE2EAssertNoAllowAction(t, "未知请求详情", detail.GetSummary().GetActions())
}

// ---- 验证 ③:safe action 带真实动作与 revision --------------------------------

func TestUnknownE2ESafeActionsWithRevision(t *testing.T) {
	sc := testagent.Scenario{Name: "unknown-e2e-actions", Events: []testagent.AgentEvent{
		testagent.UnknownNotification(unknownE2ENotifyMethod, `{"progress":3,"total":9}`),
		testagent.UnknownRequest(unknownE2EReqMethod, unknownE2EReqParams),
		testagent.UnknownResponse(unknownE2ERespCallID, unknownE2ERespRaw),
		testagent.Completed("完成"),
	}}
	srv, th, _ := newUnknownE2EEnv(t, sc)
	stream, waitDone := startUnknownE2ETurn(t, srv, th)
	rejectUnknownE2ERequest(t, stream)
	waitDone()

	ctx := context.Background()
	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("ListUnknownEvents: %v", err)
	}
	if len(resp.GetItems()) != 3 {
		t.Fatalf("一轮应落 3 条台账(通知/请求/响应),得到 %d", len(resp.GetItems()))
	}
	byKind := map[string]*pb.UnknownEventSummary{}
	for _, it := range resp.GetItems() {
		byKind[it.GetNormalizedKind()] = it
	}
	// 各 kind 的动作表(下划线口径)与 runtime 固定动作表逐一一致。
	wantActions := map[string][]string{
		runtime.KindUnknownNotification: {"inspect", "copy", "acknowledge"},
		runtime.KindUnknownRequest:      {"inspect", "copy", "open_native", "retry_later"},
		runtime.KindUnknownResponse:     {"inspect", "copy"},
	}
	seen := map[string]bool{}
	for kind, want := range wantActions {
		item, ok := byKind[kind]
		if !ok {
			t.Fatalf("缺 %s 的台账条目", kind)
		}
		if len(item.GetActions()) != len(want) {
			t.Fatalf("%s 动作条数应为 %d: %+v", kind, len(want), item.GetActions())
		}
		for i, id := range want {
			if got := item.GetActions()[i].GetActionId(); got != id {
				t.Fatalf("%s 动作 #%d 应为 %q,得到 %q", kind, i, id, got)
			}
			seen[id] = true
		}
		unknownE2EAssertNoAllowAction(t, kind+" 摘要", item.GetActions())
	}
	// 下划线四动作全部真实可达(执行器白名单 = 展示动作表的执行面)。
	for _, id := range []string{"inspect", "acknowledge", "retry_later", "open_native"} {
		if !seen[id] {
			t.Fatalf("摘要动作表缺下划线动作 %q", id)
		}
	}

	// expected_revision = 当前 → 成功:acknowledge 通知条目。
	notify := byKind[runtime.KindUnknownNotification]
	got, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId:          notify.GetEventId(),
		ExpectedRevision: int64(notify.GetRevision()),
		ActionId:         "acknowledge",
	})
	if err != nil || !got.GetOk() || got.GetConflict() {
		t.Fatalf("带当前 revision 的动作应成功: err=%v got=%+v", err, got)
	}
	if got.GetSummary().GetProcessingStatus() != runtime.StatusInspected || got.GetNewRevision() != 1 {
		t.Fatalf("acknowledge 未生效: %+v", got.GetSummary())
	}

	// retry_later 请求条目:当前 revision(0)→ 成功;之后拿旧 revision(同一份
	// List 快照里的 0)重放 → 结构化冲突结果(不是 error)。
	req := byKind[runtime.KindUnknownRequest]
	got, err = srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: req.GetEventId(), ExpectedRevision: int64(req.GetRevision()), ActionId: "retry_later",
	})
	if err != nil || !got.GetOk() || got.GetConflict() {
		t.Fatalf("retry_later 应成功: err=%v got=%+v", err, got)
	}
	if got.GetSummary().GetProcessingStatus() != runtime.StatusRetryLater {
		t.Fatalf("retry_later 未生效: %+v", got.GetSummary())
	}
	stale, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: req.GetEventId(), ExpectedRevision: int64(req.GetRevision()), ActionId: "retry_later", // 快照里的旧版本
	})
	if err != nil {
		t.Fatalf("冲突不是 error(它是正常业务态): %v", err)
	}
	if stale.GetOk() || !stale.GetConflict() {
		t.Fatalf("旧 revision 重放应回结构化冲突: %+v", stale)
	}
	if !strings.Contains(stale.GetDetail(), "刷新") {
		t.Fatalf("冲突说明应引导客户端刷新: %q", stale.GetDetail())
	}
	if stale.GetNewRevision() != 1 || stale.GetSummary().GetRevision() != 1 ||
		stale.GetSummary().GetProcessingStatus() != runtime.StatusRetryLater {
		t.Fatalf("冲突应带回当前版本与最新摘要: rev=%d summary=%+v", stale.GetNewRevision(), stale.GetSummary())
	}
}

// ---- 验证 ④:技术详情脱敏 ----------------------------------------------------
//
// 真实 provider 在登记前经 redactNativePayload 打码(native_notifications.go);
// fake 把脱敏委托给剧本,故按"登记后形态"注入:凭据键(token/api_key/password,
// 键名风格同 stderrdrain 脱敏表)已是 [redacted] 占位。本测试钉住台账 → 详情 RPC
// → 动作 inspect 全链路只透出占位、绝不复露凭据原文。
func TestUnknownE2ETechnicalDetailStaysRedacted(t *testing.T) {
	payload := `{"threadId":"th-9","token":"[redacted]","api_key":"[redacted]","password":"[redacted]","note":"状态上报"}`
	sc := testagent.Scenario{Name: "unknown-e2e-redaction", Events: []testagent.AgentEvent{
		testagent.UnknownNotification(unknownE2ENotifyMethod, payload),
		testagent.Completed("完成"),
	}}
	srv, th, _ := newUnknownE2EEnv(t, sc)
	_, waitDone := startUnknownE2ETurn(t, srv, th)
	waitDone()

	ctx := context.Background()
	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil || len(resp.GetItems()) != 1 {
		t.Fatalf("台账应有 1 条: err=%v n=%d", err, len(resp.GetItems()))
	}
	detail, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: resp.GetItems()[0].GetEventId()})
	if err != nil {
		t.Fatalf("GetUnknownEvent: %v", err)
	}
	if detail.GetRawJson() != payload {
		t.Fatalf("技术详情原文应与登记形态逐字节一致: %q", detail.GetRawJson())
	}
	// 透析表:凭据键含义如实标注"已脱敏",预览只给占位。
	digest := map[string]*pb.UnknownEventField{}
	for _, f := range detail.GetDigest() {
		digest[f.GetKey()] = f
	}
	for _, key := range []string{"token", "api_key", "password"} {
		f := digest[key]
		if f == nil || f.GetMeaning() != "凭据(已脱敏)" || f.GetPreview() != "[redacted]" {
			t.Fatalf("凭据字段 %q 的透析行不对: %+v", key, f)
		}
	}
	// 详情全量扫描:保留 [redacted] 标记,stderrdrain 风格凭据样例零出现。
	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("序列化详情: %v", err)
	}
	full := string(raw)
	if !strings.Contains(full, "[redacted]") {
		t.Fatal("技术详情应保留 [redacted] 脱敏标记")
	}
	for _, secret := range unknownE2ESecretSamples() {
		if strings.Contains(full, secret) {
			t.Fatalf("技术详情泄漏凭据样例 %q", secret)
		}
	}
	// 动作 inspect 与详情同源:同一份脱敏原文(inspect 纯读,expected=0 免 CAS)。
	got, err := srv.ExecuteUnknownEventAction(ctx, &pb.UnknownEventActionRequest{
		EventId: resp.GetItems()[0].GetEventId(), ExpectedRevision: 0, ActionId: "inspect",
	})
	if err != nil || !got.GetOk() {
		t.Fatalf("inspect 应成功: err=%v got=%+v", err, got)
	}
	if got.GetTechnical().GetRawJson() != payload {
		t.Fatalf("inspect 应与详情同一份脱敏原文: %q", got.GetTechnical().GetRawJson())
	}
}

// ---- 验证 ⑤:处置状态持久化 --------------------------------------------------

func TestUnknownE2EDispositionSurvivesRebuild(t *testing.T) {
	sc := testagent.Scenario{Name: "unknown-e2e-persist", Events: []testagent.AgentEvent{
		testagent.UnknownNotification(unknownE2ENotifyMethod, `{"ping":true}`),
		testagent.Completed("完成"),
	}}
	srv, th, _ := newUnknownE2EEnv(t, sc)
	_, waitDone := startUnknownE2ETurn(t, srv, th)
	waitDone()

	ctx := context.Background()
	before, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil || len(before.GetItems()) != 1 {
		t.Fatalf("台账应有 1 条: err=%v n=%d", err, len(before.GetItems()))
	}
	eventID := before.GetItems()[0].GetEventId()
	if _, err := srv.AcknowledgeUnknownEvent(ctx, &pb.AcknowledgeUnknownEventRequest{
		EventId: eventID, Action: runtime.StatusInspected, Note: "已人工查看",
	}); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}

	// 重建:同 dataDir,store/runtime 后端全部重开 —— 模拟 daemon 进程重启。
	dir := srv.cfg.DataDir
	_ = srv.events.Close()
	rebuilt := newUnknownE2EServerAt(t, dir)
	after, err := rebuilt.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("重建后 ListUnknownEvents: %v", err)
	}
	if len(after.GetItems()) != 1 {
		t.Fatalf("重建后台账应原样读回,得到 %d 条", len(after.GetItems()))
	}
	item := after.GetItems()[0]
	if item.GetEventId() != eventID || item.GetProcessingStatus() != runtime.StatusInspected ||
		item.GetRevision() != 1 {
		t.Fatalf("处置状态未持久化: %+v", item)
	}
	if item.GetPlainTitle() == "" {
		t.Fatalf("重建后摘要人话应照常生成: %+v", item)
	}
	// 处置备注在详情侧(Note 不在列表摘要字段里)。
	detail, err := rebuilt.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: eventID})
	if err != nil {
		t.Fatalf("重建后 GetUnknownEvent: %v", err)
	}
	if detail.GetNote() != "已人工查看" {
		t.Fatalf("处置备注未持久化: %q", detail.GetNote())
	}
}

// ---- 验证 ⑥:旧形态台账条目读出时摘要照常生成 ---------------------------------
//
// 旧 daemon 写下的条目没有本轮之后新增的任何字段(无 note/revision/decoded_fields/
// correlation_id/thread_id),新 daemon 读出时 PresentUnknownEvent/TranslateEvent
// 两个读时纯函数照常生成完整呈现 —— 人话不依赖新字段,老数据零迁移。
// (App 侧"旧 daemon 缺字段时新 App 人话降级"由既有 JVM 单测
// android .../ui/UnknownEventCardModelTest.kt 的「旧 daemon 降级」用例覆盖。)
func TestUnknownE2EOldLedgerEntryStillGetsPresentation(t *testing.T) {
	dir := t.TempDir()
	old := `{"seq":1,"dropped":0,"entries":[{` +
		`"id":"ue-1-legacy","sequence":1,"timestamp_ms":1728000000000,` +
		`"source_agent":"codex","source_method":"brandOld/notify",` +
		`"normalized_kind":"UNKNOWN_NOTIFICATION","safety_class":"UNKNOWN",` +
		`"raw_payload":"{\"k\":\"v\"}","processing_status":"NEW"}]}`
	if err := os.WriteFile(filepath.Join(dir, unknownE2EFile), []byte(old), 0o600); err != nil {
		t.Fatalf("写旧形态台账: %v", err)
	}
	srv := newUnknownE2EServerAt(t, dir)

	ctx := context.Background()
	resp, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil {
		t.Fatalf("ListUnknownEvents: %v", err)
	}
	if len(resp.GetItems()) != 1 {
		t.Fatalf("旧台账应读回 1 条,得到 %d", len(resp.GetItems()))
	}
	item := resp.GetItems()[0]
	if item.GetEventId() != "ue-1-legacy" || item.GetSourceMethod() != "brandOld/notify" ||
		item.GetProcessingStatus() != runtime.StatusNew || item.GetRevision() != 0 {
		t.Fatalf("旧条目字段应原样保留: %+v", item)
	}
	// 人话摘要照常生成:标题/解释/影响/动作一个不少(不依赖任何新字段)。
	if item.GetPlainTitle() != "电脑端传来了新消息" || item.GetPlainExplanation() == "" ||
		item.GetImpact() != "不影响正在进行的任务" {
		t.Fatalf("旧条目应吃到完整呈现: %+v", item)
	}
	unknownE2EAssertNoAllowAction(t, "旧条目摘要", item.GetActions())
	detail, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: "ue-1-legacy"})
	if err != nil {
		t.Fatalf("GetUnknownEvent: %v", err)
	}
	if detail.GetRecommendedNextStep() == "" || detail.GetSupportBundleHint() == "" {
		t.Fatalf("旧条目详情应带下一步与支持包提示: %+v", detail)
	}
}

// ---- 验证 ⑦:详情 RPC 失败不把堆栈甩给用户 ------------------------------------
//
// 读现状(如实钉住):台账损坏是 fail-soft —— 告警 + 空台账继续、原文件保留
// (runtime.NewUnknownEventLog);详情按"找不到"回结构化 NotFound,人话、无堆栈;
// 单条坏载荷条目照常出详情(透析留空、翻译兜底、处置句照给)。
func TestUnknownE2EDetailFailureStaysHumanReadable(t *testing.T) {
	// 情形 A:整本台账损坏(截断 JSON)→ daemon 照常起来,详情回人话 NotFound。
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, unknownE2EFile),
		[]byte(`{"seq":2,"dropped":0,"entries":[{"id":"ue-1-broken"`), 0o600); err != nil {
		t.Fatalf("写损坏台账: %v", err)
	}
	srv := newUnknownE2EServerAt(t, dir)

	ctx := context.Background()
	_, err := srv.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: "ue-1-broken"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("损坏台账的详情应为 NotFound,得到 %v", err)
	}
	if msg := err.Error(); strings.Contains(msg, "goroutine") || strings.Contains(msg, "panic") || strings.Contains(msg, ".go:") {
		t.Fatalf("错误信息不得带堆栈形态: %q", msg)
	}
	list, err := srv.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil || len(list.GetItems()) != 0 {
		t.Fatalf("损坏台账应 fail-soft 为空清单: err=%v n=%d", err, len(list.GetItems()))
	}

	// 情形 B:单条载荷损坏(非 JSON)→ 详情照常返回,透析留空、翻译兜底。
	srv2, th, _ := newUnknownE2EEnv(t, testagent.Scenario{Name: "unknown-e2e-badpayload", Events: []testagent.AgentEvent{
		testagent.UnknownNotification("brandNew/broken", `not-json{{{`),
		testagent.Completed("完成"),
	}})
	_, waitDone := startUnknownE2ETurn(t, srv2, th)
	waitDone()
	resp, err := srv2.ListUnknownEvents(ctx, &pb.ListUnknownEventsRequest{})
	if err != nil || len(resp.GetItems()) != 1 {
		t.Fatalf("坏载荷条目应已落账: err=%v n=%d", err, len(resp.GetItems()))
	}
	detail, err := srv2.GetUnknownEvent(ctx, &pb.GetUnknownEventRequest{EventId: resp.GetItems()[0].GetEventId()})
	if err != nil {
		t.Fatalf("坏载荷条目的详情不应失败: %v", err)
	}
	if len(detail.GetDigest()) != 0 {
		t.Fatalf("非 JSON 载荷不得编造透析表: %+v", detail.GetDigest())
	}
	if detail.GetSummary().GetSummary() == "" || !strings.Contains(detail.GetTranslation(), "原文为准") {
		t.Fatalf("坏载荷条目应有人话兜底: summary=%q translation=%q",
			detail.GetSummary().GetSummary(), detail.GetTranslation())
	}
	raw, err := json.Marshal(detail)
	if err == nil {
		if s := string(raw); strings.Contains(s, "goroutine") || strings.Contains(s, "panic") {
			t.Fatalf("详情响应不得带堆栈形态: %q", s)
		}
	}
}
