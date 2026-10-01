// pi_session_test.go —— Pi 原生会话适配器(P1 收编)的单测与契约正反例。
//
// 【打桩方式】fakeMountClient 实现 mount 握手的窄接口 piMountClient(与
// sessionNameSetter/entriesFetcher 同款打桩模式),不拉真子进程 —— "同代短路、
// 文件不符必 switch、无引用 Resume 必错"这些握手行为钉的是 pi.go mount 这
// **唯一实现**经适配器的接线形状,不是第二实现的形状。
//
// 【VerifyRuntime 正反例】正例 = 真实 OpenRuntime 交付的 pi 运行时零违约;
// 反例 = Source 与申报不符的适配器必须被点名 session.source-mismatch。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeMountClient piMountClient 打桩:记录调用轨迹,模拟"某一代 pi 子进程"的
// 挂载状态。单 goroutine 使用(适配器契约:仅轮次 goroutine 调用),无需加锁。
type fakeMountClient struct {
	file      string // 当前挂载会话文件(SessionFile() 返回值)
	id        string // 当前会话 id(SessionID() 返回值)
	stateErr  error  // GetState 注入错(模拟新建分支握手失败)
	switchErr error  // SwitchSession 注入错(模拟重挂载失败)
	calls     []string
}

func (f *fakeMountClient) SessionFile() string { return f.file }
func (f *fakeMountClient) SessionID() string   { return f.id }

func (f *fakeMountClient) GetState(_ context.Context) (map[string]any, error) {
	f.calls = append(f.calls, "get_state")
	if f.stateErr != nil {
		return nil, f.stateErr
	}
	return map[string]any{"sessionFile": f.file, "sessionId": f.id}, nil
}

func (f *fakeMountClient) SwitchSession(_ context.Context, sessionFile string) error {
	f.calls = append(f.calls, "switch:"+sessionFile)
	if f.switchErr != nil {
		return f.switchErr
	}
	// 真 SwitchSession 校验通过后会更新挂载缓存(piapp.Client.updateSession)。
	f.file = sessionFile
	return nil
}

func (f *fakeMountClient) Call(typ string, _ map[string]any, _ time.Duration) (json.RawMessage, error) {
	f.calls = append(f.calls, "call:"+typ)
	return nil, nil
}

// Close 子进程回收打桩(piMountClient 2026-09-30 起带 Close:fork 的关闭回收
// 需要;mount 握手路径从不调用,不影响既有序列断言)。
func (f *fakeMountClient) Close() {
	f.calls = append(f.calls, "close")
}

// fakeMountSeq 给每"一代"假客户端独立编号:新建会话分支回填的引用与下一代的
// 当前挂载必然不同,模拟"子进程重启后同代缓存作废"的真实形状。
var fakeMountSeq atomic.Int64

func newFakeMountClient() *fakeMountClient {
	n := fakeMountSeq.Add(1)
	return &fakeMountClient{
		file: fmt.Sprintf(`C:\fake\sessions\session-%d.jsonl`, n),
		id:   fmt.Sprintf("sess-%d", n),
	}
}

// injectFakeSessionSource 把假挂载源接进 provider:每次 mountFor 给一个全新
// "代"的假客户端(模拟每次挂载都是新子进程),引用接力全靠 thread.ExternalSession
// —— 这正是收编要钉的"按落库引用重挂载"。返回注入前调用过的客户端快照供断言。
func injectFakeSessionSource(p *PiProvider) *[]*fakeMountClient {
	seen := &[]*fakeMountClient{}
	p.sessionFor = func(*store.Thread) (piMountClient, error) {
		c := newFakeMountClient()
		*seen = append(*seen, c)
		return c, nil
	}
	return seen
}

// newPiSessionFixture 适配器 + 假挂载源的标准夹具。
func newPiSessionFixture() (*PiProvider, *piSession, *[]*fakeMountClient) {
	p := NewPiProvider()
	seen := injectFakeSessionSource(p)
	return p, &piSession{p: p}, seen
}

// newFakePiSessionSource 契约套件(runtime_descriptors_test.go)注入的假挂载源:
// 每次 mountFor 给一个全新"代"的假客户端(模拟子进程重启后同代缓存作废),
// 引用接力全靠 thread.ExternalSession —— Open 回填的引用,Resume 靠它 switch。
func newFakePiSessionSource() func(*store.Thread) (piMountClient, error) {
	return func(*store.Thread) (piMountClient, error) {
		return newFakeMountClient(), nil
	}
}

// ---- Source 同源(D2)----

// TestPiSessionSourceIsDeclaredShape:Source() 必须与 piShape.sessionModel 是
// 同一个表达式取值 —— 契约套件的 SessionSourceMatchesDescriptor 与 VerifyRuntime
// 的 source-mismatch 检查都建立在这条同源关系上。
func TestPiSessionSourceIsDeclaredShape(t *testing.T) {
	_, s, _ := newPiSessionFixture()
	if got := s.Source(); got != piShape.sessionModel {
		t.Fatalf("Source()=%q 与 piShape.sessionModel=%q 不同源", got, piShape.sessionModel)
	}
	if got := s.Source(); got != runtime.SourceHybrid {
		t.Fatalf("Pi 自报 SessionModel=HYBRID,适配器 Source() 实得 %q(设计稿:这一条没有自由度)", got)
	}
}

// ---- Open:置空引用 → mount 新会话分支回填 ----

func TestPiSessionOpenBackfillsReference(t *testing.T) {
	_, s, seen := newPiSessionFixture()
	th := &store.Thread{ID: "t1", Workspace: `C:\w`, Title: "我的会话",
		ExternalSession: store.ExternalSessionRef{SessionID: "stale", SessionFile: `C:\stale.jsonl`}}
	if err := s.Open(context.Background(), th); err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	if len(*seen) != 1 {
		t.Fatalf("Open 应恰好取一次客户端, 实得 %d", len(*seen))
	}
	c := (*seen)[0]
	// 新建分支:get_state 回填,绝不 switch(那是重挂载的事)。
	for _, call := range c.calls {
		if call == "get_state" {
			continue
		}
		if call == "call:set_session_name" {
			continue // 非空标题随 mount 新会话分支下发,双端标题一致
		}
		t.Fatalf("新建分支不应出现 %q 调用: %v", call, c.calls)
	}
	want := store.ExternalSessionRef{SessionID: c.id, SessionFile: c.file, Cwd: th.Workspace}
	if th.ExternalSession != want {
		t.Fatalf("引用应由 mount 回填: want %+v 实得 %+v", want, th.ExternalSession)
	}
}

// Open 失败必须还原旧引用:适配器不得悄悄毁掉调用方的落库引用。
func TestPiSessionOpenFailureRestoresReference(t *testing.T) {
	p := NewPiProvider()
	p.sessionFor = func(*store.Thread) (piMountClient, error) {
		return nil, errors.New("启动 pi rpc 子进程失败")
	}
	s := &piSession{p: p}
	th := &store.Thread{ID: "t1",
		ExternalSession: store.ExternalSessionRef{SessionID: "keep", SessionFile: `C:\keep.jsonl`}}
	if err := s.Open(context.Background(), th); err == nil {
		t.Fatal("客户端源失败应报错")
	}
	if th.ExternalSession.SessionFile != `C:\keep.jsonl` || th.ExternalSession.SessionID != "keep" {
		t.Fatalf("失败的 Open 应还原旧引用, 实得 %+v", th.ExternalSession)
	}
}

// ---- Resume:同代短路 / 文件不符必 switch / 无引用必错 ----

func TestPiSessionResumeSameGenerationShortCircuit(t *testing.T) {
	_, s, seen := newPiSessionFixture()
	// 预置引用与"当前代"客户端的挂载一致:mount 的同代短路必须零 RPC 返回。
	probe := newFakeMountClient()
	th := &store.Thread{ID: "t1", ExternalSession: store.ExternalSessionRef{
		SessionID: probe.id, SessionFile: probe.file}}
	// 把注入源换成"当前代就是 probe"的客户端,验证短路判据。
	s.p.sessionFor = func(*store.Thread) (piMountClient, error) { return probe, nil }
	_ = seen
	if err := s.Resume(context.Background(), th); err != nil {
		t.Fatalf("同代 Resume 失败: %v", err)
	}
	if len(probe.calls) != 0 {
		t.Fatalf("同代短路不应有任何 RPC, 实得 %v", probe.calls)
	}
}

func TestPiSessionResumeMismatchSwitchesSession(t *testing.T) {
	_, s, seen := newPiSessionFixture()
	th := &store.Thread{ID: "t1", ExternalSession: store.ExternalSessionRef{
		SessionID: "sess-old", SessionFile: `C:\fake\sessions\target.jsonl`}}
	if err := s.Resume(context.Background(), th); err != nil {
		t.Fatalf("重挂载 Resume 失败: %v", err)
	}
	c := (*seen)[0]
	// 重挂载分支只做 switch_session(同代短路未触发、新建分支的 get_state 不在):
	// 恰好一次调用,且目标是落库引用里的文件。
	if len(c.calls) != 1 || c.calls[0] != "switch:"+th.ExternalSession.SessionFile {
		t.Fatalf("文件不符必须且只须 switch_session 到落库引用, 实得 %v", c.calls)
	}
}

// switch_session 失败必须原样上报:D4 —— 降级到"新建"的决策留给调用方,
// 适配器不吞错、不擅自重开。
func TestPiSessionResumeSwitchFailurePropagates(t *testing.T) {
	p := NewPiProvider()
	p.sessionFor = func(*store.Thread) (piMountClient, error) {
		return &fakeMountClient{file: `C:\fake\sessions\other.jsonl`, switchErr: errors.New("switch_session 被扩展取消")}, nil
	}
	s := &piSession{p: p}
	th := &store.Thread{ID: "t1", ExternalSession: store.ExternalSessionRef{SessionFile: `C:\fake\sessions\target.jsonl`}}
	if err := s.Resume(context.Background(), th); err == nil {
		t.Fatal("switch_session 失败必须上报,不得假装恢复成功")
	}
}

// 契约反证的对偶(无引用 Resume 必错):错误必须是 runtime.ErrNoNativeSession
// 同族(直接复用该哨兵),且**绝不取客户端** —— 不为一个没有引用的线程冷启子进程。
func TestPiSessionResumeWithoutReferenceFails(t *testing.T) {
	p := NewPiProvider()
	called := false
	p.sessionFor = func(*store.Thread) (piMountClient, error) {
		called = true
		return newFakeMountClient(), nil
	}
	s := &piSession{p: p}
	th := &store.Thread{ID: "stranger"}
	err := s.Resume(context.Background(), th)
	if !errors.Is(err, runtime.ErrNoNativeSession) {
		t.Fatalf("无引用 Resume 应返回 ErrNoNativeSession 同族错误, 实得 %v", err)
	}
	if called {
		t.Fatal("无引用 Resume 不得取客户端(不许冷启子进程)")
	}
}

// ---- Checkpoint / Close:无额外态、无句柄 ----

func TestPiSessionCheckpointAndCloseAreNoop(t *testing.T) {
	p := NewPiProvider()
	called := false
	p.sessionFor = func(*store.Thread) (piMountClient, error) {
		called = true
		return newFakeMountClient(), nil
	}
	s := &piSession{p: p}
	th := &store.Thread{ID: "t1"}
	if err := s.Checkpoint(context.Background(), th); err != nil {
		t.Fatalf("Checkpoint 应无额外态返回 nil, 实得 %v", err)
	}
	if err := s.Close(context.Background(), th); err != nil {
		t.Fatalf("Close 应无句柄可释返回 nil, 实得 %v", err)
	}
	if called {
		t.Fatal("Checkpoint/Close 不得取客户端(pi 无对应原生调用)")
	}
}

// ---- VerifyRuntime 正反例(source-mismatch 三重防)----

// 正例:真实 OpenRuntime 交付的 pi 运行时,原生适配器在场且 Source 与申报同源,
// VerifyRuntime 必须零违约。
func TestVerifyRuntimeAcceptsPiSessionAdapter(t *testing.T) {
	p := NewPiProvider()
	injectFakeSessionSource(p)
	th := &store.Thread{ID: "th-verify-pi"}
	rt, err := p.OpenRuntime(context.Background(), th)
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("pi 的 OpenRuntime 应交付原生会话适配器(P1 收编后不再是 nil)")
	}
	rep := runtime.VerifyRuntime(rt)
	if !rep.OK() {
		for _, v := range rep.Violations {
			t.Errorf("契约违约: %s", v)
		}
	}
}

// 反例:Source 与申报不符的适配器必须被点名 —— 门禁必须能红,否则三重防里的
// VerifyRuntime 这一道只是摆设。
func TestVerifyRuntimeRejectsWrongSourceAdapter(t *testing.T) {
	p := NewPiProvider()
	injectFakeSessionSource(p)
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-verify-pi-bad"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	// 谎报成 DAEMON:正是设计稿点名的"塞替身"违约形态。
	rt.NativeSession = wrongSourceSession{&piSession{p: p}}
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

// wrongSourceSession 是反例专用包装:其余方法借 piSession,唯独 Source 撒谎。
type wrongSourceSession struct{ *piSession }

func (wrongSourceSession) Source() runtime.SessionSource { return runtime.SourceDaemon }

// ---- 契约套件两会话用例(P1 验收:从"跳过"变"通过")----

// TestPiContractSuiteSessionCasesExplicitly 把契约套件的两个会话用例
// (runtime/contract_suite.go 的 SessionSourceMatchesDescriptor /
// SessionResumeAfterRestart)从 RunAll 里单拎出来对 pi 点名执行。收编前
// rt.NativeSession=nil 时两用例在开头静默返回(跳过);收编后必须全链通过:
// Open → Checkpoint → ExportDurable → ImportDurable(模拟重启)→ Resume 按落库
// 引用重挂载 + stranger 无引用必错的反证。单独点名让红灯直接指到会话契约,
// 不淹没在整套 RunAll 里。
func TestPiContractSuiteSessionCasesExplicitly(t *testing.T) {
	p := NewPiProvider()
	injectFakeSessionSource(p)
	th := &store.Thread{ID: "th-contract-pi-explicit"}
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
