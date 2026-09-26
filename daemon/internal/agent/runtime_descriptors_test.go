package agent

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/dshapp"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// 本文件把规格 §15 L1「所有 agent 都必须通过同一套 runtime contract」落到五家
// 原生 provider 上（Echo 的样板见 echo_runtime_test.go）。
//
// 【为什么候选清单是构造调用而不是 []string{"codex","pi",…}】判定能力的只能
// 是 provider 自己的申报（Descriptor）。这份清单只负责"把候选请进来各验一遍"，
// 不承载任何能力语义；写成字符串数组会被 pa_audit 的 roster linter 记成名单债
// （AUDIT §3.3「白名单只许减、不许增」）。
func providerRuntimeCases() []Provider {
	// pi 自 P1 起交付原生会话适配器(见 pi_session.go):契约套件的会话两用例
	// 对它从"跳过"变"必须通过"。会话用例不能拉真 pi 子进程,这里注入假挂载源
	// (与 pi_session_test.go 的 fakeMountClient 同款)—— 每次挂载都是新"代"
	// 客户端,引用接力全靠 thread.ExternalSession,钉的正是"按落库引用重挂载"。
	pi := NewPiProvider()
	pi.sessionFor = newFakePiSessionSource()
	// dsh 自 P2 起同款收编(见 dsh_session.go):注入假客户端(与 dsh_test.go 的
	// fakeDshClient 同一基建),Open 走 session/new、Resume 走 list+resume 重挂载。
	// caps 必须在场:withClientCtx 的握手完成日志会读能力快照(探针夹具=实测形状)。
	dsh := NewDshProvider()
	dsh.newClient = func() (dshClient, error) {
		return &fakeDshClient{
			caps:     probeCaps(),
			newState: &dshapp.SessionState{SessionID: "s-contract"},
			list:     []dshapp.SessionListEntry{{SessionID: "s-contract"}},
			resumeBy: map[string]*dshapp.SessionState{
				"s-contract": {SessionID: "s-contract"},
			},
		}, nil
	}
	// zcode 自 P3 起同款收编(见 zcode_session.go):注入假客户端源(sessionSource
	// 钩子,等价 pi 的 sessionFor / dsh 的 newClient),Open 走 session/create、
	// Resume 走 session/resume 重挂载,不拉真子进程。
	zcode := NewZcodeProvider()
	zcode.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		return newFakeZcodeSessionClient("sess-contract"), nil
	}
	// codex 自 P3 起同款收编(见 codex_session.go):注入假 JSON-RPC 调用源
	// (sessionCall 钩子,与 callThreadResume 的 call 形参同一注入基建),
	// thread/start + thread/resume 全走假调用。
	codex := NewCodexProvider()
	codex.sessionCall = newFakeCodexSessionCalls().call
	return []Provider{
		codex,
		pi,
		zcode,
		dsh,
		// 无稳定原生 session 的那一类：daemon 侧 conversation memory 必须顶上
		// （规格 §7 对 OpenAI Compatible 的要求）。
		NewOpenAICompatible("openai-compat", "OpenAI 兼容", "http://127.0.0.1:1/v1", "unused-key", nil),
	}
}

// TestNativeProvidersDeclareRuntimeShape 是**声明层**核验：每家都必须实现
// runtime.Participant，且四个必需入口每一项都有交代（实现或显式不支持）。
func TestNativeProvidersDeclareRuntimeShape(t *testing.T) {
	for _, p := range providerRuntimeCases() {
		t.Run(p.ID(), func(t *testing.T) {
			rep := runtime.VerifyDeclaration(p.ID(), p)
			if !rep.Declared {
				t.Fatalf("%s 未实现 runtime.Participant（规格 §13 P0.5 要求所有 agent 都申报运行时形态）", p.ID())
			}
			if !rep.OK() {
				for _, v := range rep.Violations {
					t.Errorf("契约违约: %s", v)
				}
			}
			if rep.Descriptor.SessionModel == "" {
				t.Errorf("%s 未申报 SessionModel（会话事实在谁手里必须有答案）", p.ID())
			}
		})
	}
}

// TestNativeProvidersDeliverRuntime 是**实现层**核验：声明说支持的，OpenRuntime
// 必须真的交出来，且交付物要过同一套契约用例。
//
// 【为什么串行、不加 t.Parallel】契约套件的"模拟重启"分支会调
// Backend.ImportDurable，那会 reset 整个共享后端。并行跑会让家与家互相清空，
// 断言退化成掷骰子。
func TestNativeProvidersDeliverRuntime(t *testing.T) {
	for _, p := range providerRuntimeCases() {
		t.Run(p.ID(), func(t *testing.T) {
			participant, ok := p.(runtime.Participant)
			if !ok {
				t.Fatalf("%s 未实现 runtime.Participant", p.ID())
			}
			th := &store.Thread{ID: "th-contract-" + p.ID()}
			rt, err := participant.OpenRuntime(context.Background(), th)
			if err != nil {
				t.Fatalf("OpenRuntime 失败: %v", err)
			}
			if rt == nil {
				t.Fatal("OpenRuntime 返回 nil 运行时（声明支持却交不出来）")
			}
			if rep := runtime.VerifyRuntime(rt); !rep.OK() {
				for _, v := range rep.Violations {
					t.Errorf("实现层契约违约: %s", v)
				}
			}
			runtime.ContractSuite{
				T:       t,
				AgentID: p.ID(),
				RT:      rt,
				Backend: sharedRuntimeBackend(),
				Thread:  th,
			}.RunAll()
		})
	}
}

// TestNativeProvidersRejectNilThread：缺 thread 必须 fail closed —— 交付一个
// "无主运行时"会让调用方往不存在的会话上写记忆，比直接报错难查得多
// （与 Echo 同款约定，见 echo_runtime_test.go）。
func TestNativeProvidersRejectNilThread(t *testing.T) {
	for _, p := range providerRuntimeCases() {
		t.Run(p.ID(), func(t *testing.T) {
			participant, ok := p.(runtime.Participant)
			if !ok {
				t.Fatalf("%s 未实现 runtime.Participant", p.ID())
			}
			rt, err := participant.OpenRuntime(context.Background(), nil)
			if err == nil {
				t.Fatalf("缺 thread 应报错，实得 rt=%+v", rt)
			}
			if rt != nil {
				t.Fatal("报错时不得同时返回运行时（调用方会以为交付成功了）")
			}
		})
	}
}

// TestNativeProvidersHonorDeclaredSessionSource 钉住"会话归属不能撒谎"：
//
//	原生会话适配器若尚未交付，就必须是 nil（未交付）；
//	绝不允许用一个 Source=DAEMON 的替身去顶替声明为 HYBRID/NATIVE 的家。
//
// 后者会让恢复路径按错误前提决策（daemon 以为能靠原生 session 接回来，实际那家
// 根本没有），且被 VerifyRuntime 判为 source-mismatch。
func TestNativeProvidersHonorDeclaredSessionSource(t *testing.T) {
	for _, p := range providerRuntimeCases() {
		t.Run(p.ID(), func(t *testing.T) {
			participant, ok := p.(runtime.Participant)
			if !ok {
				t.Fatalf("%s 未实现 runtime.Participant", p.ID())
			}
			desc := participant.Descriptor()
			th := &store.Thread{ID: "th-source-" + p.ID()}
			rt, err := participant.OpenRuntime(context.Background(), th)
			if err != nil {
				t.Fatalf("OpenRuntime 失败: %v", err)
			}
			if rt.NativeSession == nil {
				return // 未交付是允许的：四个必需入口不含 NativeSession
			}
			if got := rt.NativeSession.Source(); got != desc.SessionModel {
				t.Fatalf("NativeSession.Source()=%q 与 descriptor.SessionModel=%q 不一致", got, desc.SessionModel)
			}
		})
	}
}

// TestPiDeliversNativeSessionAfterAdoption 钉 P1 收编的注入事实:pi 实现了
// NativeSessionFactory(D5),openSharedRuntime 必须注入其原生适配器 ——
// 不再是 nil。它是 TestNativeProvidersHonorDeclaredSessionSource 对 nil 的
// "早退放行"的补集:那条允许任何家 nil,这条只认 pi 必须非 nil。
func TestPiDeliversNativeSessionAfterAdoption(t *testing.T) {
	p := NewPiProvider()
	injectFakeSessionSource(p)
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-pi-native"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("pi 收编后 NativeSession 必须非 nil(设计稿 P1:不再是 nil)")
	}
	if _, ok := rt.NativeSession.(*piSession); !ok {
		t.Fatalf("应注入 piSession 适配器, 实得 %T", rt.NativeSession)
	}
	if got := rt.NativeSession.Source(); got != p.Descriptor().SessionModel {
		t.Fatalf("NativeSession.Source()=%q 与 descriptor.SessionModel=%q 不一致", got, p.Descriptor().SessionModel)
	}
}

// TestDshDeliversNativeSessionAfterAdoption 钉 P2 收编的注入事实:dsh 实现了
// NativeSessionFactory(D5),openSharedRuntime 必须注入其原生适配器 —— 不再是
// nil。与 TestPiDeliversNativeSessionAfterAdoption 同款:对"早退放行"的补集
// 只认 dsh 必须非 nil,且交付物就是 dshSession。
func TestDshDeliversNativeSessionAfterAdoption(t *testing.T) {
	p := NewDshProvider()
	p.newClient = func() (dshClient, error) {
		return &fakeDshClient{newState: &dshapp.SessionState{SessionID: "s-native"}}, nil
	}
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-dsh-native"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("dsh 收编后 NativeSession 必须非 nil(设计稿 P2:不再是 nil)")
	}
	if _, ok := rt.NativeSession.(*dshSession); !ok {
		t.Fatalf("应注入 dshSession 适配器, 实得 %T", rt.NativeSession)
	}
	if got := rt.NativeSession.Source(); got != p.Descriptor().SessionModel {
		t.Fatalf("NativeSession.Source()=%q 与 descriptor.SessionModel=%q 不一致", got, p.Descriptor().SessionModel)
	}
}

// TestZcodeDeliversNativeSessionAfterAdoption 钉 P3 收编的注入事实:zcode 实现
// 了 NativeSessionFactory(D5),openSharedRuntime 必须注入其原生适配器 —— 不再
// 是 nil。与 pi/dsh 同款:对"早退放行"的补集只认 zcode 必须非 nil,且交付物
// 就是 zcodeSession。
func TestZcodeDeliversNativeSessionAfterAdoption(t *testing.T) {
	p := NewZcodeProvider()
	p.sessionSource = func(context.Context) (zcodeSessionClient, error) {
		return newFakeZcodeSessionClient("sess-native-zcode"), nil
	}
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-zcode-native"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("zcode 收编后 NativeSession 必须非 nil(设计稿 P3:不再是 nil)")
	}
	if _, ok := rt.NativeSession.(*zcodeSession); !ok {
		t.Fatalf("应注入 zcodeSession 适配器, 实得 %T", rt.NativeSession)
	}
	if got := rt.NativeSession.Source(); got != p.Descriptor().SessionModel {
		t.Fatalf("NativeSession.Source()=%q 与 descriptor.SessionModel=%q 不一致", got, p.Descriptor().SessionModel)
	}
}

// TestCodexDeliversNativeSessionAfterAdoption 钉 P3 收编的注入事实:codex 实现
// 了 NativeSessionFactory(D5),openSharedRuntime 必须注入其原生适配器 —— 不再
// 是 nil。与 pi/dsh/zcode 同款:对"早退放行"的补集只认 codex 必须非 nil,且
// 交付物就是 codexSession。
func TestCodexDeliversNativeSessionAfterAdoption(t *testing.T) {
	p := NewCodexProvider()
	p.sessionCall = newFakeCodexSessionCalls().call
	rt, err := p.OpenRuntime(context.Background(), &store.Thread{ID: "th-codex-native"})
	if err != nil {
		t.Fatalf("OpenRuntime 失败: %v", err)
	}
	if rt.NativeSession == nil {
		t.Fatal("codex 收编后 NativeSession 必须非 nil(设计稿 P3:不再是 nil)")
	}
	if _, ok := rt.NativeSession.(*codexSession); !ok {
		t.Fatalf("应注入 codexSession 适配器, 实得 %T", rt.NativeSession)
	}
	if got := rt.NativeSession.Source(); got != p.Descriptor().SessionModel {
		t.Fatalf("NativeSession.Source()=%q 与 descriptor.SessionModel=%q 不一致", got, p.Descriptor().SessionModel)
	}
}

// TestProvidersWithoutFactoryKeepNativeSessionNil 钉 D5 的另一半:未实现
// NativeSessionFactory 的家维持 nil —— 六家增量收编互不牵连,任何一家的收编
// 都不得顺手给别家塞替身(替身会被 VerifyRuntime 判 source-mismatch)。
// 【P3 后现状】HYBRID 四家(codex/pi/zcode/dsh)已全部收编,SourceDaemon 的家
// (OpenAI 兼容/echo)本就被上面的早退放行豁免 —— 本条当前零实例,但作为
// 门禁保留:未来新家接入若声明 HYBRID 却未实现工厂,仍被此钉住。
func TestProvidersWithoutFactoryKeepNativeSessionNil(t *testing.T) {
	for _, p := range providerRuntimeCases() {
		participant, ok := p.(runtime.Participant)
		if !ok {
			t.Fatalf("%s 未实现 runtime.Participant", p.ID())
		}
		// daemon 侧会话的家(OpenAI 兼容)不在 D5 的 nil 面:memSession
		// (Source=DAEMON)本就是它的诚实交付,与 FullRuntime 预置一致。
		if participant.Descriptor().SessionModel == runtime.SourceDaemon {
			continue
		}
		if _, ok := p.(NativeSessionFactory); ok {
			continue // 已收编的家(pi/dsh/zcode/codex)不在本条断言范围
		}
		t.Run(p.ID(), func(t *testing.T) {
			rt, err := participant.OpenRuntime(context.Background(), &store.Thread{ID: "th-nil-" + p.ID()})
			if err != nil {
				t.Fatalf("OpenRuntime 失败: %v", err)
			}
			if rt.NativeSession != nil {
				t.Fatalf("%s 未实现工厂,NativeSession 应保持 nil(诚实表达未交付), 实得 %T", p.ID(), rt.NativeSession)
			}
		})
	}
}
