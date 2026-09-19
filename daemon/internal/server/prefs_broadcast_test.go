// prefs_broadcast_test.go —— 七项设置"广播到全部 PrefsAware provider"的单测。
//
// 【背景】SetCodexSettings / server.New 曾只把偏好灌给 codex,pi/zcode 完全不
// 消费(用户要求"Pi 和 ZCode 同理")。这里钉死:
//  1. 保存(SetCodexSettings)时,实现了 agent.PrefsAware 的 provider(不论是谁)
//     都收到**同一份合并后的完整快照**;codex 自己也经广播更新内存态;
//  2. 未实现 PrefsAware 的 provider(echo/OpenAI 兼容)不参与广播(类型断言不命中);
//  3. 启动灌入(New)把落盘偏好灌给全部 PrefsAware provider(重启不丢);
//  4. zcode 的 TitleHandler 在 New 里接线,标题变化落库(与 codex 同语义)。
package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- 假 provider ----

// recordingPrefs 实现了 agent.PrefsAware 的假 provider:记录每次收到的快照。
type recordingPrefs struct {
	id  string
	mu  sync.Mutex
	got []agent.DesktopPrefs
}

func (f *recordingPrefs) ID() string             { return f.id }
func (f *recordingPrefs) DisplayName() string    { return "fake-" + f.id }
func (f *recordingPrefs) Models() []pb.ModelInfo { return nil }
func (f *recordingPrefs) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}
func (f *recordingPrefs) ApplyPrefs(p agent.DesktopPrefs) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, p)
}
func (f *recordingPrefs) snapshots() []agent.DesktopPrefs {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]agent.DesktopPrefs(nil), f.got...)
}

// 编译期断言:fake 确实实现了可选接口。
var _ agent.PrefsAware = (*recordingPrefs)(nil)

// plainProvider 故意不实现 PrefsAware 的对照组(echo/OpenAI 兼容的替身)。
type plainProvider struct{ id string }

func (f *plainProvider) ID() string             { return f.id }
func (f *plainProvider) DisplayName() string    { return "plain-" + f.id }
func (f *plainProvider) Models() []pb.ModelInfo { return nil }
func (f *plainProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, nil
}

// newPrefsTestServer 造一个带真实(临时)FileStore 的 Server:codex 用真 provider
// (探测替换成假实现,保存路径有界返回),其余 provider 由调用方注入。
func newPrefsTestServer(t *testing.T, extra map[string]agent.Provider) (*Server, *agent.CodexProvider, *store.FileStore) {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	provs := map[string]agent.Provider{"codex": &agent.CodexProvider{}}
	for k, v := range extra {
		provs[k] = v
	}
	srv := New(Config{Store: st, Providers: provs})
	srv.probeOverride = errorProbe{agent.ErrBusy}
	return srv, provs["codex"].(*agent.CodexProvider), st
}

// ---- 1) 保存路径:SetCodexSettings 广播 ----

func TestSetCodexSettingsBroadcastsToAllPrefsAware(t *testing.T) {
	pi := &recordingPrefs{id: "pi"}
	zc := &recordingPrefs{id: "zcode"}
	echo := &plainProvider{id: "echo"}
	srv, codex, _ := newPrefsTestServer(t, map[string]agent.Provider{"pi": pi, "zcode": zc, "echo": echo})

	// 短 ctx:探测被假实现挡下,快照有界返回(同 codex_block_test 的手法)。
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	hide := true
	effort := "high"
	if _, err := srv.SetCodexSettings(ctx, &pb.SetCodexSettingsRequest{
		HideAgentReasoning: &hide,
		Effort:             &effort,
	}); err != nil {
		t.Fatalf("保存不应失败: %v", err)
	}

	// store 初始为空 → 合并后快照:effort/hide 来自本次请求,其余为零值。
	want := agent.DesktopPrefs{HideReasoning: true}
	for name, f := range map[string]*recordingPrefs{"pi": pi, "zcode": zc} {
		got := f.snapshots()
		// 第 1 次 = New() 的启动灌入(store 初始为空 → 全零值快照),
		// 第 2 次 = SetCodexSettings 的广播 —— 两条路径都必须到。
		if len(got) != 2 {
			t.Fatalf("[%s] 应收到 启动灌入+保存广播 共 2 次, got %d 次", name, len(got))
		}
		if got[0] != (agent.DesktopPrefs{}) {
			t.Fatalf("[%s] 第 1 次应为启动灌入的零值快照: %+v", name, got[0])
		}
		if got[1] != want {
			t.Fatalf("[%s] 保存广播快照不符:\n got=%+v\nwant=%+v", name, got[1], want)
		}
	}
	// codex 自己也经广播更新了内存态(Prefs() 读回验证)。
	if p := codex.Prefs(); !p.HideReasoning || p.Effort != "high" {
		t.Fatalf("codex 内存态未更新: %+v", p)
	}
	// 未实现 PrefsAware 的 provider:类型断言不命中,不在广播之列。
	if _, ok := agent.Provider(echo).(agent.PrefsAware); ok {
		t.Fatal("plainProvider 不应实现 PrefsAware")
	}
}

// ---- 2) 启动灌入:落盘偏好不因重启丢失 ----

func TestNewInjectsPersistedPrefsToAllPrefsAware(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	// 预置落盘偏好(模拟上一轮保存过的状态)。
	if err := st.SetCodexPrefs(store.CodexPrefs{HideReasoning: true, Verbosity: "low"}); err != nil {
		t.Fatalf("落盘偏好失败: %v", err)
	}
	if err := st.SetDefaultModelID("pi/zai-coding-cn/glm-5.3"); err != nil {
		t.Fatalf("落盘默认模型失败: %v", err)
	}
	if err := st.SetDefaultEffort("medium"); err != nil {
		t.Fatalf("落盘默认强度失败: %v", err)
	}

	pi := &recordingPrefs{id: "pi"}
	zc := &recordingPrefs{id: "zcode"}
	echo := &plainProvider{id: "echo"}
	_ = New(Config{Store: st, Providers: map[string]agent.Provider{"pi": pi, "zcode": zc, "echo": echo}})

	// 启动灌入必须与 DesktopPrefsFromStore 同源:六个落盘偏好 + 全局默认 model/effort。
	want := agent.DesktopPrefs{
		Model:         "pi/zai-coding-cn/glm-5.3",
		Effort:        "medium",
		HideReasoning: true,
		Verbosity:     "low",
	}
	if got := agent.DesktopPrefsFromStore(st); got != want {
		t.Fatalf("DesktopPrefsFromStore 与预期不符: got=%+v want=%+v", got, want)
	}
	for _, f := range []*recordingPrefs{pi, zc} {
		got := f.snapshots()
		if len(got) != 1 {
			t.Fatalf("[%s] 启动应恰好灌入 1 次, got %d 次", f.id, len(got))
		}
		scoped := want
		scoped.Model = ""
		scoped.Effort = ""
		if got[0] != scoped {
			t.Fatalf("[%s] 启动灌入快照不符:\n got=%+v\nwant=%+v", f.id, got[0], want)
		}
	}
}

// ---- 3) zcode 标题同步接线 ----

func TestNewWiresZcodeTitleHandler(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	if err := st.Create(&store.Thread{ID: "t1", Title: "旧标题"}); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	zp := agent.NewZcodeProvider()
	_ = New(Config{Store: st, Providers: map[string]agent.Provider{"zcode": zp}})
	if zp.TitleHandler == nil {
		t.Fatal("zcode TitleHandler 未在 New 里接线")
	}
	// 模拟 zcode 的 session.titleUpdated 回调(带的是我们的线程 ID)。
	zp.TitleHandler("t1", "自动起的新标题")
	got, ok := st.Get("t1")
	if !ok || got.Title != "自动起的新标题" {
		t.Fatalf("标题未落库: ok=%v title=%q", ok, got.Title)
	}
	// 不存在的线程 ID:静默忽略,不 panic。
	zp.TitleHandler("不存在", "x")
}
