// dsh_test.go —— DshProvider 行为契约(接口 fake,不依赖真子进程;dshapp 的
// 假子进程端到端见 internal/dshapp/client_test.go):
//   - Models():探针夹具目录 → 模型列表(TTL 缓存/失效/无静态名单);
//   - Run():新建会话 happy path / ExternalSession 重挂载(list 校验+resume+
//     目录刷新;不在列显式报错不静默新开);
//   - 探针修正①:model 切换只发原样编码串,禁裸模型名;
//   - 探针修正③:缺 key 错误 → MISSING_CREDENTIAL(ErrorInfoCarrier 保真);
//   - 审批桥:allow/reject/超时三路,绝不伪造 decision(红线 1/2);
//   - 未知 update 变体原样透传进诊断(探针修正⑤);HideReasoning 过滤。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/dshapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- fake dshClient ----

// probeCaps 探针第 1 项握手快照(image=false,list/resume/close 在场)。
func probeCaps() *dshapp.InitializeResult { return dshapp.ProbeInitializeResult() }

// probeCatalogOpts 探针第 2 项 configOptions 夹具。
func probeCatalogOpts(t *testing.T) []dshapp.ConfigOption {
	t.Helper()
	opts, err := dshapp.ParseConfigOptions(dshapp.ProbeConfigOptionsJSON)
	if err != nil {
		t.Fatalf("探针夹具应可解析: %v", err)
	}
	return opts
}

type setConfigCall struct {
	sessionID string
	optionID  string
	value     string
}

type respondCall struct {
	id      int64
	result  interface{}
	errCode int
	errMsg  string
	isError bool
}

// fakeDshClient 可编程假客户端:各方法可注入返回值/错误,并记录调用。
type fakeDshClient struct {
	mu sync.Mutex

	caps      *dshapp.InitializeResult
	newState  *dshapp.SessionState
	newErr    error
	list      []dshapp.SessionListEntry
	listErr   error
	resumeBy  map[string]*dshapp.SessionState
	resumeErr error
	// promptFn 收到 notify(fake 内已上锁快照的当轮通知钩子),闭包内用它发
	// session/update 帧,避免无锁读 notifFn(-race)。
	promptFn func(ctx context.Context, sid string, blocks []map[string]interface{}, notify func(string, json.RawMessage)) (json.RawMessage, error)
	closeErr error
	initErr  error

	newCalls   []string // cwd 记录
	resumeHits []string
	resumeCwds []string // 与 resumeHits 一一对应:rc.3 契约要求 resume 带 cwd 的回归钉
	closeHits  []string
	setCalls   []setConfigCall
	responds   []respondCall
	cancelHits []string
	closed     bool

	notifFn func(string, json.RawMessage)
	reqFn   func(int64, string, json.RawMessage)

	// initCalls 记录 Initialize 调用次数(T-DSH 预热/懒启动握手次数断言)。
	initCalls int
}

func (f *fakeDshClient) Initialize(context.Context) (*dshapp.InitializeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.initCalls++
	return f.caps, f.initErr
}
func (f *fakeDshClient) SessionNew(_ context.Context, cwd string) (*dshapp.SessionState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newCalls = append(f.newCalls, cwd)
	return f.newState, f.newErr
}
func (f *fakeDshClient) SessionList(context.Context) ([]dshapp.SessionListEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.list, f.listErr
}
func (f *fakeDshClient) SessionResume(_ context.Context, sid, cwd string) (*dshapp.SessionState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resumeHits = append(f.resumeHits, sid)
	f.resumeCwds = append(f.resumeCwds, cwd)
	if f.resumeErr != nil {
		return nil, f.resumeErr
	}
	st, ok := f.resumeBy[sid]
	if !ok {
		return nil, fmt.Errorf("fake: 未知会话 %s", sid)
	}
	return st, nil
}
func (f *fakeDshClient) SessionClose(_ context.Context, sid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeHits = append(f.closeHits, sid)
	return f.closeErr
}
func (f *fakeDshClient) SetConfigOption(_ context.Context, sid, optionID, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setCalls = append(f.setCalls, setConfigCall{sid, optionID, value})
	return nil
}
func (f *fakeDshClient) Prompt(ctx context.Context, sid string, blocks []map[string]interface{}) (json.RawMessage, error) {
	f.mu.Lock()
	fn, notify := f.promptFn, f.notifFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, sid, blocks, notify)
	}
	return json.RawMessage(`{"stopReason":"end_turn"}`), nil
}
func (f *fakeDshClient) Cancel(sid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelHits = append(f.cancelHits, sid)
}
func (f *fakeDshClient) SetTurnHandlers(notif func(string, json.RawMessage), req func(int64, string, json.RawMessage)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notifFn, f.reqFn = notif, req
}
func (f *fakeDshClient) Respond(id int64, result interface{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responds = append(f.responds, respondCall{id: id, result: result})
	return nil
}
func (f *fakeDshClient) RespondError(id int64, code int, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responds = append(f.responds, respondCall{id: id, errCode: code, errMsg: message, isError: true})
	return nil
}
func (f *fakeDshClient) SyncHandlers() {}
func (f *fakeDshClient) Alive() bool   { return true }
func (f *fakeDshClient) PID() int      { return 0 }
func (f *fakeDshClient) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

// fakeSnapshot 是 fake 调用记录的快照(显式拷贝切片,不拷含锁结构体)。
type fakeSnapshot struct {
	newCalls   []string
	resumeHits []string
	resumeCwds []string
	closeHits  []string
	cancelHits []string
	setCalls   []setConfigCall
	responds   []respondCall
}

func (f *fakeDshClient) snapshot() fakeSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fakeSnapshot{
		newCalls:   append([]string(nil), f.newCalls...),
		resumeHits: append([]string(nil), f.resumeHits...),
		resumeCwds: append([]string(nil), f.resumeCwds...),
		closeHits:  append([]string(nil), f.closeHits...),
		cancelHits: append([]string(nil), f.cancelHits...),
		setCalls:   append([]setConfigCall(nil), f.setCalls...),
		responds:   append([]respondCall(nil), f.responds...),
	}
}

// newTestDshProvider 造注入 fake 的 provider(Initialize 返回探针能力快照)。
func newTestDshProvider(t *testing.T, fake *fakeDshClient) *DshProvider {
	t.Helper()
	if fake.caps == nil {
		fake.caps = probeCaps()
	}
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) { return fake, nil }
	return p
}

// sinkCollect 收集 sink 事件。
type sinkCollect struct {
	mu  sync.Mutex
	evs []*pb.TurnServerMessage
}

func (s *sinkCollect) add(ev *pb.TurnServerMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evs = append(s.evs, ev)
}

func (s *sinkCollect) all() []*pb.TurnServerMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*pb.TurnServerMessage(nil), s.evs...)
}

// ---- Models() ----

// TestDshModelsFromProbeCatalog 探针夹具:default 哨兵 + 4 个 dsh/* 模型,
// 档位随模型透出;TTL 缓存命中不再建会话;InvalidateModels 后才重拉。
func TestDshModelsFromProbeCatalog(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-models", ConfigOptions: probeCatalogOpts(t)},
	}
	p := newTestDshProvider(t, fake)

	ms := p.Models()
	if len(ms) != 5 {
		t.Fatalf("应 1 default + 4 模型,实得 %d: %+v", len(ms), ms)
	}
	if ms[0].Id != "default" || ms[0].Provider != "dsh" {
		t.Fatalf("首条应为 default 哨兵: id=%q provider=%q", ms[0].Id, ms[0].Provider)
	}
	wantIDs := map[string]bool{
		"dsh/deepseek-flash":               false,
		"dsh/deepseek-v4-flash":            false,
		"dsh/deepseek-v4-pro":              false,
		"dsh/deepseek-v4-flash-vision-exp": false,
	}
	for i := 1; i < len(ms); i++ {
		m := &ms[i]
		if _, ok := wantIDs[m.Id]; !ok {
			t.Fatalf("意外模型 %q", m.Id)
		}
		wantIDs[m.Id] = true
		if !m.SupportsThinking || strings.Join(m.AvailableEfforts, ",") != "off,low,high,max" {
			t.Fatalf("模型 %s 档位不符: %v", m.Id, m.AvailableEfforts)
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("缺模型 %q", id)
		}
	}
	if n := len(fake.snapshot().newCalls); n != 1 {
		t.Fatalf("TTL 缓存应命中,第二次调用不得再建会话: new=%d", n)
	}

	p.InvalidateModels()
	p.Models()
	if n := len(fake.snapshot().newCalls); n != 2 {
		t.Fatalf("InvalidateModels 后应重拉: new=%d", n)
	}
}

// ---- Run():新建会话 happy path ----

func TestDshRunHappyPathNewSession(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-new-1", ConfigOptions: probeCatalogOpts(t)},
		promptFn: func(_ context.Context, sid string, _ []map[string]interface{}, notify func(string, json.RawMessage)) (json.RawMessage, error) {
			// 一条助手块 + 一条工具卡,再回终局
			env, _ := json.Marshal(map[string]interface{}{
				"sessionId": sid,
				"update": map[string]interface{}{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]interface{}{"type": "text", "text": "答案是 2"},
				},
			})
			notify("session/update", env)
			env, _ = json.Marshal(map[string]interface{}{
				"sessionId": sid,
				"update": map[string]interface{}{
					"sessionUpdate": "tool_call",
					"toolCallId":    "t1",
					"title":         "读取文件",
					"kind":          "read",
					"status":        "completed",
					"content":       []map[string]interface{}{{"type": "text", "text": "1+1=2"}},
				},
			})
			notify("session/update", env)
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p := newTestDshProvider(t, fake)
	sink := &sinkCollect{}
	th := &store.Thread{ID: "th-1", AgentID: "dsh"}

	blocks, _, err := p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "1+1"}}}}}, sink.add, nil)
	if err != nil {
		t.Fatalf("Run 应成功: %v", err)
	}
	if len(blocks) != 1 || blocks[0].GetText().GetText() != "答案是 2" {
		t.Fatalf("终稿不符: %+v", blocks)
	}
	if th.ExternalSession.SessionID != "s-new-1" || th.ExternalSession.Cwd != `G:\ws` {
		t.Fatalf("ExternalSession 应落库: %+v", th.ExternalSession)
	}
	var gotText, gotTool bool
	for _, ev := range sink.all() {
		if td := ev.GetTextDelta(); td != nil && td.Delta == "答案是 2" {
			gotText = true
		}
		if tc := ev.GetToolCall(); tc != nil {
			gotTool = true
			if tc.Phase != pb.ToolCallUpdate_FINISHED || !tc.Result.Ok || tc.Call.Name != "read" {
				t.Fatalf("工具卡状态机不符: %+v %+v", tc.Phase, tc.Result)
			}
		}
	}
	if !gotText || !gotTool {
		t.Fatalf("缺少预期事件: text=%t tool=%t", gotText, gotTool)
	}
}

// ---- Run():ExternalSession 重挂载(探针修正②④)----

// TestDshRemountNotListedFailsExplicitly 不在 session/list → 显式报错,绝不静默
// 新开(蓝本 §4.3;探针修正④校验按 sessionId+cwd)。
func TestDshRemountNotListedFailsExplicitly(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "should-not-be-created"},
		list:     []dshapp.SessionListEntry{{SessionID: "other", Cwd: `G:\ws`}},
	}
	p := newTestDshProvider(t, fake)
	th := &store.Thread{ID: "th-2", AgentID: "dsh", ExternalSession: store.ExternalSessionRef{SessionID: "s-gone", Cwd: `G:\ws`}}

	_, _, err := p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil)
	if err == nil || !strings.Contains(err.Error(), "不静默新开会话") {
		t.Fatalf("不在列应显式报错: %v", err)
	}
	if n := len(fake.snapshot().newCalls); n != 0 {
		t.Fatalf("不得静默新开会话: new=%d", n)
	}
}

// TestDshRemountValidatesCwdAndResumes 在列(sessionId+cwd 双因子)才 resume;
// resume 应答的 configOptions 刷新目录 —— 后续模型切换用 resume 后目录里的
// 编码串(探针修正②)。
func TestDshRemountValidatesCwdAndResumes(t *testing.T) {
	// resume 返回的目录:currentValue 换成 deepseek-flash(daemon 停机期间桌面端切过模型)
	resumedCatalog, err := dshapp.ParseConfigOptions(dshapp.ProbeConfigOptionsJSON)
	if err != nil {
		t.Fatalf("探针夹具应可解析: %v", err)
	}
	resumedCatalog[0].CurrentValue = `["deepseek-official","deepseek-flash"]`
	fake := &fakeDshClient{
		list: []dshapp.SessionListEntry{
			{SessionID: "s-old", Cwd: `g:\WS`}, // 大小写不同也应匹配(Windows 语义)
			{SessionID: "s-other", Cwd: `G:\other`},
		},
		resumeBy: map[string]*dshapp.SessionState{
			"s-old": {SessionID: "s-old", ConfigOptions: resumedCatalog},
		},
		promptFn: func(context.Context, string, []map[string]interface{}, func(string, json.RawMessage)) (json.RawMessage, error) {
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p := newTestDshProvider(t, fake)
	th := &store.Thread{ID: "th-3", AgentID: "dsh", ExternalSession: store.ExternalSessionRef{SessionID: "s-old", Cwd: `G:\ws`}}

	if _, _, err = p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil); err != nil {
		t.Fatalf("重挂载 Run 应成功: %v", err)
	}
	snap := fake.snapshot()
	if len(snap.resumeHits) != 1 || snap.resumeHits[0] != "s-old" {
		t.Fatalf("应恰好 resume 一次 s-old: %v", snap.resumeHits)
	}
	if len(snap.newCalls) != 0 {
		t.Fatalf("重挂载路径不得新开会话: %v", snap.newCalls)
	}

	// 目录已刷新:切 deepseek-flash 应用 resume 后目录的编码串。
	v, ok := p.encodedModelValue("deepseek-flash")
	if !ok || v != `["deepseek-official","deepseek-flash"]` {
		t.Fatalf("resume 后目录应刷新: %q %v", v, ok)
	}
}

// TestDshRemountListUnavailableGatesOnCapability 能力门控:resume 不在场时
// 显式报错,不猜(§5.2)。
func TestDshRemountListUnavailableGatesOnCapability(t *testing.T) {
	fake := &fakeDshClient{}
	fake.caps = probeCaps()
	fake.caps.AgentCapabilities.SessionCapabilities.Resume = nil // 摘掉 resume 能力
	p := newTestDshProvider(t, fake)
	th := &store.Thread{ID: "th-4", AgentID: "dsh", ExternalSession: store.ExternalSessionRef{SessionID: "s-x"}}

	_, _, err := p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil)
	if err == nil || !strings.Contains(err.Error(), "不支持会话恢复") {
		t.Fatalf("resume 能力缺席应显式报错: %v", err)
	}
}

// ---- 探针修正①:模型切换只发原样编码串 ----

func TestDshModelSwitchSendsEncodedValueNeverBareName(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-m", ConfigOptions: probeCatalogOpts(t)},
		promptFn: func(context.Context, string, []map[string]interface{}, func(string, json.RawMessage)) (json.RawMessage, error) {
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p := newTestDshProvider(t, fake)
	th := &store.Thread{ID: "th-5", AgentID: "dsh", ModelID: "dsh/deepseek-v4-pro", Effort: "max"}

	if _, _, err := p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil); err != nil {
		t.Fatalf("Run 应成功: %v", err)
	}
	snap := fake.snapshot()
	var sawModel, sawEffort bool
	for _, c := range snap.setCalls {
		if c.optionID == "model" {
			sawModel = true
			if c.value != `["deepseek-official","deepseek-v4-pro"]` {
				t.Fatalf("model value 必须是原样编码串(探针修正①): %q", c.value)
			}
		}
		if c.optionID == "reasoning_effort" {
			sawEffort = true
			if c.value != "max" {
				t.Fatalf("effort value 不符: %q", c.value)
			}
		}
	}
	if !sawModel || !sawEffort {
		t.Fatalf("缺配置切换: model=%t effort=%t calls=%+v", sawModel, sawEffort, snap.setCalls)
	}

	// 目录里没有的模型:跳过 + 日志,绝不发裸名。
	th2 := &store.Thread{ID: "th-6", AgentID: "dsh", ModelID: "dsh/not-in-catalog"}
	fake2 := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-m2", ConfigOptions: probeCatalogOpts(t)},
		promptFn: func(context.Context, string, []map[string]interface{}, func(string, json.RawMessage)) (json.RawMessage, error) {
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p2 := newTestDshProvider(t, fake2)
	if _, _, err := p2.Run(context.Background(), TurnRequest{Thread: th2, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil); err != nil {
		t.Fatalf("未知模型应跳过而非失败: %v", err)
	}
	for _, c := range fake2.snapshot().setCalls {
		if c.optionID == "model" {
			t.Fatalf("目录外模型不得发送任何 model 切换: %+v", c)
		}
	}
}

// ---- 探针修正③:缺 key → MISSING_CREDENTIAL ----

func TestDshMissingCredentialMapped(t *testing.T) {
	probeMsg := `Internal error: turn failed: llm-deepseek: no API key for provider route "deepseek-official"`
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-k"},
		promptFn: func(context.Context, string, []map[string]interface{}, func(string, json.RawMessage)) (json.RawMessage, error) {
			return nil, &dshapp.RPCError{Method: "session/prompt", Code: -32603, Message: probeMsg}
		},
	}
	p := newTestDshProvider(t, fake)
	th := &store.Thread{ID: "th-7", AgentID: "dsh"}

	_, _, err := p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil)
	var re *dshRunError
	if !errors.As(err, &re) {
		t.Fatalf("应返回 *dshRunError: %v (%T)", err, err)
	}
	if re.ErrorInfo() == nil || re.ErrorInfo().Code != "MISSING_CREDENTIAL" {
		t.Fatalf("缺 key 应归一 MISSING_CREDENTIAL: %+v", re.ErrorInfo())
	}
	if !strings.Contains(re.ErrorInfo().Message, "dsh-credentials.env") {
		t.Fatalf("缺 key 应给出人话指引(指向 dsh-credentials.env): %q", re.ErrorInfo().Message)
	}
	if !strings.Contains(re.ErrorInfo().Message, "DEEPSEEK_API_KEY") {
		t.Fatalf("人话指引应点名 DEEPSEEK_API_KEY: %q", re.ErrorInfo().Message)
	}
	if re.ErrorInfo().Details != probeMsg {
		t.Fatalf("dsh 原生 message 应保真进 Details(不丢弃): %q", re.ErrorInfo().Details)
	}
}

// TestDshRunErrorPassthrough 非缺 key 错误原样透传:不加工成 MISSING_CREDENTIAL,
// message 直接是 dsh 原生报错(铁律:不认识的输入原样透传,不替换成"未知/错误")。
func TestDshRunErrorPassthrough(t *testing.T) {
	probeMsg := `Internal error: turn failed: boom unrelated`
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-p"},
		promptFn: func(context.Context, string, []map[string]interface{}, func(string, json.RawMessage)) (json.RawMessage, error) {
			return nil, &dshapp.RPCError{Method: "session/prompt", Code: -32603, Message: probeMsg}
		},
	}
	p := newTestDshProvider(t, fake)
	th := &store.Thread{ID: "th-p", AgentID: "dsh"}
	_, _, err := p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil)
	var re *dshRunError
	if !errors.As(err, &re) {
		t.Fatalf("应返回 *dshRunError: %v (%T)", err, err)
	}
	if re.ErrorInfo().Code == "MISSING_CREDENTIAL" {
		t.Fatalf("非缺 key 错误不应归一 MISSING_CREDENTIAL: %+v", re.ErrorInfo())
	}
	if re.ErrorInfo().Message != probeMsg {
		t.Fatalf("非缺 key 错误应原样透传 message: %q", re.ErrorInfo().Message)
	}
}

// ---- 审批桥:allow / reject / 超时三路 + 未知反向请求 ----

// permParams 探针口径的审批请求(options 数组/kind 字段/optionId)。
const permParams = `{"sessionId":"s-a","options":[
  {"optionId":"allow-once","name":"Allow","kind":"allow_once"},
  {"optionId":"reject-once","name":"Reject","kind":"reject_once"}]}`

// runWithPendingApproval 把 Run 挂在"等审批应答"的节奏上,返回 (fake, 结束通道,
// 释放函数)。fake.reqFn 是同步直调:返回即审批应答已记录。
func runWithPendingApproval(t *testing.T, approve ApprovalHook) (*fakeDshClient, chan error, func()) {
	t.Helper()
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-a"},
		promptFn: func(ctx context.Context, _ string, _ []map[string]interface{}, _ func(string, json.RawMessage)) (json.RawMessage, error) {
			<-release // 挂住 Run,等审批用例驱动
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p := newTestDshProvider(t, fake)
	done := make(chan error, 1)
	go func() {
		_, _, err := p.Run(context.Background(), TurnRequest{Thread: &store.Thread{ID: "th-a", AgentID: "dsh"}, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "run cmd"}}}}}, func(*pb.TurnServerMessage) {}, approve)
		done <- err
	}()
	// 等 handlers 就位
	deadline := time.Now().Add(5 * time.Second)
	for {
		fake.mu.Lock()
		ready := fake.reqFn != nil
		fake.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Run 未装上反向请求 handler")
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Cleanup(free)
	return fake, done, free
}

func TestDshApprovalAllowPath(t *testing.T) {
	fake, done, free := runWithPendingApproval(t, func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
		return true, "", nil
	})
	fake.reqFn(7, "session/request_permission", json.RawMessage(permParams))
	free()
	if err := <-done; err != nil {
		t.Fatalf("Run 应成功: %v", err)
	}
	snap := fake.snapshot()
	if len(snap.responds) != 1 {
		t.Fatalf("应恰好应答一次: %+v", snap.responds)
	}
	r := snap.responds[0]
	if r.isError {
		t.Fatalf("allow 路不得回 error: %+v", r)
	}
	b, _ := json.Marshal(r.result)
	want := `{"outcome":{"optionId":"allow-once","outcome":"selected"}}`
	if string(b) != want {
		t.Fatalf("allow 应答形状不符:\n got=%s\nwant=%s", b, want)
	}
}

func TestDshApprovalRejectPath(t *testing.T) {
	fake, done, free := runWithPendingApproval(t, func(context.Context, *pb.ToolCallBlock) (bool, string, error) {
		return false, "", nil
	})
	fake.reqFn(8, "session/request_permission", json.RawMessage(permParams))
	free()
	<-done
	snap := fake.snapshot()
	if len(snap.responds) != 1 || snap.responds[0].isError {
		t.Fatalf("reject 路是成功应答: %+v", snap.responds)
	}
	b, _ := json.Marshal(snap.responds[0].result)
	if !strings.Contains(string(b), `"optionId":"reject-once"`) {
		t.Fatalf("reject 应选中 reject 侧: %s", b)
	}
}

// TestDshApprovalTimeoutGoesReject 超时走拒绝路径,绝不伪造"同意"(红线 1;
// 防御性实现 —— 该链路缺 key 无法实测,形状以 ACP v1 规范为准)。
func TestDshApprovalTimeoutGoesReject(t *testing.T) {
	old := dshApprovalTimeout
	dshApprovalTimeout = 80 * time.Millisecond
	t.Cleanup(func() { dshApprovalTimeout = old })

	fake, done, free := runWithPendingApproval(t, func(ctx context.Context, _ *pb.ToolCallBlock) (bool, string, error) {
		<-ctx.Done() // 用户不点,直到超时
		return false, "", ctx.Err()
	})
	fake.reqFn(9, "session/request_permission", json.RawMessage(permParams)) // 同步:返回即已按超时应答
	free()
	<-done
	snap := fake.snapshot()
	if len(snap.responds) != 1 || snap.responds[0].isError {
		t.Fatalf("超时应按成功应答回拒绝 outcome: %+v", snap.responds)
	}
	b, _ := json.Marshal(snap.responds[0].result)
	if !strings.Contains(string(b), `"optionId":"reject-once"`) {
		t.Fatalf("超时必须选中 reject 侧(失败安全): %s", b)
	}
}

// TestDshUnknownReverseRequestNoFabricatedDecision 未登记的反向请求回 error,
// 绝不伪造 decision(红线 2,codex_requests.go 第一条规则同款)。
func TestDshUnknownReverseRequestNoFabricatedDecision(t *testing.T) {
	fake, done, free := runWithPendingApproval(t, nil)
	fake.reqFn(10, "someFuture/request", json.RawMessage(`{}`))
	free()
	<-done
	snap := fake.snapshot()
	if len(snap.responds) != 1 || !snap.responds[0].isError {
		t.Fatalf("未知反向请求应回 error: %+v", snap.responds)
	}
	if snap.responds[0].errCode != -32601 {
		t.Fatalf("应回 -32601: %+v", snap.responds[0])
	}
}

// ---- 事件映射:未知变体透传 / HideReasoning / 图片门控 ----

// TestDshUnknownUpdateVariantPassthrough 探针修正⑤:未知变体原样透传进诊断
// 视图(主线 A),不猜不出错。
func TestDshUnknownUpdateVariantPassthrough(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	st := &dshTurnState{}
	notices := &nativeNoticeRecorder{}
	raw := json.RawMessage(`{"sessionUpdate":"brandNewThing","payload":{"k":1}}`)
	p.handleSessionUpdate(raw, sink.add, st, notices)
	evs := sink.all()
	if len(evs) != 1 || evs[0].GetCodexEvent() == nil {
		t.Fatalf("未知变体应进诊断视图: %+v", evs)
	}
	ev := evs[0].GetCodexEvent()
	if ev.Kind != pb.CodexEvent_STATUS || !strings.Contains(ev.RawJson, "brandNewThing") {
		t.Fatalf("诊断应原样携带原文与变体名: %+v", ev)
	}
	// 同变体限频(≤2 次)防刷屏(nativeNoticeRecorder 语义)
	p.handleSessionUpdate(raw, sink.add, st, notices)
	p.handleSessionUpdate(raw, sink.add, st, notices)
	if n := len(sink.all()); n != 2 {
		t.Fatalf("同变体应限频 2 次: %d", n)
	}
}

// TestDshThoughtChunkFiltered HideReasoning 过滤 ThinkingDelta(PrefsAware 子集)。
func TestDshThoughtChunkFiltered(t *testing.T) {
	p := NewDshProvider()
	p.ApplyPrefs(DesktopPrefs{HideReasoning: true})
	sink := &sinkCollect{}
	raw := json.RawMessage(`{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"thinking..."}}`)
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})
	for _, ev := range sink.all() {
		if ev.GetThinkingDelta() != nil {
			t.Fatal("HideReasoning 开启时不得透出 ThinkingDelta")
		}
	}
	// 关闭时透出
	p.ApplyPrefs(DesktopPrefs{})
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})
	var got bool
	for _, ev := range sink.all() {
		if td := ev.GetThinkingDelta(); td != nil && td.Delta == "thinking..." {
			got = true
		}
	}
	if !got {
		t.Fatal("关闭过滤后应透出 ThinkingDelta")
	}
}

// TestDshImageInputRefusedWhenUnsupported 能力协商:image=false 时图片输入如实
// 报错,不静默丢(R4)。
func TestDshImageInputRefusedWhenUnsupported(t *testing.T) {
	fake := &fakeDshClient{newState: &dshapp.SessionState{SessionID: "s-img"}}
	p := newTestDshProvider(t, fake) // 探针能力快照:image=false
	th := &store.Thread{ID: "th-img", AgentID: "dsh"}
	_, _, err := p.Run(context.Background(), TurnRequest{
		Thread: th, Workspace: `G:\ws`,
		InputBlocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "看图"}}},
			{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{Data: []byte{1}, MediaType: "image/png"}}},
		},
	}, func(*pb.TurnServerMessage) {}, nil)
	if err == nil || !strings.Contains(err.Error(), "不支持图片输入") {
		t.Fatalf("图片输入应如实报错: %v", err)
	}
}

// ---- 纯函数 ----

func TestDshModelSpec(t *testing.T) {
	cases := map[string]string{
		"dsh/deepseek-v4-pro": "deepseek-v4-pro",
		"dsh/default":         "",
		"default":             "",
		"":                    "",
		"codex/gpt-6":         "",
	}
	for in, want := range cases {
		if got := dshModelSpec(in); got != want {
			t.Fatalf("dshModelSpec(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDshSessionListed(t *testing.T) {
	entries := []dshapp.SessionListEntry{{SessionID: "s1", Cwd: `G:\ws`}}
	if !dshSessionListed(entries, store.ExternalSessionRef{SessionID: "s1", Cwd: `g:\WS`}) {
		t.Fatal("同 id 不同大小写 cwd 应匹配(Windows 语义)")
	}
	if !dshSessionListed(entries, store.ExternalSessionRef{SessionID: "s1"}) {
		t.Fatal("本侧 cwd 缺失退化为单因子")
	}
	if dshSessionListed(entries, store.ExternalSessionRef{SessionID: "s1", Cwd: `G:\other`}) {
		t.Fatal("cwd 不同不得匹配(探针修正④双因子)")
	}
	if dshSessionListed(entries, store.ExternalSessionRef{SessionID: "s2", Cwd: `G:\ws`}) {
		t.Fatal("id 不同不得匹配")
	}
}

func TestDshStopReasonAndPhases(t *testing.T) {
	if got := dshStopReason(json.RawMessage(`{"stopReason":"end_turn"}`)); got != "end_turn" {
		t.Fatalf("stopReason 解析不符: %q", got)
	}
	if got := dshStopReason(json.RawMessage(`{}`)); got != "" {
		t.Fatalf("缺省应为空串: %q", got)
	}
	if ph, ok := dshToolPhase("finished"); !ok || ph != pb.ToolCallUpdate_FINISHED {
		t.Fatalf("finished 应为 FINISHED: %v %v", ph, ok)
	}
	if ph, ok := dshToolPhase("in_progress"); !ok || ph != pb.ToolCallUpdate_UPDATE {
		t.Fatalf("in_progress 应为 UPDATE: %v %v", ph, ok)
	}
	if _, ok := dshToolPhase("quantum"); ok {
		t.Fatal("未知状态不得被认识(进诊断不猜)")
	}
}

// ---- CancelRun:手机取消路径(session/cancel 通知 + 部分文本交回)----

func TestDshRunCancelReturnsPartial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "s-c"},
		promptFn: func(callCtx context.Context, _ string, _ []map[string]interface{}, notify func(string, json.RawMessage)) (json.RawMessage, error) {
			// 先吐半截文本,再等取消(ctx 取消后自然返回)
			env, _ := json.Marshal(map[string]interface{}{
				"sessionId": "s-c",
				"update": map[string]interface{}{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]interface{}{"type": "text", "text": "半截"},
				},
			})
			notify("session/update", env)
			<-callCtx.Done()
			return nil, callCtx.Err()
		},
	}
	p := newTestDshProvider(t, fake)
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	blocks, _, err := p.Run(ctx, TurnRequest{Thread: &store.Thread{ID: "th-c", AgentID: "dsh"}, Workspace: `G:\ws`, InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}}, func(*pb.TurnServerMessage) {}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("取消后应返回 ctx.Err: %v", err)
	}
	if len(blocks) != 1 || blocks[0].GetText().GetText() != "半截" {
		t.Fatalf("已流出内容必须交回: %+v", blocks)
	}
	if n := len(fake.snapshot().cancelHits); n != 1 {
		t.Fatalf("应发一次 session/cancel: %d", n)
	}
}

// TestDshCloseSession 远端删除走 session/close。
func TestDshCloseSession(t *testing.T) {
	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)
	if err := p.CloseSession("s-del"); err != nil {
		t.Fatalf("CloseSession 应成功: %v", err)
	}
	if got := fake.snapshot().closeHits; len(got) != 1 || got[0] != "s-del" {
		t.Fatalf("应恰好 close 一次: %v", got)
	}
}

// ---- T-DSH 问题 B 回归:生命周期锁竞争与启动预热 ----
//
// 真机实证(2026-09-21 daemon.log):dsh 首跑握手 16:19:36→16:38:22(十位数分钟),
// 期间 ListModels/轮次就绪撞锁 busy-reject("dsh 正忙")。本组用例钉死修复语义:
// 有界排队替代立即拒绝;预热完成一次握手后用户路径零重复握手。

// holdLifeFor 用一个真实 withClientCtx 占住 lifeMu 一段时间(经 chan 通知已持锁)。
func holdLifeFor(p *DshProvider, hold time.Duration, acquired chan<- struct{}) {
	_ = p.withClientCtx(context.Background(), 0, func(context.Context, dshClient) error {
		acquired <- struct{}{} // 已持锁
		time.Sleep(hold)
		return nil
	})
}

// TestDshLockContentionQueuesInsteadOfBusyRejecting 核心回归:锁被短暂占用时,
// 后续调用应"排队等锁成功",而不是像旧实现那样立即 busy-reject —— 真机上
// "模型取样/轮次就绪与慢握手人挤人"就是靠这条从"正忙报错"变成"先后等待"。
func TestDshLockContentionQueuesInsteadOfBusyRejecting(t *testing.T) {
	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)
	p.client = fake // 预置客户端,两个调用都不走 startClient

	acquired := make(chan struct{}, 1)
	go holdLifeFor(p, 150*time.Millisecond, acquired)
	<-acquired // 持锁方已就位

	// 排队方:持有方只占 150ms,排队窗口(生产 15s)内必然等到锁 → 必须成功。
	if err := p.withClientCtx(context.Background(), 0, func(context.Context, dshClient) error {
		return nil
	}); err != nil {
		t.Fatalf("锁被短暂占用时后续调用应排队成功(旧实现此处 busy-reject): %v", err)
	}
}

// TestDshLockContentionBusyAfterWindowExpires 底线回归:持有方真卡死时,排队
// 窗口耗尽后仍必须回 dshBusy —— "一个卡死的操作不拖住全线"的 TryLock 纪律
// 弱化为"有界拖"后依然成立(窗口用包级变量注入短值,不拖慢测试)。
func TestDshLockContentionBusyAfterWindowExpires(t *testing.T) {
	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)
	p.client = fake

	oldWait := dshLockWait
	dshLockWait = 30 * time.Millisecond
	t.Cleanup(func() { dshLockWait = oldWait })

	acquired := make(chan struct{}, 1)
	go holdLifeFor(p, 300*time.Millisecond, acquired)
	<-acquired

	err := p.withClientCtx(context.Background(), 0, func(context.Context, dshClient) error {
		return nil
	})
	if !errors.Is(err, dshBusy) {
		t.Fatalf("排队窗口耗尽后应回 dshBusy: %v", err)
	}
}

// TestDshLockWaitNeverExceedsCallerDeadline ctx 传导回归:调用方自带截止时间时,
// 排队窗口必须收窄到剩余时长之内(等待 + 执行的总延迟不越过调用方预算)。
func TestDshLockWaitNeverExceedsCallerDeadline(t *testing.T) {
	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)
	p.client = fake

	acquired := make(chan struct{}, 1)
	go holdLifeFor(p, 250*time.Millisecond, acquired)
	<-acquired

	// 只给 50ms 截止:等锁就该耗掉它,锁后执行必超时 → 应得到 DeadlineExceeded,
	// 而不是越过自己的预算强行再等 15s 生产窗口。
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := p.withClientCtx(ctx, 0, func(context.Context, dshClient) error {
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, dshBusy) {
		t.Fatalf("短截止时间调用应受自身预算约束(DeadlineExceeded 或 busy),不该越过: %v", err)
	}
	// 等持有方退出,避免污染下个用例(锁释放后尾随睡眠一次)。
	time.Sleep(250 * time.Millisecond)
}

// TestDshWarmupCompletesHandshakeOnce 预热回归:Warmup 完成一次握手后 inited
// 缓存生效,再次预热/用户路径不再重复 initialize(用户路径零握手开销的根据)。
func TestDshWarmupCompletesHandshakeOnce(t *testing.T) {
	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)

	p.Warmup()
	p.Warmup() // 第二次应为缓存命中,不再握手
	fake.mu.Lock()
	n := fake.initCalls
	fake.mu.Unlock()
	if n != 1 {
		t.Fatalf("预热两次应只握手一次(旧懒启动路径每次调用都会重握手): %d", n)
	}
}

// TestDshWarmupRetriesUntilHandshakeSucceeds 预热回归:首跑握手失败(真实世界
// dsh 首跑可拖十位数分钟)时按注入的短周期重试,凭据/启动就绪后自动成功 ——
// 对应真机"16:19:36 拉起 → 16:38:22 握手完成"窗口内预热持续推进的事实。
func TestDshWarmupRetriesUntilHandshakeSucceeds(t *testing.T) {
	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)

	oldRetry, oldTries := dshWarmupRetry, dshWarmupRetries
	dshWarmupRetry, dshWarmupRetries = time.Millisecond, 3
	t.Cleanup(func() { dshWarmupRetry, dshWarmupRetries = oldRetry, oldTries })

	// 前三次握手都以"到点超时"失败(DeadlineExceeded 分支:不重启子进程,
	// 与真机"子进程继续首跑、调用方如超返回"一致)。
	fake.mu.Lock()
	fake.initErr = context.DeadlineExceeded
	fake.mu.Unlock()
	p.Warmup()

	fake.mu.Lock()
	n, stillErr := fake.initCalls, fake.initErr
	fake.mu.Unlock()
	if stillErr == nil || n != 3 {
		t.Fatalf("预热应按窗口重试 3 次后如实放弃: initCalls=%d err=%v", n, stillErr)
	}

	// 第 4 个周期握手就绪:Warmup 成功并缓存。
	fake.mu.Lock()
	fake.initErr = nil
	fake.mu.Unlock()
	p.Warmup()
	fake.mu.Lock()
	n = fake.initCalls
	fake.mu.Unlock()
	if n != 4 {
		t.Fatalf("恢复后再次预热应成功: initCalls=%d", n)
	}
}

// TestDshUsageUpdateVariantProjectsUsage 0.1.5-rc.3 探针回归钉(dsh-p0-probe.md §3):
// usage_update 变体 {"sessionUpdate":"usage_update","used":N,"size":M} 每轮至少 1 帧,
// 必须投影为 UsageUpdate(此前落 default 变 UNKNOWN,每轮必脏台账);
// 字段缺失时不猜(投影失败回落诊断透传)。
func TestDshUsageUpdateVariantProjectsUsage(t *testing.T) {
	frame := func(body string) json.RawMessage {
		return json.RawMessage(`{"sessionUpdate":"usage_update","sessionId":"s1","update":` + body + `}`)
	}
	// 探针真实形状:update 即 {used,size}(无嵌套字段),handleDshUpdate 收到的 raw
	// 是 update 对象本身(与 context_usage 同层),这里按同层形状测。
	real := dshUsageUpdateFromUsedSize(json.RawMessage(`{"used":8057,"size":1000000}`))
	if real == nil || real.Usage == nil || real.Usage.InputTokens != 8057 {
		t.Fatalf("探针实测形状应投影 InputTokens=used: %+v", real)
	}
	if dshUsageUpdateFromUsedSize(json.RawMessage(`{"used":8057}`)) != nil {
		t.Fatalf("缺 size 不应投影(不猜): 帧应落诊断透传")
	}
	_ = frame
}
