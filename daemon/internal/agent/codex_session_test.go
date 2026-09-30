// codex_session_test.go —— Codex 原生会话适配器(P3 收编)的单测与契约正反例。
//
// 【打桩方式】fakeCodexSessionCalls 以函数注入(call func(method, params))替身
// JSON-RPC 调用面 —— 与 callThreadResume 既有的"call 形参注入便于单测(无需真实
// 子进程)"同一基建。钉的是 codex.go 的 thread/start 三级回退解析(startCodexThread
// 唯一实现)与 thread/resume(config 透传,callThreadResume 唯一实现)经适配器的
// 接线形状,不是第二实现的形状。
//
// 【D4 语义钉】codex 的 Run 链在 resume 失败时会"改开新线程"(设计稿点名的
// 护栏教训:缺它时表现为"AI 失忆");那是 Run 的降级决策。适配器 Resume 失败必须
// **原样上抛且绝不发出 thread/start** —— 降级顺序不变:护栏逻辑整体复用
// callThreadResume,适配器只认"resume 成功/失败"两个出口。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeCodexSessionCall 记进轨迹的一条 JSON-RPC 调用(与 codex_read_test.go 的
// fakeCodexCall 同概念,类型名错开:那边的 params 是 RawMessage,这边按 map
// 断言 config 覆盖与 cwd)。
type fakeCodexSessionCall struct {
	method string
	params map[string]interface{}
}

// fakeCodexSessionCalls call 注入的替身:记录调用,按方法给应答。单 goroutine
// 使用(适配器契约:仅轮次 goroutine 调用),无需加锁。
type fakeCodexSessionCalls struct {
	startRes  string // thread/start 应答原文(缺省 {"threadId":"fake-tid-<n>"})
	resumeErr error  // thread/resume 注入错(模拟重挂载失败)
	startErr  error  // thread/start 注入错(模拟新建失败)
	counter   int    // fake threadId 计数(模拟 codex 每次新建都给新 id)
	calls     []fakeCodexSessionCall
}

func newFakeCodexSessionCalls() *fakeCodexSessionCalls { return &fakeCodexSessionCalls{} }

func (f *fakeCodexSessionCalls) call(method string, params interface{}) (json.RawMessage, error) {
	m, _ := params.(map[string]interface{})
	f.calls = append(f.calls, fakeCodexSessionCall{method: method, params: m})
	switch method {
	case "thread/start":
		if f.startErr != nil {
			return nil, f.startErr
		}
		if f.startRes != "" {
			return json.RawMessage(f.startRes), nil
		}
		f.counter++
		return json.RawMessage(`{"threadId":"fake-tid-` + jsonNumber(f.counter) + `"}`), nil
	case "thread/resume":
		if f.resumeErr != nil {
			return nil, f.resumeErr
		}
		return json.RawMessage(`{"threadId":"resumed"}`), nil
	}
	return nil, nil
}

func jsonNumber(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

// newCodexSessionFixture 适配器 + 假调用源的标准夹具(sessionCall 钩子,等价
// pi 的 sessionFor / dsh 的 newClient 注入点)。
func newCodexSessionFixture() (*CodexProvider, *codexSession, *fakeCodexSessionCalls) {
	p := NewCodexProvider()
	fake := newFakeCodexSessionCalls()
	p.sessionCall = fake.call
	return p, &codexSession{p: p}, fake
}

// ---- Source 同源(D2)----

// TestCodexSessionSourceIsDeclaredShape:Source() 必须与 codexShape.sessionModel
// 是同一个表达式取值 —— 契约套件的 SessionSourceMatchesDescriptor 与 VerifyRuntime
// 的 source-mismatch 检查都建立在这条同源关系上,写字面量就是造第二事实源。
func TestCodexSessionSourceIsDeclaredShape(t *testing.T) {
	_, s, _ := newCodexSessionFixture()
	if got := s.Source(); got != codexShape.sessionModel {
		t.Fatalf("Source()=%q 与 codexShape.sessionModel=%q 不同源", got, codexShape.sessionModel)
	}
	if got := s.Source(); got != runtime.SourceHybrid {
		t.Fatalf("codex 自报 SessionModel=HYBRID,适配器 Source() 实得 %q(设计稿:这一条没有自由度)", got)
	}
}

// ---- Open:thread/start 既有路径写回 CodexThreadID ----

// Open 走 thread/start(唯一实现 startCodexThread,Run 新建分支同一份):cwd 带
// thread.Workspace,引用写回专属字段 CodexThreadID(设计稿:引用在专属字段,
// 不迁 ExternalSession)并登记 threadMap 同代缓存;绝不 resume。
func TestCodexSessionOpenStartsThreadAndWritesReference(t *testing.T) {
	p, s, fake := newCodexSessionFixture()
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`, CodexThreadID: "stale-tid"}
	if err := s.Open(context.Background(), th); err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0].method != "thread/start" {
		t.Fatalf("Open 应恰好一次 thread/start, 实得 %+v", fake.calls)
	}
	if got := fake.calls[0].params["cwd"]; got != `G:\ws` {
		t.Fatalf("thread/start 应带线程工作区, 实得 %v", got)
	}
	if th.CodexThreadID != "fake-tid-1" {
		t.Fatalf("引用应写回专属字段 CodexThreadID, 实得 %q", th.CodexThreadID)
	}
	ref, ok := p.codexThreadRef("t1")
	if !ok || ref.id != "fake-tid-1" || ref.gen != int(p.gen.Load()) {
		t.Fatalf("同代缓存应登记: ref=%+v ok=%t", ref, ok)
	}
}

// 线程级设置快照 → thread/start 带 config 覆盖(与 Run 新建分支同一份逻辑:
// "改了立刻生效"对新建会话成立,verbosity/summary 键集由 ConfigOverrideFor 决定)。
func TestCodexSessionOpenCarriesConfigOverride(t *testing.T) {
	_, s, fake := newCodexSessionFixture()
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		SettingsSnapshot: &store.TurnSettings{Verbosity: "low", ReasoningSummary: "concise"}}
	if err := s.Open(context.Background(), th); err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	cfg, ok := fake.calls[0].params["config"].(map[string]interface{})
	if !ok || cfg["model_verbosity"] != "low" || cfg["model_reasoning_summary"] != "concise" {
		t.Fatalf("thread/start 应携带快照 config 覆盖: %+v", fake.calls[0].params)
	}
}

// threadId 三级回退(threadId → thread.id → id)经适配器同样成立:那是
// startCodexThread 唯一实现的形状,适配器不得自带解析。
func TestCodexSessionOpenParsesThreadIdFallbacks(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"顶层 threadId", `{"threadId":"cx-top"}`, "cx-top"},
		{"thread 对象回退", `{"thread":{"id":"cx-obj"}}`, "cx-obj"},
		{"裸 id 回退", `{"id":"cx-bare"}`, "cx-bare"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, s, fake := newCodexSessionFixture()
			fake.startRes = tc.raw
			th := &store.Thread{ID: "t1", Workspace: `G:\ws`}
			if err := s.Open(context.Background(), th); err != nil {
				t.Fatalf("Open 失败: %v", err)
			}
			if th.CodexThreadID != tc.want {
				t.Fatalf("threadId 回退解析错误: want %q 实得 %q", tc.want, th.CodexThreadID)
			}
			if ref, ok := p.codexThreadRef("t1"); !ok || ref.id != tc.want {
				t.Fatalf("同代缓存应登记解析结果: %+v ok=%t", ref, ok)
			}
		})
	}
}

// Open 失败不得毁掉调用方的落库引用:startCodexThread 在写回之前返回,
// thread.CodexThreadID 原样保留。
func TestCodexSessionOpenFailureKeepsReference(t *testing.T) {
	_, s, fake := newCodexSessionFixture()
	fake.startErr = errors.New("thread/start 被拒")
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`, CodexThreadID: "keep-tid"}
	if err := s.Open(context.Background(), th); err == nil {
		t.Fatal("thread/start 失败应报错")
	}
	if th.CodexThreadID != "keep-tid" {
		t.Fatalf("失败的 Open 应保留旧引用, 实得 %q", th.CodexThreadID)
	}
}

// ---- Resume:按 CodexThreadID 重挂载(整体复用 callThreadResume)----

// 有引用 → thread/resume 打到 CodexThreadID 且带 config 覆盖(护栏逻辑整体复用
// callThreadResume:config 透传语义与 Run 的 resume 路径一字不差);成功后登记
// threadMap 同代缓存。codex 的引用在专属字段,ExternalSession.SessionID 不在
// 认领范围(设计稿"不做清单":不统一 id 命名空间,双字段兼容以专属字段为准)。
func TestCodexSessionResumeReusesCallThreadResume(t *testing.T) {
	p, s, fake := newCodexSessionFixture()
	th := &store.Thread{ID: "t1", CodexThreadID: "tid-old",
		SettingsSnapshot: &store.TurnSettings{Verbosity: "high"}}
	if err := s.Resume(context.Background(), th); err != nil {
		t.Fatalf("重挂载 Resume 失败: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0].method != "thread/resume" {
		t.Fatalf("应恰好 resume 一次, 实得 %+v", fake.calls)
	}
	if got := fake.calls[0].params["threadId"]; got != "tid-old" {
		t.Fatalf("resume 应打到落库的 CodexThreadID, 实得 %v", got)
	}
	cfg, ok := fake.calls[0].params["config"].(map[string]interface{})
	if !ok || cfg["model_verbosity"] != "high" {
		t.Fatalf("resume 应携带 config 覆盖(callThreadResume 透传语义): %+v", fake.calls[0].params)
	}
	ref, ok := p.codexThreadRef("t1")
	if !ok || ref.id != "tid-old" || ref.gen != int(p.gen.Load()) {
		t.Fatalf("重挂载成功应登记同代缓存: ref=%+v ok=%t", ref, ok)
	}
}

// 契约反证的对偶(无引用 Resume 必错):codex 的引用在专属字段 CodexThreadID,
// 为空即 ErrNoNativeSession 同族错误,且**绝不发调用** —— 不为一个没有引用的
// 线程 initialize 拉起 codex 子进程(对齐 piSession/dshSession/zcodeSession 契约)。
func TestCodexSessionResumeWithoutReferenceFailsWithSentinel(t *testing.T) {
	p := NewCodexProvider()
	called := false
	p.sessionCall = func(string, interface{}) (json.RawMessage, error) {
		called = true
		return nil, nil
	}
	s := &codexSession{p: p}
	err := s.Resume(context.Background(), &store.Thread{ID: "stranger"})
	if !errors.Is(err, runtime.ErrNoNativeSession) {
		t.Fatalf("无引用 Resume 应返回 ErrNoNativeSession 同族错误, 实得 %v", err)
	}
	if called {
		t.Fatal("无引用 Resume 不得发调用(不许冷启子进程)")
	}
}

// resume 失败必须原样上抛且**绝不发出 thread/start**:Run 链的"改开新线程"
// 降级是护栏教训的载体(缺护栏 = 用户老会话历史全丢,表现为"AI 失忆"),它是
// 会话级产品决策,留在 Run(D4);适配器降级就会把"恢复失败"伪装成
// "恢复成功后换了新线程"。
func TestCodexSessionResumeFailurePropagatesWithoutNewThread(t *testing.T) {
	_, s, fake := newCodexSessionFixture()
	fake.resumeErr = errors.New("thread/resume 被拒")
	th := &store.Thread{ID: "t1", CodexThreadID: "tid-gone"}
	if err := s.Resume(context.Background(), th); err == nil {
		t.Fatal("resume 失败必须上报,不得假装恢复成功")
	}
	for _, c := range fake.calls {
		if c.method == "thread/start" {
			t.Fatalf("适配器不得擅自新开线程(降级决策留给调用方): %+v", fake.calls)
		}
	}
	if th.CodexThreadID != "tid-gone" {
		t.Fatalf("失败的 Resume 不得清掉调用方的落库引用, 实得 %q", th.CodexThreadID)
	}
}

// ---- Checkpoint / Close:无额外态、无句柄 ----

// Checkpoint 无额外态(引用在 thread.CodexThreadID 上,server 层轮次后落库);
// Close 无句柄可释(codex 会话存活于子进程,threadRef 带 gen 代际失效;真删除
// 走 DeleteNativeSession,不是句柄释放)。
func TestCodexSessionCheckpointAndCloseAreNoop(t *testing.T) {
	_, s, fake := newCodexSessionFixture()
	th := &store.Thread{ID: "t1"}
	if err := s.Checkpoint(context.Background(), th); err != nil {
		t.Fatalf("Checkpoint 应无额外态返回 nil, 实得 %v", err)
	}
	if err := s.Close(context.Background(), th); err != nil {
		t.Fatalf("Close 应无句柄可释返回 nil, 实得 %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("Checkpoint/Close 不得发调用(codex 无对应原生调用): %+v", fake.calls)
	}
}

// ---- VerifyRuntime 正反例(source-mismatch 三重防)----

// 正例:真实 OpenRuntime 交付的 codex 运行时,原生适配器在场且 Source 与申报
// 同源,VerifyRuntime 必须零违约。
func TestVerifyRuntimeAcceptsCodexSessionAdapter(t *testing.T) {
	p := NewCodexProvider()
	p.sessionCall = newFakeCodexSessionCalls().call
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-verify-codex"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("codex 的 OpenRuntime 应交付原生会话适配器(P3 收编后不再是 nil)")
	}
	if _, ok := rt.NativeSession.(*codexSession); !ok {
		t.Fatalf("应注入 codexSession 适配器, 实得 %T", rt.NativeSession)
	}
	if rep := runtime.VerifyRuntime(rt); !rep.OK() {
		for _, v := range rep.Violations {
			t.Errorf("契约违约: %s", v)
		}
	}
}

// 反例:Source 与申报不符的适配器必须被点名 —— 门禁必须能红,否则三重防里的
// VerifyRuntime 这一道只是摆设(pi/dsh/zcode 同款反证)。
func TestVerifyRuntimeRejectsWrongCodexSource(t *testing.T) {
	p := NewCodexProvider()
	p.sessionCall = newFakeCodexSessionCalls().call
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-verify-codex-bad"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	// 谎报成 DAEMON:正是设计稿点名的"塞替身"违约形态。
	rt.NativeSession = wrongSourceCodexSession{&codexSession{p: p}}
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

// wrongSourceCodexSession 是反例专用包装:其余方法借 codexSession,唯独 Source 撒谎。
type wrongSourceCodexSession struct{ *codexSession }

func (wrongSourceCodexSession) Source() runtime.SessionSource { return runtime.SourceDaemon }

// ---- 契约套件两会话用例(P3 验收:从"跳过"变"通过")----

// TestCodexContractSuiteSessionCasesExplicitly 把契约套件的两个会话用例
// (SessionSourceMatchesDescriptor / SessionResumeAfterRestart)对 codex 点名执行:
// Open → Checkpoint → ExportDurable → ImportDurable(模拟重启)→ Resume 按落库
// 引用重挂载 + stranger 无引用必错的反证。单独点名让红灯直接指到会话契约。
func TestCodexContractSuiteSessionCasesExplicitly(t *testing.T) {
	p := NewCodexProvider()
	p.sessionCall = newFakeCodexSessionCalls().call
	th := &store.Thread{ID: "th-contract-codex-explicit"}
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
