// zcode_session_test.go —— ZCode 原生会话适配器(P3 收编)的单测与契约正反例。
//
// 【打桩方式】fakeZcodeSessionClient 实现 zcodeSessionClient 窄接口(与 pi 的
// fakeMountClient、dsh 的 fakeDshClient 同款打桩模式),不拉真子进程 —— 钉的是
// zcode.go 的 session/resume 重挂载与 session/create 新建这**唯一实现**
// (resumeZcodeSession/createZcodeSession)经适配器的接线形状,不是第二实现的形状。
//
// 【D4 语义钉】zcode 的 Run 链(ensureSession)在 resume 失败时会"改开新会话",
// 那是会话级产品决策;适配器 Resume 失败必须**原样上抛且绝不发出 session/create**
// —— 降级决策留给调用方,账号同步错误直通(设计稿 §5 不做清单点名的形态)。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// fakeZcodeSessionClient zcodeSessionClient 打桩:记录调用轨迹,模拟某一代
// zcode 子进程的会话面。单 goroutine 使用(适配器契约:仅轮次 goroutine 调用),
// 无需加锁。
type fakeZcodeSessionClient struct {
	nextSID   string   // session/create 应答的 sessionId
	resumeErr error    // session/resume 注入错(模拟重挂载失败)
	createErr error    // session/create 注入错(模拟新建失败)
	calls     []string // 轨迹:"session/resume:<sid>" / "session/create"
}

func newFakeZcodeSessionClient(sid string) *fakeZcodeSessionClient {
	return &fakeZcodeSessionClient{nextSID: sid}
}

func (f *fakeZcodeSessionClient) CallCtx(_ context.Context, method string, params interface{}) (json.RawMessage, error) {
	switch method {
	case "session/resume":
		sid := ""
		if m, ok := params.(map[string]interface{}); ok {
			sid, _ = m["sessionId"].(string)
		}
		f.calls = append(f.calls, "session/resume:"+sid)
		if f.resumeErr != nil {
			return nil, f.resumeErr
		}
		// 快照原文给最小形状:recordProjection 对缺失字段静默跳过(与生产一致)。
		return json.RawMessage(`{"session":{"sessionId":"` + sid + `"}}`), nil
	case "session/create":
		f.calls = append(f.calls, "session/create")
		if f.createErr != nil {
			return nil, f.createErr
		}
		return json.RawMessage(`{"session":{"sessionId":"` + f.nextSID + `"}}`), nil
	}
	f.calls = append(f.calls, method)
	return nil, nil
}

// newZcodeSessionFixture 适配器 + 假客户端源的标准夹具:每次 clientFor 给一个
// 全新"代"的假客户端(模拟每次挂载都是新子进程),引用接力全靠
// thread.ExternalSession —— 这正是收编要钉的"按落库引用重挂载"。
func newZcodeSessionFixture(sid string) (*ZcodeProvider, *zcodeSession, *fakeZcodeSessionClient) {
	p := NewZcodeProvider()
	fake := newFakeZcodeSessionClient(sid)
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) { return fake, nil }
	return p, &zcodeSession{p: p}, fake
}

// ---- Source 同源(D2)----

// TestZcodeSessionSourceIsDeclaredShape:Source() 必须与 zcodeShape.sessionModel
// 是同一个表达式取值 —— 契约套件的 SessionSourceMatchesDescriptor 与 VerifyRuntime
// 的 source-mismatch 检查都建立在这条同源关系上,写字面量就是造第二事实源。
func TestZcodeSessionSourceIsDeclaredShape(t *testing.T) {
	_, s, _ := newZcodeSessionFixture("sess-x")
	if got := s.Source(); got != zcodeShape.sessionModel {
		t.Fatalf("Source()=%q 与 zcodeShape.sessionModel=%q 不同源", got, zcodeShape.sessionModel)
	}
	if got := s.Source(); got != runtime.SourceHybrid {
		t.Fatalf("zcode 自报 SessionModel=HYBRID,适配器 Source() 实得 %q(设计稿:这一条没有自由度)", got)
	}
}

// ---- Open:session/create 既有路径写回引用 ----

// Open 走 session/create(唯一实现 createZcodeSession):写回
// ExternalSession.SessionID/Cwd + 登记 threadMap 同代缓存;绝不 resume(那是
// Resume 的事)、绝不 subscribe(那是 Run 轮次内状态机的事,设计稿 §4-P3)。
func TestZcodeSessionOpenCreatesAndBackfillsReference(t *testing.T) {
	p, s, fake := newZcodeSessionFixture("sess-new-1")
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "stale"}}
	if err := s.Open(context.Background(), th); err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "session/create" {
		t.Fatalf("Open 应恰好一次 session/create, 实得 %v", fake.calls)
	}
	if th.ExternalSession.SessionID != "sess-new-1" || th.ExternalSession.Cwd != `G:\ws` {
		t.Fatalf("引用应由 session/create 路径写回: %+v", th.ExternalSession)
	}
	ref, ok := p.zcodeThreadRefFor("t1")
	if !ok || ref.id != "sess-new-1" || ref.gen != int(p.gen.Load()) {
		t.Fatalf("同代缓存应登记: ref=%+v ok=%t gen=%d", ref, ok, p.gen.Load())
	}
}

// Open 失败不得毁掉调用方的落库引用:createZcodeSession 在写回之前返回,
// thread.ExternalSession 原样保留(引用与子进程实际挂载态保持一致)。
func TestZcodeSessionOpenFailureKeepsReference(t *testing.T) {
	_, s, fake := newZcodeSessionFixture("sess-new-2")
	fake.createErr = errors.New("session/create 被拒")
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "keep", Cwd: `G:\old`}}
	if err := s.Open(context.Background(), th); err == nil {
		t.Fatal("session/create 失败应报错")
	}
	if th.ExternalSession.SessionID != "keep" || th.ExternalSession.Cwd != `G:\old` {
		t.Fatalf("失败的 Open 应保留旧引用, 实得 %+v", th.ExternalSession)
	}
}

// ---- Resume:按落库引用重挂载(复用 session/resume 唯一实现)----

// 有引用 → session/resume 打到落库的 sessionId;成功后与 ensureSession 重挂载
// 成功路径同款登记 threadMap 同代缓存;不新建、不订阅。
func TestZcodeSessionResumeRemountsViaSessionResume(t *testing.T) {
	p, s, fake := newZcodeSessionFixture("sess-new-3")
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "sess-old", Cwd: `G:\ws`}}
	if err := s.Resume(context.Background(), th); err != nil {
		t.Fatalf("重挂载 Resume 失败: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "session/resume:sess-old" {
		t.Fatalf("应恰好 resume 一次落库引用, 实得 %v", fake.calls)
	}
	ref, ok := p.zcodeThreadRefFor("t1")
	if !ok || ref.id != "sess-old" || ref.gen != int(p.gen.Load()) {
		t.Fatalf("重挂载成功应登记同代缓存: ref=%+v ok=%t", ref, ok)
	}
}

// 契约反证的对偶(无引用 Resume 必错):错误必须是 runtime.ErrNoNativeSession
// 同族(直接复用该哨兵),且**绝不取客户端** —— 不为一个没有引用的线程拉起
// zcode 子进程(对齐 piSession/dshSession/memSession 契约)。
func TestZcodeSessionResumeWithoutReferenceFailsWithSentinel(t *testing.T) {
	p := NewZcodeProvider()
	called := false
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		called = true
		return newFakeZcodeSessionClient("sess-never"), nil
	}
	s := &zcodeSession{p: p}
	err := s.Resume(context.Background(), &store.Thread{ID: "stranger"})
	if !errors.Is(err, runtime.ErrNoNativeSession) {
		t.Fatalf("无引用 Resume 应返回 ErrNoNativeSession 同族错误, 实得 %v", err)
	}
	if called {
		t.Fatal("无引用 Resume 不得取客户端(不许冷启子进程)")
	}
}

// resume 失败必须原样上抛且**绝不发出 session/create**:Run 链(ensureSession)
// 的"改开新会话"降级是会话级产品决策,留在 Run(D4);适配器吞错降级就会把
// "恢复失败"伪装成"恢复成功后开了新会话",用户老会话历史看似全丢。
func TestZcodeSessionResumeFailurePropagatesWithoutNewSession(t *testing.T) {
	_, s, fake := newZcodeSessionFixture("sess-should-not-exist")
	fake.resumeErr = errors.New("session/resume -32004 Session not found")
	th := &store.Thread{ID: "t1",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess-gone", Cwd: `G:\ws`}}
	if err := s.Resume(context.Background(), th); err == nil {
		t.Fatal("resume 失败必须上报,不得假装恢复成功")
	}
	for _, c := range fake.calls {
		if c == "session/create" {
			t.Fatalf("适配器不得擅自新建会话: %v", fake.calls)
		}
	}
}

// 账号同步错误直通:AccountSyncError 不是"会话没了",降级新建会伪造恢复成功
// (ensureSession 对它的直通语义同源,设计稿 §5);适配器路径同样不吞。
func TestZcodeSessionResumeAccountSyncErrorPropagates(t *testing.T) {
	_, s, fake := newZcodeSessionFixture("sess-x")
	fake.resumeErr = &zcodeapp.AccountSyncError{Err: errors.New("账号未登录")}
	th := &store.Thread{ID: "t1",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess-old"}}
	err := s.Resume(context.Background(), th)
	var acctErr *zcodeapp.AccountSyncError
	if !errors.As(err, &acctErr) {
		t.Fatalf("账号同步错误应原样上抛, 实得 %v", err)
	}
	for _, c := range fake.calls {
		if c == "session/create" {
			t.Fatalf("账号同步错误不得降级新建: %v", fake.calls)
		}
	}
}

// ---- Checkpoint / Close:无额外态、无句柄 ----

// Checkpoint 无额外态(引用已在 thread.ExternalSession/CodexThreadID 上,落库归
// server 层);Close 无句柄可释(zcode 的 session/close 是"真删除"语义,归删除
// 路径 DeleteNativeSession,绝不是句柄释放)。
func TestZcodeSessionCheckpointAndCloseAreNoop(t *testing.T) {
	_, s, fake := newZcodeSessionFixture("sess-x")
	th := &store.Thread{ID: "t1"}
	if err := s.Checkpoint(context.Background(), th); err != nil {
		t.Fatalf("Checkpoint 应无额外态返回 nil, 实得 %v", err)
	}
	if err := s.Close(context.Background(), th); err != nil {
		t.Fatalf("Close 应无句柄可释返回 nil, 实得 %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("Checkpoint/Close 不得取客户端发调用(zcode 无对应原生调用): %v", fake.calls)
	}
}

// ---- ensureSession(Run 链)与适配器共用同一实现的反证 ----

// 拆分后 ensureSession 的同代短路仍是第一优先:threadMap 命中当前代直接返回
// 缓存 id,零 RPC、不取客户端 —— 拆分不得改变 Run 链的解析次序。
func TestZcodeEnsureSessionSameGenerationShortCircuitStillFirst(t *testing.T) {
	p := NewZcodeProvider()
	sourced := false
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		sourced = true
		return newFakeZcodeSessionClient("sess-x"), nil
	}
	p.setZcodeThreadRef("t1", threadRef{id: "sess-cached", gen: int(p.gen.Load())})
	sid, created, snap, err := p.ensureSession(context.Background(), TurnRequest{Thread: &store.Thread{ID: "t1"}})
	if err != nil {
		t.Fatalf("ensureSession 失败: %v", err)
	}
	if sid != "sess-cached" || created || snap != nil {
		t.Fatalf("同代短路应返回缓存: sid=%q created=%t snap=%v", sid, created, snap)
	}
	if sourced {
		t.Fatal("同代短路不得取客户端")
	}
}

// ---- VerifyRuntime 正反例(source-mismatch 三重防)----

// 正例:真实 OpenRuntime 交付的 zcode 运行时,原生适配器在场且 Source 与申报
// 同源,VerifyRuntime 必须零违约。
func TestVerifyRuntimeAcceptsZcodeSessionAdapter(t *testing.T) {
	p := NewZcodeProvider()
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		return newFakeZcodeSessionClient("sess-verify-zcode"), nil
	}
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-verify-zcode"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("zcode 的 OpenRuntime 应交付原生会话适配器(P3 收编后不再是 nil)")
	}
	if _, ok := rt.NativeSession.(*zcodeSession); !ok {
		t.Fatalf("应注入 zcodeSession 适配器, 实得 %T", rt.NativeSession)
	}
	if rep := runtime.VerifyRuntime(rt); !rep.OK() {
		for _, v := range rep.Violations {
			t.Errorf("契约违约: %s", v)
		}
	}
}

// 反例:Source 与申报不符的适配器必须被点名 —— 门禁必须能红,否则三重防里的
// VerifyRuntime 这一道只是摆设(pi/dsh 同款反证)。
func TestVerifyRuntimeRejectsWrongZcodeSource(t *testing.T) {
	p := NewZcodeProvider()
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		return newFakeZcodeSessionClient("sess-verify-bad"), nil
	}
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-verify-zcode-bad"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	// 谎报成 DAEMON:正是设计稿点名的"塞替身"违约形态。
	rt.NativeSession = wrongSourceZcodeSession{&zcodeSession{p: p}}
	rep := runtime.VerifyRuntime(rt)
	found := false
	for _, v := range rep.Violations {
		if v.Code == runtime.ViolationSourceMismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("应报告 %q, 实得 %+v", runtime.ViolationSourceMismatch, rep.Violations)
	}
}

// wrongSourceZcodeSession 是反例专用包装:其余方法借 zcodeSession,唯独 Source 撒谎。
type wrongSourceZcodeSession struct{ *zcodeSession }

func (wrongSourceZcodeSession) Source() runtime.SessionSource { return runtime.SourceDaemon }

// ---- 契约套件两会话用例(P3 验收:从"跳过"变"通过")----

// TestZcodeContractSuiteSessionCasesExplicitly 把契约套件的两个会话用例
// (SessionSourceMatchesDescriptor / SessionResumeAfterRestart)对 zcode 点名执行:
// Open → Checkpoint → ExportDurable → ImportDurable(模拟重启)→ Resume 按落库
// 引用重挂载 + stranger 无引用必错的反证。单独点名让红灯直接指到会话契约。
func TestZcodeContractSuiteSessionCasesExplicitly(t *testing.T) {
	p := NewZcodeProvider()
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		return newFakeZcodeSessionClient("sess-contract-zcode"), nil
	}
	th := &store.Thread{ID: "th-contract-zcode-explicit"}
	rt, err := p.OpenRuntime(context.Background(), th)
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	suite := runtime.ContractSuite{T: t, AgentID: p.ID(), RT: rt, Backend: sharedRuntimeBackend(), Thread: th}
	if err := suite.Precondition(); err != nil {
		t.Fatalf("契约套件前条件不满足: %v", err)
	}
	suite.SessionSourceMatchesDescriptor()
	suite.SessionResumeAfterRestart()
}
