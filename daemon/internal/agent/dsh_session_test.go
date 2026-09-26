// dsh_session_test.go —— DSH 原生会话适配器(P2 收编)的单测与契约正反例。
//
// 【打桩方式】复用 dsh_test.go 的 fakeDshClient(dshClient 依赖面打桩,dshapp 的
// 假子进程端到端在 internal/dshapp/client_test.go):钉的是 dsh.go 的 remountSession
// (能力门控 + list 双因子校验 + resume + 降级位)与 createSession(session/new 登记)
// 这**唯一实现**经适配器的接线形状,不是第二实现的形状。
//
// 【D4 语义钉】Resume 的失败类型区分是本适配器的核心契约:
//   - 数据错位(list 不在列/能力缺席)→ dshRemountError.downgradeable=false,硬拒绝;
//   - 升级断裂(在列 + resume -32602)→ downgradeable=true 的错误**原样上抛**,
//     适配器绝不吞、绝不静默重建 —— 重建决策(rebuildSessionAfterUpgrade)留在
//     Run 的降级链里(设计稿 §4-P2:"升级重建路径留在 Run,适配器不吞")。
package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/dshapp"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// newFixture 适配器 + 假客户端的标准夹具(与 newTestDshProvider 同一注入点:
// newClient 即 dsh 的"挂载客户端源",等价 pi 的 sessionFor 钩子)。
func newFixture(t *testing.T, fake *fakeDshClient) (*DshProvider, *dshSession) {
	t.Helper()
	p := newTestDshProvider(t, fake)
	return p, &dshSession{p: p}
}

// ---- Source 同源(D2)----

// TestDshSessionSourceIsDeclaredShape:Source() 必须与 dshShape.sessionModel 是
// 同一个表达式取值 —— VerifyRuntime 的 source-mismatch 检查建立在这条同源关系上,
// 写字面量就会造出"声明与 Source() 两个独立来源"。
func TestDshSessionSourceIsDeclaredShape(t *testing.T) {
	_, s := newFixture(t, &fakeDshClient{})
	if got := s.Source(); got != dshShape.sessionModel {
		t.Fatalf("Source()=%q 与 dshShape.sessionModel=%q 不同源", got, dshShape.sessionModel)
	}
	if got := s.Source(); got != runtime.SourceHybrid {
		t.Fatalf("dsh 自报 SessionModel=HYBRID,适配器 Source() 实得 %q(设计稿:这一条没有自由度)", got)
	}
}

// ---- Open:session/new 既有路径回填引用 ----

// Open 走 createSession(session/new):写回 ExternalSession + 登记 threadMap 同代
// 缓存;绝不 resume(那是 Resume 的事)、绝不 close。
func TestDshSessionOpenCreatesAndBackfillsReference(t *testing.T) {
	fake := &fakeDshClient{newState: &dshapp.SessionState{SessionID: "s-open-1"}}
	p, s := newFixture(t, fake)
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`}
	if err := s.Open(context.Background(), th); err != nil {
		t.Fatalf("Open 失败: %v", err)
	}
	if th.ExternalSession.SessionID != "s-open-1" || th.ExternalSession.Cwd != `G:\ws` {
		t.Fatalf("引用应由 session/new 路径写回: %+v", th.ExternalSession)
	}
	ref, ok := p.dshThreadRefFor("t1")
	if !ok || ref.id != "s-open-1" {
		t.Fatalf("同代缓存应登记: ref=%+v ok=%t", ref, ok)
	}
	snap := fake.snapshot()
	if len(snap.resumeHits) != 0 || len(snap.closeHits) != 0 {
		t.Fatalf("新建路径不得 resume/close: %+v", snap)
	}
}

// Open 失败不得毁掉调用方的落库引用:createSession 在 session/new 失败时先返回,
// 不碰 thread.ExternalSession(引用与子进程实际挂载态保持一致)。
func TestDshSessionOpenFailureKeepsReference(t *testing.T) {
	fake := &fakeDshClient{newErr: errors.New("session/new 被拒")}
	_, s := newFixture(t, fake)
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "keep", Cwd: `G:\old`}}
	if err := s.Open(context.Background(), th); err == nil {
		t.Fatal("session/new 失败应报错")
	}
	if th.ExternalSession.SessionID != "keep" || th.ExternalSession.Cwd != `G:\old` {
		t.Fatalf("失败的 Open 应保留旧引用, 实得 %+v", th.ExternalSession)
	}
}

// ---- Resume:按落库引用重挂载(复用 remountSession,零握手第二实现)----

// 在列(sessionId+cwd 双因子)才 resume;成功后与 ensureSession 同款登记:
// threadMap 同代缓存 + resume 应答目录刷新(探针修正②)。
func TestDshSessionResumeRemountsViaListAndResume(t *testing.T) {
	resumedCatalog, err := dshapp.ParseConfigOptions(dshapp.ProbeConfigOptionsJSON)
	if err != nil {
		t.Fatalf("探针夹具应可解析: %v", err)
	}
	resumedCatalog[0].CurrentValue = `["deepseek-official","deepseek-flash"]`
	fake := &fakeDshClient{
		list: []dshapp.SessionListEntry{{SessionID: "s-old", Cwd: `G:\ws`}},
		resumeBy: map[string]*dshapp.SessionState{
			"s-old": {SessionID: "s-old", ConfigOptions: resumedCatalog},
		},
	}
	p, s := newFixture(t, fake)
	th := &store.Thread{ID: "t1", Workspace: `G:\ws`,
		ExternalSession: store.ExternalSessionRef{SessionID: "s-old", Cwd: `G:\ws`}}
	if err := s.Resume(context.Background(), th); err != nil {
		t.Fatalf("重挂载 Resume 失败: %v", err)
	}
	snap := fake.snapshot()
	if len(snap.resumeHits) != 1 || snap.resumeHits[0] != "s-old" {
		t.Fatalf("应恰好 resume 一次 s-old: %v", snap.resumeHits)
	}
	if len(snap.newCalls) != 0 {
		t.Fatalf("重挂载路径不得新开会话: %v", snap.newCalls)
	}
	ref, ok := p.dshThreadRefFor("t1")
	if !ok || ref.id != "s-old" {
		t.Fatalf("重挂载成功应登记同代缓存: %+v ok=%t", ref, ok)
	}
	// 探针修正②:resume 应答的 configOptions 已刷新进目录(模型切换取编码串的唯一来源)。
	if v, ok := p.encodedModelValue("deepseek-flash"); !ok || v != `["deepseek-official","deepseek-flash"]` {
		t.Fatalf("resume 后目录应刷新: %q %v", v, ok)
	}
}

// 契约反证的对偶:无引用 Resume 必须报 ErrNoNativeSession 同族错,且**不得启动
// 子进程** —— 不为一个没有引用的线程拉起 dsh(对齐 piSession/memSession 契约)。
func TestDshSessionResumeWithoutReferenceFailsWithSentinel(t *testing.T) {
	fake := &fakeDshClient{}
	_, s := newFixture(t, fake)
	err := s.Resume(context.Background(), &store.Thread{ID: "stranger"})
	if !errors.Is(err, runtime.ErrNoNativeSession) {
		t.Fatalf("无引用 Resume 应返回 ErrNoNativeSession 同族错误, 实得 %v", err)
	}
	if fake.initCalls != 0 {
		t.Fatal("无引用 Resume 不得取客户端(不许冷启子进程)")
	}
}

// 数据错位(list 不在列)→ 硬拒绝:downgradeable=false,绝不静默新开(蓝本 §4.3;
// 探针修正④)。这正是"数据错位拒绝 vs 升级降级重建"的第一类。
func TestDshSessionResumeNotListedIsHardReject(t *testing.T) {
	fake := &fakeDshClient{
		newState: &dshapp.SessionState{SessionID: "should-not-be-created"},
		list:     []dshapp.SessionListEntry{{SessionID: "other", Cwd: `G:\ws`}},
	}
	_, s := newFixture(t, fake)
	th := &store.Thread{ID: "t1",
		ExternalSession: store.ExternalSessionRef{SessionID: "s-gone", Cwd: `G:\ws`}}
	err := s.Resume(context.Background(), th)
	if err == nil {
		t.Fatal("不在列必须显式报错")
	}
	var re *dshRemountError
	if !errors.As(err, &re) {
		t.Fatalf("错误应保留 dshRemountError 分类载体, 实得 %v", err)
	}
	if re.downgradeable {
		t.Fatal("数据错位(list 不在列)不得标记 downgradeable —— 静默重建会掩盖错位")
	}
	if !strings.Contains(err.Error(), "不静默新开会话") {
		t.Fatalf("错误原文应如实说明拒绝原因: %v", err)
	}
	if n := len(fake.snapshot().newCalls); n != 0 {
		t.Fatalf("适配器不得静默新开会话: new=%d", n)
	}
}

// 升级断裂(在列 + resume -32602)→ 错误保留 downgradeable=true 原样上抛:
// 重建决策(rebuildSessionAfterUpgrade)留给 Run 的降级链(D4),适配器不吞。
func TestDshSessionResumeUpgradeBreakKeepsDowngradeableBit(t *testing.T) {
	fake := &fakeDshClient{
		list:      []dshapp.SessionListEntry{{SessionID: "s-old", Cwd: `G:\ws`}},
		resumeErr: &dshapp.RPCError{Method: "session/resume", Code: -32602, Message: "Invalid params"},
	}
	_, s := newFixture(t, fake)
	th := &store.Thread{ID: "t1",
		ExternalSession: store.ExternalSessionRef{SessionID: "s-old", Cwd: `G:\ws`}}
	err := s.Resume(context.Background(), th)
	if err == nil {
		t.Fatal("resume 被拒必须显式报错")
	}
	var re *dshRemountError
	if !errors.As(err, &re) {
		t.Fatalf("错误应保留 dshRemountError 分类载体, 实得 %v", err)
	}
	if !re.downgradeable {
		t.Fatal("升级断裂(在列 + -32602)必须保留 downgradeable 位供调用方决策")
	}
	if n := len(fake.snapshot().newCalls); n != 0 {
		t.Fatalf("适配器不得擅自重建(降级决策留给调用方): new=%d", n)
	}
}

// 能力门控:sessionCapabilities.resume 不在场的版本必须显式报错而非静默新建
// (设计稿 §4-P2 风险项;§5.2 一切扩展方法的使用以 initialize 应答为准)。
func TestDshSessionResumeCapabilityAbsentFailsExplicitly(t *testing.T) {
	fake := &fakeDshClient{caps: &dshapp.InitializeResult{}}
	_, s := newFixture(t, fake)
	th := &store.Thread{ID: "t1",
		ExternalSession: store.ExternalSessionRef{SessionID: "s-old", Cwd: `G:\ws`}}
	err := s.Resume(context.Background(), th)
	if err == nil || !strings.Contains(err.Error(), "不支持会话恢复") {
		t.Fatalf("能力缺席应显式报错, 实得 %v", err)
	}
	var re *dshRemountError
	if !errors.As(err, &re) || re.downgradeable {
		t.Fatalf("能力缺席是硬拒绝(非升级断裂), 实得 %+v", err)
	}
	snap := fake.snapshot()
	if len(snap.resumeHits) != 0 || len(snap.newCalls) != 0 {
		t.Fatalf("能力缺席不得发出 resume/new: %+v", snap)
	}
}

// ---- Checkpoint / Close:无额外态、无句柄 ----

// Checkpoint 无额外态(引用已写回 thread,落库归 server 层);Close 无句柄可释
// (dsh 的 session/close 是"销毁"语义,归删除路径 DeleteDshSession,不是句柄释放)。
func TestDshSessionCheckpointAndCloseAreNoop(t *testing.T) {
	fake := &fakeDshClient{}
	_, s := newFixture(t, fake)
	th := &store.Thread{ID: "t1"}
	if err := s.Checkpoint(context.Background(), th); err != nil {
		t.Fatalf("Checkpoint 应无额外态返回 nil, 实得 %v", err)
	}
	if err := s.Close(context.Background(), th); err != nil {
		t.Fatalf("Close 应无句柄可释返回 nil, 实得 %v", err)
	}
	if fake.initCalls != 0 {
		t.Fatal("Checkpoint/Close 不得取客户端(dsh 无对应原生调用)")
	}
}

// ---- DeleteDshSession:capability 门控的 session/close ----

// close 在场(探针第 1 项实测 sessionCapabilities 含 close)→ 发出 session/close。
func TestDshDeleteSessionIssuesSessionClose(t *testing.T) {
	fake := &fakeDshClient{}
	p, _ := newFixture(t, fake)
	if err := p.DeleteDshSession("s-del"); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}
	snap := fake.snapshot()
	if len(snap.closeHits) != 1 || snap.closeHits[0] != "s-del" {
		t.Fatalf("session/close 应对目标会话发出且只发出一次: %v", snap.closeHits)
	}
}

// close 能力缺席 → 显式报错且**不发调用**(§5.2:不写死协议假设,不猜)。
func TestDshDeleteSessionCapabilityAbsentFailsExplicitly(t *testing.T) {
	fake := &fakeDshClient{caps: &dshapp.InitializeResult{}}
	p, _ := newFixture(t, fake)
	err := p.DeleteDshSession("s-del")
	if err == nil || !strings.Contains(err.Error(), "close 不在场") {
		t.Fatalf("close 缺席应显式报错, 实得 %v", err)
	}
	if n := len(fake.snapshot().closeHits); n != 0 {
		t.Fatalf("能力缺席不得发出 session/close: %d", n)
	}
}

// close 失败 → 错误原样上抛,由 server 层决定"只留日志不阻塞本地删除"。
func TestDshDeleteSessionFailurePropagates(t *testing.T) {
	fake := &fakeDshClient{closeErr: errors.New("session/close 被拒")}
	p, _ := newFixture(t, fake)
	if err := p.DeleteDshSession("s-del"); err == nil {
		t.Fatal("close 失败应上抛,不得假装删除成功")
	}
}

// ---- VerifyRuntime 正反例(source-mismatch 三重防)----

// 正例:真实 OpenRuntime 交付的 dsh 运行时,原生适配器在场且 Source 与申报同源,
// VerifyRuntime 必须零违约。
func TestVerifyRuntimeAcceptsDshSessionAdapter(t *testing.T) {
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) {
		return &fakeDshClient{newState: &dshapp.SessionState{SessionID: "s-verify"}}, nil
	}
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-verify-dsh"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("dsh 的 OpenRuntime 应交付原生会话适配器(P2 收编后不再是 nil)")
	}
	if rep := runtime.VerifyRuntime(rt); !rep.OK() {
		for _, v := range rep.Violations {
			t.Errorf("契约违约: %s", v)
		}
	}
}

// 反证:Source 与申报不符的适配器必须被点名 —— 门禁必须能红,否则 VerifyRuntime
// 这一道只是摆设(pi_session_test.go 同款反证)。
func TestVerifyRuntimeRejectsWrongDshSource(t *testing.T) {
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) {
		return &fakeDshClient{newState: &dshapp.SessionState{SessionID: "s-verify-bad"}}, nil
	}
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-verify-dsh-bad"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	rt.NativeSession = wrongSourceDshSession{&dshSession{p: p}}
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

// wrongSourceDshSession 是反证专用包装:其余方法借 dshSession,唯独 Source 撒谎。
type wrongSourceDshSession struct{ *dshSession }

func (wrongSourceDshSession) Source() runtime.SessionSource { return runtime.SourceDaemon }

// ---- 契约套件两会话用例(P2 验收:从"跳过"变"通过")----

// TestDshContractSuiteSessionCasesExplicitly 把契约套件的两个会话用例
// (SessionSourceMatchesDescriptor / SessionResumeAfterRestart)对 dsh 点名执行:
// Open → Checkpoint → ExportDurable → ImportDurable(模拟重启)→ Resume 按落库
// 引用重挂载 + stranger 无引用必错的反证。单独点名让红灯直接指到会话契约。
func TestDshContractSuiteSessionCasesExplicitly(t *testing.T) {
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) {
		// caps 必须在场:withClientCtx 的握手完成日志会读能力快照(生产由真
		// 客户端保证,探针夹具即实测形状)。
		return &fakeDshClient{
			caps:     probeCaps(),
			newState: &dshapp.SessionState{SessionID: "s-contract"},
			list:     []dshapp.SessionListEntry{{SessionID: "s-contract"}},
			resumeBy: map[string]*dshapp.SessionState{
				"s-contract": {SessionID: "s-contract"},
			},
		}, nil
	}
	th := &store.Thread{ID: "th-contract-dsh-explicit"}
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
