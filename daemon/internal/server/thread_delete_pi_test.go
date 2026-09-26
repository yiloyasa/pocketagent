package server

// thread_delete_pi_test.go —— P2-1 回归钉子:删除 pi 会话必须回收子进程。
//
// 【背景】pi 一个子进程就是一个会话;修复前 DeleteThread 只清 codex rollout 与
// zcode session,piapp.Client 留在 provider 的 clients 里,node 子进程一直空转
// 到 daemon 重启(进程+游标泄漏)。真删除路径现经 nativeSessionDeleter(P2 删除
// 接口化)调 *agent.PiProvider.DeleteNativeSession(内含 DropThread);
// LocalOnly 移除路径仍经最小接口 piThreadDropper 调 DropThread(见 thread.go)。

import (
	"sync"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// piDropStub 记录 DropThread 调用的 provider 桩(复用 PiProviderStub 的接口实现)。
// P2 删除接口化后,真删除路径经 nativeSessionDeleter 进来:桩的 DeleteNativeSession
// 与生产 *agent.PiProvider 同构 —— 先 DropThread(回收子进程),文件删除在桩里省略。
type piDropStub struct {
	PiProviderStub
	mu    sync.Mutex
	drops []string
}

func (s *piDropStub) DropThread(threadID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drops = append(s.drops, threadID)
}

func (s *piDropStub) DeleteNativeSession(t *store.Thread) error {
	s.DropThread(t.ID)
	return nil
}

func (s *piDropStub) dropCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.drops)
}

func newDeleteTestServer(t *testing.T, providers map[string]agent.Provider) *Server {
	t.Helper()
	return New(Config{Store: mustStore(t), Providers: providers})
}

// TestDeleteThreadDropsPiClient 删 pi 会话:DropThread 必须被调、线程照常删除。
func TestDeleteThreadDropsPiClient(t *testing.T) {
	stub := &piDropStub{}
	srv := newDeleteTestServer(t, map[string]agent.Provider{"pi": stub})
	th := &store.Thread{ID: "th-pi-del", Title: "pi 会话", AgentID: "pi", Workspace: "C:\\ws"}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("建测试线程: %v", err)
	}
	if _, err := srv.DeleteThread(ctxBG(), &pb.DeleteThreadRequest{ThreadId: "th-pi-del"}); err != nil {
		t.Fatalf("DeleteThread: %v", err)
	}
	if got := stub.dropCount(); got != 1 {
		t.Fatalf("pi 线程删除应回收子进程(DropThread 调 %d 次)", got)
	}
	if _, ok := srv.cfg.Store.Get("th-pi-del"); ok {
		t.Fatal("线程应已从库里删除")
	}
}

// TestDeleteThreadNonPiNoDrop 非 pi 会话(codex/zcode)不得触发 DropThread;
// 删除本身照常成功。
func TestDeleteThreadNonPiNoDrop(t *testing.T) {
	stub := &piDropStub{}
	echo := &seqProvider{id: "codex"}
	srv := newDeleteTestServer(t, map[string]agent.Provider{
		"pi":    stub,
		"codex": echo,
	})
	for _, th := range []*store.Thread{
		{ID: "th-codex", Title: "codex 会话", AgentID: "codex", Workspace: "C:\\ws"},
		{ID: "th-legacy", Title: "旧库记录", Workspace: "C:\\ws"}, // AgentID 缺省 = codex
	} {
		if err := srv.cfg.Store.Create(th); err != nil {
			t.Fatalf("建测试线程: %v", err)
		}
	}
	for _, id := range []string{"th-codex", "th-legacy"} {
		if _, err := srv.DeleteThread(ctxBG(), &pb.DeleteThreadRequest{ThreadId: id}); err != nil {
			t.Fatalf("DeleteThread(%s): %v", id, err)
		}
	}
	if got := stub.dropCount(); got != 0 {
		t.Fatalf("非 pi 会话不应触发 DropThread,调了 %d 次", got)
	}
}

// TestDeleteThreadMissingProviderTolerated pi provider 不在注册表(未安装)时,
// 删除本地会话不得失败(与 codex/zcode 分支的降级纪律一致)。
func TestDeleteThreadMissingProviderTolerated(t *testing.T) {
	srv := newDeleteTestServer(t, map[string]agent.Provider{})
	th := &store.Thread{ID: "th-pi-noprovider", Title: "pi 会话", AgentID: "pi", Workspace: "C:\\ws"}
	if err := srv.cfg.Store.Create(th); err != nil {
		t.Fatalf("建测试线程: %v", err)
	}
	if _, err := srv.DeleteThread(ctxBG(), &pb.DeleteThreadRequest{ThreadId: "th-pi-noprovider"}); err != nil {
		t.Fatalf("provider 缺失时删除不得失败: %v", err)
	}
}
