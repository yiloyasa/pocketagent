// session_resolution_characteristics_20261009_test.go —— B1「bindSession 上收」的
// 特征测试(P1-7):在把生产 Run 的会话解析改调 NativeSession 适配器**之前**,
// 先把四家现行为钉死在这里 —— resume 命中 / 同代短路 / 降级新建三条路径。
// 重构后这些用例必须原样全绿(行为不变是验收线),再补充"改走适配器后"的
// 新增矩阵(见各 PostAdoption 用例)。
//
// 【打桩方式随家而异,全部复用各家既有夹具】
//   - dsh:fakeDshClient(newClient 注入),ensureSession 全矩阵可直接驱动;
//   - zcode:fakeZcodeSessionClient(sessionSource 注入);同代短路经 ensureSession,
//     重挂载/新建经唯一实现 resumeZcodeSession/createZcodeSession(Run 链与适配器
//     共用同一对协议实现,钉住它们即钉住两条链);
//   - pi:fakeMountClient(sessionFor 注入),钉 mount 的三分支(同会话短路/
//     switch_session 重挂载/get_state 新建回填)—— pi 的 Run 链与适配器共用 mount;
//   - codex:fake app-server 子进程(CODEX_BIN 自举,见 codex_effect_test.go),
//     Run 级三路径(短路/resume 命中/resume 失败降级新建)走真实 Run 装配。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/dshapp"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// ---- dsh:ensureSession 全矩阵 ------------------------------------------------------

// 同代短路:threadMap 命中当前子进程代 → 直接返回缓存 id,零 RPC、不取客户端。
func TestDshResolveSameGenShortCircuit(t *testing.T) {
	fake := &fakeDshClient{}
	p := newTestDshProvider(t, fake)
	p.setDshThreadRef("t1", threadRef{id: "s-cached", gen: int(p.gen.Load())})
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`}
	sid, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
	if err != nil {
		t.Fatalf("短路应成功: %v", err)
	}
	if sid != "s-cached" {
		t.Fatalf("短路应返回缓存 id, 实得 %q", sid)
	}
	snap := fake.snapshot()
	if len(snap.resumeHits) != 0 || len(snap.newCalls) != 0 {
		t.Fatalf("短路不得发出任何会话 RPC: %+v", snap)
	}
	if fake.initCalls != 0 {
		t.Fatal("短路不得取客户端(不冷启子进程)")
	}
}

// readyDsh 模拟 Run 步骤 0(withClientCtx 握手:置 caps 快照与 p.client),
// ensureSession 的重挂载/降级分支都在此之后发生。
func readyDsh(t *testing.T, p *DshProvider) {
	t.Helper()
	if err := p.withClientCtx(context.Background(), dshInitBudget, func(context.Context, dshClient) error { return nil }); err != nil {
		t.Fatalf("握手就绪失败: %v", err)
	}
}

// 重挂载命中:落库引用在列(sessionId+cwd 双因子)→ resume;成功后登记同代缓存。
func TestDshResolveRemountHit(t *testing.T) {
	fake := &fakeDshClient{
		list: []dshapp.SessionListEntry{{SessionID: "s-old", Cwd: `G:\ws`}},
		resumeBy: map[string]*dshapp.SessionState{
			"s-old": {SessionID: "s-old"},
		},
	}
	p := newTestDshProvider(t, fake)
	readyDsh(t, p)
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "s-old", Cwd: `G:\ws`}}
	sid, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
	if err != nil {
		t.Fatalf("重挂载应成功: %v", err)
	}
	if sid != "s-old" {
		t.Fatalf("应返回重挂载的 sessionId, 实得 %q", sid)
	}
	snap := fake.snapshot()
	if len(snap.resumeHits) != 1 || snap.resumeHits[0] != "s-old" {
		t.Fatalf("应恰好 resume 一次 s-old: %v", snap.resumeHits)
	}
	if len(snap.newCalls) != 0 {
		t.Fatalf("在列重挂载不得新开会话: %v", snap.newCalls)
	}
	if ref, ok := p.dshThreadRefFor("t1"); !ok || ref.id != "s-old" {
		t.Fatalf("重挂载成功应登记同代缓存: %+v ok=%t", ref, ok)
	}
}

// 降级新建:升级断裂(在列 + resume -32602,downgradeable)→ Run 链一次性重建
// session/new 新会话;数据错位(list 不在列)→ 显式报错,绝不静默新开。
func TestDshResolveDowngradeRebuildsAndMisalignmentRejects(t *testing.T) {
	t.Run("升级断裂重建", func(t *testing.T) {
		fake := &fakeDshClient{
			list:      []dshapp.SessionListEntry{{SessionID: "s-old", Cwd: `G:\ws`}},
			resumeErr: &dshapp.RPCError{Method: "session/resume", Code: -32602, Message: "Invalid params"},
			newState:  &dshapp.SessionState{SessionID: "s-rebuilt"},
		}
		p := newTestDshProvider(t, fake)
		readyDsh(t, p)
		th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
			ExternalSession: store.ExternalSessionRef{SessionID: "s-old", Cwd: `G:\ws`}}
		sid, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
		if err != nil {
			t.Fatalf("升级断裂应降级重建而非报错: %v", err)
		}
		if sid != "s-rebuilt" {
			t.Fatalf("应返回重建的新会话 id, 实得 %q", sid)
		}
		if th.ExternalSession.SessionID != "s-rebuilt" {
			t.Fatalf("重建后引用应写回新会话: %+v", th.ExternalSession)
		}
		if ref, ok := p.dshThreadRefFor("t1"); !ok || ref.id != "s-rebuilt" {
			t.Fatalf("重建应登记同代缓存: %+v ok=%t", ref, ok)
		}
	})
	t.Run("数据错位硬拒绝", func(t *testing.T) {
		fake := &fakeDshClient{
			list:     []dshapp.SessionListEntry{{SessionID: "other", Cwd: `G:\ws`}},
			newState: &dshapp.SessionState{SessionID: "should-not-create"},
		}
		p := newTestDshProvider(t, fake)
		readyDsh(t, p)
		th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
			ExternalSession: store.ExternalSessionRef{SessionID: "s-gone", Cwd: `G:\ws`}}
		_, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
		if err == nil {
			t.Fatal("list 不在列必须显式报错")
		}
		if n := len(fake.snapshot().newCalls); n != 0 {
			t.Fatalf("数据错位不得静默新开会话: new=%d", n)
		}
	})
}

// 无引用常规新建:session/new + 引用写回 + 同代缓存登记。
func TestDshResolveCreatesWhenNoReference(t *testing.T) {
	fake := &fakeDshClient{newState: &dshapp.SessionState{SessionID: "s-fresh"}}
	p := newTestDshProvider(t, fake)
	readyDsh(t, p)
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`}
	sid, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
	if err != nil {
		t.Fatalf("新建应成功: %v", err)
	}
	if sid != "s-fresh" || th.ExternalSession.SessionID != "s-fresh" {
		t.Fatalf("新建应返回并写回新会话 id: sid=%q ref=%+v", sid, th.ExternalSession)
	}
	if ref, ok := p.dshThreadRefFor("t1"); !ok || ref.id != "s-fresh" {
		t.Fatalf("新建应登记同代缓存: %+v ok=%t", ref, ok)
	}
}

// ---- zcode:短路 + 共用协议实现 -----------------------------------------------------

// 同代短路(ensureSession 第一优先):threadMap 命中当前代直接返回,零 RPC。
func TestZcodeResolveSameGenShortCircuit(t *testing.T) {
	p := NewZcodeProvider()
	sourced := false
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		sourced = true
		return newFakeZcodeSessionClient("sess-x"), nil
	}
	p.setZcodeThreadRef("t1", threadRef{id: "sess-cached", gen: int(p.gen.Load())})
	sid, created, snap, err := p.ensureSession(context.Background(), TurnRequest{Thread: &store.Thread{ID: "t1"}})
	if err != nil {
		t.Fatalf("短路应成功: %v", err)
	}
	if sid != "sess-cached" || created || snap != nil {
		t.Fatalf("短路应返回缓存: sid=%q created=%t snap=%v", sid, created, snap)
	}
	if sourced {
		t.Fatal("短路不得取客户端")
	}
}

// 重挂载/新建的唯一实现(Run 链与适配器共用):resume 打到落库 id、create 写回引用。
// 钉住这对函数 = 钉住两条链的协议形状(P3 拆分后的结构保证)。
func TestZcodeResolveSharedHelpersShape(t *testing.T) {
	p := NewZcodeProvider()
	fake := newFakeZcodeSessionClient("sess-new")
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "sess-old"}}
	if _, err := p.resumeZcodeSession(context.Background(), fake, "t1", "sess-old", int(p.gen.Load())); err != nil {
		t.Fatalf("resume 唯一实现应成功: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "session/resume:sess-old" {
		t.Fatalf("应恰好 resume 落库 id: %v", fake.calls)
	}
	if ref, ok := p.zcodeThreadRefFor("t1"); !ok || ref.id != "sess-old" {
		t.Fatalf("resume 成功应登记同代缓存: %+v ok=%t", ref, ok)
	}
	sid, created, _, err := p.createZcodeSession(context.Background(), fake, th, th.Workspace, int(p.gen.Load()))
	if err != nil || !created || sid != "sess-new" {
		t.Fatalf("create 唯一实现应成功: sid=%q created=%t err=%v", sid, created, err)
	}
	if th.ExternalSession.SessionID != "sess-new" || th.ExternalSession.Cwd != `G:\ws` {
		t.Fatalf("create 应写回引用: %+v", th.ExternalSession)
	}
}

// 账号同步错误是 zcode 特有的直通语义:resumeZcodeSession 原样上抛 AccountSyncError
// (Run 链据此整轮失败,不降级新建 —— 登录问题开新会话只会掩耳盗铃)。
func TestZcodeResolveAccountSyncErrorPropagates(t *testing.T) {
	p := NewZcodeProvider()
	fake := newFakeZcodeSessionClient("sess-x")
	fake.resumeErr = &zcodeapp.AccountSyncError{Err: errors.New("账号未同步")}
	th := &store.Thread{ID: "t1",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess-old"}}
	_, err := p.resumeZcodeSession(context.Background(), fake, th.ID, "sess-old", int(p.gen.Load()))
	var acc *zcodeapp.AccountSyncError
	if !errors.As(err, &acc) {
		t.Fatalf("AccountSyncError 应原样上抛, 实得 %v", err)
	}
}

// ---- zcode:上收后(经适配器)的 ensureSession 全矩阵 ---------------------------------
//
// P1-7 之前 ensureSession 的 resume/create 分支直连 p.client(真子进程),无法用
// 假客户端驱动;上收后协议路径经适配器 clientFor(sessionSource 钩子),全矩阵
// 可钉。以下用例与 dsh 家的特征测试同构:钉"Run 决策 × 适配器协议"的组合行为。

// zcodeSessionMatrix 夹具:provider + 经 sessionSource 注入的假客户端。
func zcodeSessionMatrix(resumeErr error) (*ZcodeProvider, *fakeZcodeSessionClient) {
	p := NewZcodeProvider()
	fake := newFakeZcodeSessionClient("sess-rebuilt")
	fake.resumeErr = resumeErr
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) { return fake, nil }
	return p, fake
}

// resume 命中:session/resume 打到落库 id + subscribe 紧随(轮次内状态机),
// 不发 session/create。
func TestZcodeResolvePostAdoptionResumeHit(t *testing.T) {
	p, fake := zcodeSessionMatrix(nil)
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "sess-old"}}
	sid, created, _, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
	if err != nil {
		t.Fatalf("重挂载应成功: %v", err)
	}
	if sid != "sess-old" || created {
		t.Fatalf("应返回落库 id 且非新建: sid=%q created=%t", sid, created)
	}
	want := []string{"session/resume:sess-old", "session/subscribe"}
	if strings.Join(fake.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("调用轨迹应为 resume→subscribe, 实得 %v", fake.calls)
	}
	if ref, ok := p.zcodeThreadRefFor("t1"); !ok || ref.id != "sess-old" {
		t.Fatalf("resume 成功应登记同代缓存: %+v ok=%t", ref, ok)
	}
}

// resume 失败(非账号错误)→ 降级"改开新会话":session/create 接力 + subscribe;
// 账号错误 → 原样上抛整轮失败,绝不降级新建。
func TestZcodeResolvePostAdoptionDowngradeAndAccount(t *testing.T) {
	t.Run("resume失败改开新会话", func(t *testing.T) {
		p, fake := zcodeSessionMatrix(errors.New("session expired"))
		th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
			ExternalSession: store.ExternalSessionRef{SessionID: "sess-old"}}
		sid, created, _, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
		if err != nil {
			t.Fatalf("降级新建应成功: %v", err)
		}
		if sid != "sess-rebuilt" || !created {
			t.Fatalf("应降级到新会话: sid=%q created=%t", sid, created)
		}
		if th.ExternalSession.SessionID != "sess-rebuilt" {
			t.Fatalf("新会话引用应写回: %+v", th.ExternalSession)
		}
		want := []string{"session/resume:sess-old", "session/create", "session/subscribe"}
		if strings.Join(fake.calls, ",") != strings.Join(want, ",") {
			t.Fatalf("调用轨迹应为 resume→create→subscribe, 实得 %v", fake.calls)
		}
	})
	t.Run("账号错误直通不降级", func(t *testing.T) {
		p, fake := zcodeSessionMatrix(&zcodeapp.AccountSyncError{Err: errors.New("未同步")})
		th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
			ExternalSession: store.ExternalSessionRef{SessionID: "sess-old"}}
		_, _, _, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
		var acc *zcodeapp.AccountSyncError
		if !errors.As(err, &acc) {
			t.Fatalf("AccountSyncError 应原样上抛, 实得 %v", err)
		}
		for _, c := range fake.calls {
			if c == "session/create" {
				t.Fatal("账号错误不得降级新建")
			}
		}
	})
}

// 无引用常规新建:session/create + 引用写回 + subscribe + 同代缓存登记。
func TestZcodeResolvePostAdoptionCreate(t *testing.T) {
	p, fake := zcodeSessionMatrix(nil)
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`}
	sid, created, _, err := p.ensureSession(context.Background(), TurnRequest{Thread: th, Workspace: th.Workspace})
	if err != nil {
		t.Fatalf("新建应成功: %v", err)
	}
	if sid != "sess-rebuilt" || !created || th.ExternalSession.SessionID != "sess-rebuilt" {
		t.Fatalf("新建应返回并写回: sid=%q created=%t ref=%+v", sid, created, th.ExternalSession)
	}
	want := []string{"session/create", "session/subscribe"}
	if strings.Join(fake.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("调用轨迹应为 create→subscribe, 实得 %v", fake.calls)
	}
	if ref, ok := p.zcodeThreadRefFor("t1"); !ok || ref.id != "sess-rebuilt" {
		t.Fatalf("新建应登记同代缓存: %+v ok=%t", ref, ok)
	}
}

// ---- pi:mount 三分支(Run 链与适配器共用同一实现)-----------------------------------

// fakeResolveMountClient pi mount 打桩:记录 switch/get_state 轨迹。
type fakeResolveMountClient struct {
	sessionFile string
	sessionID   string
	switched    []string
	getState    int
}

func (f *fakeResolveMountClient) SessionFile() string { return f.sessionFile }
func (f *fakeResolveMountClient) SessionID() string   { return f.sessionID }
func (f *fakeResolveMountClient) Close()              {}
func (f *fakeResolveMountClient) GetState(context.Context) (map[string]any, error) {
	f.getState++
	return map[string]any{}, nil
}
func (f *fakeResolveMountClient) SwitchSession(_ context.Context, sessionFile string) error {
	f.switched = append(f.switched, sessionFile)
	f.sessionFile = sessionFile
	return nil
}
func (f *fakeResolveMountClient) Call(string, map[string]any, time.Duration) (json.RawMessage, error) {
	return nil, nil
}

// 同会话短路:子进程已挂目标会话 → 不 switch、不 get_state,原样返回。
func TestPiResolveMountSameSessionShortCircuit(t *testing.T) {
	p := NewPiProvider()
	fake := &fakeResolveMountClient{sessionFile: `G:\pi\sessions\a.json`, sessionID: "sid-a"}
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "sid-a", SessionFile: `G:\pi\sessions\a.json`}}
	if err := p.mount(context.Background(), th, fake); err != nil {
		t.Fatalf("同会话应短路成功: %v", err)
	}
	if len(fake.switched) != 0 || fake.getState != 0 {
		t.Fatalf("同会话短路不得 switch/get_state: switched=%v getState=%d", fake.switched, fake.getState)
	}
}

// 引用不符 → switch_session 重挂载;空引用 → get_state 回填新建。
func TestPiResolveMountSwitchAndBackfill(t *testing.T) {
	p := NewPiProvider()
	t.Run("switch重挂载", func(t *testing.T) {
		fake := &fakeResolveMountClient{sessionFile: `G:\pi\sessions\b.json`}
		th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
			ExternalSession: store.ExternalSessionRef{SessionFile: `G:\pi\sessions\a.json`}}
		if err := p.mount(context.Background(), th, fake); err != nil {
			t.Fatalf("重挂载应成功: %v", err)
		}
		if len(fake.switched) != 1 || !strings.HasSuffix(fake.switched[0], "a.json") {
			t.Fatalf("应恰好 switch 到目标会话: %v", fake.switched)
		}
		if fake.getState != 0 {
			t.Fatalf("重挂载不得 get_state: %d", fake.getState)
		}
	})
	t.Run("get_state回填", func(t *testing.T) {
		fake := &fakeResolveMountClient{sessionFile: `G:\pi\sessions\c.json`, sessionID: "sid-c"}
		th := &store.Thread{ID: "t1", Workspace: `G:\ws`}
		if err := p.mount(context.Background(), th, fake); err != nil {
			t.Fatalf("新建回填应成功: %v", err)
		}
		if fake.getState != 1 || len(fake.switched) != 0 {
			t.Fatalf("新建应恰好一次 get_state 且不 switch: getState=%d switched=%v", fake.getState, fake.switched)
		}
		if th.ExternalSession.SessionID != "sid-c" || th.ExternalSession.SessionFile != `G:\pi\sessions\c.json` {
			t.Fatalf("新建应回填引用: %+v", th.ExternalSession)
		}
	})
}

// ---- codex:Run 级三路径(fake app-server,走真实 Run 装配)--------------------------

// 同代短路:threadMap 命中当前代 → 本轮零 thread/start、零 thread/resume,
// turn/start 直接打缓存 threadId。【先就绪再登记】gen 随子进程(重)启 +1,
// 必须先让 withClient 把子进程拉起、再按当前代登记缓存,否则 Run 里代已
// 变、缓存必失效(这正是同代缓存的代语义)。
func TestCodexResolveSameGenShortCircuit(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	th := &store.Thread{ID: "tc-1", Workspace: t.TempDir()}
	if err := p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		t.Fatalf("子进程就绪失败: %v", err)
	}
	p.setCodexThreadRef("tc-1", threadRef{id: "cached-tid", gen: int(p.gen.Load())})
	runOneTurn(t, p, th, th.Workspace)
	reqs := fakeCodexRequests(t, logPath)
	has := func(m string) bool {
		for _, r := range reqs {
			if r.Method == m {
				return true
			}
		}
		return false
	}
	if has("thread/start") || has("thread/resume") {
		t.Fatalf("同代短路不得发 thread/start 或 thread/resume: %v", reqs)
	}
	sp := findStartParams(t, reqs, "turn/start")
	if sp["threadId"] != "cached-tid" {
		t.Fatalf("turn/start 应打缓存 threadId, 实得 %v", sp["threadId"])
	}
}

// resume 命中:落库 CodexThreadID + 无同代缓存 → thread/resume 打到该 id,
// 不发 thread/start;本轮稳定在该线程上。
func TestCodexResolveResumeHit(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	th := &store.Thread{ID: "tc-2", Workspace: t.TempDir(), CodexThreadID: "tid-resume"}
	runOneTurn(t, p, th, th.Workspace)
	reqs := fakeCodexRequests(t, logPath)
	rp := findStartParams(t, reqs, "thread/resume")
	if rp["threadId"] != "tid-resume" {
		t.Fatalf("thread/resume 应打到落库 id, 实得 %v", rp["threadId"])
	}
	hasStart := false
	for _, r := range reqs {
		if r.Method == "thread/start" {
			hasStart = true
		}
	}
	if hasStart {
		t.Fatal("resume 命中不得再发 thread/start")
	}
	if ref, ok := p.codexThreadRef("tc-2"); !ok || ref.id != "tid-resume" {
		t.Fatalf("resume 成功应登记同代缓存: %+v ok=%t", ref, ok)
	}
}

// resume 失败 → 降级新建:改发 thread/start,新 threadId 写回 thread 并登记缓存
// (历史教训:缺这条降级护栏时表现为"AI 失忆")。
func TestCodexResolveResumeFailureDowngradesToStart(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_RESUME_ERROR", "session gone")
	th := &store.Thread{ID: "tc-3", Workspace: t.TempDir(), CodexThreadID: "tid-dead"}
	runOneTurn(t, p, th, th.Workspace)
	reqs := fakeCodexRequests(t, logPath)
	findStartParams(t, reqs, "thread/resume") // 必须真的尝试过 resume
	findStartParams(t, reqs, "thread/start")  // 降级必须改发 thread/start
	// 本轮稳定在新线程上:turn/start 打的 threadId 是新建的,不是死引用。
	if tp := findStartParams(t, reqs, "turn/start"); tp["threadId"] == "tid-dead" || tp["threadId"] == "" {
		t.Fatalf("降级后 turn/start 应打新 threadId, 实得 %v", tp["threadId"])
	}
	if th.CodexThreadID == "" || th.CodexThreadID == "tid-dead" {
		t.Fatalf("降级新建应把新 threadId 写回 thread, 实得 %q", th.CodexThreadID)
	}
	if ref, ok := p.codexThreadRef("tc-3"); !ok || ref.id != th.CodexThreadID {
		t.Fatalf("降级新建应登记同代缓存: %+v ok=%t", ref, ok)
	}
}

// ---- pi/codex:上收后(P1-7 收尾)经适配器入口的 Run 链矩阵 ---------------------------
//
// 与 zcode 的 PostAdoption 三例同构:钉"Run 决策 × 适配器协议"的入口接线。
//   - pi 的 Run 无法用假客户端整轮驱动(轮次内状态机要真 *piapp.Client),矩阵例
//     钉的是 Run 现在走的那个入口:NativeSessionFor 交付 *piSession(Run 的断言
//     目标)+ Open/Resume 经适配器复现 Run 旧链(clientFor→mount)的调用轨迹;
//     握手形状本身由 TestPiResolveMount*(共享 mount)与 TestPiSession*(适配器
//     契约)钉住,这里钉"入口换轨不换行为"。
//   - codex 的 Run 级三路径(上方 TestCodexResolve*)已经走真实 Run 装配(经
//     适配器后原样全绿即上收直证);此处补钉 Run 专属的新入口 openForRun:
//     ephemeral/config 覆盖是 Run 的一次性请求参数,只有这条入口携带。

// piSessionMatrix 夹具:provider + 经 sessionFor 注入的假挂载源(zcodeSessionMatrix
// 同构)。seen 记录每次交付的假客户端,断言调用轨迹用。
func piSessionMatrix() (*PiProvider, *piSession, *[]*fakeResolveMountClient) {
	p := NewPiProvider()
	seen := &[]*fakeResolveMountClient{}
	p.sessionFor = func(*store.Thread) (piMountClient, error) {
		c := &fakeResolveMountClient{sessionFile: `G:\pi\sessions\fresh.json`, sessionID: "sid-fresh"}
		*seen = append(*seen, c)
		return c, nil
	}
	return p, &piSession{p: p}, seen
}

func TestPiResolvePostAdoptionRunEntryViaAdapter(t *testing.T) {
	// Run 的入口契约:NativeSessionFor 交付的正是 *piSession(pi.go Run 断言的
	// 具体类型);该形态下交付不出来,NativeSessionFor 不返回 error。
	p0 := NewPiProvider()
	ns, err := p0.NativeSessionFor(&store.Thread{ID: "t0"})
	if err != nil {
		t.Fatalf("工厂不应报错: %v", err)
	}
	if _, ok := ns.(*piSession); !ok {
		t.Fatalf("Run 入口断言目标应是 *piSession, 实得 %T", ns)
	}

	t.Run("重挂载=Run断线恢复轨迹", func(t *testing.T) {
		_, ps, seen := piSessionMatrix()
		th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
			ExternalSession: store.ExternalSessionRef{SessionID: "sid-old", SessionFile: `G:\pi\sessions\a.json`}}
		if err := ps.Resume(context.Background(), th); err != nil {
			t.Fatalf("经适配器重挂载应成功: %v", err)
		}
		if len(*seen) != 1 {
			t.Fatalf("应恰好取一次客户端, 实得 %d", len(*seen))
		}
		c := (*seen)[0]
		if len(c.switched) != 1 || !strings.HasSuffix(c.switched[0], "a.json") || c.getState != 0 {
			t.Fatalf("重挂载轨迹应恰好一次 switch 且零 get_state: switched=%v getState=%d", c.switched, c.getState)
		}
	})
	t.Run("新建=Run首轮轨迹", func(t *testing.T) {
		_, ps, seen := piSessionMatrix()
		th := &store.Thread{ID: "t2", Workspace: `G:\ws`}
		if err := ps.Open(context.Background(), th); err != nil {
			t.Fatalf("经适配器新建应成功: %v", err)
		}
		c := (*seen)[0]
		if c.getState != 1 || len(c.switched) != 0 {
			t.Fatalf("新建轨迹应恰好一次 get_state 且零 switch: getState=%d switched=%v", c.getState, c.switched)
		}
		if th.ExternalSession.SessionID != "sid-fresh" || th.ExternalSession.SessionFile != `G:\pi\sessions\fresh.json` {
			t.Fatalf("新建应经 mount 回填引用: %+v", th.ExternalSession)
		}
	})
}

// Run 专属新建入口 openForRun:ephemeral 与 config 覆盖显式携带(合同面 Open 不带,
// 见 codexSession 类型注释),threadId 写回 + 同代缓存登记与 Run 旧链一字不差。
func TestCodexResolvePostAdoptionOpenForRun(t *testing.T) {
	p, cs, fake := newCodexSessionFixture()
	th := &store.Thread{ID: "tc-pa", Workspace: `G:\ws`}
	tid, err := cs.openForRun(context.Background(), th, true,
		map[string]interface{}{"model_verbosity": "low"})
	if err != nil {
		t.Fatalf("经适配器新建应成功: %v", err)
	}
	if tid == "" || th.CodexThreadID != tid {
		t.Fatalf("新 threadId 应写回 thread: tid=%q ref=%q", tid, th.CodexThreadID)
	}
	if ref, ok := p.codexThreadRef("tc-pa"); !ok || ref.id != tid || ref.gen != int(p.gen.Load()) {
		t.Fatalf("新建应登记同代缓存(当前代): %+v ok=%t want-gen=%d", ref, ok, int(p.gen.Load()))
	}
	if len(fake.calls) != 1 || fake.calls[0].method != "thread/start" {
		t.Fatalf("应恰好一次 thread/start: %+v", fake.calls)
	}
	sp := fake.calls[0].params
	if sp["cwd"] != `G:\ws` {
		t.Fatalf("cwd 应取 thread 落库值, 实得 %v", sp["cwd"])
	}
	if sp["ephemeral"] != true {
		t.Fatalf("Run 一次性参数 ephemeral 应显式携带, 实得 %v", sp["ephemeral"])
	}
	cfg, ok := sp["config"].(map[string]interface{})
	if !ok || cfg["model_verbosity"] != "low" {
		t.Fatalf("config 覆盖应显式携带: %v", sp["config"])
	}
}
