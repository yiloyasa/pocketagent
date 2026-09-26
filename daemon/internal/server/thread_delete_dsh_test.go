// thread_delete_dsh_test.go —— P2 收编 DSH 的删除分支补齐 + 删除接口化的回归钉。
//
// 【钉什么】设计稿 §6.2 审计发现:DeleteThread 的原生删除 switch 缺 dsh 分支,
// dsh 删线程只删本地(sessionCapabilities.close 已由 P0 探针实测在场,却没人调)。
// 修复走 D3 的「零名单」形态:server 侧定义可选接口 nativeSessionDeleter,四家
// HYBRID provider 各自实现,DeleteThread 只做类型断言 —— 按 agent 名字分支的
// 静态名单就此消掉(README §7:静态名单是债;AUDIT §3.3)。
//
// 三条失败语义与 codex 分支一致(逐条与 codex_thread_delete_sync_test.go 对偶):
//  1. 调用发出:真删除必须向 provider 发出原生删除;
//  2. 失败不阻塞:原生删除失败只留日志,本地删除与墓碑照常(墓碑是防复活权威);
//  3. LocalOnly 移除绝不发原生删除。

package server

import (
	"context"
	"errors"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeDshDeleter 测试替身:注册进 Providers["dsh"],记录原生删除调用并按配置
// 返回成功/失败(不碰任何真子进程)。实现形态与生产 *agent.DshProvider 一致
// (DeleteNativeSession 从 thread.ExternalSession 取会话 id)。
type fakeDshDeleter struct {
	calls []string
	err   error
}

func (f *fakeDshDeleter) ID() string          { return "dsh" }
func (f *fakeDshDeleter) DisplayName() string { return "dsh 原生删除替身" }
func (f *fakeDshDeleter) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "default", Provider: "dsh", Available: true}}
}
func (f *fakeDshDeleter) Run(ctx context.Context, req agent.TurnRequest, sink agent.Sink, approve agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, errors.New("替身不执行轮次")
}
func (f *fakeDshDeleter) DeleteNativeSession(t *store.Thread) error {
	f.calls = append(f.calls, t.ExternalSession.SessionID)
	return f.err
}

func newDshDeleteServer(t *testing.T, fake *fakeDshDeleter) *Server {
	t.Helper()
	s := newTestServer(t)
	s.cfg.Providers = map[string]agent.Provider{"dsh": fake}
	return s
}

// 真删除必须把 dsh 会话的原生删除发出去(session/close),本地记录与墓碑照常。
func TestDeleteThreadIssuesDshNativeDelete(t *testing.T) {
	fake := &fakeDshDeleter{}
	s := newDshDeleteServer(t, fake)
	th := &store.Thread{ID: "th-dsh-del", AgentID: "dsh", Title: "dsh 真删",
		ExternalSession: store.ExternalSessionRef{SessionID: "ds-del-ok", Cwd: `G:\ws`}}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-dsh-del"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}
	if len(fake.calls) != 1 || fake.calls[0] != "ds-del-ok" {
		t.Fatalf("dsh 原生删除应对外部会话 id 发出且只发出一次, got=%v", fake.calls)
	}
	if _, ok := s.cfg.Store.Get("th-dsh-del"); ok {
		t.Fatal("本地会话应已删除")
	}
	if !s.cfg.Store.IsTombstoned("ds-del-ok") {
		t.Fatal("墓碑应落盘(防原生同步链复活)")
	}
}

// 原生删除失败(session/close 报错/子进程不在)→ 只留日志,本地删除与墓碑照常:
// 用户点删除的本意是"让它从手机上消失",电脑侧残留由墓碑挡住 + 导入列表兜底。
func TestDeleteThreadSucceedsWhenDshNativeDeleteFails(t *testing.T) {
	fake := &fakeDshDeleter{err: errors.New("模拟协议故障: dsh 子进程不在")}
	s := newDshDeleteServer(t, fake)
	th := &store.Thread{ID: "th-dsh-fail", AgentID: "dsh", Title: "原生删不掉",
		ExternalSession: store.ExternalSessionRef{SessionID: "ds-del-fail", Cwd: `G:\ws`}}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-dsh-fail"}); err != nil {
		t.Fatalf("原生删除失败不得阻塞本地删除: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("失败路径同样必须先发出调用(只留日志不吞调用), got=%v", fake.calls)
	}
	if _, ok := s.cfg.Store.Get("th-dsh-fail"); ok {
		t.Fatal("本地会话应已删除")
	}
	if !s.cfg.Store.IsTombstoned("ds-del-fail") {
		t.Fatal("原生失败时墓碑更要落盘,否则下一次同步立刻复活")
	}
}

// LocalOnly 移除是可恢复的隐藏,不是真删:绝不发 dsh 原生删除。
func TestLocalOnlyRemoveSkipsDshNativeDelete(t *testing.T) {
	fake := &fakeDshDeleter{}
	s := newDshDeleteServer(t, fake)
	th := &store.Thread{ID: "th-dsh-soft", AgentID: "dsh", Title: "仅移除",
		ExternalSession: store.ExternalSessionRef{SessionID: "ds-del-soft", Cwd: `G:\ws`}}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{
		ThreadId: "th-dsh-soft", LocalOnly: true,
	}); err != nil {
		t.Fatalf("移除应成功: %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("local_only 移除不该发 dsh 原生删除, got=%v", fake.calls)
	}
}

// TestNativeSessionDeleterCoversHybridFour 钉「零名单」的两端:
//   - 正向:四家 HYBRID provider 都实现 nativeSessionDeleter —— 接口断言探测
//     在生产形态下必然命中,谁的实现丢了这条就红;
//   - 反向:daemon 侧会话的家(echo/OpenAI 兼容)沉默缺席 —— 它们没有"电脑上的
//     原生会话"可删,缺席即如实表达,不需要空实现凑数。
func TestNativeSessionDeleterCoversHybridFour(t *testing.T) {
	native := map[string]agent.Provider{
		"codex": agent.NewCodexProvider(),
		"pi":    agent.NewPiProvider(),
		"zcode": agent.NewZcodeProvider(),
		"dsh":   agent.NewDshProvider(),
	}
	for id, p := range native {
		if _, ok := p.(nativeSessionDeleter); !ok {
			t.Errorf("%s 应实现 nativeSessionDeleter(删除走接口断言,零名单)", id)
		}
	}
	daemonSide := map[string]agent.Provider{
		"echo":   agent.NewEcho(),
		"openai": agent.NewOpenAICompatible("openai", "OpenAI 兼容", "http://127.0.0.1:1/v1", "k", nil),
	}
	for id, p := range daemonSide {
		if _, ok := p.(nativeSessionDeleter); ok {
			t.Errorf("%s 是 daemon 侧会话,不应实现 nativeSessionDeleter(缺席即如实表达)", id)
		}
	}
}
