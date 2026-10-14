// hermes_provider_test.go —— HermesProvider 会话续接/审批链行为契约(假客户端
// 注入,不起真进程;形状对 acp_adapter 实测):
//   - 会话优先级:同进程内存缓存 → ExternalSession 重挂载(session/load,历史
//     回放不重投影)→ 新建(写回 ExternalSession);
//   - 重挂载纪律:能力门控(loadSession 不在场显式报错)+ 查无此会话硬拒绝
//     (不静默新开);
//   - 审批链:request_permission 反向请求 → 手机审批卡 → ApprovalHook 裁决 →
//     selected 应答;超时/无通道 → 拒绝侧/显式报错(绝不伪造同意);
//   - 流式投影:五件套 + sessionId 分流。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeHermesClient 是 hermesClient 的脚本化假实现。
type fakeHermesClient struct {
	mu      sync.Mutex
	caps    *hermesapp.AgentCapabilities
	alive   bool
	closed  bool
	sev     string // initialize 应答的 agentInfo.version
	loadErr error  // LoadSession 的失败注入(默认 ErrSessionNotFound 由 loadMissing 决定)

	newCalls, loadCalls, listCalls, modelCalls, promptCalls int
	modelArgs                                               []hermesModelCall // session/set_model 实参(守卫钉子)
	loadArgs                                                [][2]string
	loadSeenNotifHandlers                                   int // load 期间装的弃帧收集器应有
	promptSids, promptTexts                                 []string
	cancelCalls                                             []string // session/cancel 的记账(H1 看门狗钉子;方法在 hermes_cancel_test.go)

	notifFn func(json.RawMessage)
	reqFn   func(int64, string, json.RawMessage)

	responds   []fakeRespond
	respondErr []fakeRespondErr

	// promptUsage 是 Prompt 注入的轮次用量(默认 nil;用量映射钉子用,见
	// hermes_field_coverage_20261007_test.go)。
	promptUsage *hermesapp.TurnUsage
	// promptErr 注入 Prompt 的传输失败(默认 nil;非 nil 时如实回 (nil, err))。
	promptErr error
	// promptOutcome 非 nil 时原样作为终局应答返回(可给零值 = 回执空、或
	// refusal 等失败 stopReason;默认 nil = 走既有的 end_turn 脚本)。
	promptOutcome *hermesapp.PromptOutcome
	// promptQuiet 抑制回放的通知帧(默认 false = 照旧回放;压缩回执钉子等
	// 只关心应答、不想被正文帧打扰的用例置 true)。
	promptQuiet bool
	// callTurns 记 CallTurnCtx 的 (方法, 实参) —— 多块 prompt 形状钉子直读。
	callTurns []fakeCallTurn
	// callTurnResp / callTurnErr 注入 CallTurnCtx 的应答/传输失败
	// (默认 nil = 复用 Prompt 脚本并把终局应答编回 JSON)。
	callTurnResp json.RawMessage
	callTurnErr  error
}

// fakeCallTurn 记一次 CallTurnCtx 调用(方法 + 原样实参)。
type fakeCallTurn struct {
	method string
	params any
}

type fakeRespond struct {
	ID     int64
	Result map[string]any
}

type fakeRespondErr struct {
	ID   int64
	Code int
	Msg  string
}

func newFakeHermesClient(loadSession bool) *fakeHermesClient {
	caps := &hermesapp.AgentCapabilities{LoadSession: loadSession}
	if loadSession {
		caps.SessionCapabilities = &hermesapp.SessionCapabilitiesSnapshot{
			Fork: &struct{}{}, List: &struct{}{}, Resume: &struct{}{},
		}
	}
	return &fakeHermesClient{caps: caps, alive: true, sev: "9.9.9-fake"}
}

func (f *fakeHermesClient) Alive() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.alive }
func (f *fakeHermesClient) Close()      { f.mu.Lock(); f.closed = true; f.mu.Unlock() }

func (f *fakeHermesClient) Initialize(context.Context) (*hermesapp.InitializeResult, error) {
	return &hermesapp.InitializeResult{AgentCapabilities: f.caps, AgentInfo: struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}{Name: "hermes-agent", Version: f.sev}}, nil
}

func (f *fakeHermesClient) SessionNew(context.Context, string) (*hermesapp.SessionNewResult, error) {
	f.mu.Lock()
	f.newCalls++
	f.mu.Unlock()
	return &hermesapp.SessionNewResult{SessionID: "fresh-sess"}, nil
}

func (f *fakeHermesClient) LoadSession(_ context.Context, cwd, sessionID string) (*hermesapp.SessionLoadResult, error) {
	f.mu.Lock()
	f.loadCalls++
	f.loadArgs = append(f.loadArgs, [2]string{cwd, sessionID})
	f.loadSeenNotifHandlers++
	f.mu.Unlock()
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	// 模拟 server.py:历史回放(update 通知)在应答前流出。
	if f.notifFn != nil {
		f.notifFn(json.RawMessage(`{"sessionId":"old-sess","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"回放正文"}}}`))
	}
	return &hermesapp.SessionLoadResult{}, nil
}

func (f *fakeHermesClient) ListSessions(context.Context, string) (*hermesapp.SessionListResult, error) {
	f.mu.Lock()
	f.listCalls++
	f.mu.Unlock()
	return &hermesapp.SessionListResult{Sessions: []hermesapp.SessionListEntry{{
		SessionID: "old-sess", Cwd: `C:\ws`, Title: "旧会话",
	}}}, nil
}

// hermesModelCall 记一次 session/set_model 的实参(目标会话 + 换算后的模型 ID)。
type hermesModelCall struct {
	sessionID string
	modelID   string
}

func (f *fakeHermesClient) SetSessionModel(_ context.Context, sessionID, modelID string) error {
	f.mu.Lock()
	f.modelCalls++
	f.modelArgs = append(f.modelArgs, hermesModelCall{sessionID: sessionID, modelID: modelID})
	f.mu.Unlock()
	return nil
}

// Prompt 脚本:回放工具帧;文本带 need-approval 时同步发审批反向请求。
// 终局应答形状对 server.py _finish_turn(可经 promptUsage 注入 usage;
// promptErr / promptOutcome / promptQuiet 见字段注释的失败·回执·静默注入)。
func (f *fakeHermesClient) Prompt(_ context.Context, sessionID, text string) (*hermesapp.PromptOutcome, error) {
	f.mu.Lock()
	f.promptCalls++
	f.promptSids = append(f.promptSids, sessionID)
	f.promptTexts = append(f.promptTexts, text)
	notif, req := f.notifFn, f.reqFn
	usage := f.promptUsage
	injErr, injOutcome, quiet := f.promptErr, f.promptOutcome, f.promptQuiet
	f.mu.Unlock()
	if injErr != nil {
		return nil, injErr
	}
	if !quiet && notif != nil {
		notif(json.RawMessage(`{"sessionId":"` + sessionID + `","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"正文片段"}}}`))
		if strings.Contains(text, "tool-round") {
			notif(json.RawMessage(`{"sessionId":"` + sessionID + `","update":{"sessionUpdate":"tool_call","toolCallId":"tc-1","title":"write_file","kind":"edit","status":"pending","content":[{"type":"diff","path":"a.txt","newText":"hi"}]}}`))
			notif(json.RawMessage(`{"sessionId":"` + sessionID + `","update":{"sessionUpdate":"tool_call_update","toolCallId":"tc-1","status":"completed"}}`))
		}
		// 别的会话的帧:必须被 sessionId 分流,不进本轮投影。
		notif(json.RawMessage(`{"sessionId":"other-sess","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"别人的帧"}}}`))
	}
	if req != nil && strings.Contains(text, "need-approval") {
		req(9001, "session/request_permission", json.RawMessage(fakePermParams))
	}
	if injOutcome != nil {
		return injOutcome, nil
	}
	return &hermesapp.PromptOutcome{StopReason: "end_turn", Usage: usage}, nil
}

// CallTurnCtx 多块 session/prompt 通道的假实现:记账实参(形状钉子直读后复用
// Prompt 的同一套脚本 —— 回放帧/审批/回执注入都在那边),应答编回 JSON(与真实
// *hermesapp.Client.CallTurnCtx 的 json.RawMessage 出参同形)。
func (f *fakeHermesClient) CallTurnCtx(ctx context.Context, method string, params any) (json.RawMessage, error) {
	sid, text := fakePromptArgs(params)
	f.mu.Lock()
	f.callTurns = append(f.callTurns, fakeCallTurn{method: method, params: params})
	resp, injErr := f.callTurnResp, f.callTurnErr
	f.mu.Unlock()
	if injErr != nil {
		return nil, injErr
	}
	if resp != nil {
		return resp, nil
	}
	if method != "session/prompt" {
		return nil, fmt.Errorf("fake 未实现的方法 %s", method)
	}
	out, err := f.Prompt(ctx, sid, text)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// fakePromptArgs 从 session/prompt 实参里取会话 id 与文本块拼接(多块序列里
// 只认 text 块;图片块的形状断言走 callTurns 直读,不在这里编码进文本)。
func fakePromptArgs(params any) (string, string) {
	m, _ := params.(map[string]any)
	if m == nil {
		return "", ""
	}
	sid, _ := m["sessionId"].(string)
	blocks, _ := m["prompt"].([]map[string]any)
	var b strings.Builder
	for i := range blocks {
		if blocks[i]["type"] != "text" {
			continue
		}
		t, _ := blocks[i]["text"].(string)
		b.WriteString(t)
	}
	return sid, b.String()
}

const fakePermParams = `{"sessionId":"cur","toolCall":{"toolCallId":"perm-check-1","title":"运行命令: mkdir x","kind":"execute","status":"pending","rawInput":{"command":"mkdir x"}},"options":[{"optionId":"allow_once","kind":"allow_once","name":"Allow once"},{"optionId":"deny","kind":"reject_once","name":"Deny"}]}`

func (f *fakeHermesClient) SetTurnHandlers(notif func(json.RawMessage), req func(int64, string, json.RawMessage)) {
	f.mu.Lock()
	f.notifFn, f.reqFn = notif, req
	f.mu.Unlock()
}

func (f *fakeHermesClient) Respond(id int64, result any) error {
	b, _ := json.Marshal(result)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	f.mu.Lock()
	f.responds = append(f.responds, fakeRespond{ID: id, Result: m})
	f.mu.Unlock()
	return nil
}

func (f *fakeHermesClient) RespondError(id int64, code int, message string) error {
	f.mu.Lock()
	f.respondErr = append(f.respondErr, fakeRespondErr{ID: id, Code: code, Msg: message})
	f.mu.Unlock()
	return nil
}

// newHermesTestProvider 组装带假客户端的 provider(fake 经 newClient 注入)。
func newHermesTestProvider(t *testing.T, fake *fakeHermesClient) *HermesProvider {
	t.Helper()
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return fake, nil }
	return p
}

// runHermes 执行一轮并收集 sink 事件。
func runHermes(t *testing.T, p *HermesProvider, th *store.Thread, approve ApprovalHook) ([]*pb.TurnServerMessage, []pb.ContentBlock, error) {
	t.Helper()
	var mu sync.Mutex
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}
	blocks, _, err := p.Run(context.Background(), TurnRequest{
		Thread: th, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好"}}}},
		Workspace: `C:\ws`,
	}, sink, approve)
	return events, blocks, err
}

// runHermesText 带自定义输入文本的一轮(审批触发词等)。
func runHermesText(t *testing.T, p *HermesProvider, th *store.Thread, approve ApprovalHook, text string) ([]*pb.TurnServerMessage, []pb.ContentBlock, error) {
	t.Helper()
	var mu sync.Mutex
	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}
	blocks, _, err := p.Run(context.Background(), TurnRequest{
		Thread: th, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}},
		Workspace: `C:\ws`,
	}, sink, approve)
	return events, blocks, err
}

func sinkKinds(events []*pb.TurnServerMessage) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		switch ev.Msg.(type) {
		case *pb.TurnServerMessage_TextDelta:
			out = append(out, "text")
		case *pb.TurnServerMessage_ThinkingDelta:
			out = append(out, "thinking")
		case *pb.TurnServerMessage_ToolCall:
			out = append(out, "tool")
		case *pb.TurnServerMessage_ApprovalRequest:
			out = append(out, "approval")
		case *pb.TurnServerMessage_CodexEvent:
			out = append(out, "plan")
		}
	}
	return out
}

// TestHermesRunSessionContinuity 会话跨轮续接:首轮新建(写回 ExternalSession),
// 次轮同进程缓存直用,不再 session/new。
func TestHermesRunSessionContinuity(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1"}

	if _, _, err := runHermes(t, p, th, nil); err != nil {
		t.Fatalf("首轮应成功: %v", err)
	}
	if th.ExternalSession.SessionID != "fresh-sess" || th.ExternalSession.Cwd != `C:\ws` {
		t.Fatalf("ExternalSession 写回不符: %+v", th.ExternalSession)
	}
	if _, _, err := runHermes(t, p, th, nil); err != nil {
		t.Fatalf("次轮应成功: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.newCalls != 1 {
		t.Fatalf("两轮只应 session/new 一次, got %d", fake.newCalls)
	}
	if len(fake.promptSids) != 2 || fake.promptSids[0] != "fresh-sess" || fake.promptSids[1] != "fresh-sess" {
		t.Fatalf("两轮应跑在同一会话上: %v", fake.promptSids)
	}
}

// TestHermesRunRemountsViaLoad 有落库引用、无内存缓存(daemon/适配器重启)→
// session/load 重挂载;历史回放帧不得重投影到手机 sink。
func TestHermesRunRemountsViaLoad(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1", ExternalSession: store.ExternalSessionRef{SessionID: "old-sess", Cwd: `C:\ws`}}

	events, _, err := runHermes(t, p, th, nil)
	if err != nil {
		t.Fatalf("重挂载轮应成功: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.newCalls != 0 {
		t.Fatalf("重挂载路径不得 session/new, got %d", fake.newCalls)
	}
	if fake.loadCalls != 1 || fake.loadArgs[0] != [2]string{`C:\ws`, "old-sess"} {
		t.Fatalf("load 参数不符: %+v", fake.loadArgs)
	}
	if fake.promptSids[0] != "old-sess" {
		t.Fatalf("重挂载后应跑在原会话上: %v", fake.promptSids)
	}
	for _, ev := range events {
		if td := ev.GetTextDelta(); td != nil && strings.Contains(td.Delta, "回放正文") {
			t.Fatal("历史回放不得重投影到手机(会重复出气泡)")
		}
	}
	if got := sinkKinds(events); len(got) == 0 || got[0] != "text" {
		t.Fatalf("本轮正文投影缺失: %v", got)
	}
}

// TestHermesRemountGhostSessionHardRejects 查无此会话(load null)→ 显式报错,
// 绝不静默新开。
func TestHermesRemountGhostSessionHardRejects(t *testing.T) {
	fake := newFakeHermesClient(true)
	fake.loadErr = hermesapp.ErrSessionNotFound
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1", ExternalSession: store.ExternalSessionRef{SessionID: "ghost"}}

	_, _, err := runHermes(t, p, th, nil)
	if err == nil || !strings.Contains(err.Error(), "不静默新开会话") {
		t.Fatalf("幽灵会话应显式拒绝, got %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.newCalls != 0 {
		t.Fatalf("拒绝路径不得偷偷新建: %d", fake.newCalls)
	}
	if th.ExternalSession.SessionID != "ghost" {
		t.Fatal("失败时不得改写落库引用")
	}
}

// TestHermesRemountCapabilityGates initialize 未广播 loadSession → 显式报错,
// 不发 session/load。
func TestHermesRemountCapabilityGates(t *testing.T) {
	fake := newFakeHermesClient(false) // loadSession=false
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1", ExternalSession: store.ExternalSessionRef{SessionID: "old-sess"}}

	_, _, err := runHermes(t, p, th, nil)
	if err == nil || !strings.Contains(err.Error(), "loadSession 不在场") {
		t.Fatalf("能力缺席应显式报错, got %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.loadCalls != 0 {
		t.Fatalf("能力缺席不得发 session/load: %d", fake.loadCalls)
	}
}

// TestHermesApprovalAllowAllows 审批链 allow 路:反向请求 → 审批卡 → 同意 →
// selected(allow_once)应答。
func TestHermesApprovalAllowAllows(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1"}

	var gotCall *pb.ToolCallBlock
	approve := func(_ context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		gotCall = call
		return true, "", nil
	}
	events, _, err := runHermesText(t, p, th, approve, "need-approval 建个目录")
	if err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	if gotCall == nil || gotCall.Name != "运行命令: mkdir x" {
		t.Fatalf("审批卡应按请求原文构造: %+v", gotCall)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.responds) != 1 || fake.responds[0].ID != 9001 {
		t.Fatalf("应答不符: %+v", fake.responds)
	}
	out, _ := json.Marshal(fake.responds[0].Result)
	if !strings.Contains(string(out), `"optionId":"allow_once"`) || !strings.Contains(string(out), `"outcome":"selected"`) {
		t.Fatalf("应答应是 selected allow_once: %s", out)
	}
	if !containsKind(events, "approval") {
		t.Fatal("sink 应收到审批卡(ApprovalRequest)")
	}
}

// TestHermesApprovalDenyDenies 拒绝路:deny 侧应答。
func TestHermesApprovalDenyDenies(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1"}

	approve := func(context.Context, *pb.ToolCallBlock) (bool, string, error) { return false, "", nil }
	if _, _, err := runHermesText(t, p, th, approve, "need-approval 建个目录"); err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	out, _ := json.Marshal(fake.responds[0].Result)
	if !strings.Contains(string(out), `"optionId":"deny"`) {
		t.Fatalf("拒绝应答应是 selected deny: %s", out)
	}
}

// TestHermesApprovalTimeoutDenies 审批超时 → 拒绝侧(失败安全,绝不伪造同意)。
func TestHermesApprovalTimeoutDenies(t *testing.T) {
	old := hermesApprovalTimeout
	hermesApprovalTimeout = 30 * time.Millisecond
	t.Cleanup(func() { hermesApprovalTimeout = old })

	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1"}

	approve := func(ctx context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
		<-ctx.Done() // 手机端一直不点
		return false, "", ctx.Err()
	}
	if _, _, err := runHermesText(t, p, th, approve, "need-approval 建个目录"); err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	out, _ := json.Marshal(fake.responds[0].Result)
	if !strings.Contains(string(out), `"optionId":"deny"`) {
		t.Fatalf("超时应答拒绝侧: %s", out)
	}
}

// TestHermesApprovalNoChannelErrors approve==nil(审批通道缺失)→ -32000,
// 不伪装成用户意志。
func TestHermesApprovalNoChannelErrors(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1"}

	if _, _, err := runHermesText(t, p, th, nil, "need-approval 建个目录"); err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.respondErr) != 1 || fake.respondErr[0].Code != -32000 {
		t.Fatalf("无审批通道应回 -32000: %+v", fake.respondErr)
	}
}

// TestHermesUnknownReverseRequestRejected 不认识的反向请求 → -32601(不伪造)。
func TestHermesUnknownReverseRequestRejected(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)

	p.handleRequestPermission(fake, 7, "fs/read_text_file", json.RawMessage(`{}`), context.Background(),
		func(*pb.TurnServerMessage) {}, nil, "th-hermes-approval")
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.respondErr) != 1 || fake.respondErr[0].Code != -32601 {
		t.Fatalf("未知反向请求应回 -32601: %+v", fake.respondErr)
	}
}

// TestHermesStreamingProjection 投影对表:正文/工具(start→finished,diff 并入
// args)/别人的帧被 sessionId 分流。
func TestHermesStreamingProjection(t *testing.T) {
	fake := newFakeHermesClient(true)
	p := newHermesTestProvider(t, fake)
	th := &store.Thread{ID: "t1"}

	var events []*pb.TurnServerMessage
	sink := func(ev *pb.TurnServerMessage) { events = append(events, ev) }
	blocks, _, err := p.Run(context.Background(), TurnRequest{
		Thread: th, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "tool-round 建个文件"}}}},
		Workspace: `C:\ws`,
	}, sink, nil)
	if err != nil {
		t.Fatalf("轮次应成功: %v", err)
	}
	kinds := sinkKinds(events)
	if len(kinds) != 3 || kinds[0] != "text" || kinds[1] != "tool" || kinds[2] != "tool" {
		t.Fatalf("投影序列不符(别人的帧必须被分流): %v", kinds)
	}
	tc := events[1].GetToolCall()
	if tc == nil || tc.Phase != pb.ToolCallUpdate_STARTED || tc.Call.Name != "write_file" {
		t.Fatalf("工具起始帧不符: %+v", tc)
	}
	if !strings.Contains(tc.Call.JsonArgs, `"path":"a.txt"`) {
		t.Fatalf("diff 应并入 JsonArgs: %s", tc.Call.JsonArgs)
	}
	tcEnd := events[2].GetToolCall()
	if tcEnd == nil || tcEnd.Phase != pb.ToolCallUpdate_FINISHED {
		t.Fatalf("工具收口帧不符: %+v", tcEnd)
	}
	if tc.Call.Id != tcEnd.Call.Id {
		t.Fatalf("同一 toolCallId 的 start/update 应共帧 Id: %s vs %s", tc.Call.Id, tcEnd.Call.Id)
	}
	final := blocks[0].GetText().GetText()
	if final != "正文片段" {
		t.Fatalf("终局正文应只含本轮会话正文: %q", final)
	}
}

// TestHermesEmptyInputRejected 空输入拒绝(既有行为钉死)。
func TestHermesEmptyInputRejected(t *testing.T) {
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return nil, context.Canceled }
	_, _, err := p.Run(context.Background(), TurnRequest{
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "  "}}}},
	}, func(*pb.TurnServerMessage) {}, nil)
	if err == nil || !strings.Contains(err.Error(), "没有文本输入") {
		t.Fatalf("空输入应拒绝: %v", err)
	}
}

func containsKind(events []*pb.TurnServerMessage, kind string) bool {
	for _, k := range sinkKinds(events) {
		if k == kind {
			return true
		}
	}
	return false
}
